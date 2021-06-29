// +build !plan9,!windows

package osfs

import "syscall"

func setUmask(mask int) (oldmask int) {
	return syscall.Umask(mask)
}
