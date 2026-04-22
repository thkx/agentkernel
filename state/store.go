package state

import "sync"

type Store struct {
	data map[string]any
	mu   sync.RWMutex
}

func NewStore() *Store {
	return &Store{
		data: map[string]any{},
	}
}

func NewStoreFromMap(data map[string]any) *Store {
	store := NewStore()
	for k, v := range data {
		store.data[k] = CloneValue(v)
	}
	return store
}

func (s *Store) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = CloneValue(value)
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

func (s *Store) Get(key string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return CloneValue(s.data[key])
}

func (s *Store) Snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]any, len(s.data))
	for k, v := range s.data {
		snapshot[k] = CloneValue(v)
	}
	return snapshot
}
