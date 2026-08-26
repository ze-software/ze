package main

// AC-11 for the L2TP peer gate: effective-l2tp-peer.py and
// `le deployment l2tp-test` do the same thing.
//
// A Python script and a Go command share no process, so no test can call both.
// They do share the three effects this proof has on the world: the daemon it
// builds, the containers and processes it starts, and the files it writes for
// the peer to read. So both halves are pointed at one recording docker and one
// recording go, over a fixture checkout, and what is compared is the argv that
// would have reached each of them plus the bytes left on disk.
//
// The recording docker also PLAYS the daemon: it prints the two lines the proof
// watches for. That is what makes the comparison reach the verdict rather than
// stopping at the first process, and it is why both halves finish in a second
// with no Docker installed.
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

	"github.com/ze-software/ze/letools/deployment"
)

// scriptTimeout bounds one run of the Python original. The stub answers in a
// second, so a minute means the script is stuck rather than slow, and the test
// reports that instead of hanging the suite.
const scriptTimeout = time.Minute

// argSep separates the words of one recorded call. A unit separator cannot
// appear in any argument either half passes, so a call round-trips exactly.
const argSep = "\x1f"

// dockerStub records every call and plays the two processes the proof waits on.
//
// The session line is delayed by a second, which is what makes the comparison
// see the PEER: without the gap both halves reach their verdict and stop before
// xl2tpd has recorded its own argv, and the one call that proves another
// implementation was involved goes missing from both recordings at once.
const dockerStub = `#!/bin/sh
sep=$(printf '\037')
line=""
for a in "$@"; do
  if [ -z "$line" ]; then line="$a"; else line="$line$sep$a"; fi
done
printf '%s\n' "$line" >> "$ZE_RECORD_DOCKER"
case "$1" in
  image) exit 0 ;;
  pull)  exit 0 ;;
  run)   echo deadbeef ; exit 0 ;;
  rm)    exit 0 ;;
  exec)
    case "$*" in
      *"apk add"*) exit 0 ;;
      *xl2tpd*) echo "xl2tpd: dialing 127.0.0.1" >&2 ; sleep 3 ; exit 0 ;;
      *) cat >/dev/null
         echo "l2tp: L2TP listener bound on 127.0.0.1:1701" >&2
         sleep 1
         echo "l2tp: tunnel 1 session established" >&2
         sleep 3 ; exit 0 ;;
    esac ;;
esac
exit 0
`

