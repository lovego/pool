package pool

import (
	"context"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestPool(t *testing.T) {
	query, err := url.ParseQuery(`maxOpen=10&maxIdle=5`)
	if err != nil {
		t.Error(err)
	}
	pool, err := New(open, query)
	if err != nil {
		t.Error(err)
	}

	testMaxOpen(pool, t)
	testMaxIdle(pool, t)
	testGetFromIdle(pool, t)
	testGetFromOpen(pool, t)
	testGetFromIdleByReturn(pool, t)
	// testGetFromOpenByReturnOrRelease(pool, t)
}

func testMaxOpen(pool *Pool, t *testing.T) {
	var count countStruct
	var wg sync.WaitGroup
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go doGetWithCount(pool, &count, &wg, t)
	}
	wg.Wait()
	if count.successful != 10 || count.failed != 5 {
		t.Errorf("unexpected count: successful: %d, failed: %d.", count.successful, count.failed)
	}
	if pool.numOpen != 10 {
		t.Errorf("unexpected pool.numOpen: %d", pool.numOpen)
	}
}

type countStruct struct {
	sync.Mutex
	successful int
	failed     int
}

func testMaxIdle(pool *Pool, t *testing.T) {
	if len(pool.idle) != 0 {
		t.Errorf("unexpected idle size: %d", len(pool.idle))
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go doReturn(pool, &wg, t)
	}
	wg.Wait()

	if len(pool.idle) != 5 {
		t.Errorf("unexpected idle size: %d", len(pool.idle))
	}
	if pool.numOpen != 5 {
		t.Errorf("unexpected numOpen: %d", pool.numOpen)
	}
}

func testGetFromIdle(pool *Pool, t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go doGet(pool, 0, &wg, t)
	}
	wg.Wait()

	if len(pool.idle) != 0 {
		t.Errorf("unexpected idle size: %d", len(pool.idle))
	}
	if pool.numOpen != 5 {
		t.Errorf("unexpected numOpen: %d", pool.numOpen)
	}
}

func testGetFromOpen(pool *Pool, t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go doGet(pool, 0, &wg, t)
	}
	wg.Wait()

	if len(pool.idle) != 0 {
		t.Errorf("unexpected idle size: %d", len(pool.idle))
	}
	if pool.numOpen != 10 {
		t.Errorf("unexpected numOpen: %d", pool.numOpen)
	}
}

func testGetFromIdleByReturn(pool *Pool, t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go doGet(pool, time.Second, &wg, t)
	}
	time.Sleep(time.Millisecond)
	for i := 0; i < 7; i++ {
		wg.Add(1)
		go doReturn(pool, &wg, t)
	}
	wg.Wait()

	if len(pool.idle) != 2 {
		t.Errorf("unexpected idle size: %d", len(pool.idle))
	}
	if pool.numOpen != 10 {
		t.Errorf("unexpected numOpen: %d", pool.numOpen)
	}
}

func doGetWithCount(pool *Pool, count *countStruct, wg *sync.WaitGroup, t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Microsecond)
	defer cancel()
	got, err := pool.Get(ctx)

	expect := resource{}
	if got == expect && err == nil {
		count.Lock()
		count.successful++
		count.Unlock()
	} else if got == nil && err == context.DeadlineExceeded {
		count.Lock()
		count.failed++
		count.Unlock()
	} else {
		t.Error(got, err)
	}
	wg.Done()
}

func doGet(pool *Pool, timeout time.Duration, wg *sync.WaitGroup, t *testing.T) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if got, err := pool.Get(ctx); got == nil || err != nil {
		t.Error(got, err)
	}
	wg.Done()
}

func doReturn(pool *Pool, wg *sync.WaitGroup, t *testing.T) {
	if err := pool.Return(resource{}); err != nil {
		t.Error(err)
	}
	wg.Done()
}
