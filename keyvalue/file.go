package keyvalue

import (
	"context"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/keyvalue/blob"
)

var (
	_ interface {
		hackpadfs.File
		blob.Reader
		blob.ReaderAt
		blob.Writer
		blob.WriterAt
	} = &file{}
)

type file struct {
	*fileData
	offset int64
	flag   int
}

type fileData struct {
	runOnceFileRecord
	modeOverride    *hackpadfs.FileMode
	modTimeOverride time.Time

	path string // path is stored as the "key", keeping it here is for generating hackpadfs.FileInfo's
	fs   *FS
}

func (f *fileData) Mode() hackpadfs.FileMode {
	if f.modeOverride != nil {
		return *f.modeOverride
	}
	return f.runOnceFileRecord.Mode()
}

func (f *fileData) ModTime() time.Time {
	var zero time.Time
	if f.modTimeOverride != zero {
		return f.modTimeOverride
	}
	return f.runOnceFileRecord.ModTime()
}

// getFile returns a file for 'path' if it exists, os.ErrNotExist otherwise
func (fs *FS) getFile(path string) (*file, error) {
	if !hackpadfs.ValidPath(path) {
		return nil, hackpadfs.ErrInvalid
	}
	f := fileData{
		path: path,
		fs:   fs,
	}
	txn, err := fs.store.Transaction()
	if err != nil {
		return nil, err
	}
	txn.Get(path)
	results, err := txn.Commit(context.Background())
	if err != nil {
		return nil, err
	}
	f.runOnceFileRecord.record, err = results[0].Record, results[0].Err
	return &file{fileData: &f}, err
}

// setFile write the 'file' data to the store at 'path'. If 'file' is nil, the file is deleted.
func (fs *FS) setFile(path string, file FileRecord) error {
	if !hackpadfs.ValidPath(path) {
		return hackpadfs.ErrInvalid
	}
	txn, err := fs.store.Transaction()
	if err != nil {
		return err
	}
	txn.Set(path, file)
	_, err = txn.Commit(context.Background())
	return err
}

type fileInfo struct {
	Record FileRecord
	Path   string
}

func (f fileInfo) Name() string {
	return path.Base(f.Path)
}

func (f fileInfo) Size() int64 {
	return f.Record.Size()
}

func (f fileInfo) Mode() hackpadfs.FileMode {
	return f.Record.Mode()
}

func (f fileInfo) ModTime() time.Time {
	return f.Record.ModTime()
}

func (f fileInfo) IsDir() bool {
	return f.Record.Mode().IsDir()
}

func (f fileInfo) Sys() interface{} {
	return f.Record.Sys()
}

func (fs *FS) newFile(path string, flag int, mode hackpadfs.FileMode) *file {
	return &file{
		flag: flag,
		fileData: &fileData{
			fs:   fs,
			path: path,
			runOnceFileRecord: runOnceFileRecord{
				record: NewBaseFileRecord(0, time.Now(), mode, nil,
					func() (blob.Blob, error) {
						return blob.NewBytes(nil), nil
					},
					nil,
				),
			},
		},
	}
}

func (f *fileData) save() error {
	return f.fs.setFile(f.path, f)
}

func (f *fileData) info() hackpadfs.FileInfo {
	return fileInfo{Record: f, Path: f.path}
}

func (f *file) Close() error {
	if f.fileData == nil {
		return hackpadfs.ErrClosed
	}
	f.fileData = nil
	return nil
}

func (f *file) updateModTime() {
	f.modTimeOverride = time.Now()
}

func (f *file) Read(p []byte) (n int, err error) {
	n, err = f.ReadAt(p, f.offset)
	f.offset += int64(n)
	return
}

func (f *file) ReadBlob(length int) (blob blob.Blob, n int, err error) {
	blob, n, err = f.ReadBlobAt(length, f.offset)
	f.offset += int64(n)
	return
}

func (f *file) ReadAt(p []byte, off int64) (n int, err error) {
	blob, n, err := f.ReadBlobAt(len(p), off)
	if blob != nil {
		copy(p, blob.Bytes())
	}
	return n, err
}

