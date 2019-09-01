## g64drive - a Linux/Mac tool for operating 64drive by Retroactive

### Installation (binary)

g64drive ships as a static binary on both Linux and macOS, with no additional dependencies
required. Just download it and put it in your `PATH` (eg: `/usr/local/bin`) .

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

### Features

 * Support 64drive HW1 and HW2
 * Upload and download data from any available bank
 * Transparent byteswapping (with autodetection from ROM header)
 * Transparent CIC detection when uploading a ROM
 * Can specify sizes and offsets in decimal, hex, or even kilobytes/megabytes
 * CTRL+C clean shutdown during upload/download -- doesn't need to power-cycle 64drive after it
 * Shipped as static binary, very easy to install on any Linux and macOS system

What's missing:
 * Firmware upgrades
 * Standalone mode

### Bugs?

Please [file an issue](https://github.com/rasky/g64drive/issues/new) on GitHub.
