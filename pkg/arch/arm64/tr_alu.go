package arm64

import (
	"fmt"

	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// ALU 翻译 — 算术/逻辑/移动指令
// ============================================================

func (t *Translator) trAluImm(inst vm.Instruction, vmOp byte) error {
	return t.trAluImmFlags(inst, vmOp, false)
}

func (t *Translator) trAluImmFlags(inst vm.Instruction, vmOp byte, setFlags bool) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	imm64 := uint64(inst.Imm)
	if imm64 > 0xFFFFFFFF {
		// 64-bit 立即数超出 u32 范围 — 用 MOV_IMM64 加载到 R15，再用 3-reg 指令
		reg3Op := immToReg3Op(vmOp)
		if reg3Op == 0 {
			return fmt.Errorf("无法将 _IMM opcode 0x%02X 映射到 3-reg 版本", vmOp)
		}
		t.emit(vm.OpMovImm, 15)
		t.emitU64(imm64)
		t.emit(reg3Op, rd, rn, 15)
	} else {
		t.emit(vmOp, rd, rn)
		t.emitU32(uint32(imm64))
	}

	if setFlags {
		// ADDS/SUBS: 在 trunc32 之前比较，确保 N flag 正确
		t.emit(vm.OpCmpImm, rd)
		t.emitU32(0)
	}
	if !inst.SF {
		t.trunc32(rd)
	}
	return nil
}

// immToReg3Op 将 _IMM opcode 映射到对应的 3-register opcode
func immToReg3Op(immOp byte) byte {
	switch immOp {
	case vm.OpAddImm:
		return vm.OpAdd
	case vm.OpSubImm:
		return vm.OpSub
	case vm.OpAndImm:
		return vm.OpAnd
	case vm.OpOrImm:
		return vm.OpOr
	case vm.OpXorImm:
		return vm.OpXor
	case vm.OpMulImm:
		return vm.OpMul
	case vm.OpShlImm:
		return vm.OpShl
	case vm.OpShrImm:
		return vm.OpShr
	case vm.OpAsrImm:
		return vm.OpAsr
	default:
		return 0
	}
}

func (t *Translator) trAluReg(inst vm.Instruction, vmOp byte) error {
	return t.trAluRegFlags(inst, vmOp, false)
}

