package mount_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/internal/assert"
	"github.com/hack-pad/hackpadfs/internal/mounttest"
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
			return mounttest.NewFS(fs)
		},
	}
	fstest.FS(t, options)
	fstest.File(t, options)

	options = fstest.FSOptions{
		Name: "mount unused",
		TestFS: func(tb testing.TB) fstest.SetupFS {
			memRoot, err := mem.NewFS()
			requireNoError(tb, err)
			requireNoError(tb, memRoot.Mkdir("unused", 0o666))
			memUnused, err := mem.NewFS()
			requireNoError(tb, err)
			fs, err := mount.NewFS(memRoot)
			requireNoError(tb, err)
			requireNoError(tb, fs.AddMount("unused", memUnused))
			return mounttest.NewFS(fs)
		},
	}
	fstest.FS(t, options)
	fstest.File(t, options)
}

func TestAddMount(t *testing.T) {
	t.Parallel()
	newFS := func(t *testing.T) *mount.FS {
		memRoot, err := mem.NewFS()
		assert.NoError(t, err)
		fs, err := mount.NewFS(memRoot)
		assert.NoError(t, err)
		return fs
	}

	t.Run("invalid path", func(t *testing.T) {
		t.Parallel()
		fs := newFS(t)
		memFoo, err := mem.NewFS()
		assert.NoError(t, err)
		err = fs.AddMount("foo/../foo", memFoo)
		assert.Error(t, err)
		assert.Equal(t, true, errors.Is(err, hackpadfs.ErrInvalid))
		assert.Equal(t, 0, len(fs.MountPoints()))
	})

	t.Run("no file at mount point", func(t *testing.T) {
		t.Parallel()
		fs := newFS(t)
		memFoo, err := mem.NewFS()
		assert.NoError(t, err)
		err = fs.AddMount("foo", memFoo)
		assert.Error(t, err)
		assert.Equal(t, true, errors.Is(err, hackpadfs.ErrNotExist))
		assert.Equal(t, 0, len(fs.MountPoints()))
	})

	t.Run("mount point not a directory", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
		fs := newFS(t)
		memFoo, err := mem.NewFS()
		assert.NoError(t, err)
		assert.NoError(t, hackpadfs.Mkdir(fs, "foo", 0o700))
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
		t.Parallel()
		fs := newFS(t)
		memFoo, err := mem.NewFS()
		assert.NoError(t, err)
		assert.NoError(t, hackpadfs.Mkdir(fs, "foo", 0o700))

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
	t.Parallel()
	memRoot, err := mem.NewFS()
	assert.NoError(t, err)
	fs, err := mount.NewFS(memRoot)
	assert.NoError(t, err)
	assert.NoError(t, hackpadfs.Mkdir(fs, "foo", 0o700))

	{
		memFoo, err := mem.NewFS()
		assert.NoError(t, err)
		assert.NoError(t, fs.AddMount("foo", memFoo))
	}

	assert.NoError(t, hackpadfs.Mkdir(fs, "foo/bar", 0o700))
	info, err := hackpadfs.Stat(fs, "foo/bar")
	if assert.NoError(t, err) {
		assert.Equal(t, true, info.IsDir())
		assert.Equal(t, hackpadfs.FileMode(hackpadfs.ModeDir|0o700), info.Mode())
	}
}
