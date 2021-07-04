package keyvalue

import (
	"testing"

	"github.com/hack-pad/hackpadfs/fstest"
)

func TestFS(t *testing.T) {
	fstest.FS(t, fstest.FSOptions{
		Name: "keyvalue",
		TestFS: func(tb testing.TB) fstest.SetupFS {
			fs, err := NewFS(newMapStore(tb))
			if err != nil {
				tb.Fatal(err)
			}
			return fs
		},
	})
}