func (t *Translator) trAluRegFlags(inst vm.Instruction, vmOp byte, setFlags bool) error {
	// ARM64 shifted-register ALU: reg 31 = XZR (not SP)
	// decoder 已标记为 REG_XZR, mapReg 映射到 R16
	// 这里需要对 Rn/Rm 为 XZR 时先清零 R16/R15
	if inst.Rn == vm.REG_XZR {
		t.emit(vm.OpMovImm32, 16) // R16 = 0
		t.emitU32(0)
	}
	if inst.Rm == vm.REG_XZR {
		t.emit(vm.OpMovImm32, 15) // R15 = 0 (用不同寄存器避免 Rn==Rm==XZR 冲突)
		t.emitU32(0)
	}

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	// Rn 和 Rm 同时为 XZR 时，mapReg 都返回 16
	// 但 Rm 已经清零到 R15，所以需要修正
	if inst.Rm == vm.REG_XZR {
		rm = 15
	}

	if inst.Shift != 0 {
		// 32-bit 模式: VM 的移位 handler 全部按 64-bit 操作
		// ARM64 W-register shifted operand (EOR Wd, Wn, Wm, LSR #n 等) 是 32-bit 移位
		// 必须先截断输入到 32-bit，移位后再截断输出
		if !inst.SF {
			t.emit(vm.OpAndImm, 15, rm) // R15 = Rm & 0xFFFFFFFF
			t.emitU32(0xFFFFFFFF)
			rm = 15
		}
		switch inst.ShiftType {
		case 0: // LSL
			t.emit(vm.OpShlImm, 15, rm)
			t.emitU32(uint32(inst.Shift))
		case 1: // LSR
			t.emit(vm.OpShrImm, 15, rm)
			t.emitU32(uint32(inst.Shift))
		case 2: // ASR — 32-bit ASR 需要先符号扩展到 64-bit
			if !inst.SF {
				// 32-bit ASR: 先 SHL 32 使符号位到 bit63，再 ASR (32+shift)
				t.emit(vm.OpShlImm, 15, rm)
				t.emitU32(32)
				t.emit(vm.OpAsrImm, 15, 15)
				t.emitU32(32 + uint32(inst.Shift))
			} else {
				t.emit(vm.OpAsrImm, 15, rm)
				t.emitU32(uint32(inst.Shift))
			}
		case 3: // ROR
			if !inst.SF {
				// 32-bit ROR: 不能用 64-bit OpRor（高位回绕会导致错误）
				// 改用 SHR + SHL + OR 模拟: ROR32(v, n) = (v >> n) | (v << (32-n))
				shift := uint32(inst.Shift) & 31
				if shift == 0 {
					// shift=0: no rotation needed, rm already in R15
				} else {
					t.emit(vm.OpShrImm, 14, rm) // R14 = R15 >> shift
					t.emitU32(shift)
					t.emit(vm.OpShlImm, 15, rm) // R15 = R15 << (32-shift)
					t.emitU32(32 - shift)
					t.emit(vm.OpOr, 15, 15, 14) // R15 = R15 | R14
				}
			} else {
				t.emit(vm.OpMovImm32, 14)
				t.emitU32(uint32(inst.Shift))
				t.emit(vm.OpRor, 15, rm, 14)
			}
		}
		// 32-bit 模式: 截断移位结果
		if !inst.SF {
			t.trunc32(15)
		}
		t.emit(vmOp, rd, rn, 15)
	} else {
		t.emit(vmOp, rd, rn, rm)
	}
	if setFlags {
		// ADDS/SUBS: 在 trunc32 之前比较，确保 N flag 正确
		t.emit(vm.OpCmpImm, rd)
		t.emitU32(0)
	}
	if !inst.SF {
		t.trunc32(rd)
	}
	// Rd==XZR: 结果写入 R16，等价于丢弃
	return nil
}

func (t *Translator) trMov(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	val := uint64(inst.Imm) << uint(inst.Shift)
	if val <= 0xFFFFFFFF {
		t.emit(vm.OpMovImm32, rd)
		t.emitU32(uint32(val))
	} else {
		t.emit(vm.OpMovImm, rd)
		t.emitU64(val)
	}
	return nil
}

func (t *Translator) trMovK(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	shift := uint(inst.Shift)
	val := uint64(inst.Imm) << shift
	mask := uint64(0xFFFF) << shift

	// 动态选择临时寄存器，避免与 rd 冲突 (Rd=X15 时不能用 R15)
	tmp := t.pickTemp(rd)
	t.emit(vm.OpMovImm, tmp)
	t.emitU64(^mask)
	t.emit(vm.OpAnd, rd, rd, tmp)

	// OR in the new value
	if val <= 0xFFFFFFFF {
		t.emit(vm.OpOrImm, rd, rd)
		t.emitU32(uint32(val))
	} else {
		t.emit(vm.OpMovImm, tmp)
		t.emitU64(val)
		t.emit(vm.OpOr, rd, rd, tmp)
	}
	return nil
}

func (t *Translator) trMovN(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	val := ^(uint64(inst.Imm) << uint(inst.Shift))
	if val <= 0xFFFFFFFF || !inst.SF {
		t.emit(vm.OpMovImm32, rd)
		t.emitU32(uint32(val))
	} else {
		t.emit(vm.OpMovImm, rd)
		t.emitU64(val)
	}
	return nil
}

