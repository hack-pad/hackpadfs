package hackpadfs

import (
	"errors"
	gofs "io/fs"
	"time"
)

// FS provides access to a file system and its files.
// It is the minimum functionality required for a file system, and mirrors Go's io/fs.FS interface.
type FS = gofs.FS

// SubFS is an FS that can return a subset of the current FS.
// The same effect as `chroot` in a program.
type SubFS interface {
	FS
	Sub(dir string) (FS, error)
}

// OpenFileFS is an FS that can open files with the given flags and can create with the given permission.
// Should matche the behavior of os.OpenFile().
type OpenFileFS interface {
	FS
	OpenFile(name string, flag int, perm FileMode) (File, error)
}

// CreateFS is an FS that can create files. Should match the behavior of os.Create().
type CreateFS interface {
	FS
	Create(name string) (File, error)
}

// MkdirFS is an FS that can make directories. Should match the behavior of os.Mkdir().
type MkdirFS interface {
	FS
	Mkdir(name string, perm FileMode) error
}

// MkdirAllFS is an FS that can make all missing directories in a given path. Should match the behavior of os.MkdirAll().
type MkdirAllFS interface {
	FS
	MkdirAll(path string, perm FileMode) error
}

// RemoveFS is an FS that can remove files or empty directories. Should match the behavior of os.Remove().
type RemoveFS interface {
	FS
	Remove(name string) error
}

// RemoveAllFS is an FS that can remove files or directories recursively. Should match the behavior of os.RemoveAll().
type RemoveAllFS interface {
	FS
	RemoveAll(name string) error
}

// RenameFS is an FS that can move files or directories. Should match the behavior of os.Rename().
type RenameFS interface {
	FS
	Rename(oldname, newname string) error
}

// StatFS is an FS that can stat files or directories. Should match the behavior of os.Stat().
type StatFS interface {
	FS
	Stat(name string) (FileInfo, error)
}

// LstatFS is an FS that can lstat files. Same as Stat, but returns file info of symlinks instead of their target. Should match the behavior of os.Lstat().
type LstatFS interface {
	FS
	Lstat(name string) (FileInfo, error)
}

// ChmodFS is an FS that can change file or directory permissions. Should match the behavior of os.Chmod().
type ChmodFS interface {
	FS
	Chmod(name string, mode FileMode) error
}

// ChownFS is an FS that can change file or directory ownership. Should match the behavior of os.Chown().
type ChownFS interface {
	FS
	Chown(name string, uid, gid int) error
}

// ChtimesFS is an FS that can change a file's access and modified timestamps. Should match the behavior of os.Chtimes().
type ChtimesFS interface {
	FS
	Chtimes(name string, atime time.Time, mtime time.Time) error
}

// ReadDirFS is an FS that can read a directory and return its DirEntry's. Should match the behavior of os.ReadDir().
type ReadDirFS interface {
	FS
	ReadDir(name string) ([]DirEntry, error)
}

// ReadFileFS is an FS that can read an entire file in one pass. Should match the behavior of os.ReadFile().
type ReadFileFS interface {
	FS
	ReadFile(name string) ([]byte, error)
}

// SymlinkFS is an FS that can create symlinks. Should match the behavior of os.Symlink().
type SymlinkFS interface {
	FS
	Symlink(oldname, newname string) error
}

// MountFS is an FS that meshes one or more FS's together.
// Returns the FS for a file located at 'name' and its 'subPath' inside that FS.
type MountFS interface {
	FS
	Mount(name string) (mountFS FS, subPath string)
}

// ValidPath returns true if 'path' is a valid FS path. See io/fs.ValidPath() for details on FS-safe paths.
func ValidPath(path string) bool {
	return gofs.ValidPath(path)
}

// WalkDirFunc is the type of function called in WalkDir().
type WalkDirFunc = gofs.WalkDirFunc

// WalkDir recursively scans 'fs' starting at path 'root', calling 'fn' every time a new file or directory is visited.
func WalkDir(fs FS, root string, fn WalkDirFunc) error {
	return gofs.WalkDir(fs, root, fn)
}

