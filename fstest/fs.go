package fstest

import (
	// Avoid importing "os" package in fstest if we can, since not all environments may be able to support it.
	// Not to mention it should compile a little faster. :)

	"fmt"
	"io"
	"testing"
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestBaseCreate(tb testing.TB, o FSOptions) {
	_, commit := o.Setup.FS(tb)
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

func TestBaseMkdir(tb testing.TB, o FSOptions) {
	_, commit := o.Setup.FS(tb)
	fs, ok := commit().(hackpadfs.MkdirFS)
	if !ok {
		tb.Skip("FS is not a MkdirFS")
	}

	err := fs.Mkdir("foo", 0600)
	assert.NoError(tb, err)
}

func TestBaseChmod(tb testing.TB, o FSOptions) {
	setupFS, commit := o.Setup.FS(tb)
	f, err := hackpadfs.Create(setupFS, "foo")
	if assert.NoError(tb, err) {
		assert.NoError(tb, f.Close())
	}

	fs, ok := commit().(hackpadfs.ChmodFS)
	if !ok {
		tb.Skip("FS is not a ChmodFS")
	}

	err = fs.Chmod("foo", 0755)
	assert.NoError(tb, err)
}

func TestBaseChtimes(tb testing.TB, o FSOptions) {
	var (
		accessTime = time.Now()
		modifyTime = accessTime.Add(-10 * time.Second)
	)
	setupFS, commit := o.Setup.FS(tb)
	f, err := hackpadfs.Create(setupFS, "foo")
	if assert.NoError(tb, err) {
		assert.NoError(tb, f.Close())
	}

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
	if info == nil {
		return quickInfo{}
	}
	isDir := info.IsDir()
	var size int64
	if !isDir {
		size = info.Size()
	}
	return quickInfo{
		Name:  info.Name(),
		Size:  size,
		Mode:  info.Mode(),
		IsDir: isDir,
	}
}

// TestCreate verifies fs.Create().
//
// Create creates or truncates the named file.
// If the file already exists, it is truncated.
// If the file does not exist, it is created with mode 0666 (before umask).
// If successful, methods on the returned File can be used for I/O; the associated file descriptor has mode O_RDWR.
// If there is an error, it will be of type *PathError.
func TestCreate(tb testing.TB, o FSOptions) {
	_, commit := o.Setup.FS(tb)
	if _, ok := commit().(hackpadfs.CreateFS); !ok {
		tb.Skip("FS is not a CreateFS")
	}
	testCreate(tb, o, func(fs hackpadfs.FS, name string) (hackpadfs.File, error) {
		if fs, ok := fs.(hackpadfs.CreateFS); ok {
			return fs.Create(name)
		}
		tb.Skip("FS is not a CreateFS")
		return nil, nil
	})
}

func testCreate(tb testing.TB, o FSOptions, createFn func(hackpadfs.FS, string) (hackpadfs.File, error)) {
	_, commit := o.Setup.FS(tb)
	f, err := createFn(commit(), "foo") // trigger tb.Skip() for incompatible FS's
	if assert.NoError(tb, err) {
		assert.NoError(tb, f.Close())
	}

	o.tbRun(tb, "new file", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		file, err := createFn(fs, "foo")
		assert.NoError(tb, err)
		if assert.NotZero(tb, file) {
			assert.NoError(tb, file.Close())
		}
		info, err := hackpadfs.Stat(fs, "foo")
		assert.NoError(tb, err)

		o.assertEqualQuickInfo(tb, quickInfo{
			Name: "foo",
			Mode: hackpadfs.FileMode(0666),
		}, asQuickInfo(info))
	})

	o.tbRun(tb, "existing file", func(tb testing.TB) {
		const fileContents = `hello world`
		setupFS, commit := o.Setup.FS(tb)

		file, err := hackpadfs.OpenFile(setupFS, "foo", hackpadfs.FlagReadWrite|hackpadfs.FlagCreate, 0755)
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(file, []byte(fileContents))
			assert.NoError(tb, err)
			assert.NoError(tb, file.Close())
		}

		fs := commit()
		file, err = createFn(fs, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, file.Close())
		}
		info, err := hackpadfs.Stat(fs, "foo")
		assert.NoError(tb, err)

		o.assertEqualQuickInfo(tb, quickInfo{
			Name: "foo",
			Mode: 0755,
		}, asQuickInfo(info))
	})

	o.tbRun(tb, "existing directory", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		fs := commit()
		_, err := createFn(fs, "foo")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.Equal(tb, "open", err.Op)
			assert.Equal(tb, "foo", err.Path)
			assert.ErrorIs(tb, hackpadfs.ErrIsDir, err)
		}
	})

	o.tbRun(tb, "parent directory must exist", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		_, err := createFn(fs, "foo/bar")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.Equal(tb, "open", err.Op)
			assert.Equal(tb, "foo/bar", err.Path)
			assert.ErrorIs(tb, hackpadfs.ErrNotExist, err)
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
func TestMkdir(tb testing.TB, o FSOptions) {
	mkdirFS := func(tb testing.TB, fs hackpadfs.FS) hackpadfs.MkdirFS {
		if fs, ok := fs.(hackpadfs.MkdirFS); ok {
			return fs
		}
		tb.Skip("FS is not a MkdirFS")
		return nil
	}

	o.tbRun(tb, "fail dir exists", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := mkdirFS(tb, commit())
		assert.NoError(tb, fs.Mkdir("foo", 0600))
		err := fs.Mkdir("foo", 0600)
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrExist, err)
			assert.Equal(tb, "mkdir", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: hackpadfs.ModeDir | 0600, IsDir: true},
		}, fs)
	})

	o.tbRun(tb, "fail file exists", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := mkdirFS(tb, commit())
		err = fs.Mkdir("foo", 0600)
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrExist, err)
			assert.Equal(tb, "mkdir", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: 0666},
		}, fs)
	})

	o.tbRun(tb, "create sub dir", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := mkdirFS(tb, commit())
		assert.NoError(tb, fs.Mkdir("foo", 0700))
		assert.NoError(tb, fs.Mkdir("foo/bar", 0600))

		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo":     {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"foo/bar": {Mode: hackpadfs.ModeDir | 0600, IsDir: true},
		}, fs)
	})

	o.tbRun(tb, "only permission bits allowed", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := mkdirFS(tb, commit())
		assert.NoError(tb, fs.Mkdir("foo", hackpadfs.ModeSocket|0755))

		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: hackpadfs.ModeDir | 0755, IsDir: true},
		}, fs)
	})

	o.tbRun(tb, "parent directory must exist", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := mkdirFS(tb, commit())

		err := fs.Mkdir("foo/bar", 0755)
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrNotExist, err)
			assert.Equal(tb, "mkdir", err.Op)
			assert.Equal(tb, "foo/bar", err.Path)
		}
		o.tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})
}

