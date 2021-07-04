package blob

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

type Bytes struct {
	bytes []byte
}

// NewBytes returns a Blob that wraps the given byte slice.
// Mutations to this Blob are reflected in the original slice.
func NewBytes(buf []byte) *Bytes {
	return &Bytes{buf}
}

// NewBytesLength returns a new Bytes with the given length of zero-byte data.
func NewBytesLength(length int) *Bytes {
	return NewBytes(make([]byte, length))
}

func (b *Bytes) Bytes() []byte {
	return b.bytes
}

func (b *Bytes) Len() int {
	return len(b.bytes)
}

func (b *Bytes) View(start, end int64) (Blob, error) {
	return NewBytes(b.bytes[start:end]), nil
}

func (b *Bytes) Slice(start, end int64) (Blob, error) {
	buf := make([]byte, end-start)
	copy(buf, b.bytes)
	return NewBytes(buf), nil
}

func (b *Bytes) Set(dest Blob, srcStart int64) (n int, err error) {
	n = copy(b.bytes[srcStart:], dest.Bytes())
	return n, nil
}

func (b *Bytes) Grow(offset int64) error {
	b.bytes = append(b.bytes, make([]byte, offset)...)
	return nil
}

func (b *Bytes) Truncate(size int64) error {
	if int64(len(b.bytes)) < size {
		return nil
	}
	b.bytes = b.bytes[:size]
	return nil
}
