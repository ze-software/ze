// Design: docs/architecture/plugin/feature-gates.md -- the coupling check
package coupling

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

// VALIDATES: AC-20. Flow export subscribes to ribevents.BestChange, which only
// bgp-rib emits, under another gate. With its optional edge to bgp-rib the
// check is green for that pair; with the edge removed it reports flow export to
// bgp-rib over ribevents.BestChange.
//
// Method: load the real tree and the registrations this test binary links (it
// MUST be built with every feature tag, as le is), judge it, then judge it
// again with bgp-rib removed from flow-export's OptionalDependencies in memory.
// No file is edited, so nothing has to be restored.
func TestCouplingCheckFlagsUndeclaredSubscriber(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	plugins, err := repoinventory.Plugins(root)
	if err != nil {
		t.Fatal(err)
	}
	at := slices.IndexFunc(plugins, func(p repoinventory.Plugin) bool { return p.Name == "flow-export" })
	if at < 0 {
		t.Fatal("flow-export is not registered: build this test with every feature tag")
	}
	if !slices.Contains(plugins[at].OptionalDependencies, "bgp-rib") {
		t.Fatalf("flow-export declares no optional edge to bgp-rib: %v", plugins[at].OptionalDependencies)
	}

	declared, err := buildModel(root, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if found := flowExportRIBCoupling(declared.judge()); found != nil {
		t.Fatalf("with the edge declared the check still reports %+v", *found)
	}

	plugins[at].OptionalDependencies = slices.DeleteFunc(slices.Clone(plugins[at].OptionalDependencies),
		func(name string) bool { return name == "bgp-rib" })
	removed, err := buildModel(root, plugins)
	if err != nil {
		t.Fatal(err)
	}
	found := flowExportRIBCoupling(removed.judge())
	if found == nil {
		t.Fatal("with flow-export's edge to bgp-rib removed the check reports no flow-export coupling over ribevents.BestChange")
	}
	if found.Kind != KindEvent {
		t.Errorf("kind = %q, want %q", found.Kind, KindEvent)
	}
	if !slices.Equal(found.Providers, []string{"bgp-rib"}) {
		t.Errorf("providers = %v, want [bgp-rib]", found.Providers)
	}
	if !slices.Contains(found.ProviderTags, "ze_bgp") {
		t.Errorf("provider tags = %v, want ze_bgp among them", found.ProviderTags)
	}
}

// flowExportRIBCoupling answers flow-export's coupling over
// ribevents.BestChange, or nil.
func flowExportRIBCoupling(report CheckReport) *Coupling {
	for i := range report.Couplings {
		c := &report.Couplings[i]
		if c.Consumer == "flow-export" && c.Item == "ribevents.BestChange" {
			return c
		}
	}
	return nil
}

// VALIDATES: R-4. Any declared edge between consumer and producer satisfies
// the check, StartAfter included and in either direction, and no allowlist
// exists to reach green another way.
//
// Method: a synthetic tree with two plugins under two gates. Plugin a
// subscribes, through an import alias that differs from the package name, to
// a handle only plugin b emits. Judge it with no edge (one coupling), then with
// each edge kind on a, and with StartAfter on b toward a.
func TestCouplingCheckAcceptsStartAfter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "feature-gates.txt", "ze_a internal/plugins/a\nze_b internal/plugins/b\n")
	writeFile(t, root, "internal/core/ev/ev.go", `package ev

import "github.com/ze-software/ze/internal/core/events"

var Thing = events.Register[*int]("ns", "thing")
`)
	writeFile(t, root, "internal/plugins/b/b.go", `package b

import "github.com/ze-software/ze/internal/core/ev"

func emit() { ev.Thing.Emit(nil, nil) }
`)
	writeFile(t, root, "internal/plugins/a/a.go", `package a

import renamed "github.com/ze-software/ze/internal/core/ev"

func listen() { renamed.Thing.Subscribe(nil, nil) }
`)

	cases := []struct {
		name  string
		edges func(a, b *repoinventory.Plugin)
		want  int
	}{
		{"no edge", func(_, _ *repoinventory.Plugin) {}, 1},
		{"StartAfter on the consumer", func(a, _ *repoinventory.Plugin) { a.StartAfter = []string{"b"} }, 0},
		{"OptionalDependencies on the consumer", func(a, _ *repoinventory.Plugin) { a.OptionalDependencies = []string{"b"} }, 0},
		{"Dependencies on the consumer", func(a, _ *repoinventory.Plugin) { a.Dependencies = []string{"b"} }, 0},
		{"StartAfter on the producer", func(_, b *repoinventory.Plugin) { b.StartAfter = []string{"a"} }, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := repoinventory.Plugin{Name: "a", SourceDir: "internal/plugins/a"}
			b := repoinventory.Plugin{Name: "b", SourceDir: "internal/plugins/b"}
			tc.edges(&a, &b)
			built, err := buildModel(root, []repoinventory.Plugin{a, b})
			if err != nil {
				t.Fatal(err)
			}
			report := built.judge()
			if len(report.Couplings) != tc.want {
				t.Fatalf("couplings = %+v, want %d", report.Couplings, tc.want)
			}
			if tc.want == 0 {
				return
			}
			got := report.Couplings[0]
			if got.Consumer != "a" || got.Item != "renamed.Thing" || !slices.Equal(got.Providers, []string{"b"}) {
				t.Errorf("coupling = %+v, want a taking renamed.Thing from b", got)
			}
			if !slices.Equal(got.ProviderTags, []string{"ze_b"}) {
				t.Errorf("provider tags = %v, want [ze_b]", got.ProviderTags)
			}
		})
	}
}

// writeFile writes one file under root, creating its directories.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