// MkdirAll creates a directory named path, along with any necessary parents, and returns nil, or else returns an error.
// The permission bits perm (before umask) are used for all directories that MkdirAll creates.
// If path is already a directory, MkdirAll does nothing and returns nil.
func TestMkdirAll(tb testing.TB, o FSOptions) {
	mkdirAllFS := func(tb testing.TB, fs hackpadfs.FS) hackpadfs.MkdirAllFS {
		if fs, ok := fs.(hackpadfs.MkdirAllFS); ok {
			return fs
		}
		tb.Skip("FS is not a MkdirAllFS")
		return nil
	}

	o.tbRun(tb, "create one directory", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := mkdirAllFS(tb, commit())
		assert.NoError(tb, fs.MkdirAll("foo", 0700))
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
		}, fs)
	})

	o.tbRun(tb, "create multiple directories", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := mkdirAllFS(tb, commit())
		assert.NoError(tb, fs.MkdirAll("foo/bar", 0700))
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo":     {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"foo/bar": {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
		}, fs)
	})

	o.tbRun(tb, "all directories exist", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo/bar", 0644))

		fs := mkdirAllFS(tb, commit())
		assert.NoError(tb, fs.MkdirAll("foo/bar", 0600))
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo":     {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"foo/bar": {Mode: hackpadfs.ModeDir | 0644, IsDir: true},
		}, fs)
	})

	o.tbRun(tb, "file exists", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := mkdirAllFS(tb, commit())
		err = fs.MkdirAll("foo/bar", 0700)
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrNotDir, err)
			assert.Equal(tb, "mkdir", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: 0666},
		}, fs)
	})

	o.tbRun(tb, "illegal permission bits", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := mkdirAllFS(tb, commit())
		assert.NoError(tb, fs.MkdirAll("foo/bar", hackpadfs.ModeSocket|0777))
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo":     {Mode: hackpadfs.ModeDir | 0777, IsDir: true},
			"foo/bar": {Mode: hackpadfs.ModeDir | 0777, IsDir: true},
		}, fs)
	})
}

