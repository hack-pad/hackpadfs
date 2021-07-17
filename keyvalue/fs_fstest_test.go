package keyvalue_test

import (
	"testing"

	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/mem"
)

func TestFS(t *testing.T) {
	t.Parallel()
	options := fstest.FSOptions{
		Name: "keyvalue",
		TestFS: func(tb testing.TB) fstest.SetupFS {
			fs, err := mem.NewFS() // mem.FS uses keyvalue.Store under the hood. Run the tests again here for keyvalue package coverage
			if err != nil {
				tb.Fatal(err)
			}
			return fs
		},
	}
	fstest.FS(t, options)
	fstest.File(t, options)
}
