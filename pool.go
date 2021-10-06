package pool

import (
	"io"
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

type Pool struct {
	// The resource open func
	open func() (io.Closer, error)
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

func New(
	open func() (io.Closer, error),
	maxOpen, maxIdle int,
	maxIdleTime, maxLifeTime time.Duration,
) *Pool {
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
