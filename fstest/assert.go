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
func (o FSOptions) tryAssertEqualFS(tb testing.TB, expected map[string]fsEntry, actual hackpadfs.FS) {
	tb.Helper()
	fs, ok := actual.(hackpadfs.ReadDirFS)
	if !ok {
		return
	}

	for path, entry := range expected {
		entry.Mode = entry.Mode & o.Constraints.FileModeMask
		expected[path] = entry
	}

	entries := make(map[string]fsEntry)
	o.walkFSEntries(tb, fs, entries, "")
	assert.Subset(tb, expected, entries)
}

func (o FSOptions) walkFSEntries(tb testing.TB, fs hackpadfs.ReadDirFS, entries map[string]fsEntry, dir string) {
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
		mode = mode & o.Constraints.FileModeMask

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
			o.walkFSEntries(tb, fs, entries, filePath)
		}
	}
}

func (o FSOptions) assertEqualQuickInfo(tb testing.TB, a, b quickInfo) bool {
	tb.Helper()
	a.Mode = a.Mode & o.Constraints.FileModeMask
	b.Mode = b.Mode & o.Constraints.FileModeMask
	return assert.Equal(tb, a, b)
}

func (o FSOptions) assertEqualQuickInfos(tb testing.TB, a, b []quickInfo) bool {
	tb.Helper()
	for i := range a {
		a[i].Mode = a[i].Mode & o.Constraints.FileModeMask
	}
	for i := range b {
		b[i].Mode = b[i].Mode & o.Constraints.FileModeMask
	}
	return assert.Equal(tb, a, b)
}