// trEON 翻译 EON: Rd = Rn XOR NOT(shift(Rm))
func (t *Translator) trEON(inst vm.Instruction) error {
	if inst.Rn == vm.REG_XZR {
		t.emit(vm.OpMovImm32, 16)
		t.emitU32(0)
	}
	if inst.Rm == vm.REG_XZR {
		t.emit(vm.OpMovImm32, 15)
		t.emitU32(0)
	}

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	if inst.Rm == vm.REG_XZR {
		rm = 15
	}

	// R15 = shift(Rm)
	if inst.Shift != 0 {
		// 32-bit 模式: 需要先截断输入
		if !inst.SF {
			t.emit(vm.OpAndImm, 15, rm)
			t.emitU32(0xFFFFFFFF)
			rm = 15
		}
		switch inst.ShiftType {
		case 0: // LSL
			t.emit(vm.OpShlImm, 15, rm)
			t.emitU32(uint32(inst.Shift))
		case 1: // LSR
			t.emit(vm.OpShrImm, 15, rm)
			t.emitU32(uint32(inst.Shift))
		case 2: // ASR
			if !inst.SF {
				t.emit(vm.OpShlImm, 15, rm)
				t.emitU32(32)
				t.emit(vm.OpAsrImm, 15, 15)
				t.emitU32(32 + uint32(inst.Shift))
			} else {
				t.emit(vm.OpAsrImm, 15, rm)
				t.emitU32(uint32(inst.Shift))
			}
		case 3: // ROR
			if !inst.SF {
				// 32-bit ROR: 用 SHR + SHL + OR 模拟
				shift := uint32(inst.Shift) & 31
				if shift == 0 {
					// shift=0: no rotation
				} else {
					t.emit(vm.OpShrImm, 14, rm) // R14 = R15 >> shift
					t.emitU32(shift)
					t.emit(vm.OpShlImm, 15, rm) // R15 = R15 << (32-shift)
					t.emitU32(32 - shift)
					t.emit(vm.OpOr, 15, 15, 14) // R15 = R15 | R14
				}
			} else {
				t.emit(vm.OpMovImm32, 14)
				t.emitU32(uint32(inst.Shift))
				t.emit(vm.OpRor, 15, rm, 14)
			}
		}
		if !inst.SF {
			t.trunc32(15)
		}
		rm = 15
	}
	// R15 = NOT(shift(Rm))
	t.emit(vm.OpNot, 15, rm)
	// Rd = Rn XOR R15
	t.emit(vm.OpXor, rd, rn, 15)
	if !inst.SF {
		t.trunc32(rd)
	}
	return nil
}

// trMADD 翻译 MADD/MSUB
// MADD: Rd = Ra + Rn * Rm  (isSub=false)
// MSUB: Rd = Ra - Rn * Rm  (isSub=true)
// Ra 从 inst.Raw bits[14:10] 提取
func (t *Translator) trMADD(inst vm.Instruction, isSub bool) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	// Ra = bits[14:10]
	raIdx := int((inst.Raw >> 10) & 0x1F)
	if raIdx == 31 {
		raIdx = vm.REG_XZR
	}
	ra, err := t.mapReg(raIdx)
	if err != nil {
		return err
	}

	// 如果 Ra 是 XZR，先清零
	if raIdx == vm.REG_XZR {
		t.emit(vm.OpMovImm32, ra)
		t.emitU32(0)
	}

	// 动态选择临时寄存器，避免与 rd/rn/rm/ra 冲突
	tmp := t.pickTemp(rd, rn, rm, ra)
	t.emit(vm.OpMul, tmp, rn, rm)
	if isSub {
		t.emit(vm.OpSub, rd, ra, tmp)
	} else {
		t.emit(vm.OpAdd, rd, ra, tmp)
	}
	if !inst.SF {
		t.trunc32(rd)
	}
	return nil
}

