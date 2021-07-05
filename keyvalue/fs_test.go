package keyvalue

import (
	"testing"

	"github.com/hack-pad/hackpadfs/fstest"
)

func TestFS(t *testing.T) {
	t.Parallel()
	options := fstest.FSOptions{
		Name: "keyvalue",
		TestFS: func(tb testing.TB) fstest.SetupFS {
			fs, err := NewFS(newMapStore(tb))
			if err != nil {
				tb.Fatal(err)
			}
			return fs
		},
	}
	fstest.FS(t, options)
	fstest.File(t, options)
}
