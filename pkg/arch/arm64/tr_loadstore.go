package arm64

import (
	"encoding/binary"

	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// 加载/存储翻译 — LDR / STR / STP / LDP / LDR_REG
// ============================================================

func (t *Translator) trLoad(inst vm.Instruction) error {
	// ARM64: Rd=REG_XZR 在 LDR 上下文 = XZR (丢弃结果, decoder 已标记)
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	op := Op(inst.Op)
	var vmOp byte
	switch op {
	case LDRB_IMM:
		vmOp = vm.OpLoad8
	case LDR_IMM:
		if inst.SF {
			vmOp = vm.OpLoad64
		} else {
			vmOp = vm.OpLoad32
		}
	case LDRSB_IMM:
		// LDRSB: LOAD8 + SHL 56 + ASR 56 (符号扩展 8→64)
		vmOp = vm.OpLoad8
	case LDRH_IMM:
		vmOp = vm.OpLoad16
	case LDRSH_IMM:
		// LDRSH: LOAD16 + SHL 48 + ASR 48 (符号扩展 16→64)
		vmOp = vm.OpLoad16
	case LDRSW_IMM:
		// LDRSW: LOAD32 + SHL 32 + ASR 32 (符号扩展 32→64)
		vmOp = vm.OpLoad32
	default:
		vmOp = vm.OpLoad64
	}

	// post-index: load from [Rn+0], then Rn += imm
	// pre-index:  Rn += imm first, then load from [Rn+0]
	emitWriteback := func() {
		wbImm := inst.Imm
		if wbImm >= 0 {
			t.emit(vm.OpAddImm, rn, rn)
		} else {
			t.emit(vm.OpSubImm, rn, rn)
			wbImm = -wbImm
		}
		t.emitU32(uint32(wbImm))
	}

	if inst.WB == 3 {
		// pre-index: 先更新 base, 再以 offset=0 加载
		emitWriteback()
		t.emit(vmOp, rd, rn)
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, 0)
		t.code = append(t.code, b...)
	} else if inst.WB == 1 {
		// post-index: 先以 offset=0 加载, 再更新 base
		t.emit(vmOp, rd, rn)
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, 0)
		t.code = append(t.code, b...)
		emitWriteback()
	} else {
		// unsigned/unscaled offset
		if inst.Imm < 0 {
			// LDUR/STUR 负偏移: 先计算实际地址到 R16, 再以 offset=0 加载
			tmp := byte(16)
			t.emit(vm.OpSubImm, tmp, rn)
			t.emitU32(uint32(-inst.Imm))
			t.emit(vmOp, rd, tmp)
			b := make([]byte, 2)
			binary.LittleEndian.PutUint16(b, 0)
			t.code = append(t.code, b...)
		} else {
			t.emit(vmOp, rd, rn)
			b := make([]byte, 2)
			binary.LittleEndian.PutUint16(b, uint16(inst.Imm))
			t.code = append(t.code, b...)
		}
	}

	// LDRSW: 符号扩展 32→64 (SHL 32 + ASR 32)
	if op == LDRSW_IMM {
		t.emit(vm.OpShlImm, rd, rd)
		t.emitU32(32)
		t.emit(vm.OpAsrImm, rd, rd)
		t.emitU32(32)
	}
	// LDRSB: 符号扩展 8→64 (SHL 56 + ASR 56)
	if op == LDRSB_IMM {
		t.emit(vm.OpShlImm, rd, rd)
		t.emitU32(56)
		t.emit(vm.OpAsrImm, rd, rd)
		t.emitU32(56)
	}
	// LDRSH: 符号扩展 16→64 (SHL 48 + ASR 48)
	if op == LDRSH_IMM {
		t.emit(vm.OpShlImm, rd, rd)
		t.emitU32(48)
		t.emit(vm.OpAsrImm, rd, rd)
		t.emitU32(48)
	}
	return nil
}

