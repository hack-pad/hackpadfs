package tar

import (
	"sync/atomic"
)

// bufferPool maintains a collection of byte buffers with a maximum size.
// Used to control upper-bound memory usage. It's safe for concurrent use.
type bufferPool struct {
	count   int64
	size    uint64
	buffers chan *buffer
}

type buffer struct {
	Data []byte
	pool *bufferPool
}

func newBufferPool(bufferSize, maxBuffers uint64) *bufferPool {
	if maxBuffers == 0 {
		maxBuffers = 1
	}
	p := &bufferPool{
		size:    bufferSize,
		buffers: make(chan *buffer, maxBuffers),
	}
	p.addBuffer() // start with 1 buffer, ready to go
	return p
}

func (p *bufferPool) addBuffer() {
	for {
		count := atomic.LoadInt64(&p.count)
		if int(count) == cap(p.buffers) {
			return // already at max buffers, no-op
		}
		if atomic.CompareAndSwapInt64(&p.count, count, count+1) {
			break // successfully provisioned slot for new buffer
		}
	}
	buf := &buffer{
		Data: make([]byte, p.size),
		pool: p,
	}
	p.buffers <- buf
}

// Wait acquires and returns a buffer. Be sure to call buffer.Done() to return it to the pool.
func (p *bufferPool) Wait() *buffer {
	select {
	case buf := <-p.buffers:
		return buf
	default:
		p.addBuffer()
		// may not always get the new buffer, but looping could allocate more buffers far too quickly
		return <-p.buffers
	}
}

// Done returns this buffer to the pool
func (b *buffer) Done() {
	b.pool.buffers <- b
}
