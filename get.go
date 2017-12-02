package pool

import (
	"io"
	"time"
)

func (p *Pool) Get() (io.Closer, error) {
	p.mu.Lock()
	if p.idleTimeout > 0 {
		if err := p.prune(); err != nil {
			return nil, err
		}
	}
	for {
		if len(p.idle) > 0 {
			return p.getIdle(), nil
		}
		if p.maxOpen <= 0 || p.numOpen < p.maxOpen {
			return p.open()
		}
		p.mu.Unlock()
		<-p.notification
		p.mu.Lock()
	}
}

/*
Prune all the resouces that exceeds idleTimeout first in Get.
Why we don't check idleTimeout and prune resource when getIdle? The reasons:
	1. The idle map stores the resouces in time order from oldest to newest. The getIdle has to loop all the timeout resources to get the first non timeout one. so there is no effeciency difference.
	2. Standalone prune only run once in Get, on the other hand getIdle may run many times.
	3. Standalone prune make code clearer, the prune and getIdle becomes two separate things. The getIdle get first resource simply.

p.mu must be locked before call, and is unlocked when encountering error.
*/
func (p *Pool) prune() error {
	for i, n := 0, len(p.idle); i < n; i++ {
		v := p.idle[p.idleStartKey]
		if time.Since(v.idleFrom) < p.idleTimeout {
			return nil
		}
		delete(p.idle, p.idleStartKey)
		p.idleStartKey++
		p.mu.Unlock()
		if err := p.release(v.resource); err != nil {
			return err
		}
		p.mu.Lock()
	}
	if p.idleStartKey > 0 && len(p.idle) == 0 {
		p.idleStartKey = 0
	}
	return nil
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
		p.release(resource)
		return nil, err
	}
}
