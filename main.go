package main

import (
	"errors"
	"fmt"

	"github.com/rasky/g64drive/drive64"
	"github.com/spf13/cobra"
)

var (
	flagVerbose bool
)

func cmdList(cmd *cobra.Command, args []string) error {
	devices := drive64.Enumerate()

	if len(devices) == 0 {
		return errors.New("no 64drive devices found")
	}

	fmt.Printf("Found %d 64drive device(s):\n", len(devices))
	for i, d := range devices {
		fmt.Printf(" * %d: %v %v (serial: %v)\n", i, d.Manufacturer, d.Description, d.Serial)
		if flagVerbose {
			if dev, err := d.Open(); err == nil {
				if hwver, fwver, _, err := dev.SendCmdVersionRequest(); err == nil {
					fmt.Printf("   -> Hardware: %v, Firmware: %v", hwver, fwver)
				}
			}
		}
	}

	return nil
}

func cmdUpload(cmd *cobra.Command, args []string) error {
	return nil
}

func main() {
	var cmdList = &cobra.Command{
		Use:          "list",
		Short:        "List 64drive devices",
		Long:         `List all the 64drive devices attached to this computer. It can be used to make sure that a device is online`,
		RunE:         cmdList,
		SilenceUsage: true,
	}
	cmdList.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "also show hardware/firmware version of each board")

	var cmdUpload = &cobra.Command{
		Use:          "upload [file]",
		Short:        "upload data to 64drive",
		Long:         `Upload a binary file to 64drive, on the specified bank`,
		RunE:         cmdUpload,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
	}
	cmdUpload.Flags().IntP("offset", "o", 0, "offset in memory at which the file will be uploaded")
	cmdUpload.Flags().IntP("size", "s", 0, "size of data to upload (default: file size)")

	var rootCmd = &cobra.Command{Use: "g64drive"}
	rootCmd.AddCommand(cmdList, cmdUpload)
	rootCmd.Execute()
}
