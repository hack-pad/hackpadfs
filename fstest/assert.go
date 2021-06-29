package fstest

import (
	"path"
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

type fsEntry struct {
	Size  int64
	Mode  hackpadfs.FileMode
	IsDir bool
}

// tryAssertEqualFS asserts that actual is equal to the file info records in expected. If actual doesn't support ReadDir, the assertion is skipped.
func tryAssertEqualFS(tb testing.TB, expected map[string]fsEntry, actual hackpadfs.FS) {
	tb.Helper()
	fs, ok := actual.(hackpadfs.ReadDirFS)
	if !ok {
		return
	}

	entries := make(map[string]fsEntry)
	walkFSEntries(tb, fs, entries, "")
	assert.Equal(tb, expected, entries)
}

func walkFSEntries(tb testing.TB, fs hackpadfs.ReadDirFS, entries map[string]fsEntry, dir string) {
	tb.Helper()
	if dir == "" {
		dir = "."
	}
	dirs, err := fs.ReadDir(dir)
	assert.NoError(tb, err)
	for _, entry := range dirs {
		isDir := entry.IsDir()
		mode := entry.Type()
		var size int64
		info, err := entry.Info()
		if assert.NoError(tb, err) {
			mode = info.Mode()
			if !isDir {
				size = info.Size()
			}
		}

		name := entry.Name()
		assert.Equal(tb, true, hackpadfs.ValidPath(name))
		filePath := path.Join(dir, name)
		_, exists := entries[filePath]
		assert.Equal(tb, false, exists) // must not hit the same file path twice
		entries[filePath] = fsEntry{
			Size:  size,
			Mode:  mode,
			IsDir: isDir,
		}

		if isDir {
			walkFSEntries(tb, fs, entries, filePath)
		}
	}
}
