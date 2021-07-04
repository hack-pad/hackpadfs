package keyvalue

// Store holds arbitrary file data at the given 'path' location. Can be wrapped as a file system with keyvalue.NewFS().
type Store interface {
	// Get retrieves a file record for the given 'path'.
	// Returns an error if the path was not found or could not be retrieved.
	// If the path was not found, the error must satisfy errors.Is(err, hackpadfs.ErrNotExist).
	Get(path string) (FileRecord, error)
	// Set assigns 'src' to the given 'path'. Returns an error if the data could not be set.
	Set(path string, src FileRecord) error
}

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
