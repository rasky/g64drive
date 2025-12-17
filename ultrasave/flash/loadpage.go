package flash

import (
	"encoding/binary"
	"github.com/rasky/g64drive/ultrasave"
)

// Load a maximum of 128 bytes into internal page before programming.
// Assume erased state of internal page (eg. all bits set).
func (f *Flash) LoadBytePage(data []byte) error {
	// 64drive only support bursts length which are multiple of 4.
	if len(data) > f.layout.PageSize() || len(data)%4 != 0 {
		return ErrInvalidPageSize
	}

	if err := f.writeCIR(cmdLoadBytePage); err != nil {
		return err
	}

	err := f.pi.WriteBurst(f.baseAddress, data)

	// 64DRIVE BUG: On FW < 2.04 PI burst write is not working
	// (and therefore reported as Unsupported by drive64).
	// In this case we can use IO instead to workaround the bug.
	if err == ultrasave.ErrUnsupported {
		for i := 0; i < len(data); i += 4 {
			u32 := binary.BigEndian.Uint32(data[i : i+4])
			if err := f.pi.Write32(f.baseAddress+ultrasave.PiAddress(i), u32); err != nil {
				return err
			}
		}
		return nil
	}

	return err
}