func (t *Translator) trStore(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	// ARM64: Rt=REG_XZR 在 STR 上下文 = XZR (零寄存器, decoder 已标记)
	rd, err2 := t.mapReg(inst.Rd)
	if err2 != nil {
		return err2
	}
	if inst.Rd == vm.REG_XZR {
		t.emit(vm.OpMovImm32, rd)
		t.emitU32(0)
	}

	op := Op(inst.Op)
	var vmOp byte
	switch op {
	case STRB_IMM:
		vmOp = vm.OpStore8
	case STR_IMM:
		if inst.SF {
			vmOp = vm.OpStore64
		} else {
			vmOp = vm.OpStore32
		}
	case STRH_IMM:
		vmOp = vm.OpStore16
	default:
		vmOp = vm.OpStore64
	}

	emitWriteback := func() {
		wbImm := inst.Imm
		if wbImm >= 0 {
			t.emit(vm.OpAddImm, rn, rn)
		} else {
			t.emit(vm.OpSubImm, rn, rn)
			wbImm = -wbImm
		}
		t.emitU32(uint32(wbImm))
	}

	if inst.WB == 3 {
		// pre-index: 先更新 base, 再以 offset=0 存储
		emitWriteback()
		t.emit(vmOp, rn, rd)
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, 0)
		t.code = append(t.code, b...)
	} else if inst.WB == 1 {
		// post-index: 先以 offset=0 存储, 再更新 base
		t.emit(vmOp, rn, rd)
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, 0)
		t.code = append(t.code, b...)
		emitWriteback()
	} else {
		// unsigned/unscaled offset
		if inst.Imm < 0 {
			// STUR 负偏移: 先计算实际地址到 R16, 再以 offset=0 存储
			tmp := byte(16)
			t.emit(vm.OpSubImm, tmp, rn)
			t.emitU32(uint32(-inst.Imm))
			t.emit(vmOp, tmp, rd)
			b := make([]byte, 2)
			binary.LittleEndian.PutUint16(b, 0)
			t.code = append(t.code, b...)
		} else {
			t.emit(vmOp, rn, rd)
			b := make([]byte, 2)
			binary.LittleEndian.PutUint16(b, uint16(inst.Imm))
			t.code = append(t.code, b...)
		}
	}

	return nil
}

func (t *Translator) trSTP(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rt1, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rt2, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	// STP: Rt/Rt2=XZR(31) → 存零值, mapReg 会映射到 R16
	// 需要先清零 R16
	if inst.Rd == vm.REG_XZR || inst.Rm == vm.REG_XZR {
		t.emit(vm.OpMovImm32, 16) // R16 = 0
		t.emitU32(0)
	}

	vmOp := vm.OpStore64
	stride := int64(8)
	if !inst.SF {
		vmOp = vm.OpStore32
		stride = 4
	}

	if inst.WB == 3 {
		if inst.Imm >= 0 {
			t.emit(vm.OpAddImm, rn, rn)
			t.emitU32(uint32(inst.Imm))
		} else {
			t.emit(vm.OpSubImm, rn, rn)
			t.emitU32(uint32(-inst.Imm))
		}
		t.emit(vmOp, rn, rt1)
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, 0)
		t.code = append(t.code, b...)
		t.emit(vmOp, rn, rt2)
		binary.LittleEndian.PutUint16(b, uint16(stride))
		t.code = append(t.code, b...)
	} else {
		b := make([]byte, 2)
		storeImm := inst.Imm
		if inst.WB == 1 {
			storeImm = 0 // post-index: store to [Rn+0], writeback later
		}
		binary.LittleEndian.PutUint16(b, uint16(storeImm))
		t.emit(vmOp, rn, rt1)
		t.code = append(t.code, b...)
		binary.LittleEndian.PutUint16(b, uint16(storeImm+stride))
		t.emit(vmOp, rn, rt2)
		t.code = append(t.code, b...)
		if inst.WB == 1 {
			if inst.Imm >= 0 {
				t.emit(vm.OpAddImm, rn, rn)
				t.emitU32(uint32(inst.Imm))
			} else {
				t.emit(vm.OpSubImm, rn, rn)
				t.emitU32(uint32(-inst.Imm))
			}
		}
	}

	return nil
}

