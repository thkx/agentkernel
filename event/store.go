package event

import (
	"sync"

	"github.com/thkx/agentkernel/types"
)

type EventStore interface {
	Append(event types.Event) error
	Events() []types.Event
	Replay(handler func(types.Event)) error
	Snapshot() map[string]any
	AppendTimeline(entry types.ExecutionTimelineEntry) error
	Timeline() []types.ExecutionTimelineEntry
}

type InMemoryEventStore struct {
	mu       sync.RWMutex
	events   []types.Event
	snapshot map[string]any
	timeline []types.ExecutionTimelineEntry
}

func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events:   make([]types.Event, 0),
		snapshot: make(map[string]any),
		timeline: make([]types.ExecutionTimelineEntry, 0),
	}
}

func (s *InMemoryEventStore) Append(event types.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	// Update snapshot
	if result, ok := event.Result.(types.Result); ok {
		s.snapshot[event.NodeID] = result.Output
	}
	return nil
}

func (s *InMemoryEventStore) Events() []types.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy
	events := make([]types.Event, len(s.events))
	copy(events, s.events)
	return events
}

func (s *InMemoryEventStore) Replay(handler func(types.Event)) error {
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

func (s *InMemoryEventStore) AppendTimeline(entry types.ExecutionTimelineEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timeline = append(s.timeline, entry)
	return nil
}

func (s *InMemoryEventStore) Timeline() []types.ExecutionTimelineEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := make([]types.ExecutionTimelineEntry, len(s.timeline))
	copy(entries, s.timeline)
	return entries
}

// Enhanced Bus with Event Sourcing
type SourcingBus struct {
	mu    sync.Mutex
	store EventStore
	subs  []chan types.Event
}

func NewSourcingBus(store EventStore) *SourcingBus {
	return &SourcingBus{
		store: store,
		subs:  make([]chan types.Event, 0),
	}
}

func (b *SourcingBus) Publish(e types.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.store.Append(e)

	for _, s := range b.subs {
		select {
		case s <- e:
		default:
			// log non-blocking send, drop event if channel is full
		}
	}
}

func (b *SourcingBus) Replay(handler func(types.Event)) error {
	return b.store.Replay(handler)
}

func (b *SourcingBus) Subscribe() chan types.Event {
	ch := make(chan types.Event, 10)
	b.mu.Lock()
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	return ch
}

func (b *SourcingBus) Store() types.EventStore {
	return b.store
}
