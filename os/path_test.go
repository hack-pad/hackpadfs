//go:build !wasm
// +build !wasm

package os

import (
	"testing"

	"github.com/hack-pad/hackpadfs/internal/assert"
)

const (
	goosLinux = "linux"
)

func TestToOSPath(t *testing.T) {
	t.Parallel()
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
			path, err := fs.toOSPath(tc.goos, sep, "test", tc.name)
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

func TestToOSPathErr(t *testing.T) {
	t.Parallel()
	fs := NewFS()
	_, err := fs.ToOSPath(".") // ensure no error returned, not even a nil typed error
	assert.NoError(t, err)
}

func TestFromOSPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		description      string
		root             string
		volumeName       string
		goos             string
		osPath           string
		osPathVolumeName string
		expectPath       string
		expectErr        string
	}{
		{
			description: "root path unix",
			goos:        goosLinux,
			osPath:      "/",
			expectPath:  ".",
		},
		{
			description:      "root path windows",
			goos:             goosWindows,
			osPath:           `C:\`,
			osPathVolumeName: `C:`,
			expectPath:       ".",
		},
		{
			description: "sub path unix",
			goos:        goosLinux,
			osPath:      "/foo",
			expectPath:  "foo",
		},
		{
			description:      "sub path windows",
			goos:             goosWindows,
			osPath:           `C:\foo`,
			osPathVolumeName: `C:`,
			expectPath:       "foo",
		},
		{
			description: "root sub path unix",
			root:        "foo",
			goos:        goosLinux,
			osPath:      "/foo/bar",
			expectPath:  "bar",
		},
		{
			description: "disjoint root sub path unix",
			root:        "foo",
			goos:        goosLinux,
			osPath:      "/baz/bar",
			expectErr:   "test /baz/bar: invalid argument",
		},
		{
			description:      "root sub path windows",
			root:             "foo",
			goos:             goosWindows,
			osPath:           `C:\foo\bar`,
			osPathVolumeName: `C:`,
			expectPath:       "bar",
		},
		{
			description:      "letter volume path windows",
			volumeName:       `D:`,
			goos:             goosWindows,
			osPath:           `D:\foo`,
			osPathVolumeName: `D:`,
			expectPath:       "foo",
		},
		{
			description:      "letter volume sub path windows",
			root:             "foo",
			volumeName:       `D:`,
			goos:             goosWindows,
			osPath:           `D:\foo\bar`,
			osPathVolumeName: `D:`,
			expectPath:       "bar",
		},
		{
			description:      "UNC volume path windows",
			volumeName:       `\\some-host\share`,
			goos:             goosWindows,
			osPath:           `\\some-host\share\foo`,
			osPathVolumeName: `\\some-host\share`,
			expectPath:       "foo",
		},
		{
			description:      "disjoint UNC volume path windows",
			volumeName:       `\\some-host\share`,
			goos:             goosWindows,
			osPath:           `\\some-other-host\share\foo`,
			osPathVolumeName: `\\some-other-host\share`,
			expectErr:        `test \\some-other-host\share\foo: invalid argument`,
		},
		{
			description:      "UNC volume sub path windows",
			root:             "foo",
			volumeName:       `\\some-host\share`,
			goos:             goosWindows,
			osPath:           `\\some-host\share\foo\bar`,
			osPathVolumeName: `\\some-host\share`,
			expectPath:       "bar",
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
			getVolumeName := func(_ string) string { return tc.osPathVolumeName }
			path, err := fs.fromOSPath(tc.goos, sep, getVolumeName, "test", tc.osPath)
			if tc.expectErr != "" {
				if assert.Error(t, err) {
					assert.Equal(t, tc.expectErr, err.Error())
				}
				return
			}
			assert.Equal(t, tc.expectPath, path)
			assert.NoError(t, err)
		})
	}
}
