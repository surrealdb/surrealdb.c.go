package surrealdb_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	surrealdb "github.com/surrealdb/surrealdb.c.go"

	_ "github.com/surrealdb/surrealdb.c.go/driver/go"
)

func TestDrivers(t *testing.T) {
	wsURL := startSurrealDBContainer(t)

	for _, tc := range []struct {
		name     string
		endpoint string
	}{
		{"mem://", "mem://"},
		{"ws://", wsURL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testDriverQuery(t, tc.endpoint)
		})
	}
}

func startSurrealDBContainer(t *testing.T) string {
	t.Helper()

	if os.Getenv("SURREALDB_SKIP_DOCKER") != "" {
		t.Skip("SURREALDB_SKIP_DOCKER is set, skipping Docker-dependent test")
	}

	containerName := fmt.Sprintf("surrealdb-test-%d", time.Now().UnixNano())

	imageTag := os.Getenv("SURREALDB_IMAGE_TAG")
	if imageTag == "" {
		imageTag = "v2"
	}
	image := "surrealdb/surrealdb:" + imageTag

	runCmd := exec.Command("docker", "run", "-d",
		"-p", "0:8000",
		"--name", containerName,
		image,
		"start", "--unauthenticated",
	)
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker run failed: %v\n%s", err, out)
	}

	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	portCmd := exec.Command("docker", "port", containerName, "8000")
	portOut, err := portCmd.Output()
	if err != nil {
		t.Fatalf("docker port: %v", err)
	}

	hostPort := strings.TrimSpace(string(portOut))
	if idx := strings.LastIndex(hostPort, ":"); idx >= 0 {
		hostPort = hostPort[idx+1:]
	}

	wsURL := fmt.Sprintf("ws://localhost:%s", hostPort)
	waitForReady(t, wsURL)
	return wsURL
}

func waitForReady(t *testing.T, wsURL string) {
	t.Helper()
	ctx := context.Background()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		connCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
		db, err := surrealdb.Open(connCtx, wsURL)
		cancel()
		if err == nil {
			db.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("SurrealDB at %s did not become ready within 15s", wsURL)
}

func testDriverQuery(t *testing.T, endpoint string) {
	t.Helper()
	ctx := context.Background()

	db, err := surrealdb.Open(ctx, endpoint)
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.Use(ctx, "test_ns", "test_db"))

	_, err = db.Query(ctx, `DEFINE TABLE person SCHEMALESS`, nil)
	require.NoError(t, err)

	_, err = db.Query(ctx, `CREATE person:alice SET name = "Alice", age = 30`, nil)
	require.NoError(t, err)

	// Test with string ID (auto-converted from RecordID)
	type PersonStr struct {
		ID   string `cbor:"id"`
		Name string `cbor:"name"`
		Age  int64  `cbor:"age"`
	}

	results, err := surrealdb.Query[PersonStr](ctx, db, "SELECT * FROM person", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NoError(t, results[0].Err())

	got := results[0].Values()
	require.Len(t, got, 1)
	assert.Equal(t, PersonStr{ID: "person:alice", Name: "Alice", Age: 30}, got[0])

	// Test with RecordID[string] field
	type PersonRID struct {
		ID   surrealdb.RecordID[string] `cbor:"id"`
		Name string                     `cbor:"name"`
		Age  int64                      `cbor:"age"`
	}

	ridResults, err := surrealdb.Query[PersonRID](ctx, db, "SELECT * FROM person", nil)
	require.NoError(t, err)
	require.Len(t, ridResults, 1)
	require.NoError(t, ridResults[0].Err())

	ridGot := ridResults[0].Values()
	require.Len(t, ridGot, 1)
	assert.Equal(t, "person", ridGot[0].ID.Table)
	assert.Equal(t, "alice", ridGot[0].ID.ID)
	assert.Equal(t, "Alice", ridGot[0].Name)

	// Test RecordID[string] round-trip: pass as var, get it back
	rid := surrealdb.NewRecordID("person", "alice")
	selectResults, err := db.Query(ctx, "SELECT * FROM $rid", map[string]any{"rid": rid})
	require.NoError(t, err)
	require.Len(t, selectResults, 1)
	require.NoError(t, selectResults[0].Err)

	rows, ok := selectResults[0].Value.([]any)
	require.True(t, ok, "expected []any, got %T", selectResults[0].Value)
	require.Len(t, rows, 1)

	rec, ok := rows[0].(map[string]any)
	require.True(t, ok, "expected map[string]any, got %T", rows[0])
	gotRID, ok := rec["id"].(surrealdb.RecordID[any])
	require.True(t, ok, "expected RecordID[any] for id field, got %T", rec["id"])
	assert.Equal(t, "person", gotRID.Table)
	assert.Equal(t, "alice", fmt.Sprintf("%v", gotRID.ID))

	// Let must keep driver types as SurrealDB values on every backend.
	require.NoError(t, db.Let(ctx, "let_rid", surrealdb.NewRecordID("person", "alice")))
	require.NoError(t, db.Let(ctx, "let_line", surrealdb.GeometryLine{{Longitude: 1, Latitude: 2}, {Longitude: 3, Latitude: 4}}))
	for _, q := range []string{"RETURN type::is_record($let_rid)", "RETURN type::is_geometry($let_line)"} {
		res, err := db.Query(ctx, q, nil)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NoError(t, res[0].Err)
		v := res[0].Value
		if arr, ok := v.([]any); ok && len(arr) == 1 {
			v = arr[0]
		}
		assert.Equal(t, true, v, q)
	}

	// Test with RecordID[int]: numeric record IDs
	_, err = db.Query(ctx, `CREATE person:123 SET name = "Numeric", age = 99`, nil)
	require.NoError(t, err)

	type PersonIntID struct {
		ID   surrealdb.RecordID[int] `cbor:"id"`
		Name string                  `cbor:"name"`
		Age  int64                   `cbor:"age"`
	}

	intResults, err := surrealdb.Query[PersonIntID](ctx, db, "SELECT * FROM person:123", nil)
	require.NoError(t, err)
	require.Len(t, intResults, 1)
	require.NoError(t, intResults[0].Err())

	intGot := intResults[0].Values()
	require.Len(t, intGot, 1)
	assert.Equal(t, "person", intGot[0].ID.Table)
	assert.Equal(t, 123, intGot[0].ID.ID)
	assert.Equal(t, "Numeric", intGot[0].Name)

	// Test RecordID[int] round-trip: pass as var, get it back.
	// The C API (sr_value_thing) only accepts string IDs, so numeric
	// IDs are stringified and won't match person:123 (numeric).
	// See docs/upstream-fix-ideas.md for the proposed upstream fix.
	if strings.HasPrefix(endpoint, "ws://") || strings.HasPrefix(endpoint, "wss://") ||
		strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		intRID := surrealdb.NewRecordID("person", 123)
		intSelectResults, err := db.Query(ctx, "SELECT * FROM $rid", map[string]any{"rid": intRID})
		require.NoError(t, err)
		require.Len(t, intSelectResults, 1)
		require.NoError(t, intSelectResults[0].Err)

		intRows, ok := intSelectResults[0].Value.([]any)
		require.True(t, ok, "expected []any, got %T", intSelectResults[0].Value)
		require.Len(t, intRows, 1)

		intRec, ok := intRows[0].(map[string]any)
		require.True(t, ok, "expected map[string]any, got %T", intRows[0])
		assert.Equal(t, "Numeric", intRec["name"])
	}
}