// Open opens the named file for reading.
// If successful, methods on the returned file can be used for reading; the associated file descriptor has mode O_RDONLY.
// If there is an error, it will be of type *PathError.
func TestOpen(tb testing.TB, o FSOptions) {
	testOpen(tb, o, func(fs hackpadfs.FS, name string) (hackpadfs.File, error) {
		return fs.Open(name)
	})
}

func testOpen(tb testing.TB, o FSOptions, openFn func(hackpadfs.FS, string) (hackpadfs.File, error)) {
	_, commit := o.Setup.FS(tb)
	_, _ = openFn(commit(), "foo") // trigger tb.Skip() for incompatible FS's

	o.tbRun(tb, "invalid path", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		_, err := openFn(fs, "foo/../bar")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			tb.Log(err.Err)
			assert.ErrorIs(tb, hackpadfs.ErrInvalid, err)
			assert.Equal(tb, "open", err.Op)
			assert.Equal(tb, "foo/../bar", err.Path)
		}
	})

	o.tbRun(tb, "does not exist", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		_, err := openFn(fs, "foo")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrNotExist, err)
			assert.Equal(tb, "open", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
	})

	o.tbRun(tb, "open file", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := commit()
		f, err = openFn(fs, "foo")
		assert.NoError(tb, err)
		if assert.NotZero(tb, f) {
			assert.NoError(tb, f.Close())
		}
	})

	o.tbRun(tb, "supports reads", func(tb testing.TB) {
		const fileContents = `hello world`
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		var n int
		if assert.NoError(tb, err) {
			n, err = hackpadfs.WriteFile(f, []byte(fileContents))
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		}

		fs := commit()
		f, err = openFn(fs, "foo")
		if !assert.NoError(tb, err) {
			tb.FailNow()
		}
		buf := make([]byte, n)
		n2, err := io.ReadFull(f, buf)
		assert.NoError(tb, err)
		assert.Equal(tb, n, n2)
		assert.Equal(tb, fileContents, string(buf))
		assert.NoError(tb, f.Close())
	})

	o.tbRun(tb, "fails writes", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := commit()
		f, err = openFn(fs, "foo")
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(f, []byte(`bar`))
			assert.Error(tb, err)
			assert.NoError(tb, f.Close())
		}
	})
}

