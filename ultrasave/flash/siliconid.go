package flash

import (
	"encoding/binary"
)

const ExpectedTypeID = uint32(0x11118001)

type SiliconID struct {
	typeID         uint32
	manufacturerID uint16
	deviceID       uint16
}

func newSiliconID(b [8]byte) *SiliconID {
	return &SiliconID{
		typeID:         binary.BigEndian.Uint32(b[0:4]),
		manufacturerID: binary.BigEndian.Uint16(b[4:6]),
		deviceID:       binary.BigEndian.Uint16(b[6:8]),
	}
}

func (i *SiliconID) TypeID() uint32         { return i.typeID }
func (i *SiliconID) ManufacturerID() uint16 { return i.manufacturerID }
func (i *SiliconID) DeviceID() uint16       { return i.deviceID }

func (i *SiliconID) U32() [2]uint32 {
	return [...]uint32{
		i.typeID,
		uint32(i.manufacturerID)<<16 | uint32(i.deviceID),
	}
}

func (i *SiliconID) Manufacturer() string {
	switch i.manufacturerID {
	case 0x0032:
		return "Matsushita"
	case 0x00c2:
		return "Macronix"
	}
	return "Unkonwn"
}

func (i *SiliconID) Device() string {
	switch i.U32()[1] {
	case 0x00c20000:
		return "MX29L0000"
	case 0x00c20001:
		return "MX29L0001"
	case 0x00c2001e:
		return "MX29L1100"
	case 0x00c2001d:
		return "MX29L1101_A"
	case 0x00c20084:
		return "MX29L1101_B"
	case 0x00c2008e:
		return "MX29L1101_C"
	case 0x003200f1:
		return "MN63F8MPN"
	}

	return "Unknown"
}

func (f *Flash) SiliconID() (*SiliconID, error) {
	// Quirk: MX29L1100 (and maybe other MX version),
	// may need 2 writes to CIR to switch to Silicon ID mode.
	// MN63F8MPN doesn't have this quirk.

	// For simplicity we work around the quirk regardless of chip variant.

	if err := f.writeCIR(cmdSiliconID); err != nil {
		return nil, err
	}

	if err := f.writeCIR(cmdSiliconID); err != nil {
		return nil, err
	}

	// Flash address is ignored (so use 0)
	// To read the full silicon ID a burst of 8 byte is needed (can't be done using IO).
	var data [8]byte
	if err := f.pi.ReadBurst(f.baseAddress, data[:]); err != nil {
		return nil, err
	}

	return newSiliconID(data), nil
}
