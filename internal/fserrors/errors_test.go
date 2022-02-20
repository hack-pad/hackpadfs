package fserrors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestWithMessage(t *testing.T) {
	t.Parallel()
	someError := fmt.Errorf("some error")
	err := WithMessage(someError, "some message")
	if assert.Error(t, err) {
		assert.Equal(t, "some message: some error", err.Error())
	}
	assert.Equal(t, true, errors.Is(err, someError))

	err = WithMessage(nil, "some message")
	assert.NoError(t, err)
}
