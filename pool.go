package pool

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"strconv"
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
	maxOpen int
	// Max number of idle resources to keep.
	// maxIdle <= 0 means never keep any idle resources.
	maxIdle int
	// When `Get`, close a resource if it has been idle for `maxIdleTime`.
	// maxIdleTime <= 0 means never close a resource due to it's idle time.
	maxIdleTime time.Duration
	// When `Get`/`Put`, close a resource if it has been opened for `maxLifeTime`.
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

func New(open openFunc, maxOpen, maxIdle int, maxIdleTime, maxLifeTime time.Duration) (*Pool, error) {
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
		open:        open,
		maxOpen:     maxOpen,
		maxIdle:     maxIdle,
		maxIdleTime: maxIdleTime,
		maxLifeTime: maxLifeTime,
		idle:        make(chan *Resource, maxIdle),
		busy:        make(map[*Resource]struct{}, maxOpen),
	}, nil
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
	return r != nil && p.maxIdleTime > 0 && time.Since(r.idleAt) > p.maxIdleTime
}

func (p *Pool) exceedMaxLifeTime(r *Resource) bool {
	return r != nil && p.maxLifeTime > 0 && time.Since(r.openedAt) > p.maxLifeTime
}

// New2 return a pool by get params from url.Values.
// Params default value:
// maxOpen => 10;  maxIdle => 1; maxIdleTime => 10 Minute; maxLifeTime => 1 Hour.
func New2(open openFunc, values url.Values) (*Pool, error) {
	var maxOpen, maxIdle int = 10, 1
	var maxIdleTime, maxLifeTime time.Duration = 10 * time.Minute, time.Hour

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

	if s := values.Get(`maxIdleTime`); s != `` {
		if d, err := time.ParseDuration(s); err != nil {
			return nil, err
		} else {
			maxIdleTime = d
		}
	}

	if s := values.Get(`maxLifeTime`); s != `` {
		if d, err := time.ParseDuration(s); err != nil {
			return nil, err
		} else {
			maxLifeTime = d
		}
	}

	return New(open, maxOpen, maxIdle, maxIdleTime, maxLifeTime)
}
