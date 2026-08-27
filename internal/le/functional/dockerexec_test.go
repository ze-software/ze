// VALIDATES: spec-le-is-a-ze-binary AC-8 and AC-11. The Go analyzer derives
// wrappers to a fixpoint, classifies every producer fixture, and applies the
// committed floor without a Python process.
// PREVENTS: a seed-only scan, a guard from an earlier call protecting a later
// call, and a vacuous scan that reports no sites and passes the ratchet.
package functional

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

func dockerExecAnalyzeFixture(t *testing.T, sources map[string]string) DockerExecAnalysis {
	t.Helper()
	analysis, err := analyzeDockerExecSources(sources)
	if err != nil {
		t.Fatalf("analyzing fixture: %v", err)
	}
	return analysis
}

func dockerExecFixtureVerdicts(t *testing.T, source string, members ...string) []DockerExecSite {
	t.Helper()
	parts := []string{"def docker_exec_quiet(container, cmd):\n    return \"\"\n"}
	for _, member := range members {
		if member == dockerExecSeed {
			continue
		}
		parts = append(parts, "\ndef ", member, "(container, command):\n    return docker_exec_quiet(container, command)\n")
	}
	parts = append(parts, "\n", source)
	return dockerExecAnalyzeFixture(t, map[string]string{"f.py": strings.Join(parts, "")}).Sites
}

func TestDockerExecWrapperSetReachesAFixpointAcrossFiles(t *testing.T) {
	analysis := dockerExecAnalyzeFixture(t, map[string]string{
		"test/interop/interop.py": `
class FRR:
    def route_table(self):
        return self._vtysh_quiet("show ip route")

    def _vtysh_quiet(self, command):
        return docker_exec_quiet(self.container, ["vtysh", "-c", command])

def parse_lines(text):
    return text.splitlines()

def docker_exec_quiet(container, cmd):
    return ""
`,
		"test/interop-ipsec/lab.py": `
def _swanctl(container, args):
    return docker_exec_quiet(container, ["swanctl"] + args)

def summarize(container):
    out = docker_exec_quiet(container, ["show"])
    return out
`,
	})
	for _, name := range []string{dockerExecSeed, "_vtysh_quiet", "route_table", "_swanctl"} {
		if !slices.Contains(analysis.FailOpenFunctions, name) {
			t.Errorf("the fail-open set omits %s: %v", name, analysis.FailOpenFunctions)
		}
	}
	for _, name := range []string{"parse_lines", "summarize"} {
		if slices.Contains(analysis.FailOpenFunctions, name) {
			t.Errorf("the fail-open set incorrectly contains %s: %v", name, analysis.FailOpenFunctions)
		}
	}
}

func TestDockerExecVerdictsMatchEveryProducerFixture(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		verdict string
	}{
		{
			name: "bound and truth-tested",
			source: `
def route(self, prefix):
    output = self._vtysh_quiet("show bgp %s json" % prefix)
    if not output.strip():
        return None
    return json.loads(output)
`,
			verdict: dockerExecVerdictCheck,
		},
		{
			name: "bound and never tested",
			source: `
def is_dis(self):
    out = self._vtysh_quiet("show isis interface detail")
    return "DIS" in out or "Designated" in out
`,
			verdict: dockerExecVerdictOpen,
		},
		{
			name: "membership is not emptiness",
			source: `
def has_route(self, prefix):
    out = self._vtysh_quiet("show ip route")
    if prefix in out:
        return True
    return False
`,
			verdict: dockerExecVerdictOpen,
		},
		{
			name: "inline use",
			source: `
def dump(self):
    print(self._vtysh_quiet("show isis neighbor")[:500])
`,
			verdict: dockerExecVerdictOpen,
		},
		{
			name: "discarded bare call",
			source: `
def warm(self):
    self._vtysh_quiet("show version")
`,
			verdict: dockerExecVerdictDrop,
		},
		{
			name: "direct return",
			source: `
def outer(self):
    return self._vtysh_quiet("show version")
`,
			verdict: dockerExecVerdictCheck,
		},
		{
			name: "reasoned marker above",
			source: `
def dump(self):
    # fail-open-ok: diagnostic print on an already-failed run
    print(self._vtysh_quiet("show isis neighbor")[:500])
`,
			verdict: dockerExecVerdictAllow,
		},
		{
			name: "reasoned marker trailing",
			source: `
def dump(self):
    print(self._vtysh_quiet("show isis neighbor")[:500])  # fail-open-ok: diagnostic
`,
			verdict: dockerExecVerdictAllow,
		},
		{
			name: "bare marker",
			source: `
def dump(self):
    # fail-open-ok:
    print(self._vtysh_quiet("show isis neighbor")[:500])
`,
			verdict: dockerExecVerdictOpen,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sites := dockerExecFixtureVerdicts(t, test.source, "_vtysh_quiet")
			var found []DockerExecSite
			for _, site := range sites {
				if site.Function == "_vtysh_quiet" {
					continue
				}
				found = append(found, site)
			}
			if len(found) != 1 {
				t.Fatalf("fixture produced %d subject sites, want 1: %+v", len(found), found)
			}
			if found[0].Verdict != test.verdict {
				t.Errorf("verdict = %s, want %s", found[0].Verdict, test.verdict)
			}
		})
	}
}

