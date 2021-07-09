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

func newStore(db *idb.Database) keyvalue.Store {
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
	return s.extractFileRecord(path, value, err)
}

func (s *store) extractFileRecord(path string, value js.Value, err error) (keyvalue.FileRecord, error) {
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

func (s *store) Set(path string, data keyvalue.FileRecord) error {
	isRoot := path == rootPath
	if data == nil && isRoot {
		return hackpadfs.ErrNotImplemented // cannot delete root dir
	}
	err := s.setFile(path, data)
	if err != nil {
		// TODO Verify if AbortError type. If it isn't, then don't replace with syscall.ENOTDIR.
		// Should be the only reason for an abort. Later use an error handling mechanism in indexeddb pkg.
		err = hackpadfs.ErrNotDir
	}
	return err
}

func (s *store) setFile(p string, data keyvalue.FileRecord) error {
	if data == nil {
		return s.deleteRecord(p)
	}

	var extraStores []string
	var dataBlob blob.Blob
	regularFile := !data.Mode().IsDir()
	if regularFile {
		// this is a file, so include file contents
		extraStores = append(extraStores, contentsStore)

		// get data now, since it should not interrupt the transaction
		var err error
		dataBlob, err = data.Data()
		if err != nil {
			return err
		}
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
		_, err = contents.PutKey(js.ValueOf(p), toJSValue(dataBlob))
		if err != nil {
			return err
		}
	}
	fileInfo := map[string]interface{}{
		"ModTime": data.ModTime().UnixNano(),
		"Mode":    uint32(data.Mode()),
		"Size":    data.Size(),
	}
	if p != rootPath {
		fileInfo[parentKey] = path.Dir(p)
	}

	// include metadata update
	info, err := txn.ObjectStore(infoStore)
	if err != nil {
		return err
	}

	// verify a parent directory exists (except for root dir)
	dir := path.Dir(p)
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
				_ = txn.Abort()
			}
		})
	}

	_, err = info.PutKey(js.ValueOf(p), js.ValueOf(fileInfo))
	if err != nil {
		return err
	}
	return txn.Await(context.Background())
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
