// +build wasm

package indexeddb

import (
	"context"
	"path"
	"syscall/js"
	"time"

	"github.com/hack-pad/go-indexeddb/idb"
	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/keyvalue"
	"github.com/hack-pad/hackpadfs/keyvalue/blob"
)

type store struct {
	db *idb.Database
}

func newStore(db *idb.Database) *store {
	return &store{db: db}
}

func (s *store) Get(path string) (record keyvalue.FileRecord, err error) {
	txn, err := s.db.Transaction(idb.TransactionReadOnly, infoStore)
	if err != nil {
		return nil, err
	}
	files, err := txn.ObjectStore(infoStore)
	if err != nil {
		return nil, err
	}
	req, err := files.Get(js.ValueOf(path))
	if err != nil {
		return nil, err
	}
	value, err := req.Await(context.Background())
	if value.IsUndefined() {
		return nil, hackpadfs.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	initialSize := int64(value.Get("Size").Int())
	modTime := time.Unix(0, int64(value.Get("ModTime").Int()))
	mode := s.getMode(value)
	var getData func() (blob.Blob, error)
	var getDirNames func() ([]string, error)
	if mode.IsDir() {
		getDirNames = s.getDirNames(path)
	} else {
		getData = s.getFileData(path)
	}
	return keyvalue.NewBaseFileRecord(initialSize, modTime, mode, nil, getData, getDirNames), nil
}

func (s *store) getFileData(path string) func() (blob.Blob, error) {
	return func() (blob.Blob, error) {
		txn, err := s.db.Transaction(idb.TransactionReadOnly, contentsStore)
		if err != nil {
			return nil, err
		}
		files, err := txn.ObjectStore(contentsStore)
		if err != nil {
			return nil, err
		}
		req, err := files.Get(js.ValueOf(path))
		if err != nil {
			return nil, err
		}
		value, err := req.Await(context.Background())
		if value.IsUndefined() {
			return nil, hackpadfs.ErrNotExist
		}
		if err != nil {
			return nil, err
		}
		return newJSBlob(value)
	}
}

func (s *store) getDirNames(name string) func() ([]string, error) {
	return func() (_ []string, err error) {
		txn, err := s.db.Transaction(idb.TransactionReadOnly, infoStore)
		if err != nil {
			return nil, err
		}
		files, err := txn.ObjectStore(infoStore)
		if err != nil {
			return nil, err
		}

		parentIndex, err := files.Index(parentKey)
		if err != nil {
			return nil, err
		}
		keyRange, err := idb.NewKeyRangeOnly(js.ValueOf(name))
		if err != nil {
			return nil, err
		}
		keysReq, err := parentIndex.GetAllKeysRange(keyRange, 0)
		if err != nil {
			return nil, err
		}
		jsKeys, err := keysReq.Await(context.Background())
		var keys []string
		if err == nil {
			for _, jsKey := range jsKeys {
				keys = append(keys, path.Base(jsKey.String()))
			}
		}
		return keys, err
	}
}

func (s *store) getMode(fileRecord js.Value) hackpadfs.FileMode {
	mode := fileRecord.Get("Mode")
	return hackpadfs.FileMode(mode.Int())
}

const rootPath = "."

func (s *store) Set(name string, record keyvalue.FileRecord) error {
	if record == nil {
		if name == rootPath {
			return hackpadfs.ErrNotImplemented // cannot delete root dir
		}
		return s.deleteRecord(name)
	}

	var extraStores []string
	var data blob.Blob
	size := record.Size()
	regularFile := !record.Mode().IsDir()
	if regularFile {
		// this is a file, so include file contents
		extraStores = append(extraStores, contentsStore)

		// get data now, since it should not interrupt the transaction
		var err error
		data, err = record.Data()
		if err != nil {
			return err
		}
		size = int64(data.Len())
	}

	txn, err := s.db.Transaction(idb.TransactionReadWrite, infoStore, extraStores...)
	if err != nil {
		return err
	}

	if regularFile {
		contents, err := txn.ObjectStore(contentsStore)
		if err != nil {
			return err
		}
		_, err = contents.PutKey(js.ValueOf(name), toJSValue(data))
		if err != nil {
			return err
		}
	}
	fileInfo := map[string]interface{}{
		"ModTime": record.ModTime().UnixNano(),
		"Mode":    uint32(record.Mode()),
		"Size":    size,
	}
	if name != rootPath {
		fileInfo[parentKey] = path.Dir(name)
	}

	// include metadata update
	info, err := txn.ObjectStore(infoStore)
	if err != nil {
		return err
	}

	// verify a parent directory exists (except for root dir)
	dir := path.Dir(name)
	var noParentDir bool
	if dir != "" && dir != rootPath {
		req, err := info.Get(js.ValueOf(dir))
		if err != nil {
			return err
		}
		req.ListenSuccess(context.Background(), func() {
			result, err := req.Result()
			if err != nil {
				_ = txn.Abort()
				return
			}
			mode := s.getMode(result)
			if !mode.IsDir() {
				noParentDir = true
				_ = txn.Abort()
			}
		})
	}

	_, err = info.PutKey(js.ValueOf(name), js.ValueOf(fileInfo))
	if err != nil {
		return err
	}
	err = txn.Await(context.Background())
	if noParentDir {
		err = hackpadfs.ErrNotDir
	}
	return err
}

func (s *store) deleteRecord(p string) error {
	txn, err := s.db.Transaction(idb.TransactionReadWrite, infoStore, contentsStore)
	if err != nil {
		return err
	}
	info, err := txn.ObjectStore(infoStore)
	if err != nil {
		return err
	}
	contents, err := txn.ObjectStore(contentsStore)
	if err != nil {
		return err
	}
	_, err = info.Delete(js.ValueOf(p))
	if err != nil {
		return err
	}
	_, err = contents.Delete(js.ValueOf(p))
	if err != nil {
		return err
	}
	return txn.Await(context.Background())
}
