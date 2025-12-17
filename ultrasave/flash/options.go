package flash

import (
	"github.com/rasky/g64drive/ultrasave"
)

type FlashOption func(*Flash) error

// WithBaseAddress allows to override default base address.
func WithBaseAddress(baseAddress ultrasave.PiAddress) FlashOption {
	return func(f *Flash) error {
		f.baseAddress = baseAddress
		return nil
	}
}

// WithLayout allows to avoid auto detection of layout and use specified layout.
func WithLayout(layout Layout) FlashOption {
	return func(f *Flash) error {
		f.layout = layout
		return nil
	}
}
