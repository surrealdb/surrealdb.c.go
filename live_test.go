package surrealdb

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// within fails the test if f does not return before the timeout.
func within(t *testing.T, d time.Duration, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		f()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not return within %s", what, d)
	}
}

func expectClosed(t *testing.T, ch <-chan Notification) {
	t.Helper()
	select {
	case n, ok := <-ch:
		if ok {
			t.Fatalf("expected closed channel, got notification %+v", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("channel not closed after kill")
	}
}

func expectNotification(t *testing.T, ch <-chan Notification, action string) Notification {
	t.Helper()
	select {
	case n, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before notification")
		}
		if n.Action != action {
			t.Fatalf("expected %s, got %s", action, n.Action)
		}
		return n
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", action)
	}
	return Notification{}
}

// Stopping one live query must not write anything to the table, so other
// live queries on it see no events. (The old stop created and deleted a
// "poison pill" record.)
func TestLiveKillWritesNothing(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	mustQuery(t, db, ctx, "DEFINE TABLE person SCHEMALESS", nil)

	_, chA, killA, err := db.Live(ctx, "person")
	if err != nil {
		t.Fatalf("Live A: %v", err)
	}
	_, chB, killB, err := db.Live(ctx, "person")
	if err != nil {
		t.Fatalf("Live B: %v", err)
	}
	defer killB()

	within(t, 5*time.Second, "kill", killA)
	expectClosed(t, chA)

	mustQuery(t, db, ctx, `CREATE person:marker`, nil)
	n := expectNotification(t, chB, "CREATE")
	if rec, _ := n.Data.(map[string]any); fmt.Sprint(rec["id"]) != "person:marker" {
		t.Fatalf("live query B saw an unexpected record before the marker: %v", n.Data)
	}

	res := mustQuery(t, db, ctx, "SELECT * FROM person", nil)
	if rows, _ := res[0].Value.([]any); len(rows) != 1 {
		t.Fatalf("expected only the marker record, got %v", res[0].Value)
	}
}

// The old stop blocked forever when the poison pill could not be written,
// e.g. on a schemafull table with a required field.
func TestLiveKillOnSchemafullTable(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	mustQuery(t, db, ctx, "DEFINE TABLE strict SCHEMAFULL; DEFINE FIELD name ON strict TYPE string", nil)

	_, ch, kill, err := db.Live(ctx, "strict")
	if err != nil {
		t.Fatalf("Live: %v", err)
	}
	within(t, 5*time.Second, "kill", kill)
	expectClosed(t, ch)
}

// The old stop built an invalid poison pill record id from a record id
// resource and blocked forever.
func TestLiveKillOnRecordID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	mustQuery(t, db, ctx, "DEFINE TABLE person SCHEMALESS; CREATE person:alice", nil)

	_, ch, kill, err := db.Live(ctx, "person:alice")
	if err != nil {
		t.Fatalf("Live: %v", err)
	}
	within(t, 5*time.Second, "kill", kill)
	expectClosed(t, ch)
}

// A consumer that stops reading must not stop Kill from returning.
func TestLiveKillWithFullChannel(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	mustQuery(t, db, ctx, "DEFINE TABLE item SCHEMALESS", nil)

	_, ch, kill, err := db.Live(ctx, "item")
	if err != nil {
		t.Fatalf("Live: %v", err)
	}
	// More than the channel buffer, without reading.
	mustQuery(t, db, ctx, "FOR $i IN 1..200 { CREATE item }", nil)
	time.Sleep(200 * time.Millisecond)

	within(t, 5*time.Second, "kill", kill)
	for range ch {
	}
}

// Close must stop live queries cleanly and close their channels.
func TestLiveClose(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	mustQuery(t, db, ctx, "DEFINE TABLE person SCHEMALESS", nil)

	var chans []<-chan Notification
	for _, res := range []string{"person", "person", "person:bob"} {
		_, ch, _, err := db.Live(ctx, res)
		if err != nil {
			t.Fatalf("Live(%s): %v", res, err)
		}
		chans = append(chans, ch)
	}

	within(t, 5*time.Second, "Close", db.Close)
	for _, ch := range chans {
		expectClosed(t, ch)
	}
}

// The id returned by Live is the server's live query id, so it matches
// notifications and can be passed to Kill.
func TestLiveIDAndKillByID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	mustQuery(t, db, ctx, "DEFINE TABLE person SCHEMALESS", nil)

	id, ch, _, err := db.Live(ctx, "person")
	if err != nil {
		t.Fatalf("Live: %v", err)
	}

	mustQuery(t, db, ctx, `CREATE person:carol`, nil)
	n := expectNotification(t, ch, "CREATE")
	if n.QueryID != id {
		t.Errorf("Live returned id %s, notification has %s", id, n.QueryID)
	}

	within(t, 5*time.Second, "Kill", func() {
		if err := db.Kill(id); err != nil {
			t.Errorf("Kill(%s): %v", id, err)
		}
	})
	expectClosed(t, ch)
}
