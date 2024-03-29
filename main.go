package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/rasky/g64drive/drive64"
	"github.com/rasky/g64drive/windriver"
	"github.com/schollz/progressbar/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	flagVerbose      bool
	flagOffset       sizeUnit
	flagSize         sizeUnit
	flagAutoCic      bool
	flagAutoSave     bool
	flagAutoExtended bool
	flagBank         string
	flagQuiet        bool
	flagByteswapD    int
	flagByteswapU    int
	flagFwExtractOut string

	pflagAutoCic      *pflag.Flag
	pflagAutoSave     *pflag.Flag
	pflagAutoExtended *pflag.Flag
)

type sizeUnit struct {
	size int64
}

func (s *sizeUnit) String() string {
	return fmt.Sprintf("%v", s.size)
}
func (s *sizeUnit) Set(text string) error {
	if sz, err := strconv.ParseInt(text, 0, 64); err == nil {
		s.size = sz
		return nil
	}

	var v datasize.ByteSize
	if err := v.UnmarshalText([]byte(text)); err == nil {
		s.size = int64(v.Bytes())
		return nil
	}

	return errors.New("invalid size")
}
func (s *sizeUnit) Type() string {
	return "int64"
}

func printf(s string, args ...interface{}) {
	if !flagQuiet {
		fmt.Printf(s, args...)
	}
}
func vprintf(s string, args ...interface{}) {
	if flagVerbose {
		printf(s, args...)
	}
}

func flagBankParse() (drive64.Bank, error) {
	switch flagBank {
	case "rom":
		return drive64.BankCARTROM, nil
	case "sram256":
		return drive64.BankSRAM256, nil
	case "sram768":
		return drive64.BankSRAM768, nil
	case "flash":
		return drive64.BankFLASH, nil
	case "flash_pokstad2":
		return drive64.BankFLASH_POKSTAD2, nil
	case "eeprom":
		return drive64.BankEEPROM, nil
	default:
		return drive64.BankCARTROM, fmt.Errorf("invalid bank: %v", flagBank)
	}
}

// safeSigIntContext executes function f with a context which is canceled when CTRL+C is called.
// This allows to for safe CTRL+C cancelation for functions that can't be aborted at any moment.
func safeSigIntContext(f func(ctx context.Context) error) error {
	ctx := context.Background()

	// trap Ctrl+C and call cancel on the context
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()

	err := f(ctx)
	if err == context.Canceled {
		err = errors.New("SIGINT caught, exiting")
	}
	return err
}

func cmdList(cmd *cobra.Command, args []string) error {
	devices, unk := drive64.Enumerate()

	if len(devices) == 0 {
		if unk {
			return drive64.ErrUnknownDevice
		}
		return errors.New("no 64drive devices found")
	}

	printf("Found %d 64drive device(s):\n", len(devices))
	for i, d := range devices {
		printf(" * %d: %v %v (serial: %v)\n", i, d.Manufacturer, d.Description, d.Serial)
		if flagVerbose {
			if dev, err := d.Open(); err == nil {
				if hwver, fwver, _, err := dev.CmdVersionRequest(); err == nil {
					printf("   -> Hardware: %v, Firmware: %v\n", hwver, fwver)
				} else {
					return err
				}
				dev.Close()
			} else {
				return err
			}
		}
	}

	return nil
}

func download(dev *drive64.Device, w io.Writer, size int64, bank drive64.Bank, offset uint32, pbdesc string) error {
	var pbw io.Writer
	pbw = os.Stdout
	if flagQuiet {
		pbw = ioutil.Discard
	}
	pb := progressbar.NewOptions64(int64(size),
		progressbar.OptionSetDescription(pbdesc),
		progressbar.OptionSetWriter(pbw))

	return safeSigIntContext(func(ctx context.Context) error {
		defer fmt.Println()
		return dev.CmdDownload(ctx, io.MultiWriter(w, pb), size, bank, offset)
	})
}

func upload(dev *drive64.Device, r io.Reader, size int64, bank drive64.Bank, offset uint32, pbdesc string) error {
	var pbw io.Writer
	pbw = os.Stdout
	if flagQuiet {
		pbw = ioutil.Discard
	}
	pb := progressbar.NewOptions64(size,
		progressbar.OptionSetDescription(pbdesc),
		progressbar.OptionSetWriter(pbw))

	pr, pw := io.Pipe()
	go func() {
		_, ioerr := io.Copy(io.MultiWriter(pw, pb), r)
		pw.CloseWithError(ioerr)
	}()

	return safeSigIntContext(func(ctx context.Context) error {
		defer fmt.Println()
		return dev.CmdUpload(ctx, pr, size, bank, offset)
	})
}

