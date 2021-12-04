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
	modeMetadataKey    = "Mode"
	modTimeMetadataKey = "Modtime"
	modTimeFormat      = time.RFC3339Nano

	rootPath    = "files"
	filePrefix  = "file-"
	dirMetaName = "dir-meta"
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
	dir, file := path.Split(p)
	if isDir {
		return path.Join(rootPath, dir, file, dirMetaName)
	} else {
		return path.Join(rootPath, dir, filePrefix+file)
	}
}

func (s *store) objectKeyToFile(p string) string {
	p = strings.TrimPrefix(p, rootPath+"/")
	if strings.HasSuffix(p, "/") {
		p = path.Join(p, dirMetaName)
	}
	dir, file := path.Split(path.Clean(p))
	dir = path.Clean(dir)
	switch {
	case strings.HasPrefix(file, filePrefix):
		return path.Join(dir, strings.TrimPrefix(file, filePrefix))
	case file == dirMetaName:
		return dir
	default:
		panic(fmt.Sprintf("Unrecognized file name %q for full key %q ", file, p))
	}
}

func (s *store) wrapS3Err(err error) error {
	if err == nil {
		return nil
	}
	if minio.ToErrorResponse(err).Code == "NoSuchKey" {
		return hackpadfs.ErrNotExist
	}
	return err
}

func (s *store) stat(ctx context.Context, key string) (minio.ObjectInfo, error) {
	info, err := s.client.StatObject(ctx, s.options.BucketName, key, minio.StatObjectOptions{})
	return info, s.wrapS3Err(err)
}

func (s *store) Get(ctx context.Context, name string) (keyvalue.FileRecord, error) {
	key := s.fileToObjectKey(name, true)
	info, err := s.stat(ctx, key)
	if errors.Is(err, hackpadfs.ErrNotExist) {
		// not a directory, try file lookup instead
		key = s.fileToObjectKey(name, false)
		info, err = s.stat(ctx, key)
	}
	if err != nil {
		return nil, err
	}

	modTime := info.LastModified.UTC()
	if modTimeStr, ok := info.UserMetadata[modTimeMetadataKey]; ok {
		var err error
		modTime, err = time.Parse(modTimeFormat, modTimeStr)
		if err != nil {
			return nil, err
		}
	}

	var mode hackpadfs.FileMode
	if modeStr, ok := info.UserMetadata[modeMetadataKey]; ok {
		modeInt, err := strconv.ParseUint(modeStr, 8, 64)
		if err != nil {
			return nil, err
		}
		mode = hackpadfs.FileMode(modeInt)
	} else {
		return nil, hackpadfs.ErrInvalid
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
	prefix, _ := path.Split(path.Clean(key))
	return func() ([]string, error) {
		infoChan := s.client.ListObjects(context.Background(), s.options.BucketName, minio.ListObjectsOptions{
			Prefix: prefix,
		})
		var names []string
		for info := range infoChan {
			if info.Err != nil {
				return nil, s.wrapS3Err(info.Err)
			}
			if info.Key != key {
				filePath := s.objectKeyToFile(info.Key)
				names = append(names, path.Base(filePath))
			}
		}
		return names, nil
	}
}

func (s *store) getDataFunc(key string) func() (blob.Blob, error) {
	return func() (blob.Blob, error) {
		obj, err := s.client.GetObject(context.Background(), s.options.BucketName, key, minio.GetObjectOptions{})
		if err != nil {
			return nil, err
		}
		defer obj.Close()
		info, err := obj.Stat()
		if err != nil {
			return nil, err
		}

		buf := make([]byte, info.Size)
		_, err = obj.ReadAt(buf, 0)
		if err == io.EOF {
			err = nil
		}
		return blob.NewBytes(buf), err
	}
}

func (s *store) Set(ctx context.Context, name string, record keyvalue.FileRecord) (e error) {
	if record == nil {
		getRecord, err := s.Get(ctx, name)
		if errors.Is(err, hackpadfs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		key := s.fileToObjectKey(name, getRecord.Mode().IsDir())
		return s.client.RemoveObject(ctx, s.options.BucketName, key, minio.RemoveObjectOptions{})
	}

	key := s.fileToObjectKey(name, record.Mode().IsDir())

	if !record.Mode().IsDir() {
		existingRecord, err := s.Get(ctx, name)
		switch {
		case errors.Is(err, hackpadfs.ErrNotExist):
		case err != nil:
			return err
		case existingRecord.Mode().IsDir():
			return hackpadfs.ErrIsDir
		}
	}
	b, err := record.Data()
	if err != nil {
		return err
	}
	data := b.Bytes()
	length := b.Len()
	opts := minio.PutObjectOptions{
		UserMetadata: map[string]string{
			modeMetadataKey:    strconv.FormatUint(uint64(record.Mode()), 8),
			modTimeMetadataKey: record.ModTime().Format(modTimeFormat),
		},
	}
	_, err = s.client.PutObject(ctx, s.options.BucketName, key, bytes.NewReader(data), int64(length), opts)
	return err
}
