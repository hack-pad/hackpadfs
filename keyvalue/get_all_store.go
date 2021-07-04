package keyvalue

// GetAllStore is a Store that fetches files in bulk.
// Returns individual results with optional Record and Err fields for each path.
// The return slice must have the same length as paths.
type GetAllStore interface {
	GetAll(paths []string) []GetAllResult
}

type GetAllResult struct {
	Record FileRecord
	Err    error
}

func (fs *FS) getFiles(paths ...string) ([]*file, []error) {
	results := getFileRecords(fs.store, paths)
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

func getFileRecords(s Store, paths []string) []GetAllResult {
	if s, ok := s.(GetAllStore); ok {
		return s.GetAll(paths)
	}

	results := make([]GetAllResult, len(paths))
	for i, path := range paths {
		dest, err := s.Get(path)
		results[i] = GetAllResult{Record: dest, Err: err}
	}
	return results
}
