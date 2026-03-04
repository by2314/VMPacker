package vm

// ============================================================
// VM 字节码操作码（随机映射值）
//
// 逆向者在 IDA 中只能看到这些 hex 值对应的 switch-case，
// 无法直接识别指令含义。
// ============================================================

// VM 配置常量
const (
	RegCount   = 32  // 虚拟寄存器数量 (X0-X30 + XZR)
	StackSize  = 128 // 虚拟栈深度
	MaxExtFunc = 16  // 最大外部函数数
)

const (
	// 数据操作
	OpNop      byte = 0xC3 // 空操作                1B: [op]
	OpMovImm   byte = 0x5A // Rx = imm64            10B: [op][r][imm64_LE]
	OpMovImm32 byte = 0x49 // Rx = imm32 (零扩展)   6B: [op][r][imm32_LE]
	OpMovReg   byte = 0x2F // Rx = Ry               3B: [op][dst][src]
	OpLoad8    byte = 0x91 // Rx = *(u8*)(Ry+imm16) 5B: [op][dst][base][imm16]
	OpLoad32   byte = 0xA4 // Rx = *(u32*)(Ry+i16)  5B: [op][dst][base][imm16]
	OpLoad64   byte = 0xB7 // Rx = *(u64*)(Ry+i16)  5B: [op][dst][base][imm16]
	OpStore8   byte = 0xD2 // *(u8*)(Rx+i16) = Ry   5B: [op][base][src][imm16]
	OpStore32  byte = 0x19 // *(u32*)(Rx+i16) = Ry  5B: [op][base][src][imm16]
	OpStore64  byte = 0x2A // *(u64*)(Rx+i16) = Ry  5B: [op][base][src][imm16]
	OpLoad16   byte = 0xE7 // Rx = *(u16*)(Ry+i16)  5B: [op][dst][base][imm16]
	OpStore16  byte = 0xE8 // *(u16*)(Rx+i16) = Ry  5B: [op][base][src][imm16]

	// 算术运算 — 三地址: [op][d][a][b] = 4B
	OpAdd   byte = 0x37
	OpSub   byte = 0x6E
	OpMul   byte = 0x83
	OpXor   byte = 0x1B
	OpAnd   byte = 0x4D
	OpOr    byte = 0x72
	OpShl   byte = 0xAE
	OpShr   byte = 0xF1 // 逻辑右移
	OpAsr   byte = 0xDA // 算术右移
	OpUmulh byte = 0xF2 // UMULH Xd,Xn,Xm  4B: [op][d][n][m]
	OpNot   byte = 0x08 // NOT Rx, Ry — 3B
	OpRor   byte = 0x3D // 循环右移

	// 立即数算术: [op][d][s][imm32] = 7B
	OpAddImm byte = 0xE5
	OpSubImm byte = 0x78
	OpXorImm byte = 0x3C
	OpAndImm byte = 0xD9
	OpOrImm  byte = 0x6B
	OpMulImm byte = 0xB3
	OpShlImm byte = 0x7A
	OpShrImm byte = 0x8C
	OpAsrImm byte = 0x9D

	// 比较
	OpCmp    byte = 0x9F // CMP Rx, Ry          3B
	OpCmpImm byte = 0xA1 // CMP Rx, imm32       6B

	// 跳转
	OpJmp byte = 0x44 // JMP imm32           5B
	OpJe  byte = 0x58 // JE  imm32 (ZF=1)    5B
	OpJne byte = 0xBB // JNE imm32 (ZF=0)    5B
	OpJl  byte = 0x15 // JL  imm32 (SF=1)    5B (有符号小于)
	OpJge byte = 0x29 // JGE imm32 (SF=0)    5B (有符号大于等于)
	OpJgt byte = 0x36 // JGT imm32           5B
	OpJle byte = 0x47 // JLE imm32           5B
	// 无符号比较跳转
	OpJb  byte = 0x52 // JB  (无符号小于, CF)
	OpJae byte = 0x64 // JAE (无符号大于等于)
	OpJbe byte = 0x53 // JBE (无符号小于等于, CF||ZF)  B.LS
	OpJa  byte = 0x65 // JA  (无符号大于, !CF&&!ZF)    B.HI

	// 栈操作
	OpPush byte = 0x63 // PUSH Rx             2B
	OpPop  byte = 0x27 // POP  Rx             2B

	// 特殊
	OpCallNative byte = 0xAB // 调用原生函数地址    9B: [op][imm64] (BL到绝对地址)
	OpCallReg    byte = 0xBC // BLR Xn 寄存器间接调用 2B: [op][rn]
	OpBrReg      byte = 0xCD // BR  Xn 寄存器间接跳转 2B: [op][rn]
	OpRet        byte = 0xEE // RET Rx             2B
	OpHalt       byte = 0x00 // 停机               1B

	// SIMD 加载/存储: [op][rn][len] = 3B
	OpVld16 byte = 0xC1 // vtmp ← mem[R[rn]], len bytes
	OpVst16 byte = 0xC2 // mem[R[rn]] ← vtmp, len bytes

	// TBZ/TBNZ: [op][reg][bit][target32] = 7B
	OpTbz  byte = 0x16 // TBZ  Xt, #bit, target
	OpTbnz byte = 0x17 // TBNZ Xt, #bit, target

	// CCMP/CCMN: [op][cond][nzcv][rn][rm_or_imm5][sf] = 6B
	OpCcmpReg byte = 0x18 // CCMP Xn, Xm, #nzcv, cond
	OpCcmpImm byte = 0x1A // CCMP Xn, #imm5, #nzcv, cond
	OpCcmnReg byte = 0x1C // CCMN Xn, Xm, #nzcv, cond
	OpCcmnImm byte = 0x1D // CCMN Xn, #imm5, #nzcv, cond

	// SVC: [op][imm16_lo][imm16_hi] = 3B
	OpSvc byte = 0x1E // SVC #imm16

	// UDIV/SDIV: [op][d][n][m] = 4B (和 MUL 格式一样)
	OpUdiv byte = 0x1F // UDIV Xd, Xn, Xm
	OpSdiv byte = 0x21 // SDIV Xd, Xn, Xm

	// MRS: [op][d][sysreg_lo][sysreg_hi] = 4B
	OpMrs byte = 0x20 // MRS Xd, <sysreg>

	// SMULH/CLZ/CLS/RBIT/REV/REV16/REV32
	OpSmulh byte = 0x22 // SMULH Xd, Xn, Xm         4B: [op][d][n][m]
	OpClz   byte = 0x23 // CLZ Xd, Xn               3B: [op][d][n]
	OpCls   byte = 0x24 // CLS Xd, Xn               3B: [op][d][n]
	OpRbit  byte = 0x25 // RBIT Xd, Xn              3B: [op][d][n]
	OpRev   byte = 0x26 // REV Xd, Xn               3B: [op][d][n]
	OpRev16 byte = 0x28 // REV16 Xd, Xn             3B: [op][d][n]
	OpRev32 byte = 0x2B // REV32 Xd, Xn             3B: [op][d][n]

	// ADC/SBC (带进位加减)
	OpAdc byte = 0x2C // ADC Xd, Xn, Xm            4B: [op][d][n][m]
	OpSbc byte = 0x2D // SBC Xd, Xn, Xm            4B: [op][d][n][m]
)

// 标志位
const (
	FlagZero  uint32 = 1 << 0
	FlagSign  uint32 = 1 << 1
	FlagCarry uint32 = 1 << 2
)
