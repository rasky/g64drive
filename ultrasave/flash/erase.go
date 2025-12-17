package flash

import (
	"context"
	"time"
)

// Erase a sector based on the specified page, or if nil the whole chip
func (f *Flash) Erase(ctx context.Context, page *Page) error {
	if err := f.startErase(page); err != nil {
		return err
	}

	/* Status register can directly read without an explicit cmdStatus write to CIR
	   and until a new command is written to CIR.

	   At this point, WSMReady is cleared, EraseBusy is set and will
	   keep these values until completion of the procedure.

	   Upon completion of the erase procedure:
	   * EraseBusy is cleared
	   * EraseOK reflects operation success (1: OK, 0: Error)
	   * WSMReady is set

	   Erasing a sector takes around 280ms (MN63F8MPN) / 85ms (MX29L1100) [may depend on content]
	   Erasing a full chip takes around 295ms (MN63F8MPN) / 85ms (MX29L1100) [may depend on content]

	   Datasheet for MX29L1611 (not N64 compatible) claims maximum of 2000ms for an erase operation (p.34),
	   so use that as a timeout.

	*/
	status, err := f.waitStatusBitsCleared(ctx, StatusEraseBusy, 10*time.Millisecond, 2000*time.Millisecond)
	if err != nil {
		return err
	}

	// clear status before leaving
	if err := f.clearStatus(); err != nil {
		return err
	}

	// return an error if EraseOK bit is not set
	if status&StatusEraseOK == 0 {
		return ErrOperationNOK
	}

	return nil
}

func (f *Flash) startErase(page *Page) error {
	if page != nil && (*page < 0 || int(*page) >= f.layout.TotalPages()) {
		return ErrInvalidPage
	}

	if err := f.writeCIR(setupEraseCmd(page)); err != nil {
		return err
	}

	if err := f.writeCIR(cmdErase); err != nil {
		return err
	}

	return nil
}
