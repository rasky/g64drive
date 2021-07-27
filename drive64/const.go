package drive64

import (
	"errors"
	"fmt"
	"hash/crc32"
)

//go:generate stringer -type=Cmd,Bank,CIC,SaveType,UpgradeStatus -output=const_string.go

// Cmd is the type of a 64drive command send through USB
type Cmd byte

const (
	// CmdLoadFromPc loads a bank of data from PC
	CmdLoadFromPc Cmd = 0x20
	// CmdDumpToPc reads the contents of a bank to the PC
	CmdDumpToPc Cmd = 0x30
	// CmdSetCicType sets the CIC emulation
	CmdSetCicType Cmd = 0x72
	// CmdSetSaveType sets the save emulation
	CmdSetSaveType Cmd = 0x70
	// CmdVersionRequest requests the hardware and firmware version
	CmdVersionRequest Cmd = 0x80
	// CmdUpgradeStart starts a firmware upgrade
	CmdUpgradeStart Cmd = 0x84
	// CmdUpgradeReport returns information on the ongoing firmware upgrade
	CmdUpgradeReport Cmd = 0x85
)

// Variant represent the hardware variant (revision)
type Variant uint16

const (
	// VarRevA is the HW1, RevA board
	VarRevA Variant = 0x4100
	// VarRevB is the HW2, ReVB board
	VarRevB Variant = 0x4200
)

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

// Version is the FPGA configuration revision number (firmware version)
type Version uint16

func (v Version) String() string {
	return fmt.Sprintf("%d.%02d", v/100, v%100)
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

// Save emulation types supported by 64drive
type SaveType uint8

const (
	SaveNone        SaveType = 0
	SaveEeprom4Kb   SaveType = 1
	SaveEeprom16Kb  SaveType = 2
	SaveSRAM256Kb   SaveType = 3
	SaveFlashRAM1Mb SaveType = 4
	SaveSRAM768Kb   SaveType = 5
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

// UpgradeStatus represents the current status of the firmware upgrade
type UpgradeStatus uint8

// These are the possible upgrade status that can occur during a firwmare upgrade.
// To read the current upgrade status, use Device.CmdUpgradeReport.
const (
	UpgradeReset     UpgradeStatus = 0x0
	UpgradeReady     UpgradeStatus = 0x1
	UpgradeVerifying UpgradeStatus = 0x2
	UpgradeErasing00 UpgradeStatus = 0x3
	UpgradeErasing25 UpgradeStatus = 0x4
	UpgradeErasing50 UpgradeStatus = 0x5
	UpgradeErasing75 UpgradeStatus = 0x6
	UpgradeWriting00 UpgradeStatus = 0x7
	UpgradeWriting25 UpgradeStatus = 0x8
	UpgradeWriting50 UpgradeStatus = 0x9
	UpgradeWriting75 UpgradeStatus = 0xA

	UpgradeSuccess     UpgradeStatus = 0xC
	UpgradeGeneralFail UpgradeStatus = 0xD
	UpgradeBadVariant  UpgradeStatus = 0xE
	UpgradeVerifyFail  UpgradeStatus = 0xF
)

// IsFinished returns true if the UpgradeStatus represents the end of the upgrade process.
func (stat UpgradeStatus) IsFinished() bool {
	switch stat {
	case UpgradeSuccess, UpgradeGeneralFail, UpgradeBadVariant, UpgradeVerifyFail:
		return true
	default:
		return false
	}
}
