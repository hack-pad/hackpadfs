//go:build wasm
// +build wasm

package indexeddb

import (
	"context"
	"path"
	"time"

	"github.com/hack-pad/go-indexeddb/idb"
	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/indexeddb/idbblob"
	"github.com/hack-pad/hackpadfs/keyvalue"
	"github.com/hack-pad/hackpadfs/keyvalue/blob"
	"github.com/hack-pad/safejs"
)

var (
	_ interface {
		keyvalue.Store
		keyvalue.TransactionStore
	} = &store{}
)

type store struct {
	db      *idb.Database
	options Options
}

func newStore(db *idb.Database, options Options) *store {
	return &store{db: db, options: options}
}

func (s *store) Get(ctx context.Context, path string) (keyvalue.FileRecord, error) {
	txn, err := s.db.TransactionWithOptions(idb.TransactionOptions{
		Mode:       idb.TransactionReadOnly,
		Durability: s.options.TransactionDurability,
	}, infoStore)
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
	return req.Await(ctx)
}

func (s *store) getFile(files *idb.ObjectStore, path string) (*getFileRequest, error) {
	jsPath, err := safejs.ValueOf(path)
	if err != nil {
		return nil, err
	}
	req, err := files.Get(safejs.Unsafe(jsPath))
	if err != nil {
		return nil, err
	}
	return newGetFileRequest(s, path, req), nil
}

type getFileRequest struct {
	*idb.Request
	store *store
	path  string
}

func newGetFileRequest(s *store, path string, req *idb.Request) *getFileRequest {
	return &getFileRequest{
		Request: req,
		store:   s,
		path:    path,
	}
}

func (g *getFileRequest) Await(ctx context.Context) (keyvalue.FileRecord, error) {
	result, err := g.Request.Await(ctx)
	return g.parseResult(safejs.Safe(result), err)
}

func (g *getFileRequest) Result() (keyvalue.FileRecord, error) {
	result, err := g.Request.Result()
	return g.parseResult(safejs.Safe(result), err)
}

