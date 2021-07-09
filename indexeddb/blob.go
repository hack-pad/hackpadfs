// +build wasm

package indexeddb

import (
	"errors"
	"fmt"
	"sync/atomic"
	"syscall/js"

	"github.com/hack-pad/hackpadfs/internal/exception"
	"github.com/hack-pad/hackpadfs/keyvalue/blob"
)

var uint8Array = js.Global().Get("Uint8Array")

var (
	_ interface {
		blob.Blob
		blob.ViewBlob
		blob.SliceBlob
		blob.SetBlob
		blob.GrowBlob
		blob.TruncateBlob
	} = &jsBlob{}
)

type jsBlob struct {
	bytes   atomic.Value // *blob.Bytes
	jsValue atomic.Value // js.Value
	length  int64
}

func newJSBlob(buf js.Value) (_ blob.Blob, returnedErr error) {
	defer exception.Catch(&returnedErr)
	if !buf.Truthy() {
		return blob.NewBytes(nil), nil
	}
	if !buf.InstanceOf(uint8Array) {
		return nil, fmt.Errorf("invalid JS array type: %v", buf)
	}
	b := &jsBlob{}
	b.jsValue.Store(buf)
	atomic.StoreInt64(&b.length, int64(buf.Length()))
	return b, nil
}

func toJSValue(b blob.Blob) js.Value {
	if b, ok := b.(js.Wrapper); ok {
		return b.JSValue()
	}
	buf := b.Bytes()
	jsBuf := uint8Array.New(len(buf))
	js.CopyBytesToJS(jsBuf, buf)
	return jsBuf
}

func (b *jsBlob) currentBytes() *blob.Bytes {
	buf := b.bytes.Load()
	if buf != nil {
		return buf.(*blob.Bytes)
	}
	return nil
}

func (b *jsBlob) Bytes() []byte {
	if buf := b.currentBytes(); buf != nil {
		return buf.Bytes()
	}
	jsBuf := b.jsValue.Load().(js.Value)
	buf := make([]byte, jsBuf.Length())
	js.CopyBytesToGo(buf, jsBuf)
	b.bytes.Store(buf)
	return buf
}

func (b *jsBlob) JSValue() js.Value {
	return b.jsValue.Load().(js.Value)
}

func (b *jsBlob) Len() int {
	return int(atomic.LoadInt64(&b.length))
}

func catchErr(fn func() error) (returnedErr error) {
	defer exception.Catch(&returnedErr)
	return fn()
}

func (b *jsBlob) View(start, end int64) (blob.Blob, error) {
	if start == 0 && end == atomic.LoadInt64(&b.length) {
		return b, nil
	}

	var newBlob *jsBlob
	err := catchErr(func() error {
		b, err := newJSBlob(b.JSValue().Call("subarray", start, end))
		newBlob, _ = b.(*jsBlob)
		return err
	})
	if err != nil {
		return nil, err
	}

	if buf := b.currentBytes(); buf != nil {
		newBytesBlob, err := b.currentBytes().View(start, end)
		if err != nil {
			panic(err)
		}
		newBlob.bytes.Store(newBytesBlob)
	}
	return newBlob, nil
}

func (b *jsBlob) Slice(start, end int64) (blob.Blob, error) {
	if start < 0 || start > int64(b.Len()) {
		return nil, fmt.Errorf("Start index out of bounds: %d", start)
	}
	if end < 0 || end > int64(b.Len()) {
		return nil, fmt.Errorf("End index out of bounds: %d", end)
	}

	newBlob, err := newJSBlob(b.JSValue().Call("slice", start, end))
	if err != nil {
		return nil, err
	}
	if buf := b.currentBytes(); buf != nil {
		newJSBlob := newBlob.(*jsBlob)
		newBytes, err := buf.Slice(start, end)
		if err != nil {
			panic(err)
		}
		newJSBlob.bytes.Store(newBytes)
	}
	return newBlob, nil
}

func (b *jsBlob) Set(dest blob.Blob, srcStart int64) (n int, err error) {
	if srcStart < 0 {
		return 0, errors.New("negative offset")
	}
	if srcStart >= int64(b.Len()) {
		return 0, fmt.Errorf("Offset out of bounds: %d", srcStart)
	}

	err = catchErr(func() error {
		b.JSValue().Call("set", toJSValue(dest), srcStart)
		return nil
	})
	if err != nil {
		return 0, err
	}
	n = dest.Len()

	if buf := b.currentBytes(); buf != nil {
		_, err := buf.Set(dest, srcStart)
		if err != nil {
			panic(err)
		}
	}
	return n, nil
}

func (b *jsBlob) Grow(off int64) error {
	newLength := atomic.LoadInt64(&b.length) + off

	err := catchErr(func() error {
		buf := b.jsValue.Load().(js.Value)
		biggerBuf := uint8Array.New(newLength)
		biggerBuf.Call("set", buf, 0)
		b.jsValue.Store(biggerBuf)
		return nil
	})
	if err != nil {
		return err
	}
	atomic.StoreInt64(&b.length, newLength)

	if buf := b.currentBytes(); buf != nil {
		err := buf.Grow(off)
		if err != nil {
			panic(err)
		}
	}
	return nil
}

func (b *jsBlob) Truncate(size int64) error {
	if atomic.LoadInt64(&b.length) < size {
		return nil
	}

	err := catchErr(func() error {
		smallerBuf := b.JSValue().Call("slice", 0, size)
		b.jsValue.Store(smallerBuf)
		return nil
	})
	if err != nil {
		return err
	}
	atomic.StoreInt64(&b.length, size)

	if buf := b.currentBytes(); buf != nil {
		err := buf.Truncate(size)
		if err != nil {
			return err
		}
	}
	return nil
}