// OpenFile is the generalized open call; most users will use Open or Create instead.
// It opens the named file with specified flag (O_RDONLY etc.).
// If the file does not exist, and the O_CREATE flag is passed, it is created with mode perm (before umask).
// If successful, methods on the returned File can be used for I/O.
// If there is an error, it will be of type *PathError.
func TestOpenFile(tb testing.TB, o FSOptions) {
	openFileFS := func(tb testing.TB, fs hackpadfs.FS) hackpadfs.OpenFileFS {
		if fs, ok := fs.(hackpadfs.OpenFileFS); ok {
			return fs
		}
		tb.Skip("FS is not an OpenFileFS")
		return nil
	}
	o.tbRun(tb, "read-only", func(tb testing.TB) {
		testOpen(tb, o, func(fs hackpadfs.FS, name string) (hackpadfs.File, error) {
			return openFileFS(tb, fs).OpenFile(name, hackpadfs.FlagReadOnly, 0777)
		})
	})

	o.tbRun(tb, "create", func(tb testing.TB) {
		testCreate(tb, o, func(fs hackpadfs.FS, name string) (hackpadfs.File, error) {
			return openFileFS(tb, fs).OpenFile(name, hackpadfs.FlagReadWrite|hackpadfs.FlagCreate|hackpadfs.FlagTruncate, 0666)
		})
	})

	o.tbRun(tb, "create illegal perms", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := openFileFS(tb, commit())
		f, err := fs.OpenFile("foo", hackpadfs.FlagReadOnly|hackpadfs.FlagCreate, hackpadfs.ModeSocket|0777)
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: 0777},
		}, fs)
	})

	o.tbRun(tb, "truncate on existing file", func(tb testing.TB) {
		const fileContents = "hello world"
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(f, []byte(fileContents))
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		}

		fs := openFileFS(tb, commit())
		f, err = fs.OpenFile("foo", hackpadfs.FlagTruncate|hackpadfs.FlagWriteOnly, 0700)
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: 0666},
		}, fs)
	})

	o.tbRun(tb, "truncate on non-existent file", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := openFileFS(tb, commit())
		_, err := fs.OpenFile("foo", hackpadfs.FlagTruncate|hackpadfs.FlagWriteOnly, 0700)
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrNotExist, err)
			assert.Equal(tb, "open", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
	})

	o.tbRun(tb, "truncate on existing dir", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		fs := openFileFS(tb, commit())
		_, err := fs.OpenFile("foo", hackpadfs.FlagTruncate|hackpadfs.FlagWriteOnly, 0700)
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrIsDir, err)
			assert.Equal(tb, "open", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
	})

	o.tbRun(tb, "append flag writes to end", func(tb testing.TB) {
		const (
			fileContents1 = "hello world"
			fileContents2 = "sup "
		)

		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(f, []byte(fileContents1))
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		}

		fs := openFileFS(tb, commit())
		f, err = fs.OpenFile("foo", hackpadfs.FlagReadWrite|hackpadfs.FlagAppend, 0700)
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(f, []byte(fileContents2))
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		}
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: 0666, Size: int64(len(fileContents1) + len(fileContents2))},
		}, fs)
	})
}

// Remove removes the named file or (empty) directory. If there is an error, it will be of type *PathError.
func TestRemove(tb testing.TB, o FSOptions) {
	removeFS := func(tb testing.TB, fs hackpadfs.FS) hackpadfs.RemoveFS {
		if fs, ok := fs.(hackpadfs.RemoveFS); ok {
			return fs
		}
		tb.Skip("FS is not an RemoveFS")
		return nil
	}

	o.tbRun(tb, "remove file", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := removeFS(tb, commit())
		assert.NoError(tb, fs.Remove("foo"))
		o.tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	o.tbRun(tb, "remove empty dir", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))

		fs := removeFS(tb, commit())
		assert.NoError(tb, fs.Remove("foo"))
		o.tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	o.tbRun(tb, "remove non-existing file", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := removeFS(tb, commit())
		err := fs.Remove("foo")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrNotExist, err)
			assert.Equal(tb, "remove", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
		o.tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	o.tbRun(tb, "remove non-empty dir", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		f, err := hackpadfs.Create(setupFS, "foo/bar")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := removeFS(tb, commit())
		err = fs.Remove("foo")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrNotEmpty, err)
			assert.Equal(tb, "remove", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo":     {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"foo/bar": {Mode: 0666},
		}, fs)
	})
}

