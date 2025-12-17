package flash

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rasky/g64drive/drive64"
	"github.com/rasky/g64drive/ultrasave"
)

/* This is a collection of test that demonstrate some hardware behaviors.
 * Data damaging tests are disabled by default to avoid unpleasant surprises.
 * See the skipping message for detailed procedure (or look skipIfNoBackup code).
 *
 * For now, only tested with flash models: MN63F8MPN and MX29L1100.
 */

// XXX: I wanted to share this for other components (eeprom, RTC, ...), but golang doesn't allow to share test code across packages.
// So I'll keep that here for now, and revisit that later.
func SkipIfNoBackup(t *testing.T, component, backupProcedure string) {
	t.Helper()

	envVar := "G64DRIVE_ENABLE_DATA_DAMAGING_TESTS"

	for _, s := range strings.Split(os.Getenv(envVar), ",") {
		if s == component {
			return
		}
	}

	t.Skipf(`Skipping data damaging test.
To enable this test, you should first cleanup cartridge contacts, backup your data (ensure repeatable backup content when doing multiple reads), and then append "%s" to the comma separated list of enabled data damaging tests stored in environment variable %s":
%s && export %[2]s="${%[2]s:+${%[2]s},}%[1]s"`,
		component, envVar, backupProcedure)
}

func skipIfNoBackup(t *testing.T) {
	t.Helper()
	SkipIfNoBackup(t, "flash", `g64drive ultrasave flash read "flash_$(date +"%Y%m%d_%H%M%S").bin"`)
}

func setup(t *testing.T, opts ...FlashOption) *Flash {
	t.Helper()

	dev, err := drive64.NewDeviceSingle()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		dev.Close()
	})

	// Check firmware version and verify if it's new enough
	if _, fwver, _, err := dev.CmdVersionRequest(); err == nil {
		if fwver < 203 {
			t.Skipf("requires 64drive firmware >= 2.03, found: %v\nDownload a newer firmware from http://64drive.retroactive.be, and then run \"g64drive firmware upgrade\" to upgrade", fwver)
		}
	}

	err = dev.CmdStandAloneEnter()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		dev.CmdStandAloneLeave()
	})

	fla, err := New(ultrasave.Drive64ParallelInterfaceAdapter{dev}, opts...)
	if err != nil {
		t.Fatal(err)
	}

	// In any case leave the flash in cmdReadArray mode for next tests
	// This also cleanup the "open bus" behavior if it is triggered during tests.
	t.Cleanup(func() {
		fla.mustWriteCIR(t, cmdReadArray)
	})

	return fla
}

func (f *Flash) mustWriteCIR(t *testing.T, cmd Command) {
	t.Helper()
	if err := f.writeCIR(cmd); err != nil {
		t.Fatal(err)
	}
}

func (f *Flash) mustWriteIO(t *testing.T, offset ultrasave.PiAddress, data uint32) {
	t.Helper()
	if err := f.pi.Write32(f.baseAddress+offset, data); err != nil {
		t.Fatalf("unable to perform PI IO write [offset = %05x]: %v", offset, err)
	}
}

func (f *Flash) mustReadIO(t *testing.T, offset ultrasave.PiAddress) uint32 {
	t.Helper()
	u32, err := f.pi.Read32(f.baseAddress + offset)
	if err != nil {
		t.Fatalf("unable to perform PI IO read [offset = %05x]: %v", offset, err)
	}
	return u32
}

func (f *Flash) mustReadBurst(t *testing.T, offset ultrasave.PiAddress, size int) []byte {
	t.Helper()
	data := make([]byte, size)
	if err := f.pi.ReadBurst(f.baseAddress+offset, data); err != nil {
		t.Fatalf("unable to perform PI read burst [offset = %05x, size = %05x]: %v", offset, size, err)
	}
	return data
}

func isKnownSiliconID(siliconID []byte) bool {
	knownSiliconIDs := [][]byte{
		{0x11, 0x11, 0x80, 0x01, 0x00, 0x32, 0x00, 0xf1}, // MN63F8MPN
		{0x11, 0x11, 0x80, 0x01, 0x00, 0xc2, 0x00, 0x00}, // MX29L0000
		{0x11, 0x11, 0x80, 0x01, 0x00, 0xc2, 0x00, 0x01}, // MX29L0001
		{0x11, 0x11, 0x80, 0x01, 0x00, 0xc2, 0x00, 0x1e}, // MX29L1100
		{0x11, 0x11, 0x80, 0x01, 0x00, 0xc2, 0x00, 0x1d}, // MX29L1101_A
		{0x11, 0x11, 0x80, 0x01, 0x00, 0xc2, 0x00, 0x84}, // MX29L1101_B
		{0x11, 0x11, 0x80, 0x01, 0x00, 0xc2, 0x00, 0x8e}, // MX29L1101_C
	}

	for _, s := range knownSiliconIDs {
		if bytes.Equal(siliconID, s) {
			return true
		}
	}

	return false
}

// Useful for computing open bus observed value
func loExtend(u32 uint32) uint32 {
	lo := u32 & uint32(0xffff)
	return (lo << 16) | lo
}

// Useful for computing expected reads
func repeatLastbytesOfPattern(pattern []byte, n, size int) []byte {
	if len(pattern) < n {
		panic("invalid pattern")
	}

	if size <= len(pattern) {
		return pattern[:size]
	} else {
		data := make([]byte, size)
		copy(data[:len(pattern)], pattern[:])
		// repeat pattern (omitting prefix)
		j := len(pattern) - n
		for i := len(pattern); i < size; i++ {
			data[i] = pattern[j+(i%n)]
		}
		return data
	}
}

func toChar(b byte) byte {
	if b < 32 || b > 126 {
		return '.'
	}
	return b
}

// returns an output similar to hexdump -C
// It differs from encoding/hex.Dump mainly it 2 aspects (apart from being a naive implementation):
// * will pad data to a multiple of 16 bytes
// * will squeeze consecutive equal lines and print a * instead (very useful for sparse data like flash)
func hexDump(data []byte) string {
	// Pad data to a multiple of 16 to ease dumping logic
	for len(data)%16 != 0 {
		data = append(data, byte(0))
	}

	var b strings.Builder

	printStar := true
	for k := 0; k < len(data); k += 16 {
		if k >= 16 && bytes.Equal(data[k:k+16], data[k-16:k]) {
			if printStar {
				fmt.Fprintf(&b, "*\n")
				printStar = false
			}
			continue
		}

		printStar = true
		fmt.Fprintf(&b, "%05x  %02x %02x %02x %02x %02x %02x %02x %02x  %02x %02x %02x %02x %02x %02x %02x %02x  |%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s|\n", k,
			data[k+0], data[k+1], data[k+2], data[k+3], data[k+4], data[k+5], data[k+6], data[k+7],
			data[k+8], data[k+9], data[k+10], data[k+11], data[k+12], data[k+13], data[k+14], data[k+15],
			string(toChar(data[k+0])), string(toChar(data[k+1])), string(toChar(data[k+2])), string(toChar(data[k+3])),
			string(toChar(data[k+4])), string(toChar(data[k+5])), string(toChar(data[k+6])), string(toChar(data[k+7])),
			string(toChar(data[k+8])), string(toChar(data[k+9])), string(toChar(data[k+10])), string(toChar(data[k+11])),
			string(toChar(data[k+12])), string(toChar(data[k+13])), string(toChar(data[k+14])), string(toChar(data[k+15])),
		)
	}

	return b.String()
}

