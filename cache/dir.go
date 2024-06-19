package cache

import (
	"io"

	"github.com/hack-pad/hackpadfs"
)

type dir struct {
	fs     *ReadOnlyFS
	name   string
	offset int
}

func (d *dir) Read(_ []byte) (n int, err error) {
	return 0, &hackpadfs.PathError{Op: "read", Path: d.name, Err: hackpadfs.ErrIsDir}
}

func (d *dir) Close() error {
	return nil
}

func (d *dir) Stat() (hackpadfs.FileInfo, error) {
	return hackpadfs.Stat(d.fs, d.name)
}

func (d *dir) ReadDir(n int) ([]hackpadfs.DirEntry, error) {
	entries, err := hackpadfs.ReadDir(d.fs.sourceFS, d.name)
	if err != nil {
		return nil, err
	}
	if n > 0 && d.offset == len(entries) {
		return nil, io.EOF
	}
	if n <= 0 || d.offset+n > len(entries) {
		d.offset = n
	} else {
		entries = entries[d.offset : d.offset+n]
		d.offset += n
	}
	return entries, nil
}
