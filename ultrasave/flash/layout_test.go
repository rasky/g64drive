package flash

import (
	"fmt"
	"testing"
)

func shouldEqual(t *testing.T, expected, got int) {
	t.Helper()
	if expected != got {
		t.Errorf("%d != %d", expected, got)
	}
}

func TestLayoutProperties(t *testing.T) {
	var l Layout

	l = Layout_128B_128_8
	shouldEqual(t, 1, l.UnitSize())
	shouldEqual(t, 128, l.AddressesPerPage())
	shouldEqual(t, 128, l.PagesPerSector())
	shouldEqual(t, 8, l.SectorsPerChip())
	shouldEqual(t, 0x400, l.TotalPages())
	shouldEqual(t, 128, l.PageSize())
	shouldEqual(t, 128*128, l.SectorSize())
	shouldEqual(t, 8*128*128, l.ChipSize())
	shouldEqual(t, 256, l.ReadPageBoundary())

	l = Layout_64W_128_8
	shouldEqual(t, 2, l.UnitSize())
	shouldEqual(t, 64, l.AddressesPerPage())
	shouldEqual(t, 128, l.PagesPerSector())
	shouldEqual(t, 8, l.SectorsPerChip())
	shouldEqual(t, 0x400, l.TotalPages())
	shouldEqual(t, 128, l.PageSize())
	shouldEqual(t, 128*128, l.SectorSize())
	shouldEqual(t, 8*128*128, l.ChipSize())
	shouldEqual(t, 256, l.ReadPageBoundary())
}

func TestConversionToPageRange(t *testing.T) {

	tests := []struct {
		layout          Layout
		offset          int
		size            int
		expected_begin  Page
		expected_end    Page
		expected_skip   int
		expected_sbegin Page
		expected_send   Page
	}{
		// empty range
		// bytes: 0 <= k < 0
		// pages: 0 <= p < 0
		{
			layout:          Layout_128B_128_8,
			offset:          0,
			size:            0,
			expected_begin:  0,
			expected_end:    0,
			expected_skip:   0,
			expected_sbegin: 0,
			expected_send:   0,
		},
		// all bytes
		{
			layout:          Layout_128B_128_8,
			offset:          0,
			size:            Layout_128B_128_8.ChipSize(),
			expected_begin:  0,
			expected_end:    Page(Layout_128B_128_8.TotalPages()),
			expected_skip:   0,
			expected_sbegin: 0,
			expected_send:   Page(Layout_128B_128_8.TotalPages()),
		},
		// first page
		// bytes: 0 <= k < 128
		// pages: 0 <= p < 1
		{
			layout:          Layout_128B_128_8,
			offset:          0,
			size:            128,
			expected_begin:  0,
			expected_end:    1,
			expected_skip:   0,
			expected_sbegin: 0,
			expected_send:   128,
		},
		// second page
		// bytes: 128 <= k < 256
		// pages: 1 <= p < 2
		{
			layout:          Layout_128B_128_8,
			offset:          128,
			size:            128,
			expected_begin:  1,
			expected_end:    2,
			expected_skip:   0,
			expected_sbegin: 0,
			expected_send:   128,
		},
		// 1 byte at end of 1st page
		// bytes: 127 <= k < 128
		// pages: 0 <= p < 1
		{
			layout:          Layout_128B_128_8,
			offset:          127,
			size:            1,
			expected_begin:  0,
			expected_end:    1,
			expected_skip:   127,
			expected_sbegin: 0,
			expected_send:   128,
		},
		// 2 bytes at end of 1st page
		// bytes: 127 <= k < 129
		// pages: 0 <= p < 2
		{
			layout:          Layout_128B_128_8,
			offset:          127,
			size:            2,
			expected_begin:  0,
			expected_end:    2,
			expected_skip:   127,
			expected_sbegin: 0,
			expected_send:   128,
		},
		// 128 bytes at end of 1st page
		// bytes: 127 <= k < 255
		// pages: 0 <= p < 2
		{
			layout:          Layout_128B_128_8,
			offset:          127,
			size:            128,
			expected_begin:  0,
			expected_end:    2,
			expected_skip:   127,
			expected_sbegin: 0,
			expected_send:   128,
		},
		// 129 bytes at end of 1st page
		// bytes: 127 <= k < 256
		// pages: 0 <= p < 2
		{
			layout:          Layout_128B_128_8,
			offset:          127,
			size:            129,
			expected_begin:  0,
			expected_end:    2,
			expected_skip:   127,
			expected_sbegin: 0,
			expected_send:   128,
		},
		// 130 bytes at end of 1st page
		{
			layout:          Layout_128B_128_8,
			offset:          127,
			size:            130,
			expected_begin:  0,
			expected_end:    3,
			expected_skip:   127,
			expected_sbegin: 0,
			expected_send:   128,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v_%d_%d", tt.layout, tt.offset, tt.size), func(t *testing.T) {
			begin, end, skip, err := tt.layout.convertAddressRange(tt.offset, tt.size)
			if err != nil {
				t.Fatal(err)
			}
			if tt.expected_begin != begin {
				t.Errorf("unexpected begin: %d != %d", tt.expected_begin, begin)
			}
			if tt.expected_end != end {
				t.Errorf("unexpected end: %d != %d", tt.expected_end, end)
			}
			if tt.expected_skip != skip {
				t.Errorf("unexpected skip: %d != %d", tt.expected_skip, skip)
			}

			sbegin, send, err := tt.layout.pageRangeSectorBoundaries(begin, end)
			if err != nil {
				t.Fatal(err)
			}
			if tt.expected_sbegin != sbegin {
				t.Errorf("unexpected sector begin: %d != %d", tt.expected_sbegin, sbegin)
			}
			if tt.expected_send != send {
				t.Errorf("unexpected sector end: %d != %d", tt.expected_send, send)
			}
		})
	}

}
