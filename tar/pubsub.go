package tar

import (
	"context"
	"sync"
)

type pubsub struct {
	mu          sync.RWMutex
	subscribers map[string][]context.CancelFunc
	visited     map[string]bool
	ctx         context.Context
}

// newPubsub creates a new pubsub that unblocks all calls to Wait when ctx is canceled
func newPubsub(ctx context.Context) *pubsub {
	return &pubsub{
		ctx:         ctx,
		subscribers: make(map[string][]context.CancelFunc),
		visited:     make(map[string]bool),
	}
}

func (ps *pubsub) Emit(key string) {
	ps.mu.RLock()
	visited := ps.visited[key]
	ps.mu.RUnlock()
	if visited {
		return
	}
	ps.mu.Lock()
	ps.visited[key] = true
	funcs := ps.subscribers[key]
	ps.subscribers[key] = nil
	ps.mu.Unlock()
	for _, cancel := range funcs {
		cancel()
	}
}

func (ps *pubsub) Wait(key string) {
	select {
	case <-ps.ctx.Done():
		return
	default:
	}

	ps.mu.Lock()
	if ps.visited[key] {
		ps.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ps.ctx)
	ps.subscribers[key] = append(ps.subscribers[key], cancel)
	ps.mu.Unlock()

	select {
	case <-ps.ctx.Done():
	case <-ctx.Done():
	}
}
