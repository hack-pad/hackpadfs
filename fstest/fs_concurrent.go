package fstest

import (
	"fmt"
	"sync"
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

func concurrentTasks(count int, task func(int)) {
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func(i int) {
			defer wg.Done()
			task(i)
		}(i)
	}
	wg.Wait()
}

func TestConcurrentCreate(tb testing.TB, setup SetupFSFunc) {
	createFS := func(tb testing.TB) hackpadfs.CreateFS {
		_, commit := setup(tb)
		if fs, ok := commit().(hackpadfs.CreateFS); ok {
			return fs
		}
		tb.Skip("FS is not a CreateFS")
		return nil
	}

	tbRun(tb, "same file path", func(tb testing.TB) {
		fs := createFS(tb)
		concurrentTasks(10, func(i int) {
			f, err := fs.Create("foo")
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		})
	})

	tbRun(tb, "different file paths", func(tb testing.TB) {
		fs := createFS(tb)
		concurrentTasks(10, func(i int) {
			f, err := fs.Create(fmt.Sprintf("foo-%d", i))
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		})
	})
}

func TestConcurrentOpenFileCreate(tb testing.TB, setup SetupFSFunc) {
	openFileFS := func(tb testing.TB) hackpadfs.OpenFileFS {
		_, commit := setup(tb)
		if fs, ok := commit().(hackpadfs.OpenFileFS); ok {
			return fs
		}
		tb.Skip("FS is not a OpenFileFS")
		return nil
	}

	tbRun(tb, "same file path", func(tb testing.TB) {
		fs := openFileFS(tb)
		concurrentTasks(100, func(i int) {
			f, err := fs.OpenFile("foo", hackpadfs.FlagReadWrite|hackpadfs.FlagCreate|hackpadfs.FlagTruncate, 0666)
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		})
	})

	tbRun(tb, "different file paths", func(tb testing.TB) {
		fs := openFileFS(tb)
		concurrentTasks(100, func(i int) {
			f, err := fs.OpenFile(fmt.Sprintf("foo-%d", i), hackpadfs.FlagReadWrite|hackpadfs.FlagCreate|hackpadfs.FlagTruncate, 0666)
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		})
	})
}
