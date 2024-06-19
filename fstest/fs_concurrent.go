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

	task(0) // run once outside goroutine to allow "not implemented" Skips

	var wg sync.WaitGroup
	wg.Add(count - 1)
	for i := 1; i < count; i++ {
		go func(i int) {
			defer wg.Done()
			task(i)
		}(i)
	}
	wg.Wait()
}

func TestConcurrentCreate(tb testing.TB, o FSOptions) {
	o.tbRun(tb, "same file path", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		concurrentTasks(0, func(_ int) {
			f, err := hackpadfs.Create(fs, "foo")
			skipNotImplemented(tb, err)
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		})
	})

	o.tbRun(tb, "different file paths", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		concurrentTasks(0, func(i int) {
			f, err := hackpadfs.Create(fs, fmt.Sprintf("foo-%d", i))
			skipNotImplemented(tb, err)
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		})
	})
}

func TestConcurrentOpenFileCreate(tb testing.TB, o FSOptions) {
	o.tbRun(tb, "same file path", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		concurrentTasks(0, func(_ int) {
			f, err := hackpadfs.OpenFile(fs, "foo", hackpadfs.FlagReadWrite|hackpadfs.FlagCreate|hackpadfs.FlagTruncate, 0666)
			skipNotImplemented(tb, err)
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		})
	})

	o.tbRun(tb, "different file paths", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		concurrentTasks(0, func(i int) {
			f, err := hackpadfs.OpenFile(fs, fmt.Sprintf("foo-%d", i), hackpadfs.FlagReadWrite|hackpadfs.FlagCreate|hackpadfs.FlagTruncate, 0666)
			skipNotImplemented(tb, err)
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		})
	})
}

func TestConcurrentRemove(tb testing.TB, o FSOptions) {
	o.tbRun(tb, "same file path", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}
		fs := commit()
		concurrentTasks(0, func(_ int) {
			err := hackpadfs.Remove(fs, "foo")
			skipNotImplemented(tb, err)
			assert.Equal(tb, true, err == nil || errors.Is(err, hackpadfs.ErrNotExist))
		})
	})

	o.tbRun(tb, "different file paths", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		const fileCount = defaultConcurrentTasks
		for i := 0; i < fileCount; i++ {
			f, err := hackpadfs.Create(setupFS, fmt.Sprintf("foo-%d", i))
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		}
		fs := commit()
		concurrentTasks(fileCount, func(i int) {
			err := hackpadfs.Remove(fs, fmt.Sprintf("foo-%d", i))
			skipNotImplemented(tb, err)
			assert.NoError(tb, err)
		})
	})
}

func TestConcurrentMkdir(tb testing.TB, o FSOptions) {
	o.tbRun(tb, "same file path", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		concurrentTasks(0, func(_ int) {
			err := hackpadfs.Mkdir(fs, "foo", 0777)
			skipNotImplemented(tb, err)
			assert.Equal(tb, true, err == nil || errors.Is(err, hackpadfs.ErrExist))
		})
	})

	o.tbRun(tb, "different file paths", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		concurrentTasks(0, func(i int) {
			err := hackpadfs.Mkdir(fs, fmt.Sprintf("foo-%d", i), 0777)
			skipNotImplemented(tb, err)
			assert.NoError(tb, err)
		})
	})
}

func TestConcurrentMkdirAll(tb testing.TB, o FSOptions) {
	o.tbRun(tb, "same file path", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		concurrentTasks(0, func(_ int) {
			err := hackpadfs.MkdirAll(fs, "foo", 0777)
			skipNotImplemented(tb, err)
			assert.NoError(tb, err)
		})
	})

	o.tbRun(tb, "different file paths", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		concurrentTasks(0, func(i int) {
			err := hackpadfs.MkdirAll(fs, fmt.Sprintf("foo-%d", i), 0777)
			skipNotImplemented(tb, err)
			assert.NoError(tb, err)
		})
	})
}