// RemoveAll removes path and any children it contains.
// It removes everything it can but returns the first error it encounters.
// If the path does not exist, RemoveAll returns nil (no error).
// If there is an error, it will be of type *PathError.
func TestRemoveAll(tb testing.TB, o FSOptions) {
	removeAllFS := func(tb testing.TB, fs hackpadfs.FS) hackpadfs.RemoveAllFS {
		if fs, ok := fs.(hackpadfs.RemoveAllFS); ok {
			return fs
		}
		tb.Skip("FS is not an RemoveAllFS")
		return nil
	}

	o.tbRun(tb, "remove file", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := removeAllFS(tb, commit())
		assert.NoError(tb, fs.RemoveAll("foo"))
		o.tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	o.tbRun(tb, "remove empty dir", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))

		fs := removeAllFS(tb, commit())
		assert.NoError(tb, fs.RemoveAll("foo"))
		o.tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	o.tbRun(tb, "remove non-existing file", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := removeAllFS(tb, commit())
		assert.NoError(tb, fs.RemoveAll("foo"))
		o.tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})

	o.tbRun(tb, "remove non-empty dir", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		f, err := hackpadfs.Create(setupFS, "foo/bar")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := removeAllFS(tb, commit())
		assert.NoError(tb, fs.RemoveAll("foo"))
		o.tryAssertEqualFS(tb, map[string]fsEntry{}, fs)
	})
}

// TestRename verifies fs.Rename().
//
// Rename renames (moves) oldpath to newpath.
// If newpath already exists and is not a directory, Rename replaces it.
// OS-specific restrictions may apply when oldpath and newpath are in different directories.
// If there is an error, it will be of type *LinkError.
func TestRename(tb testing.TB, o FSOptions) {
	renameFS := func(tb testing.TB, fs hackpadfs.FS) hackpadfs.RenameFS {
		if fs, ok := fs.(hackpadfs.RenameFS); ok {
			return fs
		}
		tb.Skip("FS is not an RenameFS")
		return nil
	}

	o.tbRun(tb, "oldpath does not exist", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := renameFS(tb, commit())
		err := fs.Rename("foo", "bar")
		if assert.IsType(tb, &hackpadfs.LinkError{}, err) {
			err := err.(*hackpadfs.LinkError)
			assert.ErrorIs(tb, hackpadfs.ErrNotExist, err)
			assert.Equal(tb, "rename", err.Op)
			assert.Equal(tb, "foo", err.Old)
			assert.Equal(tb, "bar", err.New)
		}
	})

	o.tbRun(tb, "inside same directory", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		f, err := hackpadfs.Create(setupFS, "foo/bar")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := renameFS(tb, commit())
		assert.NoError(tb, fs.Rename("foo/bar", "foo/baz"))
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo":     {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"foo/baz": {Mode: 0666},
		}, fs)
	})

	o.tbRun(tb, "inside same directory in root", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "bar")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := renameFS(tb, commit())
		assert.NoError(tb, fs.Rename("bar", "baz"))
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"baz": {Mode: 0666},
		}, fs)
	})

	o.tbRun(tb, "same file", func(tb testing.TB) {
		const fileContents = `hello world`
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		f, err := hackpadfs.Create(setupFS, "foo/bar")
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(f, []byte(fileContents))
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		}

		fs := renameFS(tb, commit())
		assert.NoError(tb, fs.Rename("foo/bar", "foo/bar"))
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo":     {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"foo/bar": {Mode: 0666, Size: int64(len(fileContents))},
		}, fs)
	})

	o.tbRun(tb, "same directory", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))

		fs := renameFS(tb, commit())
		err := fs.Rename("foo", "foo")

		if assert.Error(tb, err) {
			assert.ErrorIs(tb, hackpadfs.ErrExist, err)
			switch err := err.(type) {
			case *hackpadfs.LinkError:
				assert.Equal(tb, "rename", err.Op)
				assert.Equal(tb, "foo", err.Old)
				assert.Equal(tb, "foo", err.New)
			default:
				assert.Equal(tb, "*os.LinkError", fmt.Sprintf("%T", err))
				assert.Equal(tb, "rename foo foo: file exists", err.Error())
			}
		}
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
		}, fs)
	})

	o.tbRun(tb, "newpath is directory", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "bar", 0700))

		fs := renameFS(tb, commit())
		err := fs.Rename("foo", "bar")
		if assert.Error(tb, err) {
			assert.ErrorIs(tb, hackpadfs.ErrExist, err)
			switch err := err.(type) {
			case *hackpadfs.LinkError:
				assert.Equal(tb, "rename", err.Op)
				assert.Equal(tb, "foo", err.Old)
				assert.Equal(tb, "bar", err.New)
			default:
				assert.Equal(tb, "*os.LinkError", fmt.Sprintf("%T", err))
				assert.Equal(tb, "rename foo bar: file exists", err.Error())
			}
		}
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"bar": {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
		}, fs)
	})

	o.tbRun(tb, "newpath in root", func(tb testing.TB) {
		const fileContents = `hello world`
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		f, err := hackpadfs.Create(setupFS, "foo/bar")
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(f, []byte(fileContents))
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		}

		fs := renameFS(tb, commit())
		assert.NoError(tb, fs.Rename("foo/bar", "baz"))
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo": {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"baz": {Mode: 0666, Size: int64(len(fileContents))},
		}, fs)
	})

	o.tbRun(tb, "newpath in subdirectory", func(tb testing.TB) {
		const fileContents = `hello world`
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		f, err := hackpadfs.Create(setupFS, "bar")
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(f, []byte(fileContents))
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		}

		fs := renameFS(tb, commit())
		assert.NoError(tb, fs.Rename("bar", "foo/baz"))
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"foo":     {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"foo/baz": {Mode: 0666, Size: int64(len(fileContents))},
		}, fs)
	})

	o.tbRun(tb, "non-empty directory", func(tb testing.TB) {
		const fileContents = `hello world`
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.Mkdir(setupFS, "foo", 0700))
		f, err := hackpadfs.Create(setupFS, "foo/bar")
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(f, []byte(fileContents))
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		}

		fs := renameFS(tb, commit())
		assert.NoError(tb, fs.Rename("foo", "baz"))
		o.tryAssertEqualFS(tb, map[string]fsEntry{
			"baz":     {Mode: hackpadfs.ModeDir | 0700, IsDir: true},
			"baz/bar": {Mode: 0666, Size: int64(len(fileContents))},
		}, fs)
	})
}

