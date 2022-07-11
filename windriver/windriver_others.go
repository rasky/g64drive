//go:build !windows

package windriver

func Search() bool {
	panic("unreachable")
}

func Elevate() {
	panic("unreachable")
}

func Install() {
	panic("unreachable")
}
