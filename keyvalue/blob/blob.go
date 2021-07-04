package blob

// Blob is a binary blob of data that can support platform-optimized mutations for better performance.
type Blob interface {
	// Bytes returns the byte slice equivalent to the data in this Blob.
	Bytes() []byte
	// Len returns the number of bytes contained in this blob.
	// Can be used to avoid unnecessary conversions and allocations from len(Bytes()).
	Len() int
}

// ViewBlob is a Blob that can return a view into the same underlying data.
// Mutating the returned Blob also mutates the original.
type ViewBlob interface {
	Blob
	View(start, end int64) (Blob, error)
}

// SliceBlob is a Blob that can return a copy of the data between start and end.
type SliceBlob interface {
	Blob
	Slice(start, end int64) (Blob, error)
}

// SetBlob is a Blob that can copy 'dest' into itself starting at the given offset into this Blob.
// Use View() on 'dest' to control the maximum that is copied into this Blob.
type SetBlob interface {
	Blob
	Set(dest Blob, offset int64) (n int, err error)
}

// GrowBlob is a Blob that can increase it's size by allocating offset bytes at the end.
type GrowBlob interface {
	Blob
	Grow(offset int64) error
}

// TruncateBlob is a Blob that can cut off bytes from the end until it is 'size' bytes long.
type TruncateBlob interface {
	Blob
	Truncate(size int64) error
}

func View(b Blob, start, end int64) (Blob, error) {
	if b, ok := b.(ViewBlob); ok {
		return b.View(start, end)
	}
	return NewBytes(b.Bytes()).View(start, end)
}

func Slice(b Blob, start, end int64) (Blob, error) {
	if b, ok := b.(SliceBlob); ok {
		return b.Slice(start, end)
	}
	return NewBytes(b.Bytes()).Slice(start, end)
}

func Set(dest Blob, src Blob, offset int64) (n int, err error) {
	if dest, ok := dest.(SetBlob); ok {
		return dest.Set(src, offset)
	}
	return NewBytes(dest.Bytes()).Set(src, offset)
}

func Grow(b Blob, offset int64) error {
	if b, ok := b.(GrowBlob); ok {
		return b.Grow(offset)
	}
	return NewBytes(b.Bytes()).Grow(offset)
}

func Truncate(b Blob, size int64) error {
	if b, ok := b.(TruncateBlob); ok {
		return b.Truncate(size)
	}
	return NewBytes(b.Bytes()).Truncate(size)
}
