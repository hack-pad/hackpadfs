package hackpadfs

import (
	"errors"
	gofs "io/fs"
	"time"
)

type FS = gofs.FS

type SubFS interface {
	FS
	Sub(dir string) (FS, error)
}

type OpenFileFS interface {
	FS
	OpenFile(name string, flag int, perm FileMode) (File, error)
}

type CreateFS interface {
	FS
	Create(name string) (File, error)
}

type MkdirFS interface {
	FS
	Mkdir(name string, perm FileMode) error
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

type LstatFS interface {
	FS
	Lstat(name string) (FileInfo, error)
}

type LstatOrStatFS interface {
	FS
	LstatOrStat(name string) (FileInfo, error)
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

type SymlinkFS interface {
	FS
	Symlink(oldname, newname string) error
}

func ValidPath(path string) bool {
	return gofs.ValidPath(path)
}

type WalkDirFunc = gofs.WalkDirFunc

func WalkDir(fs FS, root string, fn WalkDirFunc) error {
	return gofs.WalkDir(fs, root, fn)
}

func Sub(fs FS, dir string) (FS, error) {
	if fs, ok := fs.(SubFS); ok {
		return fs.Sub(dir)
	}
	return gofs.Sub(fs, dir)
}

func OpenFile(fs FS, name string, flag int, perm FileMode) (File, error) {
	if flag == FlagReadOnly {
		return fs.Open(name)
	}
	if fs, ok := fs.(OpenFileFS); ok {
		return fs.OpenFile(name, flag, perm)
	}
	return nil, ErrUnsupported
}

func Create(fs FS, name string) (File, error) {
	if fs, ok := fs.(CreateFS); ok {
		return fs.Create(name)
	}
	return OpenFile(fs, name, FlagReadWrite|FlagCreate|FlagTruncate, 0666)
}

func Mkdir(fs FS, name string, perm FileMode) error {
	if fs, ok := fs.(MkdirFS); ok {
		return fs.Mkdir(name, perm)
	}
	file, err := OpenFile(fs, name, FlagReadOnly|FlagCreate|FlagExclusive, perm|gofs.ModeDir)
	if err != nil {
		return &PathError{Op: "mkdir", Path: name, Err: err}
	}
	defer file.Close()
	return nil
}

func MkdirAll(fs FS, path string, perm FileMode) error {
	if fs, ok := fs.(MkdirAllFS); ok {
		return fs.MkdirAll(path, perm)
	}
	if !gofs.ValidPath(path) {
		return &PathError{Op: "mkdirall", Path: path, Err: ErrInvalid}
	}
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			err := Mkdir(fs, path[:i], perm)
			if err != nil && !errors.Is(err, ErrExist) {
				return err
			}
		}
	}
	return Mkdir(fs, path, perm)
}

func Remove(fs FS, name string) error {
	if fs, ok := fs.(RemoveFS); ok {
		return fs.Remove(name)
	}
	return &PathError{Op: "remove", Path: name, Err: ErrUnsupported}
}

func RemoveAll(fs FS, path string) error {
	if fs, ok := fs.(RemoveAllFS); ok {
		return fs.RemoveAll(path)
	}
	return &PathError{Op: "removeall", Path: path, Err: ErrUnsupported}
}

func Rename(fs FS, oldName, newName string) error {
	if fs, ok := fs.(RenameFS); ok {
		return fs.Rename(oldName, newName)
	}
	return &LinkError{Op: "rename", Old: oldName, New: newName, Err: ErrUnsupported}
}

func Stat(fs FS, name string) (FileInfo, error) {
	if fs, ok := fs.(StatFS); ok {
		return fs.Stat(name)
	}
	file, err := fs.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}

func Lstat(fs FS, name string) (FileInfo, error) {
	if fs, ok := fs.(LstatFS); ok {
		return fs.Lstat(name)
	}
	return nil, &PathError{Op: "lstat", Path: name, Err: ErrUnsupported}
}

func LstatOrStat(fs FS, name string) (FileInfo, error) {
	if fs, ok := fs.(LstatOrStatFS); ok {
		return fs.LstatOrStat(name)
	}
	info, err := Lstat(fs, name)
	if errors.Is(err, ErrUnsupported) {
		info, err = Stat(fs, name)
	}
	return info, err
}

func Chmod(fs FS, name string, mode FileMode) error {
	if fs, ok := fs.(ChmodFS); ok {
		return fs.Chmod(name, mode)
	}
	file, err := OpenFile(fs, name, FlagReadOnly, 0)
	if err != nil {
		return &PathError{Op: "chmod", Path: name, Err: err}
	}
	defer file.Close()
	return ChmodFile(file, mode)
}

func Chown(fs FS, name string, uid, gid int) error {
	if fs, ok := fs.(ChownFS); ok {
		return fs.Chown(name, uid, gid)
	}
	file, err := OpenFile(fs, name, FlagReadOnly, 0)
	if err != nil {
		return &PathError{Op: "chown", Path: name, Err: err}
	}
	defer file.Close()
	return ChownFile(file, uid, gid)
}

func Chtimes(fs FS, name string, atime time.Time, mtime time.Time) error {
	if fs, ok := fs.(ChtimesFS); ok {
		return fs.Chtimes(name, atime, mtime)
	}
	file, err := OpenFile(fs, name, FlagReadOnly, 0)
	if err != nil {
		return &PathError{Op: "chtimes", Path: name, Err: err}
	}
	defer file.Close()
	return ChtimesFile(file, atime, mtime)
}

func ReadDir(fs FS, name string) ([]DirEntry, error) {
	if fs, ok := fs.(ReadDirFS); ok {
		return fs.ReadDir(name)
	}
	return gofs.ReadDir(fs, name)
}

func ReadFile(fs FS, name string) ([]byte, error) {
	if fs, ok := fs.(ReadFileFS); ok {
		return fs.ReadFile(name)
	}
	return gofs.ReadFile(fs, name)
}

func Symlink(fs FS, oldname, newname string) error {
	if fs, ok := fs.(SymlinkFS); ok {
		return fs.Symlink(oldname, newname)
	}
	return &LinkError{Op: "symlink", Old: oldname, New: newname, Err: ErrUnsupported}
}
