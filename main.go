package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"

	"github.com/c2h5oh/datasize"
	"github.com/rasky/g64drive/drive64"
	"github.com/schollz/progressbar/v2"
	"github.com/spf13/cobra"
)

var (
	flagVerbose   bool
	flagOffset    sizeUnit
	flagSize      sizeUnit
	flagBank      string
	flagQuiet     bool
	flagByteswapD int
	flagByteswapU int
)

type sizeUnit struct {
	size int64
}

func (s *sizeUnit) String() string {
	return fmt.Sprintf("%v", s.size)
}
func (s *sizeUnit) Set(text string) error {
	if sz, err := strconv.ParseInt(text, 64, 0); err == nil {
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
	case "pokemon":
		return drive64.BankPOKEMON, nil
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
	devices := drive64.Enumerate()

	if len(devices) == 0 {
		return errors.New("no 64drive devices found")
	}

	printf("Found %d 64drive device(s):\n", len(devices))
	for i, d := range devices {
		printf(" * %d: %v %v (serial: %v)\n", i, d.Manufacturer, d.Description, d.Serial)
		if flagVerbose {
			if dev, err := d.Open(); err == nil {
				if hwver, fwver, _, err := dev.CmdVersionRequest(); err == nil {
					printf("   -> Hardware: %v, Firmware: %v", hwver, fwver)
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
	if size < 0 || size%512 != 0 {
		return errors.New("invalid size value")
	}
	if size == 0 {
		fi, err := f.Stat()
		if err != nil {
			return err
		}
		size = fi.Size()
	}
	vprintf("size: %v\n", size)

	var offset = uint32(flagOffset.size)
	vprintf("offset: %v\n", offset)

	var pbw io.Writer
	pbw = os.Stdout
	if flagQuiet {
		pbw = ioutil.Discard
	}
	pb := progressbar.NewOptions64(size,
		progressbar.OptionSetDescription(filepath.Base(args[0])),
		progressbar.OptionSetWriter(pbw))

	pr, pw := io.Pipe()
	go func() {
		_, ioerr := io.Copy(io.MultiWriter(pw, pb), f)
		pw.CloseWithError(ioerr)
	}()

	return safeSigIntContext(func(ctx context.Context) error {
		return dev.CmdUpload(ctx, pr, size, bank, offset, bs)
	})
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
	if size < 0 || size%512 != 0 {
		return errors.New("invalid size value")
	}
	vprintf("size: %v\n", size)

	var offset = uint32(flagOffset.size)
	vprintf("offset: %v\n", offset)

	var pbw io.Writer
	pbw = os.Stdout
	if flagQuiet {
		pbw = ioutil.Discard
	}
	pb := progressbar.NewOptions64(int64(size),
		progressbar.OptionSetDescription(filepath.Base(args[0])),
		progressbar.OptionSetWriter(pbw))

	return safeSigIntContext(func(ctx context.Context) error {
		return dev.CmdDownload(ctx, io.MultiWriter(f, pb), size, bank, offset, bs)
	})
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
	cmdUpload.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "be verbose")
	cmdUpload.Flags().IntVarP(&flagByteswapU, "byteswap", "w", -1, "byteswap format: 0=none, 2=16bit, 4=32bit, -1=autodetect")

	var cmdDownload = &cobra.Command{
		Use:          "download [file]",
		Aliases:      []string{"d"},
		Short:        "download data from 64drive",
		Long:         `Download a binary file from 64drive, on the specified bank`,
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

	var rootCmd = &cobra.Command{
		Use: "g64drive",
	}
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "do not show any output unless an error occurs")
	rootCmd.AddCommand(cmdList, cmdUpload, cmdDownload)
	rootCmd.Execute()
}
