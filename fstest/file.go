package fstest

import (
	// Avoid importing "os" package in fstest if we can, since not all environments may be able to support it.
	// Not to mention it should compile a little faster. :)
	"errors"
	"io"
	"sort"
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestFileClose(tb testing.TB, o FSOptions) {
	setupFS, commit := o.Setup.FS(tb)
	f, err := hackpadfs.Create(setupFS, "foo")
	if assert.NoError(tb, err) {
		assert.NoError(tb, f.Close())
	}

	fs := commit()
	f, err = fs.Open("foo")
	if assert.NoError(tb, err) {
		assert.NoError(tb, f.Close())
		assert.Error(tb, f.Close())
	}
}

func TestFileRead(tb testing.TB, o FSOptions) {
	const fileContents = "hello world"
	o.tbRun(tb, "read empty", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := commit()
		f, err = fs.Open("foo")
		if !assert.NoError(tb, err) {
			tb.FailNow()
		}
		buf := make([]byte, 10)
		n, err := f.Read(buf)
		assert.Equal(tb, 0, n)
		assert.Equal(tb, io.EOF, err)
		assert.NoError(tb, f.Close())
	})

	o.tbRun(tb, "read a few bytes at a time", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		const firstBufLen = 2
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(f, []byte(fileContents))
			assert.NoError(tb, err)
			assert.NoError(tb, f.Close())
		}

		fs := commit()
		f, err = fs.Open("foo")
		if !assert.NoError(tb, err) {
			tb.FailNow()
		}

		buf := make([]byte, firstBufLen)
		n, err := f.Read(buf)
		assert.Equal(tb, firstBufLen, n)
		assert.NoError(tb, err)
		assert.Equal(tb, "he", string(buf))

		buf = make([]byte, len(fileContents)*2)
		n, err = f.Read(buf)
		assert.Equal(tb, len(fileContents)-firstBufLen, n)
		if err == nil {
			// it's ok to return a nil error when finishing a read
			// but the next read must return 0 and EOF
			tmpBuf := make([]byte, len(buf))
			var zeroN int
			zeroN, err = f.Read(tmpBuf)
			assert.Equal(tb, 0, zeroN)
		}
		assert.Equal(tb, io.EOF, err)
		assert.Equal(tb, "llo world", string(buf[:n]))
		assert.NoError(tb, f.Close())
	})
}

func TestFileReadAt(tb testing.TB, o FSOptions) {
	const fileContents = "hello world"
	setupFS, commit := o.Setup.FS(tb)
	assert.NoError(tb, hackpadfs.WriteFullFile(setupFS, "foo", []byte(fileContents), 0o666))
	fs := commit()

	for _, tc := range []struct {
		description string
		bufSize     int
		off         int64
		expectN     int
		expectBuf   string
		expectErr   error
	}{
		{
			description: "at start",
			bufSize:     len(fileContents),
			off:         0,
			expectN:     len(fileContents),
			expectBuf:   "hello world",
		},
		{
			description: "negative offset",
			bufSize:     len(fileContents),
			off:         -1,
			expectErr:   errors.New("negative offset"),
		},
		{
			description: "small byte offset",
			bufSize:     len(fileContents),
			off:         2,
			expectN:     len(fileContents) - 2,
			expectBuf:   "llo world",
			expectErr:   io.EOF,
		},
		{
			description: "small read at offset",
			bufSize:     2,
			off:         2,
			expectN:     2,
			expectBuf:   "ll",
		},
		{
			description: "full read at offset",
			bufSize:     len(fileContents),
			off:         2,
			expectN:     len(fileContents) - 2,
			expectBuf:   "llo world",
			expectErr:   io.EOF,
		},
	} {
		o.tbRun(tb, tc.description, func(tb testing.TB) {
			tbParallel(tb)
			file, err := fs.Open("foo")
			if !assert.NoError(tb, err) {
				return
			}
			defer func() {
				assert.NoError(tb, file.Close())
			}()

			buf := make([]byte, tc.bufSize)
			n, err := hackpadfs.ReadAtFile(file, buf, tc.off)
			skipNotImplemented(tb, err)
			if n == tc.bufSize && err == io.EOF {
				err = nil
			}
			if tc.expectErr != nil {
				if _, ok := err.(*hackpadfs.PathError); ok {
					o.assertEqualPathErr(tb, &hackpadfs.PathError{
						Op:   "readat",
						Path: "foo",
						Err:  tc.expectErr,
					}, err)
				}
				return
			}
			assert.NoError(tb, err)
			assert.Equal(tb, tc.expectN, n)
			assert.Equal(tb, tc.expectBuf, string(buf[:n]))
		})
	}
}

