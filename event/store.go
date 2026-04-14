package event

import (
	"sync"

	"github.com/thkx/agentkernel/types"
)

type EventStore interface {
	Append(event Event) error
	Events() []Event
	Replay(handler func(Event)) error
	Snapshot() map[string]any
}

type InMemoryEventStore struct {
	mu       sync.RWMutex
	events   []Event
	snapshot map[string]any
}

func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events:   make([]Event, 0),
		snapshot: make(map[string]any),
	}
}

func (s *InMemoryEventStore) Append(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	// Update snapshot
	if result, ok := event.Result.(types.Result); ok {
		s.snapshot[string(event.NodeID)] = result.Output
	}
	return nil
}

func (s *InMemoryEventStore) Events() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy
	events := make([]Event, len(s.events))
	copy(events, s.events)
	return events
}

func (s *InMemoryEventStore) Replay(handler func(Event)) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, event := range s.events {
		handler(event)
	}
	return nil
}

func (s *InMemoryEventStore) Snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy
	snap := make(map[string]any)
	for k, v := range s.snapshot {
		snap[k] = v
	}
	return snap
}

// Enhanced Bus with Event Sourcing
type SourcingBus struct {
	*Bus
	store EventStore
}

func NewSourcingBus(store EventStore) *SourcingBus {
	return &SourcingBus{
		Bus:   NewBus(),
		store: store,
	}
}

func (b *SourcingBus) Publish(e Event) {
	b.store.Append(e)
	b.Bus.Publish(e)
}

func (b *SourcingBus) Replay(handler func(Event)) error {
	return b.store.Replay(handler)
}

func (b *SourcingBus) Store() EventStore {
	return b.store
}
