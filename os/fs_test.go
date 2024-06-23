//go:build !wasm
// +build !wasm

package os

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestFSTest(t *testing.T) {
	t.Parallel()
	oldmask := setUmask(0)
	t.Cleanup(func() {
		setUmask(oldmask)
	})

	options := fstest.FSOptions{
		Name: "osfs.FS",
		TestFS: func(tb testing.TB) fstest.SetupFS {
			fs := NewFS()
			dir := tb.TempDir()
			volumeName := filepath.VolumeName(dir)
			if volumeName != "" {
				subvFS, err := fs.SubVolume(volumeName)
				if !assert.NoError(tb, err) {
					tb.FailNow()
				}
				fs = subvFS.(*FS)
				dir = dir[len(volumeName)+1:]
			} else {
				dir = strings.TrimPrefix(dir, "/")
			}
			subFS, err := fs.Sub(dir)
			if !assert.NoError(tb, err) {
				tb.FailNow()
			}
			return subFS.(*FS)
		},
	}
	var skipFacets []fstest.Facets
	if runtime.GOOS == goosWindows {
		options.Constraints.FileModeMask = 0o200 // Windows does not support the typical file permission bits. Only the "owner writable" bit is supported.
		skipFacets = []fstest.Facets{
			{Name: "TestFSTest/osfs.FS_File/file.Seek/seek_unknown_start"},                 // Windows ignores invalid 'whence' values in Seek() calls.
			{Name: "TestFSTest/osfs.FS_FS/fs.Rename/same_directory"},                       // Windows does not return an error for renaming a directory to itself.
			{Name: "TestFSTest/osfs.FS_FS/fs.Rename/newpath_is_directory"},                 // Windows returns an access denied error when renaming a file to an existing directory.
			{Name: "TestFSTest/osfs.FS_FS/fs.Chmod/change_symlink_target_permission_bits"}, // Windows requires elevated permissions to create symlinks (sometimes).
		}
	}
	options.ShouldSkip = func(facets fstest.Facets) bool {
		for _, f := range skipFacets {
			if f.Name == facets.Name {
				return true
			}
		}
		return false
	}

	data := fstest.FS(t, options)
	assert.Subset(t, data.Skips, skipFacets)
	data = fstest.File(t, options)
	assert.Subset(t, data.Skips, skipFacets)
}
