package mount

import (
	"path"
	"strings"
	"sync"

	"github.com/hack-pad/hackpadfs"
)

var (
	_ hackpadfs.MountFS = &FS{}
)

// FS is mesh of several file systems mounted at different paths.
// Mount a file system with AddMount().
//
// For ease of use, call the standard operations via hackpadfs.OpenFile(fs, ...), hackpadfs.Mkdir(fs, ...), etc.
type FS struct {
	rootFS  hackpadfs.FS
	mountMu sync.Mutex
	mounts  sync.Map // map[string]hackpadfs.FS
}

// NewFS returns a new FS.
func NewFS(rootFS hackpadfs.FS) (*FS, error) {
	return &FS{
		rootFS: rootFS,
	}, nil
}

// AddMount mounts 'mount' at 'path'. The mount point must already exist as a directory.
func (fs *FS) AddMount(path string, mount hackpadfs.FS) error {
	err := fs.setMount(path, mount)
	if err != nil {
		return &hackpadfs.PathError{Op: "mount", Path: path, Err: err}
	}
	return nil
}

func (fs *FS) setMount(p string, mountFS hackpadfs.FS) error {
	if !hackpadfs.ValidPath(p) || p == "." {
		return hackpadfs.ErrInvalid
	}
	fs.mountMu.Lock()
	defer fs.mountMu.Unlock()
	_, loaded := fs.mounts.LoadOrStore(p, mountFS)
	if loaded {
		// cannot mount at same point as existing mount
		return hackpadfs.ErrExist
	}
	parentFS, subPath := fs.Mount(path.Dir(p)) // get this mount point's parent mount, verify dir exists
	f, err := parentFS.Open(subPath)
	if err != nil {
		fs.mounts.Delete(p)
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		fs.mounts.Delete(p)
		return err
	}
	if !info.IsDir() {
		fs.mounts.Delete(p)
		return hackpadfs.ErrNotDir
	}
	return nil
}

// Mount implements hackpadfs.MountFS
func (fs *FS) Mount(path string) (mount hackpadfs.FS, subPath string) {
	mount, mountPath := fs.mountPoint(path)
	if mountPath == "." {
		return mount, path
	}
	subPath = strings.TrimPrefix(path, mountPath+"/")
	return mount, subPath
}

func (fs *FS) mountPoint(path string) (hackpadfs.FS, string) {
	var resultPath string
	resultFS := fs.rootFS
	fs.mounts.Range(func(key, value interface{}) bool {
		mountPath, mountFS := key.(string), value.(hackpadfs.FS)
		switch {
		case strings.HasPrefix(path, mountPath+"/"):
			if len(mountPath) > len(resultPath) {
				resultFS = mountFS
			}
			return true
		case mountPath == path:
			// exact match
			return false
		default:
			return true
		}
	})
	if resultPath == "" {
		return resultFS, "."
	}
	return resultFS, resultPath
}

func (fs *FS) Open(name string) (hackpadfs.File, error) {
	mountFS, subPath := fs.Mount(name)
	return mountFS.Open(subPath)
}
