package cache_test

import (
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/cache"
	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/internal/assert"
	"github.com/hack-pad/hackpadfs/mem"
)

func TestFS(t *testing.T) {
	t.Parallel()
	options := fstest.FSOptions{
		Name: "cache",
		Setup: fstest.TestSetupFunc(func(tb testing.TB) (fstest.SetupFS, func() hackpadfs.FS) {
			sourceFS, err := mem.NewFS()
			if !assert.NoError(tb, err) {
				tb.FailNow()
			}
			cacheFS, err := mem.NewFS()
			if !assert.NoError(tb, err) {
				tb.FailNow()
			}
			fs, err := cache.NewReadOnlyFS(sourceFS, cacheFS, cache.ReadOnlyOptions{})
			if !assert.NoError(tb, err) {
				tb.FailNow()
			}
			return sourceFS, func() hackpadfs.FS {
				return fs
			}
		}),
	}
	fstest.FS(t, options)
	fstest.File(t, options)
}