func TestFileSeek(tb testing.TB, o FSOptions) {
	const fileContents = "hello world"

	o.tbRun(tb, "seek unknown start", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		file, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(file, []byte(fileContents))
			assert.NoError(tb, err)
			assert.NoError(tb, file.Close())
		}

		fs := commit()
		file, err = fs.Open("foo")
		assert.NoError(tb, err)
		_, err = hackpadfs.SeekFile(file, 0, -1)
		skipNotImplemented(tb, err)
		o.assertEqualPathErr(tb, &hackpadfs.PathError{
			Op:   "seek",
			Path: "foo",
			Err:  hackpadfs.ErrInvalid,
		}, err)
		assert.NoError(tb, file.Close())
	})

	o.tbRun(tb, "seek negative offset", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		file, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(file, []byte(fileContents))
			assert.NoError(tb, err)
			assert.NoError(tb, file.Close())
		}

		fs := commit()
		file, err = fs.Open("foo")
		assert.NoError(tb, err)
		_, err = hackpadfs.SeekFile(file, -1, io.SeekStart)
		skipNotImplemented(tb, err)
		o.assertEqualPathErr(tb, &hackpadfs.PathError{
			Op:   "seek",
			Path: "foo",
			Err:  hackpadfs.ErrInvalid,
		}, err)
		assert.NoError(tb, file.Close())
	})

	o.tbRun(tb, "seek start", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		file, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(file, []byte(fileContents))
			assert.NoError(tb, err)
			assert.NoError(tb, file.Close())
		}

		fs := commit()
		file, err = fs.Open("foo")
		assert.NoError(tb, err)
		const offset = 1
		off, err := hackpadfs.SeekFile(file, offset, io.SeekStart)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		assert.Equal(tb, int64(offset), off)
		buf := make([]byte, len(fileContents))
		n, err := file.Read(buf)
		assert.Equal(tb, true, err == nil || err == io.EOF)
		assert.Equal(tb, "ello world", string(buf[:n]))
		assert.NoError(tb, file.Close())
	})

	o.tbRun(tb, "seek current", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.WriteFullFile(setupFS, "foo", []byte(fileContents), 0o666))

		fs := commit()
		file, err := fs.Open("foo")
		assert.NoError(tb, err)
		const firstSeekOff = 5
		const offset = -1
		_, err = hackpadfs.SeekFile(file, firstSeekOff, io.SeekStart) // get close to middle
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		off, err := hackpadfs.SeekFile(file, offset, io.SeekCurrent)
		assert.NoError(tb, err)
		assert.Equal(tb, int64(firstSeekOff+offset), off)
		buf := make([]byte, len(fileContents))
		n, err := file.Read(buf)
		assert.Equal(tb, true, err == nil || err == io.EOF)
		assert.Equal(tb, "o world", string(buf[:n]))
		assert.NoError(tb, file.Close())
	})

	o.tbRun(tb, "seek end", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		file, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			_, err = hackpadfs.WriteFile(file, []byte(fileContents))
			assert.NoError(tb, err)
			assert.NoError(tb, file.Close())
		}

		fs := commit()
		file, err = fs.Open("foo")
		assert.NoError(tb, err)
		const offset = -1
		off, err := hackpadfs.SeekFile(file, offset, io.SeekEnd)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		assert.Equal(tb, int64(len(fileContents)+offset), off)
		buf := make([]byte, len(fileContents))
		n, err := file.Read(buf)
		assert.Equal(tb, true, err == nil || err == io.EOF)
		assert.Equal(tb, "d", string(buf[:n]))
		assert.NoError(tb, file.Close())
	})
}

func TestFileWrite(tb testing.TB, o FSOptions) {
	testFileWrite(tb, o, func(file hackpadfs.File, b []byte) (int, error) {
		n, err := hackpadfs.WriteFile(file, b)
		skipNotImplemented(tb, err)
		return n, err
	})
}

