package pool

import (
	"errors"
	"time"
)

// Put a resource back into the pool.
// The resource must be got from the pool and be `Put` only one time, otherwise an error is returned.
func (p *Pool) Put(r *Resource) error {
	if p.exceedMaxLifeTime(r) {
		return p.Close(r)
	}
	if err := p.removeFromBusy(r); err != nil {
		return err
	}
	select {
	case p.idle <- r:
		r.idleAt = time.Now()
		return nil
	default:
		return p.close(r)
	}
}

// Close a resource got from the pool.
// The resource must be got from the pool and be `Close`d only one time, otherwise an error is returned.
func (p *Pool) Close(r *Resource) error {
	if err := p.removeFromBusy(r); err != nil {
		return err
	}
	return p.close(r)
}

func (p *Pool) close(r *Resource) error {
	p.decrease()
	return r.Closer.Close()
}

var errorResource = errors.New("pool: the resource is not got from this pool or already been put back or closed.")

func (p *Pool) removeFromBusy(r *Resource) error {
	if r == nil || r.Closer == nil {
		return errorResource
	}
	p.Lock()
	defer p.Unlock()
	if _, ok := p.busy[r]; !ok {
		return errorResource
	}
	delete(p.busy, r)
	return nil
}
