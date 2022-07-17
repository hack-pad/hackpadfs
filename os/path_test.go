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
