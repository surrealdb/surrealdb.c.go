package cgo

import "context"

// run executes work on its own goroutine while holding one slot of the
// connection's semaphore and a read lock on the connection, so Close cannot
// free the C handle until work has returned.
//
// If ctx is cancelled first, run returns ctx.Err() straight away. The C call
// cannot be interrupted, so work keeps running in the background; when it
// finishes, the lock and slot are released and, if work succeeded, cleanup
// is called with its result.
func run[T any](ctx context.Context, c *Conn, work func() (T, error), cleanup func(T)) (T, error) {
	var zero T

	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return zero, ctx.Err()
	}

	release, err := c.acquire()
	if err != nil {
		<-c.sem
		return zero, err
	}

	type result struct {
		val T
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, err := work()
		release()
		<-c.sem
		ch <- result{val: v, err: err}
	}()

	select {
	case res := <-ch:
		return res.val, res.err
	case <-ctx.Done():
		go func() {
			res := <-ch
			if cleanup != nil && res.err == nil {
				cleanup(res.val)
			}
		}()
		return zero, ctx.Err()
	}
}
