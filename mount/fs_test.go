package mount_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/internal/assert"
	"github.com/hack-pad/hackpadfs/mem"
	"github.com/hack-pad/hackpadfs/mount"
)

func TestFS(t *testing.T) {
	t.Parallel()

	requireNoError := func(tb testing.TB, err error) {
		if !assert.NoError(tb, err) {
			tb.FailNow()
		}
	}

	options := fstest.FSOptions{
		Name: "mount",
		TestFS: func(tb testing.TB) fstest.SetupFS {
			mem, err := mem.NewFS()
			requireNoError(tb, err)
			fs, err := mount.NewFS(mem)
			requireNoError(tb, err)
			return &allMountFS{fs}
		},
	}
	fstest.FS(t, options)
	fstest.File(t, options)

	options = fstest.FSOptions{
		Name: "mount unused",
		TestFS: func(tb testing.TB) fstest.SetupFS {
			memRoot, err := mem.NewFS()
			requireNoError(tb, err)
			requireNoError(tb, memRoot.Mkdir("unused", 0666))
			memUnused, err := mem.NewFS()
			requireNoError(tb, err)
			fs, err := mount.NewFS(memRoot)
			requireNoError(tb, fs.AddMount("unused", memUnused))
			return &allMountFS{fs}
		},
	}
	fstest.FS(t, options)
	fstest.File(t, options)
}

func TestAddMount(t *testing.T) {
	newFS := func(t *testing.T) *mount.FS {
		memRoot, err := mem.NewFS()
		assert.NoError(t, err)
		fs, err := mount.NewFS(memRoot)
		assert.NoError(t, err)
		return fs
	}

	t.Run("invalid path", func(t *testing.T) {
		fs := newFS(t)
		memFoo, err := mem.NewFS()
		assert.NoError(t, err)
		err = fs.AddMount("foo/../foo", memFoo)
		assert.Error(t, err)
		assert.Equal(t, true, errors.Is(err, hackpadfs.ErrInvalid))
		assert.Equal(t, 0, len(fs.MountPoints()))
	})

	t.Run("no file at mount point", func(t *testing.T) {
		fs := newFS(t)
		memFoo, err := mem.NewFS()
		assert.NoError(t, err)
		err = fs.AddMount("foo", memFoo)
		assert.Error(t, err)
		assert.Equal(t, true, errors.Is(err, hackpadfs.ErrNotExist))
		assert.Equal(t, 0, len(fs.MountPoints()))
	})

	t.Run("mount point not a directory", func(t *testing.T) {
		fs := newFS(t)
		memFoo, err := mem.NewFS()
		assert.NoError(t, err)
		f, err := hackpadfs.Create(fs, "foo")
		assert.NoError(t, err)
		assert.NoError(t, f.Close())

		err = fs.AddMount("foo", memFoo)
		assert.Error(t, err)
		assert.Equal(t, true, errors.Is(err, hackpadfs.ErrNotDir))
		assert.Equal(t, 0, len(fs.MountPoints()))
	})

	t.Run("mount point already exists", func(t *testing.T) {
		fs := newFS(t)
		memFoo, err := mem.NewFS()
		assert.NoError(t, err)
		assert.NoError(t, hackpadfs.Mkdir(fs, "foo", 0700))
		err = fs.AddMount("foo", memFoo)
		assert.NoError(t, err)

		err = fs.AddMount("foo", memFoo)
		assert.Error(t, err)
		assert.Equal(t, true, errors.Is(err, hackpadfs.ErrExist))
		assert.Equal(t, []mount.Point{
			{Path: "foo"},
		}, fs.MountPoints())
	})

	t.Run("concurrent conflicting mounts only succeed once", func(t *testing.T) {
		fs := newFS(t)
		memFoo, err := mem.NewFS()
		assert.NoError(t, err)
		assert.NoError(t, hackpadfs.Mkdir(fs, "foo", 0700))

		var wg sync.WaitGroup
		const maxAttempts = 3
		wg.Add(maxAttempts)
		errs := make([]error, maxAttempts)
		for i := range errs {
			go func(i int) {
				defer wg.Done()
				errs[i] = fs.AddMount("foo", memFoo)
			}(i)
		}
		wg.Wait()

		isNil := 0
		for i := range errs {
			if errs[i] == nil {
				isNil++
			}
		}
		assert.Equal(t, 1, isNil)
		assert.Equal(t, []mount.Point{
			{Path: "foo"},
		}, fs.MountPoints())
	})
}

func TestMount(t *testing.T) {
	memRoot, err := mem.NewFS()
	assert.NoError(t, err)
	fs, err := mount.NewFS(memRoot)
	assert.NoError(t, err)
	assert.NoError(t, hackpadfs.Mkdir(fs, "foo", 0700))

	{
		memFoo, err := mem.NewFS()
		assert.NoError(t, err)
		assert.NoError(t, fs.AddMount("foo", memFoo))
	}

	assert.NoError(t, hackpadfs.Mkdir(fs, "foo/bar", 0700))
	info, err := hackpadfs.Stat(fs, "foo/bar")
	if assert.NoError(t, err) {
		assert.Equal(t, true, info.IsDir())
		assert.Equal(t, hackpadfs.FileMode(hackpadfs.ModeDir|0700), info.Mode())
	}
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