func upgradeFirmware(dev *drive64.Device, rpk *drive64.RPK) error {
	if err := safeSigIntContext(func(ctx context.Context) error {
		// Upload firmware asset to CARTROM
		vprintf("Uploading firmware\n")
		if err := dev.CmdUpload(ctx, bytes.NewReader(rpk.Asset), int64(len(rpk.Asset)), drive64.BankCARTROM, 0); err != nil {
			return err
		}

		// Download firmware asset, compare CRC32, and verify that it's not corrupted
		vprintf("Verifying firmware\n")
		crc := crc32.NewIEEE()
		if err := dev.CmdDownload(ctx, crc, int64(len(rpk.Asset)), drive64.BankCARTROM, 0); err != nil {
			return err
		}
		if crc.Sum32() != crc32.ChecksumIEEE(rpk.Asset) {
			return errors.New("firmware transfer failed - 64drive SDRAM failure?")
		}

		if stat, err := dev.CmdUpgradeReport(); err != nil {
			return err
		} else if stat != drive64.UpgradeReady {
			return fmt.Errorf("upgrade module is not ready (%v) -- try power-cycling your 64drive unit", stat)
		}

		return nil

	}); err != nil {
		return err
	}

	_, swver, _, err := dev.CmdVersionRequest()
	if err != nil {
		return err
	}

	fmt.Printf("Ready to upgrade 64drive (serial %v)\n", dev.Description().Serial)
	fmt.Printf("Current firmware: %v\n", swver)
	fmt.Printf("New firmware %v (%v) - %v\n", rpk.Metadata.ContentVersionText, rpk.Metadata.Date, rpk.Metadata.ContentNote)
	fmt.Printf("Do you want to proceed (Y/N):")
	var resp string
	if _, err := fmt.Scanln(&resp); err != nil || strings.ToLower(resp) != "y" {
		return nil
	}

	if err := dev.CmdUpgradeStart(); err != nil {
		return err
	}

	pb := progressbar.NewOptions64(10, progressbar.OptionSetDescription("Upgrading"))
	pbidx := 0
	curstat := drive64.UpgradeReady
	for !curstat.IsFinished() {
		stat, err := dev.CmdUpgradeReport()
		if err == nil && stat != curstat {
			newidx := pbidx
			var pbdesc string
			switch stat {
			case drive64.UpgradeVerifying:
				newidx = 1
				pbdesc = "Verifying"
			case drive64.UpgradeErasing00:
				newidx = 2
				pbdesc = "Erasing"
			case drive64.UpgradeErasing25:
				newidx = 3
				pbdesc = "Erasing"
			case drive64.UpgradeErasing50:
				newidx = 4
				pbdesc = "Erasing"
			case drive64.UpgradeErasing75:
				newidx = 5
				pbdesc = "Erasing"
			case drive64.UpgradeWriting00:
				newidx = 6
				pbdesc = "Flashing"
			case drive64.UpgradeWriting25:
				newidx = 7
				pbdesc = "Flashing"
			case drive64.UpgradeWriting50:
				newidx = 8
				pbdesc = "Flashing"
			case drive64.UpgradeWriting75:
				newidx = 9
				pbdesc = "Flashing"
			case drive64.UpgradeSuccess:
				newidx = 10
				pbdesc = "Finished"
			}
			if newidx != pbidx {
				pb.Describe(pbdesc)
				pb.Add(newidx - pbidx)
				pbidx = newidx
			}

			curstat = stat
		}

		time.Sleep(100 * time.Millisecond)
	}

	pb.Finish()

	switch curstat {
	case drive64.UpgradeGeneralFail:
		return errors.New("Upgrade failed: general failure")
	case drive64.UpgradeBadVariant:
		return errors.New("Upgrade failed: wrong hardware variant")
	case drive64.UpgradeVerifyFail:
		return errors.New("Upgrade failed: firmware verification failure")
	case drive64.UpgradeSuccess:
		printf("\nFirmware upgraded correctly -- power-cycle your 64drive unit\n")
		return nil
	default:
		return fmt.Errorf("unexpected upgrade status: %v", curstat)
	}
}

