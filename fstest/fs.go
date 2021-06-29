package fstest

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestBaseCreate(tb testing.TB, setup SetupFSFunc) {
	_, commit := setup(tb)
	fs, ok := commit().(hackpadfs.CreateFS)
	if !ok {
		tb.Skip("FS is not a CreateFS")
	}

	file, err := fs.Create("foo")
	assert.NoError(tb, err)
	if assert.NotZero(tb, file) {
		assert.NoError(tb, file.Close())
	}
}

func TestBaseMkdir(tb testing.TB, setup SetupFSFunc) {
	_, commit := setup(tb)
	fs, ok := commit().(hackpadfs.MkdirFS)
	if !ok {
		tb.Skip("FS is not a MkdirFS")
	}

	err := fs.Mkdir("foo", 0600)
	assert.NoError(tb, err)
}

func TestBaseChmod(tb testing.TB, setup SetupFSFunc) {
	setupFS, commit := setup(tb)
	f, err := hackpadfs.Create(setupFS, "foo")
	assert.NoError(tb, err)
	assert.NoError(tb, f.Close())

	fs, ok := commit().(hackpadfs.ChmodFS)
	if !ok {
		tb.Skip("FS is not a ChmodFS")
	}

	err = fs.Chmod("foo", 0755)
	assert.NoError(tb, err)
}

func TestBaseChtimes(tb testing.TB, setup SetupFSFunc) {
	var (
		accessTime = time.Now()
		modifyTime = accessTime.Add(-10 * time.Second)
	)
	setupFS, commit := setup(tb)
	f, err := hackpadfs.Create(setupFS, "foo")
	assert.NoError(tb, err)
	assert.NoError(tb, f.Close())

	fs, ok := commit().(hackpadfs.ChtimesFS)
	if !ok {
		tb.Skip("FS is not a ChtimesFS")
	}

	err = fs.Chtimes("foo", accessTime, modifyTime)
	assert.NoError(tb, err)
}

type quickInfo struct {
	Name  string
	Size  int64
	Mode  hackpadfs.FileMode
	IsDir bool
}

func asQuickInfo(info hackpadfs.FileInfo) quickInfo {
	return quickInfo{
		Name:  info.Name(),
		Size:  info.Size(),
		Mode:  info.Mode(),
		IsDir: info.IsDir(),
	}
}

// TestCreate verifies fs.Create().
//
// Create creates or truncates the named file.
// If the file already exists, it is truncated.
// If the file does not exist, it is created with mode 0666 (before umask).
// If successful, methods on the returned File can be used for I/O; the associated file descriptor has mode O_RDWR.
// If there is an error, it will be of type *PathError.
func TestCreate(tb testing.TB, setup SetupFSFunc) {
	testFsCreate(tb, setup, func(fs hackpadfs.FS, name string) (hackpadfs.File, error) {
		if fs, ok := fs.(hackpadfs.CreateFS); ok {
			return fs.Create(name)
		}
		tb.Skip("FS is not a CreateFS")
		return nil, nil
	})
}

