package hackpadfs

import (
	"io/fs"
	"time"
)

type FS interface {
	Open(name string) (File, error)
}

type OpenFileFS interface {
	FS
	OpenFile(name string, flag int, perm FileMode) (File, error)
}

type MkdirAllFS interface {
	FS
	MkdirAll(path string, perm FileMode) error
}

type RemoveFS interface {
	FS
	Remove(name string) error
}

type RemoveAllFS interface {
	FS
	RemoveAll(name string) error
}

type RenameFS interface {
	FS
	Rename(oldname, newname string) error
}

type StatFS interface {
	FS
	Stat(name string) (FileInfo, error)
}

type ChmodFS interface {
	FS
	Chmod(name string, mode FileMode) error
}

type ChownFS interface {
	FS
	Chown(name string, uid, gid int) error
}

type ChtimesFS interface {
	FS
	Chtimes(name string, atime time.Time, mtime time.Time) error
}

type ReadDirFS interface {
	FS
	ReadDir(name string) ([]DirEntry, error)
}

type ReadFileFS interface {
	FS
	ReadFile(name string) ([]byte, error)
}

func OpenFile(fs FS, name string, flag int, perm FileMode) (File, error)
func Create(fs FS, name string) (File, error)
func Mkdir(fs FS, name string, perm FileMode) error
func MkdirAll(fs FS, path string, perm FileMode) error
func Remove(fs FS, name string) error
func RemoveAll(fs FS, path string) error
func Rename(fs FS, oldname, newname string) error
func Stat(fs FS, name string) (FileInfo, error)
func Chmod(fs FS, name string, mode FileMode) error
func Chown(fs FS, name string, uid, gid int) error
func Chtimes(fs FS, name string, atime time.Time, mtime time.Time) error
func ReadFSDir(fs FS, name string) ([]DirEntry, error)
func ReadFSFile(fs FS, name string) ([]byte, error)

type WalkDirFunc = fs.WalkDirFunc

func WalkDir(fs FS, root string, fn WalkDirFunc) error
