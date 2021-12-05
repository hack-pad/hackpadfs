package s3

import (
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/keyvalue"
)

// FS is an S3-based file system, storing files and metadata in an object storage bucket.
type FS struct {
	kv    *keyvalue.FS
	store *store
}

// Options provides configuration options for a new FS.
type Options struct {
	Endpoint        string
	BucketName      string
	AccessKeyID     string
	SecretAccessKey string
	Insecure        bool
}

// NewFS returns a new FS.
func NewFS(options Options) (*FS, error) {
	store, err := newStore(options)
	if err != nil {
		return nil, err
	}
	kv, err := keyvalue.NewFS(store)
	return &FS{
		kv:    kv,
		store: store,
	}, err
}

// Open implements hackpadfs.FS
func (fs *FS) Open(name string) (hackpadfs.File, error) {
	return fs.kv.Open(name)
}

// OpenFile implements hackpadfs.OpenFileFS
func (fs *FS) OpenFile(name string, flag int, perm hackpadfs.FileMode) (hackpadfs.File, error) {
	return fs.kv.OpenFile(name, flag, perm)
}

// Mkdir implements hackpadfs.MkdirFS
func (fs *FS) Mkdir(name string, perm hackpadfs.FileMode) error {
	return fs.kv.Mkdir(name, perm)
}

// MkdirAll implements hackpadfs.MkdirAllFS
func (fs *FS) MkdirAll(path string, perm hackpadfs.FileMode) error {
	return fs.kv.MkdirAll(path, perm)
}

// Remove implements hackpadfs.RemoveFS
func (fs *FS) Remove(name string) error {
	return fs.kv.Remove(name)
}

// Rename implements hackpadfs.RenameFS
func (fs *FS) Rename(oldname, newname string) error {
	return fs.kv.Rename(oldname, newname)
}

// Stat implements hackpadfs.StatFS
func (fs *FS) Stat(name string) (hackpadfs.FileInfo, error) {
	return fs.kv.Stat(name)
}

// Chmod implements hackpadfs.ChmodFS
func (fs *FS) Chmod(name string, mode hackpadfs.FileMode) error {
	return fs.kv.Chmod(name, mode)
}

// Chtimes implements hackpadfs.ChtimesFS
func (fs *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return fs.kv.Chtimes(name, atime, mtime)
}
