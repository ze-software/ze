package testqemu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/runner"
)

// VALIDATES: the guest command for one .ci points the harness at the cross-built daemons
// and narrows `test qemu all` to that one file, through paths the guest resolves from the
// checkout it mounts.
// METHOD: OneCICommand over a fixture directory and file.
// PREVENTS: a guest run of the host-built ze (no break, or the wrong architecture) and a
// guest run of a whole suite where one .ci was asked for.
func TestOneCICommand(t *testing.T) {
	got := OneCICommand("tmp/qemu/x/linux-amd64/bin", "test/plugin/gated.ci", ArchAMD64)
	want := "ZE_BIN=tmp/qemu/x/linux-amd64/bin/ze ZE_STRIPPED_BIN=tmp/qemu/x/linux-amd64/bin/ze-stripped" +
		" ZE_STRIPPED_FLOW_BIN=tmp/qemu/x/linux-amd64/bin/ze-stripped-flow" +
		" tmp/qemu/linux-amd64/le test qemu all test test/plugin/gated.ci"
	if got != want {
		t.Errorf("OneCICommand\n got %q\nwant %q", got, want)
	}
}

// VALIDATES: the host command a needs-linux skip reason ends with
// (runner.GuestRoute) is a line `le test qemu ci` itself accepts, naming a path
// a guest suite walks. The runner sits below le and spells the command as a
// literal, so this is the check that compares the copy with its source.
// METHOD: GuestRoute over a .ci under this checkout, then the action table's
// own parser (List.Accepts) and the guest's suite selection (vmSuiteFor) over
// the words it printed.
// PREVENTS: a skip reason naming a verb or keyword le does not hold, or the
// guest-only `test qemu all`, which refuses on the host that printed it.
func TestRunnerGuestRouteIsTheOneCIAction(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	ciFile := filepath.Join(dir, "..", "..", "..", "..", "test", "plugin", "gated.ci")
	route, err := runner.GuestRoute(ciFile)
	if err != nil {
		t.Fatalf("GuestRoute(%q): %v", ciFile, err)
	}
	words := strings.Fields(route)
	if len(words) != 6 || strings.Join(words[:3], " ") != "./le "+area {
		t.Fatalf("route %q is not a `./le %s` line", route, area)
	}
	if words[3] != oneCIVerb {
		t.Fatalf("route %q names verb %q, want the host action %q", route, words[3], oneCIVerb)
	}
	if err := Actions().Accepts(words[3], words[4:]); err != nil {
		t.Fatalf("le %s refuses the route the runner prints, %q: %v", area, route, err)
	}
	if _, err := vmSuiteFor(words[5]); err != nil {
		t.Fatalf("no guest suite runs the path the route names, %q: %v", words[5], err)
	}
}

// VALIDATES: `le test qemu ci` refuses, before any build or boot, a path no
// guest suite walks and a .ci the checkout does not hold, and admits one it does.
// METHOD: oneCIAdmits over a fixture checkout holding one .ci.
// PREVENTS: a cross-build and a guest boot spent on a run the guest then refuses.
func TestOneCIAdmitsOnlyAHeldSuiteTest(t *testing.T) {
	root := t.TempDir()
	held := filepath.Join(root, "test", "plugin", "held.ci")
	if err := os.MkdirAll(filepath.Dir(held), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(held, []byte("option=needs-linux\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := oneCIAdmits(root, "test/plugin/held.ci"); err != nil {
		t.Errorf("a held suite .ci was refused: %v", err)
	}
	for _, file := range []string{"internal/x.ci", "test/plugin/held.et", "test/plugin/absent.ci"} {
		err := oneCIAdmits(root, file)
		if err == nil {
			t.Errorf("oneCIAdmits(%q) admitted it", file)
			continue
		}
		if !strings.HasSuffix(err.Error(), "\n./le test qemu ci test test/<dir>/<name>.ci") {
			t.Errorf("oneCIAdmits(%q) refusal does not end with the command to run: %v", file, err)
		}
	}
}

// VALIDATES: `le test qemu ci` boots the own-system carrier with harness, the
// kernel it resolved, its timeout, and the one-.ci guest command over the
// daemons `guest-binaries` builds.
// METHOD: oneCIArguments for a fixture kernel and file.
// PREVENTS: a host run that boots without harness (no /workspace, not root),
// against daemons the host did not cross-build, or of a whole suite.
func TestOneCIArguments(t *testing.T) {
	got := oneCIArguments("/k/vmlinuz", "test/plugin/gated.ci", ArchAMD64)
	if !got.Has(KeywordHarness) {
		t.Error("no harness: the guest command would run without the checkout at /workspace or root")
	}
	if kernel := got.One(keywordKernel); kernel != "/k/vmlinuz" {
		t.Errorf("kernel %q, want /k/vmlinuz", kernel)
	}
	if timeout := got.One(keywordTimeout); timeout != oneCICommandTimeout {
		t.Errorf("timeout %q, want %q", timeout, oneCICommandTimeout)
	}
	want := OneCICommand("tmp/qemu/linux-amd64/bin", "test/plugin/gated.ci", ArchAMD64)
	if command := got.One(keywordCommand); command != want {
		t.Errorf("command\n got %q\nwant %q", command, want)
	}
}
