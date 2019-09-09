## g64drive - a Linux/Mac tool for operating 64drive by Retroactive

### Installation (binary)

g64drive ships as a static binary on both Linux and macOS, with no additional dependencies
required. Just download it and put it in your `PATH` (eg: `/usr/local/bin`):

 * Download [Linux 64-bit binary](https://github.com/rasky/g64drive/releases/download/v0.1/g64drive-linux64.binary)
 * Download [macOS 64-bit binary](https://github.com/rasky/g64drive/releases/download/v0.1/g64drive-mac.binary)

### Installation (source)

Make sure you have:

 * Go 1.12 or newer
 * libftdi1-dev

installed on your system. Then, to download and build g64drive from source code, simply run:

```
    $ GO111MODULE=on go build github.com/rasky/g64drive
```

### Usage quicksheet

Make sure you can reach your 64drive:
```
    $ g64drive list -v
    Found 1 64drive device(s):
    * 0: Retroactive 64drive USB device (serial: RA3B53SW)
      -> Hardware: HW2 (Rev B), Firmware: 2.05
```

Upload a ROM to the CARTROM bank (with byteswap and CIC type autodetection):
```
    $ g64drive upload myrom.v64 -v
    64drive serial: RA3B53SW
    upload bank: BankCARTROM
    byteswap: 2
    size: 33554432
    offset: 0
    myrom.z64 100% |████████████████████████████████████████|  [1s:0s]
    Autoset CIC type: CIC6102
```

Download data from the CARTROM bank:
```
    $ g64drive download myrom.v64 -v --size 32M
    64drive serial: RA3B53SW
    download bank: BankCARTROM
    byteswap: 0
    size: 33554432
    offset: 0
    myrom.z64 100% |████████████████████████████████████████|  [1s:0s]
```

See firmware pack information:

```
    $ g64drive firmware info 64drive_firm_hw2_205.rpk
    Key                | Value
    ---------------------------------------------------------------------------------
    Copyright          | (c) 2018 Retroactive LLC
    Date               | 2018-01-04
    File               | firmware.bin
    Type               | Firmware
    Product            | 64drive
    Device             | EP4CE10F17
    Device Magic       | UDEV
    Device Variant     | B
    Content Version    | 2.05
    Prerequisites      |
    Content Note       | Adds support for USB communication from N64, rewritable UFLC
                       | boards.
    Content Changes    | 1. Block-based USB communication pipe is now implemented,
                       | see Hardware Spec
                       | 2. Added standalone commands to allow read/write of UFLC
                       | boards intended for UltraHDMI upgrade distribution
    Content Errata     |
    Content Extra      |
```

Upgrade firmware:

```
    $ g64drive firmware upgrade 64drive_firm_hw2_205.rpk
    Ready to upgrade 64drive (serial RA3B53SW)
    Current firmware: 2.04
    New firmware 2.05 (2018-01-04) - Adds support for USB communication from N64, rewritable UFLC boards.
    Do you want to proceed (Y/N):y
    Finished 100% |████████████████████████████████████████|  [23s:0s]
    Firmware upgraded correctly -- power-cycle your 64drive unit
```

### Features

 * Support 64drive HW1 and HW2
 * Upload and download data from any available bank
 * Transparent byteswapping (with autodetection from ROM header)
 * Transparent CIC detection when uploading a ROM, or later at any time
 * Can specify sizes and offsets in decimal, hex, or even kilobytes/megabytes
 * Firmware upgrades (flashing `.rpk` file as distributed by Retroactive)
 * CTRL+C clean shutdown during upload/download -- don't need to power-cycle 64drive after it
 * Shipped as static binary, very easy to install on any Linux and macOS system

What's missing:
 * Standalone mode

### Bugs?

Please [file an issue](https://github.com/rasky/g64drive/issues/new) on GitHub.
