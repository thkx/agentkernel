package worker

import (
	"sync"
)

type Pool struct {
	workers int
	tasks   chan func()
	wg      sync.WaitGroup
}

func NewPool(workers int) *Pool {
	p := &Pool{
		workers: workers,
		tasks:   make(chan func(), workers*2),
	}

	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	return p
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for task := range p.tasks {
		task()
	}
}

func (p *Pool) Submit(task func()) {
	p.tasks <- task
}

func (p *Pool) Close() {
	close(p.tasks)
	p.wg.Wait()
}