func TestMX29L1100(t *testing.T) {
	t.Skip("Still WIP")

	// Specify Layout to avoid the "auto detection" using SiliconID
	// as to not interfere with Mode transitions tests.
	f := setup(t, WithLayout(Layout_64W_128_8))

	// Please edit to match your own cart
	expectedArrayData := []byte{
		0x00, 0x01, 0xd6, 0xb3, 0xc0, 0x50, 0x00, 0x00, 0x4e, 0xfb, 0x00, 0x06, 0x1c, 0x00, 0xe3, 0x42,
		0x00, 0x15, 0x15, 0x2d, 0x5b, 0x39, 0x3d, 0x38, 0x4e, 0xfb, 0x04, 0x60, 0x53, 0x72, 0xb5, 0x7b,
	}
	expectedSiliconID := []byte{
		0x11, 0x11, 0x80, 0x01, 0x00, 0xc2, 0x00, 0x1e, 0x11, 0x11, 0x80, 0x01, 0x00, 0xc2, 0x00, 0x1e,
		0x11, 0x11, 0x80, 0x01, 0x00, 0xc2, 0x00, 0x1e, 0x11, 0x11, 0x80, 0x01, 0x00, 0xc2, 0x00, 0x1e,
	}
	expectedInternalPage := []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	}
	expectedStatus := []byte{
		0x00, 0x8c, 0x00, 0x8c, 0x00, 0x8c, 0x00, 0x8c, 0x00, 0x8c, 0x00, 0x8c, 0x00, 0x8c, 0x00, 0x8c,
		0x00, 0x8c, 0x00, 0x8c, 0x00, 0x8c, 0x00, 0x8c, 0x00, 0x8c, 0x00, 0x8c, 0x00, 0x8c, 0x00, 0x8c,
	}
	var expectedStatusWithSiliconID []byte
	expectedStatusWithSiliconID = append(expectedStatusWithSiliconID, expectedSiliconID[:2]...)
	expectedStatusWithSiliconID = append(expectedStatusWithSiliconID, expectedStatus[2:]...)
	var expectedStatusWithArrayData []byte
	expectedStatusWithArrayData = append(expectedStatusWithArrayData, expectedArrayData[:2]...)
	expectedStatusWithArrayData = append(expectedStatusWithArrayData, expectedStatus[2:]...)
	var expectedStatusWithInternalPage []byte
	expectedStatusWithInternalPage = append(expectedStatusWithInternalPage, expectedInternalPage[:2]...)
	expectedStatusWithInternalPage = append(expectedStatusWithInternalPage, expectedStatus[2:]...)

	// This test is designed to exhibit some quirks when switching between modes
	// Some transitions may need multiple writes to CIR to ensure proper reading of data.
	// Further investigation is needed to clarify what is needed to ensure proper transition between modes.

	// The core of this test is to write the CIR and then check expected values when reading (multiple times) starting from offset 0.
	mustWriteCIRAndRead := func(t *testing.T, cmd Command, expectedRead [][]byte, comment string) {
		t.Helper()
		cmdName := cmd.String()
		t.Logf("%s -> CIR: %s", cmdName, comment)
		f.mustWriteCIR(t, cmd)

		got := make([][]byte, len(expectedRead))
		for i := 0; i < len(got); i++ {
			got[i] = f.mustReadBurst(t, 0, len(expectedRead[i]))
			if !bytes.Equal(got[i], expectedRead[i]) {
				t.Errorf("unexpected read:\nexpected:\n%s\ngot:\n%s", hexDump(expectedRead[i]), hexDump(got[i]))
			}

			t.Logf("\n%s", hexDump(got[i]))
		}
	}

	// The following transitions are tested:
	// Implicit initial state -> LoadBytePage: 1 or 2 writes

	// LoadBytePage -> SiliconID: 2 writes
	// LoadBytePage -> Status: 2 writes
	// LoadBytePage -> ReadArray: 1 write

	// SiliconID -> Status: 2 writes
	// SiliconID -> ReadArray: 1 write
	// SiliconID -> LoadBytePage: 1 write

	// Status-> SiliconID: 1 write
	// Status -> LoadBytePage: 1 write
	// Status -> ReadArray: 1 write

	// ReadArray -> LoadBytePage: 1 write
	// ReadArray -> Status: 1 write (or 2 writes - but I need to write a test that demonstrate that)
	// ReadArray -> SiliconID: 1 write

	t.Log("Initial state is ReadArray")

	// NOTE: doing this read makes LoadPageByte not exhibit the quirk
	//t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))

	// LoadBytePage may need 2 writes to CIR to be effective
	// NOT ALWAYS THE CASE... (maybe depends on temperature ?)
	mustWriteCIRAndRead(t, cmdLoadBytePage, [][]byte{expectedArrayData, expectedArrayData}, "[quirk] still reads values from previous mode")
	mustWriteCIRAndRead(t, cmdLoadBytePage, [][]byte{expectedInternalPage, expectedInternalPage}, "second time should read Internal Page")

	// SiliconID may need 2 writes to CIR to be effective
	mustWriteCIRAndRead(t, cmdSiliconID, [][]byte{expectedInternalPage, expectedInternalPage}, "[quirk] still reads values from previous mode")
	mustWriteCIRAndRead(t, cmdSiliconID, [][]byte{expectedSiliconID, expectedSiliconID}, "second time should read SiliconID")

	// Status may need 2 writes to CIR to be effective
	mustWriteCIRAndRead(t, cmdStatus, [][]byte{expectedSiliconID, expectedSiliconID}, "[quirk] still reads values from previous mode")
	mustWriteCIRAndRead(t, cmdStatus, [][]byte{expectedStatusWithSiliconID, expectedStatus}, "second time should read Status, [quirk] with first 16bit word from previous mode")

	// This time Status -> SiliconID is effective on first write to CIR
	mustWriteCIRAndRead(t, cmdSiliconID, [][]byte{expectedSiliconID, expectedSiliconID}, "should read SiliconID")

	// ReadArray seems to always to be effective at first write to CIR
	mustWriteCIRAndRead(t, cmdReadArray, [][]byte{expectedArrayData, expectedArrayData}, "should read Array data")

	// This time ReadArray -> LoadBytePage is effective on first write to CIR
	mustWriteCIRAndRead(t, cmdLoadBytePage, [][]byte{expectedInternalPage, expectedInternalPage}, "should read Internal Page")

	// SiliconID may need 2 writes to CIR to be effective
	mustWriteCIRAndRead(t, cmdSiliconID, [][]byte{expectedInternalPage, expectedInternalPage}, "[quirk] still reading values from previous mode")
	mustWriteCIRAndRead(t, cmdSiliconID, [][]byte{expectedSiliconID, expectedSiliconID}, "second time should read SiliconID")

	// Status may need 2 writes to CIR to be effective
	mustWriteCIRAndRead(t, cmdStatus, [][]byte{expectedSiliconID, expectedSiliconID}, "[quirk] still reading values from previous mode")
	mustWriteCIRAndRead(t, cmdStatus, [][]byte{expectedStatusWithSiliconID, expectedStatus}, "second time should read Status, [quirk] with first 16bit word from previous mode")

	// This time Status -> SiliconID is effective on first write to CIR
	mustWriteCIRAndRead(t, cmdSiliconID, [][]byte{expectedSiliconID, expectedSiliconID}, "should read SiliconID")

	// ReadArray seems to always to be effective at first write to CIR
	mustWriteCIRAndRead(t, cmdReadArray, [][]byte{expectedArrayData, expectedArrayData}, "should read Array data")

	// This time ReadArray -> LoadBytePage is effective on first write to CIR
	mustWriteCIRAndRead(t, cmdLoadBytePage, [][]byte{expectedInternalPage, expectedInternalPage}, "should read Internal Page")

	// SiliconID may need 2 writes to CIR to be effective
	mustWriteCIRAndRead(t, cmdSiliconID, [][]byte{expectedInternalPage, expectedInternalPage}, "[quirk] still reading values from previous mode")
	mustWriteCIRAndRead(t, cmdSiliconID, [][]byte{expectedSiliconID, expectedSiliconID}, "second time should read SiliconID")

	// Status may need 2 writes to CIR to be effective
	mustWriteCIRAndRead(t, cmdStatus, [][]byte{expectedSiliconID, expectedSiliconID}, "[quirk] still reading values from previous mode")
	mustWriteCIRAndRead(t, cmdStatus, [][]byte{expectedStatusWithSiliconID, expectedStatus}, "second time should read Status, [quirk] with first 16bit word from previous mode")

	mustWriteCIRAndRead(t, cmdLoadBytePage, [][]byte{expectedInternalPage, expectedInternalPage}, "should read Internal Page")
	mustWriteCIRAndRead(t, cmdStatus, [][]byte{expectedInternalPage, expectedInternalPage}, "[quirk] still reading values from previous mode")
	mustWriteCIRAndRead(t, cmdStatus, [][]byte{expectedStatusWithInternalPage, expectedStatus}, "second time should read Status, [quirk] with first 16bit word from previous mode")

	mustWriteCIRAndRead(t, cmdReadArray, [][]byte{expectedArrayData, expectedArrayData}, "should read Array data")
	mustWriteCIRAndRead(t, cmdStatus, [][]byte{expectedStatusWithArrayData, expectedStatus}, "should read Status; [quirk] with first 16bit word from previous mode")

	mustWriteCIRAndRead(t, cmdReadArray, [][]byte{expectedArrayData, expectedArrayData}, "should read Array data")
	mustWriteCIRAndRead(t, cmdSiliconID, [][]byte{expectedSiliconID, expectedSiliconID}, "should read SiliconID")
	mustWriteCIRAndRead(t, cmdLoadBytePage, [][]byte{expectedInternalPage, expectedInternalPage}, "should read Internal Page")
	mustWriteCIRAndRead(t, cmdReadArray, [][]byte{expectedArrayData, expectedArrayData}, "should read Array data")
	mustWriteCIRAndRead(t, cmdLoadBytePage, [][]byte{expectedInternalPage, expectedInternalPage}, "should read Internal Page")
	mustWriteCIRAndRead(t, cmdReadArray, [][]byte{expectedArrayData}, "should read Array data")
	mustWriteCIRAndRead(t, cmdStatus, [][]byte{expectedStatusWithArrayData, expectedStatus}, "should read Status; [quirk] with first 16bit word from previous mode")

	// OLD VERSION OF TEST THAT JUST PRINT STUFF - NOT DELETED YET BECAUSE THEY GIVE SOME DIFFERENT RESULTS

	// Switching between modes, may require more than one write to CIR
	// ReadArray -> Status: 2 writes
	// ReadArray -> SiliconID: 1 writes (2 if starting from initial ReadArray state)
	// ReadArray -> LoadBytePage: 1 write
	// Status -> ReadArray: 1 write
	// Status -> SiliconID: 1 write
	// Status -> LoadBytePage: 1 write
	// SiliconID -> ReadArray: 1 write
	// SiliconID -> Status: 2 writes
	// SiliconID -> LoadBytePage: 1 write
	// LoadBytePage -> ReadArray: 1 write
	// LoadBytePage -> SiliconID: 1 write
	// LoadBytePage -> Status: 2 writes

	// Switching to status mode seems to require 2 writes to CIR to ensure proper reading of status

	// XXX If doing only 1 write to CIR status is not read, we're still reading Array
	t.Log("[RA] 1 CIR status - 3 reads - 1 CIR Array")
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdReadArray)

	// Doing 2 write to CIR allows to read status, but first word seems to come from ReadArray
	// so a dummy IO may be needed
	t.Log("2 CIR status - 3 reads - 1 CIR Array")
	f.mustWriteCIR(t, cmdStatus)
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdReadArray)

	// Doing 2 write to CIR allows to read status, but first word seems to come from ReadArray
	// so a dummy IO may be needed
	t.Log("2 CIR status - 1 dummy IO, 3 reads - 1 CIR Array")
	f.mustWriteCIR(t, cmdStatus)
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("dummy read: %08x\n", f.mustReadIO(t, 0))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdReadArray)

	// Doing 2 write to CIR allows to read status, but first word seems to come from ReadArray
	// so a dummy IO may be needed
	t.Log("1 CIR status - 1 dummy IO, 3 reads - 1 CIR Array")
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("dummy read: %08x\n", f.mustReadIO(t, 0))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdReadArray)

	// If doing only 1 write to CIR status is not read, we're still reading from last mode
	t.Log("2 CIR siliconID, 1 CIR status - 3 reads - 1 CIR Array")
	f.mustWriteCIR(t, cmdSiliconID)
	f.mustWriteCIR(t, cmdSiliconID)
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdReadArray)

	// If doing only 1 write to CIR status is not read, we're still reading from last mode
	t.Log("2 CIR siliconID - 2 CIR status - 3 reads - 1 CIR Array")
	f.mustWriteCIR(t, cmdSiliconID)
	f.mustWriteCIR(t, cmdSiliconID)
	f.mustWriteCIR(t, cmdStatus)
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdReadArray)

	// If doing only 1 write to CIR status is not read, we're still reading from last mode
	t.Log("1 CIR siliconID - 2 reads - 2 CIR status - 3 reads - 1 CIR Array")
	f.mustWriteCIR(t, cmdSiliconID)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdStatus)
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdReadArray)

	// This is what libultra does (but with only 1 read in between
	t.Log("1 CIR status - 2 reads - 1 CIR status - 2 read - 1 CIR siliconID - 2 reads - 1 CIR Array")
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdSiliconID)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdReadArray)

	t.Log("1 CIR siliconID - 2 reads - 2 CIR status - 3 reads - 1 CIR SiliconID - 2 reads")
	f.mustWriteCIR(t, cmdSiliconID)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdStatus)
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	f.mustWriteCIR(t, cmdSiliconID)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))

	t.Log("1 CIR ReadArray - 2 reads - 1 CIR LoadBytePage - 3 reads - 1 CIR ReadArray - 2 reads")
	t.Log("CIR ReadArray")
	f.mustWriteCIR(t, cmdReadArray)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Log("CIR LoadBytePage")
	f.mustWriteCIR(t, cmdLoadBytePage)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Log("CIR ReadArray")
	f.mustWriteCIR(t, cmdReadArray)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Log("CIR SiliconID")
	f.mustWriteCIR(t, cmdSiliconID)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Log("CIR SiliconID")
	f.mustWriteCIR(t, cmdSiliconID)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Log("CIR LoadBytePage")
	f.mustWriteCIR(t, cmdLoadBytePage)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Log("CIR Status")
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Log("CIR Status")
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Log("CIR LoadBytePage")
	f.mustWriteCIR(t, cmdLoadBytePage)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Log("CIR ReadArray")
	f.mustWriteCIR(t, cmdReadArray)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Log("CIR SiliconID")
	f.mustWriteCIR(t, cmdSiliconID)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Log("CIR Status")
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Log("CIR Status")
	f.mustWriteCIR(t, cmdStatus)
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))

	/*
	   f.mustWriteCIR(t, cmdReadArray)

	   t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, 32)))
	   t.Logf("\n%s", hexDump(f.mustReadBurst(t, 4, 32)))

	   t.Logf("%05x %08x\n", 0, f.mustReadIO(t, 0))
	   t.Logf("%05x %08x\n", 4, f.mustReadIO(t, 4))

	   t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0x100, 32)))
	   t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0x104, 32)))

	   t.Logf("%05x %08x\n", 0, f.mustReadIO(t, 0x100))
	   t.Logf("%05x %08x\n", 4, f.mustReadIO(t, 0x104))
	*/
}

