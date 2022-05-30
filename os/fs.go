package os

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hack-pad/hackpadfs"
)

const (
	goosLinux   = "linux"
	goosWindows = "windows"
)

// FS wraps the 'os' package as an FS implementation.
type FS struct {
	root       string
	volumeName string
}

// NewFS returns a new FS. All file paths are relative to the root path.
// Root is '/' on Unix and 'C:\' on Windows.
// Use fs.Sub() to select a different root path. SubVolume on Windows can set the volume name.
func NewFS() *FS {
	return &FS{}
}

// SubVolume is like Sub, but only sets the volume name (i.e. for Windows).
// Calling SubVolume again on the returned FS results in an error.
func (fs *FS) SubVolume(volumeName string) (hackpadfs.FS, error) {
	if fs.root != "" {
		return nil, &hackpadfs.PathError{Op: "subvolume", Path: volumeName, Err: errors.New("subvolume not supported on a SubFS")}
	}
	if fs.volumeName != "" {
		return nil, &hackpadfs.PathError{Op: "subvolume", Path: volumeName, Err: errors.New("subvolume can only be called once per os.FS")}
	}
	if vol := filepath.VolumeName(volumeName); vol != volumeName {
		return nil, &hackpadfs.PathError{Op: "subvolume", Path: volumeName, Err: fmt.Errorf("sub volume must be equal to resolved volume: %q != %q", volumeName, vol)}
	}
	return &FS{
		volumeName: volumeName,
	}, nil
}

// Sub implements hackpadfs.SubFS
func (fs *FS) Sub(dir string) (hackpadfs.FS, error) {
	if !hackpadfs.ValidPath(dir) {
		return nil, &hackpadfs.PathError{Op: "sub", Path: dir, Err: hackpadfs.ErrInvalid}
	}
	return &FS{
		root:       path.Join(fs.root, dir),
		volumeName: fs.volumeName,
	}, nil
}

func (fs *FS) rootedPath(op, name string) (string, *hackpadfs.PathError) {
	return fs.rootedPathGOOS(runtime.GOOS, filepath.Separator, op, name)
}

func (fs *FS) rootedPathGOOS(goos string, separator rune, op, name string) (string, *hackpadfs.PathError) {
	if !hackpadfs.ValidPath(name) {
		return "", &hackpadfs.PathError{Op: op, Path: name, Err: hackpadfs.ErrInvalid}
	}
	name = path.Join("/", fs.root, name)
	filePath := joinSepPath(string(separator), fs.getVolumeName(goos), fromSeparator(separator, name))
	return filePath, nil
}

func joinSepPath(separator, elem1, elem2 string) string {
	elem1 = strings.TrimRight(elem1, separator)
	elem2 = strings.TrimLeft(elem2, separator)
	return elem1 + separator + elem2
}

func fromSeparator(separator rune, path string) string {
	if separator == '/' {
		return path
	}
	return strings.ReplaceAll(path, "/", string(separator))
}

func (fs *FS) getVolumeName(goos string) string {
	if goos == goosWindows && fs.volumeName == "" {
		return `C:`
	}
	return fs.volumeName
}

// wrapRelPathErr restores path names to the caller's path names, without the root path prefix
func (fs *FS) wrapRelPathErr(err error) error {
	rootedPath, rootedErr := fs.rootedPath("", ".")
	if rootedErr != nil {
		panic(rootedErr)
	}
	separator := string(filepath.Separator)
	switch e := err.(type) {
	case *hackpadfs.PathError:
		errCopy := *e
		errCopy.Path = strings.TrimPrefix(errCopy.Path, rootedPath)
		errCopy.Path = strings.TrimPrefix(errCopy.Path, separator)
		err = &errCopy
	case *os.LinkError:
		errCopy := &hackpadfs.LinkError{Op: e.Op, Old: e.Old, New: e.New, Err: e.Err}
		errCopy.Old = strings.TrimPrefix(errCopy.Old, rootedPath)
		errCopy.Old = strings.TrimPrefix(errCopy.Old, separator)
		errCopy.New = strings.TrimPrefix(errCopy.New, rootedPath)
		errCopy.New = strings.TrimPrefix(errCopy.New, separator)
		err = errCopy
	}
	return err
}

// Open implements hackpadfs.FS
func (fs *FS) Open(name string) (hackpadfs.File, error) {
	name, pathErr := fs.rootedPath("open", name)
	if pathErr != nil {
		return nil, pathErr
	}
	file, err := os.Open(name)
	return fs.wrapFile(file), fs.wrapRelPathErr(err)
}

// OpenFile implements hackpadfs.OpenFileFS
func (fs *FS) OpenFile(name string, flag int, perm hackpadfs.FileMode) (hackpadfs.File, error) {
	name, pathErr := fs.rootedPath("open", name)
	if pathErr != nil {
		return nil, pathErr
	}
	file, err := os.OpenFile(name, flag, perm)
	return fs.wrapFile(file), fs.wrapRelPathErr(err)
}

