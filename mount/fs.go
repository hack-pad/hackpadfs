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
	err := fs.addMount(path, mount)
	if err != nil {
		return &hackpadfs.PathError{Op: "mount", Path: path, Err: err}
	}
	return nil
}

func (fs *FS) addMount(p string, mountFS hackpadfs.FS) error {
	if !hackpadfs.ValidPath(p) || p == "." {
		return hackpadfs.ErrInvalid
	}
	_, loaded := fs.mounts.Load(p)
	if loaded {
		// cannot mount at same point as existing mount
		return hackpadfs.ErrExist
	}
	fs.mountMu.Lock()
	defer fs.mountMu.Unlock()

	dir, base := path.Split(p)
	parentFS, subPath := fs.Mount(dir) // get this mount point's parent mount, verify dir exists
	f, err := parentFS.Open(path.Join(subPath, base))
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return hackpadfs.ErrNotDir
	}
	// TODO Handle data race when directory is removed or becomes a file between the Stat and the mount.

	_, loaded = fs.mounts.LoadOrStore(p, mountFS)
	if loaded {
		// cannot mount at same point as existing mount
		return hackpadfs.ErrExist
	}
	return nil
}

// Mount implements hackpadfs.MountFS
func (fs *FS) Mount(path string) (mount hackpadfs.FS, subPath string) {
	mount, mountPath := fs.mountPoint(path)
	if mountPath == "." {
		return mount, path
	}
	subPath = path
	subPath = strings.TrimPrefix(subPath, mountPath)
	subPath = strings.TrimPrefix(subPath, "/")
	if subPath == "" {
		subPath = "."
	}
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
				resultPath, resultFS = mountPath, mountFS
			}
			return true
		case mountPath == path:
			// exact match
			resultPath, resultFS = mountPath, mountFS
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

type Point struct {
	Path string
}

func (fs *FS) MountPoints() []Point {
	var points []Point
	fs.mounts.Range(func(key, value interface{}) bool {
		path := key.(string)
		points = append(points, Point{path})
		return true
	})
	return points
}
