package drive64

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ziutek/ftdi"
)

var (
	ErrNoDevices       = errors.New("no 64drive devices found")
	ErrMultipleDevices = errors.New("multiple 64drive devices found")
	ErrFrozen          = errors.New("64drive seems frozen, please reset it")
	ErrUnsupported     = errors.New("operation is not supported on this 64drive revision")
	ErrInvalidFifoHead = errors.New("invalid FIFO header")
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
	if err == nil {
		err = usb.PurgeBuffers()
	}
	return &Device{usb: drive64Device{usb}, desc: *d}, err
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

type drive64Device struct {
	*ftdi.Device
}

func (d *drive64Device) Read(buf []byte) (int, error) {
	// Sometimes, Drive64 is busy and FTDI returns 0-byte reads from USB.
	// This does not conform with Go io.Reader protocol (eg: they make
	// io.ReadFull stuck), so we want to retry a few times, and eventually
	// return a busy error.
	for retry := 0; retry < 5; retry++ {
		n, err := d.Device.Read(buf)
		if n == 0 && err == nil {
			continue
		}
		return n, err
	}
	return 0, ErrFrozen
}

type Device struct {
	usb  drive64Device
	desc DeviceDesc
}

// NewDeviceSingle opens a connected 64drive device, that must be the only one
// connected to this PC. If multiple devices are found, it returns ErrMultipleDevices.
// If no devices are found, it returns ErrNoDevices.
func NewDeviceSingle() (*Device, error) {
	devs := Enumerate()
	if len(devs) == 0 {
		return nil, ErrNoDevices
	}
	if len(devs) > 1 {
		return nil, ErrMultipleDevices
	}
	return devs[0].Open()
}

// NewDeviceBySerial opens a specified 64drive, identified by its serial number.
// If no device is found, ErrNoDevices is returned.
func NewDeviceBySerial(serial string) (*Device, error) {
	for _, d := range Enumerate() {
		if d.Serial == serial {
			return d.Open()
		}
	}
	return nil, ErrNoDevices
}

// Description returns a DeviceDesc that describes the current device
func (d *Device) Description() DeviceDesc {
	return d.desc
}

// Close closes an open 64drive device
func (d *Device) Close() error {
	return d.usb.Close()
}

// SendCmd sends a raw command to 64drive. This is a low-level method, most
// clients should use one of the Cmd* methods.
func (d *Device) SendCmd(cmd Cmd, args []uint32, in []byte, out []byte) error {
	var buf bytes.Buffer
	var abuf [4]byte

	buf.Write([]byte{byte(cmd), 0x43, 0x4D, 0x44})
	for _, a := range args {
		binary.BigEndian.PutUint32(abuf[:], a)
		buf.Write(abuf[:])
	}
	if len(in) != 0 {
		buf.Write(in)
	}
	if n, err := d.usb.Write(buf.Bytes()); err != nil {
		return err
	} else if n != buf.Len() {
		// Don't trust go-ftdi to implement Go io.Writer interface correctly
		panic("partial USB write")
	}

	if len(out) > 0 {
		if _, err := io.ReadFull(&d.usb, out); err != nil {
			return err
		}
	}
	if _, err := io.ReadFull(&d.usb, abuf[:]); err != nil {
		return err
	}
	if abuf[0] != 0x43 || abuf[1] != 0x4D || abuf[2] != 0x50 || abuf[3] != byte(cmd) {
		return fmt.Errorf("SendCmd: invalid completion packet (%x)", abuf)
	}
	return nil
}

// CmdVersionRequest gets the 64drive hardware and firmware version, and a magic ID that identifies
// the device (it is used during firmware upgrades to make sure that the firmware being uploaded is designed
// for this device).
func (d *Device) CmdVersionRequest() (hwver Variant, fwver Version, magic [4]byte, err error) {
	var res [8]byte
	if err = d.SendCmd(CmdVersionRequest, nil, nil, res[:]); err != nil {
		return
	}
	hwver = Variant(binary.BigEndian.Uint16(res[0:2]))
	fwver = Version(binary.BigEndian.Uint16(res[2:4]))
	copy(magic[:], res[4:8])
	return
}

// CmdSetCicType configures the 64drive CIC emulation to the specified CIC version.
// CIC emulation is only available on 64drive HW2/RevB. The function will return
// ErrUnsupported when executed on HW1/RevA device.
func (d *Device) CmdSetCicType(cic CIC) error {
	hwver, _, _, err := d.CmdVersionRequest()
	if err != nil {
		return err
	}
	if hwver < VarRevB {
		return ErrUnsupported
	}
	var args [1]uint32
	args[0] = 0x80000000 | uint32(cic)
	return d.SendCmd(CmdSetCicType, args[:], nil, nil)
}

func idealChunkSize(size int64) int {
	switch {
	case size >= 16*1024*1024:
		return 32 * 128 * 1024
	case size >= 2*1024*1024:
		return 16 * 128 * 1024
	default:
		return 4 * 128 * 1024
	}
}

func (d *Device) CmdUpload(ctx context.Context, r io.Reader, n int64, bank Bank, offset uint32) error {
	var cmdargs [2]uint32
	cmdargs[0] = offset

	chunkSize := idealChunkSize(n)
	d.usb.SetWriteChunkSize(chunkSize + 12)

	for n >= 0 && ctx.Err() == nil {
		sz := chunkSize
		if n > 0 && int64(sz) > n {
			sz = int(n)
		}
		buf := make([]byte, sz)
		read, err := io.ReadFull(r, buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		// Transfer must be multiple of 512 bytes. Pad with FF to mimic
		// non-initialized ROM. NOTE: this seems like a firmware bug on 64drive,
		// docs say that 32-bit alignment should be enough.
		for len(buf)%512 != 0 {
			buf = append(buf, 0xFF)
			read += 1
		}

		cmdargs[1] = uint32(bank)<<24 | uint32(read)
		if err := d.SendCmd(CmdLoadFromPc, cmdargs[:], buf, nil); err != nil {
			return err
		}
		cmdargs[0] += uint32(read)
		n -= int64(read)
	}

	return ctx.Err()
}

func (d *Device) CmdDownload(ctx context.Context, w io.Writer, n int64, bank Bank, offset uint32) error {
	var cmdargs [2]uint32
	cmdargs[0] = offset

	chunkSize := idealChunkSize(n)
	d.usb.SetReadChunkSize(chunkSize)
	for n > 0 && ctx.Err() == nil {
		sz := chunkSize
		if int64(sz) > n {
			sz = int(n)
		}
		buf := make([]byte, sz)
		cmdargs[1] = uint32(bank)<<24 | uint32(sz)
		if err := d.SendCmd(CmdDumpToPc, cmdargs[:], nil, buf); err != nil {
			return err
		}

		read, err := w.Write(buf)
		if err != nil {
			return err
		} else if read != len(buf) {
			panic("provided writer does not respect io.Writer interface")
		}

		cmdargs[0] += uint32(read)
		n -= int64(read)
	}

	return ctx.Err()
}

// CmdUpgradeStart triggers a firmware upgrade. The firmware must have been already loaded
// in BankCARTROM at offset 0. The upgrade happens in background; use CmdUpgradeReport to
// get a report on the status of the upgrade.
// NOTE: removing power during an upgrade might brick the 64drive unit.
func (d *Device) CmdUpgradeStart() error {
	return d.SendCmd(CmdUpgradeStart, nil, nil, nil)
}

// CmdUpgradeReport reports the status of an ongoing firmware update
func (d *Device) CmdUpgradeReport() (UpgradeStatus, error) {
	var buf [4]byte
	if err := d.SendCmd(CmdUpgradeReport, nil, nil, buf[:]); err != nil {
		return 0, err
	}
	val := binary.BigEndian.Uint32(buf[:])
	return UpgradeStatus(val & 0xF), nil
}

func (d *Device) CmdFifoRead(ctx context.Context) (typ uint8, data []byte, err error) {
	var head [4]byte
	var n int

	for ctx.Err() == nil {
		if _, err = d.usb.Read(head[:]); err == nil {
			break
		} else if err == ErrFrozen {
			time.Sleep(10 * time.Millisecond)
			continue
		} else {
			return
		}
	}
	if ctx.Err() != nil {
		err = ctx.Err()
		return
	}

	if string(head[:]) != "DMA@" {
		err = ErrInvalidFifoHead
		return
	}

	var size [4]byte
	if n, err = d.usb.Read(size[:]); err != nil || n != 4 {
		err = fmt.Errorf("USB FIFO Read: invalid packet size")
		return
	}

	typ = size[0]
	len := (int(size[1]) << 16) | (int(size[2]) << 8) | int(size[3])
	data = make([]byte, len)
	if n, err = d.usb.Read(data); err != nil || n != len {
		err = fmt.Errorf("USB FIFO Read: invalid short packet")
		return
	}

	if _, err = d.usb.Read(head[:]); err != nil || string(head[:]) != "CMPH" {
		err = fmt.Errorf("USB FIFO Read: invalid short packet")
		return
	}

	err = ctx.Err()
	return
}
