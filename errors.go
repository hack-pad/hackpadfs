package hackpadfs

import (
	"io/fs"
	"os"
	"syscall"
)

var (
	ErrInvalid     = fs.ErrInvalid
	ErrPermission  = fs.ErrPermission
	ErrExist       = fs.ErrExist
	ErrNotExist    = fs.ErrNotExist
	ErrClosed      = fs.ErrClosed
	ErrIsDir       = syscall.EISDIR
	ErrNotDir      = syscall.ENOTDIR
	ErrUnsupported = syscall.ENOSYS

	SkipDir = fs.SkipDir
)

type PathError = fs.PathError

type LinkError = os.LinkError
