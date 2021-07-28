package pathlock

import "sync"

type Lock struct {
	pathLocks sync.Map
}

func New() *Lock {
	return &Lock{}
}

func (l *Lock) Lock(path string) {
	var newMu sync.Mutex
	muInterface, _ := l.pathLocks.LoadOrStore(path, &newMu)
	mu := muInterface.(*sync.Mutex)
	mu.Lock()
}

func (l *Lock) Unlock(path string) {
	muInterface, _ := l.pathLocks.Load(path)
	mu := muInterface.(*sync.Mutex)
	mu.Unlock()
}