// OpenFile attempts to call fs.Open() or fs.OpenFile() if available. Fails with a not implemented error otherwise.
func OpenFile(fs FS, name string, flag int, perm FileMode) (File, error) {
	if flag == FlagReadOnly {
		return fs.Open(name)
	}
	if fs, ok := fs.(OpenFileFS); ok {
		return fs.OpenFile(name, flag, perm)
	}
	if fs, ok := fs.(MountFS); ok {
		mountFS, subPath := fs.Mount(name)
		return OpenFile(mountFS, subPath, flag, perm)
	}
	return nil, &PathError{Op: "open", Path: name, Err: ErrNotImplemented}
}

// Create attempts to call an optimized fs.Create() if available, falls back to OpenFile() with create flags.
func Create(fs FS, name string) (File, error) {
	if fs, ok := fs.(CreateFS); ok {
		return fs.Create(name)
	}
	return OpenFile(fs, name, FlagReadWrite|FlagCreate|FlagTruncate, 0666)
}

// Mkdir creates a directory. Fails with a not implemented error if it's not a MkdirFS.
func Mkdir(fs FS, name string, perm FileMode) error {
	if fs, ok := fs.(MkdirFS); ok {
		return fs.Mkdir(name, perm)
	}
	if fs, ok := fs.(MountFS); ok {
		mountFS, subPath := fs.Mount(name)
		return Mkdir(mountFS, subPath, perm)
	}
	return &PathError{Op: "mkdir", Path: name, Err: ErrNotImplemented}
}

// MkdirAll attempts to call an optimized fs.MkdirAll(), falls back to multiple fs.Mkdir() calls.
func MkdirAll(fs FS, path string, perm FileMode) error {
	if fs, ok := fs.(MkdirAllFS); ok {
		return fs.MkdirAll(path, perm)
	}
	if fs, ok := fs.(MountFS); ok {
		mountFS, subPath := fs.Mount(path)
		return MkdirAll(mountFS, subPath, perm)
	}
	if !gofs.ValidPath(path) {
		return &PathError{Op: "mkdirall", Path: path, Err: ErrInvalid}
	}
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			err := Mkdir(fs, path[:i], perm)
			if err != nil {
				pathErr, ok := err.(*PathError)
				if !ok || !errors.Is(err, ErrExist) {
					return err
				}
				info, statErr := Stat(fs, pathErr.Path)
				if statErr != nil {
					return err
				}
				if !info.IsDir() {
					return &PathError{Op: "mkdir", Path: pathErr.Path, Err: ErrNotDir}
				}
			}
		}
	}
	return Mkdir(fs, path, perm)
}

// Remove removes a file with fs.Remove(). Fails with a not implemented error if it's not a RemoveFS.
func Remove(fs FS, name string) error {
	if fs, ok := fs.(RemoveFS); ok {
		return fs.Remove(name)
	}
	if fs, ok := fs.(MountFS); ok {
		return Remove(fs.Mount(name))
	}
	return &PathError{Op: "remove", Path: name, Err: ErrNotImplemented}
}

// RemoveAll removes files recursively with fs.RemoveAll(). Fails with a not implemented error if it's not a RemoveAllFS.
func RemoveAll(fs FS, path string) error {
	if fs, ok := fs.(RemoveAllFS); ok {
		return fs.RemoveAll(path)
	}
	if fs, ok := fs.(MountFS); ok {
		return RemoveAll(fs.Mount(path))
	}
	return &PathError{Op: "removeall", Path: path, Err: ErrNotImplemented}
}

// Rename moves files with fs.Rename(). Fails with a not implemented error if it's not a RenameFS.
func Rename(fs FS, oldName, newName string) error {
	if fs, ok := fs.(RenameFS); ok {
		return fs.Rename(oldName, newName)
	}
	return &LinkError{Op: "rename", Old: oldName, New: newName, Err: ErrNotImplemented}
}

// Stat attempts to call an optimized fs.Stat(), falls back to fs.Open() and file.Stat().
func Stat(fs FS, name string) (FileInfo, error) {
	if fs, ok := fs.(StatFS); ok {
		return fs.Stat(name)
	}
	if fs, ok := fs.(MountFS); ok {
		return Stat(fs.Mount(name))
	}
	file, err := fs.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return file.Stat()
}

