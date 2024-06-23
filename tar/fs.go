// Package tar contains a tar file based file system.
package tar

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/fserrors"
	"github.com/hack-pad/hackpadfs/mem"
)

// ReaderFS is a tar-based file system, reading files and metadata from a tar archive's io.Reader.
//
// A ReaderFS is ready to use immediately after returning from the non-blocking constructor NewReaderFS().
// This means the files are concurrently unpacked and accessible.
// If a file has not yet been unpacked, that file's operation will block until it is unpacked.
// If a directory's dir entries are accessed, that operation will block until the entire archive has been unpacked. (Tar ordering can't be guaranteed.)
type ReaderFS struct {
	unarchiveFS baseFS
	ps          *pubsub
	// callerCtx is passed in through the constructor, controlling when we should stop reading
	callerCtx context.Context
	// callerCancel can cancel the callerCtx early, like when reading completes
	callerCancel context.CancelFunc
	// readerCtx represents the "canceled" state of the reader, Done() returns this done channel
	readerCtx context.Context
	// readerDone is called once reading has completed - either from cancelling callertCtx, encountering an error, or completing successfully
	readerDone   context.CancelFunc
	unarchiveErr atomic.Value
}

var _ hackpadfs.FS = &ReaderFS{}

// ReaderFSOptions provides configuration options for a new ReaderFS.
type ReaderFSOptions struct {
	// UnarchiveFS is the destination FS to unarchive the reader into. Defaults to mem.FS.
	UnarchiveFS baseFS
}

type baseFS interface {
	hackpadfs.OpenFileFS
	hackpadfs.ChmodFS
	hackpadfs.MkdirFS
}

// NewReaderFS returns a new ReaderFS from the given tar archive reader and options.
// Attempts to close the reader once the tar has completely unpacked.
//
// If using a gzipped tar, be sure to pass the tar reader and not the original gzip reader.
func NewReaderFS(ctx context.Context, r io.Reader, options ReaderFSOptions) (_ *ReaderFS, retErr error) {
	defer func() { retErr = fserrors.WithMessage(retErr, "tar") }()

	if options.UnarchiveFS == nil {
		var err error
		options.UnarchiveFS, err = mem.NewFS()
		if err != nil {
			return nil, err
		}
	}

	if dirEntries, err := hackpadfs.ReadDir(options.UnarchiveFS, "."); err != nil || len(dirEntries) != 0 {
		var names []string
		for _, dirEntry := range dirEntries {
			names = append(names, dirEntry.Name())
		}
		return nil, fmt.Errorf("root '.' of options.UnarchiveFS must be an empty directory, got: %T %v %s", options.UnarchiveFS, err, names)
	}

	ctx, cancel := context.WithCancel(ctx)
	readerCtx, readerDone := context.WithCancel(context.Background())
	fs := &ReaderFS{
		unarchiveFS:  options.UnarchiveFS,
		ps:           newPubsub(ctx),
		callerCtx:    ctx,
		callerCancel: cancel,
		readerCtx:    readerCtx,
		readerDone:   readerDone,
	}
	go fs.read(r)
	return fs, nil
}

func (fs *ReaderFS) read(r io.Reader) {
	err := fs.readErr(r)
	if err != nil {
		fs.unarchiveErr.Store(err)
	}
	fs.callerCancel()
	fs.readerDone()

	if closer, ok := r.(io.Closer); ok {
		_ = closer.Close()
	}
}

func (fs *ReaderFS) readErr(r io.Reader) error {
	archive := tar.NewReader(r)
	const (
		mebibyte       = 1 << 20
		kibibyte       = 1 << 10
		maxMemory      = 20 * mebibyte
		bigBufMemory   = 4 * mebibyte
		smallBufMemory = 150 * kibibyte

		// at least a couple big and small buffers, then a large quantity of small ones make up the remainder
		bigBufCount   = 2
		smallBufCount = (maxMemory - bigBufCount*bigBufMemory) / smallBufMemory
	)
	// set up some buffer pools to reduce maximum memory usage. small buffers are for every file's first read, big buffers for secondary reads.
	smallPool := newBufferPool(smallBufMemory, smallBufCount)
	bigPool := newBufferPool(bigBufMemory, bigBufCount)

	mkdirCache := make(map[string]bool) // avoid calling Mkdir more than once on the same path
	cachedMkdirAll := func(path string, perm hackpadfs.FileMode) error {
		if _, ok := mkdirCache[path]; ok {
			return nil
		}
		err := hackpadfs.MkdirAll(fs.unarchiveFS, path, perm)
		if err == nil {
			mkdirCache[path] = true
		}
		return err
	}

	var wg sync.WaitGroup
	errs := make(chan error, 1)
	for {
		select {
		case err := <-errs:
			return err
		default:
		}
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fserrors.WithMessage(err, "next tar file")
		}
		err = fs.readProcessFile(header, archive, &wg, errs, cachedMkdirAll, smallPool, bigPool)
		if err != nil {
			return err
		}
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case err := <-errs:
		return err
	case <-done:
		return nil
	}
}