// trSMADDL 翻译 SMADDL/SMSUBL
// SMADDL: Xd = Xa + SEXT(Wn) * SEXT(Wm)  (isSub=false)
// SMSUBL: Xd = Xa - SEXT(Wn) * SEXT(Wm)  (isSub=true)
// Ra 从 inst.Raw bits[14:10] 提取
// 翻译策略: R14=SEXT(Wn), R15=SEXT(Wm), R15=R14*R15, Rd=Ra+/-R15
func (t *Translator) trSMADDL(inst vm.Instruction, isSub bool) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	// Ra = bits[14:10]
	raIdx := int((inst.Raw >> 10) & 0x1F)
	if raIdx == 31 {
		raIdx = vm.REG_XZR
	}
	ra, err := t.mapReg(raIdx)
	if err != nil {
		return err
	}

	// 如果 Ra 是 XZR，先清零 (SMULL/SMNEGL alias)
	if raIdx == vm.REG_XZR {
		t.emit(vm.OpMovImm32, ra)
		t.emitU32(0)
	}

	// R14 = SEXT(Wn): SHL 32 → ASR 32
	t.emit(vm.OpShlImm, 14, rn, 32)
	t.emit(vm.OpAsrImm, 14, 14, 32)

	// R15 = SEXT(Wm): SHL 32 → ASR 32
	t.emit(vm.OpShlImm, 15, rm, 32)
	t.emit(vm.OpAsrImm, 15, 15, 32)

	// R15 = R14 * R15
	t.emit(vm.OpMul, 15, 14, 15)

	// Rd = Ra +/- R15
	if isSub {
		t.emit(vm.OpSub, rd, ra, 15)
	} else {
		t.emit(vm.OpAdd, rd, ra, 15)
	}
	// SMADDL 结果总是 64-bit, 不需要 trunc32
	return nil
}

// trUMADDL 翻译 UMADDL/UMSUBL
// UMADDL: Xd = Xa + ZEXT(Wn) * ZEXT(Wm)  (isSub=false)
// UMSUBL: Xd = Xa - ZEXT(Wn) * ZEXT(Wm)  (isSub=true)
// Ra 从 inst.Raw bits[14:10] 提取
// 翻译策略: R14=ZEXT(Wn), R15=ZEXT(Wm), R15=R14*R15, Rd=Ra+/-R15
func (t *Translator) trUMADDL(inst vm.Instruction, isSub bool) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	// Ra = bits[14:10]
	raIdx := int((inst.Raw >> 10) & 0x1F)
	if raIdx == 31 {
		raIdx = vm.REG_XZR
	}
	ra, err := t.mapReg(raIdx)
	if err != nil {
		return err
	}

	// 如果 Ra 是 XZR，先清零 (UMULL/UMNEGL alias)
	if raIdx == vm.REG_XZR {
		t.emit(vm.OpMovImm32, ra)
		t.emitU32(0)
	}

	// R14 = ZEXT(Wn): 清零高 32 位
	t.emit(vm.OpMovReg, 14, rn)
	t.trunc32(14)

	// R15 = ZEXT(Wm): 清零高 32 位
	t.emit(vm.OpMovReg, 15, rm)
	t.trunc32(15)

	// R15 = R14 * R15
	t.emit(vm.OpMul, 15, 14, 15)

	// Rd = Ra +/- R15
	if isSub {
		t.emit(vm.OpSub, rd, ra, 15)
	} else {
		t.emit(vm.OpAdd, rd, ra, 15)
	}
	// UMADDL 结果总是 64-bit, 不需要 trunc32
	return nil
}

// trUmulh 翻译 UMULH Xd, Xn, Xm — 无符号高 64 位乘法
// 格式: [OpUmulh][d][n][m] = 4B
// UMULH 始终 64-bit (sf=1), 无 32-bit 变体
func (t *Translator) trUmulh(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	// Rd == XZR → 结果写入 R16, 等价于丢弃
	t.emit(vm.OpUmulh, rd, rn, rm)
	return nil
}

