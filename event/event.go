package event

import "sync"

type Event struct {
	NodeID  string
	TraceID string
	SpanID  string
	Result  any
}

type Bus struct {
	subscribers []chan Event
	mu          sync.Mutex
}

func NewBus() *Bus {
	return &Bus{}
}

func (b *Bus) Subscribe() chan Event {
	ch := make(chan Event, 10)

	b.mu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()

	return ch
}

func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, s := range b.subscribers {
		s <- e
	}
}

func (b *Bus) Replay(handler func(Event)) {
	// Bus doesn't store events, so no replay
}