// resolvePath converts a tar based path to a rooted FS path
func resolvePath(p string) string {
	p = path.Clean(p)
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		p = "."
	}
	return p
}

func (fs *ReaderFS) readProcessFile(
	header *tar.Header, r io.Reader,
	wg *sync.WaitGroup, errs chan error,
	mkdirAll func(string, hackpadfs.FileMode) error,
	smallPool, bigPool *bufferPool,
) error {
	select {
	case <-fs.callerCtx.Done():
		return fs.callerCtx.Err()
	default:
	}

	originalName := header.Name
	p := resolvePath(originalName)
	info := header.FileInfo()

	dir := path.Dir(p)
	err := mkdirAll(dir, 0o700)
	if err != nil {
		return fserrors.WithMessage(err, "prepping base dir")
	}

	if info.IsDir() {
		// assume dir does not exist yet, then chmod if it does exist
		wg.Add(1)
		go func() { // continue prepping dir in the background
			defer wg.Done()
			err := fs.unarchiveFS.Mkdir(p, info.Mode())
			if err != nil {
				if !errors.Is(err, hackpadfs.ErrExist) {
					errs <- fserrors.WithMessage(err, "copying dir")
					return
				}
				err = fs.unarchiveFS.Chmod(p, info.Mode())
				if err != nil {
					errs <- fserrors.WithMessage(err, "copying dir")
					return
				}
			}
		}()
		return nil
	}

	reader := fullReader{r} // fullReader: call f.Write as few times as possible, since large files can be expensive to write in many batches (Hackpad JS Blobs)
	// read once. if we reached EOF, then write it to fs asynchronously
	smallBuf := smallPool.Wait()
	n, err := reader.Read(smallBuf.Data)
	switch err {
	case io.EOF:
		wg.Add(1)
		go func() { // continue prepping small file in the background
			err := fs.writeFile(p, info, smallBuf, n, nil, nil)
			smallBuf.Done()
			if err != nil {
				errs <- err
			}
			wg.Done()
		}()
		return nil
	case nil:
		// prep large file in the foreground to finish reading the file (going to next file in tar invalidates the file reader)
		bigBuf := bigPool.Wait()
		err := fs.writeFile(p, info, smallBuf, n, reader, bigBuf)
		bigBuf.Done()
		smallBuf.Done()
		return err
	default:
		return err
	}
}

func (fs *ReaderFS) writeFile(path string, info hackpadfs.FileInfo, initialBuf *buffer, n int, r io.Reader, copyBuf *buffer) (returnedErr error) {
	f, err := fs.unarchiveFS.OpenFile(path, hackpadfs.FlagWriteOnly|hackpadfs.FlagCreate|hackpadfs.FlagTruncate, info.Mode())
	if err != nil {
		return fserrors.WithMessage(err, "opening destination file")
	}
	defer func() {
		_ = f.Close()
		if returnedErr == nil {
			fs.ps.Emit(path) // only emit for non-dirs, dirs will wait until the total tar read completes to ensure correctness
		}
	}()

	fWriter, ok := f.(io.Writer)
	if !ok {
		return hackpadfs.ErrNotImplemented
	}

	_, err = fWriter.Write(initialBuf.Data[:n])
	if err != nil {
		return fserrors.WithMessage(err, "write: copying file")
	}

	if r == nil {
		// a nil reader signals we already did a read of N bytes and hit EOF,
		// so the above copy is sufficient, return now
		return nil
	}

	_, err = io.CopyBuffer(fWriter, r, copyBuf.Data)
	return fserrors.WithMessage(err, "copybuf: copying file")
}

type fullReader struct {
	io.Reader
}

func (f fullReader) Read(p []byte) (n int, err error) {
	n, err = io.ReadFull(f.Reader, p)
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	return
}

// Open implements hackpadfs.FS
func (fs *ReaderFS) Open(name string) (hackpadfs.File, error) {
	if !hackpadfs.ValidPath(name) {
		return nil, &hackpadfs.PathError{Op: "open", Path: name, Err: hackpadfs.ErrInvalid}
	}
	fs.ps.Wait(name)
	if unarchiveErr := fs.UnarchiveErr(); unarchiveErr != nil {
		return nil, &hackpadfs.PathError{Op: "open", Path: name, Err: unarchiveErr}
	}
	return fs.unarchiveFS.Open(name)
}

// Done returns a channel that's closed when either the tar has been completely unpacked or the unarchive fails.
// The channel may close some time after the initial context is closed.
func (fs *ReaderFS) Done() <-chan struct{} {
	return fs.readerCtx.Done()
}

// UnarchiveErr returns the error, if any, that occurred during unpacking. Behavior is undefined if Done() has not completed.
func (fs *ReaderFS) UnarchiveErr() error {
	err, _ := fs.unarchiveErr.Load().(error)
	return err
}
