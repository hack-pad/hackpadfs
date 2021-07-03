// +build plan9 windows

package os

func setUmask(mask int) (oldmask int) {
	return 0
}
