package pool

import (
	"io"
	"sync"
	"time"
)

type Value struct {
	resource io.Closer
	idleFrom time.Time
}

type Pool struct {
	// The resource open func
	openFunc func() (io.Closer, error)
	// Max number of resources can be opened at a given moment. Default value 10 is used if maxOpen <= 0.
	maxOpen int
	// Max number of idle resources to keep. Value of 0 is used if maxIdle < 0.
	maxIdle int
	// Close a resource after how long it's opened. maxLine <= 0 means never close for this reason.
	maxLife time.Duration

	// internal fields
	mu sync.Mutex
	// current number of opened resources, including the idle ones and the busy ones.
	numOpen   int
	resources chan interface{}
}

func New(
	openFunc func() (io.Closer, error), maxOpen, maxIdle int, maxLife time.Duration,
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
		openFunc:  openFunc,
		maxOpen:   maxOpen,
		maxIdle:   maxIdle,
		maxLife:   maxLife,
		resources: make(chan interface{}, maxIdle),
	}
	return p
}
