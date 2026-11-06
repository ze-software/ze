// Design: docs/architecture/plugin/feature-gates.md -- generated gate rows share committed inputs.
package gatedtable

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// TestGatedRowsKeepTheCommittedManifest drives Derive over a committed plugin,
// then changes both its working-tree registration and the manifest's gate.
func TestGatedRowsKeepTheCommittedManifest(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt": "ze_example internal/plugins/example\n",
		"cmd/ze/main.go": "package main\nimport _ \"github.com/ze-software/ze/internal/plugins/example\"\nfunc main() {}\n",
		"internal/component/plugin/registry/registry.go": "package registry\ntype Registration struct { Name, Description string; RunEngine func(); ConfigRoots []string }\nfunc Register(Registration) error { return nil }\n",
		"internal/plugins/example/register_linux.go": "package example\nimport r \"github.com/ze-software/ze/internal/component/plugin/registry\"\nfunc init() { _ = r.Register(r.Registration{Name: \"example\", Description: \"committed\", ConfigRoots: []string{\"example-root\"}, RunEngine: func(){}}) }\n",
	}
	for name, body := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "--initial-branch=main"}, {"add", "."},
		{"-c", "user.name=Gate Fixture", "-c", "user.email=gate@example.invalid", "commit", "-m", "committed gates"},
	} {
		command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
		if raw, err := command.CombinedOutput(); err != nil {
			t.Fatalf("fixture git %v: %v: %s", args, err, raw)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "feature-gates.txt"), []byte("ze_dirty internal/plugins/example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal/plugins/example/register_linux.go"), []byte("package example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rows, err := Derive(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "example" || !slices.Equal(rows[0].Tags, []string{"ze_example"}) || !slices.Equal(rows[0].ConfigRoots, []string{"example-root"}) {
		t.Fatalf("gate table mixed committed and dirty inputs: %+v", rows)
	}
}
