package fstest

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

const defaultConcurrentTasks = 10

func concurrentTasks(count int, task func(int)) {
	if count == 0 {
		count = defaultConcurrentTasks
	}
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
		concurrentTasks(0, func(i int) {
			f, err := fs.Create("foo")
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		})
	})

	tbRun(tb, "different file paths", func(tb testing.TB) {
		fs := createFS(tb)
		concurrentTasks(0, func(i int) {
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
		concurrentTasks(0, func(i int) {
			f, err := fs.OpenFile("foo", hackpadfs.FlagReadWrite|hackpadfs.FlagCreate|hackpadfs.FlagTruncate, 0666)
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		})
	})

	tbRun(tb, "different file paths", func(tb testing.TB) {
		fs := openFileFS(tb)
		concurrentTasks(0, func(i int) {
			f, err := fs.OpenFile(fmt.Sprintf("foo-%d", i), hackpadfs.FlagReadWrite|hackpadfs.FlagCreate|hackpadfs.FlagTruncate, 0666)
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		})
	})
}

func TestConcurrentRemove(tb testing.TB, setup SetupFSFunc) {
	removeFS := func(tb testing.TB, commit func() hackpadfs.FS) hackpadfs.RemoveFS {
		if fs, ok := commit().(hackpadfs.RemoveFS); ok {
			return fs
		}
		tb.Skip("FS is not a RemoveFS")
		return nil
	}

	tbRun(tb, "same file path", func(tb testing.TB) {
		setupFS, commit := setup(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}
		fs := removeFS(tb, commit)
		concurrentTasks(0, func(i int) {
			err := fs.Remove("foo")
			assert.Equal(tb, true, err == nil || errors.Is(err, hackpadfs.ErrNotExist))
		})
	})

	tbRun(tb, "different file paths", func(tb testing.TB) {
		setupFS, commit := setup(tb)
		const fileCount = defaultConcurrentTasks
		for i := 0; i < fileCount; i++ {
			f, err := hackpadfs.Create(setupFS, fmt.Sprintf("foo-%d", i))
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		}
		fs := removeFS(tb, commit)
		concurrentTasks(fileCount, func(i int) {
			err := fs.Remove(fmt.Sprintf("foo-%d", i))
			assert.NoError(tb, err)
		})
	})
}
