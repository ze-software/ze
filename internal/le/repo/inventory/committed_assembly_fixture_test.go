// Design: docs/architecture/plugin/feature-gates.md -- authored committed inputs for external-body certificates.
package repoinventory

import (
	"os/exec"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const asmOrdinaryDir = "internal/plugins/ordinary"
const asmTargetFile = asmOrdinaryDir + "/boundary_linux_amd64.s"
const asmTargetBody = "//go:build linux && amd64 && gc && ze_le && ze_fixture\n\n#include \"textflag.h\"\nTEXT ·inventoryGate(SB),NOSPLIT,$0-56\n JMP syscall·Syscall(SB)\n"
const asmSchema = "module fixture { namespace urn:fixture; prefix f; rpc fixture-list { } }\n"
const asmExtraSchema = "module extra { namespace urn:extra; prefix e; rpc extra-check { } }\n"

type asmInventoryFixture struct {
	root      string
	goSources map[string]string
}

// newASMInventoryFixture establishes exact ordinary registration/report controls
// before the unavailable-body input is introduced. It never compiles its source.
func newASMInventoryFixture(t *testing.T) *asmInventoryFixture {
	t.Helper()
	fixture := &asmInventoryFixture{root: t.TempDir(), goSources: make(map[string]string)}
	write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n")
	write(t, fixture.root, "feature-gates.txt", "ze_fixture "+asmOrdinaryDir+"\n")
	fixture.goFile(t, "cmd/ze/main.go", "//go:build ze_le && ze_fixture\npackage main\nimport _ \"github.com/ze-software/ze/"+asmOrdinaryDir+"\"\nfunc main() {}\n")
	fixture.goFile(t, "internal/component/plugin/registry/registry.go", `package registry
type Registration struct { Name, Description, YANG, Features string; Families, Dependencies, OptionalDependencies, StartAfter, ConfigRoots, RFCs []string; CapabilityCodes []uint8; RunEngine, InProcessNLRIDecoder, InProcessNLRIEncoder, InProcessRouteEncoder, InProcessConfigNLRIBuilder func() }
func Register(Registration) error { return nil }
`)
	fixture.goFile(t, "internal/component/config/yang/register.go", "package yang\nfunc RegisterModule(name, content string) {}\n")
	fixture.goFile(t, asmOrdinaryDir+"/register.go", `package ordinary
import (
 r "github.com/ze-software/ze/internal/component/plugin/registry"
 y "github.com/ze-software/ze/internal/component/config/yang"
)
func engine() {}
func init() {
 _ = r.Register(r.Registration{Name: "ordinary", Description: "ordinary committed control", Families: []string{"ipv4/unicast"}, CapabilityCodes: []uint8{64}, OptionalDependencies: []string{"optional"}, StartAfter: []string{"ordered"}, ConfigRoots: []string{"fixture"}, RFCs: []string{"fixture-rfc"}, Features: "fixture feature", RunEngine: engine, YANG: fixtureSchema, InProcessNLRIDecoder: engine, InProcessNLRIEncoder: engine, InProcessRouteEncoder: engine, InProcessConfigNLRIBuilder: engine})
 y.RegisterModule("extra.yang", extraSchema)
}
const fixtureSchema = `+strconv.Quote(asmSchema)+"\nconst extraSchema = "+strconv.Quote(asmExtraSchema)+"\n")
	write(t, fixture.root, asmOrdinaryDir+"/yang/fixture.yang", asmSchema)
	write(t, fixture.root, asmOrdinaryDir+"/yang/extra.yang", asmExtraSchema)
	write(t, fixture.root, "test/ui/fixture.ci", "cmd=foreground:exec=ze fixture list\n")
	write(t, fixture.root, "test/schema/extra.ci", "cmd=foreground:exec=ze extra check\n")
	committedGit(t, fixture.root, "init", "--initial-branch=main")
	commitInventoryFixture(t, fixture.root)
	inv, err := Collect(fixture.root)
	if err != nil {
		t.Fatalf("ordinary registered control: %v", err)
	}
	fixture.checkReport(t, inv)
	return fixture
}

func (fixture *asmInventoryFixture) goFile(t *testing.T, name, body string) {
	t.Helper()
	fixture.goSources[name] = body
	write(t, fixture.root, name, body)
}

func (fixture *asmInventoryFixture) addBoundary(t *testing.T, alias string) {
	t.Helper()
	imported := "\"syscall\""
	if alias != "syscall" {
		imported = alias + " \"syscall\""
	}
	fixture.goFile(t, asmOrdinaryDir+"/boundary.go", "package ordinary\nimport "+imported+"\nfunc inventoryGate(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err "+alias+".Errno)\nfunc init() { _, _, _ = inventoryGate(0, 0, 0, 0) }\n")
	write(t, fixture.root, asmTargetFile, asmTargetBody)
}

func (fixture *asmInventoryFixture) checkPlugins(t *testing.T, plugins []Plugin) {
	t.Helper()
	want := []Plugin{{
		Name: "ordinary", Description: "ordinary committed control", Families: []string{"ipv4/unicast"},
		Capabilities: []uint8{64}, OptionalDependencies: []string{"optional"}, StartAfter: []string{"ordered"},
		ConfigRoots: []string{"fixture"}, RFCs: []string{"fixture-rfc"}, Features: "fixture feature",
		SourceDir: asmOrdinaryDir, YANGFiles: []string{asmOrdinaryDir + "/yang/extra.yang", asmOrdinaryDir + "/yang/fixture.yang"},
		HasYANG: true, HasDecoder: true, HasEncoder: true,
	}}
	if !reflect.DeepEqual(plugins, want) {
		t.Fatalf("committed plugin metadata = %+v, want %+v", plugins, want)
	}
}

// checkReport derives code counts from the authored fixture corpus, not a host
// registry, live filesystem walk or a second invocation of the report producer.
func (fixture *asmInventoryFixture) checkReport(t *testing.T, inv Inventory) {
	t.Helper()
	fixture.checkPlugins(t, inv.Plugins)
	if !reflect.DeepEqual(inv.Families, map[string]string{"ipv4/unicast": "ordinary"}) {
		t.Errorf("families = %v", inv.Families)
	}
	if !reflect.DeepEqual(inv.Capabilities, map[string]string{"64": "ordinary"}) {
		t.Errorf("capabilities = %v", inv.Capabilities)
	}
	wantSupport := []Family{{Family: "ipv4/unicast", Plugin: "ordinary", Decode: true, Encode: true, RouteEncode: true, ConfigNLRI: true}}
	if !reflect.DeepEqual(inv.FamilySupport, wantSupport) {
		t.Errorf("family support = %+v", inv.FamilySupport)
	}
	wantModules := []YANGModule{{Name: "extra.yang", Source: "plugin:ordinary"}, {Name: "fixture", Source: "plugin:ordinary"}}
	if !reflect.DeepEqual(inv.YANGModules, wantModules) {
		t.Errorf("registered schemas = %+v, want %+v", inv.YANGModules, wantModules)
	}
	wantRPC := []RPC{{Name: "extra-check", Module: "extra.yang", Covered: true}, {Name: "fixture-list", Module: "fixture.yang", Covered: true}}
	if !reflect.DeepEqual(inv.RPCList, wantRPC) || inv.TotalRPCs != 2 || !reflect.DeepEqual(inv.RPCsByModule, map[string]int{"extra.yang": 1, "fixture.yang": 1}) {
		t.Errorf("RPC report = %+v totals %d %v", inv.RPCList, inv.TotalRPCs, inv.RPCsByModule)
	}
	if inv.TotalTests != 2 || !reflect.DeepEqual(inv.TestCounts, map[string]int{"ui": 1, "schema": 1}) {
		t.Errorf("CI report = %d %v", inv.TotalTests, inv.TestCounts)
	}
	wantStats := make([]AreaStats, 0, 3)
	for _, area := range []string{"internal", "pkg", "cmd"} {
		stats := AreaStats{Area: area + "/"}
		packages := make(map[string]bool)
		for name, body := range fixture.goSources {
			if !strings.HasPrefix(name, area+"/") {
				continue
			}
			stats.Files++
			packages[path.Dir(name)] = true
			stats.Lines += strings.Count(body, "\n")
			if !strings.HasSuffix(body, "\n") {
				stats.Lines++
			}
		}
		stats.Packages = len(packages)
		wantStats = append(wantStats, stats)
	}
	if !reflect.DeepEqual(inv.PackageStats, wantStats) {
		t.Errorf("Go report population = %+v, want exact authored %+v", inv.PackageStats, wantStats)
	}
}

// asmGitInput writes only test-owned Git objects; no shared index or checkout is
// involved. It supports explicit empty-tree and gitlink shadow fixtures.
func asmGitInput(t *testing.T, root, input string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	command.Stdin = strings.NewReader(input)
	raw, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %v: %s", args, err, raw)
	}
	return strings.TrimSpace(string(raw))
}

