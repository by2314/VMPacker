/*
 * vm_dispatch.h — 间接 Dispatch 跳转表
 *
 * 当 VM_INDIRECT_DISPATCH 宏定义时启用。
 * 使用绝对函数指针数组，运行时在栈上填充，
 * 通过间接调用 handler，断裂 IDA 交叉引用。
 *
 * 核心机制:
 *   jump_table[opcode] = (vm_handler_fn)handler
 *   handler = jump_table[opcode]; handler(vm);
 *
 * 注意: 跳转表必须在栈上分配 — stub 是 RX-only flat binary,
 * BSS/data 段不可写。
 */
#ifndef VM_DISPATCH_H
#define VM_DISPATCH_H

#ifdef VM_INDIRECT_DISPATCH

#include "vm_opcodes.h"
#include "vm_sections.h"
#include "vm_types.h"

/* Handler 函数签名: 接收 vm_ctx_t*, 返回指令步进字节数 */
typedef u32 (*vm_handler_fn)(vm_ctx_t *vm);

/* HALT 哨兵值: handler 返回此值表示需要退出 VM */
#define VM_STEP_HALT 0xFFFFFFFFu

/* RET 哨兵值: handler 返回此值表示 RET 指令 */
#define VM_STEP_RET 0xFFFFFFFEu

/* ================================================================
 * Handler 包装函数
 *
 * 现有 handler 是 static inline，编译器可能内联。
 * 包装函数使用 noinline 确保生成独立函数体，
 * 使间接调用机制真正生效。
 * ================================================================ */

/* ---- 系统 ---- */
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_halt(vm_ctx_t *vm) {
  (void)vm;
  return VM_STEP_HALT;
}

__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_ret(vm_ctx_t *vm) {
  u8 r = vm->bc[vm->pc + 1];
  vm->R[0] = vm->R[r & 31];
  return VM_STEP_RET;
}

__attribute__((noinline)) VM_SECTION_SYSTEM static u32
hw_unknown(vm_ctx_t *vm) {
  (void)vm;
  return VM_STEP_HALT;
}

/* ---- NOP ---- */
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_nop(vm_ctx_t *vm) {
  return h_nop(vm);
}

