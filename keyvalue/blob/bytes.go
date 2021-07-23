package blob

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

var (
	// ensure Bytes conforms to these interfaces:
	_ interface {
		Blob
		ViewBlob
		SliceBlob
		SetBlob
		GrowBlob
		TruncateBlob
	} = &Bytes{}
)

// Bytes is a Blob that wraps a byte slice.
type Bytes struct {
	bytes  []byte
	length int64
	mu     sync.Mutex
}

// NewBytes returns a Blob that wraps the given byte slice.
// Mutations to this Blob are reflected in the original slice.
func NewBytes(buf []byte) *Bytes {
	return &Bytes{bytes: buf, length: int64(len(buf))}
}

// NewBytesLength returns a new Bytes with the given length of zero-byte data.
func NewBytesLength(length int) *Bytes {
	return NewBytes(make([]byte, length))
}

// Bytes implements Blob.
func (b *Bytes) Bytes() []byte {
	return b.bytes
}

// Len implements Blob.
func (b *Bytes) Len() int {
	return int(atomic.LoadInt64(&b.length))
}

// View implements Blob.
func (b *Bytes) View(start, end int64) (Blob, error) {
	if start < 0 || start > int64(b.Len()) {
		return nil, fmt.Errorf("Start index out of bounds: %d", start)
	}
	if end < 0 || end > int64(b.Len()) {
		return nil, fmt.Errorf("End index out of bounds: %d", end)
	}
	return NewBytes(b.bytes[start:end]), nil
}

// Slice implements Blob.
func (b *Bytes) Slice(start, end int64) (Blob, error) {
	if start < 0 || start > int64(b.Len()) {
		return nil, fmt.Errorf("Start index out of bounds: %d", start)
	}
	if end < 0 || end > int64(b.Len()) {
		return nil, fmt.Errorf("End index out of bounds: %d", end)
	}
	buf := make([]byte, end-start)
	b.mu.Lock()
	copy(buf, b.bytes)
	b.mu.Unlock()
	return NewBytes(buf), nil
}

// Set implements Blob.
func (b *Bytes) Set(dest Blob, srcStart int64) (n int, err error) {
	if srcStart < 0 {
		return 0, errors.New("negative offset")
	}
	if srcStart >= int64(b.Len()) && srcStart == 0 && dest.Len() > 0 {
		return 0, fmt.Errorf("Offset out of bounds: %d", srcStart)
	}
	b.mu.Lock()
	n = copy(b.bytes[srcStart:], dest.Bytes())
	b.mu.Unlock()
	return n, nil
}

// Grow implements Blob.
func (b *Bytes) Grow(offset int64) error {
	b.mu.Lock()
	b.bytes = append(b.bytes, make([]byte, offset)...)
	atomic.StoreInt64(&b.length, int64(len(b.bytes)))
	b.mu.Unlock()
	return nil
}

// Truncate implements Blob.
func (b *Bytes) Truncate(size int64) error {
	if int64(b.Len()) < size {
		return nil
	}
	b.mu.Lock()
	if int64(b.Len()) >= size {
		b.bytes = b.bytes[:size]
		atomic.StoreInt64(&b.length, int64(len(b.bytes)))
	}
	b.mu.Unlock()
	return nil
}