func (t *Translator) trLDP(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rt1, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rt2, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	vmOp := vm.OpLoad64
	stride := int64(8)
	if !inst.SF {
		vmOp = vm.OpLoad32
		stride = 4
	}

	if inst.WB == 3 {
		if inst.Imm >= 0 {
			t.emit(vm.OpAddImm, rn, rn)
			t.emitU32(uint32(inst.Imm))
		} else {
			t.emit(vm.OpSubImm, rn, rn)
			t.emitU32(uint32(-inst.Imm))
		}
		t.emit(vmOp, rt1, rn)
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, 0)
		t.code = append(t.code, b...)
		t.emit(vmOp, rt2, rn)
		binary.LittleEndian.PutUint16(b, uint16(stride))
		t.code = append(t.code, b...)
	} else {
		b := make([]byte, 2)
		loadImm := inst.Imm
		if inst.WB == 1 {
			loadImm = 0 // post-index: load from [Rn+0], writeback later
		}
		// 当 rt1 == rn 时, 第一个 load 会覆写基地址寄存器
		// ARM64 LDP 是原子操作, 两个 load 共用原始基地址
		// 需要先保存 rn 到临时寄存器
		baseReg := rn
		if rt1 == rn {
			tmp := t.pickTemp(rt1, rt2, rn)
			t.emit(vm.OpMovReg, tmp, rn)
			baseReg = tmp
		}
		binary.LittleEndian.PutUint16(b, uint16(loadImm))
		t.emit(vmOp, rt1, baseReg)
		t.code = append(t.code, b...)
		binary.LittleEndian.PutUint16(b, uint16(loadImm+stride))
		t.emit(vmOp, rt2, baseReg)
		t.code = append(t.code, b...)
		if inst.WB == 1 {
			if inst.Imm >= 0 {
				t.emit(vm.OpAddImm, rn, rn)
				t.emitU32(uint32(inst.Imm))
			} else {
				t.emit(vm.OpSubImm, rn, rn)
				t.emitU32(uint32(-inst.Imm))
			}
		}
	}

	return nil
}

func (t *Translator) trLoadReg(inst vm.Instruction) error {
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

	option := (inst.Raw >> 13) & 7
	s := (inst.Raw >> 12) & 1
	size := (inst.Raw >> 30) & 3

	shift := uint32(0)
	_ = option
	if s == 1 {
		shift = size
	}

	tmp := t.pickTemp(rd, rn, rm)
	if shift > 0 {
		t.emit(vm.OpShlImm, tmp, rm)
		t.emitU32(shift)
		t.emit(vm.OpAdd, tmp, rn, tmp)
	} else {
		t.emit(vm.OpAdd, tmp, rn, rm)
	}

	op := Op(inst.Op)
	var vmOp byte
	switch op {
	case LDRB_REG:
		vmOp = vm.OpLoad8
	case LDRH_REG:
		vmOp = vm.OpLoad16
	default:
		if inst.SF {
			vmOp = vm.OpLoad64
		} else {
			vmOp = vm.OpLoad32
		}
	}

	t.emit(vmOp, rd, tmp)
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 0)
	t.code = append(t.code, b...)
	return nil
}

func (t *Translator) trStoreReg(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rd, err := t.mapReg(inst.Rd) // Rt (source register for store)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	option := (inst.Raw >> 13) & 7
	s := (inst.Raw >> 12) & 1
	size := (inst.Raw >> 30) & 3

	shift := uint32(0)
	_ = option
	if s == 1 {
		shift = size
	}

	tmp := t.pickTemp(rn, rd, rm)
	if shift > 0 {
		t.emit(vm.OpShlImm, tmp, rm)
		t.emitU32(shift)
		t.emit(vm.OpAdd, tmp, rn, tmp)
	} else {
		t.emit(vm.OpAdd, tmp, rn, rm)
	}

	op := Op(inst.Op)
	var vmOp byte
	switch op {
	case STRB_REG:
		vmOp = vm.OpStore8
	case STRH_REG:
		vmOp = vm.OpStore16
	default:
		if inst.SF {
			vmOp = vm.OpStore64
		} else {
			vmOp = vm.OpStore32
		}
	}

	t.emit(vmOp, tmp, rd)
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 0)
	t.code = append(t.code, b...)
	return nil
}

