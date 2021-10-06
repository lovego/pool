package pool

import (
	"context"
	"errors"
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
	select {
	case r := <-p.idle:
		return r, nil
	default:
	}

	if r, err := p.tryOpen(); err != nil {
		return nil, err
	} else if r != nil {
		return r, nil
	}

	select {
	case r := <-p.idle:
		return r, nil
	case <-ctx.Done():
		return nil, errorTimeout
	}
}

func (p *Pool) tryOpen() (*Resource, error) {
	if !p.tryIncrease() {
		return nil, nil
	}
	resource, err := p.open()
	if resource != nil && err == nil {
		return &Resource{Closer: resource, openedAt: time.Now()}, nil
	}
	if resource != nil {
		if err := resource.Close(); err != nil {
			return nil, err
		}
	}
	p.Lock()
	defer p.Unlock()
	if err := p.decrease(); err != nil {
		return nil, err
	}
	return nil, err
}

func (p *Pool) tryIncrease() bool {
	p.Lock()
	defer p.Unlock()
	if p.opened >= p.maxOpen {
		return false
	}
	p.opened++
	return true
}
