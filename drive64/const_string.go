// Code generated by "stringer -type=Cmd,Bank,CIC,SaveType,UpgradeStatus -output=const_string.go"; DO NOT EDIT.

package drive64

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[CmdLoadFromPc-32]
	_ = x[CmdDumpToPc-48]
	_ = x[CmdSetCicType-114]
	_ = x[CmdSetSaveType-112]
	_ = x[CmdSetExtended-116]
	_ = x[CmdVersionRequest-128]
	_ = x[CmdUpgradeStart-132]
	_ = x[CmdUpgradeReport-133]
}

const (
	_Cmd_name_0 = "CmdLoadFromPc"
	_Cmd_name_1 = "CmdDumpToPc"
	_Cmd_name_2 = "CmdSetSaveType"
	_Cmd_name_3 = "CmdSetCicType"
	_Cmd_name_4 = "CmdSetExtended"
	_Cmd_name_5 = "CmdVersionRequest"
	_Cmd_name_6 = "CmdUpgradeStartCmdUpgradeReport"
)

var (
	_Cmd_index_6 = [...]uint8{0, 15, 31}
)

func (i Cmd) String() string {
	switch {
	case i == 32:
		return _Cmd_name_0
	case i == 48:
		return _Cmd_name_1
	case i == 112:
		return _Cmd_name_2
	case i == 114:
		return _Cmd_name_3
	case i == 116:
		return _Cmd_name_4
	case i == 128:
		return _Cmd_name_5
	case 132 <= i && i <= 133:
		i -= 132
		return _Cmd_name_6[_Cmd_index_6[i]:_Cmd_index_6[i+1]]
	default:
		return "Cmd(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[BankCARTROM-1]
	_ = x[BankSRAM256-2]
	_ = x[BankSRAM768-3]
	_ = x[BankFLASH-4]
	_ = x[BankFLASH_POKSTAD2-5]
	_ = x[BankEEPROM-6]
}

const _Bank_name = "BankCARTROMBankSRAM256BankSRAM768BankFLASHBankFLASH_POKSTAD2BankEEPROM"

var _Bank_index = [...]uint8{0, 11, 22, 33, 42, 60, 70}

func (i Bank) String() string {
	i -= 1
	if i >= Bank(len(_Bank_index)-1) {
		return "Bank(" + strconv.FormatInt(int64(i+1), 10) + ")"
	}
	return _Bank_name[_Bank_index[i]:_Bank_index[i+1]]
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[CIC6101-0]
	_ = x[CIC6102-1]
	_ = x[CIC7101-2]
	_ = x[CIC7102-3]
	_ = x[CICX103-4]
	_ = x[CICX105-5]
	_ = x[CICX106-6]
	_ = x[CIC5101-7]
	_ = x[CIC8303-8]
	_ = x[CIC8401-9]
	_ = x[CIC5167-10]
	_ = x[CICDDUS-11]
}

const _CIC_name = "CIC6101CIC6102CIC7101CIC7102CICX103CICX105CICX106CIC5101CIC8303CIC8401CIC5167CICDDUS"

var _CIC_index = [...]uint8{0, 7, 14, 21, 28, 35, 42, 49, 56, 63, 70, 77, 84}

func (i CIC) String() string {
	if i >= CIC(len(_CIC_index)-1) {
		return "CIC(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _CIC_name[_CIC_index[i]:_CIC_index[i+1]]
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[SaveNone-0]
	_ = x[SaveEeprom4Kbit-1]
	_ = x[SaveEeprom16Kbit-2]
	_ = x[SaveSRAM256Kbit-3]
	_ = x[SaveFlashRAM1Mbit-4]
	_ = x[SaveSRAM768Kbit-5]
	_ = x[SaveFlashRAM1Mbit_PokStad2-6]
}

const _SaveType_name = "SaveNoneSaveEeprom4KbitSaveEeprom16KbitSaveSRAM256KbitSaveFlashRAM1MbitSaveSRAM768KbitSaveFlashRAM1Mbit_PokStad2"

var _SaveType_index = [...]uint8{0, 8, 23, 39, 54, 71, 86, 112}

func (i SaveType) String() string {
	if i >= SaveType(len(_SaveType_index)-1) {
		return "SaveType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _SaveType_name[_SaveType_index[i]:_SaveType_index[i+1]]
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[UpgradeReset-0]
	_ = x[UpgradeReady-1]
	_ = x[UpgradeVerifying-2]
	_ = x[UpgradeErasing00-3]
	_ = x[UpgradeErasing25-4]
	_ = x[UpgradeErasing50-5]
	_ = x[UpgradeErasing75-6]
	_ = x[UpgradeWriting00-7]
	_ = x[UpgradeWriting25-8]
	_ = x[UpgradeWriting50-9]
	_ = x[UpgradeWriting75-10]
	_ = x[UpgradeSuccess-12]
	_ = x[UpgradeGeneralFail-13]
	_ = x[UpgradeBadVariant-14]
	_ = x[UpgradeVerifyFail-15]
}

const (
	_UpgradeStatus_name_0 = "UpgradeResetUpgradeReadyUpgradeVerifyingUpgradeErasing00UpgradeErasing25UpgradeErasing50UpgradeErasing75UpgradeWriting00UpgradeWriting25UpgradeWriting50UpgradeWriting75"
	_UpgradeStatus_name_1 = "UpgradeSuccessUpgradeGeneralFailUpgradeBadVariantUpgradeVerifyFail"
)

var (
	_UpgradeStatus_index_0 = [...]uint8{0, 12, 24, 40, 56, 72, 88, 104, 120, 136, 152, 168}
	_UpgradeStatus_index_1 = [...]uint8{0, 14, 32, 49, 66}
)

func (i UpgradeStatus) String() string {
	switch {
	case i <= 10:
		return _UpgradeStatus_name_0[_UpgradeStatus_index_0[i]:_UpgradeStatus_index_0[i+1]]
	case 12 <= i && i <= 15:
		i -= 12
		return _UpgradeStatus_name_1[_UpgradeStatus_index_1[i]:_UpgradeStatus_index_1[i+1]]
	default:
		return "UpgradeStatus(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
