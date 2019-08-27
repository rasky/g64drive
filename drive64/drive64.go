package drive64

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ziutek/ftdi"
)

var (
	ErrNoDevices       = errors.New("no 64drive devices found")
	ErrMultipleDevices = errors.New("multiple 64drive devices found")
)

// VendorIDs used by 64drive (actually, FTDI)
const vid = 0x0403

// ProductIDs used by 64drive (actually, FTDI)
var pids = []int{0x6010, 0x6011, 0x6012, 0x6013, 0x6014}

// DeviceDesc describes a 64drive device found attached to the system.
type DeviceDesc struct {
	Manufacturer string // USB Manufacturer string (should always be "Retroactive")
	Description  string // USB Description string (usually "64drive USB device" or "64drive USB device A")
	Serial       string // Unique serial number of the device
	VendorID     int    // USB vendor ID
	ProductID    int    // USB product ID
}

// Open this 64drive device
func (d *DeviceDesc) Open() (*Device, error) {
	usb, err := ftdi.Open(vid, d.ProductID, d.Description, d.Serial, 0, ftdi.ChannelAny)
	if err == nil {
		err = usb.SetBitmode(0xFF, ftdi.ModeReset)
	}
	if err == nil {
		err = usb.SetBitmode(0xFF, ftdi.ModeSyncFF)
	}
	return &Device{usb: usb}, err
}

// Enumerate returns a list of all 64drive devices found attached to this system
func Enumerate() []DeviceDesc {
	var devices []DeviceDesc

	for _, pid := range pids {
		devs, err := ftdi.FindAll(vid, pid)
		if err != nil {
			panic(err)
		}
		for _, d := range devs {
			if d.Manufacturer == "Retroactive" && strings.HasPrefix(d.Description, "64drive") {
				devices = append(devices, DeviceDesc{
					Manufacturer: d.Manufacturer,
					Description:  d.Description,
					Serial:       d.Serial,
					VendorID:     vid,
					ProductID:    pid,
				})
			}
		}
	}

	return devices
}

type Device struct {
	usb *ftdi.Device
}

// DeviceNewSingle opens a connected 64drive device, that must be the only one
// connected to this PC. If multiple devices are found, it returns ErrMultipleDevices.
// If no devices are found, it returns ErrNoDevices.
func DeviceNewSingle() (*Device, error) {
	devs := Enumerate()
	if len(devs) == 0 {
		return nil, ErrNoDevices
	}
	if len(devs) > 1 {
		return nil, ErrMultipleDevices
	}
	return devs[0].Open()
}

// DeviceNewBySerial opens a specified 64drive, identified by its serial number.
// If no device is found, ErrNoDevices is returned.
func DeviceNewBySerial(serial string) (*Device, error) {
	for _, d := range Enumerate() {
		if d.Serial == serial {
			return d.Open()
		}
	}
	return nil, ErrNoDevices
}

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

// Close closes an open 64drive device
func (d *Device) Close() error {
	return d.usb.Close()
}

// SendCmd sends a raw command to 64drive. This is a low-level method, most
// clients should use one of the SendCmd* methods.
func (d *Device) SendCmd(cmd Cmd, args []uint32, out []byte) error {
	var buf bytes.Buffer
	var abuf [4]byte

	buf.Write([]byte{byte(cmd), 0x43, 0x4D, 0x44})
	for _, a := range args {
		binary.BigEndian.PutUint32(abuf[:], a)
		buf.Write(abuf[:])
	}
	if n, err := d.usb.Write(buf.Bytes()); err != nil {
		return err
	} else if n != buf.Len() {
		// Don't trust go-ftdi to implement Go io.Writer interface correctly
		panic("partial USB write")
	}

	if len(out) > 0 {
		if _, err := io.ReadFull(d.usb, out); err != nil {
			return err
		}
	}
	if _, err := io.ReadFull(d.usb, abuf[:]); err != nil {
		return err
	}
	if abuf[0] != 0x43 || abuf[1] != 0x4D || abuf[2] != 0x50 || abuf[3] != byte(cmd) {
		return fmt.Errorf("SendCmd: invalid completion packet (%x)", abuf)
	}
	return nil
}

// SendCmdVersionRequest gets the 64drive hardware and firmware version, and a magic ID that identifies
// the device (it is used during firmware upgrades to make sure that the firmware being uploaded is designed
// for this device).
func (d *Device) SendCmdVersionRequest() (hwver Variant, fwver Version, magic uint32, err error) {
	var res [8]byte
	if err = d.SendCmd(CmdVersionRequest, nil, res[:]); err != nil {
		return
	}
	hwver = Variant(binary.BigEndian.Uint16(res[0:2]))
	fwver = Version(binary.BigEndian.Uint16(res[2:4]))
	magic = binary.BigEndian.Uint32(res[4:8])
	return
}