// trLoadRegSigned 翻译 LDRSB/LDRSH/LDRSW (register offset)
// 加载后做符号扩展
func (t *Translator) trLoadRegSigned(inst vm.Instruction) error {
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

	option := (inst.Raw >> 13) & 7
	s := (inst.Raw >> 12) & 1
	size := (inst.Raw >> 30) & 3

	shift := uint32(0)
	_ = option
	if s == 1 {
		shift = size
	}

	tmp := t.pickTemp(rd, rn, rm)
	if shift > 0 {
		t.emit(vm.OpShlImm, tmp, rm)
		t.emitU32(shift)
		t.emit(vm.OpAdd, tmp, rn, tmp)
	} else {
		t.emit(vm.OpAdd, tmp, rn, rm)
	}

	op := Op(inst.Op)
	var vmOp byte
	var shlBits uint32
	switch op {
	case LDRSB_REG:
		vmOp = vm.OpLoad8
		shlBits = 56
	case LDRSH_REG:
		vmOp = vm.OpLoad16
		shlBits = 48
	case LDRSW_REG:
		vmOp = vm.OpLoad32
		shlBits = 32
	default:
		vmOp = vm.OpLoad64
		shlBits = 0
	}

	t.emit(vmOp, rd, tmp)
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 0)
	t.code = append(t.code, b...)

	// 符号扩展
	if shlBits > 0 {
		t.emit(vm.OpShlImm, rd, rd)
		t.emitU32(shlBits)
		t.emit(vm.OpAsrImm, rd, rd)
		t.emitU32(shlBits)
	}
	return nil
}

// trLdrLiteral 翻译 LDR literal (PC-relative) 指令
// ARM64: LDR Xt/Wt, [PC + imm19*4]
// VM:   MOV_IMM64 tmp, abs_addr; LOAD Rd, tmp, 0
//
//	(LDRSW: 再做 SHL+ASR 符号扩展)
func (t *Translator) trLdrLiteral(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	// 计算绝对目标地址:
	// PC = funcAddr + inst.Offset
	// target = PC + imm (imm already = imm19*4 from postLdrLiteral)
	absAddr := t.funcAddr + uint64(inst.Offset) + uint64(inst.Imm)

	// 使用临时寄存器保存地址
	tmp := byte(16) // R16 = XZR/临时寄存器

	// MOV_IMM64 tmp, absAddr
	t.emit(vm.OpMovImm, tmp)
	ab := make([]byte, 8)
	binary.LittleEndian.PutUint64(ab, absAddr)
	t.code = append(t.code, ab...)

	// 选择 LOAD 宽度
	isLDRSW := (inst.WB == 4) // postLdrLiteral 用 WB=4 标记 LDRSW
	var vmOp byte
	if isLDRSW {
		vmOp = vm.OpLoad32 // 先加载 32-bit，后面再符号扩展
	} else if inst.SF {
		vmOp = vm.OpLoad64
	} else {
		vmOp = vm.OpLoad32
	}

	// LOAD Rd, tmp, 0
	t.emit(vmOp, rd, tmp)
	lb := make([]byte, 2)
	binary.LittleEndian.PutUint16(lb, 0) // offset = 0
	t.code = append(t.code, lb...)

	// LDRSW: 32-bit → 64-bit 符号扩展 (SHL rd, rd, 32; ASR rd, rd, 32)
	if isLDRSW {
		t.emit(vm.OpShlImm, rd, rd)
		si := make([]byte, 4)
		binary.LittleEndian.PutUint32(si, 32)
		t.code = append(t.code, si...)

		t.emit(vm.OpAsrImm, rd, rd)
		binary.LittleEndian.PutUint32(si, 32)
		t.code = append(t.code, si...)
	}

	// 32-bit LDR (非 LDRSW): 截断高 32 位
	if !inst.SF && !isLDRSW {
		t.trunc32(rd)
	}

	return nil
}

// trLdar 翻译 LDAR/LDARB/LDARH/LDAXR/LDAXRB/LDAXRH
// 在单线程 VM 中等价于普通 load from [Rn] with offset=0
// inst.Shift = access bytes (1/2/4/8), 由 postAcqRel 设置
func (t *Translator) trLdar(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	var vmOp byte
	switch inst.Shift {
	case 1:
		vmOp = vm.OpLoad8
	case 2:
		vmOp = vm.OpLoad16
	case 4:
		vmOp = vm.OpLoad32
	default:
		vmOp = vm.OpLoad64
	}

	t.emit(vmOp, rd, rn)
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 0)
	t.code = append(t.code, b...)
	return nil
}