// Stat returns a FileInfo describing the named file. If there is an error, it will be of type *PathError.
func TestStat(tb testing.TB, o FSOptions) {
	testStat(tb, o, func(tb testing.TB, fs hackpadfs.FS, path string) (hackpadfs.FileInfo, error) {
		if fs, ok := fs.(hackpadfs.StatFS); ok {
			return fs.Stat(path)
		}
		tb.Skip("FS is not an StatFS")
		return nil, nil
	})
}

func testStat(tb testing.TB, o FSOptions, stater func(testing.TB, hackpadfs.FS, string) (hackpadfs.FileInfo, error)) {
	o.tbRun(tb, "invalid path", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		_, err := stater(tb, fs, "foo/../bar")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.Equal(tb, "foo/../bar", err.Path)
			assert.ErrorIs(tb, hackpadfs.ErrInvalid, err)
		}
	})

	o.tbRun(tb, "stat root", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		info, err := stater(tb, fs, ".")
		if assert.NoError(tb, err) {
			assert.Equal(tb, true, info.IsDir())
		}
	})

	o.tbRun(tb, "stat a file", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}
		assert.NoError(tb, hackpadfs.Chmod(setupFS, "foo", 0755))

		fs := commit()
		info, err := stater(tb, fs, "foo")
		assert.NoError(tb, err)
		o.assertEqualQuickInfo(tb, quickInfo{
			Name: "foo",
			Mode: 0755,
		}, asQuickInfo(info))
		assert.NotPanics(tb, func() {
			_ = info.Sys()
		})
	})

	o.tbRun(tb, "stat a directory", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		err := hackpadfs.Mkdir(setupFS, "foo", 0755)
		assert.NoError(tb, err)

		fs := commit()
		info, err := stater(tb, fs, "foo")
		assert.NoError(tb, err)
		o.assertEqualQuickInfo(tb, quickInfo{
			Name:  "foo",
			Mode:  hackpadfs.ModeDir | 0755,
			IsDir: true,
		}, asQuickInfo(info))
	})

	o.tbRun(tb, "stat nested files", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		err := hackpadfs.Mkdir(setupFS, "foo", 0755)
		assert.NoError(tb, err)
		err = hackpadfs.Mkdir(setupFS, "foo/bar", 0755)
		assert.NoError(tb, err)
		f, err := hackpadfs.Create(setupFS, "foo/bar/baz")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := commit()
		info1, err := stater(tb, fs, "foo/bar")
		assert.NoError(tb, err)
		info2, err := stater(tb, fs, "foo/bar/baz")
		assert.NoError(tb, err)
		o.assertEqualQuickInfo(tb, quickInfo{
			Name:  "bar",
			Mode:  hackpadfs.ModeDir | 0755,
			IsDir: true,
		}, asQuickInfo(info1))
		o.assertEqualQuickInfo(tb, quickInfo{
			Name: "baz",
			Mode: 0666,
		}, asQuickInfo(info2))
	})
}

