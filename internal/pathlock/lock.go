// Package pathlock contains Mutex, which locks and unlocks using file paths as keys.
package pathlock

import "sync"

// Mutex is a path-based locker. Lock a given path for exclusive access to that path.
type Mutex struct {
	pathLocks sync.Map
}

// New returns a new Mutex
func New() *Mutex {
	return &Mutex{}
}

// Lock blocks access to 'path' until Unlock is called
func (l *Mutex) Lock(path string) {
	var newMu sync.Mutex
	muInterface, _ := l.pathLocks.LoadOrStore(path, &newMu)
	mu := muInterface.(*sync.Mutex)
	mu.Lock()
}

// Unlock unblocks access to 'path'
func (l *Mutex) Unlock(path string) {
	muInterface, _ := l.pathLocks.Load(path)
	mu := muInterface.(*sync.Mutex)
	mu.Unlock()
}
