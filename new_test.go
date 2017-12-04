package pool

import (
	"io"
	"net/url"
	"reflect"
	"testing"
	"time"
)

type resource struct{}

func (r resource) Close() error {
	return nil
}

func open() (io.Closer, error) {
	return resource{}, nil
}

func TestNew(t *testing.T) {
	testCases := []struct {
		input  string
		expect *Pool
	}{
		{``, &Pool{maxIdle: -1}},
		{`maxOpen=10&maxIdle=5&idleTimeout=1h`, &Pool{
			maxOpen: 10, maxIdle: 5, idleTimeout: time.Hour,
		}},
	}
	for _, tc := range testCases {
		query, err := url.ParseQuery(tc.input)
		if err != nil {
			t.Errorf("input: %s, error: %v", tc.input, err)
		}
		got, err := New(open, query)
		if err != nil {
			t.Errorf("input: %s, error: %v", tc.input, err)
		}
		got.openFunc = nil // func is not comparable.
		got.notification = nil
		tc.expect.idle = make(map[uint64]Value)
		if !reflect.DeepEqual(got, tc.expect) {
			t.Errorf("\nexpect: %+v\n   got: %+v", tc.expect, got)
		}
	}
}
