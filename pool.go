package pool

import (
	"io"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type Value struct {
	resource io.Closer
	idleFrom time.Time
}

type Pool struct {
	mu sync.Mutex

	// max number of resouces can be opened at a given moment.
	maxOpen int // <= 0 means unlimited
	// current number of opened resources, including the idle ones and the busy ones.
	numOpen int
	// the resource open func
	openFunc func() (io.Closer, error)

	// max number of idle resources to keep. < 0 means unlimited.
	maxIdle int
	// the idle resources map.
	idle         map[uint64]Value
	idleStartKey uint64
	// how long after a resource keep idle, it will be released. <=0 means a resouece won't be released for idle time reason.
	idleTimeout time.Duration

	// notifcation when a resource becomes idle or released.
	notification chan struct{}
}

func New(open func() (io.Closer, error), q url.Values) (*Pool, error) {
	maxOpen, maxIdle, idleTimeout, err := parseParams(q)
	if err != nil {
		return nil, err
	}

	p := &Pool{
		maxOpen: maxOpen, openFunc: open,
		maxIdle: maxIdle, idle: make(map[uint64]Value), idleTimeout: idleTimeout,
		notification: make(chan struct{}),
	}
	return p, nil
}

func parseParams(q url.Values) (maxOpen, maxIdle int, idleTimeout time.Duration, err error) {
	if str := q.Get(`maxOpen`); str != `` {
		if maxOpen, err = strconv.Atoi(str); err != nil {
			return
		}
	}
	if str := q.Get(`maxIdle`); str != `` {
		if maxIdle, err = strconv.Atoi(str); err != nil {
			return
		}
	} else {
		maxIdle = -1
	}
	if str := q.Get(`idleTimeout`); str != `` {
		if idleTimeout, err = time.ParseDuration(str); err != nil {
			return
		}
	}
	return
}