func testFsCreate(tb testing.TB, setup SetupFSFunc, createFn func(hackpadfs.FS, string) (hackpadfs.File, error)) {
	tbRun(tb, "new file", func(tb testing.TB) {
		_, commit := setup(tb)
		fs := commit()
		file, err := createFn(fs, "foo")
		assert.NoError(tb, err)
		if assert.NotZero(tb, file) {
			assert.NoError(tb, file.Close())
		}
		info, err := hackpadfs.Stat(fs, "foo")
		assert.NoError(tb, err)

		assert.Equal(tb, quickInfo{
			Name: "foo",
			Mode: hackpadfs.FileMode(0666),
		}, asQuickInfo(info))
	})

	tbRun(tb, "existing file", func(tb testing.TB) {
		const fileContents = `hello world`
		setupFS, commit := setup(tb)

		file, err := hackpadfs.Create(setupFS, "foo")
		assert.NoError(tb, err)
		_, err = hackpadfs.WriteFile(file, []byte(fileContents))
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())
		assert.NoError(tb, hackpadfs.Chmod(setupFS, "foo", 0755))

		fs := commit()
		file, err = createFn(fs, "foo")
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())
		info, err := hackpadfs.Stat(fs, "foo")
		assert.NoError(tb, err)

		assert.Equal(tb, quickInfo{
			Name: "foo",
			Mode: 0755,
		}, asQuickInfo(info))
	})

	tbRun(tb, "existing directory", func(tb testing.TB) {
		setupFS, commit := setup(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		fs := commit()
		_, err := createFn(fs, "foo")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.Equal(tb, "open", err.Op)
			assert.Equal(tb, "foo", err.Path)
			assert.Equal(tb, true, errors.Is(err, hackpadfs.ErrIsDir))
		}
	})

	tbRun(tb, "parent directory must exist", func(tb testing.TB) {
		_, commit := setup(tb)
		fs := commit()
		_, err := createFn(fs, "foo/bar")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.Equal(tb, "open", err.Op)
			assert.Equal(tb, "foo/bar", err.Path)
			assert.Equal(tb, true, errors.Is(err, hackpadfs.ErrNotExist))
		}
	})
}

func asQuickDirInfos(tb testing.TB, entries []hackpadfs.DirEntry) []quickInfo {
	tb.Helper()
	var dirs []quickInfo
	for _, entry := range entries {
		dirs = append(dirs, asQuickDirInfo(tb, entry))
	}
	return dirs
}

func asQuickDirInfo(tb testing.TB, entry hackpadfs.DirEntry) quickInfo {
	tb.Helper()
	mode := entry.Type()
	var size int64
	info, err := entry.Info()
	if assert.NoError(tb, err) {
		mode = info.Mode()
		if !entry.IsDir() {
			size = info.Size()
		}
	}
	return quickInfo{
		Name:  entry.Name(),
		Size:  size,
		Mode:  mode,
		IsDir: entry.IsDir(),
	}
}

// TestMkdir verifies fs.Mkdir().
//
// Mkdir creates a new directory with the specified name and permission bits (before umask). If there is an error, it will be of type *PathError.
func TestMkdir(tb testing.TB, setup SetupFSFunc) {
	mkdirFS := func(tb testing.TB, fs hackpadfs.FS) hackpadfs.MkdirFS {
		if fs, ok := fs.(hackpadfs.MkdirFS); ok {
			return fs
		}
		tb.Skip("FS is not a MkdirFS")
		return nil
	}

	tbRun(tb, "fail dir exists", func(tb testing.TB) {
		_, commit := setup(tb)
		fs := mkdirFS(tb, commit())
		assert.NoError(tb, fs.Mkdir("foo", 0600))
		err := fs.Mkdir("foo", 0600)
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.Equal(tb, true, errors.Is(err, hackpadfs.ErrExist))
			assert.Equal(tb, "mkdir", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
		tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: hackpadfs.ModeDir | 0600, IsDir: true},
		}, fs)
	})

	tbRun(tb, "fail file exists", func(tb testing.TB) {
		setupFS, commit := setup(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())

		fs := mkdirFS(tb, commit())
		err = fs.Mkdir("foo", 0600)
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.Equal(tb, true, errors.Is(err, hackpadfs.ErrExist))
			assert.Equal(tb, "mkdir", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
		tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: 0666},
		}, fs)
	})

	tbRun(tb, "create sub dir", func(tb testing.TB) {
		_, commit := setup(tb)
		fs := mkdirFS(tb, commit())
		assert.NoError(tb, fs.Mkdir("foo", 0700))
		assert.NoError(tb, fs.Mkdir("foo/bar", 0600))

		tryAssertEqualFS(tb, map[string]fsEntry{
			"foo":     {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"foo/bar": {Mode: hackpadfs.ModeDir | 0600, IsDir: true},
		}, fs)
	})

	tbRun(tb, "only permission bits allowed", func(tb testing.TB) {
		_, commit := setup(tb)
		fs := mkdirFS(tb, commit())
		assert.NoError(tb, fs.Mkdir("foo", hackpadfs.ModeSocket|0755))

		tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: hackpadfs.ModeDir | 0755, IsDir: true},
		}, fs)
	})

	tbRun(tb, "parent directory must exist", func(tb testing.TB) {
		_, commit := setup(tb)
		fs := mkdirFS(tb, commit())

		err := fs.Mkdir("foo/bar", 0755)
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.Equal(tb, true, errors.Is(err, hackpadfs.ErrNotExist))
			assert.Equal(tb, "mkdir", err.Op)
			assert.Equal(tb, "foo/bar", err.Path)
		}
		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})
}

