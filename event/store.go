package event

import (
	"fmt"
	"os"
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

var _ EventStore = (*InMemoryEventStore)(nil)

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
	if event.Kind == "" || event.Kind == types.EventKindExecution {
		switch payload := event.Result.(type) {
		case types.Result:
			s.snapshot[event.NodeID] = payload.Output
		case types.ExecutionEvent:
			if payload.Result != nil {
				s.snapshot[event.NodeID] = payload.Result.Output
			}
		}
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
	mu     sync.Mutex
	store  EventStore
	subs   map[chan types.Event]struct{}
	closed bool
}

func NewSourcingBus(store EventStore) *SourcingBus {
	return &SourcingBus{
		store: store,
		subs:  make(map[chan types.Event]struct{}),
	}
}

func (b *SourcingBus) Publish(e types.Event) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	subs := make([]chan types.Event, 0, len(b.subs))
	for ch := range b.subs {
		subs = append(subs, ch)
	}
	b.store.Append(e)
	b.mu.Unlock()

	for _, s := range subs {
		select {
		case s <- e:
		default:
			// Log dropped event due to full channel
			fmt.Fprintf(os.Stderr, "event bus: dropped event %v due to full subscriber channel\n", e)
		}
	}
}

func (b *SourcingBus) Replay(handler func(types.Event)) error {
	return b.store.Replay(handler)
}

func (b *SourcingBus) Subscribe() chan types.Event {
	ch := make(chan types.Event, 10)
	b.mu.Lock()
	if b.closed {
		close(ch)
		b.mu.Unlock()
		return ch
	}
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *SourcingBus) Unsubscribe(ch chan types.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subs[ch]; !ok {
		return
	}
	delete(b.subs, ch)
	close(ch)
}

func (b *SourcingBus) Store() types.EventStore {
	return b.store
}

func (b *SourcingBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for ch := range b.subs {
		close(ch)
		delete(b.subs, ch)
	}
	return nil
}
