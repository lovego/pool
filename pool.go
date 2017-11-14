package mailer

import (
	"io"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type Value struct {
	resource io.Closer
	idleFrom time.Time
}

type Pool struct {
	mu sync.Mutex

	idleTimeout  time.Duration
	maxIdle      int // <  0 means unlimited
	idle         map[uint64]*Value
	idleStartKey uint64

	maxOpen int // <= 0 means unlimited
	numOpen int
	opener  func() (io.Closer, error)

	cond *sync.Cond
}

func NewPool(opener func() (io.Closer, error), q url.Values) (*Pool, error) {
	maxOpen, maxIdle, err := poolParams(q)
	if err != nil {
		return nil, err
	}

	p := &Pool{
		maxIdle: maxIdle, idle: make(map[uint64]*Value),
		maxOpen: maxOpen, opener: opener,
	}
	return p, nil
}

func (p *Pool) Get() (io.Closer, error) {
	p.mu.Lock()
	if p.idleTimeout > 0 {
		p.prune()
	}
	for {
		if len(p.idle) > 0 {
			return p.getIdle(), nil
		}
		if p.maxOpen <= 0 || p.numOpen < p.maxOpen {
			return p.open()
		}
		if p.cond == nil {
			p.cond = sync.NewCond(&p.mu)
		}
		p.cond.Wait()
	}
}

func (p *Pool) Put(resource io.Closer) error {
	return nil
}

func (p *Pool) Release(resource io.Closer) error {
	return nil
}

func (p *Pool) Close() error {
	return nil
}

// p.mu must be locked
func (p *Pool) prune() {
	for k, end := p.idleStartKey, p.idleStartKey+len(p.idle); k < end; {
		v := p.idle[k]
		if time.Since(v.idleFrom) < p.idleTimeout {
			return
		}
		delete(p.idle, k)
		p.close(v.resource)
	}
}

// p.mu must be locked
func (p *Pool) getIdle() io.Closer {
	v := p.idle[p.idleStartKey]
	delete(p.idle, p.idleStartKey)
	if len(p.idle) > 0 {
		p.idleStartKey++
	} else {
		p.idleStartKey = 0
	}
	p.mu.Unlock()
	return v.resource
}

// p.mu must be locked
func (p *Pool) open() (io.Closer, error) {
	p.numOpen++ // optimistically
	p.mu.Unlock()
	if resource, err := p.opener(); err == nil {
		return resource, nil
	} else {
		p.mu.Lock()
		p.close(resource)
		p.mu.Unlock()
		return nil, err
	}
}

// p.mu must be locked
func (p *Pool) close(resource io.Closer) {
	p.numOpen--
	if p.cond != nil {
		p.cond.Signal()
	}
	if resource != nil {
		p.mu.Unlock()
		defer p.mu.Lock()
		if err := resource.Close(); err != nil {
			log.Printf("pool Close error: %v", err)
		}
	}
}

func poolParams(q url.Values) (maxOpen, maxIdle int, err error) {
	maxOpen, maxIdle = -1, -1
	if str := q.Get(`maxOpen`); str != `` {
		if maxOpen, err = strconv.Atoi(str); err != nil {
			return
		}
	}
	if str := q.Get(`maxIdle`); str != `` {
		if maxIdle, err = strconv.Atoi(str); err != nil {
			return
		}
	}
	return
}