func TestReads(t *testing.T) {
	f := setup(t)
	SiliconID, err := f.SiliconID()
	if err != nil {
		t.Fatal(err)
	}

	// When in Status mode, read address lowest 16bits are ignored.
	// Only the length matters. The following pattern is read:
	// 00 <u8> 00 <u8> 00 <u8> ... (with u8 being the status register bits)
	// If address bit 17 is set, reads return the "open bus" value
	// (lowest half of 32 bit address repeated on the upper half).
	// After an open bus read, all subsequent reads until a new command is written to CIR will
	// return the open bus value.
	t.Run("status mode", func(t *testing.T) {
		f.mustWriteCIR(t, cmdStatus)

		// Some chips (for now only tested on MX29L1100) require 2 writes to CIR to fully switch to Status mode
		// And the first read from Status will have the upper 16bit from previous mode
		// Discard the first 32bit using a dummy IO.
		if SiliconID.ManufacturerID() == 0xc2 {
			f.mustWriteCIR(t, cmdStatus)
			f.mustReadIO(t, 0)
		}

		// expected read for address range [0x00000:0x0ffff]
		// 00 <u8> 00 <u8> .... (with u8 being the status register bits)
		expectedRead := func(status Status, n int) []byte {
			return repeatLastbytesOfPattern([]byte{0x00, byte(status)}, 2, n)
		}
		expectedRead32 := func(status Status) uint32 {
			return binary.BigEndian.Uint32(expectedRead(status, 4))
		}

		t.Run("4 byte aligned IO reads", func(t *testing.T) {
			// range [0x00000:0x0ffff] always return the pattern 00 <u8> 00 <u8> (with u8 being the status register bits)
			t.Run("range 0x00000:0x0ffff", func(t *testing.T) {
				for i := 0; i < 0x100; i += 4 {
					u32 := f.mustReadIO(t, ultrasave.PiAddress(i))
					expected, got := expectedRead32(Status(u32&0xff)), u32
					if expected != got {
						t.Errorf("unexpected values @%05x expected %08x != got %08x", i, expected, got)
					}
				}
			})

			// range [0x10000:0x1ffff] always return open bus
			// writing a new value to CIR is needed to "reset" internal state and allow proper reading
			t.Run("range 0x10000:0x1ffff", func(t *testing.T) {
				for i := 0x1cba4; i < 0x1cca4; i += 4 {
					u32 := f.mustReadIO(t, ultrasave.PiAddress(i))
					expected, got := loExtend(u32), u32
					if expected != got {
						t.Errorf("unexpected values @%05x expected %08x != got %08x", i, expected, got)
					}
				}

				// after reading in open range values, all other read will return open range values
				// until we reset CIR
				t.Run("corrupted reads after open bus", func(t *testing.T) {
					i := ultrasave.PiAddress(4)
					u32 := f.mustReadIO(t, i)
					expected, got := loExtend(u32), u32
					if expected != got {
						t.Errorf("unexpected values @%05x expected %08x != got %08x", i, expected, got)
					}
				})

				f.mustWriteCIR(t, cmdStatus)

				t.Run("proper read after new CIR", func(t *testing.T) {
					i := ultrasave.PiAddress(4)
					u32 := f.mustReadIO(t, i)
					expected, got := expectedRead32(Status(u32&0xff)), u32
					if expected != got {
						t.Errorf("unexpected values @%05x expected %08x != got %08x", i, expected, got)
					}
				})
			})
		})

		t.Run("DMA reads", func(t *testing.T) {
			// range [0x00000:0x0ffff]
			// offset doesn't matter, just the size of the burst
			t.Run("range [0x00000:0x0ffff]", func(t *testing.T) {
				testCases := []struct {
					offset ultrasave.PiAddress
					size   int
				}{
					{offset: 0x00000, size: 0x04},
					{offset: 0x00000, size: 0x08},
					{offset: 0x00000, size: 0x10000},
					{offset: 0x00004, size: 0x10},
					{offset: 0x00008, size: 0x10},
					{offset: 0x07ffc, size: 0x20},
					{offset: 0x08000, size: 0x8000},
				}
				for _, tc := range testCases {
					t.Run(fmt.Sprintf("offset=%05x size=%x", tc.offset, tc.size), func(t *testing.T) {
						data := f.mustReadBurst(t, tc.offset, tc.size)
						t.Logf("\n%s", hexDump(data))
						// in this case data[1] = 0x80 because there shouldn't be any WSM activity
						expected, got := expectedRead(Status(data[1]), tc.size), data
						if !bytes.Equal(expected, got) {
							t.Errorf("unexpected values (size=%05x) @%05x:expected:\n%s\n!= got:\n%s", tc.size, tc.offset, hexDump(expected), hexDump(got))
						}
					})
				}
			})

			// range [0x10000:0x1ffff] (open bus)
			t.Run("range [0x10000:0x1ffff]", func(t *testing.T) {
				// open bus
				f.mustWriteCIR(t, cmdStatus)
				data := f.mustReadBurst(t, 0x0fff8, 0x10)
				t.Logf("data @%05x:\n%s", 0x0fff8, hexDump(data))

				f.mustWriteCIR(t, cmdStatus)
				data = f.mustReadBurst(t, 0x10000, 0x8000)
				t.Logf("data @%05x:\n%s", 0x10000, hexDump(data))

				f.mustWriteCIR(t, cmdStatus)
				data = f.mustReadBurst(t, 0x1cba4, 0x20)
				t.Logf("data @%05x:\n%s", 0x1cba4, hexDump(data))

				f.mustWriteCIR(t, cmdStatus)
				data = f.mustReadBurst(t, 0x18000, 0x8000)
				t.Logf("data @%05x:\n%s", 0x18000, hexDump(data))

				f.mustWriteCIR(t, cmdStatus)
				data = f.mustReadBurst(t, 0x20000, 0x100)
				t.Logf("data @%05x:\n%s", 0x20000, hexDump(data))

				f.mustWriteCIR(t, cmdStatus)
			})
		})
	})

	// When in SiliconID mode, read address lowest 16bits are ignored.
	// Only the length matters. For length N <= 8 it returns the first N
	// bytes of SiliconID. For length > 8, the behavior is variant dependant:
	// MN63F8MPN: last 16bit word is repeated
	// MX29L1100: SiliconID bytes are repeated
	//
	// MN63F8MPN: If address bit 17 is set, reads return the "open bus" value
	// (lowest half of 32 bit address repeated on the upper half).
	// After an open bus read, all subsequent reads until a new command is written to CIR will
	// return the open bus value.
	t.Run("silicon ID mode", func(t *testing.T) {
		f.mustWriteCIR(t, cmdSiliconID)
		f.mustWriteCIR(t, cmdSiliconID)
		siliconID := f.mustReadBurst(t, 0, 8)

		// expected read for address range [0x00000:0x0ffff]
		var expectedRead func(int) []byte
		switch SiliconID.Device() {
		case "MN63F8MPN":
			expectedRead = func(n int) []byte {
				return repeatLastbytesOfPattern(siliconID, 2, n)
			}
		default:
			expectedRead = func(n int) []byte {
				return repeatLastbytesOfPattern(siliconID, 8, n)
			}
		}

		expectedRead32 := func() uint32 {
			return binary.BigEndian.Uint32(expectedRead(4))
		}

		// data read should match known Silicon ID
		t.Run("known ID", func(t *testing.T) {
			if !isKnownSiliconID(siliconID) {
				t.Errorf("unexpected silicon ID %02x", siliconID)
			}
		})

		t.Run("4 byte aligned IO reads", func(t *testing.T) {

			// range [0x00000:0x0ffff] always return the first u32 (eg. TypeID)
			t.Run("range 0x00000:0x0ffff", func(t *testing.T) {
				for i := 0; i < 0x100; i += 4 {
					u32 := f.mustReadIO(t, ultrasave.PiAddress(i))
					expected, got := expectedRead32(), u32
					if expected != got {
						t.Errorf("unexpected values @%05x expected %08x != got %08x", i, expected, got)
					}
				}
			})

			switch SiliconID.Device() {
			case "MN63F8MPN":
				// range [0x10000:0x1ffff] always return open bus
				// writing a new value to CIR is needed to "reset" internal state and allow proper reading
				t.Run("range 0x10000:0x1ffff", func(t *testing.T) {
					for i := 0x1cba4; i < 0x1cca4; i += 4 {
						u32 := f.mustReadIO(t, ultrasave.PiAddress(i))
						expected, got := loExtend(u32), u32
						if expected != got {
							t.Errorf("unexpected values @%05x expected %08x != got %08x", i, expected, got)
						}
					}

					// after reading in open range values, all other read will return open range values
					// until we reset CIR
					t.Run("corrupted reads after open bus", func(t *testing.T) {
						i := ultrasave.PiAddress(4)
						u32 := f.mustReadIO(t, i)
						expected, got := loExtend(u32), u32
						if expected != got {
							t.Errorf("unexpected values @%05x expected %08x != got %08x", i, expected, got)
						}
					})

					f.mustWriteCIR(t, cmdSiliconID)

					t.Run("proper read after new CIR", func(t *testing.T) {
						i := ultrasave.PiAddress(4)
						u32 := f.mustReadIO(t, i)
						expected, got := expectedRead32(), u32
						if expected != got {
							t.Errorf("unexpected values @%05x expected %08x != got %08x", i, expected, got)
						}
					})
				})
			case "MX29L1100":
				// range [0x10000:0x1ffff] still returns SiliconID
				t.Run("range 0x10000:0x1ffff", func(t *testing.T) {
					for i := 0; i < 0x100; i += 4 {
						u32 := f.mustReadIO(t, ultrasave.PiAddress(i))
						expected, got := expectedRead32(), u32
						if expected != got {
							t.Errorf("unexpected values @%05x expected %08x != got %08x", i, expected, got)
						}
					}
				})
			}
		})

		t.Run("DMA reads", func(t *testing.T) {
			// range [0x00000:0x0ffff]
			// offset doesn't matter, just the size of the burst
			t.Run("range 0x00000:0x0ffff", func(t *testing.T) {
				testCases := []struct {
					offset ultrasave.PiAddress
					size   int
				}{
					{offset: 0x00000, size: 0x04},
					{offset: 0x00000, size: 0x08},
					{offset: 0x00000, size: 0x10000},
					{offset: 0x00004, size: 0x10},
					{offset: 0x00008, size: 0x10},
					{offset: 0x07ffc, size: 0x20},
					{offset: 0x08000, size: 0x8000},
				}
				for _, tc := range testCases {
					t.Run(fmt.Sprintf("offset=%05x size=%x", tc.offset, tc.size), func(t *testing.T) {
						data := f.mustReadBurst(t, tc.offset, tc.size)
						t.Logf("\n%s", hexDump(data))
						expected, got := expectedRead(tc.size), data
						if !bytes.Equal(expected, got) {
							t.Errorf("unexpected values (size=%05x) @%05x:expected:\n%s\n!= got:\n%s", tc.size, tc.offset, hexDump(expected), hexDump(got))
						}
					})
				}
			})

			// range [0x10000:0x1ffff] (open bus)
			t.Run("range [0x10000:0x1ffff]", func(t *testing.T) {
				// open bus
				f.mustWriteCIR(t, cmdSiliconID)
				data := f.mustReadBurst(t, 0x0fff8, 0x10)
				t.Logf("data @%05x:\n%s", 0x0fff8, hexDump(data))

				f.mustWriteCIR(t, cmdSiliconID)
				data = f.mustReadBurst(t, 0x10000, 0x8000)
				t.Logf("data @%05x:\n%s", 0x10000, hexDump(data))

				f.mustWriteCIR(t, cmdSiliconID)
				data = f.mustReadBurst(t, 0x1cba4, 0x20)
				t.Logf("data @%05x:\n%s", 0x1cba4, hexDump(data))

				f.mustWriteCIR(t, cmdSiliconID)
				data = f.mustReadBurst(t, 0x18000, 0x8000)
				t.Logf("data @%05x:\n%s", 0x18000, hexDump(data))

				f.mustWriteCIR(t, cmdSiliconID)
				data = f.mustReadBurst(t, 0x20000, 0x100)
				t.Logf("data @%05x:\n%s", 0x20000, hexDump(data))

				f.mustWriteCIR(t, cmdSiliconID)
			})
		})
	})

	//
	t.Run("read array mode", func(t *testing.T) {
		f.mustWriteCIR(t, cmdReadArray)

		// In ReadArray mode address need to be adjusted depending on byte / word addressing
		// We use f.layout.ReadAddress for that.

		t.Run("DMA after 0x20000", func(t *testing.T) {
			data := f.mustReadBurst(t, f.layout.ReadAddress(0x2abcd), 0x20)
			t.Logf("data @0x2abcd:\n%s", hexDump(data))

			var expected []byte
			switch SiliconID.Device() {
			case "MN63F8MPN":
				expected = repeatLastbytesOfPattern([]byte{0xab, 0xcd}, 2, 0x20)
			default:
				expected = repeatLastbytesOfPattern([]byte{0xff}, 1, 0x20)
			}
			if got := data; !bytes.Equal(expected, got) {
				t.Errorf("unexpected values expected %02x != got %02x", expected, got)
			}
			// contrary to Status and SiliconID mode, we don't need to write a new command into CIR
			// to read normal data.
		})

		t.Run("IO after 0x20000", func(t *testing.T) {
			data := f.mustReadIO(t, f.layout.ReadAddress(0x2abcd))
			t.Logf("data @0x2abcd:\n%08x", data)
			var expected uint32
			switch SiliconID.Device() {
			case "MN63F8MPN":
				expected = loExtend(0xabcd)
			default:
				expected = 0xffffffff
			}
			if got := data; expected != got {
				t.Errorf("unexpected values expected %08x != got %08x", expected, got)
			}
		})

		t.Run("DMA crossing 256-page boundary", func(t *testing.T) {
			// In this test we read 16 bytes from [boundary - 4: boundary + 12]
			// and observe that DMA and IO behavior are not the same.
			for _, boundary := range []int{0x08000, 0x10000, 0x18000} {
				t.Run(fmt.Sprintf("DMA behavior at boundary - offset %05x", boundary), func(t *testing.T) {
					startAddress := boundary - 4
					burstLen := 16

					// use IO reads to build reference data

					// dataFromIO is just sequential IO read
					var dataFromIO []byte
					for i := 0; i < burstLen; i = i + 4 {
						dataFromIO = binary.BigEndian.AppendUint32(dataFromIO, f.mustReadIO(t, f.layout.ReadAddress(startAddress+i)))
					}

					// expectedData wraps around at boundary based on first DMA address
					var expectedData []byte
					expectedData = binary.BigEndian.AppendUint32(expectedData, f.mustReadIO(t, f.layout.ReadAddress(startAddress)))
					for i := 0; i < (burstLen - 4); i = i + 4 {
						expectedData = binary.BigEndian.AppendUint32(expectedData, f.mustReadIO(t, f.layout.ReadAddress((startAddress&^0x7fff)+(i&0x7fff))))
					}

					if bytes.Equal(expectedData, dataFromIO) {
						t.Skip("current flash content can't demonstrate 256-page boundary crossing behavior")
					}

					dataFromBurst := f.mustReadBurst(t, f.layout.ReadAddress(startAddress), burstLen)

					if bytes.Equal(dataFromIO, dataFromBurst) {
						t.Errorf("unexpected equality of data when crossing boundary %x", boundary)
					}

					if !bytes.Equal(expectedData, dataFromBurst) {
						t.Errorf("unexpected inequality %x != %x at boundary %x", expectedData, dataFromBurst, boundary)
					}

					t.Logf("data from IO:\n%s", hexDump(dataFromIO))
					t.Logf("data from burst:\n%s", hexDump(dataFromBurst))
					t.Logf("expected data:\n%s", hexDump(expectedData))
				})
			}
		})
	})
}

