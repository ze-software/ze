package main

// AC-11 for the VPP interface gate: effective-vpp-iface.py and
// `le deployment vpp-iface-test` do the same thing.
//
// A Python script and a Go command share no process, so one test cannot call
// both directly. This proof instead compares their effects. These effects
// include the daemon build, container, ze processes, VPP commands, and written
// configuration files. Recording stand-ins replace docker and go over a fixture
// checkout. The test compares their argv and the bytes that they leave on disk.
//
// The recording docker also PLAYS VPP. It creates the two sockets awaited by
// the run. It answers `show plugins` with three plugin names and returns the
// required object for each feature query. These responses let the comparison
// reach all four verdicts instead of stopping at the container. Thus, both
// halves finish in seconds without Docker.
//
// This file lives beside the script rather than beside the port, so that step 14
// deletes the script and its parity proof in one commit.

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/deployment"
)

// ifaceScriptTimeout bounds one run of the Python original. The stubs answer
// immediately, so a two-minute run is stuck rather than slow. The test reports
// that state instead of blocking the suite. This bound exceeds the L2TP proof
// bound because one case waits through the script's 25-second scenario timeout.
const ifaceScriptTimeout = 2 * time.Minute

// The scratch files both halves write for VPP and ze to read.
var ifaceScratchFiles = []string{
	"startup.conf", "tunnel.conf", "mirror.conf", "wg.conf", "lcp.conf",
}

// vppDockerStub records every call and plays the VPP daemon.
//
// It gets the scratch directory from the container argument
// `-v <work>:/run/vpp`. This is the only place where either half names the
// directory. It creates both sockets there when VPP starts. Without these
// sockets, the run waits for its bound and never starts a scenario.
//
// EACH FEATURE PROBE WAITS FOR ITS OWN ze PROCESS. An immediate probe can let
// the run finish and stop the daemon before the daemon stub records its line.
// The recording then omits the call that proves ze started but still matches the
// other half. On 2026-08-26, this produced 13 calls instead of 14. Thus, the
// test asserts the count and compares it. The L2TP comparison found the same
// defect in another form during step 10.
const vppDockerStub = `#!/bin/sh
# record writes one call into the recording, its words separated by the unit
# separator so an argument carrying a space round-trips exactly.
#
# It is called by each branch rather than once at the top, because a probe MUST
# record after the ze process it waits for has recorded: recording first would
# put the two calls in the recording in the opposite order from the one they
# happened in.
record() {
  sep=$(printf '\037')
  line=""
  for a in "$@"; do
    if [ -z "$line" ]; then line="$a"; else line="$line$sep$a"; fi
  done
  printf '%s\n' "$line" >> "$ZE_RECORD_DOCKER"
}

# await_daemon answers once the ze process for one scenario has recorded its own
# call, and gives up after three seconds so a broken run reports rather than
# hangs.
await_daemon() {
  for _ in $(seq 60); do
    if [ -f "$ZE_VPP_WORK.$1" ]; then return 0; fi
    sleep 0.05
  done
  return 0
}

case "$1" in
  image) record "$@" ; exit ${ZE_IMAGE_EXIT:-0} ;;
  pull)  record "$@" ; exit 0 ;;
  rm)    record "$@" ; exit 0 ;;
  logs)  record "$@" ; echo "vpp: container log" ; exit 0 ;;
  run)
    record "$@"
    for a in "$@"; do
      case "$a" in
        *:/run/vpp) printf '%s' "${a%:/run/vpp}" > "$ZE_VPP_WORK" ;;
      esac
    done
    echo deadbeef ; exit 0 ;;
  exec)
    case "$*" in
      *"vpp -c /run/vpp/startup.conf"*)
        record "$@"
        w=$(cat "$ZE_VPP_WORK") ; : > "$w/api.sock" ; : > "$w/cli.sock" ; exit 0 ;;
      *"show plugins"*)
        record "$@"
        if [ "${ZE_PLUGINS_EXIT:-0}" != "0" ]; then
          echo "show plugins: unknown input" >&2 ; exit "$ZE_PLUGINS_EXIT"
        fi
        echo "wireguard_plugin.so" ; echo "linux_cp_plugin.so" ; echo "linux_nl_plugin.so" ; exit 0 ;;
      *"show version"*) record "$@" ; echo "vpp v24.02-release" ; exit 0 ;;
      *"show gre tunnel"*)
        await_daemon tunnel.conf ; record "$@"
        if [ "${ZE_GRE_EXIT:-0}" != "0" ]; then
          echo "show gre tunnel: unknown input 'gre tunnel'" >&2 ; exit "$ZE_GRE_EXIT"
        fi
        echo "[0] instance 0 src 10.10.10.1 dst 10.10.10.2" ; exit 0 ;;
      *"show interface span"*)
        await_daemon mirror.conf ; record "$@"
        echo "msrc0    rx    mdst0" ; exit 0 ;;
      *"show interface"*) record "$@" ; echo "local0    0    down" ; exit 0 ;;
      *"show wireguard interface"*)
        await_daemon wg.conf ; record "$@"
        echo "[0] wg0 port 51820" ; exit 0 ;;
      *"show lcp"*)
        await_daemon lcp.conf ; record "$@"
        echo "itf-pair: [0] loop0 tap0 host" ; exit 0 ;;
      *"ip link show"*) record "$@" ; echo "1: lo: <LOOPBACK,UP>" ; exit 0 ;;
      *"start /run/vpp/"*)
        record "$@"
        conf=""
        for a in "$@"; do
          case "$a" in /run/vpp/*.conf) conf=${a#/run/vpp/} ;; esac
        done
        : > "$ZE_VPP_WORK.$conf"
        echo "ze: interface backend vpp ready" >&2 ; sleep 5 ; exit 0 ;;
      *) record "$@" ; exit 0 ;;
    esac ;;
  *) record "$@" ; exit 0 ;;
esac
exit 0
`