func TestDockerExecEveryEmptinessShapeChecksTheBoundValue(t *testing.T) {
	guards := []string{
		"if not out:\n        return None",
		"if out:\n        return out",
		"if out == \"\":\n        return None",
		"if out is None:\n        return None",
		"if len(out) == 0:\n        return None",
		"assert out, \"empty\"",
		"while not out:\n        out = \"x\"",
	}
	for _, guard := range guards {
		t.Run(strings.Split(guard, "\n")[0], func(t *testing.T) {
			source := strings.Join([]string{
				"def probe(self):\n",
				"    out = self._vtysh_quiet(\"show version\")\n    ",
				guard,
				"\n    return out\n",
			}, "")
			sites := dockerExecFixtureVerdicts(t, source, "_vtysh_quiet")
			if sites[len(sites)-1].Verdict != dockerExecVerdictCheck {
				t.Errorf("guard %q did not check its call: %+v", guard, sites)
			}
		})
	}
}

func TestDockerExecAFormerGuardDoesNotProtectANewerAssignment(t *testing.T) {
	sites := dockerExecFixtureVerdicts(t, `
def route_count(self, family):
    output = self._vtysh_quiet("show bgp %s json" % family)
    if not output:
        return 0
    output = self._vtysh_quiet("show bgp %s summary" % family)
    return len(output.splitlines())
`, "_vtysh_quiet")
	var verdicts []string
	for _, site := range sites {
		if site.Function == "route_count" {
			verdicts = append(verdicts, site.Verdict)
		}
	}
	if !slices.Equal(verdicts, []string{dockerExecVerdictCheck, dockerExecVerdictOpen}) {
		t.Errorf("route_count verdicts = %v, want checked then unchecked", verdicts)
	}
}

func TestDockerExecATestAboveTheCallDoesNotProtectIt(t *testing.T) {
	sites := dockerExecFixtureVerdicts(t, `
def probe(self):
    if not out:
        return None
    out = self._vtysh_quiet("show isis neighbor")
    return "Up" in out
`, "_vtysh_quiet")
	if got := sites[len(sites)-1].Verdict; got != dockerExecVerdictOpen {
		t.Errorf("verdict = %s, want unchecked", got)
	}
}

func TestDockerExecInlineCallsAreSeparateUncheckedSites(t *testing.T) {
	sites := dockerExecFixtureVerdicts(t, `
def dump(self):
    print(self._vtysh_quiet("show isis neighbor")[:500])
    payload = json.loads(docker_exec_quiet("c", ["show", "-j"]))
    return payload
`, "_vtysh_quiet")
	unchecked := 0
	for _, site := range sites {
		if site.Function == "dump" && site.Verdict == dockerExecVerdictOpen {
			unchecked++
		}
	}
	if unchecked != 2 {
		t.Errorf("dump has %d unchecked sites, want 2: %+v", unchecked, sites)
	}
}

func TestDockerExecNonMemberCallsAreNotSites(t *testing.T) {
	analysis := dockerExecAnalyzeFixture(t, map[string]string{"f.py": `
def docker_exec_quiet(container, cmd):
    return ""

def dump(self):
    out = docker_exec("c", ["true"])
    return "x" in out
`})
	if len(analysis.Sites) != 0 {
		t.Errorf("a non-member became a site: %+v", analysis.Sites)
	}
}

func dockerExecWriteTree(t *testing.T, baseline any, extra string) string {
	t.Helper()
	root := t.TempDir()
	interop := filepath.Join(root, "test", "interop")
	health := filepath.Join(root, "test", "health")
	if err := os.MkdirAll(interop, 0o750); err != nil {
		t.Fatalf("creating interop fixture: %v", err)
	}
	if err := os.MkdirAll(health, 0o750); err != nil {
		t.Fatalf("creating health fixture: %v", err)
	}
	source := `
def docker_exec_quiet(container, cmd):
    return ""

def _vtysh_quiet(container, command):
    return docker_exec_quiet(container, ["vtysh", "-c", command])

def established(container):
    out = _vtysh_quiet(container, "show bgp summary")
    return "Established" in out
` + extra
	if err := os.WriteFile(filepath.Join(interop, "interop.py"), []byte(source), 0o600); err != nil {
		t.Fatalf("writing Python fixture: %v", err)
	}
	body, err := json.Marshal(map[string]any{dockerExecBaselineKey: baseline})
	if err != nil {
		t.Fatalf("encoding baseline: %v", err)
	}
	if err := os.WriteFile(filepath.Join(health, "docker-exec-baseline.json"), body, 0o600); err != nil {
		t.Fatalf("writing baseline: %v", err)
	}
	return root
}