func cmdUpload(cmd *cobra.Command, args []string) error {
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()

	dev, err := drive64.NewDeviceSingle()
	if err != nil {
		return err
	}
	defer dev.Close()
	vprintf("64drive serial: %v\n", dev.Description().Serial)

	bank, err := flagBankParse()
	if err != nil {
		return err
	}
	vprintf("upload bank: %v\n", bank)

	var bs drive64.ByteSwapper
	if flagByteswapU < 0 {
		var magic [4]byte
		f.ReadAt(magic[:], 0)
		bs, err = drive64.ByteSwapDetect(magic[:])
		if err != nil {
			return err
		}
	} else if flagByteswapU == 0 || flagByteswapU == 2 || flagByteswapU == 4 {
		bs = drive64.ByteSwapper(flagByteswapU)
	} else {
		return errors.New("invalid byteswap value")
	}
	vprintf("byteswap: %v\n", bs)

	size := flagSize.size
	if size < 0 {
		return errors.New("invalid size value (negative number")
	}
	if size%512 != 0 {
		return errors.New("invalid size value (must be multiple of 512)")
	}
	if size == 0 {
		fi, err := f.Stat()
		if err != nil {
			return err
		}
		size = fi.Size()
	}
	vprintf("size: %v\n", size)

	offset := uint32(flagOffset.size)
	vprintf("offset: %v\n", offset)

	// --autocic defaults to true when uploading a ROM to CARTROM at offset 0
	if !pflagAutoCic.Changed && bank == drive64.BankCARTROM && offset == 0 {
		flagAutoCic = true
	}
	if !pflagAutoSave.Changed && bank == drive64.BankCARTROM && offset == 0 {
		flagAutoSave = true
	}

	// --extended defaults to true when uploading a ROM to CARTROM at offset 0, if the ROM is larger than 64MB
	// Otherwise, the ROM would uploaded correctly but the data would be inaccessible.
	if !pflagAutoExtended.Changed && bank == drive64.BankCARTROM && offset == 0 && size > 64*1024*1024 {
		if hwvar, fwver, _, err := dev.CmdVersionRequest(); err == nil && hwvar == drive64.VarRevA {
			// Return an appropriate error message for HW1, taking into account that the user doesn't
			// probably know what extended mode is.
			return errors.New("ROMs larger than 64 MiB not supported on 64drive HW1")
		} else if fwver < 206 {
			return errors.New("ROMs larger than 64 MiB not supported on 64drive firmware < 2.06")
		}
		flagAutoExtended = true
	}

	if flagAutoExtended {
		vprintf("Set extended mode\n")
		if hwvar, fwver, _, err := dev.CmdVersionRequest(); err == nil && hwvar == drive64.VarRevA {
			return errors.New("extended mode not supported on 64drive HW1")
		} else if fwver < 206 {
			return errors.New("extended mode not supported on 64drive firmware < 2.06")
		} else if err := dev.CmdSetExtended(true); err != nil {
			return err
		}
	}

	vprintf("uploading\n")
	rommd5 := md5.New()
	if err := upload(dev, io.TeeReader(bs.NewReader(f), rommd5), size, bank, offset, filepath.Base(args[0])); err != nil {
		return err
	}

	if flagAutoCic {
		cic, err := cicAutodetect(dev)
		if err != nil {
			return err
		}
		vprintf("Autoset CIC type: %v\n", cic)

		if err := dev.CmdSetCicType(cic); err != nil {
			if err == drive64.ErrUnsupported {
				vprintf("Setting CIC not supported on 64drive HW1, skipping\n")
			} else {
				return err
			}
		}
	}

	if flagAutoSave {
		rommd5 := hex.EncodeToString(rommd5.Sum(nil))
		st := drive64.SaveNone
		game := romdb_search(rommd5)
		if game.Name != "" {
			vprintf("Detected game: %v\n", game.Name)
			switch game.SaveType {
			case "Eeprom 4KB":
				st = drive64.SaveEeprom4Kbit
			case "Eeprom 16KB":
				st = drive64.SaveEeprom16Kbit
			case "Flash RAM":
				st = drive64.SaveFlashRAM1Mbit
				// Special case: for Pokemon Stadium 2, 64drive HW1
				// needs a special save type. This happens because HW1 only
				// has 64Mb of RDRAM, and the ROM is 64Mb. Normally, the 1Mbit
				// is stolen at the end of the RDRAM/ROM but this specific game
				// has non-blank data at the end. So the 64drive firmware can use
				// a different (hardcoded) address where to put the save data,
				// overriding a portion that is known to be blank. Since this
				// address is hardcoded in the firmware, we cannot use it for
				// anything but this specific game.
				if strings.HasPrefix(game.Name, "Pokemon Stadium 2") {
					if hwvar, _, _, err := dev.CmdVersionRequest(); err == nil && hwvar == drive64.VarRevA {
						st = drive64.SaveFlashRAM1Mbit_PokStad2
					}
				}
			case "SRAM":
				st = drive64.SaveSRAM256Kbit
			}
		} else {
			// Download the header and see if it matches
			var header bytes.Buffer
			if err := dev.CmdDownload(context.Background(), &header, 512,
				drive64.BankCARTROM, 0); err != nil {
				vprintf("Error reading back ROM header: %v\n", err)
			} else {
				var buf = header.Bytes()
				if buf[0x3C] == 'E' && buf[0x3D] == 'D' {
					vprintf("ED64 ROM header detected\n")
					var cfg uint8 = buf[0x3F]
					switch cfg >> 4 {
					case 0:
						st = drive64.SaveNone
					case 1:
						st = drive64.SaveEeprom4Kbit
					case 2:
						st = drive64.SaveEeprom16Kbit
					case 3:
						st = drive64.SaveSRAM256Kbit
					case 4:
						st = drive64.SaveSRAM768Kbit
					case 5:
						st = drive64.SaveFlashRAM1Mbit
					case 6:
						fmt.Printf("WARNING: the ROM requested a 1Mbit SRAM savetype, which is not supported by 64drive\n")
						st = drive64.SaveNone
					default:
						vprintf("WARNING: invalid ED64 ROM confing header value: %02x\n", cfg)
					}
				}
			}
		}
		vprintf("Autoset save type: %v\n", st)
		if err := dev.CmdSetSaveType(st); err != nil {
			return err
		}
	}

	return nil
}

