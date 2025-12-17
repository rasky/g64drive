package flash

import (
	"context"
	"time"
)

// Status register is 8-bit.
type Status uint8

const (
	// Operation is in progress when set
	StatusProgramBusy = Status(0x01)
	StatusEraseBusy   = Status(0x02)

	// Operation result (set = success, cleared = failure)
	// Quirk: Default state of OperationOK depends on chip model
	// MN63F8MPN keep them cleared, and only set them upon operation completion.
	// MX29L1100 keep them set, and only clear them upon operation completion.
	StatusProgramOK = Status(0x04)
	StatusEraseOK   = Status(0x08)

	// 0x10: ???
	// 0x20: ???
	// 0x40: ???

	// When set, WSM is ready to accept Erase / Program commands.
	StatusWSMReady = Status(0x80)
)

func (f *Flash) Status() (Status, error) {
	// Quirk: MX29L1100 (and maybe other MX version),
	// may need at least 2 writes to CIR
	// before outputing proper status register content.
	// Also, upper 16bit of first read are corrupted and come from previous mode
	// but since we only care about the lower 8 bits, that's not problematic.
	// MN63F8MPN doesn't have this quirk.

	// For simplicity we work around the quirk regardless of chip variant.

	if err := f.writeCIR(cmdStatus); err != nil {
		return 0, err
	}

	if err := f.writeCIR(cmdStatus); err != nil {
		return 0, err
	}

	return f.status()
}

func (f *Flash) ClearStatus() error {
	if err := f.writeCIR(cmdStatus); err != nil {
		return err
	}

	// TOVERIFY?: do we also need to write to CIR twice ?
	if err := f.writeCIR(cmdStatus); err != nil {
		return err
	}

	return f.clearStatus()
}

// read status assuming status mode is already effective
func (f *Flash) status() (Status, error) {
	// Flash address is ignored (so use 0)
	u32, err := f.pi.Read32(f.baseAddress)
	if err != nil {
		return 0, err
	}

	// Only the lower 8bit are relevant
	return Status(u32 & 0xff), nil
}

// clear status assuming status mode is already effective
func (f *Flash) clearStatus() error {
	// On MN63F8MPN address and value are ignored
	// TOVERIFY: is that the case for MX29L1100 ?
	return f.pi.Write32(f.baseAddress, 0)
}

// poll status until mask bits are cleared assuming status mode is already effective
func (f *Flash) waitStatusBitsCleared(parentCtx context.Context, mask Status, period time.Duration, timeout time.Duration) (Status, error) {
	// Ensure waiting loop won't exceed given timeout
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	for ctx.Err() == nil {
		status, err := f.status()
		if err != nil {
			return 0, err
		}

		if status&mask == 0 {
			return status, nil
		}

		select {
		case <-ctx.Done():
		case <-time.After(period):
		}
	}

	return 0, ctx.Err()
}
