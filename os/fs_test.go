//go:build !wasm
// +build !wasm

package os

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestRootedPathGOOS(t *testing.T) {
	for _, tc := range []struct {
		description string
		root        string
		volumeName  string
		goos        string
		name        string
		expectPath  string
		expectErr   string
	}{
		{
			description: "root path unix",
			goos:        goosLinux,
			name:        ".",
			expectPath:  "/",
		},
		{
			description: "root path windows",
			goos:        goosWindows,
			name:        ".",
			expectPath:  `C:\`,
		},
		{
			description: "sub path unix",
			goos:        goosLinux,
			name:        "foo",
			expectPath:  "/foo",
		},
		{
			description: "sub path windows",
			goos:        goosWindows,
			name:        "foo",
			expectPath:  `C:\foo`,
		},
		{
			description: "root sub path unix",
			root:        "foo",
			goos:        goosLinux,
			name:        "bar",
			expectPath:  "/foo/bar",
		},
		{
			description: "root sub path windows",
			root:        "foo",
			goos:        goosWindows,
			name:        "bar",
			expectPath:  `C:\foo\bar`,
		},
		{
			description: "letter volume path windows",
			volumeName:  `D:`,
			goos:        goosWindows,
			name:        "foo",
			expectPath:  `D:\foo`,
		},
		{
			description: "letter volume sub path windows",
			root:        "foo",
			volumeName:  `D:`,
			goos:        goosWindows,
			name:        "bar",
			expectPath:  `D:\foo\bar`,
		},
		{
			description: "UNC volume path windows",
			volumeName:  `\\some-host\share`,
			goos:        goosWindows,
			name:        "foo",
			expectPath:  `\\some-host\share\foo`,
		},
		{
			description: "UNC volume sub path windows",
			root:        "foo",
			volumeName:  `\\some-host\share`,
			goos:        goosWindows,
			name:        "bar",
			expectPath:  `\\some-host\share\foo\bar`,
		},
	} {
		tc := tc // enable parallel sub-tests
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			fs := &FS{
				root:       tc.root,
				volumeName: tc.volumeName,
			}
			sep := '/'
			if tc.goos == goosWindows {
				sep = '\\'
			}
			path, err := fs.rootedPathGOOS(tc.goos, sep, "test", tc.name)
			if tc.expectErr != "" {
				if assert.Error(t, err) {
					assert.Equal(t, tc.expectErr, err.Error())
				}
				return
			}
			assert.Equal(t, tc.expectPath, path)
			assert.Zero(t, err)
		})
	}
}

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
	fstest.FS(t, options)
	fstest.File(t, options)
}
