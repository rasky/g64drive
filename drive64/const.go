package drive64

import (
	"errors"
	"fmt"
	"hash/crc32"
)

//go:generate stringer -type=Cmd,Bank,CIC -output=const_string.go

// Cmd is the type of a 64drive command send through USB
type Cmd byte

const (
	// CmdLoadFromPc loads a bank of data from PC
	CmdLoadFromPc Cmd = 0x20
	// CmdDumpToPc reads the contents of a bank to the PC
	CmdDumpToPc Cmd = 0x30
	// CmdSetCicType sets the CIC emulation
	CmdSetCicType Cmd = 0x72
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

// CIC is the Nintendo 64 protection chip. This type represents on the
// several versions that were produced.
type CIC uint8

// Predefined Nintendo 64 CIC types, which 64drive can emulate.
const (
	CIC6101 CIC = 0
	CIC6102 CIC = 1
	CIC7101 CIC = 2
	CIC7102 CIC = 3
	CICX103 CIC = 4
	CICX105 CIC = 5
	CICX106 CIC = 6
	CIC5101 CIC = 7
)

// NewCICFromString parses a string representing the CIC name (eg. "6103") and
// returns the corresponding CIC value, or an error if the string doesn't match
// any known CIC variant.
func NewCICFromString(name string) (CIC, error) {
	switch name {
	case "6101":
		return CIC6101, nil
	case "6102":
		return CIC6101, nil
	case "7101":
		return CIC7101, nil
	case "7102":
		return CIC7101, nil
	case "6103", "7103", "X103", "x103":
		return CICX103, nil
	case "6105", "7105", "X105", "x105":
		return CICX105, nil
	case "6106", "7106", "X106", "x106":
		return CICX106, nil
	case "5101":
		return CIC5101, nil
	default:
		return 0, errors.New("invalid CIC variant")
	}
}

// NewCICFromHeader detects a CIC variant from a ROM header
func NewCICFromHeader(header []uint8) (CIC, error) {
	crc := crc32.ChecksumIEEE(header[0x40:0x1000])
	switch crc {
	case 0x6170A4A1:
		return CIC6101, nil
	case 0x90BB6CB5:
		return CIC6102, nil
	case 0x0B050EE0:
		return CICX103, nil
	case 0x98BC2C86:
		return CICX105, nil
	case 0xACC8580A:
		return CICX106, nil
	default:
		return 0, fmt.Errorf("cannot detect CIC from ROM header (%08x)", crc)
	}
}
