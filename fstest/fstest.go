// Package fstest runs test suites against a target FS. See fstest.FS() to get started.
package fstest

import (
	"errors"
	"sync"
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

	// Contraints limits tests to a reduced set of assertions. Avoid setting any of these options.
	// For example, setting FileModeMask limits FileMode assertions on a file's Stat() result.
	//
	// NOTE: This MUST NOT be used lightly. Any custom constraints severely impairs the quality of a standardized file system.
	Constraints Constraints

	// ShouldSkip determines if the current test with features defined by 'facets' should be skipped.
	// ShouldSkip() is intended for handling undefined behavior in existing systems outside one's control.
	//
	// NOTE: This MUST NOT be used lightly. Any custom skips severely impairs the quality of a standardized file system.
	ShouldSkip func(facets Facets) bool

	skippedTests *sync.Map // type: Facets -> struct{}
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

// Constraints limits tests to a reduced set of assertions due to non-standard behavior. Avoid setting any of these.
type Constraints struct {
	// FileModeMask disables mode checks on the specified bits. Defaults to checking all bits (0).
	FileModeMask hackpadfs.FileMode
	// AllowErrPathPrefix enables more flexible FS path checks on error values by allowing an undefined path prefix.
	AllowErrPathPrefix bool
}

// Facets contains details for the current test.
// Used in FSOptions.ShouldSkip() to inspect and skip tests that should not apply to this FS.
type Facets struct {
	// Name is the full name of the current test
	Name string
}

func setupOptions(options *FSOptions) error {
	options.skippedTests = new(sync.Map)
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
	if options.ShouldSkip == nil {
		options.ShouldSkip = func(_ Facets) bool {
			return false
		}
	}
	return nil
}

func (o FSOptions) tbRun(tb testing.TB, name string, subtest func(tb testing.TB)) {
	tb.Helper()
	switch tb := tb.(type) {
	case *testing.T:
		tb.Run(name, func(t *testing.T) {
			t.Helper()
			o.tbRunInner(t, name, subtest)
		})
	case *testing.B:
		tb.Run(name, func(b *testing.B) {
			b.Helper()
			o.tbRunInner(b, name, subtest)
		})
	default:
		tb.Errorf("Unrecognized testing type: %T", tb)
	}
}

func (o FSOptions) tbRunInner(tb testing.TB, _ string, subtest func(tb testing.TB)) {
	tb.Helper()
	facets := Facets{
		Name: tb.Name(),
	}

	defer func() {
		if tb.Skipped() {
			o.skippedTests.Store(facets, struct{}{})
		}
	}()

	if o.ShouldSkip(facets) {
		tb.Skipf("FSOption.ShouldSkip: %#v", facets)
	}
	subtest(tb)
}

// TestData reports metadata from test runs.
type TestData struct {
	// Skips includes details for every skipped test.
	// Useful for verifying compliance with fstest's standard checks. For instance, os.FS checks (almost) none are skipped.
	Skips []Facets
}

func (o FSOptions) generateTestData() TestData {
	var data TestData
	o.skippedTests.Range(func(key, _ interface{}) bool {
		data.Skips = append(data.Skips, key.(Facets))
		return true
	})
	return data
}

// FS runs file system tests. All FS interfaces from hackpadfs.*FS are tested.
func FS(tb testing.TB, options FSOptions) TestData {
	tb.Helper()

	err := setupOptions(&options)
	if err != nil {
		tb.Fatal(err)
		return TestData{}
	}
	options.tbRun(tb, options.Name+"_FS", func(tb testing.TB) {
		tbParallel(tb)
		tb.Helper()
		runFS(tb, options)
	})
	return options.generateTestData()
}

// File runs file tests. All File interfaces from hackpadfs.*File are tested.
func File(tb testing.TB, options FSOptions) TestData {
	tb.Helper()

	err := setupOptions(&options)
	if err != nil {
		tb.Fatal(err)
		return TestData{}
	}
	options.tbRun(tb, options.Name+"_File", func(tb testing.TB) {
		tbParallel(tb)
		tb.Helper()
		runFile(tb, options)
	})
	return options.generateTestData()
}

func tbParallel(tb testing.TB) {
	if par, ok := tb.(interface{ Parallel() }); ok {
		par.Parallel()
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
	r.options.tbRun(r.tb, name, func(tb testing.TB) {
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

	runner.Run("fs.Chmod", TestChmod)
	runner.Run("fs.Chtimes", TestChtimes)
	runner.Run("fs.Create", TestCreate)
	runner.Run("fs.Mkdir", TestMkdir)
	runner.Run("fs.MkdirAll", TestMkdirAll)
	runner.Run("fs.Open", TestOpen)
	runner.Run("fs.OpenFile", TestOpenFile)
	runner.Run("fs.ReadDir", TestReadDir)
	runner.Run("fs.ReadFile", TestReadFile)
	runner.Run("fs.Remove", TestRemove)
	runner.Run("fs.RemoveAll", TestRemoveAll)
	runner.Run("fs.Rename", TestRename)
	runner.Run("fs.Stat", TestStat)
	runner.Run("fs.WriteFile", TestWriteFile)
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

func skipNotImplemented(tb testing.TB, err error) {
	tb.Helper()
	if errors.Is(err, hackpadfs.ErrNotImplemented) {
		tb.Skip(err)
	}
}
