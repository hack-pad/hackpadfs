package mem

import (
	"testing"

	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestFS(t *testing.T) {
	t.Parallel()
	options := fstest.FSOptions{
		Name: "mem",
		TestFS: func(tb testing.TB) fstest.SetupFS {
			fs, err := NewFS()
			if !assert.NoError(tb, err) {
				tb.FailNow()
			}
			return fs
		},
	}
	fstest.FS(t, options)
	fstest.File(t, options)
}
