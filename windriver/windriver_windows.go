package windriver

/*
#include <stdio.h>
#include <libwdi.h>
#cgo windows CFLAGS: -Ilibwdi/include
#cgo windows LDFLAGS: ${SRCDIR}/libwdi/lib/libwdi.a -lsetupapi -lole32

#define INF_NAME "g64drive-winusb.inf"

BOOL is_64drive(struct wdi_device_info *device) {
	if (device->vid != 0x0403) return FALSE;
	if (device->pid < 0x6010 || device->pid > 0x6014) return FALSE;
	if (!strstr(device->desc, "64drive")) return FALSE;
	return TRUE;
}

void libwdi_search(int *ndevs, int *ninst) {
	struct wdi_options_create_list opts = { .list_all = TRUE };
	struct wdi_device_info *device, *list;

	*ndevs = 0;
	*ninst = 0;
	if (wdi_create_list(&list, &opts) == WDI_SUCCESS) {
		for (device = list; device != NULL; device = device->next) {
			if (is_64drive(device)) {
				*ndevs += 1;
				if (stricmp(device->driver, "WINUSB") != 0) *ninst += 1;
			}
		}
		wdi_destroy_list(list);
	}
}

void libwdi_install(const char *tempdir) {
	struct wdi_options_create_list opts = { .list_all = TRUE };
	struct wdi_device_info *device, *list;

	if (wdi_create_list(&list, &opts) == WDI_SUCCESS) {
		for (device = list; device != NULL; device = device->next) {
			if (!is_64drive(device)) continue;

			printf("Installing driver for USB device: \"%s\" (%04X:%04X)\n",
				device->desc, device->vid, device->pid);
			printf("Press Y to proceed...");
			char ch; scanf("%c", &ch);
			if (ch != 'y' && ch != 'Y') { printf("\n"); continue; }

			printf("Preparing driver...\n");
			struct wdi_options_prepare_driver popts = {
				.driver_type = WDI_WINUSB,
				.vendor_name = "Retroactive",
			};
			enum wdi_error err = wdi_prepare_driver(device, tempdir, INF_NAME, &popts);
			if (err != WDI_SUCCESS) {
				printf("ERROR during driver preparation: %s\n\n", wdi_strerror(err));
				continue;
			}

			printf("I will proceed installing the driver now.\n");
			printf("The process can take up to 5 (five) minutes. Please be VERY patient.\n\n");
			printf("Installing driver...\n");
			struct wdi_options_install_driver iopts = {
				.pending_install_timeout = 300000,
			};
			err = wdi_install_driver(device, tempdir, INF_NAME, &iopts);
			if (err != WDI_SUCCESS) {
				printf("\nERROR during driver installation: %s\n\n", wdi_strerror(err));
				continue;
			} else {
				printf("\nDriver installation suceeded.\n");
			}
			printf("Press ENTER to continue...");
			scanf("%c", &ch);
		}
		wdi_destroy_list(list);
	}
}
*/
import "C"

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

func Search() bool {
	var ndevs, ninst C.int
	C.libwdi_search(&ndevs, &ninst)
	if ndevs == 0 {
		fmt.Println("ERROR: no 64drive USB device found")
		fmt.Println("Make sure your 64drive is connected via USB to this PC.")
		return false
	}
	if ninst == 0 {
		fmt.Println("The correct driver for 64drive is already installed")
		return false
	}
	return true
}

func Elevate() {
	verb := "runas"
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	args := strings.Join(os.Args[1:], " ") + " ELEVATED_SKIP"

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(args)

	var showCmd int32 = 1 //SW_NORMAL

	err := windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
	if err != nil {
		fmt.Printf("ERROR: cannot re-run with elevated privileges: %s\n", err)
		fmt.Printf("Please try running from an administration prompt\n")
	}
}

func Install() error {
	tdir, err := ioutil.TempDir("", "g64drive")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tdir)

	C.libwdi_install(C.CString(tdir))
	time.Sleep(1 * time.Second)
	return nil
}
