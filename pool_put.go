package pool

import (
	"errors"
	"time"
)

// Put a resource back into the pool.
// The resource must be got from the pool and be `Put` only one time, otherwise an error is returned.
func (p *Pool) Put(r *Resource) error {
	if err := p.removeFromBusy(r); err != nil {
		return err
	}
	select {
	case p.idle <- r:
		r.IdleAt = time.Now()
	default:
		p.close(r)
	}
	return nil
}

// Close a resource got from the pool.
// The resource must be got from the pool and be `Close`d only one time, otherwise an error is returned.
func (p *Pool) Close(r *Resource) error {
	if err := p.removeFromBusy(r); err != nil {
		return err
	}
	p.close(r)
	return nil
}

func (p *Pool) close(r *Resource) {
	p.decrease()
	// network connection often is already closed, but is doesn't matter, so drop the return err.
	r.Closer.Close()
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
