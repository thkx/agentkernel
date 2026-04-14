package worker

import (
	"sync"
)

type Pool struct {
	workers    int
	tasks      chan func()
	wg         sync.WaitGroup
	shutdownCh chan struct{}
}

func NewPool(workers int) *Pool {
	p := &Pool{
		workers:    workers,
		tasks:      make(chan func(), workers*2),
		shutdownCh: make(chan struct{}),
	}

	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	return p
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case task := <-p.tasks:
			task()
		case <-p.shutdownCh:
			return
		}
	}
}

func (p *Pool) Submit(task func()) {
	select {
	case p.tasks <- task:
	case <-p.shutdownCh:
		return
	}
}

func (p *Pool) Close() {
	close(p.shutdownCh)
	p.wg.Wait()
}
