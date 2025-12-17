package flash

import (
	"context"
	"fmt"
)

/* This file provides higher-level I/O functionality.
 * It abstracts away all underlying complexities of the Flash technology (Erase / Program, Sectors / Pages),
 * and gives a more familiar linear byte addressing model.
 * TODO: provide io.Reader and io.Writer interface.
 */

// Higher level function to read data from flash
// without having to care about internal flash layout.
// FIXME: add support for progress bar
func (f *Flash) Read(ctx context.Context, offset, size int) ([]byte, error) {
	begin, end, skip, err := f.layout.convertAddressRange(offset, size)
	if err != nil {
		return nil, err
	}

	data, err := f.ReadPages(ctx, begin, end)
	if err != nil {
		return nil, err
	}

	return data[skip : skip+size], nil
}

// Higher level function to write data to flash
// without having to care about internal flash layout.
// FIXME: add support for progress bar by adding a io.Reader parameter ?
// FIXME: add some verbose print
func (f *Flash) Write(ctx context.Context, offset int, data []byte) error {
	layout := f.layout

	begin, end, skip, err := layout.convertAddressRange(offset, len(data))
	if err != nil {
		return err
	}

	// Deduce impacted sectors
	sbegin, send, err := layout.pageRangeSectorBoundaries(begin, end)
	if err != nil {
		return err
	}

	fmt.Printf("begin=%d, end=%d, skip=%d, sbegin=%d, send=%d\n", begin, end, skip, sbegin, send)

	// Prepare data that will effectively be written
	sectorSize := layout.SectorSize()
	var sectorsData []byte
	if offset%sectorSize == 0 && len(data)%sectorSize == 0 {
		fmt.Printf("Skipping sector read\n")
		// If data fits exactly in a sector, we can skip reading it's content
		// as it will be all overwritten
		sectorsData = data
	} else {
		fmt.Printf("Reading sector\n")
		// Otherwise we have to combine current sector data
		// with data to be written
		var err error
		sectorsData, err = f.ReadPages(ctx, sbegin, send)
		if err != nil {
			return err
		}
		copy(sectorsData[skip:skip+len(data)], data)
	}

	// Ensure WSM is ready before starting erase command
	if err := f.ClearStatus(); err != nil {
		return err
	}
	if status, err := f.Status(); err != nil {
		return err
	} else {
		fmt.Printf("WSM check: %02x\n", status)
		if status&StatusWSMReady != StatusWSMReady {
			fmt.Printf("WSM not ready yet: %02x\n", status)
			return fmt.Errorf("WSM not ready yet: %02x", status)
		}
	}

	// Erase impacted sectors
	if int(sbegin) == 0 && int(send) == layout.TotalPages() {
		fmt.Printf("Erasing chip\n")
		// If all sectors are impacted use full chip erasure
		if err := f.Erase(ctx, nil); err != nil {
			return err
		}
	} else {
		// Otherwise erase sectors one by one
		for p := sbegin; ctx.Err() == nil && p < send; p = layout.SectorEnd(p) {
			fmt.Printf("Erasing sector p=%d\n", p)
			if err := f.Erase(ctx, &p); err != nil {
				return err
			}
		}
	}

	// Load and program all pages
	pageSize := layout.PageSize()
	idx := 0
	for p := sbegin; ctx.Err() == nil && p < send; p++ {
		fmt.Printf("Programming page p=%d\n", p)
		if err := f.LoadBytePage(sectorsData[idx : idx+pageSize]); err != nil {
			return err
		}

		if err := f.ProgramPage(ctx, p); err != nil {
			return err
		}

		idx = idx + pageSize
	}

	return ctx.Err()
}
