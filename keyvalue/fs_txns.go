package keyvalue

type transactionOnly struct {
	store Store
}

func newFSTransactioner(store Store) *transactionOnly {
	return &transactionOnly{store}
}

func (t *transactionOnly) Transaction() (Transaction, error) {
	return TransactionOrSerial(t.store)
}
