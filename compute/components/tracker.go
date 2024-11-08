// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package components

import "sync"

// set represents a set of coordinator or worker ids.
type set = map[string]struct{}

// Tracker collects coordinator and worker instances that are currently present
// in the distributed system. All its methods are safe for concurrent use by
// multiple goroutines.
type Tracker struct {
	mu           sync.RWMutex // protects following fields
	coordinators set          // ids of alive coordinators
	workers      set          // ids of alive workers
}

// NewTracker creates and returns a tracker with no alive components.
//
// All its methods are safe for concurrent use by multiple goroutines.
func NewTracker() *Tracker {
	return &Tracker{sync.RWMutex{}, make(set), make(set)}
}

// TryJoin registers a coordinator or worker with the given id. If the component
// is not yet tracked, true is returned; otherwise false.
func (t *Tracker) TryJoin(role ComponentRole, id string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	var set set
	switch role {
	case RoleCoordinator:
		set = t.coordinators
	case RoleWorker:
		set = t.workers
	}
	if _, ok := set[id]; ok {
		return false
	}
	set[id] = struct{}{}
	return true
}

// Leave deregisters a leaving coordinator or worker with the given id.
func (t *Tracker) Leave(role ComponentRole, id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch role {
	case RoleCoordinator:
		delete(t.coordinators, id)
	case RoleWorker:
		delete(t.workers, id)
	}
}

// Count gets the number of coordinators and workers currently alive.
func (t *Tracker) Count() (cc, cw int) {
	t.mu.RLock()

	defer t.mu.RUnlock()
	return len(t.coordinators), len(t.workers)
}
