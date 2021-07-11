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

func (s *store) Get(path string) (keyvalue.FileRecord, error) {
	txn, err := s.db.Transaction(idb.TransactionReadOnly, infoStore)
	if err != nil {
		return nil, err
	}
	files, err := txn.ObjectStore(infoStore)
	if err != nil {
		return nil, err
	}
	req, err := s.getFile(files, path)
	if err != nil {
		return nil, err
	}
	return req.Await(context.Background())
}

func (s *store) getFile(files *idb.ObjectStore, path string) (*getFileRequest, error) {
	req, err := files.Get(js.ValueOf(path))
	if err != nil {
		return nil, err
	}
	return newGetFileRequest(s, path, req), nil
}

type getFileRequest struct {
	store *store
	path  string
	req   *idb.Request
}

func newGetFileRequest(s *store, path string, req *idb.Request) *getFileRequest {
	return &getFileRequest{
		store: s,
		path:  path,
		req:   req,
	}
}

func (g *getFileRequest) Await(ctx context.Context) (keyvalue.FileRecord, error) {
	return g.parseResult(g.req.Await(ctx))
}

func (g *getFileRequest) Result() (keyvalue.FileRecord, error) {
	return g.parseResult(g.req.Result())
}

func (g *getFileRequest) parseResult(result js.Value, err error) (keyvalue.FileRecord, error) {
	if result.IsUndefined() {
		return nil, hackpadfs.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	initialSize := int64(result.Get("Size").Int())
	modTime := time.Unix(0, int64(result.Get("ModTime").Int()))
	mode := getMode(result)
	var getData func() (blob.Blob, error)
	var getDirNames func() ([]string, error)
	if mode.IsDir() {
		getDirNames = g.store.getDirNames(g.path)
	} else {
		getData = g.store.getFileData(g.path)
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

func getMode(fileRecord js.Value) hackpadfs.FileMode {
	mode := fileRecord.Get("Mode")
	return hackpadfs.FileMode(mode.Int())
}

const rootPath = "."

func (s *store) Set(name string, record keyvalue.FileRecord) error {
	includeContents := record == nil || !record.Mode().IsDir() // i.e. "should delete" OR "is a regular file"
	stores := []string{infoStore}
	if includeContents {
		stores = append(stores, contentsStore)
	}
	var data blob.Blob
	if record != nil {
		// get data now, since it should not interrupt the transaction
		var err error
		data, err = record.Data()
		if err != nil {
			return err
		}
	}

	txn, err := s.db.Transaction(idb.TransactionReadWrite, stores[0], stores[1:]...)
	if err != nil {
		return err
	}
	infos, err := txn.ObjectStore(infoStore)
	if err != nil {
		return err
	}
	var contents *idb.ObjectStore
	if includeContents {
		contents, err = txn.ObjectStore(contentsStore)
		if err != nil {
			return err
		}
	}

	if record == nil {
		if name == rootPath {
			return hackpadfs.ErrNotImplemented // cannot delete root dir
		}
		err = deleteRecord(infos, contents, name)
		if err != nil {
			return err
		}
		return txn.Await(context.Background())
	}

	if includeContents {
		err := setFileContents(contents, name, data)
		if err != nil {
			return err
		}
	}
	// always set metadata to update size when contents change
	validateErrs, err := validateAndSetFileMeta(infos, name, record, data)
	if err != nil {
		return err
	}
	err = txn.Await(context.Background())
	if vErr := <-validateErrs; vErr != nil {
		return vErr
	}
	return err
}

func deleteRecord(infos, contents *idb.ObjectStore, name string) error {
	jsName := js.ValueOf(name)
	_, err := infos.Delete(jsName)
	if err != nil {
		return err
	}
	_, err = contents.Delete(jsName)
	return err
}

func setFileContents(contents *idb.ObjectStore, name string, data blob.Blob) error {
	_, err := contents.PutKey(js.ValueOf(name), toJSValue(data))
	return err
}

// validateAndSetFileMeta verifies the file by 'name' has a parent directory, then updates the file metadata. If not nil, 'data' is used to detect size instead of record.Size().
func validateAndSetFileMeta(infos *idb.ObjectStore, name string, record keyvalue.FileRecord, data blob.Blob) (<-chan error, error) {
	var size int64
	if data == nil {
		size = record.Size()
	} else {
		size = int64(data.Len())
	}
	fileInfo := map[string]interface{}{
		"ModTime": record.ModTime().UnixNano(),
		"Mode":    uint32(record.Mode()),
		"Size":    size,
	}
	if name != rootPath {
		fileInfo[parentKey] = path.Dir(name)
	}

	parentExistsErr, err := requireParentDirectoryExists(infos, name)
	if err != nil {
		return nil, err
	}

	_, err = infos.PutKey(js.ValueOf(name), js.ValueOf(fileInfo))
	return parentExistsErr, err
}

// requireParentDirectoryExists returns an async err chan. Async error is nil if directory exists.
func requireParentDirectoryExists(infos *idb.ObjectStore, name string) (<-chan error, error) {
	existsErr := make(chan error, 1)
	dir := path.Dir(name)
	if dir == "" || dir == rootPath {
		existsErr <- nil
		close(existsErr)
		return existsErr, nil
	}

	req, err := infos.Get(js.ValueOf(dir))
	if err != nil {
		return nil, err
	}
	req.Listen(context.Background(), func() {
		result, err := req.Result()
		if err != nil {
			if txn, err := infos.Transaction(); err == nil {
				_ = txn.Abort()
			}
			existsErr <- err
			close(existsErr)
			return
		}
		mode := getMode(result)
		if !mode.IsDir() {
			if txn, err := infos.Transaction(); err == nil {
				_ = txn.Abort()
			}
			existsErr <- hackpadfs.ErrNotDir
			close(existsErr)
			return
		}
		existsErr <- nil
		close(existsErr)
	}, func() {
		existsErr <- nil
		close(existsErr)
	})
	return existsErr, nil
}