// trStlr 翻译 STLR/STLRB/STLRH
// 在单线程 VM 中等价于普通 store to [Rn] with offset=0
func (t *Translator) trStlr(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rd, err := t.mapReg(inst.Rd) // Rt is source
	if err != nil {
		return err
	}
	if inst.Rd == vm.REG_XZR {
		t.emit(vm.OpMovImm32, rd)
		t.emitU32(0)
	}

	var vmOp byte
	switch inst.Shift {
	case 1:
		vmOp = vm.OpStore8
	case 2:
		vmOp = vm.OpStore16
	case 4:
		vmOp = vm.OpStore32
	default:
		vmOp = vm.OpStore64
	}

	t.emit(vmOp, rn, rd)
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 0)
	t.code = append(t.code, b...)
	return nil
}

// trStlxr 翻译 STLXR/STLXRB/STLXRH
// 在单线程 VM 中: store + status register = 0 (always succeed)
// inst.Rm = status register (Ws), inst.Rd = Rt (source), inst.Rn = base
func (t *Translator) trStlxr(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rs, err := t.mapReg(inst.Rm) // status register
	if err != nil {
		return err
	}

	if inst.Rd == vm.REG_XZR {
		t.emit(vm.OpMovImm32, rd)
		t.emitU32(0)
	}

	var vmOp byte
	switch inst.Shift {
	case 1:
		vmOp = vm.OpStore8
	case 2:
		vmOp = vm.OpStore16
	case 4:
		vmOp = vm.OpStore32
	default:
		vmOp = vm.OpStore64
	}

	t.emit(vmOp, rn, rd)
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 0)
	t.code = append(t.code, b...)

	// Status register = 0 (exclusive store always succeeds in single-threaded VM)
	t.emit(vm.OpMovImm32, rs)
	t.emitU32(0)
	return nil
}

// trLdpsw 翻译 LDPSW - Load pair of signed words
// 加载两个 32-bit 值并 sign-extend 到 64-bit
// Rd=Rt1, Rm=Rt2, Rn=base, Imm=offset(已缩放), WB=寻址模式
func (t *Translator) trLdpsw(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rt1, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rt2, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	const stride = int64(4) // 32-bit = 4 bytes

	if inst.WB == 3 { // pre-index
		if inst.Imm >= 0 {
			t.emit(vm.OpAddImm, rn, rn)
			t.emitU32(uint32(inst.Imm))
		} else {
			t.emit(vm.OpSubImm, rn, rn)
			t.emitU32(uint32(-inst.Imm))
		}
		t.emit(vm.OpLoad32, rt1, rn)
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, 0)
		t.code = append(t.code, b...)
		t.emit(vm.OpLoad32, rt2, rn)
		binary.LittleEndian.PutUint16(b, uint16(stride))
		t.code = append(t.code, b...)
	} else {
		loadImm := inst.Imm
		if inst.WB == 1 {
			loadImm = 0 // post-index
		}
		baseReg := rn
		if rt1 == rn {
			tmp := t.pickTemp(rt1, rt2, rn)
			t.emit(vm.OpMovReg, tmp, rn)
			baseReg = tmp
		}
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, uint16(loadImm))
		t.emit(vm.OpLoad32, rt1, baseReg)
		t.code = append(t.code, b...)
		binary.LittleEndian.PutUint16(b, uint16(loadImm+stride))
		t.emit(vm.OpLoad32, rt2, baseReg)
		t.code = append(t.code, b...)
		if inst.WB == 1 { // post-index writeback
			if inst.Imm >= 0 {
				t.emit(vm.OpAddImm, rn, rn)
				t.emitU32(uint32(inst.Imm))
			} else {
				t.emit(vm.OpSubImm, rn, rn)
				t.emitU32(uint32(-inst.Imm))
			}
		}
	}

	// Sign-extend each 32-bit result to 64-bit
	// SHL #32 then ASR #32 实现 sign-extend from bit 31
	t.emit(vm.OpShlImm, rt1, rt1)
	t.emitU32(32)
	t.emit(vm.OpAsrImm, rt1, rt1)
	t.emitU32(32)
	t.emit(vm.OpShlImm, rt2, rt2)
	t.emitU32(32)
	t.emit(vm.OpAsrImm, rt2, rt2)
	t.emitU32(32)
	return nil
}