// Chmod changes the mode of the named file to mode.
// If the file is a symbolic link, it changes the mode of the link's target.
// If there is an error, it will be of type *PathError.
//
// A different subset of the mode bits are used, depending on the operating system.
//
// fstest will only check permission bits
func TestChmod(tb testing.TB, o FSOptions) {
	chmodFS := func(tb testing.TB, fs hackpadfs.FS) hackpadfs.ChmodFS {
		if fs, ok := fs.(hackpadfs.ChmodFS); ok {
			return fs
		}
		tb.Skip("FS is not an ChmodFS")
		return nil
	}

	o.tbRun(tb, "change permission bits", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := chmodFS(tb, commit())
		err = fs.Chmod("foo", 0755)
		assert.NoError(tb, err)
		info, err := hackpadfs.Stat(fs, "foo")
		assert.NoError(tb, err)
		o.assertEqualQuickInfo(tb, quickInfo{
			Name: "foo",
			Mode: 0755,
		}, asQuickInfo(info))
	})

	o.tbRun(tb, "change symlink target permission bits", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		if _, ok := setupFS.(hackpadfs.SymlinkFS); !ok {
			tb.Skip("FS is not an SymlinkFS")
		}
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}
		assert.NoError(tb, hackpadfs.Symlink(setupFS, "foo", "bar"))

		fs := chmodFS(tb, commit())
		err = fs.Chmod("foo", 0755)
		assert.NoError(tb, err)
		linkInfo, err := hackpadfs.Stat(fs, "foo")
		assert.NoError(tb, err)
		info, err := hackpadfs.Stat(fs, "bar")
		assert.NoError(tb, err)
		o.assertEqualQuickInfo(tb, quickInfo{
			Name: "foo",
			Mode: 0755,
		}, asQuickInfo(linkInfo))
		o.assertEqualQuickInfo(tb, quickInfo{
			Name: "bar",
			Mode: 0755,
		}, asQuickInfo(info))
	})
}

// Chtimes changes the access and modification times of the named file, similar to the Unix utime() or utimes() functions.
//
// The underlying filesystem may truncate or round the values to a less precise time unit. If there is an error, it will be of type *PathError.
func TestChtimes(tb testing.TB, o FSOptions) {
	var (
		accessTime = time.Now()
		modifyTime = accessTime.Add(-1 * time.Minute)
	)

	chtimesFS := func(tb testing.TB, fs hackpadfs.FS) hackpadfs.ChtimesFS {
		if fs, ok := fs.(hackpadfs.ChtimesFS); ok {
			return fs
		}
		tb.Skip("FS is not an ChtimesFS")
		return nil
	}

	o.tbRun(tb, "file does not exist", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := chtimesFS(tb, commit())
		err := fs.Chtimes("foo", accessTime, modifyTime)
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrNotExist, err)
			assert.Equal(tb, "chtimes", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
	})

	o.tbRun(tb, "change access and modify times", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := chtimesFS(tb, commit())
		err = fs.Chtimes("foo", accessTime, modifyTime)
		assert.NoError(tb, err)
		info, err := hackpadfs.Stat(fs, "foo")
		assert.NoError(tb, err)
		if o.assertEqualQuickInfo(tb, quickInfo{
			Name: "foo",
			Mode: 0666,
		}, asQuickInfo(info)) {
			assert.Equal(tb, modifyTime.Format(time.RFC3339), info.ModTime().Local().Format(time.RFC3339))
		}
	})
}