func TestWrites(t *testing.T) {
	skipIfNoBackup(t)

	f := setup(t)

	SiliconID, err := f.SiliconID()
	if err != nil {
		t.Fatal(err)
	}

	l := f.layout
	pageSize := l.PageSize()

	// use sector 7 because on my test cart there is no valuable data here
	page := Page(0x380)
	sectorFirstPage, sectorLastPage := l.SectorBegin(page), l.SectorEnd(page)

	// Backup data
	backupData, elapsed := func() (data []byte, elapsed time.Duration) {
		defer func(now time.Time) { elapsed = time.Since(now) }(time.Now())
		var err error

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		data, err = f.ReadPages(ctx, sectorFirstPage, sectorLastPage)
		if err != nil {
			t.Fatal(err)
		}

		return
	}()
	// around XXms
	t.Logf("reading chip took %s", elapsed)

	// Ensure WSM is ready before starting erase command
	if status, err := f.Status(); err != nil {
		t.Fatal(err)
	} else {
		if expected, got := StatusWSMReady, status&StatusWSMReady; expected != got {
			t.Fatalf("unexpected status: %02x != %02x", expected, got)
		}
	}

	t.Run("cmdErase is ignored if run without preliminary chip/sector erase setup", func(t *testing.T) {
		f.mustWriteCIR(t, cmdErase)
		// immediately read some content to see if some mode switch is done
		t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, pageSize)))

		// explicitly switch to status mode to see if some errors are reported
		status, err := f.Status()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("status: %02x", status)

		// clear status just in case
		if err := f.ClearStatus(); err != nil {
			t.Fatal(err)
		}

		// switch to read array for good measure
		f.mustWriteCIR(t, cmdReadArray)

		t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, pageSize)))
	})

	t.Run("dump internal page content", func(t *testing.T) {
		f.mustWriteCIR(t, cmdLoadBytePage)
		t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, pageSize)))
	})

	// bits 23-10 of cmdSectorErase are ignored
	f.mustWriteCIR(t, cmdSectorErase|Command(0xfffc00)|Command(page))
	f.mustWriteCIR(t, cmdErase)

	status, elapsed := func() (status Status, elapsed time.Duration) {
		defer func(now time.Time) { elapsed = time.Since(now) }(time.Now())
		var err error

		// Status register can be read directly after writing erase command
		// without having to explicitly write cmdStatus to CIR.
		// At this point status should equal EraseBusy (eg. WSMReady cleared, EraseBusy set).
		t.Run("status after start Erase", func(t *testing.T) {
			if status, err = f.status(); err != nil {
				t.Fatal(err)
			} else {
				if status == StatusEraseOK|StatusWSMReady {
					t.Skip("operation finished before our measurement")
				}

				if expected, got := StatusEraseBusy, status; expected != got {
					t.Errorf("unexpected status: %02x != %02x", expected, got)
				}
			}
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		status, err = f.waitStatusBitsCleared(ctx, StatusEraseBusy, 10*time.Millisecond, 500*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}

		return
	}()

	// Erasing a sector should take around 280ms (MN63F8MPN) / 85ms (MX29L1100) (caveat: this include USB latency)
	t.Logf("erase sector took %s", elapsed)
	if expectedDuration := 280 * time.Millisecond; (elapsed - expectedDuration).Abs() > expectedDuration/10 {
		t.Logf("warning: erase operation duration is not within %s +- 10%%", expectedDuration)
	}

	// On erase successful completion, we expect EraseOK + WSMReady
	t.Run("status after Erase completed", func(t *testing.T) {
		if expected, got := StatusEraseOK|StatusWSMReady, status&(StatusEraseOK|StatusWSMReady); expected != got {
			t.Errorf("unexpected status: %02x != %02x", expected, got)
		}
	})

	// (MN63F8MPN only): clearing status also works with any value and any offset, not just 0 (as done in clearStatus)
	// For MX29L1100 it seems to corrupt the first 16bits of what gets written to flash array
	f.mustWriteIO(t, 0x0, 0x0)

	t.Run("status after clear", func(t *testing.T) {
		status, err := f.status()
		if err != nil {
			t.Fatal(err)
		}
		if expected, got := StatusWSMReady, status&StatusWSMReady; expected != got {
			t.Errorf("unexpected status: %02x != %02x", expected, got)
		}
	})

	// Verify that sector has been erased
	sectorData, elapsed := func() (data []byte, elapsed time.Duration) {
		defer func(now time.Time) { elapsed = time.Since(now) }(time.Now())
		var err error

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		data, err = f.ReadPages(ctx, sectorFirstPage, sectorLastPage)
		if err != nil {
			t.Fatal(err)
		}

		return
	}()
	// around 6ms
	t.Logf("reading sector took %s", elapsed)

	t.Run("sector data is erased", func(t *testing.T) {
		for i, v := range sectorData {
			if v != byte(0xff) {
				t.Errorf("unexpected value @%05x after erase: 0xff != %02x", i, v)
				t.Logf("data:\n%s", hexDump(sectorData))
				break
			}
		}
	})

	// Load byte page
	f.mustWriteCIR(t, cmdLoadBytePage)

	t.Run("initial internal page is erased - internal page can be read using DMA", func(t *testing.T) {
		internalPage := f.mustReadBurst(t, 0, pageSize)
		for i, v := range internalPage {
			if v != byte(0xff) {
				t.Errorf("unexpected value @%05x after erase: 0xff != %02x", i, v)
				t.Logf("data:\n%s", hexDump(internalPage))
				break
			}
		}
	})

	// Detect if writes to internal page can only clear bits
	// By writing a value and it's inverse in the same location we should get all bits cleared
	// if it's the case otherwise we get the last written value.
	f.mustWriteIO(t, ultrasave.PiAddress(0x10), 0xdeadbeef)
	f.mustWriteIO(t, ultrasave.PiAddress(0x10), ^uint32(0xdeadbeef))

	var expectedHoleData = ^uint32(0xdeadbeef)

	switch SiliconID.Device() {
	case "MN63F8MPN":
		t.Run("writes to internal page only clear bits", func(t *testing.T) {
			expectedHoleData = uint32(0)
			u32 := f.mustReadIO(t, ultrasave.PiAddress(0x10))
			if expected, got := expectedHoleData, u32; expected != got {
				t.Errorf("unexpected values expected %08x != got %08x", expected, got)
			}
		})
	default: // MX29L1100
		t.Run("writes to internal page behave normally", func(t *testing.T) {
			u32 := f.mustReadIO(t, ultrasave.PiAddress(0x10))
			if expected, got := expectedHoleData, u32; expected != got {
				t.Errorf("unexpected values expected %08x != got %08x", expected, got)
			}
		})
	}

	// Re-writing Load Byte Page command while already being in Load Byte Page mode doesn't change internal page content
	f.mustWriteCIR(t, cmdLoadBytePage)

	t.Run("re-writing Load Byte Page command doesn't change internal page content", func(t *testing.T) {
		got := f.mustReadBurst(t, 0, pageSize)
		expected := bytes.Repeat([]byte{0xff}, pageSize)
		binary.BigEndian.PutUint32(expected[0x10:0x14], expectedHoleData)

		t.Logf("data:\n%s", hexDump(got))
		if !bytes.Equal(expected, got) {
			t.Errorf("expected:\n%s\ngot:\n%s", hexDump(expected), hexDump(got))
		}
	})

	// Load "random" page bytes with a "hole" in the range [0x20:0x2f]
	for k := 0; k < pageSize; k += 4 {
		v := []byte{byte(k), byte(k + 1), byte(k + 2), byte(k + 3)}
		if k >= 0x20 && k < 0x30 {
			continue
		}
		f.mustWriteIO(t, ultrasave.PiAddress(k), binary.BigEndian.Uint32(v))
	}

	// Read back internal page
	expectedPageData := f.mustReadBurst(t, 0, pageSize)
	t.Run("internal page is readable until programmed", func(t *testing.T) {
		for i, v := range expectedPageData {
			expected, got := byte(i), v
			switch {
			case i >= 0x10 && i < 0x14:
				if SiliconID.Device() == "MN63F8MPN" {
					expected = byte(0x00)
				}
			case i >= 0x20 && i < 0x30:
				expected = byte(0xff)
			}

			if expected != got {
				t.Errorf("unexpected value @%05x after programming: %02x != %02x", i, expected, got)
				t.Logf("data:\n%s", hexDump(expectedPageData))
				break
			}
		}
	})

	// Switch to ReadArray doesn't seem to affect internal page content.
	f.mustWriteCIR(t, cmdReadArray)
	f.mustWriteCIR(t, cmdReadArray)
	f.mustWriteCIR(t, cmdReadArray)
	f.mustWriteCIR(t, cmdLoadBytePage)
	t.Run("dump internal page content", func(t *testing.T) {
		f.mustWriteCIR(t, cmdLoadBytePage)
		t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, pageSize)))
	})

	// bits 23-10 of programPageCmd are ignored
	f.mustWriteCIR(t, programPageCmd(page)|0xfffc00)

	status, elapsed = func() (status Status, elapsed time.Duration) {
		defer func(now time.Time) { elapsed = time.Since(now) }(time.Now())
		var err error

		// Status register can be read directly after writing program command
		// without having to explicitly write cmdStatus to CIR.
		// At this point status should equal ProgramBusy (eg. WSMReady cleared, ProgramBusy set).
		t.Run("status after start Program", func(t *testing.T) {
			if status, err = f.status(); err != nil {
				t.Fatal(err)
			} else {
				if status == StatusProgramOK|StatusWSMReady {
					t.Skip("operation finished before our measurement")
				}

				if expected, got := StatusProgramBusy, status; expected != got {
					t.Errorf("unexpected status: %02x != %02x", expected, got)
				}
			}
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		status, err = f.waitStatusBitsCleared(ctx, StatusProgramBusy, 1*time.Millisecond, 10*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}

		return
	}()

	// Programming a page could take as low as 280µs and up to 1ms (on MN63F8MPN)
	// (~3ms on MX29L1100)
	// but given the way we conduct the measures (64drive + USB + OS overhead)
	// there is too much jitter to assert anything.
	// TODO: maybe we should take a statistical approach to measure that.
	t.Logf("program page took %s", elapsed)
	if expectedDuration := 500 * time.Microsecond; (elapsed - expectedDuration).Abs() > expectedDuration {
		t.Logf("warning: program operation duration is not within %s +- 100%%", expectedDuration)
	}

	// On program successful completion, we expect ProgramOK + WSMReady
	t.Run("status after Program completed", func(t *testing.T) {
		if expected, got := StatusProgramOK|StatusWSMReady, status&(StatusProgramOK|StatusWSMReady); expected != got {
			t.Errorf("unexpected status: %02x != %02x", expected, got)
		}
	})

	// Clearing status also works with any value and any offset, not just 0 (as done in clearStatus)
	// (only for MN63F8MPN)
	f.mustWriteIO(t, 0x0, 0x0)

	t.Run("status after clear", func(t *testing.T) {
		status, err := f.status()
		if err != nil {
			t.Fatal(err)
		}
		if expected, got := StatusWSMReady, status&StatusWSMReady; expected != got {
			t.Errorf("unexpected status: %02x != %02x", expected, got)
		}
	})

	t.Run("dump internal page content", func(t *testing.T) {
		f.mustWriteCIR(t, cmdLoadBytePage)
		t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, pageSize)))
		f.mustWriteCIR(t, cmdReadArray)
		f.mustWriteCIR(t, cmdReadArray)
		f.mustWriteCIR(t, cmdReadArray)
	})

	// Verify that page has been programmed
	pageData, elapsed := func() (data []byte, elapsed time.Duration) {
		defer func(now time.Time) { elapsed = time.Since(now) }(time.Now())
		var err error

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		data, err = f.ReadPages(ctx, page, page+1)
		if err != nil {
			t.Fatal(err)
		}

		return
	}()
	// around 290µs (MN63F8MPN)
	t.Logf("reading page took %s", elapsed)

	t.Run("page data is programmed", func(t *testing.T) {
		if expected, got := expectedPageData, pageData; !bytes.Equal(expected, got) {
			t.Errorf("expected:\n%s\ngot:\n%s", hexDump(expectedPageData), hexDump(pageData))
		}
	})

	t.Run("internal page is erased after programming - internal page can be read using IO", func(t *testing.T) {
		f.mustWriteCIR(t, cmdLoadBytePage)
		for k := 0; k < pageSize; k += 4 {
			expected, got := uint32(0xffffffff), f.mustReadIO(t, ultrasave.PiAddress(k))
			if expected != got {
				t.Errorf("unexpected value @%05x after programming: %08x != %08x", k, expected, got)
			}
		}
	})

	// Restore backup data
	f.Write(context.Background(), f.layout.PageSize()*int(sectorFirstPage), backupData)

	/*
		Internal page content seems persisted until programmed or power off (tested only on MX29L1100)
				// Put some non "erased" bits to see if it's persisted across runs
				if err := f.LoadBytePage(backupData[0:pageSize]); err != nil {
					t.Fatal(err)
				}

				t.Run("dump internal page content", func(t *testing.T) {
					f.mustWriteCIR(t, cmdLoadBytePage)
					t.Logf("\n%s", hexDump(f.mustReadBurst(t, 0, pageSize)))
				})
	*/
}

