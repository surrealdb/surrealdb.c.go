package modulecheck

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestConsumerModuleTidy(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", "off")

	for _, tc := range []struct {
		name    string
		imports string
	}{
		{"library", `_ "github.com/surrealdb/surrealdb.c.go"`},
		{"remote_driver", `_ "github.com/surrealdb/surrealdb.c.go/driver/go"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mod := fmt.Sprintf("module consumer.test/install\n\ngo 1.24\n\nrequire github.com/surrealdb/surrealdb.c.go v0.1.0\n\nreplace github.com/surrealdb/surrealdb.c.go => %q\n", filepath.ToSlash(root))
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o600); err != nil {
				t.Fatal(err)
			}
			source := "package consumer\n\nimport " + tc.imports + "\n"
			if err := os.WriteFile(filepath.Join(dir, "consumer.go"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
			cmd.Dir = dir
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("consumer go mod tidy failed: %v\n%s", err, output)
			}
		})
	}
}
