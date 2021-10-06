package pool

import (
	"errors"
	"fmt"
)

var errorResource1 = errors.New("pool: the resource is not got from this pool or already been put back.")

// Put a resource back into the pool.
// The resource must be got from the pool and be `Put` only one time, otherwise an error is returned.
func (p *Pool) Put(r *Resource) error {
	if !p.removeFromBusy(r) {
		return errorResource1
	}
	select {
	case p.idle <- r:
	default:
		p.Unlock()
		if err := r.Closer.Close(); err != nil {
			return err
		}
		p.Lock()
		if err := p.decrease(); err != nil {
			return err
		}
	}
	p.Unlock()
	return nil
}

var errorResource2 = errors.New("pool: the resource is not got from this pool or already been closed.")

// Close a resource got from the pool.
// The resource must be got from the pool and be `Close`d only one time, otherwise an error is returned.
func (p *Pool) Close(r *Resource) error {
	if !p.removeFromBusy(r) {
		return errorResource2
	}
	p.Unlock()
	if err := r.Closer.Close(); err != nil {
		return err
	}
	p.Lock()
	if err := p.decrease(); err != nil {
		return err
	}
	p.Unlock()
	return nil
}

func (p *Pool) decrease() error {
	p.opened--
	if p.opened < len(p.idle) {
		// this could happen in duplicate Close.
		return fmt.Errorf("pool: opened(%d) < idle(%d)", p.opened, p.idle)
	}
	return nil
}

func (p *Pool) removeFromBusy(r *Resource) bool {
	p.Lock()
	if _, ok := p.busy[r]; !ok {
		p.Unlock()
		return false
	}
	delete(p.busy, r)
	return true
}
