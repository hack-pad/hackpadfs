package hackpadfs

import (
	"io"
	gofs "io/fs"
	"time"
)

type FileMode = gofs.FileMode

const (
	ModeDir        = gofs.ModeDir
	ModeAppend     = gofs.ModeAppend
	ModeExclusive  = gofs.ModeExclusive
	ModeTemporary  = gofs.ModeTemporary
	ModeSymlink    = gofs.ModeSymlink
	ModeDevice     = gofs.ModeDevice
	ModeNamedPipe  = gofs.ModeNamedPipe
	ModeSocket     = gofs.ModeSocket
	ModeSetuid     = gofs.ModeSetuid
	ModeSetgid     = gofs.ModeSetgid
	ModeCharDevice = gofs.ModeCharDevice
	ModeSticky     = gofs.ModeSticky
	ModeIrregular  = gofs.ModeIrregular

	ModeType = gofs.ModeType
	ModePerm = gofs.ModePerm
)

type FileInfo = gofs.FileInfo

type DirEntry = gofs.DirEntry

type File = gofs.File

type ReadWriterFile interface {
	File
	io.Writer
}

type DirReaderFile interface {
	File
	ReadDir(n int) ([]DirEntry, error)
}

type SeekerFile interface {
	File
	io.Seeker
}

type SyncerFile interface {
	File
	Sync() error
}

type TruncaterFile interface {
	File
	Truncate(size int64) error
}

type ChmoderFile interface {
	File
	Chmod(mode FileMode) error
}

type ChownerFile interface {
	File
	Chown(uid, gid int) error
}

type ChtimeserFile interface {
	File
	Chtimes(atime time.Time, mtime time.Time) error
}

func ChmodFile(file File, mode FileMode) error {
	if file, ok := file.(ChmoderFile); ok {
		return file.Chmod(mode)
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return &PathError{Op: "chmod", Path: info.Name(), Err: ErrUnsupported}
}

func ChownFile(file File, uid, gid int) error {
	if file, ok := file.(ChownerFile); ok {
		return file.Chown(uid, gid)
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return &PathError{Op: "chmod", Path: info.Name(), Err: ErrUnsupported}
}

func ChtimesFile(file File, atime, mtime time.Time) error {
	if file, ok := file.(ChtimeserFile); ok {
		return file.Chtimes(atime, mtime)
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return &PathError{Op: "chtimes", Path: info.Name(), Err: ErrUnsupported}
}

func WriteFile(file File, p []byte) (n int, err error) {
	if file, ok := file.(ReadWriterFile); ok {
		return file.Write(p)
	}
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	return 0, &PathError{Op: "write", Path: info.Name(), Err: ErrUnsupported}
}

func ReadDirFile(file File, n int) ([]DirEntry, error) {
	if file, ok := file.(DirReaderFile); ok {
		return file.ReadDir(n)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return nil, &PathError{Op: "readdir", Path: info.Name(), Err: ErrUnsupported}
}

func SeekFile(file File, offset int64, whence int) (int64, error) {
	if file, ok := file.(SeekerFile); ok {
		return file.Seek(offset, whence)
	}
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	return 0, &PathError{Op: "seek", Path: info.Name(), Err: ErrUnsupported}
}

func SyncFile(file File) error {
	if file, ok := file.(SyncerFile); ok {
		return file.Sync()
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return &PathError{Op: "sync", Path: info.Name(), Err: ErrUnsupported}
}

func TruncateFile(file File, size int64) error {
	if file, ok := file.(TruncaterFile); ok {
		return file.Truncate(size)
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return &PathError{Op: "truncate", Path: info.Name(), Err: ErrUnsupported}
}
