package surrealdb

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestCloseWhileInUse closes a connection while other goroutines are still
// running queries on it. Close must wait for in-flight calls instead of
// freeing the C handle underneath them, and later calls must fail with
// ErrClosed. Run with -race.
func TestCloseWhileInUse(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	mustQuery(t, db, ctx, "DEFINE TABLE item SCHEMALESS", nil)

	const workers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for {
				_, err := db.Query(ctx, "CREATE item SET n = rand::int(); SELECT count() FROM item GROUP ALL", nil)
				if errors.Is(err, ErrClosed) {
					return
				}
				if err != nil {
					errs <- err
					return
				}
			}
		}()
	}

	close(start)
	time.Sleep(50 * time.Millisecond)
	db.Close()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("workers did not stop after Close")
	}

	close(errs)
	for err := range errs {
		t.Errorf("query failed with an error other than ErrClosed: %v", err)
	}
}

// TestUseAfterClose checks that every operation on a closed connection
// returns ErrClosed rather than passing a freed handle to the C library.
func TestUseAfterClose(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	db.Close()
	db.Close() // a second Close is a no-op

	if err := db.Use(ctx, "ns", "db"); !errors.Is(err, ErrClosed) {
		t.Errorf("Use after Close: got %v, want ErrClosed", err)
	}
	if _, err := db.Query(ctx, "RETURN 1", nil); !errors.Is(err, ErrClosed) {
		t.Errorf("Query after Close: got %v, want ErrClosed", err)
	}
	if _, err := db.Query(ctx, "RETURN $x", map[string]any{"x": 1}); !errors.Is(err, ErrClosed) {
		t.Errorf("Query with vars after Close: got %v, want ErrClosed", err)
	}
	if _, _, _, err := db.Live(ctx, "person"); !errors.Is(err, ErrClosed) {
		t.Errorf("Live after Close: got %v, want ErrClosed", err)
	}
}

// TestConcurrentClose calls Close from several goroutines at once.
func TestConcurrentClose(t *testing.T) {
	db := openTestDB(t)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db.Close()
		}()
	}
	wg.Wait()
}

// A caller may reuse its variables as soon as Query returns, including when
// it returns early because the context was cancelled. Run with -race.
func TestQueryVarsReusableAfterCancel(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	vars := map[string]any{"n": 0}
	for i := 0; i < 200; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		go cancel()
		_, _ = db.Query(ctx, "RETURN $n", vars)
		vars["n"] = i
		cancel()
	}
}

func TestSignInParamsReusableAfterCancel(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	nested := map[string]any{"n": 0}
	creds := map[string]any{"NS": "test_ns", "DB": "test_db", "AC": "none", "extra": nested}
	for i := 0; i < 200; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		go cancel()
		_, _ = db.SignIn(ctx, creds)
		nested["n"] = i
		cancel()
	}
}
