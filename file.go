package hackpadfs

import (
	"io"
	"io/fs"
)

type FileMode = fs.FileMode

type FileInfo = fs.FileInfo

type DirEntry = fs.DirEntry

type File interface {
	Stat() (FileInfo, error)
	io.Closer
}

type ReaderFile interface {
	File
	io.Reader
}

type WriterFile interface {
	File
	io.Writer
}

type ReadWriterFile interface {
	File
	io.Reader
	io.Writer
}

type ReadDirFile interface {
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

func ReadFile(file File, p []byte) (n int, err error)
func WriteFile(file File, p []byte) (n int, err error)
func ReadDir(file File, n int) ([]DirEntry, error)
func SeekFile(file File, offset int64, whence int) (int64, error)
func SyncFile(file File) error
func TruncateFile(file File, size int64) error
