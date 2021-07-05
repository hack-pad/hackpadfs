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

func TestConcurrentCreate(tb testing.TB, setup TestSetup) {
	createFS := func(tb testing.TB) hackpadfs.CreateFS {
		_, commit := setup.FS(tb)
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

func TestConcurrentOpenFileCreate(tb testing.TB, setup TestSetup) {
	openFileFS := func(tb testing.TB) hackpadfs.OpenFileFS {
		_, commit := setup.FS(tb)
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

func TestConcurrentRemove(tb testing.TB, setup TestSetup) {
	removeFS := func(tb testing.TB, commit func() hackpadfs.FS) hackpadfs.RemoveFS {
		if fs, ok := commit().(hackpadfs.RemoveFS); ok {
			return fs
		}
		tb.Skip("FS is not a RemoveFS")
		return nil
	}

	tbRun(tb, "same file path", func(tb testing.TB) {
		setupFS, commit := setup.FS(tb)
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
		setupFS, commit := setup.FS(tb)
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

func TestConcurrentMkdir(tb testing.TB, setup TestSetup) {
	mkdirFS := func(tb testing.TB) hackpadfs.MkdirFS {
		_, commit := setup.FS(tb)
		if fs, ok := commit().(hackpadfs.MkdirFS); ok {
			return fs
		}
		tb.Skip("FS is not a MkdirFS")
		return nil
	}

	tbRun(tb, "same file path", func(tb testing.TB) {
		fs := mkdirFS(tb)
		concurrentTasks(0, func(i int) {
			err := fs.Mkdir("foo", 0777)
			assert.Equal(tb, true, err == nil || errors.Is(err, hackpadfs.ErrExist))
		})
	})

	tbRun(tb, "different file paths", func(tb testing.TB) {
		fs := mkdirFS(tb)
		concurrentTasks(0, func(i int) {
			err := fs.Mkdir(fmt.Sprintf("foo-%d", i), 0777)
			assert.NoError(tb, err)
		})
	})
}

func TestConcurrentMkdirAll(tb testing.TB, setup TestSetup) {
	mkdirAllFS := func(tb testing.TB) hackpadfs.MkdirAllFS {
		_, commit := setup.FS(tb)
		if fs, ok := commit().(hackpadfs.MkdirAllFS); ok {
			return fs
		}
		tb.Skip("FS is not a MkdirAllFS")
		return nil
	}

	tbRun(tb, "same file path", func(tb testing.TB) {
		fs := mkdirAllFS(tb)
		concurrentTasks(0, func(i int) {
			err := fs.MkdirAll("foo", 0777)
			assert.NoError(tb, err)
		})
	})

	tbRun(tb, "different file paths", func(tb testing.TB) {
		fs := mkdirAllFS(tb)
		concurrentTasks(0, func(i int) {
			err := fs.MkdirAll(fmt.Sprintf("foo-%d", i), 0777)
			assert.NoError(tb, err)
		})
	})
}