func cmdDownload(cmd *cobra.Command, args []string) error {
	dev, err := drive64.NewDeviceSingle()
	if err != nil {
		return err
	}
	defer dev.Close()
	vprintf("64drive serial: %v\n", dev.Description().Serial)

	bank, err := flagBankParse()
	if err != nil {
		return err
	}
	vprintf("download bank: %v\n", bank)

	var bs drive64.ByteSwapper
	if flagByteswapD == 0 || flagByteswapD == 2 || flagByteswapD == 4 {
		bs = drive64.ByteSwapper(flagByteswapD)
	} else {
		return errors.New("invalid byteswap value")
	}
	vprintf("byteswap: %v\n", bs)

	f, err := os.Create(args[0])
	if err != nil {
		return err
	}
	defer f.Close()

	size := flagSize.size
	if size < 0 {
		return errors.New("invalid size value (negative number")
	}
	vprintf("size: %v\n", size)

	var offset = uint32(flagOffset.size)
	vprintf("offset: %v\n", offset)

	return download(dev, bs.NewWriter(f), size, bank, offset, filepath.Base(args[0]))
}

func cicAutodetect(dev *drive64.Device) (drive64.CIC, error) {
	var header bytes.Buffer
	if err := dev.CmdDownload(context.Background(), &header, 0x1000,
		drive64.BankCARTROM, 0); err != nil {
		return 0, err
	}
	return drive64.NewCICFromHeader(header.Bytes())
}

func cmdCic(cmd *cobra.Command, args []string) error {
	var cic drive64.CIC
	if args[0] != "auto" {
		var err error
		if cic, err = drive64.NewCICFromString(args[0]); err != nil {
			return err
		}
	}

	dev, err := drive64.NewDeviceSingle()
	if err != nil {
		return err
	}
	defer dev.Close()

	if args[0] == "auto" {
		var err error
		if cic, err = cicAutodetect(dev); err != nil {
			return err
		}
	}

	vprintf("64drive serial: %v\n", dev.Description().Serial)
	vprintf("CIC type: %v\n", cic)

	return dev.CmdSetCicType(cic)
}

