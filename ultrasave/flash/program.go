package flash

import (
	"context"
	"time"
)

func (f *Flash) ProgramPage(ctx context.Context, page Page) error {
	if err := f.startProgramPage(page); err != nil {
		return err
	}

	/* Status register can directly read without an explicit cmdStatus write to CIR
	   and until a new command is written to CIR.

	   At this point, WSMReady is cleared, ProgramBusy is set and will
	   keep these values until completion of the procedure.

	   Upon completion of the program procedure:
	   * ProgramBusy is cleared
	   * ProgramOK reflects operation success (1: OK, 0: Error)
	   * WSMReady is set

	   Programming a page takes around 380µs (MN63F8MPN) / 3.5ms (MX29L1100)

	   Datasheet for MX29L1611 (not N64 compatible) claims maximum of 500ms for program operation (p.34),
	   so use that as a timeout.
	*/

	status, err := f.waitStatusBitsCleared(ctx, StatusProgramBusy, 1*time.Millisecond, 500*time.Millisecond)
	if err != nil {
		return err
	}

	// clear status before leaving
	if err := f.clearStatus(); err != nil {
		return err
	}

	// return an error if ProgramOK bit is not set
	if status&StatusProgramOK == 0 {
		return ErrOperationNOK
	}

	return nil
}

func (f *Flash) startProgramPage(page Page) error {
	if page < 0 || int(page) >= f.layout.TotalPages() {
		return ErrInvalidPage
	}

	if err := f.writeCIR(programPageCmd(page)); err != nil {
		return err
	}

	return nil
}
