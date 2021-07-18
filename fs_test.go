package hackpadfs_test

import (
	"testing"

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
