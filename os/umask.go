// +build !plan9,!windows

package os

import "syscall"

func setUmask(mask int) (oldmask int) {
	return syscall.Umask(mask)
}