func TestFullChipReadEraseProgram(t *testing.T) {
	skipIfNoBackup(t)

	f := setup(t)

	firstPage, lastPage := Page(0), Page(f.layout.TotalPages())

	// Backup data
	backupData, elapsed := func() (data []byte, elapsed time.Duration) {
		defer func(now time.Time) { elapsed = time.Since(now) }(time.Now())
		var err error

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		data, err = f.ReadPages(ctx, firstPage, lastPage)
		if err != nil {
			t.Fatal(err)
		}

		return
	}()
	// around 46ms
	t.Logf("reading chip took %s", elapsed)

	// Ensure WSM is ready before starting erase command
	if status, err := f.Status(); err != nil {
		t.Fatal(err)
	} else {
		if expected, got := StatusWSMReady, status&StatusWSMReady; expected != got {
			t.Fatalf("unexpected status: %02x != %02x", expected, got)
		}
	}

	// Full chip erase
	f.mustWriteCIR(t, cmdChipErase)
	f.mustWriteCIR(t, cmdErase)

	status, elapsed := func() (status Status, elapsed time.Duration) {
		defer func(now time.Time) { elapsed = time.Since(now) }(time.Now())
		var err error

		// Status register can be read directly after writing erase command
		// without having to explicitly write cmdStatus to CIR.
		// At this point status should equal EraseBusy (eg. WSMReady cleared, EraseBusy set).
		t.Run("status after start Erase", func(t *testing.T) {
			if status, err = f.status(); err != nil {
				t.Fatal(err)
			} else {
				if status == StatusEraseOK|StatusWSMReady {
					t.Skip("operation finished before our measurement")
				}

				if expected, got := StatusEraseBusy, status; expected != got {
					t.Errorf("unexpected status: %02x != %02x", expected, got)
				}
			}
		})

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		status, err = f.waitStatusBitsCleared(ctx, StatusEraseBusy, 5*time.Millisecond, 500*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}

		return
	}()

	// Erasing the chip should take around 295ms (MN63F8MPN) / 85ms (MX29L1100) (caveat: this include USB latency)
	t.Logf("erase chip took %s", elapsed)
	if expectedDuration := 300 * time.Millisecond; (elapsed - expectedDuration).Abs() > expectedDuration/10 {
		t.Logf("warning: erase operation duration is not within %s +- 10%%", expectedDuration)
	}

	// On erase successful completion, we expect EraseOK + WSMReady
	t.Run("status after Erase completed", func(t *testing.T) {
		if expected, got := StatusEraseOK|StatusWSMReady, status&(StatusEraseOK|StatusWSMReady); expected != got {
			t.Errorf("unexpected status: %02x != %02x", expected, got)
		}
	})

	// Clearing status also works with any value and any offset, not just 0 (as done in clearStatus)
	// (MN63F8MPN only)
	f.mustWriteIO(t, 0, 0)

	t.Run("status after clear", func(t *testing.T) {
		status, err := f.status()
		if err != nil {
			t.Fatal(err)
		}
		if expected, got := StatusWSMReady, status&StatusWSMReady; expected != got {
			t.Errorf("unexpected status: %02x != %02x", expected, got)
		}
	})

	// Verify that chip has been erased
	erasedData, elapsed := func() (data []byte, elapsed time.Duration) {
		defer func(now time.Time) { elapsed = time.Since(now) }(time.Now())
		var err error

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		data, err = f.ReadPages(ctx, firstPage, lastPage)
		if err != nil {
			t.Fatal(err)
		}

		return
	}()
	// around 46ms
	t.Logf("reading chip took %s", elapsed)

	t.Run("chip data is erased", func(t *testing.T) {
		for i, v := range erasedData {
			if v != byte(0xff) {
				t.Errorf("unexpected value @%05x after erase: 0xff != %02x", i, v)
				t.Logf("data:\n%s", hexDump(erasedData))
				break
			}
		}
	})

	// Reprogram pages
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	elapsed = func() (elapsed time.Duration) {
		defer func(now time.Time) { elapsed = time.Since(now) }(time.Now())
		pageSize := f.layout.PageSize()
		idx := 0
		for p := firstPage; p < lastPage; p++ {
			t.Logf("programming page %d", p)
			if err := f.LoadBytePage(backupData[idx : idx+pageSize]); err != nil {
				t.Fatal(err)
			}

			if err := f.ProgramPage(ctx, p); err != nil {
				t.Fatal(err)
			}

			idx += pageSize
		}

		return
	}()
	// around 4-5s
	t.Logf("reprogramming chip took %s", elapsed)

	// Verify that chip has been restored
	restoredData, elapsed := func() (data []byte, elapsed time.Duration) {
		defer func(now time.Time) { elapsed = time.Since(now) }(time.Now())
		var err error

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		data, err = f.ReadPages(ctx, firstPage, lastPage)
		if err != nil {
			t.Fatal(err)
		}

		return
	}()
	// around 46ms
	t.Logf("reading chip took %s", elapsed)

	t.Run("chip data is restored", func(t *testing.T) {
		got := restoredData
		expected := backupData
		if !bytes.Equal(expected, got) {
			t.Errorf("expected:\n%s\ngot:\n%s", hexDump(expected), hexDump(got))
		}
	})
}
