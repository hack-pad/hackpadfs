package hackpadfs_test

import (
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/mem"
)

func TestSubFS(t *testing.T) {
	t.Parallel()

	// hackpadfs.Sub calls io/fs.Sub under the hood.
	// This wraps the fs in a way that only respects the io/fs interfacse.
	fstest.FS(t, fstest.FSOptions{
		Name: "subfs",
		TestFS: func(t testing.TB) fstest.SetupFS {
			memFS, err := mem.NewFS()
			if err != nil {
				t.Fatal(err)
			}

			err = memFS.Mkdir("subdir", hackpadfs.ModePerm)
			if err != nil {
				t.Fatal(err)
			}

			subFS, err := hackpadfs.Sub(memFS, "subdir")
			if err != nil {
				t.Fatal(err)
			}

			return subFS.(fstest.SetupFS)
		},
	})
}
