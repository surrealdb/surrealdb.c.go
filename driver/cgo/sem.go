package cgo

import "context"

func runWithSem[T any](ctx context.Context, sem chan struct{}, work func() (T, error), cleanup func(T)) (T, error) {
	var zero T

	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return zero, ctx.Err()
	}

	type result struct {
		val T
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, err := work()
		ch <- result{val: v, err: err}
	}()

	select {
	case res := <-ch:
		<-sem
		return res.val, res.err
	case <-ctx.Done():
		go func() {
			res := <-ch
			<-sem
			if cleanup != nil && res.err == nil {
				cleanup(res.val)
			}
		}()
		return zero, ctx.Err()
	}
}
