package keyvalue

import (
	"context"
	"sync"
	"sync/atomic"
)

type TransactionStore interface {
	Transaction() (Transaction, error)
}

type OpID int64

type OpResult struct {
	Op     OpID
	Record FileRecord
	Err    error
}

type OpHandler func(txn Transaction, result OpResult) error

type Transaction interface {
	Get(path string) OpID
	GetHandler(path string, handler OpHandler) OpID
	Set(path string, src FileRecord) OpID
	SetHandler(path string, src FileRecord, handler OpHandler) OpID
	Commit(ctx context.Context) ([]OpResult, error)
	Abort() error
}

type unsafeSerialTransaction struct {
	ctx       context.Context
	abort     context.CancelFunc
	nextOp    OpID
	store     Store
	resultsMu sync.Mutex
	results   map[OpID]OpResult
}

// TransactionOrSerial attempts to produce a Transaction from 'store'.
// If unsupported, returns an unsafe transaction instead, whic runs each action serially without transactional safety.
//
// This is used in FS to attempt transactions whenever possible.
// Since some Stores don't need transactions, they aren't required to implement TransactionStore.
func TransactionOrSerial(store Store) (Transaction, error) {
	if store, ok := store.(TransactionStore); ok {
		return store.Transaction()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &unsafeSerialTransaction{
		ctx:     ctx,
		abort:   cancel,
		store:   store,
		results: make(map[OpID]OpResult),
	}, nil
}

func (u *unsafeSerialTransaction) newOp() OpID {
	nextOp := atomic.AddInt64((*int64)(&u.nextOp), 1)
	return OpID(nextOp - 1)
}

func (u *unsafeSerialTransaction) setResult(op OpID, result OpResult) {
	u.resultsMu.Lock()
	u.results[op] = result
	u.resultsMu.Unlock()
}

func abortErr(ctx, extraCtx context.Context) error {
	if extraCtx == nil {
		extraCtx = context.Background()
	}
	select {
	case <-extraCtx.Done():
		return extraCtx.Err()
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (u *unsafeSerialTransaction) Get(path string) OpID {
	return u.GetHandler(path, func(txn Transaction, result OpResult) error {
		return nil
	})
}

func (u *unsafeSerialTransaction) GetHandler(path string, handler OpHandler) OpID {
	op := u.newOp()
	if err := abortErr(u.ctx, nil); err != nil {
		u.setResult(op, OpResult{Op: op, Err: err})
		return op
	}

	record, err := u.store.Get(path)
	result := OpResult{Op: op, Record: record, Err: err}
	err = handler(u, result)
	if result.Err == nil && err != nil {
		result.Err = err
	}
	u.setResult(op, result)
	return op
}

func (u *unsafeSerialTransaction) Set(path string, src FileRecord) OpID {
	return u.SetHandler(path, src, func(txn Transaction, result OpResult) error {
		return nil
	})
}

func (u *unsafeSerialTransaction) SetHandler(path string, src FileRecord, handler OpHandler) OpID {
	op := u.newOp()
	if err := abortErr(u.ctx, nil); err != nil {
		u.setResult(op, OpResult{Op: op, Err: err})
		return op
	}

	err := u.store.Set(path, src)
	result := OpResult{Op: op, Err: err}
	err = handler(u, result)
	if result.Err == nil && err != nil {
		result.Err = err
	}
	u.setResult(op, result)
	return op
}

func (u *unsafeSerialTransaction) Commit(ctx context.Context) ([]OpResult, error) {
	if err := abortErr(u.ctx, ctx); err != nil {
		return nil, err
	}
	u.abort()
	opCount := atomic.LoadInt64((*int64)(&u.nextOp))
	results := make([]OpResult, opCount)
	u.resultsMu.Lock()
	for op, result := range u.results {
		results[op] = result
	}
	u.resultsMu.Unlock()
	return results, nil
}

func (u *unsafeSerialTransaction) Abort() error {
	u.abort()
	return nil
}