func TestDockerExecBaselineRatchetPreservesBothDirections(t *testing.T) {
	atFloor, err := CheckDockerExec(dockerExecWriteTree(t, 1, ""))
	if err != nil {
		t.Fatalf("checking at floor: %v", err)
	}
	if atFloor.Code() != 0 {
		t.Errorf("the floor-equal report exits %d, want 0", atFloor.Code())
	}

	over, err := CheckDockerExec(dockerExecWriteTree(t, 1, `
def newly_added(container):
    out = _vtysh_quiet(container, "show isis neighbor")
    return "Up" in out
`))
	if err != nil {
		t.Fatalf("checking over floor: %v", err)
	}
	if over.Code() != 1 {
		t.Errorf("the increased report exits %d, want 1", over.Code())
	}
	if len(over.UncheckedSites) != 2 {
		t.Errorf("the increased report names %d sites, want 2", len(over.UncheckedSites))
	}

	below, err := CheckDockerExec(dockerExecWriteTree(t, 5, ""))
	if err != nil {
		t.Fatalf("checking below floor: %v", err)
	}
	if !strings.Contains(below.Text(), "lower the baseline") {
		t.Errorf("a falling floor gives no tightening instruction: %q", below.Text())
	}
}

func TestDockerExecUnreadableInputsFailClosed(t *testing.T) {
	t.Run("missing baseline", func(t *testing.T) {
		root := dockerExecWriteTree(t, 1, "")
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(dockerExecBaselineRel))); err != nil {
			t.Fatalf("removing fixture baseline: %v", err)
		}
		if _, err := CheckDockerExec(root); err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("missing baseline error = %v", err)
		}
	})
	t.Run("non-numeric baseline", func(t *testing.T) {
		if _, err := CheckDockerExec(dockerExecWriteTree(t, "many", "")); err == nil || !strings.Contains(err.Error(), "not a number") {
			t.Fatalf("non-numeric baseline error = %v", err)
		}
	})
	t.Run("invalid Python", func(t *testing.T) {
		root := dockerExecWriteTree(t, 1, "")
		path := filepath.Join(root, "test", "interop", "broken.py")
		if err := os.WriteFile(path, []byte("def (:\n"), 0o600); err != nil {
			t.Fatalf("writing invalid Python: %v", err)
		}
		if _, err := CheckDockerExec(root); err == nil || !strings.Contains(err.Error(), "broken.py") {
			t.Fatalf("invalid Python error = %v", err)
		}
	})
}

func TestDockerExecSelftestExercisesEveryVerdict(t *testing.T) {
	report, err := SelftestDockerExec()
	if err != nil {
		t.Fatalf("selftest: %v", err)
	}
	if report.Status != "OK" {
		t.Errorf("selftest status = %q, want OK", report.Status)
	}
	seen := make(map[string]bool)
	for _, site := range report.Verdicts {
		seen[site.Verdict] = true
	}
	for _, verdict := range []string{dockerExecVerdictCheck, dockerExecVerdictDrop, dockerExecVerdictAllow, dockerExecVerdictOpen} {
		if !seen[verdict] {
			t.Errorf("selftest does not exercise %s: %+v", verdict, report.Verdicts)
		}
	}
}

func TestDockerExecLiveScanCannotPassVacuously(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("locating repository: %v", err)
	}
	analysis, err := AnalyzeDockerExec(root)
	if err != nil {
		t.Fatalf("scanning repository: %v", err)
	}
	if len(analysis.FailOpenFunctions) <= 15 {
		t.Errorf("the fixpoint collapsed to %d members: %v", len(analysis.FailOpenFunctions), analysis.FailOpenFunctions)
	}
	for _, member := range []string{dockerExecSeed, "_vtysh_quiet", "_swanctl", "vtysh"} {
		if !slices.Contains(analysis.FailOpenFunctions, member) {
			t.Errorf("the live fail-open set omits %s", member)
		}
	}
	if len(analysis.Sites) <= 100 {
		t.Errorf("the live scan found only %d sites", len(analysis.Sites))
	}
	report, err := CheckDockerExec(root)
	if err != nil {
		t.Fatalf("checking live floor: %v", err)
	}
	if report.Code() != 0 {
		t.Errorf("live unchecked count %d exceeds floor %d", report.Counts.Unchecked, report.Baseline)
	}
}
