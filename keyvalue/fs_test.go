package keyvalue

import (
	"errors"
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestIgnoreErrExist(t *testing.T) {
	t.Parallel()
	someError := errors.New("some error")
	assert.Equal(t, someError, ignoreErrExist(someError))

	assert.Equal(t, nil, ignoreErrExist(hackpadfs.ErrExist))

	assert.Equal(t, nil, ignoreErrExist(&hackpadfs.PathError{Err: hackpadfs.ErrExist}))
}
