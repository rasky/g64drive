package ultrasave

import (
	"errors"
)

var (
	// Implementors of ParallelInterface may return
	// this error if the requested operation is not supported.
	ErrUnsupported = errors.New("operation is not supported")
)

// Parallel Interface (PI) has a 32bit address space.
type PiAddress uint32

// ParallelInterface provides 32bit IO and burst IO.
// This interface decouple and abstract all interactions with PI devices.
type ParallelInterface interface {
	Read32(address PiAddress) (uint32, error)
	Write32(address PiAddress, data uint32) error
	ReadBurst(address PiAddress, data []byte) error
	WriteBurst(address PiAddress, data []byte) error
}