// trAddSubExt 翻译 ADD/SUB (extended register): Rd = Rn op extend(Rm, shift)
// option (ShiftType): 0=UXTB, 1=UXTH, 2=UXTW, 3=UXTX, 4=SXTB, 5=SXTH, 6=SXTW, 7=SXTX
// imm3 (Shift): 额外左移量 0-4
func (t *Translator) trAddSubExt(inst vm.Instruction, vmOp byte, setFlags bool) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	// Rn=31 在 extended register 中是 SP (decoder 未做 XZR 替换)
	// Rm 已被 decoder 做了 XZR 替换

	// 动态选择临时寄存器，避免与 rd/rn/rm 冲突
	tmp := t.pickTemp(rd, rn, rm)

	if inst.Rm == vm.REG_XZR {
		t.emit(vm.OpMovImm32, tmp)
		t.emitU32(0)
		rm = tmp
	}

	// tmp = extend(Rm)
	option := inst.ShiftType
	switch option {
	case 0: // UXTB: zero-extend byte
		t.emit(vm.OpAndImm, tmp, rm)
		t.emitU32(0xFF)
	case 1: // UXTH: zero-extend halfword
		t.emit(vm.OpAndImm, tmp, rm)
		t.emitU32(0xFFFF)
	case 2: // UXTW: zero-extend word
		t.emit(vm.OpAndImm, tmp, rm)
		t.emitU32(0xFFFFFFFF)
	case 3: // UXTX: no extension (64-bit)
		t.emit(vm.OpMovReg, tmp, rm)
	case 4: // SXTB: sign-extend byte → SHL 56, ASR 56
		t.emit(vm.OpShlImm, tmp, rm)
		t.emitU32(56)
		t.emit(vm.OpAsrImm, tmp, tmp)
		t.emitU32(56)
	case 5: // SXTH: sign-extend halfword → SHL 48, ASR 48
		t.emit(vm.OpShlImm, tmp, rm)
		t.emitU32(48)
		t.emit(vm.OpAsrImm, tmp, tmp)
		t.emitU32(48)
	case 6: // SXTW: sign-extend word → SHL 32, ASR 32
		t.emit(vm.OpShlImm, tmp, rm)
		t.emitU32(32)
		t.emit(vm.OpAsrImm, tmp, tmp)
		t.emitU32(32)
	case 7: // SXTX: no extension (64-bit signed = nop)
		t.emit(vm.OpMovReg, tmp, rm)
	}

	// 额外左移
	if inst.Shift > 0 {
		t.emit(vm.OpShlImm, tmp, tmp)
		t.emitU32(uint32(inst.Shift))
	}

	// Rd = Rn op tmp
	t.emit(vmOp, rd, rn, tmp)

	if setFlags {
		t.emit(vm.OpCmpImm, rd)
		t.emitU32(0)
	}
	if !inst.SF {
		t.trunc32(rd)
	}
	return nil
}

// trCCMP 翻译 CCMP/CCMN (reg/imm)
// 字节码: [op][cond][nzcv][rn][rm_or_imm5][sf] = 6B
// inst.Cond = condition, inst.WB = nzcv (default flags)
// isNeg: true=CCMN, false=CCMP
// isImm: true=imm5 variant (inst.Rm reused as imm5), false=reg variant
func (t *Translator) trCCMP(inst vm.Instruction, isNeg bool, isImm bool) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	var vmOp byte
	if isNeg {
		if isImm {
			vmOp = vm.OpCcmnImm
		} else {
			vmOp = vm.OpCcmnReg
		}
	} else {
		if isImm {
			vmOp = vm.OpCcmpImm
		} else {
			vmOp = vm.OpCcmpReg
		}
	}

	var rmOrImm byte
	if isImm {
		rmOrImm = byte(inst.Rm) // Rm field reused as imm5
	} else {
		rm, err := t.mapReg(inst.Rm)
		if err != nil {
			return err
		}
		rmOrImm = rm
	}

	var sf byte
	if inst.SF {
		sf = 1
	}

	t.emit(vmOp, byte(inst.Cond), byte(inst.WB), rn, rmOrImm, sf)
	return nil
}