func testFileWrite(tb testing.TB, o FSOptions, writer func(hackpadfs.File, []byte) (int, error)) {
	o.tbRun(tb, "write-read", func(tb testing.TB) {
		const fileContents = "hello world"
		setupFS, commit := o.Setup.FS(tb)
		f, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, f.Close())
		}

		fs := commit()
		f, err = hackpadfs.OpenFile(fs, "foo", hackpadfs.FlagReadWrite, 0)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		n, err := writer(f, []byte(fileContents))
		assert.Equal(tb, len(fileContents), n)
		assert.NoError(tb, err)
		assert.NoError(tb, f.Close())
		f, err = fs.Open("foo")
		assert.NoError(tb, err)
		buf := make([]byte, len(fileContents))
		_, _ = f.Read(buf)
		assert.Equal(tb, fileContents, string(buf))
		assert.NoError(tb, f.Close())
	})

	o.tbRun(tb, "write-truncate-write-read", func(tb testing.TB) {
		const fileContents = "hello world"
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, hackpadfs.WriteFullFile(setupFS, "foo", []byte(fileContents), 0o666))

		fs := commit()
		file, err := hackpadfs.OpenFile(fs, "foo", hackpadfs.FlagReadWrite|hackpadfs.FlagTruncate, 0)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		n, err := writer(file, []byte(fileContents))
		assert.Equal(tb, len(fileContents), n)
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())
		file, err = fs.Open("foo")
		assert.NoError(tb, err)
		buf := make([]byte, len(fileContents))
		_, _ = file.Read(buf)
		assert.Equal(tb, fileContents, string(buf))
		assert.NoError(tb, file.Close())
	})
}

func TestFileWriteAt(tb testing.TB, o FSOptions) {
	o.tbRun(tb, "negative offset", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		file, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, file.Close())
		}

		fs := commit()
		file, err = hackpadfs.OpenFile(fs, "foo", hackpadfs.FlagReadWrite, 0)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		n, err := hackpadfs.WriteAtFile(file, []byte("hello"), -1)
		assert.Equal(tb, 0, n)
		assert.Error(tb, err)
		assert.NoError(tb, file.Close())

		o.assertEqualPathErr(tb, &hackpadfs.PathError{
			Op:   "writeat",
			Path: "foo",
			Err:  errors.New("negative offset"),
		}, err)
	})

	o.tbRun(tb, "no offset", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		file, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, file.Close())
		}

		fs := commit()
		file, err = hackpadfs.OpenFile(fs, "foo", hackpadfs.FlagReadWrite, 0)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		const fileContents = "hello world"
		n, err := hackpadfs.WriteAtFile(file, []byte(fileContents), 0)
		assert.Equal(tb, len(fileContents), n)
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())

		file, err = fs.Open("foo")
		assert.NoError(tb, err)
		buf := make([]byte, len(fileContents))
		_, _ = file.Read(buf)
		assert.Equal(tb, fileContents, string(buf))
		assert.NoError(tb, file.Close())
	})

	o.tbRun(tb, "offset inside file", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		file, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, file.Close())
		}

		fs := commit()
		file, err = hackpadfs.OpenFile(fs, "foo", hackpadfs.FlagReadWrite, 0)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		const fileContents = "hello world"
		const newContents = "hi"
		const offset = 5
		_, err = hackpadfs.WriteAtFile(file, []byte(fileContents), 0)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		n, err := hackpadfs.WriteAtFile(file, []byte(newContents), offset)
		assert.Equal(tb, len(newContents), n)
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())

		file, err = fs.Open("foo")
		assert.NoError(tb, err)
		buf := make([]byte, len(fileContents))
		_, _ = file.Read(buf)
		assert.Equal(tb, "hellohiorld", string(buf))
		assert.NoError(tb, file.Close())
	})

	o.tbRun(tb, "offset outside file", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		file, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, file.Close())
		}

		fs := commit()
		file, err = hackpadfs.OpenFile(fs, "foo", hackpadfs.FlagReadWrite, 0)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		const fileContents = "hello world"
		const offset = 5
		n, err := hackpadfs.WriteAtFile(file, []byte(fileContents), offset)
		assert.Equal(tb, len(fileContents), n)
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())

		file, err = fs.Open("foo")
		assert.NoError(tb, err)
		buf := make([]byte, offset+len(fileContents))
		_, _ = file.Read(buf)
		assert.Equal(tb, append(make([]byte, offset), []byte(fileContents)...), buf)
		assert.NoError(tb, file.Close())
	})
}

