package pool

import (
	"errors"
	"io"
	"time"
)

var ErrNilResource = errors.New(`nil resource`)

// Return returns a resource to the pool.
func (p *Pool) Return(resource io.Closer) error {
	if resource == nil {
		return ErrNilResource
	}
	p.mu.Lock()
	if p.maxIdle >= 0 && len(p.idle) >= p.maxIdle {
		return p.release(resource)
	}
	p.idle[p.idleStartKey+uint64(len(p.idle))] = Value{
		resource: resource, idleFrom: time.Now(),
	}
	p.mu.Unlock()

	// notify waiting Get there is a resource becomes idle now, go to get it.
	select {
	case p.notification <- struct{}{}:
	default:
	}
	return nil
}

// Release releases a resource.
func (p *Pool) Release(resource io.Closer) error {
	if resource == nil {
		return ErrNilResource
	}
	p.mu.Lock()
	return p.release(resource)
}

// p.mu must be locked
func (p *Pool) release(resource io.Closer) error {
	p.numOpen--
	p.mu.Unlock()
	// notify waiting Get there is a resource released, now you can open a new one.
	select {
	case p.notification <- struct{}{}:
	default:
	}
	if resource != nil {
		if err := resource.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Pool) Close() error {
	return nil
}
