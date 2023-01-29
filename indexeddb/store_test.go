package indexeddb

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
	"github.com/hack-pad/hackpadfs/keyvalue"
	"github.com/hack-pad/hackpadfs/keyvalue/blob"
)

func init() {
	log.SetOutput(io.Discard)
}

func nowTruncated() time.Time {
	t := time.Now()
	return t.Truncate(time.Second)
}

func testFile(contents string) (keyvalue.FileRecord, blob.Blob) {
	data := []byte(contents)
	return keyvalue.NewBaseFileRecord(
		int64(len(data)),
		nowTruncated(),
		0600,
		nil,
		func() (blob.Blob, error) {
			return blob.NewBytes(data), nil
		},
		nil,
	), blob.NewBytes(data)
}

func TestStoreGetSet(t *testing.T) {
	store := newStore(makeFS(t).db, Options{})

	ctx := context.Background()
	_, err := store.Get(ctx, "bar")
	assert.ErrorIs(t, hackpadfs.ErrNotExist, err)

	setRecord, _ := testFile("baz")
	err = store.Set(ctx, "bar", setRecord)
	assert.NoError(t, err)

	getRecord, err := store.Get(ctx, "bar")
	assert.NoError(t, err)

	assert.Equal(t, setRecord.ModTime(), getRecord.ModTime())
	assert.Equal(t, setRecord.Mode(), getRecord.Mode())
	assert.Equal(t, setRecord.Size(), getRecord.Size())
	assert.Equal(t, setRecord.Sys(), getRecord.Sys())

	setData, err := setRecord.Data()
	assert.NoError(t, err)
	getData, err := getRecord.Data()
	assert.NoError(t, err)
	assert.Equal(t, setData.Bytes(), getData.Bytes())
	assert.Equal(t, setData.Len(), getData.Len())

	_, setErr := setRecord.ReadDirNames()
	_, getErr := getRecord.ReadDirNames()
	assert.Equal(t, setErr, getErr)
}