func TestFileReadDir(tb testing.TB, o FSOptions) {
	o.tbRun(tb, "list initial root", func(tb testing.TB) {
		_, commit := o.Setup.FS(tb)
		fs := commit()
		file, err := fs.Open(".")
		assert.NoError(tb, err)
		_, err = hackpadfs.ReadDirFile(file, 0)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())
	})

	o.tbRun(tb, "list root", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		file, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, file.Close())
		}
		assert.NoError(tb, setupFS.Mkdir("bar", 0o700))

		fs := commit()
		file, err = fs.Open(".")
		assert.NoError(tb, err)
		entries, err := hackpadfs.ReadDirFile(file, 0)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())
		sort.SliceStable(entries, func(a, b int) bool {
			return entries[a].Name() < entries[b].Name()
		})
		o.assertSubsetQuickInfos(tb, []quickInfo{
			{Name: "bar", Mode: hackpadfs.ModeDir | 0o700, IsDir: true},
			{Name: "foo", Mode: 0o666},
		}, asQuickDirInfos(tb, entries))
	})

	o.tbRun(tb, "readdir batches", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		file, err := hackpadfs.Create(setupFS, "foo")
		if assert.NoError(tb, err) {
			assert.NoError(tb, file.Close())
		}
		assert.NoError(tb, setupFS.Mkdir("bar", 0o700))

		fs := commit()
		file, err = fs.Open(".")
		assert.NoError(tb, err)
		entries1, err := hackpadfs.ReadDirFile(file, 1)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		entries2, err := hackpadfs.ReadDirFile(file, 1)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())

		file, err = fs.Open(".")
		assert.NoError(tb, err)
		entriesAll, err := hackpadfs.ReadDirFile(file, 0)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())

		var entries []hackpadfs.DirEntry
		entries = append(entries, entries1...)
		entries = append(entries, entries2...)
		assert.Equal(tb, 2, len(entries))
		o.assertSubsetQuickInfos(tb, asQuickDirInfos(tb, entries), asQuickDirInfos(tb, entriesAll))
		o.assertSubsetQuickInfos(tb, []quickInfo{
			{Name: "bar", Mode: hackpadfs.ModeDir | 0o700, IsDir: true},
			{Name: "foo", Mode: 0o666},
		}, asQuickDirInfos(tb, entriesAll))
	})

	o.tbRun(tb, "readdir high N", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, setupFS.Mkdir("bar", 0o700))

		fs := commit()
		file, err := fs.Open(".")
		assert.NoError(tb, err)
		entries, err := hackpadfs.ReadDirFile(file, 100000000)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())
		o.assertSubsetQuickInfos(tb,
			[]quickInfo{{Name: "bar", Mode: hackpadfs.ModeDir | 0o700, IsDir: true}},
			asQuickDirInfos(tb, entries),
		)
	})

	o.tbRun(tb, "list empty subdirectory", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, setupFS.Mkdir("foo", 0o700))

		fs := commit()
		file, err := fs.Open("foo")
		assert.NoError(tb, err)
		entries, err := hackpadfs.ReadDirFile(file, 0)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())
		assert.Equal(tb, 0, len(entries))
	})

	o.tbRun(tb, "list subdirectory", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		assert.NoError(tb, setupFS.Mkdir("foo", 0o700))
		file, err := hackpadfs.Create(setupFS, "foo/bar")
		if assert.NoError(tb, err) {
			assert.NoError(tb, file.Close())
		}
		file, err = hackpadfs.Create(setupFS, "foo/baz")
		if assert.NoError(tb, err) {
			assert.NoError(tb, file.Close())
		}
		assert.NoError(tb, setupFS.Mkdir("foo/boo", 0o700))

		fs := commit()
		file, err = fs.Open("foo")
		assert.NoError(tb, err)
		entries, err := hackpadfs.ReadDirFile(file, 0)
		skipNotImplemented(tb, err)
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())
		sort.SliceStable(entries, func(a, b int) bool {
			return entries[a].Name() < entries[b].Name()
		})
		o.assertEqualQuickInfos(tb, []quickInfo{
			{Name: "bar", Mode: 0o666},
			{Name: "baz", Mode: 0o666},
			{Name: "boo", Mode: hackpadfs.ModeDir | 0o700, IsDir: true},
		}, asQuickDirInfos(tb, entries))
	})

	o.tbRun(tb, "list on file", func(tb testing.TB) {
		setupFS, commit := o.Setup.FS(tb)
		{
			f, err := hackpadfs.Create(setupFS, "foo")
			if assert.NoError(tb, err) {
				assert.NoError(tb, f.Close())
			}
		}
		fs := commit()
		file, err := fs.Open("foo")
		assert.NoError(tb, err)
		tb.Cleanup(func() { assert.NoError(tb, file.Close()) })
		entries, err := hackpadfs.ReadDirFile(file, 0)
		skipNotImplemented(tb, err)
		if assert.IsType(tb, &hackpadfs.PathError{}, err) {
			err := err.(*hackpadfs.PathError)
			assert.ErrorIs(tb, hackpadfs.ErrNotDir, err)
			assert.Contains(tb, []string{
				"fdopendir",  // macOS
				"readdirent", // Linux
				"readdir",    // Windows
			}, err.Op)
			o.assertEqualErrPath(tb, "foo", err.Path)
		}
		assert.Equal(tb, 0, len(entries))
	})
}

