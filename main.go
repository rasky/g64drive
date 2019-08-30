package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/rasky/g64drive/drive64"
	"github.com/schollz/progressbar/v2"
	"github.com/spf13/cobra"
)

var (
	flagVerbose bool
	flagOffset  uint
	flagSize    uint
	flagBank    string
	flagQuiet   bool
)

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

func cmdList(cmd *cobra.Command, args []string) error {
	devices := drive64.Enumerate()

	if len(devices) == 0 {
		return errors.New("no 64drive devices found")
	}

	if !flagQuiet {
		fmt.Printf("Found %d 64drive device(s):\n", len(devices))
	}
	for i, d := range devices {
		if !flagQuiet {
			fmt.Printf(" * %d: %v %v (serial: %v)\n", i, d.Manufacturer, d.Description, d.Serial)
		}
		if flagVerbose {
			if dev, err := d.Open(); err == nil {
				if hwver, fwver, _, err := dev.CmdVersionRequest(); err == nil {
					if !flagQuiet {
						fmt.Printf("   -> Hardware: %v, Firmware: %v", hwver, fwver)
					}
				} else {
					return err
				}
				dev.Close()
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

	bank, err := flagBankParse()
	if err != nil {
		return err
	}

	var magic [4]byte
	f.Read(magic[:])
	bs, err := drive64.ByteSwapDetect(magic[:])
	if err != nil {
		return err
	}
	f.Seek(0, io.SeekStart)

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	var pbw io.Writer
	pbw = os.Stdout
	if flagQuiet {
		pbw = ioutil.Discard
	}
	pb := progressbar.NewOptions64(fi.Size(),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetDescription(filepath.Base(args[0])),
		progressbar.OptionSetWriter(pbw))

	pr, pw := io.Pipe()
	go func() {
		_, ioerr := io.Copy(io.MultiWriter(pw, pb), f)
		pw.CloseWithError(ioerr)
	}()

	//dev.IdealChunkSize(fi.Size())
	return dev.CmdUpload(pr, 512*1024, bank, bs)
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
	cmdUpload.Flags().UintVarP(&flagOffset, "offset", "o", 0, "offset in memory at which the file will be uploaded")
	cmdUpload.Flags().UintVarP(&flagSize, "size", "s", 0, "size of data to upload (default: file size)")
	cmdUpload.Flags().StringVarP(&flagBank, "bank", "b", "rom", "bank where data should be uploaded (default: rom)")

	var rootCmd = &cobra.Command{Use: "g64drive"}
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "do not show any output unless an error occurs")
	rootCmd.AddCommand(cmdList, cmdUpload)
	rootCmd.Execute()
}
