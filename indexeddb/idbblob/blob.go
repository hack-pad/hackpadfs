//go:build wasm
// +build wasm

package idbblob

import (
	"errors"
	"fmt"
	"sync/atomic"
	"syscall/js"

	"github.com/hack-pad/hackpadfs/internal/exception"
	"github.com/hack-pad/hackpadfs/internal/jswrapper"
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
	} = &Blob{}
)

// Blob is a blob.Blob for JS environments, optimized for reading and writing to a Uint8Array without en/decoding back and forth.
type Blob struct {
	bytes   atomic.Value // *blob.Bytes
	jsValue atomic.Value // js.Value
	length  int64
}

// New creates a Blob wrapping the given JS Uint8Array buffer.
func New(buf js.Value) (_ *Blob, returnedErr error) {
	defer exception.Catch(&returnedErr)
	if !buf.Truthy() {
		return FromBlob(blob.NewBytes(nil)), nil
	}
	if !buf.InstanceOf(uint8Array) {
		return nil, fmt.Errorf("invalid JS array type: %v", buf)
	}
	return newBlob(buf), nil
}

// NewLength creates a zero-value Blob with 'length' bytes.
func NewLength(length int) (_ *Blob, err error) {
	defer exception.Catch(&err)
	jsBuf := uint8Array.New(length)
	return newBlob(jsBuf), nil
}

func newBlob(buf js.Value) *Blob {
	b := &Blob{}
	b.jsValue.Store(buf)
	atomic.StoreInt64(&b.length, int64(buf.Length()))
	return b
}

// FromBlob creates a Blob from the given blob.Blob, either wrapping the JS value or copying the bytes if incompatible.
func FromBlob(b blob.Blob) *Blob {
	if b, ok := b.(jswrapper.Wrapper); ok {
		return newBlob(b.JSValue())
	}
	buf := b.Bytes()
	jsBuf := uint8Array.New(len(buf))
	js.CopyBytesToJS(jsBuf, buf)
	return newBlob(jsBuf)
}

func (b *Blob) currentBytes() *blob.Bytes {
	buf := b.bytes.Load()
	if buf != nil {
		return buf.(*blob.Bytes)
	}
	return nil
}

// Bytes implememts blob.Blob
func (b *Blob) Bytes() []byte {
	if buf := b.currentBytes(); buf != nil {
		return buf.Bytes()
	}
	jsBuf := b.jsValue.Load().(js.Value)
	buf := make([]byte, jsBuf.Length())
	js.CopyBytesToGo(buf, jsBuf)
	b.bytes.Store(blob.NewBytes(buf))
	return buf
}

// JSValue implements jswrapper.Wrapper
func (b *Blob) JSValue() js.Value {
	return b.jsValue.Load().(js.Value)
}

// Len implements blob.Blob
func (b *Blob) Len() int {
	return int(atomic.LoadInt64(&b.length))
}

func catchErr(fn func() error) (returnedErr error) {
	defer exception.Catch(&returnedErr)
	return fn()
}

// View implements blob.ViewBlob
func (b *Blob) View(start, end int64) (blob.Blob, error) {
	if start == 0 && end == atomic.LoadInt64(&b.length) {
		return b, nil
	}

	var newBlob *Blob
	err := catchErr(func() error {
		var err error
		newBlob, err = New(b.JSValue().Call("subarray", start, end))
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

// Slice implements blob.SliceBlob
func (b *Blob) Slice(start, end int64) (blob.Blob, error) {
	if start < 0 || start > int64(b.Len()) {
		return nil, fmt.Errorf("Start index out of bounds: %d", start)
	}
	if end < 0 || end > int64(b.Len()) {
		return nil, fmt.Errorf("End index out of bounds: %d", end)
	}

	newBlob, err := New(b.JSValue().Call("slice", start, end))
	if err != nil {
		return nil, err
	}
	if buf := b.currentBytes(); buf != nil {
		newBytes, err := buf.Slice(start, end)
		if err != nil {
			panic(err)
		}
		newBlob.bytes.Store(newBytes)
	}
	return newBlob, nil
}

// Set implements blob.SetBlob
func (b *Blob) Set(src blob.Blob, destStart int64) (n int, err error) {
	if destStart < 0 {
		return 0, errors.New("negative offset")
	}
	if destStart >= int64(b.Len()) && destStart == 0 && src.Len() > 0 {
		return 0, fmt.Errorf("Offset out of bounds: %d", destStart)
	}

	err = catchErr(func() error {
		b.JSValue().Call("set", FromBlob(src), destStart)
		return nil
	})
	if err != nil {
		return 0, err
	}
	n = src.Len()

	if buf := b.currentBytes(); buf != nil {
		_, err := buf.Set(src, destStart)
		if err != nil {
			panic(err)
		}
	}
	return n, nil
}

// Grow implements blob.GrowBlob
func (b *Blob) Grow(off int64) error {
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

// Truncate implements TruncateBlob
func (b *Blob) Truncate(size int64) error {
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
