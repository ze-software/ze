// The tool is called as a function here, which is the whole point of compiling
// it. Its predecessor, internal/le/cli/list/commandlist_test.go, forked `go run` and
// asserted on the process's combined output; every case below says what the old
// assertion proved and where that proof now lives.

package repoinventory

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// write puts a file into the fixture tree, creating the directories above it.
func write(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// fixture builds a tree holding one of everything the inventory counts.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "internal/plugins/thing/yang/thing.yang",
		"module thing {\n  rpc thing-list { }\n  rpc thing-clear { }\n}\n")
	write(t, dir, "internal/component/config/yang/modules/core.yang",
		"module core {\n  rpc core-show { }\n}\n")
	write(t, dir, "internal/thing/one.go", "package thing\n\nvar a = 1\n")
	write(t, dir, "internal/thing/two.go", "package thing\n")
	write(t, dir, "cmd/ze/main.go", "package main\n\nfunc main() {}\n")
	write(t, dir, "test/ui/one.ci", "cmd=foreground:exec=ze thing list\n")
	write(t, dir, "test/ui/two.ci", "cmd=foreground:exec=ze other\n")
	write(t, dir, "test/bgp/three.ci", "cmd=foreground:exec=ze core show\n")
	return dir
}

// committedInventoryFixture retains the original report inputs and adds the
// declared entry and registry package required by committed-source membership.
func committedInventoryFixture(t *testing.T) string {
	t.Helper()
	root := fixture(t)
	write(t, root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n")
	write(t, root, "feature-gates.txt", "ze_thing internal/thing\n")
	write(t, root, "internal/thing/one.go", `package thing
import r "github.com/ze-software/ze/internal/component/plugin/registry"
func engine() {}
func init() { _ = r.Register(r.Registration{Name: "thing", Description: "committed fixture", RunEngine: engine, YANG: "module thing { namespace urn:thing; prefix t; }"}) }
`)
	write(t, root, "cmd/ze/main.go", "package main\nimport _ \"github.com/ze-software/ze/internal/thing\"\nfunc main() {}\n")
	write(t, root, "internal/component/plugin/registry/registry.go", "package registry\ntype Registration struct { Name, Description, YANG string; RunEngine func() }\nfunc Register(Registration) error { return nil }\n")
	committedGit(t, root, "init", "--initial-branch=main")
	commitInventoryFixture(t, root)
	return root
}

func commitInventoryFixture(t *testing.T, root string) {
	t.Helper()
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "committed fixture")
}

