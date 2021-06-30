package osfs

import (
	"io"
	"os"
	"syscall"
	"time"

	"github.com/hack-pad/hackpadfs"
)

type file struct {
	fs     *FS
	osFile *os.File
}

func (fs *FS) wrapFile(f *os.File) hackpadfs.File {
	return &file{fs: fs, osFile: f}
}

func (f *file) Chmod(mode hackpadfs.FileMode) error {
	return f.fs.wrapRelPathErr(f.osFile.Chmod(mode))
}

func (f *file) Chown(uid, gid int) error {
	return f.fs.wrapRelPathErr(f.osFile.Chown(uid, gid))
}

func (f *file) Close() error {
	return f.fs.wrapRelPathErr(f.osFile.Close())
}

func (f *file) Fd() uintptr {
	return f.osFile.Fd()
}

func (f *file) Name() string {
	return f.osFile.Name()
}

func (f *file) Read(b []byte) (n int, err error) {
	n, err = f.osFile.Read(b)
	return n, f.fs.wrapRelPathErr(err)
}

func (f *file) ReadAt(b []byte, off int64) (n int, err error) {
	n, err = f.osFile.ReadAt(b, off)
	return n, f.fs.wrapRelPathErr(err)
}

func (f *file) ReadDir(n int) ([]hackpadfs.DirEntry, error) {
	entries, err := f.osFile.ReadDir(n)
	return entries, f.fs.wrapRelPathErr(err)
}

func (f *file) ReadFrom(r io.Reader) (n int64, err error) {
	n, err = f.osFile.ReadFrom(r)
	return n, f.fs.wrapRelPathErr(err)
}

func (f *file) Seek(offset int64, whence int) (ret int64, err error) {
	ret, err = f.osFile.Seek(offset, whence)
	return ret, f.fs.wrapRelPathErr(err)
}

func (f *file) SetDeadline(t time.Time) error {
	return f.fs.wrapRelPathErr(f.osFile.SetDeadline(t))
}

func (f *file) SetReadDeadline(t time.Time) error {
	return f.fs.wrapRelPathErr(f.osFile.SetReadDeadline(t))
}

func (f *file) SetWriteDeadline(t time.Time) error {
	return f.fs.wrapRelPathErr(f.osFile.SetWriteDeadline(t))
}

func (f *file) Stat() (hackpadfs.FileInfo, error) {
	info, err := f.osFile.Stat()
	return info, f.fs.wrapRelPathErr(err)
}

func (f *file) Sync() error {
	return f.fs.wrapRelPathErr(f.osFile.Sync())
}

func (f *file) SyscallConn() (syscall.RawConn, error) {
	conn, err := f.osFile.SyscallConn()
	return conn, f.fs.wrapRelPathErr(err)
}

func (f *file) Truncate(size int64) error {
	return f.fs.wrapRelPathErr(f.osFile.Truncate(size))
}

func (f *file) Write(b []byte) (n int, err error) {
	n, err = f.osFile.Write(b)
	return n, f.fs.wrapRelPathErr(err)
}

func (f *file) WriteAt(b []byte, off int64) (n int, err error) {
	n, err = f.osFile.WriteAt(b, off)
	return n, f.fs.wrapRelPathErr(err)
}

func (f *file) WriteString(s string) (n int, err error) {
	n, err = f.osFile.WriteString(s)
	return n, f.fs.wrapRelPathErr(err)
}
