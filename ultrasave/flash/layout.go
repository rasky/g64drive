package flash

import (
	"github.com/rasky/g64drive/ultrasave"
)

// Address of a programmable 128-byte/64-word block of data.
type Page int

type Layout struct {
	// 2^unitBits gives the size of 1 unit of data
	unitBits int

	// 2^offsetBits gives the number of addressable unit of data per page
	offsetBits int

	// 2^pageBits gives the number of pages per sector
	pageBits int

	// 2^sectorBits gives the number of sectors per chip
	sectorBits int

	// 2^readPageBits gives the size (expessed in number of page) of the read page boundary.
	readPageBits int
}

var (
	// 128 byte x 128 pages x 8 sectors - 256 page read boundary
	Layout_128B_128_8 = Layout{
		unitBits:     0,
		offsetBits:   7,
		pageBits:     7,
		sectorBits:   3,
		readPageBits: 8,
	}

	// 64 word x 128 pages x 8 sectors - 256 page read boundary
	Layout_64W_128_8 = Layout{
		unitBits:     1,
		offsetBits:   6,
		pageBits:     7,
		sectorBits:   3,
		readPageBits: 8,
	}
)

func (l Layout) UnitSize() int {
	return 1 << l.unitBits
}

func (l Layout) AddressesPerPage() int {
	return 1 << l.offsetBits
}

func (l Layout) PagesPerSector() int {
	return 1 << l.pageBits
}

func (l Layout) SectorsPerChip() int {
	return 1 << l.sectorBits
}

func (l Layout) ReadPageBoundary() int {
	return 1 << l.readPageBits
}

func (l Layout) TotalPages() int {
	return l.PagesPerSector() * l.SectorsPerChip()
}

func (l Layout) PageSize() int {
	return l.UnitSize() * l.AddressesPerPage()
}

func (l Layout) SectorSize() int {
	return l.PagesPerSector() * l.PageSize()
}

func (l Layout) ChipSize() int {
	return l.SectorsPerChip() * l.SectorSize()
}

func (l Layout) SectorBegin(page Page) Page {
	mask := uint(l.PagesPerSector() - 1)
	return Page(uint(page) & ^mask)
}

func (l Layout) SectorEnd(page Page) Page {
	mask := uint(l.PagesPerSector() - 1)
	return Page(uint(page)|mask) + 1
}

func (l Layout) ReadBurstEnd(page Page) Page {
	mask := uint(l.ReadPageBoundary() - 1)
	return Page(uint(page)|mask) + 1

}

// In read array mode, read address must be adjusted for
// 16-word addresses chips.
func (l Layout) ReadAddress(offset int) ultrasave.PiAddress {
	return ultrasave.PiAddress(offset / l.UnitSize())
}

func (l Layout) PageReadAddress(page Page) ultrasave.PiAddress {
	return ultrasave.PiAddress(int(page) * l.AddressesPerPage())
}

func (l Layout) validateOffsetSize(offset, size int) error {
	chipSize := l.ChipSize()
	if offset < 0 || offset >= chipSize {
		return ErrInvalidOffsetSize
	}

	if size < 0 || size > chipSize {
		return ErrInvalidOffsetSize
	}

	if offset+size > chipSize {
		return ErrInvalidOffsetSize
	}

	return nil
}

func (l Layout) validatePageRange(begin, end Page) error {
	totalPages := Page(l.TotalPages())

	if begin < 0 || begin >= totalPages {
		return ErrInvalidPageRange
	}
	if end < 0 || end > totalPages {
		return ErrInvalidPageRange
	}
	if end < begin {
		return ErrInvalidPageRange
	}

	return nil
}

// convert offset / size into a Page range (+ remainder)
// page range is [begin,end[
func (l Layout) convertAddressRange(offset, size int) (Page, Page, int, error) {
	pageSize := l.PageSize()

	if err := l.validateOffsetSize(offset, size); err != nil {
		return 0, 0, 0, err
	}

	begin := offset / pageSize
	remainder := offset % pageSize

	end := begin
	if size > 0 {
		end = ((offset + size - 1) / pageSize) + 1
	}

	return Page(begin), Page(end), remainder, nil
}

func (l Layout) pageRangeSectorBoundaries(begin, end Page) (Page, Page, error) {
	if err := l.validatePageRange(begin, end); err != nil {
		return 0, 0, ErrInvalidPageRange
	}

	sbegin := l.SectorBegin(begin)
	send := sbegin
	if end > begin {
		send = l.SectorEnd(end - 1)
	}

	return sbegin, send, nil
}
