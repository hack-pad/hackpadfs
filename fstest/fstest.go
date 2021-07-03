// Package fstest runs test suites against a target FS. See fstest.FS() to get started.
package fstest

import (
	"testing"

	"github.com/hack-pad/hackpadfs"
)

// FSOptions contains required and optional settings for running fstest against your FS.
type FSOptions struct {
	// Name of this test run. Required.
	Name string
	// TestFS sets up the current sub-test and returns an FS. Required if SetupFS is not set.
	// Must support running in parallel with other tests. For a global FS like 'osfs', return a sub-FS rooted in a temporary directory for each call to TestFS.
	// Cleanup should be run via tb.Cleanup() tasks.
	TestFS func(tb testing.TB) SetupFS

	// SetupFS returns an FS that can prepare files and a commit function.
	// When commit is called, SetupFS's changes must be copied into a new test FS (like TestFS does) and return it.
	//
	// In many cases, this is not needed and all preparation can be done with only the TestFS() option.
	// However, in more niche file systems like a read-only FS, it is necessary to commit files to a normal FS, then copy them into a read-only store.
	SetupFS SetupFSFunc

	// TODO add a "skip" func, enables checking a test matrix before running
}

type SetupFS interface {
	hackpadfs.FS
	hackpadfs.OpenFileFS
	hackpadfs.ChmodFS
	hackpadfs.ChtimesFS
	hackpadfs.SymlinkFS
}

type SetupFSFunc func(tb testing.TB) (SetupFS, func() hackpadfs.FS)

func FS(tb testing.TB, options FSOptions) {
	tb.Helper()

	if options.Name == "" {
		tb.Error("FS test name is required")
		return
	}
	if options.TestFS == nil && options.SetupFS == nil {
		tb.Error("TestFS func is required")
		return
	}
	if options.SetupFS == nil {
		// Default will call TestFS() once for setup, then return the same one for the test itself.
		options.SetupFS = func(tb testing.TB) (SetupFS, func() hackpadfs.FS) {
			fs := options.TestFS(tb)
			return fs, func() hackpadfs.FS { return fs }
		}
	}

	tbRun(tb, options.Name, func(tb testing.TB) {
		tbParallel(tb)
		tb.Helper()
		runFS(tb, options)
	})
}

func tbParallel(tb testing.TB) {
	if par, ok := tb.(interface{ Parallel() }); ok {
		par.Parallel()
	}
}

func tbRun(tb testing.TB, name string, subtest func(tb testing.TB)) {
	switch tb := tb.(type) {
	case *testing.T:
		tb.Run(name, func(t *testing.T) {
			t.Helper()
			subtest(t)
		})
	case *testing.B:
		tb.Run(name, func(b *testing.B) {
			b.Helper()
			subtest(b)
		})
	default:
		tb.Errorf("Unrecognized testing type: %T", tb)
	}
}

type subtaskFunc func(tb testing.TB, setup SetupFSFunc)

type tbSubtask struct {
	Name string
	Task subtaskFunc
}

func newSubtask(name string, task subtaskFunc) *tbSubtask {
	return &tbSubtask{Name: name, Task: task}
}

func (task *tbSubtask) Run(tb testing.TB, options FSOptions) {
	tbRun(tb, task.Name, func(tb testing.TB) {
		tbParallel(tb)
		tb.Helper()
		task.Task(tb, options.SetupFS)
	})
}

func runFS(tb testing.TB, options FSOptions) {
	newSubtask("base fs.Create", TestBaseCreate).Run(tb, options)
	newSubtask("base fs.Mkdir", TestBaseMkdir).Run(tb, options)
	newSubtask("base fs.Chmod", TestBaseChmod).Run(tb, options)
	newSubtask("base fs.Chtimes", TestBaseChtimes).Run(tb, options)
	newSubtask("base file.Close", TestFileClose).Run(tb, options)

	newSubtask("fs.Create", TestCreate).Run(tb, options)
	newSubtask("fs.Mkdir", TestMkdir).Run(tb, options)
	newSubtask("fs.MkdirAll", TestMkdirAll).Run(tb, options)
	newSubtask("fs.Open", TestOpen).Run(tb, options)
	newSubtask("fs.OpenFile", TestOpenFile).Run(tb, options)
	newSubtask("fs.Remove", TestRemove).Run(tb, options)
	newSubtask("fs.RemoveAll", TestRemoveAll).Run(tb, options)
	newSubtask("fs.Rename", TestRename).Run(tb, options)
	newSubtask("fs.Stat", TestStat).Run(tb, options)
	newSubtask("fs.Chmod", TestChmod).Run(tb, options)
	newSubtask("fs.Chtimes", TestChtimes).Run(tb, options)
	// TODO Symlink

	newSubtask("file.Read", TestFileRead).Run(tb, options)
	newSubtask("file.ReadAt", TestFileReadAt).Run(tb, options)
	newSubtask("file.Seek", TestFileSeek).Run(tb, options)
	newSubtask("file.Write", TestFileWrite).Run(tb, options)
	newSubtask("file.WriteAt", TestFileWriteAt).Run(tb, options)
	newSubtask("file.ReadDir", TestFileReadDir).Run(tb, options)
	newSubtask("file.Stat", TestFileStat).Run(tb, options)
	newSubtask("file.Sync", TestFileSync).Run(tb, options)
	newSubtask("file.Truncate", TestFileTruncate).Run(tb, options)
}
