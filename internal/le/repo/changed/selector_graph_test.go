// Design: docs/architecture/testing/verify-freshness-scope.md -- graph failures widen.
// Goal: preserve Go diagnostics and all import edges without unrelated VCS work.
// Method: decode malformed records and drive the real Go command over isolated modules.
package repochanged

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestPackageGraphRejectsIncompleteRecords keeps a partial listing from narrowing scope.
func TestPackageGraphRejectsIncompleteRecords(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name   string
		record packageRecord
		want   string
	}{
		{name: "pattern-error", record: packageRecord{ImportPath: "./...", Error: &packageError{Err: "module diagnostic\nwith a tab\there"}}, want: "module diagnostic\nwith a tab\there"},
		{name: "missing-directory", record: packageRecord{ImportPath: "example.com/missing"}, want: "no absolute package directory"},
		{name: "missing-import-path", record: packageRecord{Dir: root}, want: "without an import path"},
		{name: "incomplete", record: packageRecord{ImportPath: "example.com/partial", Dir: root, Incomplete: true}, want: "incomplete package"},
		{name: "dependency-error", record: packageRecord{ImportPath: "example.com/partial", Dir: root, DepsErrors: []packageError{{Pos: "source.go:3", Err: "dependency is unavailable"}}}, want: "source.go:3: dependency is unavailable"},
		{name: "outside-root", record: packageRecord{ImportPath: "example.com/outside", Dir: filepath.Dir(root)}, want: "outside the repository"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			valid, err := json.Marshal(packageRecord{ImportPath: "example.com/valid", Dir: root})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(test.record)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := parsePackageGraph(root, string(valid)+"\n"+string(raw))
			if err == nil {
				t.Fatalf("accepted an incomplete graph: %+v", graph)
			}
			if graph != nil {
				t.Fatal("returned the earlier partial graph beside an error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("diagnostic = %q, want %q", err, test.want)
			}
		})
	}
	for _, listing := range []string{"", "{", "{} trailing junk"} {
		if graph, err := parsePackageGraph(root, listing); err == nil {
			t.Fatalf("accepted malformed listing %q: %+v", listing, graph)
		}
	}
}

// TestSelectorWidensWithTheUnderlyingGoDiagnostic exercises successful go list -e failure records.
func TestSelectorWidensWithTheUnderlyingGoDiagnostic(t *testing.T) {
	root := writeScopeFixture(t)
	writeFile(t, root, "core/broken.go", "package another\n")
	paths := scopePathsFile(t, "core/core.go")
	stdout, stderr, code := runScopeSelector(t, scopeSelectorAction(t), root, "--print=both", "--paths-from="+paths)
	if code != 0 {
		t.Fatalf("safe widening exited %d: %s", code, stderr)
	}
	packages, tags, err := splitScopeSections(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(packages, []string{everyPackage}) {
		t.Fatalf("failed graph narrowed to %v", packages)
	}
	if !slices.Equal(tags, []string{"ze_bgp", "ze_ssh"}) {
		t.Fatalf("failed graph dropped feature tags: %v", tags)
	}
	if !strings.Contains(stderr, "found packages") {
		t.Fatalf("lost the underlying Go diagnostic: %s", stderr)
	}
	if strings.Contains(stderr, "Rel:") {
		t.Fatalf("replaced a package error with a path error: %s", stderr)
	}
}

// TestPackageGraphOwnsItsModuleEnvironment excludes ambient workspace, flags and VCS scans.
func TestPackageGraphOwnsItsModuleEnvironment(t *testing.T) {
	root := canonicalRoot(writeScopeFixture(t))
	writeFile(t, root, "cmd/probe/main.go", "package main\nimport _ \"example.com/fixture/core\"\nfunc main() {}\n")
	writeFile(t, root, "mid/mid_test.go", "package mid\nimport _ \"example.com/fixture/ssh\"\n")
	writeFile(t, root, "mid/external_test.go", "package mid_test\nimport _ \"example.com/fixture/bgp\"\n")
	writeFile(t, root, ".git", "gitdir: /no/such/graph-fixture-repository\n")
	goenv := filepath.Join(t.TempDir(), "goenv")
	writeFile(t, filepath.Dir(goenv), filepath.Base(goenv), "GOFLAGS=-invalid-graph-fixture-flag\n")
	t.Setenv("GOENV", goenv)
	t.Setenv("GOFLAGS", "-invalid-graph-fixture-flag")
	t.Setenv("GOWORK", filepath.Join(root, "absent.work"))
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	graph, err := loadPackageGraph(root, []string{"ze_ssh", "ze_bgp"})
	if err != nil {
		t.Fatalf("load isolated graph: %v", err)
	}
	for dependency, importer := range map[string]string{
		"example.com/fixture/core": "example.com/fixture/mid",
		"example.com/fixture/ssh":  "example.com/fixture/mid",
		"example.com/fixture/bgp":  "example.com/fixture/mid",
	} {
		if !slices.Contains(graph.importers[dependency], importer) {
			t.Errorf("lost direct or test edge %s <- %s: %v", dependency, importer, graph.importers[dependency])
		}
	}
	if !slices.Contains(graph.importers["example.com/fixture/ssh"], "example.com/fixture/hub") {
		t.Error("lost the tag-gated importer")
	}
}

// TestPackageGraphReportsDeadline uses an expired context, not a sleep or a wedged process.
func TestPackageGraphReportsDeadline(t *testing.T) {
	root := writeScopeFixture(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	graph, err := loadPackageGraphContext(ctx, root, []string{"ze_ssh"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline = (%+v, %v), want context deadline exceeded", graph, err)
	}
	if graph != nil {
		t.Fatal("deadline returned a partial graph")
	}
}
