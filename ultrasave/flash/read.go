package flash

import (
	"context"
)

/* This file provides reading functionalities.
 *
 * Even though flash chip allows to read individual bytes / words
 * it's more convenient to expose a Page based read function for the lowest API level
 * and build a function on top with relaxed constrains (see io.go).
 * This is motivated by the following reasons
 * * Program and Erase operations works with pages
 * * Read burst may not cross the ReadBurst boundary
 * * PI alignment constrains may prevent some other admissible flash reads.
 */

// Reads whole consecutive pages [begin, end[
func (f *Flash) ReadPages(ctx context.Context, begin, end Page) ([]byte, error) {
	if err := f.layout.validatePageRange(begin, end); err != nil {
		return nil, err
	}

	if err := f.writeCIR(cmdReadArray); err != nil {
		return nil, err
	}

	pageSize := f.layout.PageSize()
	data := make([]byte, int(end-begin)*pageSize)
	idx := 0

	for begin < end {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		burstEnd := f.layout.ReadBurstEnd(begin)
		if burstEnd >= end {
			burstEnd = end
		}

		count := int(burstEnd - begin)

		if err := f.pi.ReadBurst(f.baseAddress+f.layout.PageReadAddress(begin), data[idx:idx+count*pageSize]); err != nil {
			return nil, err
		}

		begin = begin + Page(count)
		idx = idx + count*pageSize
	}

	return data, nil
}
