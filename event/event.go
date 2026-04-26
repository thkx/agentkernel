package event

import "sync"

type Event struct {
	NodeID  string
	TraceID string
	SpanID  string
	Result  any
}

type Bus struct {
	subscribers map[chan Event]struct{}
	closed      bool
	mu          sync.Mutex
}

func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[chan Event]struct{}),
	}
}

func (b *Bus) Subscribe() chan Event {
	ch := make(chan Event, 10)

	b.mu.Lock()
	if b.closed {
		close(ch)
		b.mu.Unlock()
		return ch
	}
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	return ch
}

func (b *Bus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subscribers[ch]; !ok {
		return
	}
	delete(b.subscribers, ch)
	close(ch)
}

func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	for s := range b.subscribers {
		s <- e
	}
}

func (b *Bus) Replay(handler func(Event)) {
	// Bus doesn't store events, so no replay
}

func (b *Bus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for ch := range b.subscribers {
		close(ch)
		delete(b.subscribers, ch)
	}
	return nil
}