// Lstat stats files and does not follow symlinks. Fails with a not implemented error if it's not a LstatFS.
func Lstat(fs FS, name string) (FileInfo, error) {
	if fs, ok := fs.(LstatFS); ok {
		return fs.Lstat(name)
	}
	if fs, ok := fs.(MountFS); ok {
		return Lstat(fs.Mount(name))
	}
	return nil, &PathError{Op: "lstat", Path: name, Err: ErrNotImplemented}
}

// LstatOrStat attempts to call an optimized fs.LstatOrStat(), fs.Lstat(), or fs.Stat() - whichever is supported first.
func LstatOrStat(fs FS, name string) (FileInfo, error) {
	if fs, ok := fs.(MountFS); ok {
		return LstatOrStat(fs.Mount(name))
	}
	info, err := Lstat(fs, name)
	if errors.Is(err, ErrNotImplemented) {
		info, err = Stat(fs, name)
	}
	return info, err
}

// Chmod attempts to call an optimized fs.Chmod(), falls back to opening the file and running file.Chmod().
func Chmod(fs FS, name string, mode FileMode) error {
	if fs, ok := fs.(ChmodFS); ok {
		return fs.Chmod(name, mode)
	}
	if fs, ok := fs.(MountFS); ok {
		mountFS, subPath := fs.Mount(name)
		return Chmod(mountFS, subPath, mode)
	}
	file, err := fs.Open(name)
	if err != nil {
		return &PathError{Op: "chmod", Path: name, Err: err}
	}
	defer func() { _ = file.Close() }()
	return ChmodFile(file, mode)
}

// Chown attempts to call an optimized fs.Chown(), falls back to opening the file and running file.Chown().
func Chown(fs FS, name string, uid, gid int) error {
	if fs, ok := fs.(ChownFS); ok {
		return fs.Chown(name, uid, gid)
	}
	if fs, ok := fs.(MountFS); ok {
		mountFS, subPath := fs.Mount(name)
		return Chown(mountFS, subPath, uid, gid)
	}
	file, err := fs.Open(name)
	if err != nil {
		return &PathError{Op: "chown", Path: name, Err: err}
	}
	defer func() { _ = file.Close() }()
	return ChownFile(file, uid, gid)
}

// Chtimes attempts to call an optimized fs.Chtimes(), falls back to opening the file and running file.Chtimes().
func Chtimes(fs FS, name string, atime time.Time, mtime time.Time) error {
	if fs, ok := fs.(ChtimesFS); ok {
		return fs.Chtimes(name, atime, mtime)
	}
	if fs, ok := fs.(MountFS); ok {
		mountFS, subPath := fs.Mount(name)
		return Chtimes(mountFS, subPath, atime, mtime)
	}
	file, err := fs.Open(name)
	if err != nil {
		return &PathError{Op: "chtimes", Path: name, Err: err}
	}
	defer func() { _ = file.Close() }()
	return ChtimesFile(file, atime, mtime)
}

// ReadDir attempts to call an optimized fs.ReadDir(), falls back to io/fs.ReadDir().
func ReadDir(fs FS, name string) ([]DirEntry, error) {
	if fs, ok := fs.(ReadDirFS); ok {
		return fs.ReadDir(name)
	}
	if fs, ok := fs.(MountFS); ok {
		return ReadDir(fs.Mount(name))
	}
	return gofs.ReadDir(fs, name)
}

// ReadFile attempts to call an optimized fs.ReadFile(), falls back to io/fs.ReadFile().
func ReadFile(fs FS, name string) ([]byte, error) {
	if fs, ok := fs.(ReadFileFS); ok {
		return fs.ReadFile(name)
	}
	if fs, ok := fs.(MountFS); ok {
		return ReadFile(fs.Mount(name))
	}
	return gofs.ReadFile(fs, name)
}

// Symlink creates a symlink. Fails with a not implemented error if it's not a SymlinkFS.
func Symlink(fs FS, oldname, newname string) error {
	if fs, ok := fs.(SymlinkFS); ok {
		return fs.Symlink(oldname, newname)
	}
	return &LinkError{Op: "symlink", Old: oldname, New: newname, Err: ErrNotImplemented}
}
