package platform

import (
	"fmt"
	"sort"
	"sync"
)

// Store is a concurrency-safe in-memory store. It stands in for the real
// backend (CRM, billing, ...) until those integrations are wired up.
type Store[T any] struct {
	mu     sync.RWMutex
	prefix string
	seq    int
	items  map[string]T
}

func NewStore[T any](idPrefix string) *Store[T] {
	return &Store[T]{prefix: idPrefix, items: map[string]T{}}
}

// NextID returns a new unique ID such as "LEAD-1001".
func (s *Store[T]) NextID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return fmt.Sprintf("%s-%d", s.prefix, 1000+s.seq)
}

func (s *Store[T]) Put(id string, v T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[id] = v
}

func (s *Store[T]) Get(id string) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.items[id]
	return v, ok
}

// Update applies fn to the item under the write lock.
func (s *Store[T]) Update(id string, fn func(*T) error) (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.items[id]
	if !ok {
		var zero T
		return zero, fmt.Errorf("%s not found", id)
	}
	if err := fn(&v); err != nil {
		return v, err
	}
	s.items[id] = v
	return v, nil
}

// List returns items matching keep (all if nil), ordered by ID.
func (s *Store[T]) List(keep func(T) bool) []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.items))
	for id := range s.items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := []T{}
	for _, id := range ids {
		if v := s.items[id]; keep == nil || keep(v) {
			out = append(out, v)
		}
	}
	return out
}
