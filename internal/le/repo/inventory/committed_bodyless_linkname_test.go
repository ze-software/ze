// Design: docs/architecture/plugin/feature-gates.md -- a bodyless pull from the standard library is a library call.
package repoinventory

import (
	"strings"
	"testing"
)

// bodylessLinknameSource builds the initializer file for the linkname tests.
// It imports the standard-library packages both sinks witness, plus unsafe,
// which a real linkname file carries. The fixture is never compiled.
func bodylessLinknameSource(declarations, call string) string {
	return callableOwnerSource(callableOwnerCase{
		declarations: declarations,
		extraImport:  "_ \"unsafe\"\n clock \"time\"\n \"syscall\"",
	}, call)
}

// bodylessLinknameFixture is the external-effects fixture whose two sinks
// also witness syscall, so a call into syscall has a proved import route.
func bodylessLinknameFixture(t *testing.T, kind string) *asmInventoryFixture {
	t.Helper()
	fixture := externalEffectsFixture(t, kind)
	for _, sink := range []struct{ directory, name string }{
		{registrationPackage, "registry"},
		{"internal/component/config/yang", "yang"},
	} {
		fixture.goFile(t, sink.directory+"/external_witness.go", "package "+sink.name+"\nimport (_ \"os\"; _ \"io\"; _ \"sort\"; _ \"time\"; _ \"syscall\")\n")
	}
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)
	return fixture
}

// TestCommittedBodylessStandardLinknameIsAStandardCall reproduces the
// golang.org/x/sys/unix syscall_prlimit shape: a bodyless declaration pulled
// from a standard-library symbol and called during initialization. Each case
// runs the direct standard-library call and the linked call over the same
// arguments and requires the same verdict: an inert argument publishes the
// full report, and a callback that registers refuses through the external
// capability model, so the linked call's arguments are tracked like a direct
// call's.
//
// MUTATION: in certifyAssembly, change `len(own) == 1 && len(definitions) == 0`
// to `len(own) == 2 && len(definitions) == 0`: every linked call refuses with
// "0 selected definitions" while its direct twin publishes.
// MUTATION: in assemblyResult, delete `result.join(inputs)`: the linked
// registering callback no longer reaches the external model and the report
// publishes while the direct twin refuses.
func TestCommittedBodylessStandardLinknameIsAStandardCall(t *testing.T) {
	for _, scenario := range []struct {
		name, declarations     string
		directInert, linkInert string
		directHook, linkHook   string
	}{
		{
			name:         "prlimit",
			declarations: "//go:linkname inventoryLimit syscall.prlimit\nfunc inventoryLimit(pid, resource int, newlimit, old *syscall.Rlimit) error\n",
			directInert:  "_ = syscall.Setrlimit(0, nil)",
			linkInert:    "_ = inventoryLimit(0, 0, nil, nil)",
		},
		{
			name:         "callback",
			declarations: "//go:linkname inventoryAfter time.AfterFunc\nfunc inventoryAfter(d int64, f func()) bool\n",
			directInert:  "_ = clock.AfterFunc(0, func() {})",
			linkInert:    "_ = inventoryAfter(0, func() {})",
			directHook:   "_ = clock.AfterFunc(0, func() { install(0) })",
			linkHook:     "_ = inventoryAfter(0, func() { install(0) })",
		},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(scenario.name+"/"+kind, func(t *testing.T) {
				fixture := bodylessLinknameFixture(t, kind)
				for _, call := range []string{scenario.directInert, scenario.linkInert} {
					fixture.goFile(t, callableOwnerCaller, bodylessLinknameSource(scenario.declarations, call))
					commitInventoryFixture(t, fixture.root)
					externalEffectsPositive(t, fixture)
				}
				if scenario.directHook == "" {
					return
				}
				for _, call := range []string{scenario.directHook, scenario.linkHook} {
					fixture.goFile(t, callableOwnerCaller, bodylessLinknameSource(scenario.declarations, call))
					commitInventoryFixture(t, fixture.root)
					externalEffectsRefusal(t, fixture, "registration")
				}
			})
		}
	}
}

// TestCommittedBodylessModuleLinknameRefuses pulls a bodyless declaration
// from a module path. A module may be retained code, so the declaration stays
// linkname authority and the report refuses with today's cause.
//
// MUTATION: in indexLinkname, drop the standardLinkTarget condition so every
// bodyless pull returns early: the cause becomes the certificate's module
// refusal instead of "linkname authority is unsupported".
func TestCommittedBodylessModuleLinknameRefuses(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := bodylessLinknameFixture(t, kind)
			declarations := "//go:linkname inventoryLimit example.com/x.y\nfunc inventoryLimit() error\n"
			fixture.goFile(t, callableOwnerCaller, bodylessLinknameSource(declarations, "_ = inventoryLimit()"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "linkname authority is unsupported")
		})
	}
}

// TestCommittedBodylessWithoutLinknameRefuses keeps the original refusal for
// a bodyless declaration with no linkname and no assembly body.
//
// MUTATION: in certifyAssembly, delete the `len(definitions) != 1` refusal: a
// bodyless entry with no directive and no definition indexes an empty
// definition list, so the run panics instead of refusing with today's message.
func TestCommittedBodylessWithoutLinknameRefuses(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := bodylessLinknameFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, bodylessLinknameSource("func inventoryLimit() error\n", "_ = inventoryLimit()"))
			commitInventoryFixture(t, fixture.root)
			inv, err := Collect(fixture.root)
			if err == nil {
				t.Fatalf("bodyless initialization published a report: %+v", inv.Plugins)
			}
			if inv.Plugins != nil {
				t.Fatalf("bodyless refusal leaked partial plugins: %+v", inv.Plugins)
			}
			for _, want := range []string{"inventoryLimit", "has 0 selected definitions, not one proved body"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("bodyless refusal lacks %q: %v", want, err)
				}
			}
		})
	}
}
