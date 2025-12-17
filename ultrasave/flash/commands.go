package flash

import (
	"fmt"
	"github.com/rasky/g64drive/ultrasave"
)

// Command Internal Register (CIR) accepts 32-bit commands.
type Command uint32

// TOVERIFY: there may exists other commands like:
// erase suspend, erase resume, abort, sector protect, sector unprotect, sector protect verify.
const (
	cmdChipErase    = Command(0x3c000000)
	cmdSectorErase  = Command(0x4b000000)
	cmdErase        = Command(0x78000000)
	cmdProgramPage  = Command(0xa5000000)
	cmdLoadBytePage = Command(0xb4000000)
	cmdStatus       = Command(0xd2000000)
	cmdSiliconID    = Command(0xe1000000)
	cmdReadArray    = Command(0xf0000000)
)

// Write command to CIR.
func (f *Flash) writeCIR(cmd Command) error {
	// CIR is located a flash address 0x00010000.
	const cirOffset = ultrasave.PiAddress(0x00010000)
	return f.pi.Write32(f.baseAddress+cirOffset, uint32(cmd))
}

func (cmd Command) String() string {
	const OpMask = Command(0xff000000)
	// PageMask should depend on Layout,
	// but we can assume 16bit mask for printing hex value
	// given that PI operate on 16bits words.
	const PageMask = Command(0x0000ffff)

	switch cmd & OpMask {
	case cmdChipErase:
		return "ChipErase"
	case cmdSectorErase:
		fmt.Sprintf("SectorErase: Page=%04x", uint16(cmd&PageMask))
	case cmdErase:
		return "Erase"
	case cmdProgramPage:
		fmt.Sprintf("ProgramPage: Page=%04x", uint16(cmd&PageMask))
	case cmdLoadBytePage:
		return "LoadBytePage"
	case cmdStatus:
		return "Status"
	case cmdSiliconID:
		return "SiliconID"
	case cmdReadArray:
		return "ReadArray"
	default:
		return fmt.Sprintf("Unknown command: %08x", uint32(cmd))
	}

	return ""
}

// Return setup erase command for either chip erase or specific sector.
// page = nil : full chip erase
// page = &Page(p) : the whole sector the page p belongs to will be erased
func setupEraseCmd(page *Page) Command {
	if page == nil {
		return cmdChipErase
	} else {
		return cmdSectorErase | Command(*page)
	}
}

func programPageCmd(page Page) Command {
	return cmdProgramPage | Command(page)
}
