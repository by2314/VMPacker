package arm64

import (
	"fmt"

	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// 位域翻译 — UBFM / SBFM / EXTR
// ============================================================

func (t *Translator) trUBFM(inst vm.Instruction) error {
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

	regSize := uint32(32)
	if inst.SF {
		regSize = 64
	}

	switch {
	case imms == regSize-1:
		t.emit(vm.OpShrImm, rd, rn)
		t.emitU32(immr)
	case imms+1 == immr:
		t.emit(vm.OpShlImm, rd, rn)
		t.emitU32(regSize - immr)
	case imms == 7 && immr == 0:
		t.emit(vm.OpAndImm, rd, rn)
		t.emitU32(0xFF)
	case imms == 15 && immr == 0:
		t.emit(vm.OpAndImm, rd, rn)
		t.emitU32(0xFFFF)
	default:
		width := imms + 1
		if imms >= immr {
			t.emit(vm.OpShrImm, rd, rn)
			t.emitU32(immr)
			if width >= 32 {
				mask64 := uint64((1 << width) - 1)
				t.emit(vm.OpMovImm, 15)
				t.emitU64(mask64)
				t.emit(vm.OpAnd, rd, rd, 15)
			} else {
				mask := uint32((1 << width) - 1)
				t.emit(vm.OpAndImm, rd, rd)
				t.emitU32(mask)
			}
		} else {
			// UBFIZ: (Rn & mask) << shift
			shift := regSize - immr
			if width >= 32 {
				mask64 := uint64((1 << width) - 1)
				t.emit(vm.OpMovImm, 15)
				t.emitU64(mask64)
				t.emit(vm.OpAnd, rd, rn, 15)
			} else {
				mask := uint32((1 << width) - 1)
				t.emit(vm.OpAndImm, rd, rn)
				t.emitU32(mask)
			}
			t.emit(vm.OpShlImm, rd, rd)
			t.emitU32(shift)
		}
	}
	if !inst.SF {
		t.trunc32(rd)
	}
	return nil
}

func (t *Translator) trSBFM(inst vm.Instruction) error {
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

	regSize := uint32(32)
	if inst.SF {
		regSize = 64
	}

	if imms == regSize-1 {
		// ASR: 对于32-bit，先trunc32确保高32位为0，再用64-bit ASR
		if !inst.SF {
			// 先将源值符号扩展到64位：SHL 32, ASR 32 使bit31扩展到bit63
			t.emit(vm.OpShlImm, rd, rn)
			t.emitU32(32)
			t.emit(vm.OpAsrImm, rd, rd)
			t.emitU32(32 + immr)
			t.trunc32(rd)
		} else {
			t.emit(vm.OpAsrImm, rd, rn)
			t.emitU32(immr)
		}
		return nil
	}
	if immr == 0 {
		// SXTB/SXTH/SXTW: 符号扩展
		// VM寄存器是64-bit，所以需要用64-bit的shift宽度来做sign extension
		var shiftAmt uint32
		if inst.SF {
			shiftAmt = 64 - (imms + 1)
		} else {
			// 32-bit: 先SHL到bit63位置，再ASR回来，最后trunc32
			shiftAmt = 64 - (imms + 1)
		}
		t.emit(vm.OpShlImm, rd, rn)
		t.emitU32(shiftAmt)
		t.emit(vm.OpAsrImm, rd, rd)
		t.emitU32(shiftAmt)
		if !inst.SF {
			t.trunc32(rd)
		}
		return nil
	}
	return fmt.Errorf("复杂 SBFM (immr=%d, imms=%d) 暂不支持", immr, imms)
}

func (t *Translator) trEXTR(inst vm.Instruction) error {
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
	lsb := uint32(inst.Imm)
	regSize := uint32(32)
	if inst.SF {
		regSize = 64
	}

	if inst.Rn == inst.Rm {
		// ROR alias: 直接用 VM 的 OpRor handler
		// OpRor 需要 shift 在寄存器中: Rd = (Rn >> Rm) | (Rn << (64-Rm))
		tmp := t.pickTemp(rd, rn)
		t.emit(vm.OpMovImm32, tmp)
		t.emitU32(lsb)
		t.emit(vm.OpRor, rd, rn, tmp)
	} else {
		// EXTR: result = (Rm >> lsb) | (Rn << (regSize-lsb))
		// 策略: 先 MovReg 备份 Rn 到 tmp，再 ShrImm rd=Rm>>lsb，
		//        再 ShlImm tmp=tmp<<shift，最后 Or
		tmp := t.pickTemp(rd, rn, rm)
		t.emit(vm.OpMovReg, tmp, rn)
		t.emit(vm.OpShrImm, rd, rm)
		t.emitU32(lsb)
		t.emit(vm.OpShlImm, tmp, tmp)
		t.emitU32(regSize - lsb)
		t.emit(vm.OpOr, rd, rd, tmp)
	}
	if !inst.SF {
		t.trunc32(rd)
	}
	return nil
}