// TestCollectCountsAFixtureTree pins every number the tree contributes. The old
// subprocess test asserted only that the header and a total were printed; a
// count that was short by a directory passed it.
func TestCollectCountsAFixtureTree(t *testing.T) {
	dir := committedInventoryFixture(t)
	inv, err := Collect(dir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	if inv.TotalRPCs != 3 {
		t.Errorf("counted %d RPCs, want 3", inv.TotalRPCs)
	}
	if got := inv.RPCsByModule["thing.yang"]; got != 2 {
		t.Errorf("thing.yang holds %d RPCs, want 2", got)
	}
	if len(inv.RPCList) != 3 {
		t.Errorf("listed %d RPCs, want 3", len(inv.RPCList))
	}
	if inv.TotalTests != 3 {
		t.Errorf("counted %d .ci files, want 3", inv.TotalTests)
	}
	if got := inv.TestCounts["ui"]; got != 2 {
		t.Errorf("test/ui holds %d .ci files, want 2", got)
	}

	// Four files over three directories: 4 + 1 + 3 + 3 lines.
	packages, files, lines := inv.totals()
	if packages != 3 || files != 4 || lines != 11 {
		t.Errorf("counted %d packages, %d files and %d lines; want 3, 4 and 11", packages, files, lines)
	}
	if inv.Generated == "" {
		t.Error("the page carries no generation time")
	}
}

// TestCollectResolvesRPCCoverage pins the join between the .yang declarations
// and the .ci content: `thing-list` is exercised as `thing list` and
// `core-show` as `core show`, and an RPC no test names stays uncovered.
func TestCollectResolvesRPCCoverage(t *testing.T) {
	dir := committedInventoryFixture(t)
	inv, err := Collect(dir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	covered := map[string]bool{}
	for _, rpc := range inv.RPCList {
		covered[rpc.Name] = rpc.Covered
	}
	for name, want := range map[string]bool{"thing-list": true, "core-show": true, "thing-clear": false} {
		if covered[name] != want {
			t.Errorf("%s coverage is %v, want %v", name, covered[name], want)
		}
	}
	if got := inv.coveredRPCs(); got != 2 {
		t.Errorf("counted %d covered RPCs, want 2", got)
	}
}

// TestCollectRPCsUseOnlyCommittedCodeAreas checks the public report across two
// committed trees: source-area RPCs remain present, while duplicate and invalid
// YANG files in tracked session evidence never enter the report population.
func TestCollectRPCsUseOnlyCommittedCodeAreas(t *testing.T) {
	dir := committedInventoryFixture(t)
	write(t, dir, "pkg/example/pkg.yang", "module pkg {\n  rpc pkg-show { }\n}\n")
	write(t, dir, "cmd/example/cmd.yang", "module cmd {\n  rpc cmd-show { }\n}\n")
	commitInventoryFixture(t, dir)
	before, err := Collect(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantCounts := map[string]int{"cmd.yang": 1, "core.yang": 1, "pkg.yang": 1, "thing.yang": 2}
	if len(before.RPCList) != 5 || !reflect.DeepEqual(before.RPCsByModule, wantCounts) {
		t.Fatalf("source areas missing from RPC population: %v, %v", before.RPCsByModule, before.RPCList)
	}
	write(t, dir, "tmp/session/thing.yang", "module thing {\n  rpc scratch-only { }\n}\n")
	write(t, dir, "tmp/evidence/crash/hidden.yang", "module hidden {\n  rpc hidden-show { }\n}\n")
	write(t, dir, "tmp/evidence/crash/invalid.yang", "module invalid {\n")
	commitInventoryFixture(t, dir)
	after, err := Collect(dir)
	if err != nil {
		t.Fatalf("tracked session evidence entered the committed report: %v", err)
	}
	if !reflect.DeepEqual(after.RPCsByModule, before.RPCsByModule) || !reflect.DeepEqual(after.RPCList, before.RPCList) {
		t.Fatalf("session evidence contaminated RPCs: counts %v, RPCs %v; want %v, %v",
			after.RPCsByModule, after.RPCList, before.RPCsByModule, before.RPCList)
	}
}

// TestCollectFailsOnAFileItCannotRead commits unsupported source symlinks
// before collection. Go, YANG and CI inputs must each refuse with their filename
// rather than publish an inventory that silently skipped committed source.
func TestCollectFailsOnAFileItCannotRead(t *testing.T) {
	for _, tc := range []struct{ name, link string }{
		{"go file", "internal/thing/broken.go"},
		{"yang file", "internal/plugins/thing/yang/broken.yang"},
		{"ci file", "test/ui/broken.ci"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := committedInventoryFixture(t)
			path := filepath.Join(dir, tc.link)
			if err := os.Symlink(filepath.Join(dir, "nowhere"), path); err != nil {
				t.Fatalf("create the dangling link: %v", err)
			}
			commitInventoryFixture(t, dir)

			_, err := Collect(dir)
			if err == nil {
				t.Fatal("the inventory published counts taken from a walk that skipped a file")
			}
			if !strings.Contains(err.Error(), filepath.Base(tc.link)) {
				t.Errorf("the error does not name the file it could not read: %v", err)
			}
		})
	}
}

// TestCollectFailsOnALineTooLongToScan is what
// internal/le/cli/list/commandlist_test.go's TestInventoryStopsOnUnreadableFile
// proved, as a function call. bufio.Scanner stops on a line above
// MaxScanTokenSize, so the rpc below it is never counted; the tool must report
// that rather than publish the short count.
func TestCollectFailsOnALineTooLongToScan(t *testing.T) {
	dir := committedInventoryFixture(t)
	body := "module big {\n  // " + strings.Repeat("x", 70*1024) + "\n  rpc do-a-thing { }\n}\n"
	write(t, dir, "internal/plugins/big/yang/big.yang", body)
	commitInventoryFixture(t, dir)

	_, err := Collect(dir)
	if err == nil {
		t.Fatal("the inventory published counts from a file it could not read to the end")
	}
	if !strings.Contains(err.Error(), "big.yang") {
		t.Errorf("the error does not name the unreadable file: %v", err)
	}
}

// TestCollectTreatsAMissingAreaAsEmpty is the ONE walk error that is not a
// failure. A tree with no pkg/ directory holds no pkg/ code, which is a fact
// about the tree; refusing it would make the tool unusable on any tree that is
// not a full checkout.
func TestCollectTreatsAMissingAreaAsEmpty(t *testing.T) {
	root := committedInventoryFixture(t) // Complete committed entry, deliberately no pkg/.
	inv, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.PackageStats) != len(codeAreas) {
		t.Fatalf("the page holds %d area rows, want %d", len(inv.PackageStats), len(codeAreas))
	}
	found := false
	for _, stats := range inv.PackageStats {
		if stats.Area != "pkg/" {
			continue
		}
		found = true
		if stats.Packages != 0 || stats.Files != 0 || stats.Lines != 0 {
			t.Fatalf("absent pkg/ counted %+v", stats)
		}
	}
	if !found {
		t.Fatal("missing source area lost its explicit zero row")
	}
	if inv.TotalTests != 3 || inv.TotalRPCs != 3 {
		t.Fatalf("missing pkg/ changed other report areas: tests %d RPCs %d", inv.TotalTests, inv.TotalRPCs)
	}
}

// TestCollectReadsCommittedRegistrationsAndSchemas checks actual registration
// resolution and schema metadata from one committed fixture despite dirty source.
func TestCollectReadsCommittedRegistrationsAndSchemas(t *testing.T) {
	root := committedInventoryFixture(t)
	write(t, root, "internal/thing/one.go", "package thing\n// dirty replacement\n")
	inv, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Plugins) != 1 || inv.Plugins[0].Name != "thing" || inv.Plugins[0].Description != "committed fixture" {
		t.Fatalf("report mixed committed fixture and host/dirty registrations: %+v", inv.Plugins)
	}
	found := false
	for _, module := range inv.YANGModules {
		if module.Name == "thing" {
			found = true
		}
	}
	if !found {
		t.Fatal("committed registration schema is missing from report")
	}
}

