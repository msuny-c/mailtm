package mailtm

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct {
	now   atomic.Value
	after chan time.Duration
}

func newFakeClock(t0 time.Time) *fakeClock {
	fc := &fakeClock{after: make(chan time.Duration, 10)}
	fc.now.Store(t0)
	return fc
}
func (f *fakeClock) Now() time.Time        { return f.now.Load().(time.Time) }
func (f *fakeClock) Sleep(d time.Duration) {}
func (f *fakeClock) After(d time.Duration) <-chan time.Time {
	f.after <- d
	ch := make(chan time.Time, 1)
	ch <- f.Now()
	return ch
}

func TestTokenBucket_TakeAndCancel(t *testing.T) {
	fc := newFakeClock(time.Unix(0, 0))
	tb := NewTokenBucket(1, 1, fc)
	ctx, cancel := context.WithCancel(context.Background())
	if err := tb.take(ctx, 1); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = tb.take(ctx, 1)
	}()
	// observe that After was requested
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for After")
	case <-func() <-chan struct{} {
		ch := make(chan struct{})
		go func() { <-fc.after; close(ch) }()
		return ch
	}():
	}
	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("take did not return on cancel")
	}
}
