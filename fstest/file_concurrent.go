package fstest

import (
	"fmt"
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestConcurrentFileRead(tb testing.TB, o FSOptions) {
	o.tbRun(tb, "same file path", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.WriteFullFile(setupFS, "foo", []byte("hello world"), 0666))
		fs := commit()
		concurrentTasks(0, func(_ int) {
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

	o.tbRun(tb, "different file paths", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		const fileCount = 10
		for i := 0; i < fileCount; i++ {
			assert.NoError(tb, hackpadfs.WriteFullFile(setupFS, fmt.Sprintf("foo-%d", i), []byte("hello world"), 0666))
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

func TestConcurrentFileWrite(tb testing.TB, o FSOptions) {
	o.tbRun(tb, "same file path", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}
		fs := commit()
		concurrentTasks(0, func(_ int) {
			f, err := hackpadfs.OpenFile(fs, "foo", hackpadfs.FlagWriteOnly, 0)
			skipNotImplemented(tb, err)
			n, err := hackpadfs.WriteFile(f, []byte("hello"))
			skipNotImplemented(tb, err)
			assert.Equal(tb, 5, n)
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		})
	})

	o.tbRun(tb, "different file paths", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		const fileCount = 10
		for i := 0; i < fileCount; i++ {
			f, err := hackpadfs.Create(setupFS, fmt.Sprintf("foo-%d", i))
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		}
		fs := commit()
		concurrentTasks(fileCount, func(i int) {
			f, err := hackpadfs.OpenFile(fs, fmt.Sprintf("foo-%d", i), hackpadfs.FlagWriteOnly, 0)
			skipNotImplemented(tb, err)
			n, err := hackpadfs.WriteFile(f, []byte("hello"))
			skipNotImplemented(tb, err)
			assert.Equal(tb, 5, n)
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		})
	})
}

func TestConcurrentFileStat(tb testing.TB, o FSOptions) {
	setupFS, commit := o.Setup.FS(tb)
	f, err := hackpadfs.Create(setupFS, "foo")
	if assert.NoError(tb, err) {
		assert.NoError(tb, f.Close())
	}
	fs := commit()
	concurrentTasks(0, func(_ int) {
		f, err := fs.Open("foo")
		if assert.NoError(tb, err) {
			info, err := f.Stat()
			assert.NoError(tb, err)
			assert.Equal(tb, quickInfo{
				Name: "foo",
				Mode: 0666,
			}, asQuickInfo(info))
			assert.NoError(tb, f.Close())
		}
	})
}