func (g *getFileRequest) parseResult(result safejs.Value, err error) (keyvalue.FileRecord, error) {
	if result.IsUndefined() {
		return nil, hackpadfs.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	jsInitialSize, err := result.Get("Size")
	if err != nil {
		return nil, err
	}
	initialSize, err := jsInitialSize.Int()
	if err != nil {
		return nil, err
	}
	jsModTime, err := result.Get("ModTime")
	if err != nil {
		return nil, err
	}
	intModTime, err := jsModTime.Int()
	if err != nil {
		return nil, err
	}
	modTime := time.Unix(0, int64(intModTime))
	mode, err := getMode(result)
	if err != nil {
		return nil, err
	}
	var getData func() (blob.Blob, error)
	var getDirNames func() ([]string, error)
	if mode.IsDir() {
		getDirNames = g.store.getDirNames(g.path)
	} else {
		getData = g.store.getFileData(g.path)
	}
	return keyvalue.NewBaseFileRecord(int64(initialSize), modTime, mode, nil, getData, getDirNames), nil
}

func (s *store) getFileData(path string) func() (blob.Blob, error) {
	return func() (blob.Blob, error) {
		txn, err := s.db.TransactionWithOptions(idb.TransactionOptions{
			Mode:       idb.TransactionReadOnly,
			Durability: s.options.TransactionDurability,
		}, contentsStore)
		if err != nil {
			return nil, err
		}
		files, err := txn.ObjectStore(contentsStore)
		if err != nil {
			return nil, err
		}
		jsPath, err := safejs.ValueOf(path)
		if err != nil {
			return nil, err
		}
		req, err := files.Get(safejs.Unsafe(jsPath))
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
		return idbblob.New(value)
	}
}

func (s *store) getDirNames(name string) func() ([]string, error) {
	return func() (_ []string, err error) {
		txn, err := s.db.TransactionWithOptions(idb.TransactionOptions{
			Mode:       idb.TransactionReadOnly,
			Durability: s.options.TransactionDurability,
		}, infoStore)
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
		jsName, err := safejs.ValueOf(name)
		if err != nil {
			return nil, err
		}
		keyRange, err := idb.NewKeyRangeOnly(safejs.Unsafe(jsName))
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

func getMode(fileRecord safejs.Value) (hackpadfs.FileMode, error) {
	mode, err := fileRecord.Get("Mode")
	if err != nil {
		return 0, err
	}
	intMode, err := mode.Int()
	return hackpadfs.FileMode(intMode), err
}

const rootPath = "."

func (s *store) Set(ctx context.Context, name string, record keyvalue.FileRecord) error {
	includeContents := record == nil || record.Mode().IsRegular() // i.e. "should delete" OR "is a regular file"
	stores := []string{infoStore}
	if includeContents {
		stores = append(stores, contentsStore)
	}
	var data blob.Blob
	if record != nil && record.Mode().IsRegular() {
		// get data now, since it should not interrupt the transaction
		var err error
		data, err = record.Data()
		if err != nil {
			return err
		}
	}

	txn, err := s.db.TransactionWithOptions(idb.TransactionOptions{
		Mode:       idb.TransactionReadWrite,
		Durability: s.options.TransactionDurability,
	}, stores[0], stores[1:]...)
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
		return txn.Await(ctx)
	}

	if includeContents {
		err := setFileContents(contents, name, data)
		if err != nil {
			return err
		}
	}
	// always set metadata to update size when contents change
	_, parentExistsReq, err := validateAndSetFileMeta(ctx, infos, name, record, data)
	if err != nil {
		return err
	}
	err = txn.Await(ctx)
	if pErr := parentExistsReq.Err(); pErr != nil {
		return pErr
	}
	return err
}

func deleteRecord(infos, contents *idb.ObjectStore, name string) error {
	jsName, err := safejs.ValueOf(name)
	if err != nil {
		return err
	}
	_, err = infos.Delete(safejs.Unsafe(jsName))
	if err != nil {
		return err
	}
	_, err = contents.Delete(safejs.Unsafe(jsName))
	return err
}

func setFileContents(contents *idb.ObjectStore, name string, data blob.Blob) error {
	jsName, err := safejs.ValueOf(name)
	if err != nil {
		return err
	}
	_, err = contents.PutKey(safejs.Unsafe(jsName), idbblob.FromBlob(data).JSValue())
	return err
}

// validateAndSetFileMeta verifies the file by 'name' has a parent directory, then updates the file metadata. If not nil, 'data' is used to detect size instead of record.Size().
func validateAndSetFileMeta(ctx context.Context, infos *idb.ObjectStore, name string, record keyvalue.FileRecord, data blob.Blob) (*idb.Request, *parentDirExistsReq, error) {
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

	parentExistsReq, err := requireParentDirectoryExists(ctx, infos, name)
	if err != nil {
		return nil, nil, err
	}

	jsName, err := safejs.ValueOf(name)
	if err != nil {
		return nil, nil, err
	}
	jsFileInfo, err := safejs.ValueOf(fileInfo)
	if err != nil {
		return nil, nil, err
	}
	req, err := infos.PutKey(safejs.Unsafe(jsName), safejs.Unsafe(jsFileInfo))
	return req, parentExistsReq, err
}

type parentDirExistsReq struct {
	*idb.Request
	notExists bool
}

func (p *parentDirExistsReq) Result() error {
	_, err := p.Request.Result()
	if err != nil {
		return err
	}
	return p.Err()
}

func (p *parentDirExistsReq) Err() error {
	if p.notExists {
		return hackpadfs.ErrNotDir
	}
	return p.Request.Err()
}

// requireParentDirectoryExists returns an async err chan. Async error is nil if directory exists.
func requireParentDirectoryExists(ctx context.Context, infos *idb.ObjectStore, name string) (*parentDirExistsReq, error) {
	dir := path.Dir(name)
	if dir == "" || dir == rootPath {
		return nil, nil
	}

	jsDir, err := safejs.ValueOf(dir)
	if err != nil {
		return nil, err
	}
	req, err := infos.Get(safejs.Unsafe(jsDir))
	if err != nil {
		return nil, err
	}
	parentReq := &parentDirExistsReq{
		Request: req,
	}
	listenErr := req.ListenSuccess(ctx, func() {
		result, err := req.Result()
		if err != nil {
			if txn, err := infos.Transaction(); err == nil {
				_ = txn.Abort()
			}
			return
		}
		mode, err := getMode(safejs.Safe(result))
		if err == nil && !mode.IsDir() {
			if txn, err := infos.Transaction(); err == nil {
				_ = txn.Abort()
			}
			parentReq.notExists = true
			return
		}
	})
	return parentReq, listenErr
}

func (s *store) Transaction(options keyvalue.TransactionOptions) (keyvalue.Transaction, error) {
	mode := idb.TransactionReadOnly
	stores := []string{infoStore}
	if options.Mode == keyvalue.TransactionReadWrite {
		mode = idb.TransactionReadWrite
		stores = append(stores, contentsStore)
	}
	ctx, cancel := context.WithCancel(context.Background())
	txn, err := s.db.TransactionWithOptions(idb.TransactionOptions{
		Mode:       mode,
		Durability: s.options.TransactionDurability,
	}, stores[0], stores[1:]...)
	return &transaction{
		ctx:     ctx,
		abort:   cancel,
		store:   s,
		txn:     txn,
		results: make(map[keyvalue.OpID]keyvalue.OpResult),
	}, err
}
