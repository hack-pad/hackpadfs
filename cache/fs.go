package cache

import (
	"errors"
	"io"
	"path"
	"sync"

	"github.com/hack-pad/hackpadfs"
)

type writableFS interface {
	hackpadfs.OpenFileFS
	hackpadfs.MkdirFS
}

type ReadOnlyFS struct {
	sourceFS  hackpadfs.FS
	cacheFS   writableFS
	cacheInfo sync.Map

	mu      sync.Mutex
	options ReadOnlyOptions
}

type ReadOnlyOptions struct {
	RetainData func(name string, info hackpadfs.FileInfo) bool
}

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

func (fs *ReadOnlyFS) Open(name string) (hackpadfs.File, error) {
	// if source file is a dir or encounters an error, return early
	info, err := fs.Stat(name)
	switch {
	case err == nil && info.IsDir():
		return &dir{fs: fs, name: name}, nil
	case err != nil:
		return nil, err
	}

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
		f.Close()
		return nil, err
	}
	if _, err := hackpadfs.SeekFile(f, 0, io.SeekStart); err != nil {
		// attempt to seek to first byte. if unsuccessful, re-open file from the cache
		f.Close()
		f, err = fs.cacheFS.Open(name)
	}
	return f, err
}

func (fs *ReadOnlyFS) copyFile(name string, f hackpadfs.File, info hackpadfs.FileInfo) error {
	parentName := path.Dir(name)
	if err := hackpadfs.MkdirAll(fs.cacheFS, parentName, 0700); err != nil {
		return &hackpadfs.PathError{Op: "open", Path: parentName, Err: err}
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	destFile, err := fs.cacheFS.OpenFile(name, hackpadfs.FlagWriteOnly|hackpadfs.FlagCreate|hackpadfs.FlagTruncate, info.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	destFileWriter, ok := destFile.(io.Writer)
	if !ok {
		return &hackpadfs.PathError{Op: "open", Path: name, Err: hackpadfs.ErrPermission}
	}
	buf := make([]byte, 512)
	_, err = io.CopyBuffer(destFileWriter, f, buf)
	return err
}

func (fs *ReadOnlyFS) Stat(name string) (hackpadfs.FileInfo, error) {
	if infoV, loaded := fs.cacheInfo.Load(name); loaded {
		return infoV.(hackpadfs.FileInfo), nil
	}
	f, err := fs.sourceFS.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	fs.cacheInfo.Store(name, info)
	return info, nil
}
