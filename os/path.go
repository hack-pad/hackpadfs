package os

import (
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hack-pad/hackpadfs"
)

// ToOSPath converts a valid 'io/fs' package path to the equivalent 'os' package path for this FS
func (fs *FS) ToOSPath(fsPath string) (string, error) {
	osPath, err := fs.toOSPath(runtime.GOOS, filepath.Separator, "ospath", fsPath)
	if err != nil { // handle typed err
		return "", err
	}
	return osPath, nil
}

func (fs *FS) rootedPath(op, name string) (string, *hackpadfs.PathError) {
	return fs.toOSPath(runtime.GOOS, filepath.Separator, op, name)
}

func (fs *FS) toOSPath(goos string, separator rune, op, fsPath string) (string, *hackpadfs.PathError) {
	if !hackpadfs.ValidPath(fsPath) {
		return "", &hackpadfs.PathError{Op: op, Path: fsPath, Err: hackpadfs.ErrInvalid}
	}
	fsPath = path.Join("/", fs.root, fsPath)
	filePath := joinSepPath(string(separator), fs.getVolumeName(goos), fromSeparator(separator, fsPath))
	return filePath, nil
}

func joinSepPath(separator, elem1, elem2 string) string {
	elem1 = strings.TrimRight(elem1, separator)
	elem2 = strings.TrimLeft(elem2, separator)
	return elem1 + separator + elem2
}

func fromSeparator(separator rune, path string) string {
	if separator == '/' {
		return path
	}
	return strings.ReplaceAll(path, "/", string(separator))
}

func (fs *FS) getVolumeName(goos string) string {
	if goos == goosWindows && fs.volumeName == "" {
		return `C:`
	}
	return fs.volumeName
}
