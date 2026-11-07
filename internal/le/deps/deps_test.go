// VALIDATES: the ledger parser, the third-party predicate, the import reader
// and the added-import difference the commit gate and the check apply.
// PREVENTS: a malformed row silently withdrawing an approval, a standard
// library or Ze path judged as third party, and a kept import judged as new.
package deps

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestParseLedger reads rows and comments, and refuses each malformed row
// naming its line rather than skipping it.
func TestParseLedger(t *testing.T) {
	ledger, err := Parse("# comment\n\nexample.org/a\tinternal/x\tin use\nexample.org/b\t.\tat the root\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !ledger.Allows("example.org/a", "internal/x") {
		t.Fatal("a listed row is not allowed")
	}
	if ledger.Allows("example.org/a", "internal/y") {
		t.Fatal("a package is allowed for an importer no row names")
	}
	if !ledger.Allows("example.org/b", ".") {
		t.Fatal("a root importer row is not allowed")
	}
	cases := []struct{ name, text, want string }{
		{"two fields", "example.org/a\tinternal/x\n", "line 1: want three"},
		{"empty reason", "example.org/a\tinternal/x\t \n", "reason is empty"},
		{"unclean importer", "example.org/a\tinternal/x/\tr\n", "not a clean directory"},
		{"escaping importer", "example.org/a\t../x\tr\n", "leaves the checkout"},
		{"absolute importer", "example.org/a\t/x\tr\n", "relative to the checkout"},
		{"duplicate", "example.org/a\tx\tr\nexample.org/a\tx\tq\n", "line 2: repeats the row of line 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.text)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error containing %q, got %v", tc.want, err)
			}
		})
	}
}

// TestLedgerWithRendersSortedOnce replaces a row's reason in place and keeps
// the rendered ledger sorted and parseable.
func TestLedgerWithRendersSortedOnce(t *testing.T) {
	ledger, err := Parse("example.org/b\tx\told\n")
	if err != nil {
		t.Fatal(err)
	}
	ledger = ledger.With(Row{Package: "example.org/a", Importer: "y", Reason: "first"})
	ledger = ledger.With(Row{Package: "example.org/b", Importer: "x", Reason: "new"})
	again, err := Parse(ledger.Render())
	if err != nil {
		t.Fatalf("the rendered ledger does not parse: %v", err)
	}
	want := []Row{{"example.org/a", "y", "first"}, {"example.org/b", "x", "new"}}
	if !slices.Equal(again.Rows(), want) {
		t.Fatalf("rows: got %+v, want %+v", again.Rows(), want)
	}
}

// TestAddedThirdParty keeps only third-party paths the after side adds.
func TestAddedThirdParty(t *testing.T) {
	const module = "github.com/ze-software/ze"
	src := []byte("//go:build linux\n\npackage x\n\nimport (\n\t\"fmt\"\n\tnl \"github.com/vishvananda/netlink\"\n\t\"golang.org/x/sys/unix\"\n\t\"github.com/ze-software/ze/internal/core/env\"\n)\n")
	after, err := Imports("x.go", src)
	if err != nil {
		t.Fatalf("Imports: %v", err)
	}
	added := Added(module, []string{"golang.org/x/sys/unix"}, after)
	if !slices.Equal(added, []string{"github.com/vishvananda/netlink"}) {
		t.Fatalf("added: got %q", added)
	}
	for _, path := range []string{"fmt", "net/http", module, module + "/pkg/ze"} {
		if ThirdParty(module, path) {
			t.Errorf("%s judged third party", path)
		}
	}
	if ThirdParty(module, "github.com/ze-software/zebra") {
		return
	}
	t.Error("a module sharing Ze's prefix without a slash was judged first party")
}

// TestJudgedSkipsOtherModules keeps vendor/, testdata/ and a nested module out
// of the judged population.
func TestJudgedSkipsOtherModules(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "examples", "plugin")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{
		"internal/x/x.go":            true,
		"main.go":                    true,
		"internal/x/x_test.go":       true,
		"vendor/example.org/a/a.go":  false,
		"internal/x/testdata/f.go":   false,
		"examples/plugin/main.go":    false,
		"examples/plugin/sub/sub.go": false,
		"internal/x/README.md":       false,
	}
	for rel, want := range cases {
		got, err := Judged(root, rel)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if got != want {
			t.Errorf("%s: judged %v, want %v", rel, got, want)
		}
	}
}

// TestModuleOf names the longest vendored module a package is in, and none
// for a package no module provides.
func TestModuleOf(t *testing.T) {
	modules := "# golang.org/x/sys v0.1.0\n## explicit\ngolang.org/x/sys/unix\n# golang.org/x/sys/cpu v0.1.0\n"
	if got := ModuleOf(modules, "golang.org/x/sys/unix"); got != "golang.org/x/sys" {
		t.Errorf("unix: got %q", got)
	}
	if got := ModuleOf(modules, "golang.org/x/sys/cpu"); got != "golang.org/x/sys/cpu" {
		t.Errorf("cpu: got %q", got)
	}
	if got := ModuleOf(modules, "example.org/none"); got != "" {
		t.Errorf("unvendored: got %q", got)
	}
}
