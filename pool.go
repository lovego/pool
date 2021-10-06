package pool

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

type Resource struct {
	io.Closer
	idleAt   time.Time
	openedAt time.Time
}

func (r *Resource) Resource() io.Closer {
	return r.Closer
}

type openFunc func(context.Context) (io.Closer, error)

type Pool struct {
	// The resource open func
	open openFunc
	// Max number of resources can be opened at a given moment.
	// Default value 10 is used if maxOpen <= 0.
	maxOpen int
	// Max number of idle resources to keep.
	// Value of 0 is used if maxIdle < 0.
	maxIdle int
	// Close a resource after how long it has been idle.
	// maxIdleTime <= 0 means never close a resource due to it's idle time.
	maxIdleTime time.Duration
	// Close a resource after how long it has been opened.
	// maxLifeTime <= 0 means never close a resource due to it's life time.
	maxLifeTime time.Duration

	// current number of opened resources, including the idle ones and the busy ones.
	opened int
	// idle resources
	idle chan *Resource
	// busy resources
	busy map[*Resource]struct{}

	sync.Mutex
}

func New(open openFunc, maxOpen, maxIdle int, maxIdleTime, maxLifeTime time.Duration) *Pool {
	if maxOpen <= 0 {
		maxOpen = 10
	}
	if maxIdle < 0 {
		maxIdle = 0
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}

	p := &Pool{
		open:        open,
		maxOpen:     maxOpen,
		maxIdle:     maxIdle,
		maxIdleTime: maxIdleTime,
		maxLifeTime: maxLifeTime,
		idle:        make(chan *Resource, maxIdle),
		busy:        make(map[*Resource]struct{}, maxOpen),
	}
	return p
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

func (p *Pool) decrease() error {
	p.Lock()
	defer p.Unlock()
	p.opened--
	if p.opened < len(p.idle) {
		// this could happen in duplicate Close.
		return fmt.Errorf("pool: opened(%d) < idle(%d)", p.opened, p.idle)
	}
	return nil
}

func (p *Pool) closeIfShould(r *Resource) bool {
	if p.exceedMaxIdleTime(r) || p.exceedMaxLifeTime(r) {
		if err := p.close(r); err != nil {
			log.Println("pool: close resource:", err)
		}
		return true
	}
	return false
}

func (p *Pool) exceedMaxIdleTime(r *Resource) bool {
	return p.maxIdleTime > 0 && time.Since(r.idleAt) > p.maxIdleTime
}

func (p *Pool) exceedMaxLifeTime(r *Resource) bool {
	return p.maxLifeTime > 0 && time.Since(r.openedAt) > p.maxLifeTime
}