const goStub = `#!/bin/sh
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

// run is what one half of the comparison did.
type run struct {
	code    int
	docker  []string
	build   []string
	scratch map[string]string
}

// peerFixture builds a checkout each half can be pointed at: the manifest the
// build tags derive from, the go.mod that makes it a module, and a copy of the
// script beside the module it imports.
//
// The script is COPIED rather than run in place because it finds the tree by
// walking up from its own file. A run in place would build into the real
// checkout's tmp/evidence and mount the real checkout into a container.
func peerFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"feature-gates.txt": "ze_bgp internal/component/bgp\nze_l2tp internal/component/l2tp\n",
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
	for _, name := range []string{"effective-l2tp-peer.py", "feature_tags.py"} {
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

// peerStubs writes the two recording programs and answers their directory.
func peerStubs(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for name, body := range map[string]string{"docker": dockerStub, "go": goStub} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
			t.Fatalf("write the %s stub: %v", name, err)
		}
	}
	return dir
}

// calls answers one entry per recorded call, its words rejoined with a space.
func calls(t *testing.T, path string) []string {
	t.Helper()

	body, err := os.ReadFile(path) //nolint:gosec // the test wrote this path
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read the recording: %v", err)
	}

	var lines []string
	for line := range strings.SplitSeq(strings.TrimRight(string(body), "\n"), "\n") {
		if line != "" {
			lines = append(lines, strings.ReplaceAll(line, argSep, " "))
		}
	}
	return lines
}

// scratchFiles answers the peer's three configuration files out of the one
// scratch directory the run made under the fixture.
func scratchFiles(t *testing.T, root string) map[string]string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(root, "tmp", "evidence", "l2tp-peer-*"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("the run made %d scratch directories under %s, want 1 (%v)", len(matches), root, err)
	}

	files := map[string]string{}
	for _, name := range []string{"xl2tpd.conf", "l2tp-secrets", "ppp-options"} {
		body, err := os.ReadFile(filepath.Join(matches[0], name)) //nolint:gosec // a path this test's own run wrote
		if err != nil {
			t.Fatalf("read the scratch file %s: %v", name, err)
		}
		files[name] = string(body)
	}
	files["<work>"] = matches[0]
	return files
}

// normalize removes the three things that cannot match between two runs: the
// tree each was pointed at, the scratch directory each made, and the container
// name each derived from its own process id.
func normalize(lines []string, root, work string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.ReplaceAll(line, work, "<work>")
		line = strings.ReplaceAll(line, root, "<root>")
		if at := strings.Index(line, "ze-l2tp-evidence-"); at >= 0 {
			end := at + len("ze-l2tp-evidence-")
			for end < len(line) && line[end] >= '0' && line[end] <= '9' {
				end++
			}
			line = line[:at] + "<container>" + line[end:]
		}
		out = append(out, line)
	}
	return out
}

// runPeerScript runs the Python original over its own fixture.
func runPeerScript(t *testing.T) run {
	t.Helper()

	root := peerFixture(t)
	stubDir := peerStubs(t)
	record := t.TempDir()

	ctx, cancel := context.WithTimeout(t.Context(), scriptTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", filepath.Join(root, "scripts", "evidence", "effective-l2tp-peer.py")) //nolint:gosec // a path this test built
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ZE_RECORD_DOCKER="+filepath.Join(record, "docker"),
		"ZE_RECORD_GO="+filepath.Join(record, "go"),
	)
	out, err := cmd.CombinedOutput()

	code := 0
	if err != nil {
		exit, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			t.Fatalf("run the script: %v\n%s", err, out)
		}
		code = exit.ExitCode()
	}

	scratch := scratchFiles(t, root)
	return run{
		code:    code,
		docker:  normalize(calls(t, filepath.Join(record, "docker")), root, scratch["<work>"]),
		build:   normalize(calls(t, filepath.Join(record, "go")), root, scratch["<work>"]),
		scratch: scratch,
	}
}

// runPeerCommand runs the ported command over its own fixture, through the same
// stubs and the same production code path the binary runs.
func runPeerCommand(t *testing.T) run {
	t.Helper()

	root := peerFixture(t)
	stubDir := peerStubs(t)
	record := t.TempDir()

	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ZE_RECORD_DOCKER", filepath.Join(record, "docker"))
	t.Setenv("ZE_RECORD_GO", filepath.Join(record, "go"))

	proof := deployment.NewL2TP(root)
	proof.Progress = io.Discard
	// The stub answers at once, so the real twenty-second bounds would only
	// slow a failure down. Five seconds is far longer than the stub needs and
	// short enough that a broken port reports rather than hangs.
	proof.ListenerWait = 5 * time.Second
	proof.SessionWait = 5 * time.Second

	report, err := proof.Run()
	code := 0
	switch {
	case err != nil:
		code = 1
	case !report.Established:
		code = 1
	}

	scratch := scratchFiles(t, root)
	return run{
		code:    code,
		docker:  normalize(calls(t, filepath.Join(record, "docker")), root, scratch["<work>"]),
		build:   normalize(calls(t, filepath.Join(record, "go")), root, scratch["<work>"]),
		scratch: scratch,
	}
}

func sameCalls(t *testing.T, what string, script, command []string) {
	t.Helper()

	if len(script) != len(command) {
		t.Fatalf("the script made %d %s calls and the command made %d:\nscript:\n  %s\ncommand:\n  %s",
			len(script), what, len(command), strings.Join(script, "\n  "), strings.Join(command, "\n  "))
	}
	for i := range script {
		if script[i] != command[i] {
			t.Errorf("%s call %d differs:\nscript:  %s\ncommand: %s", what, i, script[i], command[i])
		}
	}
}

// VALIDATES: both halves reach the same verdict, build the daemon with the same
// tags, start the same containers and processes in the same order, and write
// the peer the same three files.
// PREVENTS: every way this port could change what the proof actually does, none
// of which the one-line "OK" on stdout would show.
func TestScriptAndCommandProveL2TPTheSameWay(t *testing.T) {
	script := runPeerScript(t)
	command := runPeerCommand(t)

	if script.code != 0 || command.code != 0 {
		t.Fatalf("the run answered script=%d command=%d, want 0 and 0", script.code, command.code)
	}

	// The counts are stated rather than only compared, so that two halves that
	// both did nothing cannot pass. Six docker calls: the image inspect, the
	// container, the package install, the daemon, the peer, and the removal.
	if len(script.build) != 1 || len(script.docker) != 6 {
		t.Fatalf("the script made %d go and %d docker calls, want 1 and 6:\n  %s",
			len(script.build), len(script.docker), strings.Join(script.docker, "\n  "))
	}

	sameCalls(t, "go", script.build, command.build)
	sameCalls(t, "docker", script.docker, command.docker)

	for _, name := range []string{"xl2tpd.conf", "l2tp-secrets", "ppp-options"} {
		if script.scratch[name] != command.scratch[name] {
			t.Errorf("the peer's %s differs:\nscript:\n%s\ncommand:\n%s",
				name, script.scratch[name], command.scratch[name])
		}
	}
}

// VALIDATES: the daemon each half builds carries every gate feature-gates.txt
// declares.
// PREVENTS: the regression this derivation exists for, in either half. ze_l2tp
// became a gate on 2026-07-24 and the script went on building a daemon with no
// L2TP for a month.
func TestBothHalvesBuildTheDaemonWithEveryGate(t *testing.T) {
	script := runPeerScript(t)
	command := runPeerCommand(t)

	for _, one := range []struct {
		half  string
		build []string
	}{{"script", script.build}, {"command", command.build}} {
		if len(one.build) != 1 {
			t.Fatalf("the %s ran go %d times, want 1: %v", one.half, len(one.build), one.build)
		}
		for _, tag := range []string{"ze_core", "ze_distro", "ze_bgp", "ze_l2tp"} {
			if !strings.Contains(one.build[0], tag) {
				t.Errorf("the %s built without %s: %s", one.half, tag, one.build[0])
			}
		}
	}
}

// VALIDATES: the script fails OPEN on a manifest that declares no gate, and the
// port does not.
// PREVENTS: this difference being lost. It is the 2026-08-26 evidence row in
// plan/journal/zero-value-as-valid-answer.md, fixed in the port only, and this
// test goes red the day somebody fixes the script -- which is when it should be
// deleted along with the script.
func TestThePortRefusesTheManifestTheScriptIgnores(t *testing.T) {
	root := peerFixture(t)
	stubDir := peerStubs(t)
	record := t.TempDir()

	manifest := filepath.Join(root, "feature-gates.txt")
	if err := os.WriteFile(manifest, []byte("# no gate is declared here\n"), 0o600); err != nil {
		t.Fatalf("empty the fixture manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), scriptTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", filepath.Join(root, "scripts", "evidence", "effective-l2tp-peer.py")) //nolint:gosec // a path this test built
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ZE_RECORD_DOCKER="+filepath.Join(record, "docker"),
		"ZE_RECORD_GO="+filepath.Join(record, "go"),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the script now refuses a manifest with no gate; delete this test with the script:\n%v\n%s", err, out)
	}

	built := calls(t, filepath.Join(record, "go"))
	if len(built) != 1 || strings.Contains(built[0], "ze_l2tp") {
		t.Fatalf("the script's build no longer omits the gates: %v", built)
	}

	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ZE_RECORD_DOCKER", filepath.Join(record, "docker-port"))
	t.Setenv("ZE_RECORD_GO", filepath.Join(record, "go-port"))

	proof := deployment.NewL2TP(root)
	proof.Progress = io.Discard
	if _, err := proof.Run(); err == nil {
		t.Error("the port built a daemon from a manifest that declares no gate")
	}
	if built := calls(t, filepath.Join(record, "go-port")); len(built) != 0 {
		t.Errorf("the port ran go anyway: %v", built)
	}
}
