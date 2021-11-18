package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/keyvalue"
	"github.com/hack-pad/hackpadfs/keyvalue/blob"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	_ interface {
		keyvalue.Store
	} = &store{}
)

const (
	// for some reason these keys must be Header-cased
	modeMetadataKey = "Mode"
	modTimeKey      = "Modtime"
	modTimeFormat   = time.RFC3339Nano

	rootPath   = "files"
	filePrefix = "file-"
	dirRoot    = "directory"
)

type store struct {
	options Options
	client  *minio.Client
}

func newStore(options Options) (*store, error) {
	client, err := minio.New(options.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(options.AccessKeyID, options.SecretAccessKey, ""),
		Secure: !options.Insecure,
	})
	if err != nil {
		return nil, err
	}
	return &store{
		options: options,
		client:  client,
	}, nil
}

func (s *store) fileToObjectKey(p string, isDir bool) string {
	if isDir {
		return path.Join(rootPath, p, dirRoot)
	} else {
		return path.Join(rootPath, filePrefix+p)
	}
}

func (s *store) objectKeyToFile(p string) (filePath string, isDir bool) {
	p = strings.TrimPrefix(p, rootPath+"/")
	if path.Base(p) == dirRoot {
		return path.Dir(p), true
	} else {
		dir, base := path.Split(p)
		fileName := strings.TrimPrefix(base, filePrefix)
		return path.Join(dir, fileName), false
	}
}

func (s *store) resolveFSErr(err error) error {
	if minio.ToErrorResponse(err).Code == "NoSuchKey" {
		return hackpadfs.ErrNotExist
	}
	return err
}

func (s *store) openObject(ctx context.Context, key string) (*minio.Object, minio.ObjectInfo, error) {
	obj, err := s.client.GetObject(ctx, s.options.BucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, minio.ObjectInfo{}, err
	}
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, minio.ObjectInfo{}, s.resolveFSErr(err)
	}
	return obj, info, nil
}

func (s *store) Get(ctx context.Context, name string) (keyvalue.FileRecord, error) {
	key := s.fileToObjectKey(name, false)
	fmt.Println("-- Getting:", name, key)
	obj, info, err := s.openObject(ctx, key)
	if errors.Is(err, hackpadfs.ErrNotExist) {
		key = s.fileToObjectKey(name, true)
		obj, info, err = s.openObject(ctx, key)
	}
	if err != nil {
		return nil, err
	}
	defer obj.Close()

	modTime := info.LastModified.UTC()
	if modTimeStr, ok := info.UserMetadata[modTimeKey]; ok {
		var err error
		modTime, err = time.Parse(modTimeFormat, modTimeStr)
		if err != nil {
			return nil, err
		}
	}

	mode := hackpadfs.FileMode(0777)
	if modeStr, ok := info.UserMetadata[modeMetadataKey]; ok {
		modeInt, err := strconv.ParseUint(modeStr, 8, 64)
		if err != nil {
			return nil, err
		}
		mode = hackpadfs.FileMode(modeInt)
	}

	var getData func() (blob.Blob, error)
	var getDirNames func() ([]string, error)
	if mode.IsDir() {
		getDirNames = s.getDirNamesFunc(key)
	} else {
		getData = s.getDataFunc(key)
	}

	return keyvalue.NewBaseFileRecord(info.Size, modTime, mode, nil, getData, getDirNames), nil
}

func (s *store) getDirNamesFunc(key string) func() ([]string, error) {
	return func() ([]string, error) {
		infoChan := s.client.ListObjects(context.Background(), s.options.BucketName, minio.ListObjectsOptions{
			Prefix: key + "/",
		})
		var names []string
		for info := range infoChan {
			if info.Err != nil {
				return nil, s.resolveFSErr(info.Err)
			}
			fmt.Println("-- Listing ", key, " found:", info.Key)
			filePath, isDir := s.objectKeyToFile(info.Key)
			if !isDir {
				names = append(names, path.Base(filePath))
			}
		}
		return names, nil
	}
}

func (s *store) getDataFunc(key string) func() (blob.Blob, error) {
	return func() (blob.Blob, error) {
		obj, info, err := s.openObject(context.Background(), key)
		if err != nil {
			return nil, err
		}
		defer obj.Close()

		buf := make([]byte, info.Size)
		_, err = obj.ReadAt(buf, 0)
		if err == io.EOF {
			err = nil
		}
		return blob.NewBytes(buf), err
	}
}

func (s *store) Set(ctx context.Context, name string, record keyvalue.FileRecord) error {
	if record == nil {
		return s.delete(ctx, name)
	}

	key := s.fileToObjectKey(name, record.Mode().IsDir())
	fmt.Println("-- Setting:", name, key, record)
	b, err := record.Data()
	if err != nil {
		return err
	}
	data := b.Bytes()
	length := b.Len()
	_, err = s.client.PutObject(ctx, s.options.BucketName, key, bytes.NewReader(data), int64(length), minio.PutObjectOptions{
		UserMetadata: map[string]string{
			modeMetadataKey: strconv.FormatUint(uint64(record.Mode()), 8),
			modTimeKey:      record.ModTime().Format(modTimeFormat),
		},
	})
	fmt.Printf("Upload error: %#v\n", err)
	return err
}

func (s *store) delete(ctx context.Context, name string) error {
	record, err := s.Get(ctx, name)
	if err != nil {
		return err
	}
	isDir := record.Mode().IsDir()
	if isDir {
		dirNames, err := record.ReadDirNames()
		if err != nil {
			return err
		}
		if len(dirNames) > 0 {
			return hackpadfs.ErrNotEmpty
		}
	}

	key := s.fileToObjectKey(name, isDir)
	fmt.Println("-- Deleting:", name, key)
	return s.client.RemoveObject(ctx, s.options.BucketName, key, minio.RemoveObjectOptions{})
}