// trLdadd 翻译 LDADD — 原子加 (单线程简化)
// 语义: old = Mem[Rn]; Mem[Rn] = old + Rs; Rt = old
// Rd=Rt (destination for old value), Rm=Rs (source value), Rn=base
// inst.Shift = access bytes (4 or 8)
func (t *Translator) trLdadd(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rt, err := t.mapReg(inst.Rd) // Rt: receives old value
	if err != nil {
		return err
	}
	rs, err := t.mapReg(inst.Rm) // Rs: addend
	if err != nil {
		return err
	}

	var loadOp, storeOp byte
	if inst.Shift <= 4 {
		loadOp = vm.OpLoad32
		storeOp = vm.OpStore32
	} else {
		loadOp = vm.OpLoad64
		storeOp = vm.OpStore64
	}
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 0)

	// 选择不与 rt/rs/rn 冲突的临时寄存器
	tmp := t.pickTemp(rt, rs, rn)

	// Step 1: old = [Rn] → tmp
	t.emit(loadOp, tmp, rn)
	t.code = append(t.code, b...)

	// 当 rt == rn 时 (如 LDADD X1,X0,[X0])，
	// Step 2 的 ADD 会覆写 base(rn=R0) 为 new value，
	// 导致 Step 3 的 Store 用错误地址。
	// 修复: 用第2个 temp 存 new value, Store 后再赋给 rt
	if rt == rn {
		tmp2, _ := t.pickTemp2(rt, rs, tmp)
		// Step 2: new = old + Rs → tmp2
		t.emit(vm.OpAdd, tmp2, tmp, rs)
		// Step 3: [Rn] = tmp2 (rn 还是原始基地址)
		t.emit(storeOp, rn, tmp2)
		t.code = append(t.code, b...)
		// Step 4: Rt(=Rn) = old (tmp)
		t.emit(vm.OpMovReg, rt, tmp)
	} else {
		// 正常路径: rt != rn, 不会覆写 base
		// Step 2: new = old + Rs → Rt
		t.emit(vm.OpAdd, rt, tmp, rs)
		// Step 3: [Rn] = Rt
		t.emit(storeOp, rn, rt)
		t.code = append(t.code, b...)
		// Step 4: Rt = old (tmp)
		t.emit(vm.OpMovReg, rt, tmp)
	}
	return nil
}

// trCas 翻译 CAS — 比较并交换 (单线程简化)
// 语义: old = Mem[Rn]; if old == Xs then Mem[Rn] = Xt; Xs = old
// Rm=Rs (compare/dest), Rd=Rt (new value), Rn=base
// inst.Shift = access bytes (4 or 8)
func (t *Translator) trCas(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rt, err := t.mapReg(inst.Rd) // Rt: new value to store
	if err != nil {
		return err
	}
	rs, err := t.mapReg(inst.Rm) // Rs: compare value, also receives old
	if err != nil {
		return err
	}

	var loadOp, storeOp byte
	if inst.Shift <= 4 {
		loadOp = vm.OpLoad32
		storeOp = vm.OpStore32
	} else {
		loadOp = vm.OpLoad64
		storeOp = vm.OpStore64
	}
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 0)

	// 单线程: CAS 总是成功的 (没有竞争)
	// 选择不与 rt/rs/rn 冲突的临时寄存器
	tmp := t.pickTemp(rt, rs, rn)

	// Step 1: old = [Rn] → tmp
	t.emit(loadOp, tmp, rn)
	t.code = append(t.code, b...)

	// Step 2: [Rn] = Rt (unconditionally — single threaded simplification)
	t.emit(storeOp, rn, rt)
	t.code = append(t.code, b...)

	// Step 3: Rs = old (tmp)
	t.emit(vm.OpMovReg, rs, tmp)
	return nil
}