// vppGoStub records the build and writes an executable at the requested output
// path. The next run therefore finds the daemon that it expects to have built.
const vppGoStub = `#!/bin/sh
sep=$(printf '\037')
line=""
for a in "$@"; do
  if [ -z "$line" ]; then line="$a"; else line="$line$sep$a"; fi
done
printf '%s\n' "$line" >> "$ZE_RECORD_GO"
prev=""
for a in "$@"; do
  if [ "$prev" = "-o" ]; then mkdir -p "$(dirname "$a")"; : > "$a"; chmod +x "$a"; fi
  prev="$a"
done
exit 0
`

// ifaceRun is what one half of the comparison did.
type ifaceRun struct {
	code    int
	docker  []string
	build   []string
	scratch map[string]string
}

// ifaceFixture builds a checkout for each half. It contains the manifest used
// for build tags and a go.mod that defines the module. It also puts a script
// copy beside the imported module.
//
// The script is COPIED rather than run in place because it finds the tree by
// walking up from its own file. A run in place would build into the real
// checkout's tmp/evidence and mount the real checkout into a container.
func ifaceFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"feature-gates.txt": "ze_bgp internal/component/bgp\nze_vpp internal/component/vpp\n",
		"go.mod":            "module example.test/m\n\ngo 1.26\n",
	}
	for rel, body := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o600); err != nil {
			t.Fatalf("write the fixture %s: %v", rel, err)
		}
	}

	dir := filepath.Join(root, "scripts", "evidence")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create the fixture script directory: %v", err)
	}
	for _, name := range []string{"effective-vpp-iface.py", "feature_tags.py"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
			t.Fatalf("copy %s into the fixture: %v", name, err)
		}
	}

	return root
}

// vppStubs writes the two recording programs and answers their directory.
func vppStubs(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for name, body := range map[string]string{"docker": vppDockerStub, "go": vppGoStub} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
			t.Fatalf("write the %s stub: %v", name, err)
		}
	}
	return dir
}

