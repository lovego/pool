package pool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"
)

var testPool, _ = New(openTestResource, 10, 5, time.Minute, time.Hour)

type testResource struct {
}

func (tr testResource) Close() error {
	return nil
}

func openTestResource(ctx context.Context) (io.Closer, error) {
	return testResource{}, nil
}

func ExamplePool() {
	r1, err := testPool.Get(context.Background())
	checkResultAndPrintPoolStatus(r1, err, testPool)

	r2, err := testPool.Get(context.Background())
	checkResultAndPrintPoolStatus(r2, err, testPool)

	if err := testPool.Put(r1); err != nil {
		fmt.Println(err)
	} else {
		printPoolStatus(testPool)
	}

	if err := testPool.Put(r2); err != nil {
		fmt.Println(err)
	} else {
		printPoolStatus(testPool)
	}

	r3, err := testPool.Get(context.Background())
	checkResultAndPrintPoolStatus(r3, err, testPool)

	r4, err := testPool.Get(context.Background())
	checkResultAndPrintPoolStatus(r4, err, testPool)

	if err := testPool.Close(r3); err != nil {
		fmt.Println(err)
	} else {
		printPoolStatus(testPool)
	}

	if err := testPool.Close(r4); err != nil {
		fmt.Println(err)
	} else {
		printPoolStatus(testPool)
	}

	// Output:
	// 1 1 0
	// 2 2 0
	// 2 1 1
	// 2 0 2
	// 2 1 1
	// 2 2 0
	// 1 1 0
	// 0 0 0
}

func ExamplePool_concurrently() {
	var resources = make(chan *Resource, testPool.maxOpen)

	var wg sync.WaitGroup
	for i := 0; i < testPool.maxOpen; i++ {
		wg.Add(1)
		go func() {
			r, err := testPool.Get(context.Background())
			checkResult(r, err)
			resources <- r
			wg.Done()
		}()
	}
	wg.Wait()
	printPoolStatus(testPool)

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Millisecond)
	fmt.Println(testPool.Get(ctx))
	printPoolStatus(testPool)

	go func() {
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Millisecond)
		r, err := testPool.Get(ctx)
		checkResult(r, err)
	}()
	time.Sleep(time.Millisecond) // wait for the previous goroutine to be ready.

	for i := 0; i < testPool.maxOpen; i++ {
		wg.Add(1)
		go func(i int) {
			var fn func(*Resource) error
			if i < 6 || rand.Int()%2 == 0 {
				fn = testPool.Put
			} else {
				fn = testPool.Close
			}
			if err := fn(<-resources); err != nil {
				fmt.Println(err)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	printPoolStatus(testPool)

	// Output:
	// 10 10 0
	// <nil> pool: get resource timeout.
	// 10 10 0
	// 6 1 5
}

func checkResultAndPrintPoolStatus(r *Resource, err error, p *Pool) {
	checkResult(r, err)
	printPoolStatus(p)
}

func checkResult(r *Resource, err error) {
	if r == nil || r.Resource() == nil || !r.openedAt.Before(time.Now()) || err != nil {
		fmt.Println(r, err)
	}
}

func printPoolStatus(p *Pool) {
	fmt.Println(p.opened, len(p.busy), len(p.idle))
}

func ExampleNew() {
	p, err := New(openTestResource, 0, -1, time.Minute, time.Hour)
	fmt.Println(p, err)

	p, _ = New(openTestResource, 10, 11, time.Minute, time.Hour)
	fmt.Println(p.maxOpen, p.maxIdle)

	// Output:
	// <nil> pool: invalid maxOpen: 0
	// 10 10
}

func ExamplePool_Get_error() {
	p, _ := New(func(ctx context.Context) (io.Closer, error) {
		return testResource{}, errors.New("error")
	}, 10, 5, time.Minute, time.Hour)
	fmt.Println(p.Get(context.Background()))
	printPoolStatus(p)

	// Output:
	// <nil> error
	// 0 0 0
}

func ExamplePool_Get_closeIfShould() {
	p, _ := New(openTestResource, 1, 1, 10*time.Millisecond, 30*time.Millisecond)
	r1, err := p.Get(context.Background())
	checkResult(r1, err)
	if err := p.Put(r1); err != nil {
		fmt.Println(err)
	}

	time.Sleep(11 * time.Millisecond)
	printPoolStatus(p)

	r2, err := p.Get(context.Background())
	checkResult(r2, err)
	printPoolStatus(p)

	time.Sleep(31 * time.Millisecond)
	if err := p.Put(r2); err != nil {
		fmt.Println(err)
	}
	printPoolStatus(p)

	// Output:
	// 1 0 1
	// 1 1 0
	// 0 0 0
}

func ExamplePool_errorResource() {
	fmt.Println(testPool.Put(nil))
	fmt.Println(testPool.Put(&Resource{Closer: testResource{}}))

	fmt.Println(testPool.Close(nil))
	fmt.Println(testPool.Close(&Resource{}))

	// Output:
	// pool: the resource is not got from this pool or already been put back or closed.
	// pool: the resource is not got from this pool or already been put back or closed.
	// pool: the resource is not got from this pool or already been put back or closed.
	// pool: the resource is not got from this pool or already been put back or closed.
}

func ExamplePool_decrease() {
	defer func() {
		fmt.Println(strings.HasSuffix(recover().(string), " pool: opened(-1) < idle(0)"))
	}()
	p, _ := New(openTestResource, 1, 1, time.Minute, time.Hour)
	p.decrease()
	// Output: true
}
