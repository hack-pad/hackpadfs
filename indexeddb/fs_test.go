//go:build wasm
// +build wasm

package indexeddb

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"github.com/hack-pad/go-indexeddb/idb"
	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

const (
	testDBPrefix = "hackpadfs-test-"
)

func makeFS(tb testing.TB) *FS {
	n, err := rand.Int(rand.Reader, big.NewInt(1000))
	assert.NoError(tb, err)
	name := fmt.Sprintf("%s%s/%d", testDBPrefix, tb.Name(), n.Int64())

	factory := idb.Global()

	fs, err := NewFS(context.Background(), name, Options{})
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		logFS(tb, fs)
		req, err := factory.DeleteDatabase(name)
		assert.NoError(tb, err)
		assert.NoError(tb, req.Await(context.Background()))
	})
	return fs
}

func TestFS(t *testing.T) {
	t.Parallel()
	options := fstest.FSOptions{
		Name: "indexeddb",
		TestFS: func(tb testing.TB) fstest.SetupFS {
			return makeFS(tb)
		},
	}
	fstest.FS(t, options)
	fstest.File(t, options)
}

func logFS(tb testing.TB, fs hackpadfs.FS) {
	if !tb.Failed() {
		return
	}
	tb.Log("FS contents:")
	assert.NoError(tb, hackpadfs.WalkDir(fs, ".", func(path string, d hackpadfs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			tb.Log(path+":", "Unexpected error getting file info:", err)
			return nil
		}
		if info.Mode().IsDir() {
			tb.Log(path+":", info.Mode())
		} else {
			tb.Log(path+":", info.Mode(), info.Size())
		}
		return nil
	}))

}

func TestClear(t *testing.T) {
	t.Parallel()

	fs := makeFS(t)

	f, err := hackpadfs.Create(fs, "foo")
	if assert.NoError(t, err) {
		assert.NoError(t, f.Close())
	}

	assert.NoError(t, fs.Clear(context.Background()))

	f, err = fs.Open(".")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if assert.NoError(t, err) {
		assert.Equal(t, true, info.IsDir())
	}
	dirEntries, err := hackpadfs.ReadDirFile(f, -1)
	if assert.NoError(t, err) {
		assert.Equal(t, 0, len(dirEntries))
	}
}
