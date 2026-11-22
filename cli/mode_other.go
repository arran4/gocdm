//go:build !linux

package cli

func readSystemLoginUID() (uint32, bool) {
	return 0, false
}
