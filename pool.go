package pool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type Resource struct {
	io.Closer
	IdleAt   time.Time
	OpenedAt time.Time
}

func (r *Resource) Resource() io.Closer {
	return r.Closer
}

// the func to open a resource
type OpenFunc func(context.Context) (io.Closer, error)

// the func to check if a resource is usable.
type UsableFunc func(context.Context, *Resource) bool

type Pool struct {
	// The func to open a resource
	open OpenFunc
	// The func to check if a resource is usable when got from the pool.
	usable UsableFunc

	// Max number of resources can be opened at a given moment.
	maxOpen int
	// Max number of idle resources to keep.
	// maxIdle <= 0 means never keep any idle resources.
	maxIdle int

	// current number of opened resources, including the idle ones and the busy ones.
	opened int
	// idle resources
	idle chan *Resource
	// busy resources
	busy map[*Resource]struct{}

	sync.Mutex
}

func New(open OpenFunc, usable UsableFunc, maxOpen, maxIdle int) (*Pool, error) {
	if open == nil {
		return nil, errors.New("pool: nil open func")
	}
	if maxOpen <= 0 {
		return nil, fmt.Errorf("pool: invalid maxOpen: %d", maxOpen)
	}
	if maxIdle < 0 {
		maxIdle = 0
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}

	return &Pool{
		open:    open,
		usable:  usable,
		maxOpen: maxOpen,
		maxIdle: maxIdle,
		idle:    make(chan *Resource, maxIdle),
		busy:    make(map[*Resource]struct{}, maxOpen),
	}, nil
}

// New2 return a pool by params from url.Values.
// Params default value: maxOpen => 10;  maxIdle => 1;
func New2(open OpenFunc, usable UsableFunc, values url.Values) (*Pool, error) {
	var maxOpen, maxIdle int = 10, 1

	if s := values.Get(`maxOpen`); s != `` {
		if i, err := strconv.Atoi(s); err != nil {
			return nil, err
		} else {
			maxOpen = i
		}
	}

	if s := values.Get(`maxIdle`); s != `` {
		if i, err := strconv.Atoi(s); err != nil {
			return nil, err
		} else {
			maxIdle = i
		}
	}

	return New(open, usable, maxOpen, maxIdle)
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

func (p *Pool) decrease() {
	p.Lock()
	defer p.Unlock()
	p.opened--
	// this should not happen.
	if p.opened < len(p.idle) {
		panic(fmt.Sprintf("%s pool: opened(%d) < idle(%d)",
			time.Now().Format(time.RFC3339), p.opened, len(p.idle),
		))
	}
}

func (p *Pool) closeIfShould(ctx context.Context, r *Resource) bool {
	if p.usable != nil && !p.usable(ctx, r) {
		p.close(r)
		return true
	}
	return false
}