// trUDIV 翻译 UDIV Xd, Xn, Xm — 无符号除法
// 格式: [OpUdiv][d][n][m] = 4B (和 MUL 格式一样)
func (t *Translator) trUDIV(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	t.emit(vm.OpUdiv, rd, rn, rm)
	if !inst.SF {
		t.trunc32(rd)
	}
	return nil
}

// trSDIV 翻译 SDIV Xd, Xn, Xm — 有符号除法
// 格式: [OpSdiv][d][n][m] = 4B
// 32位模式: 先 sext32 操作数(用临时寄存器)，SDIV，再 trunc32 结果
func (t *Translator) trSDIV(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	if !inst.SF {
		// 32位 SDIV: 操作数需要从低32位符号扩展到64位
		// 动态选择临时寄存器，避免与 rd/rn/rm 冲突
		tmpN, tmpM := t.pickTemp2(rd, rn, rm)
		t.emit(vm.OpMovReg, tmpN, rn)
		t.sext32(tmpN)
		t.emit(vm.OpMovReg, tmpM, rm)
		t.sext32(tmpM)
		t.emit(vm.OpSdiv, rd, tmpN, tmpM)
		t.trunc32(rd)
	} else {
		t.emit(vm.OpSdiv, rd, rn, rm)
	}
	return nil
}

// trMRS 翻译 MRS Xd, <sysreg> — 读取系统寄存器
// 格式: [OpMrs][d][sysreg_lo][sysreg_hi] = 4B
// sysreg 是 15-bit 编码，存为 uint16 LE
func (t *Translator) trMRS(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	sysreg := uint16(inst.Imm & 0x7FFF)
	t.emit(vm.OpMrs, rd, byte(sysreg&0xFF), byte(sysreg>>8))
	return nil
}

// trBitLogicalNot 翻译 Rd = Rn OP NOT(shift(Rm))
// vmOp: OpAnd → BIC, OpOr → ORN, OpXor → EON
// setFlags: true → BICS (BIC + set flags)
func (t *Translator) trBitLogicalNot(inst vm.Instruction, vmOp byte, setFlags bool) error {
	if inst.Rn == vm.REG_XZR {
		t.emit(vm.OpMovImm32, 16) // R16 = 0
		t.emitU32(0)
	}
	if inst.Rm == vm.REG_XZR {
		t.emit(vm.OpMovImm32, 15) // R15 = 0
		t.emitU32(0)
	}

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	if inst.Rm == vm.REG_XZR {
		rm = 15
	}

	// R15 = shift(Rm)
	if inst.Shift != 0 {
		if !inst.SF {
			t.emit(vm.OpAndImm, 15, rm)
			t.emitU32(0xFFFFFFFF)
			rm = 15
		}
		switch inst.ShiftType {
		case 0: // LSL
			t.emit(vm.OpShlImm, 15, rm)
			t.emitU32(uint32(inst.Shift))
		case 1: // LSR
			t.emit(vm.OpShrImm, 15, rm)
			t.emitU32(uint32(inst.Shift))
		case 2: // ASR
			if !inst.SF {
				t.emit(vm.OpShlImm, 15, rm)
				t.emitU32(32)
				t.emit(vm.OpAsrImm, 15, 15)
				t.emitU32(32 + uint32(inst.Shift))
			} else {
				t.emit(vm.OpAsrImm, 15, rm)
				t.emitU32(uint32(inst.Shift))
			}
		case 3: // ROR
			if !inst.SF {
				shift := uint32(inst.Shift) & 31
				if shift != 0 {
					t.emit(vm.OpShrImm, 14, rm)
					t.emitU32(shift)
					t.emit(vm.OpShlImm, 15, rm)
					t.emitU32(32 - shift)
					t.emit(vm.OpOr, 15, 15, 14)
				}
			} else {
				t.emit(vm.OpMovImm32, 14)
				t.emitU32(uint32(inst.Shift))
				t.emit(vm.OpRor, 15, rm, 14)
			}
		}
		if !inst.SF {
			t.trunc32(15)
		}
		rm = 15
	}
	// R15 = NOT(shift(Rm))
	t.emit(vm.OpNot, 15, rm)
	// Rd = Rn OP R15
	t.emit(vmOp, rd, rn, 15)
	if setFlags {
		t.emit(vm.OpCmpImm, rd)
		t.emitU32(0)
	}
	if !inst.SF {
		t.trunc32(rd)
	}
	return nil
}

