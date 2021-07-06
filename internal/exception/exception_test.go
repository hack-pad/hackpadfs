// +build wasm

package exception

import (
	"errors"
	"syscall/js"
	"testing"

	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestCatch(t *testing.T) {
	t.Parallel()

	t.Run("no error and no panic", func(t *testing.T) {
		t.Parallel()
		resultErr := func() (err error) {
			defer Catch(&err)
			// no-op
			return nil
		}()
		assert.NoError(t, resultErr)
	})

	t.Run("error and no panic", func(t *testing.T) {
		t.Parallel()
		someErr := errors.New("my error")
		resultErr := func() (err error) {
			defer Catch(&err)
			// no-op
			return someErr
		}()
		assert.Equal(t, someErr, resultErr)
	})

	t.Run("panic with error", func(t *testing.T) {
		t.Parallel()
		someErr := errors.New("some error")
		resultErr := func() (err error) {
			defer Catch(&err)
			panic(someErr)
		}()
		assert.Equal(t, someErr, resultErr)
	})

	t.Run("panic with js.Value", func(t *testing.T) {
		t.Parallel()
		someErr := testJSErrValue()
		resultErr := func() (err error) {
			defer Catch(&err)
			panic(someErr)
		}()
		assert.Equal(t, js.Error{Value: someErr}, resultErr)
	})

	t.Run("panic with other type", func(t *testing.T) {
		t.Parallel()
		someErr := "some other type"
		resultErr := func() (err error) {
			defer Catch(&err)
			panic(someErr)
		}()
		assert.Equal(t, someErr, resultErr.Error())
	})
}

func testJSErrValue() (value js.Value) {
	defer func() {
		recoverVal := recover()
		value = recoverVal.(js.Wrapper).JSValue()
	}()
	js.Global().Get("Function").New(`throw Exception("some error")`).Invoke()
	panic("not a JS value. line above should do the panic")
}

func TestCatchHandler(t *testing.T) {
	t.Parallel()
	var calledHandler bool
	resultErr := func() (err error) {
		defer CatchHandler(func(handlerErr error) {
			calledHandler = true
			err = handlerErr
		})
		panic("some error")
	}()
	if assert.Error(t, resultErr) {
		assert.Equal(t, "some error", resultErr.Error())
	}
	assert.Equal(t, true, calledHandler)
}
