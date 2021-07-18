package mounttest

import (
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/mount"
)

func NewFS(fs *mount.FS) interface {
	hackpadfs.FS
	hackpadfs.OpenFileFS
	hackpadfs.MkdirFS
	hackpadfs.MkdirAllFS
	hackpadfs.RemoveFS
	hackpadfs.StatFS
	hackpadfs.LstatFS
	hackpadfs.ChmodFS
	hackpadfs.ChownFS
	hackpadfs.ChtimesFS
	hackpadfs.ReadDirFS
	hackpadfs.ReadFileFS
	hackpadfs.MountFS
} {
	return &allMountFS{fs}
}

// allMountFS wraps a mount.FS with the usual functions implemented by a mounted file system, so fstest won't skip the capability-based tests
type allMountFS struct {
	*mount.FS
}

func (fs *allMountFS) OpenFile(name string, flag int, perm hackpadfs.FileMode) (hackpadfs.File, error) {
	return hackpadfs.OpenFile(fs.FS, name, flag, perm)
}

func (fs *allMountFS) Create(name string) (hackpadfs.File, error) {
	return hackpadfs.Create(fs.FS, name)
}

func (fs *allMountFS) Mkdir(name string, perm hackpadfs.FileMode) error {
	return hackpadfs.Mkdir(fs.FS, name, perm)
}

func (fs *allMountFS) MkdirAll(path string, perm hackpadfs.FileMode) error {
	return hackpadfs.MkdirAll(fs.FS, path, perm)
}

func (fs *allMountFS) Remove(name string) error {
	return hackpadfs.Remove(fs.FS, name)
}

func (fs *allMountFS) Stat(name string) (hackpadfs.FileInfo, error) {
	return hackpadfs.Stat(fs.FS, name)
}

func (fs *allMountFS) Lstat(name string) (hackpadfs.FileInfo, error) {
	return hackpadfs.Lstat(fs.FS, name)
}

func (fs *allMountFS) Chmod(name string, mode hackpadfs.FileMode) error {
	return hackpadfs.Chmod(fs.FS, name, mode)
}

func (fs *allMountFS) Chown(name string, uid, gid int) error {
	return hackpadfs.Chown(fs.FS, name, uid, gid)
}

func (fs *allMountFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return hackpadfs.Chtimes(fs.FS, name, atime, mtime)
}

func (fs *allMountFS) ReadDir(name string) ([]hackpadfs.DirEntry, error) {
	return hackpadfs.ReadDir(fs.FS, name)
}

func (fs *allMountFS) ReadFile(name string) ([]byte, error) {
	return hackpadfs.ReadFile(fs.FS, name)
}
