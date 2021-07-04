package keyvalue

import "context"

func (fs *FS) getFiles(paths ...string) ([]*file, []error) {
	results, err := getFileRecords(fs.store, paths)
	if err != nil {
		return nil, []error{err}
	}
	files := make([]*file, len(paths))
	errs := make([]error, len(paths))
	for i := range paths {
		result, err := results[i].Record, results[i].Err
		files[i], errs[i] = &file{
			fileData: &fileData{
				runOnceFileRecord: runOnceFileRecord{record: result},
				path:              paths[i],
				fs:                fs,
			},
		}, err
	}
	return files, errs
}

func getFileRecords(store *transactionOnly, paths []string) ([]OpResult, error) {
	txn, err := store.Transaction()
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		txn.Get(path)
	}
	return txn.Commit(context.Background())
}
