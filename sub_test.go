package hackpadfs_test

import (
	"path"
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/internal/assert"
	"github.com/hack-pad/hackpadfs/mem"
)

func TestSub(t *testing.T) {
	t.Parallel()
	options := fstest.FSOptions{
		Name:  "sub",
		Setup: fstest.TestSetupFunc(setupSubFS),
	}
	data := fstest.FS(t, options)
	assert.Zero(t, data.Skips)

	options.Constraints = fstest.Constraints{
		AllowErrPathPrefix: true,
	}
	data = fstest.File(t, options)
	assert.Zero(t, data.Skips)
}

func setupSubFS(tb testing.TB) (fstest.SetupFS, func() hackpadfs.FS) {
	memRoot, err := mem.NewFS()
	requireNoError(tb, err)
	return memRoot, func() hackpadfs.FS {
		const subDir = "subfs-subdir"
		requireNoError(tb, memRoot.Mkdir(subDir, 0o700))
		dirEntries, err := hackpadfs.ReadDir(memRoot, ".")
		requireNoError(tb, err)
		for _, entry := range dirEntries {
			if entry.Name() == subDir {
				continue
			}
			err := memRoot.Rename(entry.Name(), path.Join(subDir, entry.Name()))
			requireNoError(tb, err)
		}

		fs, err := hackpadfs.Sub(memRoot, subDir)
		requireNoError(tb, err)
		return fs
	}
}