// ifaceScratch answers the configuration files out of the one scratch directory
// the run made under the fixture, and the directory itself.
//
// The recording marks a file as absent when the run did not write it. The two
// halves write the four scenario configurations at different times. Thus, the
// set of existing files is part of the comparison.
func ifaceScratch(t *testing.T, root string) map[string]string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(root, "tmp", "evidence", "vpp-iface-*"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("the run made %d scratch directories under %s, want 1 (%v)", len(matches), root, err)
	}

	files := map[string]string{"<work>": matches[0]}
	for _, name := range ifaceScratchFiles {
		body, err := os.ReadFile(filepath.Join(matches[0], name)) //nolint:gosec // a path this test's own run wrote
		if err != nil {
			files[name] = "<absent>"
			continue
		}
		files[name] = string(body)
	}
	return files
}

// ifaceNormalize removes three values that differ between runs: the input tree,
// scratch directory, and container name. Each run derives the container name
// from its own process ID.
func ifaceNormalize(lines []string, root, work string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.ReplaceAll(line, work, "<work>")
		line = strings.ReplaceAll(line, root, "<root>")
		if at := strings.Index(line, "ze-vpp-iface-"); at >= 0 {
			end := at + len("ze-vpp-iface-")
			for end < len(line) && line[end] >= '0' && line[end] <= '9' {
				end++
			}
			line = line[:at] + "<container>" + line[end:]
		}
		out = append(out, line)
	}
	return out
}

// ifaceEnv is the environment both halves are run under, with the stub
// directory in front of PATH and every recording named.
func ifaceEnv(stubDir, record string, extra ...string) []string {
	base := []string{
		"PATH=" + stubDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"ZE_RECORD_DOCKER=" + filepath.Join(record, "docker"),
		"ZE_RECORD_GO=" + filepath.Join(record, "go"),
		"ZE_VPP_WORK=" + filepath.Join(record, "work"),
	}
	return append(base, extra...)
}

// runIfaceScript runs the Python original over its own fixture.
func runIfaceScript(t *testing.T, extra ...string) ifaceRun {
	t.Helper()

	root := ifaceFixture(t)
	stubDir := vppStubs(t)
	record := t.TempDir()

	ctx, cancel := context.WithTimeout(t.Context(), ifaceScriptTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", filepath.Join(root, "scripts", "evidence", "effective-vpp-iface.py")) //nolint:gosec // a path this test built
	cmd.Dir = root
	cmd.Env = append(os.Environ(), ifaceEnv(stubDir, record, extra...)...)
	out, err := cmd.CombinedOutput()

	code := 0
	if err != nil {
		exit, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			t.Fatalf("run the script: %v\n%s", err, out)
		}
		code = exit.ExitCode()
	}

	scratch := ifaceScratch(t, root)
	return ifaceRun{
		code:    code,
		docker:  ifaceNormalize(calls(t, filepath.Join(record, "docker")), root, scratch["<work>"]),
		build:   ifaceNormalize(calls(t, filepath.Join(record, "go")), root, scratch["<work>"]),
		scratch: scratch,
	}
}

// runIfaceCommand runs the ported command over its own fixture, through the
// same stubs and the same production code path the binary runs.
func runIfaceCommand(t *testing.T, extra ...string) ifaceRun {
	t.Helper()

	root := ifaceFixture(t)
	stubDir := vppStubs(t)
	record := t.TempDir()

	for _, entry := range ifaceEnv(stubDir, record, extra...) {
		name, value, _ := strings.Cut(entry, "=")
		t.Setenv(name, value)
	}

	proof := deployment.NewVPPIface(root)
	proof.Progress = io.Discard
	// The stubs answer at once, so the real bounds would only slow a failure
	// down. Five seconds is far longer than a stub needs and short enough that
	// a broken port reports rather than hangs.
	proof.SocketWait = 5 * time.Second
	proof.ScenarioWait = 5 * time.Second

	report, err := proof.Run()
	code := 0
	switch {
	case err != nil:
		code = 1
	case !report.Passed:
		code = 1
	}

	scratch := ifaceScratch(t, root)
	return ifaceRun{
		code:    code,
		docker:  ifaceNormalize(calls(t, filepath.Join(record, "docker")), root, scratch["<work>"]),
		build:   ifaceNormalize(calls(t, filepath.Join(record, "go")), root, scratch["<work>"]),
		scratch: scratch,
	}
}