// asmCommitHeaderEntry rebuilds only ancestor tree objects so the header can be
// a tree (including empty) or gitlink, which a normal flat index cannot express.
func asmCommitHeaderEntry(t *testing.T, root, entry string) {
	t.Helper()
	head := strings.TrimSpace(committedGit(t, root, "rev-parse", "HEAD"))
	directory := asmOrdinaryDir
	entries := committedGit(t, root, "ls-tree", head+":"+directory)
	childTree := asmGitInput(t, root, entries+entry+"\n", "mktree")
	for directory != "." {
		parent, child := path.Dir(directory), path.Base(directory)
		revision := head
		if parent != "." {
			revision += ":" + parent
		}
		rows := strings.Split(strings.TrimSuffix(committedGit(t, root, "ls-tree", revision), "\n"), "\n")
		found := false
		for index, row := range rows {
			_, name, ok := strings.Cut(row, "\t")
			if ok && name == child {
				rows[index] = "040000 tree " + childTree + "\t" + child
				found = true
			}
		}
		if !found {
			t.Fatalf("fixture parent tree lacks %s", directory)
		}
		childTree = asmGitInput(t, root, strings.Join(rows, "\n")+"\n", "mktree")
		directory = parent
	}
	commit := asmGitInput(t, root, "", "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit-tree", childTree, "-p", head, "-m", "header object fixture")
	committedGit(t, root, "update-ref", "refs/heads/main", commit, head)
}

// asmExpectRefusal logs the actual boundary so Main can distinguish an old
// unavailable-body prerequisite from an exercised linkage/header discriminator.
func asmExpectRefusal(t *testing.T, fixture *asmInventoryFixture) {
	t.Helper()
	inv, err := Collect(fixture.root)
	if err == nil {
		t.Fatalf("unproved body/linkage published a partial successful report: %+v", inv.Plugins)
	}
	if inv.Plugins != nil {
		t.Fatalf("refusal leaked partial plugins: %+v: %v", inv.Plugins, err)
	}
	t.Logf("actual committed boundary refusal (classify prerequisite masking separately): %v", err)
}

func asmCheckSourcesStayGo(t *testing.T, snapshot *Snapshot, want int) {
	t.Helper()
	sources, _ := snapshot.Sources()
	if len(sources) != want {
		t.Fatalf("Go AST population = %d, want %d", len(sources), want)
	}
	if slices.ContainsFunc(sources, func(source SourceFile) bool { return !strings.HasSuffix(source.Path, ".go") }) {
		t.Fatal("assembly evidence entered Go Sources")
	}
}
