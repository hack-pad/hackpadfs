package keyvalue

import (
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/keyvalue/blob"
)

// FileRecord represents a file inside a Store.
// A FileRecord's receivers may only be called once and their return values cached by the wrapping FS. Therefore, each receiver must return consistent values unless otherwise specified.
//
// NOTE: Does not require retrieving the file's name. The name can be stored separately to simplify your store.
type FileRecord interface {
	// Data returns the Blob representing a copy of this file's contents. Returns an error if file is a directory.
	Data() (blob.Blob, error)
	// ReadDir returns this file's directory entries. Returns an error if not a directory or failed during retrieval.
	ReadDirNames() ([]string, error)
	// Size returns the number of bytes in this file's contents.
	// May return the size at initial fetch time, rather than at call time.
	Size() int64
	// Mode returns this file's FileMode.
	Mode() hackpadfs.FileMode
	// ModTime returns this file's last modified time.
	ModTime() time.Time
	// Sys returns the underlying data source (can return nil)
	Sys() interface{}
}

var (
	_ FileRecord = &BaseFileRecord{}
)

// BaseFileRecord is a FileRecord
type BaseFileRecord struct {
	getData     func() (blob.Blob, error)
	getDirNames func() ([]string, error)
	initialSize int64
	modTime     time.Time
	mode        hackpadfs.FileMode
	sys         interface{}
}

func NewBaseFileRecord(
	initialSize int64,
	modTime time.Time,
	mode hackpadfs.FileMode,
	sys interface{},
	getData func() (blob.Blob, error),
	getDirNames func() ([]string, error),
) *BaseFileRecord {
	return &BaseFileRecord{
		getData:     getData,
		getDirNames: getDirNames,

		initialSize: initialSize,
		modTime:     modTime,
		mode:        mode,
		sys:         sys,
	}
}

func (b *BaseFileRecord) Data() (blob.Blob, error) {
	if b.getData == nil {
		if b.mode.IsDir() {
			return nil, hackpadfs.ErrIsDir
		}
		return nil, hackpadfs.ErrUnsupported
	}
	return b.getData()
}

func (b *BaseFileRecord) ReadDirNames() ([]string, error) {
	if b.getDirNames == nil {
		if !b.mode.IsDir() {
			return nil, hackpadfs.ErrNotDir
		}
		return nil, hackpadfs.ErrUnsupported
	}
	return b.getDirNames()
}

func (b *BaseFileRecord) Size() int64 {
	return b.initialSize
}

func (b *BaseFileRecord) Mode() hackpadfs.FileMode {
	return b.mode
}

func (b *BaseFileRecord) ModTime() time.Time {
	return b.modTime
}

func (b *BaseFileRecord) Sys() interface{} {
	return b.sys
}