// withoutPluginQuery answers the call sequence with every `show plugins` removed.
//
// A plugin query reads state instead of changing it, and the halves use
// different query counts. The script repeats it for each gated scenario, for six
// total queries. The port queries each plugin once and saves the answer. Calls
// that change state or carry a verdict must match. Thus, the test removes plugin
// queries from the sequence and asserts them separately.
func withoutPluginQuery(calls []string) []string {
	out := make([]string, 0, len(calls))
	for _, call := range calls {
		if strings.Contains(call, "show plugins") {
			continue
		}
		out = append(out, call)
	}
	return out
}

// countPluginQueries answers how many times a half asked `show plugins`.
func countPluginQueries(calls []string) int {
	seen := 0
	for _, call := range calls {
		if strings.Contains(call, "show plugins") {
			seen++
		}
	}
	return seen
}

// VALIDATES: both halves reach the same verdict and build the daemon with the
// same tags. They start the same container and four ze processes in the same
// order. They also send the same feature queries to VPP and leave the same five
// configuration files.
// PREVENTS: changes in the work done by the port. The four OK lines on stdout do
// not expose these changes.
func TestScriptAndCommandProveVPPInterfacesTheSameWay(t *testing.T) {
	script := runIfaceScript(t)
	command := runIfaceCommand(t)

	if script.code != 0 || command.code != 0 {
		t.Fatalf("the run answered script=%d command=%d, want 0 and 0", script.code, command.code)
	}

	scriptCalls := withoutPluginQuery(script.docker)
	commandCalls := withoutPluginQuery(command.docker)

	// The test STATES the counts instead of only comparing them, so two inactive
	// halves cannot pass. It expects fourteen calls: image inspection, container,
	// VPP, and version query. Four scenarios each add a ze process and feature
	// query. The LCP link listing and removal are the final two calls.
	if len(script.build) != 1 || len(scriptCalls) != 14 {
		t.Fatalf("the script made %d go and %d non-query docker calls, want 1 and 14:\n  %s",
			len(script.build), len(scriptCalls), strings.Join(scriptCalls, "\n  "))
	}
	if len(commandCalls) != 14 {
		t.Fatalf("the command made %d non-query docker calls, want 14:\n  %s",
			len(commandCalls), strings.Join(commandCalls, "\n  "))
	}

	// Both halves must have ASKED about the plugins, or the two gated scenarios
	// ran on an answer nobody obtained.
	for _, one := range []struct {
		half  string
		calls []string
	}{{"script", script.docker}, {"command", command.docker}} {
		if seen := countPluginQueries(one.calls); seen == 0 {
			t.Errorf("the %s never asked `show plugins`, so its two gated scenarios decided on nothing", one.half)
		}
	}

	sameCalls(t, "go", script.build, command.build)
	sameCalls(t, "docker", scriptCalls, commandCalls)

	for _, name := range ifaceScratchFiles {
		if script.scratch[name] != command.scratch[name] {
			t.Errorf("the scratch file %s differs:\nscript:\n%s\ncommand:\n%s",
				name, script.scratch[name], command.scratch[name])
		}
	}
}

// VALIDATES: every one of the four features is actually exercised, by naming
// the ze process and the VPP query each of them owes.
// PREVENTS: the trap this comparison is most exposed to. Two recordings that
// agree on nothing prove nothing, and a port that stopped after the first
// scenario would still match a script that did the same.
func TestBothHalvesDriveEveryVPPFeature(t *testing.T) {
	script := runIfaceScript(t)
	command := runIfaceCommand(t)

	owed := []string{
		"start /run/vpp/tunnel.conf", "show gre tunnel",
		"start /run/vpp/mirror.conf", "show interface span",
		"start /run/vpp/wg.conf", "show wireguard interface",
		"start /run/vpp/lcp.conf", "show lcp",
		"ip link show",
	}
	for _, one := range []struct {
		half  string
		calls []string
	}{{"script", script.docker}, {"command", command.docker}} {
		for _, want := range owed {
			if anyCall(one.calls, want) {
				continue
			}
			t.Errorf("the %s never made a call carrying %q:\n  %s",
				one.half, want, strings.Join(one.calls, "\n  "))
		}
	}
}

