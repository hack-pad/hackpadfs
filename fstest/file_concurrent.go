package fstest

import (
	"fmt"
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestConcurrentFileRead(tb testing.TB, options FSOptions) {
	tbRun(tb, "same file path", func(tb testing.TB) {
		setupFS, commit := options.Setup.FS(tb)
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
		setupFS, commit := options.Setup.FS(tb)
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

func TestConcurrentFileWrite(tb testing.TB, options FSOptions) {
	tbRun(tb, "same file path", func(tb testing.TB) {
		setupFS, commit := options.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}
		fs, ok := commit().(hackpadfs.OpenFileFS)
		if !ok {
			tb.Skip("FS is not a OpenFileFS")
		}
		concurrentTasks(0, func(i int) {
			f, err := fs.OpenFile("foo", hackpadfs.FlagWriteOnly, 0)
			if f, ok := f.(hackpadfs.ReadWriterFile); assert.NoError(tb, err) && ok {
				n, err := f.Write([]byte("hello"))
				assert.Equal(tb, 5, n)
				assert.NoError(tb, err)
				assert.NoError(tb, f.Close())
			}
		})
	})

	tbRun(tb, "different file paths", func(tb testing.TB) {
		setupFS, commit := options.Setup.FS(tb)
		const fileCount = 10
		for i := 0; i < fileCount; i++ {
			f, err := hackpadfs.Create(setupFS, fmt.Sprintf("foo-%d", i))
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		}
		fs, ok := commit().(hackpadfs.OpenFileFS)
		if !ok {
			tb.Skip("FS is not a OpenFileFS")
		}
		concurrentTasks(fileCount, func(i int) {
			f, err := fs.OpenFile(fmt.Sprintf("foo-%d", i), hackpadfs.FlagWriteOnly, 0)
			if f, ok := f.(hackpadfs.ReadWriterFile); assert.NoError(tb, err) && ok {
				n, err := f.Write([]byte("hello"))
				assert.Equal(tb, 5, n)
				assert.NoError(tb, err)
				assert.NoError(tb, f.Close())
			}
		})
	})
}

func TestConcurrentFileStat(tb testing.TB, options FSOptions) {
	setupFS, commit := options.Setup.FS(tb)
	f, err := hackpadfs.Create(setupFS, "foo")
	if assert.NoError(tb, err) {
		assert.NoError(tb, f.Close())
	}
	fs := commit()
	concurrentTasks(0, func(i int) {
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
