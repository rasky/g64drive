package drive64

import "fmt"

//go:generate stringer -type=Cmd,Bank -output=const_string.go

// Cmd is the type of a 64drive command send through USB
type Cmd byte

const (
	// CmdLoadFromPc loads a bank of data from PC
	CmdLoadFromPc Cmd = 0x20
	// CmdDumpToPc reads the contents of a bank to the PC
	CmdDumpToPc Cmd = 0x30
	// CmdVersionRequest request the hardware and firmware version
	CmdVersionRequest Cmd = 0x80
)

// Variant represent the hardware variant (revision)
type Variant uint16

const (
	// VarRevA is the HW1, RevA board
	VarRevA Variant = 0x4100
	// VarRevB is the HW2, ReVB board
	VarRevB Variant = 0x4200
)

// Version is the FPGA configuration revision number (firmware version)
type Version uint16

func (v Version) String() string {
	return fmt.Sprintf("%d.%02d", v/100, v%100)
}

func (v Variant) String() string {
	switch v {
	case VarRevA:
		return "HW1 (Rev A)"
	case VarRevB:
		return "HW2 (Rev B)"
	default:
		return fmt.Sprintf("UNKVAR (%02x)", uint16(v))
	}
}

// Bank represents a Nintendo 64 memory bank
type Bank uint8

// Predefined Nintendo64 banks, which can be used as a target for
// a memory download or upload operation.
const (
	BankCARTROM Bank = 1
	BankSRAM256 Bank = 2
	BankSRAM768 Bank = 3
	BankFLASH   Bank = 4
	BankPOKEMON Bank = 5
	BankEEPROM  Bank = 6
)
