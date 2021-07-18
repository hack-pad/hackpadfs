package hackpadfs_test

import (
	"errors"
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
	options := fstest.FSOptions{
		Name: "hackpadfs",
		TestFS: func(tb testing.TB) fstest.SetupFS {
			memRoot, err := mem.NewFS()
			requireNoError(tb, err)
			fs, err := mount.NewFS(memRoot)
			requireNoError(tb, err)
			return mounttest.NewFS(fs)
		},
	}
	fstest.FS(t, options)
	fstest.File(t, options)
}

func requireNoError(tb testing.TB, err error) {
	if !assert.NoError(tb, err) {
		tb.FailNow()
	}
}

type simpler interface {
	hackpadfs.FS
	hackpadfs.OpenFileFS
	hackpadfs.MkdirFS
	hackpadfs.RemoveFS
}

type simplerFS struct {
	simpler
}

func makeSimplerFS(t *testing.T) *simplerFS {
	fs, err := mem.NewFS()
	requireNoError(t, err)
	return &simplerFS{fs}
}

func TestMkdirAllFS(t *testing.T) {
	t.Parallel()

	t.Run("invalid path", func(t *testing.T) {
		t.Parallel()
		fs := makeSimplerFS(t)
		err := hackpadfs.MkdirAll(fs, "foo/../bar", 0700)
		if assert.IsType(t, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.Equal(t, "mkdirall", err.Op)
			assert.Equal(t, "foo/../bar", err.Path)
			assert.Equal(t, true, errors.Is(err, hackpadfs.ErrInvalid))
		}
	})

	t.Run("make all", func(t *testing.T) {
		t.Parallel()
		fs := makeSimplerFS(t)
		err := hackpadfs.MkdirAll(fs, "foo/bar", 0700)
		assert.NoError(t, err)
	})

	t.Run("make once", func(t *testing.T) {
		t.Parallel()
		fs := makeSimplerFS(t)
		assert.NoError(t, fs.simpler.Mkdir("foo", 0600))
		err := hackpadfs.MkdirAll(fs, "foo/bar", 0700)
		assert.NoError(t, err)
	})

	t.Run("file exists", func(t *testing.T) {
		t.Parallel()
		fs := makeSimplerFS(t)
		f, err := fs.simpler.OpenFile("foo", hackpadfs.FlagReadWrite|hackpadfs.FlagCreate, 0600)
		requireNoError(t, err)
		requireNoError(t, f.Close())
		err = hackpadfs.MkdirAll(fs, "foo/bar", 0700)
		if assert.IsType(t, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.Equal(t, "mkdir", err.Op)
			assert.Equal(t, "foo", err.Path)
			assert.Equal(t, true, errors.Is(err, hackpadfs.ErrNotDir))
		}
	})
}
