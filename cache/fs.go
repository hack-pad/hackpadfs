// Package cache contains a read-only cache file system.
package cache

import (
	"errors"
	"io"
	"path"
	"sync"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/pathlock"
)

type writableFS interface {
	hackpadfs.OpenFileFS
	hackpadfs.MkdirFS
}

// ReadOnlyFS is a read-only cache for an FS. Source FS data must not change. Data is assumed unchanged to increase performance.
type ReadOnlyFS struct {
	sourceFS  hackpadfs.FS
	cacheFS   writableFS
	cacheInfo sync.Map
	options   ReadOnlyOptions
	pathlock  pathlock.Mutex
}

// ReadOnlyOptions contain options for creating a ReadOnlyFS
type ReadOnlyOptions struct {
	RetainData func(name string, info hackpadfs.FileInfo) bool
}

// NewReadOnlyFS creates a new ReadOnlyFS with the given 'source' of data, a writable 'cache' FS, and any additional options.
func NewReadOnlyFS(source hackpadfs.FS, cache writableFS, options ReadOnlyOptions) (*ReadOnlyFS, error) {
	if options.RetainData == nil {
		options.RetainData = func(string, hackpadfs.FileInfo) bool { return true }
	}
	return &ReadOnlyFS{
		sourceFS: source,
		cacheFS:  cache,
		options:  options,
	}, nil
}

// Open implements hackpadfs.FS
func (fs *ReadOnlyFS) Open(name string) (hackpadfs.File, error) {
	// if source file is a dir or encounters an error, return early
	info, err := fs.Stat(name)
	switch {
	case err == nil && info.IsDir():
		return &dir{fs: fs, name: name}, nil
	case err != nil:
		return nil, err
	}

	fs.pathlock.Lock(name)
	defer fs.pathlock.Unlock(name)
	{
		// if file is in cache, return it. continue otherwise
		f, err := fs.cacheFS.Open(name)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, hackpadfs.ErrNotExist) {
			return nil, err
		}
	}

	f, err := fs.sourceFS.Open(name) // guaranteed not to be a directory
	if err != nil {
		return nil, err
	}
	if !fs.options.RetainData(name, info) {
		return f, nil
	}

	err = fs.copyFile(name, f, info)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if _, seekErr := hackpadfs.SeekFile(f, 0, io.SeekStart); seekErr != nil {
		// attempt to seek to first byte. if unsuccessful, re-open file from the cache
		_ = f.Close()
		f, err = fs.cacheFS.Open(name)
	}
	return f, err
}

func (fs *ReadOnlyFS) copyFile(name string, f hackpadfs.File, info hackpadfs.FileInfo) error {
	parentName := path.Dir(name)
	if err := hackpadfs.MkdirAll(fs.cacheFS, parentName, 0o700); err != nil {
		return &hackpadfs.PathError{Op: "open", Path: parentName, Err: err}
	}
	destFile, err := fs.cacheFS.OpenFile(name, hackpadfs.FlagWriteOnly|hackpadfs.FlagCreate|hackpadfs.FlagTruncate, info.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	destFileWriter, ok := destFile.(io.Writer)
	if !ok {
		return &hackpadfs.PathError{Op: "open", Path: name, Err: hackpadfs.ErrPermission}
	}
	buf := make([]byte, 512)
	_, err = io.CopyBuffer(destFileWriter, f, buf)
	return err
}

// Stat implements hackpadfs.StatFS
func (fs *ReadOnlyFS) Stat(name string) (hackpadfs.FileInfo, error) {
	if infoV, loaded := fs.cacheInfo.Load(name); loaded {
		return infoV.(hackpadfs.FileInfo), nil
	}
	f, err := fs.sourceFS.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	fs.cacheInfo.Store(name, info)
	return info, nil
}
