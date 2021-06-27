package hackpadfs

import (
	"io/fs"
	"syscall"
)

var (
	ErrInvalid     = fs.ErrInvalid
	ErrPermission  = fs.ErrPermission
	ErrExist       = fs.ErrExist
	ErrNotExist    = fs.ErrNotExist
	ErrClosed      = fs.ErrClosed
	ErrUnsupported = syscall.ENOSYS

	SkipDir = fs.SkipDir
)

type PathError = fs.PathError

type LinkError struct {
	Op  string
	Old string
	New string
	Err error
}

func (e *LinkError) Error() string {
	return e.Op + " " + e.Old + " " + e.New + ": " + e.Err.Error()
}

func (e *LinkError) Unwrap() error {
	return e.Err
}
