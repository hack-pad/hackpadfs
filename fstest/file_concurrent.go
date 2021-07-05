package fstest

import (
	"fmt"
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestConcurrentFileRead(tb testing.TB, setup SetupFSFunc) {
	tbRun(tb, "same file path", func(tb testing.TB) {
		setupFS, commit := setup(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			_, err := hackpadfs.WriteFile(f, []byte("hello world"))
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		}
		fs := commit()
		concurrentTasks(0, func(i int) {
			f, err := fs.Open("foo")
			if assert.NoError(tb, err) {
				buf := make([]byte, 5)
				n, err := f.Read(buf)
				assert.Equal(tb, 5, n)
				assert.NoError(tb, err)
				assert.Equal(tb, []byte("hello"), buf)
				assert.NoError(tb, f.Close())
			}
		})
	})

	tbRun(tb, "different file paths", func(tb testing.TB) {
		setupFS, commit := setup(tb)
		const fileCount = 10
		for i := 0; i < fileCount; i++ {
			f, err := hackpadfs.Create(setupFS, fmt.Sprintf("foo-%d", i))
			if assert.NoError(tb, err) {
				_, err := hackpadfs.WriteFile(f, []byte("hello world"))
				assert.NoError(tb, err)
				assert.NoError(tb, f.Close())
			}
		}
		fs := commit()
		concurrentTasks(fileCount, func(i int) {
			f, err := fs.Open(fmt.Sprintf("foo-%d", i))
			if assert.NoError(tb, err) {
				buf := make([]byte, 5)
				n, err := f.Read(buf)
				assert.Equal(tb, 5, n)
				assert.NoError(tb, err)
				assert.Equal(tb, []byte("hello"), buf)
				assert.NoError(tb, f.Close())
			}
		})
	})
}