func cmdSaveType(cmd *cobra.Command, args []string) error {
	var savetype drive64.SaveType
	var err error
	if savetype, err = drive64.NewSaveTypeFromString(args[0]); err != nil {
		return err
	}

	dev, err := drive64.NewDeviceSingle()
	if err != nil {
		return err
	}
	defer dev.Close()

	vprintf("64drive serial: %v\n", dev.Description().Serial)
	vprintf("Save type: %v\n", savetype)

	return dev.CmdSetSaveType(savetype)

}

func cmdExtended(cmd *cobra.Command, args []string) error {
	dev, err := drive64.NewDeviceSingle()
	if err != nil {
		return err
	}
	defer dev.Close()

	var extended bool
	switch args[0] {
	case "t", "true", "1":
		extended = true
	}

	vprintf("64drive serial: %v\n", dev.Description().Serial)
	vprintf("Extended mode: %v\n", extended)

	if hwvar, fwver, _, err := dev.CmdVersionRequest(); err == nil && hwvar == drive64.VarRevA {
		return errors.New("extended mode not supported on 64drive HW1")
	} else if fwver < 206 {
		return errors.New("extended mode not supported on 64drive firmware < 2.06")
	} else {
		return dev.CmdSetExtended(extended)
	}
}

func fwCmd(filename string, cb func(rpk *drive64.RPK) error) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	rpk, err := drive64.NewRPKFromReader(f)
	if err != nil {
		return err
	}

	return cb(rpk)
}

func cmdFirmwareInfo(cmd *cobra.Command, args []string) error {
	return fwCmd(args[0], func(rpk *drive64.RPK) error {
		if !flagQuiet {
			rpk.DumpMetadata()
		}
		return nil
	})
}

func cmdFirmwareExtract(cmd *cobra.Command, args []string) error {
	return fwCmd(args[0], func(rpk *drive64.RPK) error {
		fn := rpk.Metadata.File
		if flagFwExtractOut != "" {
			fn = flagFwExtractOut
		}
		if err := ioutil.WriteFile(fn, rpk.Asset, 0666); err != nil {
			return err
		}
		printf("Written %q (%d bytes)", fn, len(rpk.Asset))
		return nil
	})
}

func cmdFirmwareUpgrade(cmd *cobra.Command, args []string) error {
	return fwCmd(args[0], func(rpk *drive64.RPK) error {
		switch rpk.Metadata.Type {
		case 2: // Firmware
		case 1:
			return errors.New("bootloader upgrade not yet implemented")
		default:
			return errors.New("unknown firmware type")
		}

		dev, err := drive64.NewDeviceSingle()
		if err != nil {
			return err
		}
		defer dev.Close()
		vprintf("64drive serial: %v\n", dev.Description().Serial)

		if hwvar, _, magic, err := dev.CmdVersionRequest(); err != nil {
			return err
		} else {
			if !bytes.Equal(magic[:], []byte(rpk.Metadata.Magic)[:4]) {
				return errors.New("firmware archive not meant for this device (different product)")
			}
			v := []byte(rpk.Metadata.Variant + "\000")[:2]
			if hwvar != drive64.Variant(binary.BigEndian.Uint16(v)) {
				return errors.New("firmware archive not meant for this device (different hardware variant)")
			}
		}

		switch rpk.Metadata.Type {
		case drive64.RPKAssetFirmware:
			return upgradeFirmware(dev, rpk)
		case drive64.RPKAssetBootloader:
			return errors.New("bootloader upgrade not implemented")
		default:
			return fmt.Errorf("unknown asset type: %s (%08x)", rpk.Metadata.TypeText, rpk.Metadata.Type)
		}
	})
}

