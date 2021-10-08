package pool

import (
	"context"
	"errors"
	"log"
	"time"
)

func (p *Pool) Get(ctx context.Context) (*Resource, error) {
	r, err := p.get(ctx)
	if r != nil {
		p.Lock()
		p.busy[r] = struct{}{}
		p.Unlock()
	}
	return r, err
}

var errorTimeout = errors.New("pool: get resource timeout.")

func (p *Pool) get(ctx context.Context) (*Resource, error) {
	if r := p.getIdle(); r != nil {
		return r, nil
	}

	if r, err := p.tryOpen(ctx); err != nil {
		return nil, err
	} else if r != nil {
		return r, nil
	}

	return p.waitIdle(ctx)
}

func (p *Pool) getIdle() *Resource {
loop:
	select {
	case r := <-p.idle:
		if p.closeIfShould(r) {
			goto loop
		}
		return r
	default:
		return nil
	}
}

func (p *Pool) waitIdle(ctx context.Context) (*Resource, error) {
loop:
	select {
	case r := <-p.idle:
		if p.closeIfShould(r) {
			goto loop
		}
		return r, nil
	case <-ctx.Done():
		return nil, errorTimeout
	}
}

func (p *Pool) tryOpen(ctx context.Context) (*Resource, error) {
	if !p.tryIncrease() {
		return nil, nil
	}
	resource, err := p.open(ctx)
	if resource != nil && err == nil {
		return &Resource{Closer: resource, openedAt: time.Now()}, nil
	}
	if resource != nil {
		if err := resource.Close(); err != nil {
			log.Println("pool: close resource:", err)
		}
	}
	p.decrease()
	return nil, err
}