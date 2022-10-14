package hackpadfs

// This file is a modified copy of io/fs/sub.go from the Go standard library

import (
	"errors"
	"io/fs"
	"path"
	"time"
)

// Sub attempts to call an optimized fs.Sub() if available.
// Falls back an implementation that passes through
func Sub(fsys FS, dir string) (FS, error) {
	if !ValidPath(dir) {
		return nil, &PathError{Op: "sub", Path: dir, Err: ErrInvalid}
	}
	if dir == "." {
		return fsys, nil
	}
	if fsys, ok := fsys.(SubFS); ok {
		return fsys.Sub(dir)
	}
	return &subFS{fsys, dir}, nil
}

type subFS struct {
	fsys FS
	dir  string
}

// fullName maps name to the fully-qualified name dir/name.
func (f *subFS) fullName(op string, name string) (string, error) {
	if !ValidPath(name) {
		return "", &PathError{Op: op, Path: name, Err: ErrInvalid}
	}
	return path.Join(f.dir, name), nil
}

// shorten maps name, which should start with f.dir, back to the suffix after f.dir.
func (f *subFS) shorten(name string) (rel string, ok bool) {
	if name == f.dir {
		return ".", true
	}
	if len(name) >= len(f.dir)+2 && name[len(f.dir)] == '/' && name[:len(f.dir)] == f.dir {
		return name[len(f.dir)+1:], true
	}
	return "", false
}

// fixErr shortens any reported names in PathErrors by stripping f.dir.
func (f *subFS) fixErr(err error) error {
	if e, ok := err.(*PathError); ok {
		if short, ok := f.shorten(e.Path); ok {
			e.Path = short
		}
	}
	if e, ok := err.(*LinkError); ok {
		if short, ok := f.shorten(e.Old); ok {
			e.Old = short
		}
		if short, ok := f.shorten(e.New); ok {
			e.New = short
		}
	}
	return err
}

func (f *subFS) Open(name string) (File, error) {
	full, err := f.fullName("open", name)
	if err != nil {
		return nil, err
	}
	file, err := f.fsys.Open(full)
	return file, f.fixErr(err)
}

func (f *subFS) ReadDir(name string) ([]DirEntry, error) {
	full, err := f.fullName("read", name)
	if err != nil {
		return nil, err
	}
	dir, err := ReadDir(f.fsys, full)
	return dir, f.fixErr(err)
}

func (f *subFS) ReadFile(name string) ([]byte, error) {
	full, err := f.fullName("read", name)
	if err != nil {
		return nil, err
	}
	data, err := ReadFile(f.fsys, full)
	return data, f.fixErr(err)
}

func (f *subFS) Glob(pattern string) ([]string, error) {
	// Check pattern is well-formed.
	if _, err := path.Match(pattern, ""); err != nil {
		return nil, err
	}
	if pattern == "." {
		return []string{"."}, nil
	}

	full := f.dir + "/" + pattern
	list, err := fs.Glob(f.fsys, full)
	for i, name := range list {
		name, ok := f.shorten(name)
		if !ok {
			return nil, errors.New("invalid result from inner fsys Glob: " + name + " not in " + f.dir) // can't use fmt in this package
		}
		list[i] = name
	}
	return list, f.fixErr(err)
}

func (f *subFS) Sub(dir string) (FS, error) {
	if dir == "." {
		return f, nil
	}
	full, err := f.fullName("sub", dir)
	if err != nil {
		return nil, err
	}
	return &subFS{f.fsys, full}, nil
}

func (f *subFS) OpenFile(name string, flag int, perm FileMode) (File, error) {
	full, err := f.fullName("open", name)
	if err != nil {
		return nil, err
	}
	file, err := OpenFile(f.fsys, full, flag, perm)
	return file, f.fixErr(err)
}

func (f *subFS) Create(name string) (File, error) {
	full, err := f.fullName("create", name)
	if err != nil {
		return nil, err
	}
	file, err := Create(f.fsys, full)
	return file, f.fixErr(err)
}

func (f *subFS) Mkdir(name string, perm FileMode) error {
	full, err := f.fullName("mkdir", name)
	if err != nil {
		return err
	}
	err = Mkdir(f.fsys, full, perm)
	return f.fixErr(err)
}

func (f *subFS) MkdirAll(path string, perm FileMode) error {
	full, err := f.fullName("mkdir", path)
	if err != nil {
		return err
	}
	err = MkdirAll(f.fsys, full, perm)
	return f.fixErr(err)
}

func (f *subFS) Remove(name string) error {
	full, err := f.fullName("remove", name)
	if err != nil {
		return err
	}
	err = Remove(f.fsys, full)
	return f.fixErr(err)
}

func (f *subFS) RemoveAll(name string) error {
	full, err := f.fullName("remove", name)
	if err != nil {
		return err
	}
	err = RemoveAll(f.fsys, full)
	return f.fixErr(err)
}

func (f *subFS) Stat(name string) (FileInfo, error) {
	full, err := f.fullName("stat", name)
	if err != nil {
		return nil, err
	}
	stat, err := Stat(f.fsys, full)
	return stat, f.fixErr(err)
}

func (f *subFS) Lstat(name string) (FileInfo, error) {
	full, err := f.fullName("stat", name)
	if err != nil {
		return nil, err
	}
	stat, err := Lstat(f.fsys, full)
	return stat, f.fixErr(err)
}

func (f *subFS) Chmod(name string, mode FileMode) error {
	full, err := f.fullName("chmod", name)
	if err != nil {
		return err
	}
	err = Chmod(f.fsys, full, mode)
	return f.fixErr(err)
}

func (f *subFS) Chown(name string, uid, gid int) error {
	full, err := f.fullName("chown", name)
	if err != nil {
		return err
	}
	err = Chown(f.fsys, full, uid, gid)
	return f.fixErr(err)
}

func (f *subFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	full, err := f.fullName("chtimes", name)
	if err != nil {
		return err
	}
	err = Chtimes(f.fsys, full, atime, mtime)
	return f.fixErr(err)
}

func (f *subFS) Rename(oldname, newname string) error {
	oldfull, err := f.fullName("rename", oldname)
	if err != nil {
		return err
	}
	newfull, err := f.fullName("rename", newname)
	if err != nil {
		return err
	}
	err = Rename(f.fsys, oldfull, newfull)
	return f.fixErr(err)
}

func (f *subFS) Symlink(oldname, newname string) error {
	oldfull, err := f.fullName("symlink", oldname)
	if err != nil {
		return err
	}
	newfull, err := f.fullName("symlink", newname)
	if err != nil {
		return err
	}
	err = Symlink(f.fsys, oldfull, newfull)
	return f.fixErr(err)
}

func (f *subFS) Mount(name string) (mountFS FS, subPath string) {
	return f.fsys, path.Join(f.dir, name)
}