func (f *file) ReadBlobAt(length int, off int64) (b blob.Blob, n int, err error) {
	if off >= int64(f.Size()) {
		return nil, 0, io.EOF
	}
	max := int64(f.Size())
	end := off + int64(length)
	if end > max {
		end = max
	}
	data, err := f.Data()
	if err != nil {
		return nil, 0, err
	}
	b, err = blob.View(data, off, end)
	if err != nil {
		return nil, 0, err
	}
	n = b.Len()
	if off+int64(n) == max {
		return b, n, io.EOF
	}
	return b, n, nil
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	newOffset := f.offset
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset += offset
	case io.SeekEnd:
		newOffset = int64(f.Size()) + offset
	default:
		return 0, fmt.Errorf("Unknown seek type: %d", whence)
	}
	if newOffset < 0 {
		return 0, fmt.Errorf("Cannot seek to negative offset: %d", newOffset)
	}
	f.offset = newOffset
	return newOffset, nil
}

func (f *file) Write(p []byte) (n int, err error) {
	n, err = f.WriteBlob(blob.NewBytes(p))
	return
}

func (f *file) WriteBlob(p blob.Blob) (n int, err error) {
	n, err = f.writeBlobAt("write", p, f.offset)
	f.offset += int64(n)
	return
}

func (f *file) WriteAt(p []byte, off int64) (n int, err error) {
	return f.WriteBlobAt(blob.NewBytes(p), off)
}

func (f *file) WriteBlobAt(p blob.Blob, off int64) (n int, err error) {
	return f.writeBlobAt("writeat", p, off)
}

func (f *file) writeBlobAt(op string, p blob.Blob, off int64) (n int, err error) {
	if f.flag&hackpadfs.FlagAppend != 0 {
		off = int64(f.Size())
	}

	endIndex := off + int64(p.Len())
	if int64(f.Size()) < endIndex {
		data, err := f.Data()
		if err != nil {
			return 0, &hackpadfs.PathError{Op: op, Path: f.path, Err: err}
		}
		err = blob.Grow(data, endIndex-int64(f.Size()))
		if err != nil {
			return 0, &hackpadfs.PathError{Op: op, Path: f.path, Err: err}
		}
	}
	data, err := f.Data()
	if err != nil {
		return 0, &hackpadfs.PathError{Op: op, Path: f.path, Err: err}
	}
	n, err = blob.Set(data, p, off)
	if err != nil {
		return n, &hackpadfs.PathError{Op: op, Path: f.path, Err: err}
	}
	if n != 0 {
		f.updateModTime()
	}
	err = f.save()
	return
}

func (f *file) Stat() (hackpadfs.FileInfo, error) {
	return fileInfo{Record: &f.runOnceFileRecord, Path: f.path}, nil
}

func (f *file) Truncate(size int64) error {
	if f.Mode().IsDir() {
		return &hackpadfs.PathError{Op: "truncate", Path: f.path, Err: hackpadfs.ErrIsDir}
	}
	length := int64(f.Size())
	switch {
	case size < 0:
		return &hackpadfs.PathError{Op: "truncate", Path: f.path, Err: hackpadfs.ErrInvalid}
	case size == length:
		return nil
	case size > length:
		data, err := f.Data()
		if err != nil {
			return &hackpadfs.PathError{Op: "truncate", Path: f.path, Err: err}
		}
		err = blob.Grow(data, size-length)
		if err != nil {
			return &hackpadfs.PathError{Op: "truncate", Path: f.path, Err: err}
		}
	case size < length:
		data, err := f.Data()
		if err != nil {
			return &hackpadfs.PathError{Op: "truncate", Path: f.path, Err: err}
		}
		err = blob.Truncate(data, size)
		if err != nil {
			return &hackpadfs.PathError{Op: "truncate", Path: f.path, Err: err}
		}
	}
	f.updateModTime()
	return f.save()
}