func cmdDebug(cmd *cobra.Command, args []string) error {
	dev, err := drive64.NewDeviceSingle()
	if err != nil {
		return err
	}
	defer dev.Close()

	// Check firmware version and verify if it's new enough
	if _, fwver, _, err := dev.CmdVersionRequest(); err == nil {
		if fwver < 205 {
			return fmt.Errorf("\"g64drive debug\" requires 64drive firmware >= 2.05, found: %v\nDownload a newer firmware from http://64drive.retroactive.be, and then run \"g64drive firmware upgrade\" to upgrade", fwver)
		}
	}

	return safeSigIntContext(func(ctx context.Context) error {
		for ctx.Err() == nil {
			if typ, data, err := dev.CmdFifoRead(ctx); err != nil {
				// To allow running FIFO reads while a stream of data is already
				// in progress, errors are not blocking and do not print
				// header errors which is what we expect when we jump into the
				// middle of the stream
				if err != drive64.ErrInvalidFifoHead {
					fmt.Fprintf(os.Stderr, "%v\n", err)
				}
			} else {
				switch typ {
				case 1:
					// Since packets are padded to be aligned, text packets
					// might contain trailing zeros.
					data = bytes.TrimRight(data, "\000")
					fmt.Printf("%s", data)
				default:
					// ignoring unknown packet type
				}
			}
		}
		return ctx.Err()
	})
}

func cmdDriverInstall(cmd *cobra.Command, args []string) error {
	if !windriver.Search() {
		return nil
	}

	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		fmt.Println("Elevating to administrator privilege...")
		windriver.Elevate()
		return nil
	}

	return windriver.Install()
}

