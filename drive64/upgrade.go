package drive64

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/mitchellh/go-wordwrap"
	lzf "github.com/zhuyie/golzf"
	"gopkg.in/restruct.v1"
)

type blobHead struct {
	Magic    [2]byte
	Version  byte
	Compress byte
	Children uint32
	Length   uint32
	ULength  uint32
	Crc      uint32
}

type blob struct {
	Head     blobHead
	Body     []uint8
	Children []*blob
}

func newBlobFromReader(r io.Reader) (*blob, error) {
	// Read the blob header and check its integrity
	var h blobHead
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	if h.Magic[0] != 'P' || h.Version != '0' || (h.Compress != 'C' && h.Compress != 'U') {
		return nil, errors.New("invalid blob header in RPK")
	}

	// Read the full body (including children)
	body := make([]byte, h.Length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	if crc32.ChecksumIEEE(body) != h.Crc {
		return nil, errors.New("corrupted blob in RPK (invalid CRC)")
	}

	// Decompress the full body (if required)
	if h.Compress == 'C' {
		ubody := make([]byte, h.ULength)
		if n, err := lzf.Decompress(body, ubody); err != nil {
			return nil, err
		} else if n != int(h.ULength) {
			return nil, errors.New("invalid uncompressed size in blob")
		}
		body = ubody
	}

	// Recursively parse children blobs (if any)
	bodyR := bytes.NewReader(body)
	var children []*blob
	for c := uint32(0); c < h.Children; c++ {
		if b, err := newBlobFromReader(bodyR); err != nil {
			return nil, err
		} else {
			children = append(children, b)
		}
	}

	// Save in body what it is left from children parsing
	body, err := ioutil.ReadAll(bodyR)
	if err != nil {
		return nil, err
	}

	// Check end-of-blob marker
	var eob [4]byte
	if _, err := io.ReadFull(r, eob[:]); err != nil {
		return nil, err
	} else if string(eob[:]) != "EOB\000" {
		return nil, errors.New("invalid end-of-blob marker")
	}

	return &blob{h, body, children}, nil
}

type RPK struct {
	Metadata struct {
		Format      uint32
		Copyright   string `struct:"[64]byte"`
		Date        string `struct:"[32]byte"`
		File        string `struct:"[64]byte"`
		Type        uint32
		TypeText    string `struct:"[32]byte"`
		Product     string `struct:"[16]byte"`
		ProductText string `struct:"[64]byte"`
		Device      string `struct:"[32]byte"`
		Magic       string `struct:"[16]byte"`
		Variant     [8]byte

		ContentVersion        uint16
		ContentVersionSpecial uint8
		ContentVersionText    string `struct:"[16]byte"`
		Prerequisites         uint32 `struct:"skip=1"`
		PrerequisitesText     string `struct:"[128]byte"`
		ContentNote           string `struct:"[128]byte"`
		ContentChanges        string `struct:"[1024]byte"`
		ContentErrata         string `struct:"[128]byte"`
		ContentExtra          string `struct:"[128]byte"`
	}
	Asset []byte
}

func NewRPKFromReader(r io.Reader) (*RPK, error) {
	root, err := newBlobFromReader(r)
	if err != nil {
		return nil, err
	}

	if root.Head.Magic[1] != 'F' {
		return nil, errors.New("unexpected root blob with magic " + string(root.Head.Magic[:]))
	}

	rpk := new(RPK)
	foundMetadata := false
	foundAsset := false
	for _, c := range root.Children {
		switch c.Head.Magic[1] {
		case 'M':
			if foundMetadata {
				return nil, errors.New("duplicate metadata found")
			}
			foundMetadata = true
			if len(c.Children) != 0 {
				return nil, errors.New("unexpected metadata children blobs")
			}
			if err := restruct.Unpack(c.Body, binary.LittleEndian, &rpk.Metadata); err != nil {
				return nil, err
			}
		case 'A':
			if foundAsset {
				return nil, errors.New("duplicate asset found")
			}
			foundAsset = true
			if len(c.Children) != 0 {
				return nil, errors.New("unexpected asset children blobs")
			}
			rpk.Asset = c.Body
		}
	}

	return rpk, nil
}

func (rpk *RPK) DumpMetadata() {
	w := os.Stdout
	const sfmt = "%-18s | "
	const sep = "---------------------------------------------------------------------------------"

	hpad := fmt.Sprintf(sfmt, "")
	fmt.Fprintf(w, sfmt, "Key")
	fmt.Fprintf(w, "Value\n%s\n", sep)

	dumpField := func(name string, value string) {
		fmt.Fprintf(w, sfmt, name)
		valueLines := strings.Split(wordwrap.WrapString(value, 60), "\n")
		if len(valueLines) > 0 {
			fmt.Fprint(w, valueLines[0], "\n")
			for _, h := range valueLines[1:] {
				fmt.Fprint(w, hpad, h, "\n")
			}
		} else {
			fmt.Fprint(w, "\n")
		}
	}

	dumpField("Copyright", rpk.Metadata.Copyright)
	dumpField("Date", rpk.Metadata.Date)
	dumpField("File", rpk.Metadata.File)
	dumpField("Type", rpk.Metadata.TypeText)
	dumpField("Product", rpk.Metadata.ProductText)
	dumpField("Device", rpk.Metadata.Device)
	dumpField("Magic", rpk.Metadata.Magic)
	dumpField("Content Version", rpk.Metadata.ContentVersionText)
	dumpField("Prerequisites", rpk.Metadata.PrerequisitesText)
	dumpField("Content Note", rpk.Metadata.ContentNote)
	dumpField("Content Changes", rpk.Metadata.ContentChanges)
	dumpField("Content Errata", rpk.Metadata.ContentErrata)
	dumpField("Content Extra", rpk.Metadata.ContentExtra)
}
