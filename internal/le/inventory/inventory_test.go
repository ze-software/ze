// The tool is called as a function here, which is the whole point of compiling
// it. Its predecessor, scripts/inventory/commands_test.go, forked `go run` and
// asserted on the process's combined output; every case below says what the old
// assertion proved and where that proof now lives.

package inventory

import (
	"os"
	"path/filepath"
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

// TestCollectCountsAFixtureTree pins every number the tree contributes. The old
// subprocess test asserted only that the header and a total were printed; a
// count that was short by a directory passed it.
func TestCollectCountsAFixtureTree(t *testing.T) {
	dir := fixture(t)
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

	// Three files over two directories: 3 + 1 + 3 lines.
	packages, files, lines := inv.totals()
	if packages != 2 || files != 3 || lines != 7 {
		t.Errorf("counted %d packages, %d files and %d lines; want 2, 3 and 7", packages, files, lines)
	}
	if inv.Generated == "" {
		t.Error("the page carries no generation time")
	}
}

// TestCollectResolvesRPCCoverage pins the join between the .yang declarations
// and the .ci content: `thing-list` is exercised as `thing list` and
// `core-show` as `core show`, and an RPC no test names stays uncovered.
func TestCollectResolvesRPCCoverage(t *testing.T) {
	dir := fixture(t)
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

// TestCollectFailsOnAFileItCannotRead is the behavior the script did NOT have,
// asserted in the direction of the fix. Every number this tool publishes is a
// count of what the walk saw, so a file the walk lists and cannot open makes a
// count short. The script returned nil there and published the short count
// under a header claiming the output is always accurate.
//
// A dangling symbolic link is the cheapest way to reach that path: the walk
// lists it, because the directory entry says it is a file, and opening it fails.
func TestCollectFailsOnAFileItCannotRead(t *testing.T) {
	for _, tc := range []struct{ name, link string }{
		{"go file", "internal/thing/broken.go"},
		{"yang file", "internal/plugins/thing/yang/broken.yang"},
		{"ci file", "test/ui/broken.ci"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := fixture(t)
			path := filepath.Join(dir, tc.link)
			if err := os.Symlink(filepath.Join(dir, "nowhere"), path); err != nil {
				t.Fatalf("create the dangling link: %v", err)
			}

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
// scripts/inventory/commands_test.go's TestInventoryStopsOnUnreadableFile
// proved, as a function call. bufio.Scanner stops on a line above
// MaxScanTokenSize, so the rpc below it is never counted; the tool must report
// that rather than publish the short count.
func TestCollectFailsOnALineTooLongToScan(t *testing.T) {
	dir := fixture(t)
	body := "module big {\n  // " + strings.Repeat("x", 70*1024) + "\n  rpc do-a-thing { }\n}\n"
	write(t, dir, "internal/plugins/big/yang/big.yang", body)

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
	dir := t.TempDir()
	inv, err := Collect(dir)
	if err != nil {
		t.Fatalf("an empty tree was refused: %v", err)
	}
	if len(inv.PackageStats) != len(codeAreas) {
		t.Fatalf("the page holds %d area rows, want %d", len(inv.PackageStats), len(codeAreas))
	}
	for _, stats := range inv.PackageStats {
		if stats.Packages != 0 || stats.Files != 0 || stats.Lines != 0 {
			t.Errorf("%s counted %+v over an empty tree", stats.Area, stats)
		}
	}
	if inv.TotalTests != 0 || inv.TotalRPCs != 0 {
		t.Errorf("an empty tree answered %d tests and %d RPCs", inv.TotalTests, inv.TotalRPCs)
	}
}

// TestCollectReadsTheLiveRegistry pins the half of the answer that comes from
// this process rather than from the tree. It is why the tool blank-imports the
// product: a plugin list parsed out of source would go stale.
func TestCollectReadsTheLiveRegistry(t *testing.T) {
	inv, err := Collect(t.TempDir())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(inv.Plugins) == 0 {
		t.Fatal("the plugin registry answered nothing: the composition root registered no plugin")
	}
	for _, plugin := range inv.Plugins {
		if plugin.Name == "" {
			t.Errorf("a registered plugin carries no name: %+v", plugin)
		}
	}
	if len(inv.YANGModules) == 0 {
		t.Error("no YANG module is loaded")
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

// TestDescribeModulesAttributesEachModule reads the attribution end to end over
// a path map, which is what turns a module name into "plugin:<dir>".
func TestDescribeModulesAttributesEachModule(t *testing.T) {
	modules := describeModules(map[string]string{
		"ze-static.yang": "internal/plugins/static/yang/ze-static.yang",
	})
	if len(modules) == 0 {
		t.Fatal("no YANG module is loaded, so this test cannot see an attribution")
	}
	sorted := true
	for i := 1; i < len(modules); i++ {
		if modules[i-1].Name > modules[i].Name {
			sorted = false
		}
	}
	if !sorted {
		t.Error("the module list is not sorted by name")
	}
	for _, module := range modules {
		if module.Source == "" {
			t.Errorf("%s is attributed to nothing", module.Name)
		}
		if module.Name == "ze-static.yang" && module.Source != "plugin:static" {
			t.Errorf("ze-static.yang is attributed to %q, want plugin:static", module.Source)
		}
	}
}

// TestRPCNameIn pins the two declaration forms a .yang file uses.
func TestRPCNameIn(t *testing.T) {
	cases := map[string]string{
		"rpc foo-bar {":              "foo-bar",
		"rpc foo { description x; }": "foo",
		"rpc bare":                   "bare",
	}
	for line, want := range cases {
		if got := rpcNameIn(line); got != want {
			t.Errorf("rpcNameIn(%q) = %q, want %q", line, got, want)
		}
	}
}

// TestAnswerRefusesArguments: the tree is the checkout, so there is nothing for
// an argument to select. Accepting one silently would let `le inventory --json`
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

// TestVanishedTellsAMissingFileFromADanglingLink is the distinction the whole
// error policy rests on. Both fail to open with the same error, and only one of
// them means a published count fell short.
//
// The case that made this necessary was real: a concurrent session's temporary
// file under tmp/ was listed by the walk and removed before this tool reached
// it, and `le inventory` refused the checkout over it.
func TestVanishedTellsAMissingFileFromADanglingLink(t *testing.T) {
	dir := t.TempDir()

	gone := filepath.Join(dir, "gone.go")
	if !vanished(gone, os.ErrNotExist) {
		t.Error("a path that does not exist was not reported as vanished")
	}

	link := filepath.Join(dir, "dangling.go")
	if err := os.Symlink(filepath.Join(dir, "nowhere"), link); err != nil {
		t.Fatalf("create the dangling link: %v", err)
	}
	if vanished(link, os.ErrNotExist) {
		t.Error("a dangling symbolic link was reported as vanished: its directory entry is still there")
	}

	present := filepath.Join(dir, "present.go")
	write(t, dir, "present.go", "package fixture\n")
	if vanished(present, os.ErrPermission) {
		t.Error("a file that exists and could not be read was reported as vanished")
	}
}

// TestScanLinesSkipsAFileThatIsNoLongerThere pins the caller's contract: a
// vanished file contributes no line and stops nothing. It is the case that made
// vanished necessary, and it is the only one where a walked path that cannot be
// opened is not an error.
func TestScanLinesSkipsAFileThatIsNoLongerThere(t *testing.T) {
	lines := 0
	if err := scanLines(filepath.Join(t.TempDir(), "gone.go"), func(string) { lines++ }); err != nil {
		t.Errorf("a file that vanished was reported as an error: %v", err)
	}
	if lines != 0 {
		t.Errorf("a file that vanished contributed %d lines", lines)
	}
}
