package keyvalue

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/internal/assert"
	"github.com/hack-pad/hackpadfs/keyvalue/blob"
)

type mapStore struct {
	tb      testing.TB
	records map[string]mapRecord
}

type mapRecord struct {
	store   *mapStore
	path    string
	data    blob.Blob
	mode    hackpadfs.FileMode
	modTime time.Time
}

func (m mapRecord) Data() (blob.Blob, error) { return m.data, nil }
func (m mapRecord) Size() int64              { return int64(m.data.Len()) }
func (m mapRecord) Mode() hackpadfs.FileMode { return m.mode }
func (m mapRecord) ModTime() time.Time       { return m.modTime }
func (m mapRecord) Sys() interface{}         { return nil }

func (m mapRecord) ReadDirNames() ([]string, error) {
	var names []string
	prefix := m.path + "/"
	if m.path == "." {
		prefix = ""
	}
	for p := range m.store.records {
		if strings.HasPrefix(p, prefix) {
			p = strings.TrimPrefix(p, prefix)
			if strings.ContainsRune(p, '/') {
				names = append(names, p)
			}
		}
	}
	return names, nil
}

func (m mapRecord) String() string {
	dataStr := "dir"
	if !m.mode.IsDir() {
		dataStr = fmt.Sprintf("%dB", m.data.Len())
	}
	return fmt.Sprintf("%q: %s %s %s", m.path, m.mode, dataStr, m.modTime.Format(time.RFC3339))
}

func newMapStore(tb testing.TB) Store {
	store := &mapStore{
		tb:      tb,
		records: make(map[string]mapRecord),
	}
	err := store.Set(".", mapRecord{
		store: store,
		path:  ".",
		data:  blob.NewBytes(nil),
		mode:  hackpadfs.ModeDir | 0755,
	})
	assert.NoError(tb, err)
	return store
}

func (m *mapStore) Get(path string) (FileRecord, error) {
	m.tb.Log("getting", path)
	record, ok := m.records[path]
	if !ok {
		return nil, hackpadfs.ErrNotExist
	}
	return record, nil
}

func (m *mapStore) Set(path string, src FileRecord) error {
	if src == nil {
		m.tb.Log("deleting", path)
		delete(m.records, path)
	} else {
		data, err := src.Data()
		if err != nil {
			return err
		}
		record := mapRecord{
			store:   m,
			path:    path,
			data:    data,
			mode:    src.Mode(),
			modTime: src.ModTime(),
		}
		m.tb.Log("setting", record)
		m.records[path] = record
	}
	return nil
}

func TestFS(t *testing.T) {
	fstest.FS(t, fstest.FSOptions{
		Name: "keyvalue",
		TestFS: func(tb testing.TB) fstest.SetupFS {
			fs, err := NewFS(newMapStore(tb))
			if err != nil {
				tb.Fatal(err)
			}
			return fs
		},
	})
}
