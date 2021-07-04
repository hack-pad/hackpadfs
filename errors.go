package hackpadfs

import (
	"io/fs"
	"syscall"
)

var (
	ErrInvalid    = syscall.EINVAL // TODO update to fs.ErrInvalid, once errors.Is supports it
	ErrPermission = fs.ErrPermission
	ErrExist      = fs.ErrExist
	ErrNotExist   = fs.ErrNotExist
	ErrClosed     = fs.ErrClosed

	ErrIsDir          = syscall.EISDIR
	ErrNotDir         = syscall.ENOTDIR
	ErrNotEmpty       = syscall.ENOTEMPTY
	ErrNotImplemented = syscall.ENOSYS

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
