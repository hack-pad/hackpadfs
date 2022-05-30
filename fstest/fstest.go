// Package fstest runs test suites against a target FS. See fstest.FS() to get started.
package fstest

import (
	"errors"
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

	// Setup returns an FS that can prepare files and a commit function. Required of TestFS is not set.
	// When commit is called, SetupFS's changes must be copied into a new test FS (like TestFS does) and return it.
	//
	// In many cases, this is not needed and all preparation can be done with only the TestFS() option.
	// However, in more niche file systems like a read-only FS, it is necessary to commit files to a normal FS, then copy them into a read-only store.
	Setup TestSetup

	// Contraints limits tests to a reduced set of assertions.
	// For example, setting FileModeMask limits FileMode assertions on a file's Stat() result.
	Constraints Constraints
}

// SetupFS is an FS that supports the baseline interfaces for creating files/directories and changing their metadata.
// This FS is used to initialize a test's environment.
type SetupFS interface {
	hackpadfs.FS
	hackpadfs.OpenFileFS
	hackpadfs.MkdirFS
	hackpadfs.ChmodFS
	hackpadfs.ChtimesFS
}

// TestSetup returns a new SetupFS and a "commit" function.
// SetupFS is used to initialize a test's environment with the necessary files and metadata.
// commit() creates the FS under test from those setup files.
type TestSetup interface {
	FS(tb testing.TB) (setupFS SetupFS, commit func() hackpadfs.FS)
}

// TestSetupFunc is an adapter to use a function as a TestSetup.
type TestSetupFunc func(tb testing.TB) (SetupFS, func() hackpadfs.FS)

// FS implements TestSetup
func (fn TestSetupFunc) FS(tb testing.TB) (SetupFS, func() hackpadfs.FS) {
	return fn(tb)
}

// Contraints limits tests to a reduced set of assertions
type Constraints struct {
	// FileModeMask disables mode checks on the specified bits. Defaults to checking all bits (0).
	FileModeMask hackpadfs.FileMode
	// InvalidSeekWhenceUndefined is true when the behavior of Seek() with an invalid 'whence' is not defined. Windows seems to be the only candidate where no error occurs.
	InvalidSeekWhenceUndefined bool
}

func setupOptions(options *FSOptions) error {
	if options.Name == "" {
		return errors.New("FS test name is required")
	}
	if options.TestFS == nil && options.Setup == nil {
		return errors.New("TestFS func is required")
	}
	if options.Setup == nil {
		// Default will call TestFS() once for setup, then return the same one for the test itself.
		options.Setup = TestSetupFunc(func(tb testing.TB) (SetupFS, func() hackpadfs.FS) {
			fs := options.TestFS(tb)
			return fs, func() hackpadfs.FS { return fs }
		})
	}
	return nil
}

// FS runs file system tests. All FS interfaces from hackpadfs.*FS are tested.
func FS(tb testing.TB, options FSOptions) {
	tb.Helper()

	err := setupOptions(&options)
	if err != nil {
		tb.Fatal(err)
		return
	}
	tbRun(tb, options.Name, func(tb testing.TB) {
		tbParallel(tb)
		tb.Helper()
		runFS(tb, options)
	})
}

// File runs file tests. All File interfaces from hackpadfs.*File are tested.
func File(tb testing.TB, options FSOptions) {
	tb.Helper()

	err := setupOptions(&options)
	if err != nil {
		tb.Fatal(err)
		return
	}
	tbRun(tb, options.Name, func(tb testing.TB) {
		tbParallel(tb)
		tb.Helper()
		runFile(tb, options)
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

type tbSubtaskRunner struct {
	tb      testing.TB
	options FSOptions
}

func newSubtaskRunner(tb testing.TB, options FSOptions) *tbSubtaskRunner {
	return &tbSubtaskRunner{tb, options}
}

type subtaskFunc func(tb testing.TB, options FSOptions)

func (r *tbSubtaskRunner) Run(name string, subtask subtaskFunc) {
	tbRun(r.tb, name, func(tb testing.TB) {
		tbParallel(tb)
		tb.Helper()
		subtask(tb, r.options)
	})
}

func runFS(tb testing.TB, options FSOptions) {
	runner := newSubtaskRunner(tb, options)
	runner.Run("base fs.Create", TestBaseCreate)
	runner.Run("base fs.Mkdir", TestBaseMkdir)
	runner.Run("base fs.Chmod", TestBaseChmod)
	runner.Run("base fs.Chtimes", TestBaseChtimes)

	runner.Run("fs.Create", TestCreate)
	runner.Run("fs.Mkdir", TestMkdir)
	runner.Run("fs.MkdirAll", TestMkdirAll)
	runner.Run("fs.Open", TestOpen)
	runner.Run("fs.OpenFile", TestOpenFile)
	runner.Run("fs.ReadFile", TestReadFile)
	runner.Run("fs.Remove", TestRemove)
	runner.Run("fs.RemoveAll", TestRemoveAll)
	runner.Run("fs.Rename", TestRename)
	runner.Run("fs.Stat", TestStat)
	runner.Run("fs.Chmod", TestChmod)
	runner.Run("fs.Chtimes", TestChtimes)
	// TODO Symlink

	runner.Run("fs_concurrent.Create", TestConcurrentCreate)
	runner.Run("fs_concurrent.OpenFileCreate", TestConcurrentOpenFileCreate)
	runner.Run("fs_concurrent.Mkdir", TestConcurrentMkdir)
	runner.Run("fs_concurrent.MkdirAll", TestConcurrentMkdirAll)
	runner.Run("fs_concurrent.Remove", TestConcurrentRemove)
}

func runFile(tb testing.TB, options FSOptions) {
	runner := newSubtaskRunner(tb, options)
	runner.Run("base file.Close", TestFileClose)

	runner.Run("file.Read", TestFileRead)
	runner.Run("file.ReadAt", TestFileReadAt)
	runner.Run("file.Seek", TestFileSeek)
	runner.Run("file.Write", TestFileWrite)
	runner.Run("file.WriteAt", TestFileWriteAt)
	runner.Run("file.ReadDir", TestFileReadDir)
	runner.Run("file.Stat", TestFileStat)
	runner.Run("file.Sync", TestFileSync)
	runner.Run("file.Truncate", TestFileTruncate)

	runner.Run("file_concurrent.Read", TestConcurrentFileRead)
	runner.Run("file_concurrent.Write", TestConcurrentFileWrite)
	runner.Run("file_concurrent.Stat", TestConcurrentFileStat)
}