// VALIDATES: the daemon built by each half includes every feature declared in
// feature-gates.txt.
// PREVENTS: recurrence of the regression that required this derivation. ze_l2tp
// became a gate on 2026-07-24, but no evidence script changed. For one month,
// all scripts built a daemon with the feature compiled out.
func TestBothHalvesBuildTheVPPDaemonWithEveryGate(t *testing.T) {
	script := runIfaceScript(t)
	command := runIfaceCommand(t)

	for _, one := range []struct {
		half  string
		build []string
	}{{"script", script.build}, {"command", command.build}} {
		if len(one.build) != 1 {
			t.Fatalf("the %s ran go %d times, want 1: %v", one.half, len(one.build), one.build)
		}
		for _, tag := range []string{"ze_core", "ze_distro", "ze_bgp", "ze_vpp"} {
			if !strings.Contains(one.build[0], tag) {
				t.Errorf("the %s built without %s: %s", one.half, tag, one.build[0])
			}
		}
	}
}

// VALIDATES: a FAILED `show plugins` stops the port but not the script.
// PREVENTS: loss of this difference. `plugin_loaded` in effective-vpp-iface.py
// ignores the query exit status and searches its error text for the plugin name.
// If the container CLI socket disappears, all three plugins answer "not loaded".
// The script then skips both gated scenarios and reports a pass over half the
// proof. The 2026-08-26 row in
// plan/journal/zero-value-as-valid-answer.md records this defect. Only the port
// fixes it. When somebody fixes the script, this test fails and must be deleted
// with the script.
func TestThePortRefusesThePluginQueryTheScriptIgnores(t *testing.T) {
	script := runIfaceScript(t, "ZE_PLUGINS_EXIT=1")
	if script.code != 0 {
		t.Fatalf("the script now refuses a failed `show plugins` (exit %d); delete this test with the script", script.code)
	}
	// It reported that pass over HALF the proof: the two ungated scenarios ran
	// and the two gated ones were skipped on an answer nobody obtained.
	for _, ran := range []string{"show gre tunnel", "show interface span"} {
		if !anyCall(script.docker, ran) {
			t.Fatalf("the script no longer runs %q, so this case proves nothing", ran)
		}
	}
	for _, skipped := range []string{"show wireguard interface", "show lcp"} {
		if anyCall(script.docker, skipped) {
			t.Fatalf("the script no longer skips %q on a failed plugin query", skipped)
		}
	}

	command := runIfaceCommand(t, "ZE_PLUGINS_EXIT=1")
	if command.code == 0 {
		t.Error("the port reported a pass over a plugin query that failed")
	}
	if anyCall(command.docker, "start /run/vpp/") {
		t.Error("the port started ze anyway, on a plugin answer nobody obtained")
	}
}

// anyCall reports whether any recorded call carries needle.
func anyCall(calls []string, needle string) bool {
	for _, call := range calls {
		if strings.Contains(call, needle) {
			return true
		}
	}
	return false
}

// VALIDATES: a GRE probe that FAILED contributes no evidence to the port, and
// does contribute it to the script.
// PREVENTS: this difference being lost.
// The script's fallback matches "gre" in text that includes the FAILED probe's error line.
// Thus, it reads `show gre tunnel: unknown input` as a tunnel.
// The scenario reports OK with nothing created.
// This test has the same journal row and deletion moment as the test above.
//
// This case is the slow one in the file.
// The script's scenario wait is fixed at 25 seconds.
// Thus, the cost occurs once here instead of in every case.
func TestThePortRefusesTheGREEvidenceTheScriptAccepts(t *testing.T) {
	script := runIfaceScript(t, "ZE_GRE_EXIT=1")
	if script.code != 0 {
		t.Fatalf("the script now refuses a failed GRE probe (exit %d); delete this test with the script", script.code)
	}

	command := runIfaceCommand(t, "ZE_GRE_EXIT=1")
	if command.code == 0 {
		t.Error("the port reported a pass over a GRE probe that never succeeded")
	}
	if anyCall(command.docker, "start /run/vpp/mirror.conf") {
		t.Error("the port went on to the next scenario after a failed one")
	}
}