// TestRPCHasCoverage pins the three spellings a test can name an RPC in. The
// glob form is the one a reader would not guess: `.ci` files write
// `peer * list` where the middle word is an address.
func TestRPCHasCoverage(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"hyphenated", "exec=ze peer-list\n", true},
		{"spaced", "exec=ze peer list\n", true},
		{"globbed", "expect=stdout:contains=peer * list\n", true},
		{"absent", "exec=ze route show\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rpcHasCoverage("peer-list", tc.content); got != tc.want {
				t.Errorf("rpcHasCoverage over %q = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

// TestRPCHasCoverageNeedsNoGlobForAOneWordName guards the glob branch against a
// name with no hyphen in it, where there is no second half to glob to.
func TestRPCHasCoverageNeedsNoGlobForAOneWordName(t *testing.T) {
	if rpcHasCoverage("commit", "exec=ze rollback\n") {
		t.Error("a one-word RPC matched content that does not name it")
	}
	if !rpcHasCoverage("commit", "exec=ze commit\n") {
		t.Error("a one-word RPC did not match content that names it")
	}
}

// TestPluginDirOf pins how a YANG module is attributed to a plugin.
func TestPluginDirOf(t *testing.T) {
	cases := []struct {
		path string
		want string
		in   bool
	}{
		{"internal/plugins/static/yang/static.yang", "static", true},
		{"internal/component/bgp/plugins/rib/yang/rib.yang", "rib", true},
		{"internal/component/config/yang/modules/core.yang", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			dir, under := pluginDirOf(tc.path)
			if under != tc.in || dir != tc.want {
				t.Errorf("pluginDirOf(%q) = %q, %v; want %q, %v", tc.path, dir, under, tc.want, tc.in)
			}
		})
	}
}

// TestCollectExtractsASingleWordRPC preserves the valid single-word declaration
// case at the public committed-source boundary, not through a copied line parser.
func TestCollectExtractsASingleWordRPC(t *testing.T) {
	root := committedInventoryFixture(t)
	write(t, root, "pkg/example/single.yang", "module single {\n namespace urn:single;\n prefix s;\n rpc foo { description x; }\n}\n")
	commitInventoryFixture(t, root)
	inv, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	if inv.TotalRPCs != 4 || len(inv.RPCList) != 4 || inv.RPCsByModule["single.yang"] != 1 {
		t.Fatalf("single-word RPC missing from committed population: counts %v, RPCs %v", inv.RPCsByModule, inv.RPCList)
	}
	found := 0
	for _, rpc := range inv.RPCList {
		if rpc.Module != "single.yang" {
			continue
		}
		found++
		if rpc.Name != "foo" {
			t.Fatalf("single-word declaration extracted as %q, want foo", rpc.Name)
		}
	}
	if found != 1 {
		t.Fatalf("single-word module has %d extracted RPCs, want 1", found)
	}
}

// TestAnswerRefusesArguments: the tree is the checkout, so there is nothing for
// an argument to select. Accepting one silently would let `le repo inventory --json`
// look like it worked while doing nothing.
func TestAnswerRefusesArguments(t *testing.T) {
	payload, code := Answer([]string{"--json"})
	if code == 0 {
		t.Error("an argument was accepted")
	}
	if payload != nil {
		t.Errorf("a refused call answered a payload: %v", payload)
	}
}

// TestAnswerAnswersTheInventory is AC-7 at the tool's own boundary: the payload
// is the data, never a rendering of it.
func TestAnswerAnswersTheInventory(t *testing.T) {
	payload, code := Answer(nil)
	if code != 0 {
		t.Fatalf("collecting the inventory answered %d", code)
	}
	inv, ok := payload.(Inventory)
	if !ok {
		t.Fatalf("the payload is %T, want Inventory", payload)
	}
	if len(inv.Plugins) == 0 {
		t.Error("the payload holds no plugin")
	}
}
