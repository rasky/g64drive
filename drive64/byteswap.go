package drive64

import (
	"encoding/binary"
	"errors"
)

// ByteSwapper is a helper for byteswapping memory buffers
type ByteSwapper uint8

const (
	// BSNone is the no-op byteswapper
	BSNone ByteSwapper = 0
	// BSTwo byteswaps groups of 2 consecutive bytes in a buffer
	BSTwo ByteSwapper = 2
	// BSFour byteswaps groups of 4 consecutive bytes in a buffer
	BSFour ByteSwapper = 4
)

var (
	// ErrCannotDetectByteswap indicates that a byteswap autodetection has failed
	ErrCannotDetectByteswap = errors.New("cannot detect byteswap format")
	// ErrInvalidBufferForByteswap indicates the the buffer has a length that is invalid for byteswapping
	ErrInvalidBufferForByteswap = errors.New("invalid buffer size for byteswapping")
)

// ByteSwap byteswaps a memory buffer. The memory size must have a length multiple
// of two or four respectively for BSTwo and BSFour. In case the length is invalid
// ErrInvalidBufferForByteswap is returned.
func (bs ByteSwapper) ByteSwap(buf []byte) error {
	switch bs {
	case BSNone:
		return nil
	case BSTwo:
		if len(buf)%2 != 0 {
			return ErrInvalidBufferForByteswap
		}
		for i := 0; i < len(buf); i += 2 {
			var x = binary.LittleEndian.Uint16(buf[i : i+2])
			binary.BigEndian.PutUint16(buf[i:i+2], x)
		}
		return nil
	case BSFour:
		if len(buf)%4 != 0 {
			return ErrInvalidBufferForByteswap
		}
		for i := 0; i < len(buf); i += 4 {
			var x = binary.LittleEndian.Uint32(buf[i : i+4])
			binary.BigEndian.PutUint32(buf[i:i+4], x)
		}
		return nil
	default:
		panic("unreachable")
	}
}

// ByteSwapDetect detects the correct byteswapping format from a Nintendo 64 ROM header,
// by peeking at the magic number in the first 4 bytes. If the magic number is not
// found, ErrCannotDetectByteswap is returned.
func ByteSwapDetect(romHeader []byte) (ByteSwapper, error) {
	if len(romHeader) >= 4 {
		switch binary.BigEndian.Uint32(romHeader[:4]) {
		case 0x80371240:
			return BSNone, nil
		case 0x37804012:
			return BSTwo, nil
		case 0x40123780:
			return BSFour, nil
		}
	}
	return BSNone, ErrCannotDetectByteswap
}