/* ---- 数据移动 ---- */
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_mov_imm(vm_ctx_t *vm) {
  return h_mov_imm(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_mov_imm32(vm_ctx_t *vm) {
  return h_mov_imm32(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_mov_reg(vm_ctx_t *vm) {
  return h_mov_reg(vm);
}

/* ---- 内存 ---- */
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_load8(vm_ctx_t *vm) {
  return h_load8(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_load32(vm_ctx_t *vm) {
  return h_load32(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_load64(vm_ctx_t *vm) {
  return h_load64(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_store8(vm_ctx_t *vm) {
  return h_store8(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_store32(vm_ctx_t *vm) {
  return h_store32(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_store64(vm_ctx_t *vm) {
  return h_store64(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_load16(vm_ctx_t *vm) {
  return h_load16(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_store16(vm_ctx_t *vm) {
  return h_store16(vm);
}

/* ---- ALU 三寄存器 ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_add(vm_ctx_t *vm) {
  return h_add(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_sub(vm_ctx_t *vm) {
  return h_sub(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_mul(vm_ctx_t *vm) {
  return h_mul(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_xor(vm_ctx_t *vm) {
  return h_xor(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_and(vm_ctx_t *vm) {
  return h_and(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_or(vm_ctx_t *vm) {
  return h_or(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_shl(vm_ctx_t *vm) {
  return h_shl(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_shr(vm_ctx_t *vm) {
  return h_shr(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_asr(vm_ctx_t *vm) {
  return h_asr(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_not(vm_ctx_t *vm) {
  return h_not(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_ror(vm_ctx_t *vm) {
  return h_ror(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_umulh(vm_ctx_t *vm) {
  return h_umulh(vm);
}

/* ---- ALU 立即数 ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_add_imm(vm_ctx_t *vm) {
  return h_add_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_sub_imm(vm_ctx_t *vm) {
  return h_sub_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_xor_imm(vm_ctx_t *vm) {
  return h_xor_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_and_imm(vm_ctx_t *vm) {
  return h_and_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_or_imm(vm_ctx_t *vm) {
  return h_or_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_mul_imm(vm_ctx_t *vm) {
  return h_mul_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_shl_imm(vm_ctx_t *vm) {
  return h_shl_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_shr_imm(vm_ctx_t *vm) {
  return h_shr_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_asr_imm(vm_ctx_t *vm) {
  return h_asr_imm(vm);
}

/* ---- 比较 ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_cmp(vm_ctx_t *vm) {
  return h_cmp(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_cmp_imm(vm_ctx_t *vm) {
  return h_cmp_imm(vm);
}

/* ---- 分支 (返回 0 表示 pc 已设置) ---- */
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jmp(vm_ctx_t *vm) {
  h_jmp(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_je(vm_ctx_t *vm) {
  h_je(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jne(vm_ctx_t *vm) {
  h_jne(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jl(vm_ctx_t *vm) {
  h_jl(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jge(vm_ctx_t *vm) {
  h_jge(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jgt(vm_ctx_t *vm) {
  h_jgt(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jle(vm_ctx_t *vm) {
  h_jle(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jb(vm_ctx_t *vm) {
  h_jb(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jae(vm_ctx_t *vm) {
  h_jae(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jbe(vm_ctx_t *vm) {
  h_jbe(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_ja(vm_ctx_t *vm) {
  h_ja(vm);
  return 0;
}

/* ---- 栈操作 ---- */
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_push(vm_ctx_t *vm) {
  return h_push(vm);
}
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_pop(vm_ctx_t *vm) {
  return h_pop(vm);
}

/* ---- 原生调用 ---- */
__attribute__((noinline)) VM_SECTION_SYSTEM static u32
hw_call_nat(vm_ctx_t *vm) {
  return h_call_nat(vm);
}
__attribute__((noinline)) VM_SECTION_SYSTEM static u32
hw_call_reg(vm_ctx_t *vm) {
  return h_call_reg(vm);
}
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_br_reg(vm_ctx_t *vm) {
  return h_br_reg(vm);
}

/* ---- SIMD ---- */
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_vld16(vm_ctx_t *vm) {
  return h_vld16(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_vst16(vm_ctx_t *vm) {
  return h_vst16(vm);
}

/* ---- TBZ/TBNZ (分支, 返回 0) ---- */
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_tbz(vm_ctx_t *vm) {
  h_tbz(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_tbnz(vm_ctx_t *vm) {
  h_tbnz(vm);
  return 0;
}

/* ---- CCMP/CCMN ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_ccmp_reg(vm_ctx_t *vm) {
  return h_ccmp_reg(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_ccmp_imm(vm_ctx_t *vm) {
  return h_ccmp_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_ccmn_reg(vm_ctx_t *vm) {
  return h_ccmn_reg(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_ccmn_imm(vm_ctx_t *vm) {
  return h_ccmn_imm(vm);
}

/* ---- SVC ---- */
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_svc(vm_ctx_t *vm) {
  return h_svc(vm);
}

/* ---- UDIV/SDIV ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_udiv(vm_ctx_t *vm) {
  return h_udiv(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_sdiv(vm_ctx_t *vm) {
  return h_sdiv(vm);
}

/* ---- MRS ---- */
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_mrs(vm_ctx_t *vm) {
  return h_mrs(vm);
}

/* ---- SMULH/CLZ/CLS/RBIT/REV ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_smulh(vm_ctx_t *vm) {
  return h_smulh(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_clz(vm_ctx_t *vm) {
  return h_clz(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_cls(vm_ctx_t *vm) {
  return h_cls(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_rbit(vm_ctx_t *vm) {
  return h_rbit(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_rev(vm_ctx_t *vm) {
  return h_rev(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_rev16(vm_ctx_t *vm) {
  return h_rev16(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_rev32(vm_ctx_t *vm) {
  return h_rev32(vm);
}

/* ---- ADC/SBC ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_adc(vm_ctx_t *vm) {
  return h_adc(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_sbc(vm_ctx_t *vm) {
  return h_sbc(vm);
}

/* ================================================================
 * 跳转表运行时初始化 (绝对函数指针)
 *
 * 使用循环填充默认值，避免 GCC 范围初始化器
 * 在 -nostdlib 下生成隐式 memset/memcpy 调用。
 *
 * tbl: 调用方栈上分配的 vm_handler_fn[256] 数组
 * ================================================================ */
__attribute__((noinline)) static void vm_init_jump_table(vm_handler_fn *tbl) {
  for (int i = 0; i < 256; i++)
    tbl[i] = hw_unknown;

  /* 系统 */
  tbl[OP_NOP] = hw_nop;
  tbl[OP_HALT] = hw_halt;
  tbl[OP_RET] = hw_ret;

  /* 数据移动 */
  tbl[OP_MOV_IMM] = hw_mov_imm;
  tbl[OP_MOV_IMM32] = hw_mov_imm32;
  tbl[OP_MOV_REG] = hw_mov_reg;

  /* 内存 */
  tbl[OP_LOAD8] = hw_load8;
  tbl[OP_LOAD32] = hw_load32;
  tbl[OP_LOAD64] = hw_load64;
  tbl[OP_STORE8] = hw_store8;
  tbl[OP_STORE32] = hw_store32;
  tbl[OP_STORE64] = hw_store64;
  tbl[OP_LOAD16] = hw_load16;
  tbl[OP_STORE16] = hw_store16;

  /* ALU 三寄存器 */
  tbl[OP_ADD] = hw_add;
  tbl[OP_SUB] = hw_sub;
  tbl[OP_MUL] = hw_mul;
  tbl[OP_XOR] = hw_xor;
  tbl[OP_AND] = hw_and;
  tbl[OP_OR] = hw_or;
  tbl[OP_SHL] = hw_shl;
  tbl[OP_SHR] = hw_shr;
  tbl[OP_ASR] = hw_asr;
  tbl[OP_NOT] = hw_not;
  tbl[OP_ROR] = hw_ror;
  tbl[OP_UMULH] = hw_umulh;

  /* ALU 立即数 */
  tbl[OP_ADD_IMM] = hw_add_imm;
  tbl[OP_SUB_IMM] = hw_sub_imm;
  tbl[OP_XOR_IMM] = hw_xor_imm;
  tbl[OP_AND_IMM] = hw_and_imm;
  tbl[OP_OR_IMM] = hw_or_imm;
  tbl[OP_MUL_IMM] = hw_mul_imm;
  tbl[OP_SHL_IMM] = hw_shl_imm;
  tbl[OP_SHR_IMM] = hw_shr_imm;
  tbl[OP_ASR_IMM] = hw_asr_imm;

  /* 比较 */
  tbl[OP_CMP] = hw_cmp;
  tbl[OP_CMP_IMM] = hw_cmp_imm;

  /* 分支 */
  tbl[OP_JMP] = hw_jmp;
  tbl[OP_JE] = hw_je;
  tbl[OP_JNE] = hw_jne;
  tbl[OP_JL] = hw_jl;
  tbl[OP_JGE] = hw_jge;
  tbl[OP_JGT] = hw_jgt;
  tbl[OP_JLE] = hw_jle;
  tbl[OP_JB] = hw_jb;
  tbl[OP_JAE] = hw_jae;
  tbl[OP_JBE] = hw_jbe;
  tbl[OP_JA] = hw_ja;

  /* 栈操作 */
  tbl[OP_PUSH] = hw_push;
  tbl[OP_POP] = hw_pop;

  /* 原生调用 */
  tbl[OP_CALL_NAT] = hw_call_nat;
  tbl[OP_CALL_REG] = hw_call_reg;
  tbl[OP_BR_REG] = hw_br_reg;

  /* SIMD */
  tbl[OP_VLD16] = hw_vld16;
  tbl[OP_VST16] = hw_vst16;

  /* TBZ/TBNZ */
  tbl[OP_TBZ] = hw_tbz;
  tbl[OP_TBNZ] = hw_tbnz;

  /* CCMP/CCMN */
  tbl[OP_CCMP_REG] = hw_ccmp_reg;
  tbl[OP_CCMP_IMM] = hw_ccmp_imm;
  tbl[OP_CCMN_REG] = hw_ccmn_reg;
  tbl[OP_CCMN_IMM] = hw_ccmn_imm;

  /* SVC */
  tbl[OP_SVC] = hw_svc;

  /* UDIV/SDIV */
  tbl[OP_UDIV] = hw_udiv;
  tbl[OP_SDIV] = hw_sdiv;

  /* MRS */
  tbl[OP_MRS] = hw_mrs;

  /* SMULH/CLZ/CLS/RBIT/REV */
  tbl[OP_SMULH] = hw_smulh;
  tbl[OP_CLZ] = hw_clz;
  tbl[OP_CLS] = hw_cls;
  tbl[OP_RBIT] = hw_rbit;
  tbl[OP_REV] = hw_rev;
  tbl[OP_REV16] = hw_rev16;
  tbl[OP_REV32] = hw_rev32;

  /* ADC/SBC */
  tbl[OP_ADC] = hw_adc;
  tbl[OP_SBC] = hw_sbc;
}

#endif /* VM_INDIRECT_DISPATCH */
#endif /* VM_DISPATCH_H */
