package s3

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sync/atomic"
	"testing"
	"time"
	"unicode"

	"github.com/hack-pad/hackpadfs/fstest"
	"github.com/hack-pad/hackpadfs/internal/assert"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	minioServer "github.com/minio/minio/cmd"
)

const (
	maxBucketPrefixLength = 50
	testDBHost            = "localhost:9000"
	testDBAccessKeyID     = "minioadmin"
	testDBSecretKey       = "minioadmin"
)

var minioClient *minio.Client

func init() {
	path, err := ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}
	go func() {
		defer func() {
			_ = os.RemoveAll(path)
		}()
		minioServer.Main([]string{
			"minio", "server", path,
			"--address", ":9000",
			"--console-address", ":9001",
		})
	}()

	minioClient, err = minio.New(testDBHost, &minio.Options{
		Creds:  credentials.NewStaticV4(testDBAccessKeyID, testDBSecretKey, ""),
		Secure: false,
	})
	if err != nil {
		panic(err)
	}
	time.Sleep(2 * time.Second)
	_, err = minioClient.ListBuckets(context.Background())
	if err != nil {
		panic(err)
	}
}

func cleanTestName(tb testing.TB) string {
	nameRunes := []rune(tb.Name())
	for i, r := range nameRunes {
		// Naming rules: https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html
		switch {
		case unicode.IsLetter(r):
			if !unicode.IsLower(r) {
				nameRunes[i] = unicode.ToLower(r)
			}
		case unicode.IsNumber(r) || r == '-':
		default:
			nameRunes[i] = '-'
		}
	}
	if nameRunes[0] == '-' {
		nameRunes[0] = '0'
	}
	if lastRuneIx := len(nameRunes) - 1; nameRunes[lastRuneIx] == '-' {
		nameRunes[lastRuneIx] = '0'
	}
	name := string(nameRunes)
	if len(name) > maxBucketPrefixLength {
		name = name[:maxBucketPrefixLength]
	}
	return name
}

var testNumber uint64

func makeFS(tb testing.TB) *FS {
	bucketName := fmt.Sprintf("%s-%d", cleanTestName(tb), atomic.AddUint64(&testNumber, 1))

	ctx := context.Background()
	err := minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		tb.Fatal(err)
	}

	fs, err := NewFS(Options{
		Endpoint:        testDBHost,
		BucketName:      bucketName,
		Insecure:        true,
		AccessKeyID:     testDBAccessKeyID,
		SecretAccessKey: testDBSecretKey,
	})
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		assert.NoError(tb, minioClient.RemoveBucketWithOptions(ctx, bucketName, minio.RemoveBucketOptions{
			ForceDelete: true,
		}))
	})
	return fs
}

func TestFS(t *testing.T) {
	t.Parallel()
	options := fstest.FSOptions{
		Name: "s3",
		TestFS: func(tb testing.TB) fstest.SetupFS {
			return makeFS(tb)
		},
	}
	fstest.FS(t, options)
	fstest.File(t, options)
}
