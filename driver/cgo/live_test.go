package cgo

import (
	"context"
	"testing"
	"time"
)

func openConn(t *testing.T) *Conn {
	t.Helper()
	ctx := context.Background()
	c, err := Connect(ctx, "mem://", DefaultConfig())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := c.Use(ctx, "test", "test"); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if _, err := c.Query(ctx, "DEFINE TABLE person SCHEMALESS", nil); err != nil {
		t.Fatalf("DEFINE TABLE: %v", err)
	}
	return c
}

func waitReader(t *testing.T, lq *LiveHandle, what string) {
	t.Helper()
	select {
	case <-lq.done:
	case <-time.After(5 * time.Second):
		t.Fatalf("stream reader did not exit %s", what)
	}
}

// Close frees the connection, which ends every stream, so readers parked in
// sr_stream_next return and free their streams.
func TestCloseEndsParkedReaders(t *testing.T) {
	c := openConn(t)
	var handles []*LiveHandle
	for i := 0; i < 5; i++ {
		lq, err := c.Live(context.Background(), "person")
		if err != nil {
			t.Fatalf("Live: %v", err)
		}
		handles = append(handles, lq)
	}
	c.Kill(handles[0]) // a killed but still parked reader
	c.Close()
	for _, lq := range handles {
		waitReader(t, lq, "after Close")
	}
}

// After Kill, the reader exits on the live query's next event and frees the
// stream, without delivering the event.
func TestKilledReaderExitsOnNextEvent(t *testing.T) {
	ctx := context.Background()
	c := openConn(t)
	defer c.Close()

	lq, err := c.Live(ctx, "person")
	if err != nil {
		t.Fatalf("Live: %v", err)
	}
	c.Kill(lq)
	if _, ok := <-lq.Ch; ok {
		t.Fatal("notification delivered after Kill")
	}
	if _, err := c.Query(ctx, "CREATE person:wake", nil); err != nil {
		t.Fatalf("CREATE: %v", err)
	}
	waitReader(t, lq, "after the next event")
}

// Repeated Live/Kill/Close cycles must not crash when streams are freed
// after the connection.
func TestLiveCloseCycles(t *testing.T) {
	for i := 0; i < 50; i++ {
		c := openConn(t)
		for j := 0; j < 3; j++ {
			lq, err := c.Live(context.Background(), "person")
			if err != nil {
				t.Fatalf("Live: %v", err)
			}
			if j == 0 {
				c.Kill(lq)
			}
		}
		c.Close()
	}
}
