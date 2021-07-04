package keyvalue

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
	"github.com/hack-pad/hackpadfs/keyvalue/blob"
)

type mapStore struct {
	tb      testing.TB
	mu      sync.Mutex
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
			if !strings.ContainsRune(p, '/') {
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

type mapTransaction struct {
	ctx     context.Context
	abort   context.CancelFunc
	store   *mapStore
	op      OpID
	results []OpResult
}

func (m *mapStore) Transaction() (Transaction, error) {
	ctx, cancel := context.WithCancel(context.Background())
	txn := &mapTransaction{
		ctx:   ctx,
		abort: cancel,
		store: m,
	}
	m.mu.Lock()
	return txn, nil
}

func (txn *mapTransaction) prepOp() (OpID, error) {
	select {
	case <-txn.ctx.Done():
		return 0, txn.ctx.Err()
	default:
	}

	op := txn.op
	txn.op++
	return op, nil
}

func (txn *mapTransaction) Get(path string) OpID {
	return txn.GetHandler(path, func(txn Transaction, result OpResult) error {
		return nil
	})
}

func (txn *mapTransaction) GetHandler(path string, handler OpHandler) OpID {
	op, err := txn.prepOp()
	if err != nil {
		txn.results = append(txn.results, OpResult{Op: op, Err: err})
		return op
	}
	record, err := txn.store.Get(path)
	result := OpResult{Op: op, Record: record, Err: err}
	err = handler(txn, result)
	if result.Err == nil && err != nil {
		result.Err = err
	}
	txn.results = append(txn.results, result)
	return op
}

func (txn *mapTransaction) Set(path string, src FileRecord) OpID {
	return txn.SetHandler(path, src, func(txn Transaction, result OpResult) error {
		return nil
	})
}

func (txn *mapTransaction) SetHandler(path string, src FileRecord, handler OpHandler) OpID {
	op, err := txn.prepOp()
	if err != nil {
		txn.results = append(txn.results, OpResult{Op: op, Err: err})
		return op
	}
	err = txn.store.Set(path, src)
	result := OpResult{Op: op, Err: err}
	err = handler(txn, result)
	if result.Err == nil && err != nil {
		result.Err = err
	}
	txn.results = append(txn.results, result)
	return op
}

func (txn *mapTransaction) Commit(ctx context.Context) ([]OpResult, error) {
	txn.abort()
	txn.store.mu.Unlock()
	return txn.results, nil
}

func (txn *mapTransaction) Abort() error {
	txn.abort()
	txn.store.mu.Unlock()
	return nil
}
