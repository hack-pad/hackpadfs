//go:build wasm
// +build wasm

package indexeddb

import (
	"context"
	"testing"

	"github.com/hack-pad/hackpadfs"
	"github.com/hack-pad/hackpadfs/internal/assert"
	"github.com/hack-pad/hackpadfs/keyvalue"
)

func TestGetSet(t *testing.T) {
	t.Parallel()
	store := newStore(makeFS(t).db, Options{})
	txn, err := store.Transaction(keyvalue.TransactionOptions{
		Mode: keyvalue.TransactionReadWrite,
	})
	assert.NoError(t, err)

	var expectOpIDs []keyvalue.OpID
	addOp := func(op keyvalue.OpID) {
		expectOpIDs = append(expectOpIDs, op)
	}
	addOp(txn.GetHandler("bar", keyvalue.OpHandlerFunc(func(_ keyvalue.Transaction, result keyvalue.OpResult) error {
		assert.ErrorIs(t, hackpadfs.ErrNotExist, result.Err)
		return nil
	})))
	setRecord, setData := testFile("baz")
	addOp(txn.Set("bar", setRecord, setData))
	addOp(txn.Get("bar"))
	addOp(txn.SetHandler("biff", nil, nil, keyvalue.OpHandlerFunc(func(_ keyvalue.Transaction, result keyvalue.OpResult) error {
		assert.NoError(t, result.Err)
		return nil
	})))

	ctx := context.Background()
	results, err := txn.Commit(ctx)
	assert.NoError(t, err)

	var resultOpIDs []keyvalue.OpID
	for _, result := range results {
		resultOpIDs = append(resultOpIDs, result.Op)
	}
	assert.Equal(t, expectOpIDs, resultOpIDs)

	expected := []keyvalue.OpResult{
		{Op: 0, Err: hackpadfs.ErrNotExist},
		{Op: 1},
		{Op: 2, Record: setRecord},
		{Op: 3},
	}
	if assert.Equal(t, len(expected), len(results)) {
		for i := range expected {
			expect, result := expected[i], results[i]
			assertEqualOpResult(t, expect, result)
		}
	}
}

func assertEqualOpResult(t *testing.T, expected, actual keyvalue.OpResult) {
	t.Helper()
	t.Log("Op:", expected.Op)
	assert.Equal(t, expected.Op, actual.Op)
	assert.Equal(t, expected.Err, actual.Err)

	if expected.Record == nil {
		assert.Equal(t, nil, actual.Record, "actual record should be nil")
		return
	}
	if !assert.NotEqual(t, nil, actual.Record, "actual record should not be nil") {
		return
	}

	expectData, err := expected.Record.Data()
	if !assert.NoError(t, err, "should get expected data") {
		return
	}
	actualData, err := actual.Record.Data()
	if !assert.NoError(t, err, "should get actual data") {
		return
	}
	assert.Equal(t, string(expectData.Bytes()), string(actualData.Bytes()), "data should be equal")
	assert.Equal(t, expected.Record.Size(), actual.Record.Size(), "size should be equal")
	assert.Equal(t, expected.Record.Mode(), actual.Record.Mode(), "mode should be equal")
	assert.Equal(t, expected.Record.ModTime(), actual.Record.ModTime(), "mod time should be equal")
	assert.Equal(t, expected.Record.Sys(), actual.Record.Sys(), "sys should be equal")
}