func TestFileStat(tb testing.TB, o FSOptions) {
	testStat(tb, o, func(tb testing.TB, fs hackpadfs.FS, path string) (hackpadfs.FileInfo, error) {
		tb.Helper()
		f, err := fs.Open(path)
		if err != nil {
			return nil, err
		}
		tb.Cleanup(func() {
			assert.NoError(tb, f.Close())
		})
		return f.Stat()
	})
}

func TestFileSync(tb testing.TB, o FSOptions) {
	setupFS, commit := o.Setup.FS(tb)
	const fileContents = "hello world"
	file, err := hackpadfs.Create(setupFS, "foo")
	if assert.NoError(tb, err) {
		_, err = hackpadfs.WriteFile(file, []byte(fileContents))
		assert.NoError(tb, err)
		assert.NoError(tb, file.Close())
	}

	fs := commit()
	if openFileFS, ok := fs.(hackpadfs.OpenFileFS); ok {
		// some FSs require write access to perform a sync (Windows), so try to add that access
		file, err = openFileFS.OpenFile("foo", hackpadfs.FlagWriteOnly, 0)
	} else {
		file, err = fs.Open("foo")
	}
	assert.NoError(tb, err)
	_, err = hackpadfs.WriteFile(file, []byte("hello"))
	skipNotImplemented(tb, err)
	assert.NoError(tb, err)
	err = hackpadfs.SyncFile(file)
	skipNotImplemented(tb, err)
	assert.NoError(tb, err)
	assert.NoError(tb, file.Close())
}

func TestFileTruncate(tb testing.TB, o FSOptions) {
	const fileContents = "hello world"
	for _, tc := range []struct {
		description   string
		size          int64
		expectErrKind error
	}{
		{
			description:   "negative size",
			size:          -1,
			expectErrKind: hackpadfs.ErrInvalid,
		},
		{
			description: "zero size",
			size:        0,
		},
		{
			description: "small size",
			size:        1,
		},
		{
			description: "too big",
			size:        int64(len(fileContents)) * 2,
		},
	} {
		o.tbRun(tb, tc.description, func(tb testing.TB) {
			setupFS, commit := o.Setup.FS(tb)
			file, err := hackpadfs.Create(setupFS, "foo")
			if assert.NoError(tb, err) {
				_, err = hackpadfs.WriteFile(file, []byte(fileContents))
				assert.NoError(tb, err)
				assert.NoError(tb, file.Close())
			}

			fs := commit()
			file, err = hackpadfs.OpenFile(fs, "foo", hackpadfs.FlagReadWrite, 0)
			skipNotImplemented(tb, err)
			assert.NoError(tb, err)
			err = hackpadfs.TruncateFile(file, tc.size)
			skipNotImplemented(tb, err)
			assert.NoError(tb, file.Close())
			if tc.expectErrKind != nil {
				assert.Error(tb, err)
				o.assertEqualPathErr(tb, &hackpadfs.PathError{
					Op:   "truncate",
					Path: "foo",
					Err:  tc.expectErrKind,
				}, err)
				o.tryAssertEqualFS(tb, map[string]fsEntry{
					"foo": {Mode: 0o666, Size: int64(len(fileContents))},
				}, fs)
			} else {
				assert.NoError(tb, err)
				o.tryAssertEqualFS(tb, map[string]fsEntry{
					"foo": {Mode: 0o666, Size: tc.size},
				}, fs)
			}
		})
	}
}