func main() {
	var cmdList = &cobra.Command{
		Use:          "list",
		Aliases:      []string{"l"},
		Short:        "List 64drive devices",
		Long:         `List all the 64drive devices attached to this computer. It can be used to make sure that a device is online`,
		RunE:         cmdList,
		SilenceUsage: true,
	}
	cmdList.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "also show hardware/firmware version of each board")

	var cmdUpload = &cobra.Command{
		Use:          "upload [file]",
		Aliases:      []string{"u"},
		Short:        "upload data to 64drive",
		Long:         `Upload a binary file to 64drive, on the specified bank`,
		RunE:         cmdUpload,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}
	cmdUpload.Flags().VarP(&flagOffset, "offset", "o", "offset in memory at which the file will be uploaded")
	cmdUpload.Flags().VarP(&flagSize, "size", "s", "size of data to upload (default: file size)")
	cmdUpload.Flags().StringVarP(&flagBank, "bank", "b", "rom", "bank where data should be uploaded")
	cmdUpload.Flags().BoolVarP(&flagAutoCic, "autocic", "c", false, "autoset CIC after upload (default: true if uploading a ROM)")
	cmdUpload.Flags().BoolVarP(&flagAutoSave, "autosave", "S", false, "autoset save type after upload (default: true if uploading a ROM)")
	cmdUpload.Flags().BoolVarP(&flagAutoExtended, "extended", "e", false, "set extended mode after upload (default: true if uploading a >64Mb ROM)")
	cmdUpload.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "be verbose")
	cmdUpload.Flags().IntVarP(&flagByteswapU, "byteswap", "w", -1, "byteswap format: 0=none, 2=16bit, 4=32bit, -1=autodetect")
	pflagAutoCic = cmdUpload.Flag("autocic")
	pflagAutoSave = cmdUpload.Flag("autosave")
	pflagAutoExtended = cmdUpload.Flag("extended")

	var cmdDownload = &cobra.Command{
		Use:     "download [file]",
		Aliases: []string{"d"},
		Short:   "download data from 64drive",
		Long: `Download a binary file from 64drive, on the specified bank.
Supported banks are: rom, sram256, sram768, flash, flash_pokstad2, eeprom.`,
		RunE:         cmdDownload,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}
	cmdDownload.Flags().VarP(&flagOffset, "offset", "o", "offset in memory at which the file will be uploaded")
	cmdDownload.Flags().VarP(&flagSize, "size", "s", "size of data to download")
	cmdDownload.Flags().StringVarP(&flagBank, "bank", "b", "rom", "bank where data should be uploaded")
	cmdDownload.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "be verbose")
	cmdDownload.Flags().IntVarP(&flagByteswapD, "byteswap", "w", 0, "byteswap format: 0=none, 2=16bit, 4=32bit")
	cmdDownload.MarkFlagRequired("size")

	var cmdCic = &cobra.Command{
		Use:     "cic [type]",
		Aliases: []string{"c"},
		Short:   "change the CIC emulated variant",
		Long: `Change the variant of CIC that the 64drive emulates, possibly autodetecting it from the current ROM header.
The variant type can be specified using its name, such as "6103". By specifying "auto", the current ROM header
will be transferred from 64drive and analyzed, and the correct CIC variant will be automatically selected.`,
		Example: `  g64drive cic 6105     
    -- sets CIC emulation to the 6105 variant.  

  g64drive cic auto
    -- autodetect and set CIC type from the currently-loaded ROM header.`,
		RunE:         cmdCic,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}
	cmdCic.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "be verbose")

	var cmdSaveType = &cobra.Command{
		Use:     "savetype [type]",
		Aliases: []string{"st"},
		Short:   "change the emulated save type",
		Long: `Change the variant of save memory that the 64drive emulates.
The save type can be specified using one of the following names:
"none", "eeprom4kbit", "eeprom16kbit", "sram256kbit", "flash1mbit", "sram768kbit", "flash1mbit_pokstad2".`,
		Example: `  g64drive savetype eeprom16kbit     
    -- sets save type emulation to EEPROM with 16Kbit of space.`,
		RunE:         cmdSaveType,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}
	cmdSaveType.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "be verbose")

	var cmdExtended = &cobra.Command{
		Use:     "extended [bool]",
		Aliases: []string{"ext"},
		Short:   "enabel or disabled the 64drive extended mode",
		Long: `Extended mode allows 64drive to expose the full 240 MiB of SDRAM as ROM to the N64.
This is required to run ROMs larger than 64 MiB, because otherwise the upper part of the ROM would not
be accessible. Please notice that extended mode is only available on 64Drive HW2 with firmware >= 2.06.`,
		Example: `  g64drive extended true     
    -- enable extended mode.`,
		RunE:         cmdExtended,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}
	cmdExtended.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "be verbose")

	var cmdFirmwareInfo = &cobra.Command{
		Use:   "info [file.rpk]",
		Short: "show information on 64drive firmware file",
		Example: `  g64drive firmware info 64drive_firm_hw2_205.rpk
	-- show information on the specified firwmare file.`,
		RunE:         cmdFirmwareInfo,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}

	var cmdFirmwareExtract = &cobra.Command{
		Use:   "extract [file.rpk]",
		Short: "extract the raw binary firmware",
		Long: `extract the raw binary firmware contained in the RPK firmware container.
By default, the original name is used (eg: firmware.bin), but a different file name can be specified`,
		Example: `  g64drive firmware extract 64drive_firm_hw2_205.rpk
	-- extract the raw binary firmware from the firmware container.`,
		RunE:         cmdFirmwareExtract,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}
	cmdFirmwareExtract.Flags().StringVarP(&flagFwExtractOut, "output", "o", "", "output file (default: original name)")

	var cmdFirmwareUpgrade = &cobra.Command{
		Use:   "upgrade [file.rpk]",
		Short: "upgrade 64drive firmware",
		Example: `  g64drive firmware upgrade 64drive_firm_hw2_205.rpk
	-- install the firmware upgrade.`,
		RunE:         cmdFirmwareUpgrade,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}

	var cmdFirmware = &cobra.Command{
		Use:   "firmware",
		Short: "manage firmware/bootloader upgrades",
	}
	cmdFirmware.AddCommand(cmdFirmwareUpgrade, cmdFirmwareInfo, cmdFirmwareExtract)
	cmdFirmware.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "be verbose")

	var cmdDebug = &cobra.Command{
		Use:   "debug",
		Short: "debug a running program using libdragon/UNFLoader protocol",
		Example: `  g64drive debug
	-- see the output of the program`,
		RunE: cmdDebug,
	}

	var cmdDriverInstall = &cobra.Command{
		Use:   "driverinstall",
		Short: "install Windows drivers for 64drive",
		Long: `This command will attempt to perform an automatic driver installation for a 64drive device.
Make sure 64drive is connected to the PC before running.
The driver that will be installed is the standard Microsoft WinUSB driver, which is used by g64drive.
No proprietary FTDI/D2XX is required and will not be installed.`,
		RunE: cmdDriverInstall,
	}

	var rootCmd = &cobra.Command{
		Use: "g64drive",
	}
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "do not show any output unless an error occurs")
	rootCmd.AddCommand(cmdList, cmdUpload, cmdDownload, cmdCic, cmdSaveType, cmdExtended, cmdFirmware, cmdDebug)
	if runtime.GOOS == "windows" {
		rootCmd.AddCommand(cmdDriverInstall)
	}
	if rootCmd.Execute() != nil {
		os.Exit(1)
	}
}