// trBFM 翻译 BFM Xd, Xn, #immr, #imms — 位域移动
// BFI alias:   imms < immr → dst_lsb = regsize-immr, width = imms+1
// BFXIL alias: imms >= immr → dst_lsb = 0, width = imms-immr+1
func (t *Translator) trBFM(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	immr := uint32(inst.Imm)
	imms := uint32(inst.Shift)
	regsize := uint32(64)
	if !inst.SF {
		regsize = 32
	}

	var width, srcLSB, dstLSB uint32
	if imms >= immr {
		// BFXIL: 从 Rn 提取 bits[imms:immr] 插入 Rd 底部
		width = imms - immr + 1
		srcLSB = immr
		dstLSB = 0
	} else {
		// BFI: 从 Rn 提取低 width 位插入 Rd 的 dstLSB 位置
		width = imms + 1
		srcLSB = 0
		dstLSB = regsize - immr
	}

	mask := uint64((1 << width) - 1)

	// 选择不与 rd/rn 冲突的临时寄存器
	tmp, tmp2 := t.pickTemp2(rd, rn)

	// tmp = (Rn >> srcLSB) & mask
	if srcLSB > 0 {
		t.emit(vm.OpShrImm, tmp, rn)
		t.emitU32(srcLSB)
	} else {
		t.emit(vm.OpMovReg, tmp, rn)
	}
	if mask <= 0xFFFFFFFF {
		t.emit(vm.OpAndImm, tmp, tmp)
		t.emitU32(uint32(mask))
	} else {
		t.emit(vm.OpMovImm, tmp2)
		t.emitU64(mask)
		t.emit(vm.OpAnd, tmp, tmp, tmp2)
	}

	// tmp <<= dstLSB
	if dstLSB > 0 {
		t.emit(vm.OpShlImm, tmp, tmp)
		t.emitU32(dstLSB)
	}

	// Rd = (Rd & ~(mask << dstLSB)) | tmp
	clearMask := ^(mask << dstLSB)
	if !inst.SF {
		clearMask &= 0xFFFFFFFF
	}
	if clearMask <= 0xFFFFFFFF {
		t.emit(vm.OpAndImm, rd, rd)
		t.emitU32(uint32(clearMask))
	} else {
		t.emit(vm.OpMovImm, tmp2)
		t.emitU64(clearMask)
		t.emit(vm.OpAnd, rd, rd, tmp2)
	}
	t.emit(vm.OpOr, rd, rd, tmp)

	if !inst.SF {
		t.trunc32(rd)
	}
	return nil
}

// trSmulh 翻译 SMULH Xd, Xn, Xm — 有符号高 64 位乘法
// 格式: [OpSmulh][d][n][m] = 4B, 始终 64-bit
func (t *Translator) trSmulh(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	t.emit(vm.OpSmulh, rd, rn, rm)
	return nil
}

// trUnary 翻译单操作数指令: CLZ/CLS/RBIT/REV/REV16/REV32
// 格式: [vmOp][d][n] = 3B
func (t *Translator) trUnary(inst vm.Instruction, vmOp byte) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	t.emit(vmOp, rd, rn)
	if !inst.SF {
		t.trunc32(rd)
	}
	return nil
}
