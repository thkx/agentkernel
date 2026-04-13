package event

import "sync"

type Event struct {
	TaskID string
	Result any
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