// Create implements hackpadfs.CreateFS
func (fs *FS) Create(name string) (hackpadfs.File, error) {
	name, pathErr := fs.rootedPath("create", name)
	if pathErr != nil {
		return nil, pathErr
	}
	file, err := os.Create(name)
	return fs.wrapFile(file), fs.wrapRelPathErr(err)
}

// Mkdir implements hackpadfs.MkdirFS
func (fs *FS) Mkdir(name string, perm hackpadfs.FileMode) error {
	name, err := fs.rootedPath("mkdir", name)
	if err != nil {
		return err
	}
	return fs.wrapRelPathErr(os.Mkdir(name, perm))
}

// MkdirAll implements hackpadfs.MkdirAllFS
func (fs *FS) MkdirAll(path string, perm hackpadfs.FileMode) error {
	path, err := fs.rootedPath("mkdirall", path)
	if err != nil {
		return err
	}
	return fs.wrapRelPathErr(os.MkdirAll(path, perm))
}

// Remove implements hackpadfs.RemoveFS
func (fs *FS) Remove(name string) error {
	name, err := fs.rootedPath("remove", name)
	if err != nil {
		return err
	}
	return fs.wrapRelPathErr(os.Remove(name))
}

// RemoveAll implements hackpadfs.RemoveAllFS
func (fs *FS) RemoveAll(name string) error {
	name, err := fs.rootedPath("removeall", name)
	if err != nil {
		return err
	}
	return fs.wrapRelPathErr(os.RemoveAll(name))
}

// Rename implements hackpadfs.RenameFS
func (fs *FS) Rename(oldname, newname string) error {
	oldname, err := fs.rootedPath("", oldname)
	if err != nil {
		return &hackpadfs.LinkError{Op: "rename", Old: oldname, New: newname, Err: err.Err}
	}
	newname, err = fs.rootedPath("", newname)
	if err != nil {
		return &hackpadfs.LinkError{Op: "rename", Old: oldname, New: newname, Err: err.Err}
	}
	return fs.wrapRelPathErr(os.Rename(oldname, newname))
}

// Stat implements hackpadfs.StatFS
func (fs *FS) Stat(name string) (hackpadfs.FileInfo, error) {
	name, pathErr := fs.rootedPath("stat", name)
	if pathErr != nil {
		return nil, pathErr
	}
	info, err := os.Stat(name)
	return info, fs.wrapRelPathErr(err)
}

// Lstat implements hackpadfs.LstatFS
func (fs *FS) Lstat(name string) (hackpadfs.FileInfo, error) {
	name, pathErr := fs.rootedPath("lstat", name)
	if pathErr != nil {
		return nil, pathErr
	}
	info, err := os.Lstat(name)
	return info, fs.wrapRelPathErr(err)
}

// Chmod implements hackpadfs.ChmodFS
func (fs *FS) Chmod(name string, mode hackpadfs.FileMode) error {
	name, err := fs.rootedPath("chmod", name)
	if err != nil {
		return err
	}
	return fs.wrapRelPathErr(os.Chmod(name, mode))
}

// Chown implements hackpadfs.ChownFS
func (fs *FS) Chown(name string, uid, gid int) error {
	name, err := fs.rootedPath("chown", name)
	if err != nil {
		return err
	}
	return fs.wrapRelPathErr(os.Chown(name, uid, gid))
}

// Chtimes implements hackpadfs.ChtimesFS
func (fs *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	name, err := fs.rootedPath("chtimes", name)
	if err != nil {
		return err
	}
	return fs.wrapRelPathErr(os.Chtimes(name, atime, mtime))
}

// ReadDir implements hackpadfs.ReadDirFS
func (fs *FS) ReadDir(name string) ([]hackpadfs.DirEntry, error) {
	name, pathErr := fs.rootedPath("readdir", name)
	if pathErr != nil {
		return nil, pathErr
	}
	entries, err := os.ReadDir(name)
	return entries, fs.wrapRelPathErr(err)
}

// ReadFile implements hackpadfs.ReadFile
func (fs *FS) ReadFile(name string) ([]byte, error) {
	name, pathErr := fs.rootedPath("readfile", name)
	if pathErr != nil {
		return nil, pathErr
	}
	contents, err := os.ReadFile(name)
	return contents, fs.wrapRelPathErr(err)
}

// Symlink implements hackpadfs.SymlinkFS
func (fs *FS) Symlink(oldname, newname string) error {
	oldname, pathErr := fs.rootedPath("symlink", oldname)
	if pathErr != nil {
		return &hackpadfs.LinkError{Op: "symlink", Old: oldname, New: newname, Err: pathErr.Err}
	}
	newname, pathErr = fs.rootedPath("symlink", newname)
	if pathErr != nil {
		return &hackpadfs.LinkError{Op: "symlink", Old: oldname, New: newname, Err: pathErr.Err}
	}
	return fs.wrapRelPathErr(os.Symlink(oldname, newname))
}