// MkdirAll creates a directory named path, along with any necessary parents, and returns nil, or else returns an error.
// The permission bits perm (before umask) are used for all directories that MkdirAll creates.
// If path is already a directory, MkdirAll does nothing and returns nil.
func TestMkdirAll(tb testing.TB, setup SetupFSFunc) {
	mkdirAllFS := func(tb testing.TB, fs hackpadfs.FS) hackpadfs.MkdirAllFS {
		if fs, ok := fs.(hackpadfs.MkdirAllFS); ok {
			return fs
		}
		tb.Skip("FS is not a MkdirAllFS")
		return nil
	}

	tbRun(tb, "create one directory", func(tb testing.TB) {
		_, commit := setup(tb)
		fs := mkdirAllFS(tb, commit())
		assert.NoError(tb, fs.MkdirAll("foo", 0700))
		tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
		}, fs)
	})

	tbRun(tb, "create multiple directories", func(tb testing.TB) {
		_, commit := setup(tb)
		fs := mkdirAllFS(tb, commit())
		assert.NoError(tb, fs.MkdirAll("foo/bar", 0700))
		tryAssertEqualFS(tb, map[string]fsEntry{
			"foo":     {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"foo/bar": {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
		}, fs)
	})

	tbRun(tb, "all directories exist", func(tb testing.TB) {
		setupFS, commit := setup(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo/bar", 0644))

		fs := mkdirAllFS(tb, commit())
		assert.NoError(tb, fs.MkdirAll("foo/bar", 0600))
		tryAssertEqualFS(tb, map[string]fsEntry{
			"foo":     {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"foo/bar": {Mode: hackpadfs.ModeDir | 0644, IsDir: true},
		}, fs)
	})

	tbRun(tb, "file exists", func(tb testing.TB) {
		setupFS, commit := setup(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())

		fs := mkdirAllFS(tb, commit())
		err = fs.MkdirAll("foo/bar", 0700)
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.Equal(tb, true, errors.Is(err, hackpadfs.ErrNotDir))
			assert.Equal(tb, "mkdir", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
		tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: 0666},
		}, fs)
	})

	tbRun(tb, "illegal permission bits", func(tb testing.TB) {
		_, commit := setup(tb)
		fs := mkdirAllFS(tb, commit())
		assert.NoError(tb, fs.MkdirAll("foo/bar", hackpadfs.ModeSocket|0777))
		tryAssertEqualFS(tb, map[string]fsEntry{
			"foo":     {Mode: hackpadfs.ModeDir | 0777, IsDir: true},
			"foo/bar": {Mode: hackpadfs.ModeDir | 0777, IsDir: true},
		}, fs)
	})
}

// Open opens the named file for reading.
// If successful, methods on the returned file can be used for reading; the associated file descriptor has mode O_RDONLY.
// If there is an error, it will be of type *PathError.
func TestOpen(tb testing.TB, setup SetupFSFunc) {
	testOpen(tb, setup, func(fs hackpadfs.FS, name string) (hackpadfs.File, error) {
		return fs.Open(name)
	})
}

func testOpen(tb testing.TB, setup SetupFSFunc, openFn func(hackpadfs.FS, string) (hackpadfs.File, error)) {
	tbRun(tb, "does not exist", func(tb testing.TB) {
		_, commit := setup(tb)
		fs := commit()
		_, err := openFn(fs, "foo")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.Equal(tb, true, errors.Is(err, hackpadfs.ErrNotExist))
			assert.Equal(tb, "open", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
	})

	tbRun(tb, "open file", func(tb testing.TB) {
		setupFS, commit := setup(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())

		fs := commit()
		f, err = openFn(fs, "foo")
		assert.NoError(tb, err)
		if assert.NotZero(tb, f) {
			assert.NoError(tb, f.Close())
		}
	})

	tbRun(tb, "supports reads", func(tb testing.TB) {
		const fileContents = `hello world`
		setupFS, commit := setup(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		assert.NoError(tb, err)
		n, err := hackpadfs.WriteFile(f, []byte(fileContents))
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())

		fs := commit()
		f, err = openFn(fs, "foo")
		assert.NoError(tb, err)
		buf := make([]byte, n)
		n2, err := io.ReadFull(f, buf)
		assert.NoError(tb, err)
		assert.Equal(tb, n, n2)
		assert.Equal(tb, fileContents, string(buf))
	})

	tbRun(tb, "fails writes", func(tb testing.TB) {
		setupFS, commit := setup(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())

		fs := commit()
		f, err = openFn(fs, "foo")
		assert.NoError(tb, err)
		_, err = hackpadfs.WriteFile(f, []byte(`bar`))
		assert.Error(tb, err)
		assert.NoError(tb, f.Close())
	})
}

/*
// OpenFile is the generalized open call; most users will use Open or Create instead.
// It opens the named file with specified flag (O_RDONLY etc.).
// If the file does not exist, and the O_CREATE flag is passed, it is created with mode perm (before umask).
// If successful, methods on the returned File can be used for I/O.
// If there is an error, it will be of type *PathError.
func TestOpenFile(tb testing.TB, setup SetupFSFunc) {
	tbRun(tb, "read-only", func(tb testing.TB) {
		testFsOpen(tb, undertest, expected, func(fs hackpadfs.Fs, name string) (hackpadfs.File, error) {
			return fs.OpenFile(name, hackpadfs.O_RDONLY, 0777)
		})
	})

	tbRun(tb, "create", func(tb testing.TB) {
		testFsCreate(tb, undertest, expected, func(fs hackpadfs.Fs, name string) (hackpadfs.File, error) {
			return fs.OpenFile(name, hackpadfs.O_RDWR|hackpadfs.O_CREATE, 0666)
		})
	})

	tbRun(tb, "create illegal perms", func(tb testing.TB) {
		f, err = fs.OpenFile("foo", hackpadfs.O_RDWR|hackpadfs.O_CREATE, hackpadfs.ModeSocket|0777)
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "truncate on existing file", func(tb testing.TB) {
		const fileContents = "hello world"

		f, err = fs.Create("foo")
		assert.NoError(tb, err)
		_, err = f.Write([]byte(fileContents))
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		_, err = fs.OpenFile("foo", hackpadfs.O_TRUNC, 0700)
		assert.NoError(tb, err)

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "truncate on non-existent file", func(tb testing.TB) {
		_, err := fs.OpenFile("foo", hackpadfs.O_TRUNC, 0700)
		assert.Error(tb, err)

		assert.True(tb, hackpadfs.IsNotExist(err))
		assert.IsType(tb, &hackpadfs.PathError{}, err)
		pathErr := err.(*hackpadfs.PathError)
		assert.Equal(tb, "open", pathErr.Op)
		assert.Equal(tb, "foo", strings.TrimPrefix(pathErr.Path, "/"))
	})

	tbRun(tb, "truncate on existing dir", func(tb testing.TB) {
		assert.NoError(tb, fs.Mkdir("foo", 0700))
		_, err := fs.OpenFile("foo", hackpadfs.O_TRUNC, 0700)
		assert.Error(tb, err)

		assert.True(tb, hackpadfs.IsDirErr(err))
		assert.IsType(tb, &hackpadfs.PathError{}, err)
		pathErr := err.(*hackpadfs.PathError)
		assert.Equal(tb, "open", pathErr.Op)
		assert.Equal(tb, "foo", strings.TrimPrefix(pathErr.Path, "/"))
	})

	tbRun(tb, "append flag writes to end", func(tb testing.TB) {
		const (
			fileContents1 = "hello world"
			fileContents2 = "sup "
		)

		f, err = fs.Create("foo")
		assert.NoError(tb, err)
		_, err = f.Write([]byte(fileContents1))
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		f, err = fs.OpenFile("foo", hackpadfs.O_RDWR|hackpadfs.O_APPEND, 0700)
		assert.NoError(tb, err)
		_, err = f.Write([]byte(fileContents2))
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})
}

// Remove removes the named file or (empty) directory. If there is an error, it will be of type *PathError.
func TestRemove(tb testing.TB, setup SetupFSFunc) {
	tbRun(tb, "remove file", func(tb testing.TB) {
		f, err = fs.Create("foo")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		assert.NoError(tb, fs.Remove("foo"))
		undertestStat := statFS(tb, expected.FS())

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "remove empty dir", func(tb testing.TB) {
		err = fs.Mkdir("foo", 0700)
		assert.NoError(tb, err)
		assert.NoError(tb, fs.Remove("foo"))

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "remove non-existing file", func(tb testing.TB) {
		err := fs.Remove("foo")
		assert.Error(tb, err)

		assert.True(tb, hackpadfs.IsNotExist(err))
		assert.IsType(tb, &hackpadfs.PathError{}, err)
		pathErr := err.(*hackpadfs.PathError)
		assert.Equal(tb, "remove", pathErr.Op)
		assert.Equal(tb, "foo", strings.TrimPrefix(pathErr.Path, "/"))
		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "remove non-empty dir", func(tb testing.TB) {
		err = fs.Mkdir("foo", 0700)
		assert.NoError(tb, err)
		f, err = fs.Create("foo/bar")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		err := fs.Remove("foo")
		assert.Error(tb, err)

		assert.True(tb, hackpadfs.IsExist(err))
		assert.IsType(tb, &hackpadfs.PathError{}, err)
		pathErr := err.(*hackpadfs.PathError)
		assert.Equal(tb, "remove", pathErr.Op)
		assert.Equal(tb, "foo", strings.TrimPrefix(pathErr.Path, "/"))
		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})
}

// RemoveAll removes path and any children it contains.
// It removes everything it can but returns the first error it encounters.
// If the path does not exist, RemoveAll returns nil (no error).
// If there is an error, it will be of type *PathError.
func TestRemoveAll(tb testing.TB, setup SetupFSFunc) {
	tbRun(tb, "remove file", func(tb testing.TB) {
		f, err = fs.Create("foo")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		assert.NoError(tb, fs.RemoveAll("foo"))
		undertestStat := statFS(tb, expected.FS())

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "remove empty dir", func(tb testing.TB) {
		err = fs.Mkdir("foo", 0700)
		assert.NoError(tb, err)
		assert.NoError(tb, fs.RemoveAll("foo"))

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "remove non-existing file", func(tb testing.TB) {
		assert.NoError(tb, fs.RemoveAll("foo"))

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "remove non-empty dir", func(tb testing.TB) {
		err = fs.Mkdir("foo", 0700)
		assert.NoError(tb, err)
		f, err = fs.Create("foo/bar")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		assert.NoError(tb, fs.RemoveAll("foo"))

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})
}

// Rename renames (moves) oldpath to newpath.
// If newpath already exists and is not a directory, Rename replaces it.
// OS-specific restrictions may apply when oldpath and newpath are in different directories.
// If there is an error, it will be of type *LinkError.
func TestRename(tb testing.TB, setup SetupFSFunc) {
	tbRun(tb, "inside same directory", func(tb testing.TB) {
		assert.NoError(tb, fs.Mkdir("foo", 0700))
		f, err = fs.Create("foo/bar")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		assert.NoError(tb, fs.Rename("foo/bar", "foo/baz"))

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "inside same directory in root", func(tb testing.TB) {
		f, err = fs.Create("bar")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		assert.NoError(tb, fs.Rename("bar", "baz"))

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "same file", func(tb testing.TB) {
		const fileContents = `hello world`
		assert.NoError(tb, fs.Mkdir("foo", 0700))
		f, err = fs.Create("foo/bar")
		assert.NoError(tb, err)
		_, err = f.Write([]byte(fileContents))
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		assert.NoError(tb, fs.Rename("foo/bar", "foo/bar"))

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "same directory", func(tb testing.TB) {
		assert.NoError(tb, fs.Mkdir("foo", 0700))
		err := fs.Rename("foo", "foo")

		assert.Error(tb, err)
		assert.True(tb, hackpadfs.IsExist(err))
		assert.IsType(tb, &hackpadfs.LinkError{}, err)
		linkErr := err.(*hackpadfs.LinkError)
		assert.Equal(tb, "rename", linkErr.Op)
		assert.Equal(tb, "foo", strings.TrimPrefix(linkErr.Old, "/"))
		assert.Equal(tb, "foo", strings.TrimPrefix(linkErr.New, "/"))

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "newpath is directory", func(tb testing.TB) {
		assert.NoError(tb, fs.Mkdir("foo", 0700))
		assert.NoError(tb, fs.Mkdir("bar", 0700))
		err := fs.Rename("foo", "bar")
		assert.Error(tb, err)

		assert.Error(tb, err)
		assert.True(tb, hackpadfs.IsExist(err))
		assert.IsType(tb, &hackpadfs.LinkError{}, err)
		linkErr := err.(*hackpadfs.LinkError)
		assert.Equal(tb, "rename", linkErr.Op)
		assert.Equal(tb, "foo", strings.TrimPrefix(linkErr.Old, "/"))
		assert.Equal(tb, "bar", strings.TrimPrefix(linkErr.New, "/"))

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "newpath in root", func(tb testing.TB) {
		const fileContents = `hello world`
		assert.NoError(tb, fs.Mkdir("foo", 0700))
		f, err = fs.Create("foo/bar")
		assert.NoError(tb, err)
		_, err = f.Write([]byte(fileContents))
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		assert.NoError(tb, fs.Rename("foo/bar", "baz"))

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	tbRun(tb, "newpath in subdirectory", func(tb testing.TB) {
		const fileContents = `hello world`
		assert.NoError(tb, fs.Mkdir("foo", 0700))
		f, err = fs.Create("bar")
		assert.NoError(tb, err)
		_, err = f.Write([]byte(fileContents))
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		assert.NoError(tb, fs.Rename("bar", "foo/baz"))

		tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})
}

// Stat returns a FileInfo describing the named file. If there is an error, it will be of type *PathError.
func TestStat(tb testing.TB, setup SetupFSFunc) {
	testStat(tb, undertest, expected, func(tb testing.TB, fsTest FSTester, path string) (hackpadfs.FileInfo, error) {
		return fsTest.FS().Stat(path)
	})
}

func testStat(tb testing.TB, undertest, expected FSTester, stater func(testing.TB, FSTester, string) (hackpadfs.FileInfo, error)) {
	tbRun(tb, "stat a file", func(tb testing.TB) {
		f, err = undertest.WriteFS().Create("foo")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		assert.NoError(tb, undertest.WriteFS().Chmod("foo", 0755))
		info, err := stater(tb, undertest, "foo")
		assert.NoError(tb, err)

		assertEqualFileInfo(tb, eInfo, info)
	})

	tbRun(tb, "stat a directory", func(tb testing.TB) {
		err = undertest.WriteFS().Mkdir("foo", 0755)
		assert.NoError(tb, err)
		info, err := stater(tb, undertest, "foo")
		assert.NoError(tb, err)

		assertEqualFileInfo(tb, eInfo, info)
	})

	tbRun(tb, "stat nested files", func(tb testing.TB) {
		err = undertest.WriteFS().Mkdir("foo", 0755)
		assert.NoError(tb, err)
		err = undertest.WriteFS().Mkdir("foo/bar", 0755)
		assert.NoError(tb, err)
		f, err = undertest.WriteFS().Create("foo/bar/baz")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		uInfo1, err := stater(tb, undertest, "foo/bar")
		assert.NoError(tb, err)
		uInfo2, err := stater(tb, undertest, "foo/bar/baz")
		assert.NoError(tb, err)

		assertEqualFileInfo(tb, eInfo1, uInfo1)
		assertEqualFileInfo(tb, eInfo2, uInfo2)
	})
}

// Chmod changes the mode of the named file to mode.
// If the file is a symbolic link, it changes the mode of the link's target.
// If there is an error, it will be of type *PathError.
//
// A different subset of the mode bits are used, depending on the operating system.
//
// fstest will only check permission bits
func TestChmod(tb testing.TB, setup SetupFSFunc) {
	tbRun(tb, "change permission bits", func(tb testing.TB) {
		f, err = fs.Create("foo")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())

		err = fs.Chmod("foo", 0755)
		assert.NoError(tb, err)
		info, err := fs.Stat("foo")
		assert.NoError(tb, err)
		assertEqualFileInfo(tb, eInfo, info)
	})

	uLinker, uOK := fs.(hackpadfs.Symlinker)
	eLinker, eOK := expected.FS().(hackpadfs.Symlinker)
	if !uOK {
		t.Skip("Skipping symlink tests, 'undertest' does not support hackpadfs.Symlinker")
	}
	if !eOK {
		t.Skip("Skipping symlink tests, 'expected' does not support hackpadfs.Symlinker")
	}

	tbRun(tb, "change symlink targets permission bits", func(tb testing.TB) {
		f, err = fs.Create("foo")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		assert.NoError(tb, uLinker.SymlinkIfPossible("foo", "bar"))

		err = fs.Chmod("foo", 0755)
		assert.NoError(tb, err)
		uLinkInfo, err := fs.Stat("foo")
		assert.NoError(tb, err)
		info, err := fs.Stat("bar")
		assert.NoError(tb, err)

		assertEqualFileInfo(tb, eLinkInfo, uLinkInfo)
		assertEqualFileInfo(tb, eInfo, info)
	})
}

// Chtimes changes the access and modification times of the named file, similar to the Unix utime() or utimes() functions.
//
// The underlying filesystem may truncate or round the values to a less precise time unit. If there is an error, it will be of type *PathError.
func TestChtimes(tb testing.TB, setup SetupFSFunc) {
	var (
		accessTime = time.Now()
		modifyTime = accessTime.Add(-1 * time.Minute)
	)

	tbRun(tb, "file does not exist", func(tb testing.TB) {
		err := fs.Chtimes("foo", accessTime, modifyTime)
		assert.Error(tb, err)

		assert.True(tb, hackpadfs.IsNotExist(err))
		assert.IsType(tb, &hackpadfs.PathError{}, err)
		pathErr := err.(*hackpadfs.PathError)
		assert.Equal(tb, "chtimes", pathErr.Op)
		assert.Equal(tb, "foo", strings.TrimPrefix(pathErr.Path, "/"))
	})

	tbRun(tb, "change access and modify times", func(tb testing.TB) {
		f, err = fs.Create("foo")
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())

		err = fs.Chtimes("foo", accessTime, modifyTime)
		assert.NoError(tb, err)
		info, err := fs.Stat("foo")
		assert.NoError(tb, err)

		assertEqualFileInfo(tb, eInfo, info)
	})
}

// TODO Symlink
*/
