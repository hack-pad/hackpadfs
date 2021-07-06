// +build wasm

package indexeddb

import (
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
	bytes   atomic.Value // []byte
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

func (b *jsBlob) Bytes() []byte {
	if buf := b.bytes.Load(); buf != nil {
		return buf.([]byte)
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

func (b *jsBlob) View(start, end int64) (_ blob.Blob, returnedErr error) {
	if start == 0 && end == atomic.LoadInt64(&b.length) {
		return b, nil
	}
	// TODO is it accurate to return a new bytes blob for View? won't be synced with the JSValue
	if buf := b.bytes.Load(); buf != nil {
		return blob.NewBytes(buf.([]byte)).View(start, end)
	}
	defer exception.Catch(&returnedErr)
	return newJSBlob(b.JSValue().Call("subarray", start, end))
}

func (b *jsBlob) Slice(start, end int64) (_ blob.Blob, err error) {
	if buf := b.bytes.Load(); buf != nil {
		return blob.NewBytes(buf.([]byte)).Slice(start, end)
	}
	return newJSBlob(b.JSValue().Call("slice", start, end))
}

func (b *jsBlob) Set(w blob.Blob, off int64) (n int, returnedErr error) {
	// TODO need better consistency if this is to be thread-safe
	if buf := b.bytes.Load(); buf != nil {
		bytes := blob.NewBytes(b.bytes.Load().([]byte))
		_, err := bytes.Set(w, off)
		if err != nil {
			return 0, err
		}
	}

	defer exception.Catch(&returnedErr)

	buf := b.jsValue.Load().(js.Value)
	buf.Call("set", w, off)
	n = w.Len()
	return n, nil
}

func (b *jsBlob) Grow(off int64) (returnedErr error) {
	// TODO need better consistency if this is to be thread-safe
	newLength := atomic.LoadInt64(&b.length) + off
	if buf := b.bytes.Load(); buf != nil {
		biggerBuf := buf.([]byte)
		biggerBuf = append(biggerBuf, make([]byte, off)...)
		b.bytes.Store(biggerBuf)
	}

	defer exception.Catch(&returnedErr)

	buf := b.jsValue.Load().(js.Value)
	biggerBuf := uint8Array.New(newLength)
	biggerBuf.Call("set", buf, 0)
	b.jsValue.Store(biggerBuf)
	atomic.StoreInt64(&b.length, newLength)
	return nil
}

func (b *jsBlob) Truncate(size int64) (returnedErr error) {
	if atomic.LoadInt64(&b.length) < size {
		return
	}
	if buf := b.bytes.Load(); buf != nil {
		bytes := buf.([]byte)
		b.bytes.Store(bytes[:size])
	}

	defer exception.Catch(&returnedErr)

	buf := b.jsValue.Load().(js.Value)
	smallerBuf := buf.Call("slice", 0, size)
	b.jsValue.Store(smallerBuf)
	atomic.StoreInt64(&b.length, size)
	return nil
}