func TestReadFile(tb testing.TB, o FSOptions) {
	readFileFS := func(tb testing.TB, fs hackpadfs.FS) hackpadfs.ReadFileFS {
		if fs, ok := fs.(hackpadfs.ReadFileFS); ok {
			return fs
		}
		tb.Skip("FS is not a ReadFileFS")
		return nil
	}

	o.tbRun(tb, "not exists", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := readFileFS(tb, commit())
		_, err := fs.ReadFile("foo")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrNotExist, err)
			assert.Equal(tb, "open", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
	})

	o.tbRun(tb, "exists", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		const contents = "hello"
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			_, err := hackpadfs.WriteFile(f, []byte(contents))
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		}

		fs := readFileFS(tb, commit())
		buf, err := fs.ReadFile("foo")
		assert.NoError(tb, err)
		assert.Equal(tb, []byte(contents), buf)
	})
}

func TestReadDir(tb testing.TB, o FSOptions) {
	readDirFS := func(tb testing.TB, fs hackpadfs.FS) hackpadfs.ReadDirFS {
		if fs, ok := fs.(hackpadfs.ReadDirFS); ok {
			return fs
		}
		tb.Skip("FS is not a ReadDirFS")
		return nil
	}

	o.tbRun(tb, "exists", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		err := hackpadfs.MkdirAll(setupFS, "foo/bar", 0700)
		assert.NoError(tb, err)
		f, err := hackpadfs.Create(setupFS, "foo/baz")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}
		f, err = hackpadfs.Create(setupFS, "foo/biff")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}
		fs := readDirFS(tb, commit())
		dir, err := fs.ReadDir("foo")
		assert.NoError(tb, err)
		if assert.Equal(tb, 3, len(dir)) {
			// entries should be sorted alphabetically

			// dir entry 0
			assert.Equal(tb, "bar", dir[0].Name())
			info, err := dir[0].Info()
			assert.NoError(tb, err)
			assert.Equal(tb, quickInfo{
				Name:  "bar",
				Mode:  hackpadfs.ModeDir | 0700,
				IsDir: true,
			}, asQuickInfo(info))
			assert.Equal(tb, true, dir[0].IsDir())
			assert.Equal(tb, hackpadfs.ModeDir, dir[0].Type())

			// dir entry 1
			assert.Equal(tb, "baz", dir[1].Name())
			info, err = dir[1].Info()
			assert.NoError(tb, err)
			assert.Equal(tb, quickInfo{
				Name:  "baz",
				Mode:  0666,
				IsDir: false,
			}, asQuickInfo(info))
			assert.Equal(tb, false, dir[1].IsDir())
			assert.Equal(tb, hackpadfs.FileMode(0), dir[1].Type())

			// dir entry 2
			assert.Equal(tb, "biff", dir[2].Name())
			info, err = dir[2].Info()
			assert.NoError(tb, err)
			assert.Equal(tb, quickInfo{
				Name:  "biff",
				Mode:  0666,
				IsDir: false,
			}, asQuickInfo(info))
			assert.Equal(tb, false, dir[2].IsDir())
			assert.Equal(tb, hackpadfs.FileMode(0), dir[2].Type())
		}
	})

	o.tbRun(tb, "not exists", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := readDirFS(tb, commit())
		_, err := fs.ReadDir("foo")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrNotExist, err)
			assert.Equal(tb, "open", err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
	})

	o.tbRun(tb, "file is not a dir", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := readDirFS(tb, commit())
		_, err = fs.ReadDir("foo")
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrNotDir, err)
			assert.Contains(tb, []string{
				"fdopendir",
				"readdirent",
			}, err.Op)
			assert.Equal(tb, "foo", err.Path)
		}
	})
}

// TODO Symlink
