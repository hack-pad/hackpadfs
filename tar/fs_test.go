package tar

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"runtime"
	"testing"
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/internal/assert"
	"github.com/hack-pad/hackpadfs/internal/fserrors"
	"github.com/hack-pad/hackpadfs/mem"
)

func TestFS(t *testing.T) {
	t.Parallel()
	options := fstest.FSOptions{
		Name: "tar",
		Setup: fstest.TestSetupFunc(func(tb testing.TB) (fstest.SetupFS, func() hackpadfs.FS) {
			setupFS, err := mem.NewFS()
			if !assert.NoError(tb, err) {
				tb.FailNow()
			}

			return setupFS, func() hackpadfs.FS {
				return newTarFromFS(tb, setupFS)
			}
		}),
	}
	fstest.FS(t, options)
	fstest.File(t, options)
}

func newTarFromFS(tb testing.TB, src hackpadfs.FS) *ReaderFS {
	r, err := buildTarFromFS(tb, src)
	if !assert.NoError(tb, err) {
		tb.FailNow()
	}

	fs, err := NewReaderFS(context.Background(), r, ReaderFSOptions{})
	if !assert.NoError(tb, err) {
		tb.FailNow()
	}
	return fs
}

func buildTarFromFS(tb testing.TB, src hackpadfs.FS) (io.Reader, error) {
	var buf bytes.Buffer
	archive := tar.NewWriter(&buf)
	defer func() { assert.NoError(tb, archive.Close()) }()

	err := hackpadfs.WalkDir(src, ".", copyTarWalk(src, archive))
	return &buf, fserrors.WithMessage(err, "Failed building tar from FS walk")
}

func copyTarWalk(src hackpadfs.FS, archive *tar.Writer) hackpadfs.WalkDirFunc {
	return func(path string, dir hackpadfs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := dir.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = path
		if info.IsDir() {
			header.Name += "/"
		}
		err = archive.WriteHeader(header)
		if err != nil {
			return err
		}
		fileBytes, err := hackpadfs.ReadFile(src, path)
		if err != nil {
			return err
		}
		_, err = archive.Write(fileBytes)
		return err
	}
}

// TestNewTarFromFS is a sanity check on the constructor we use in fstest. Just make sure it behaves normally for simple cases.
func TestNewTarFromFS(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		description string
		do          func(t *testing.T, fs hackpadfs.FS)
	}{
		{
			description: "empty",
			do:          func(_ *testing.T, _ hackpadfs.FS) {},
		},
		{
			description: "one file",
			do: func(t *testing.T, fs hackpadfs.FS) {
				_, err := hackpadfs.Create(fs, "foo")
				if !assert.NoError(t, err) {
					t.FailNow()
				}
			},
		},
		{
			description: "one dir",
			do: func(t *testing.T, fs hackpadfs.FS) {
				err := hackpadfs.Mkdir(fs, "foo", 0700)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
			},
		},
		{
			description: "dir with one nested file",
			do: func(t *testing.T, fs hackpadfs.FS) {
				err := hackpadfs.Mkdir(fs, "foo", 0700)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				_, err = hackpadfs.Create(fs, "foo/bar")
				if !assert.NoError(t, err) {
					t.FailNow()
				}
			},
		},
	} {
		tc := tc // enable parallel sub-tests
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			memFS, err := mem.NewFS()
			if !assert.NoError(t, err) {
				t.FailNow()
			}
			tc.do(t, memFS)
			timer := time.NewTimer(50 * time.Millisecond)
			done := make(chan struct{})

			go func() {
				tarFS := newTarFromFS(t, memFS)
				assert.NoError(t, err)
				assert.NotEqual(t, nil, tarFS)
				close(done)
			}()

			select {
			case <-done:
				timer.Stop()
			case <-timer.C:
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, true)
				t.Fatalf("Took too long:\n%s", string(buf[:n]))
			}
		})
	}
}
