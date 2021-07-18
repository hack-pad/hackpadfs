package keyvalue

import (
	"errors"
	"testing"
	"time"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
	"github.com/hack-pad/hackpadfs/keyvalue/blob"
)

func TestBaseFileRecordData(t *testing.T) {
	t.Parallel()

	t.Run("not dir not regular file", func(t *testing.T) {
		t.Parallel()
		b := NewBaseFileRecord(0, time.Now(), 0, nil, nil, nil)
		_, err := b.Data()
		assert.Error(t, err)
		assert.Equal(t, true, errors.Is(err, hackpadfs.ErrNotImplemented))
	})

	t.Run("is dir", func(t *testing.T) {
		t.Parallel()
		b := NewBaseFileRecord(0, time.Now(), hackpadfs.ModeDir, nil, nil, nil)
		_, err := b.Data()
		assert.Error(t, err)
		assert.Equal(t, true, errors.Is(err, hackpadfs.ErrIsDir))
	})

	t.Run("regular file", func(t *testing.T) {
		t.Parallel()
		expectedData, expectedErr := blob.NewBytesLength(10), errors.New("some error")
		b := NewBaseFileRecord(0, time.Now(), 0, nil, func() (blob.Blob, error) {
			return expectedData, expectedErr
		}, nil)
		data, err := b.Data()
		assert.Equal(t, expectedData, data)
		assert.Equal(t, expectedErr, err)
	})
}

func TestBaseFileRecordReadDirNames(t *testing.T) {
	t.Parallel()

	t.Run("dir and missing dir names", func(t *testing.T) {
		t.Parallel()
		b := NewBaseFileRecord(0, time.Now(), hackpadfs.ModeDir, nil, nil, nil)
		_, err := b.ReadDirNames()
		assert.Error(t, err)
		assert.Equal(t, true, errors.Is(err, hackpadfs.ErrNotImplemented))
	})

	t.Run("is dir", func(t *testing.T) {
		t.Parallel()
		expectedDirNames, expectedErr := []string{"foo"}, errors.New("some error")
		b := NewBaseFileRecord(0, time.Now(), 0, nil, nil, func() ([]string, error) {
			return expectedDirNames, expectedErr
		})
		dirNames, err := b.ReadDirNames()
		assert.Equal(t, expectedDirNames, dirNames)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("regular file", func(t *testing.T) {
		t.Parallel()
		b := NewBaseFileRecord(0, time.Now(), 0, nil, nil, nil)
		_, err := b.ReadDirNames()
		assert.Error(t, err)
		assert.Equal(t, true, errors.Is(err, hackpadfs.ErrNotDir))
	})
}

func TestBaseFileRecordSys(t *testing.T) {
	t.Parallel()
	const someSys = "some sys"
	assert.Equal(t, someSys, (&BaseFileRecord{sys: someSys}).Sys())
}
