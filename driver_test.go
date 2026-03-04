package surrealdb_test

import (
	"context"
	"fmt"
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

	containerName := fmt.Sprintf("surrealdb-test-%d", time.Now().UnixNano())

	runCmd := exec.Command("docker", "run", "-d",
		"-p", "0:8000",
		"--name", containerName,
		"surrealdb/surrealdb:v2",
		"start", "--unauthenticated",
	)
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Skipf("docker not available or image not pulled: %v\n%s", err, out)
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

	type Person struct {
		ID   string `cbor:"id"`
		Name string `cbor:"name"`
		Age  int64  `cbor:"age"`
	}

	results, err := surrealdb.Query[Person](ctx, db, "SELECT * FROM person", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NoError(t, results[0].Err())

	got := results[0].Values()
	require.Len(t, got, 1)
	assert.Equal(t, Person{ID: "person:alice", Name: "Alice", Age: 30}, got[0])
}
