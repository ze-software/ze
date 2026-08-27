package main

// AC-11 for the on-host L2TP PPP gate: effective-l2tp-ppp.py and
// `le deployment l2tp-ppp-test` do the same thing.
//
// A Python script and a Go command share no process, so one test cannot call
// both directly. This proof instead compares their effects on the machine. The
// effects include the daemon build, kernel-module requests, two namespaces, and
// a veth pair. They also include two daemons, PPP and L2TP kernel queries, and
// the daemon input files. Recording stand-ins replace `ip`, `go`, and `modprobe`
// over a fixture checkout. The test compares their argv and written bytes.
//
// THE RECORDING `ip` PLAYS THE KERNEL.
// It answers the link and address listings.
// For any other namespace command, it EXECS the command.
// Thus, the two daemon stand-ins start like the real daemons.
// They play the session: ze announces its listener, waits for the peer, and reports the session.
//
// It reports the withdraw after the peer leaves.
// Without this sequence, both halves stop at the listener.
// Their comparison then proves nothing.
//
// BOTH HALVES RUN INSIDE A USER NAMESPACE.
// The proof correctly refuses to start without CAP_NET_ADMIN and PPPoL2TP support.
// If it skips these checks, it reports a pass on a machine that cannot run the proof.
// `unshare --user --map-root-user` gives the process the capability inside its namespace.
//
// A bind mount over /sys/module supplies the module evidence.
// Both halves use the same environment.
// The Go half re-executes THIS test binary in the namespace and still calls the port as a function (AC-5).
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
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/deployment"
)

// pppHalfTimeout bounds one run of either half. The stand-ins answer within
// milliseconds, so a two-minute run is stuck rather than slow. The test reports
// that state instead of blocking the suite.
const pppHalfTimeout = 2 * time.Minute

// pppChildEnv marks the re-executed test binary that runs the port inside the
// namespace. A test binary run without it does nothing, which is what keeps the
// helper out of an ordinary `go test` run.
const pppChildEnv = "ZE_L2TP_PPP_PARITY_CHILD"

// pppRootEnv names the fixture checkout the re-executed half is pointed at.
const pppRootEnv = "ZE_L2TP_PPP_PARITY_ROOT"

// The addresses the recording `ip` answers with for each namespace, as
// `ip -o addr show dev` writes them: the local address bare, the peer address
// carrying its prefix length.
const (
	pppZeAddrLine  = `3: ppp0    inet 10.100.0.1 peer 10.100.0.2/32 scope global ppp0`
	pppLACAddrLine = `4: ppp1    inet 10.100.0.2 peer 10.100.0.1/32 scope global ppp1`
)

// The same two listings with every address moved one step up the pool, so that
// neither interface carries an address the proof asked for.
//
// 10.100.0.10 is the last address of the pool this proof configures, and the
// gateway 10.100.0.1 is a substring of it. A substring test therefore reads
// this listing as the right one, which is what the script does.
const (
	pppZeShiftedAddrLine  = `3: ppp0    inet 10.100.0.10 peer 10.100.0.20/32 scope global ppp0`
	pppLACShiftedAddrLine = `4: ppp1    inet 10.100.0.20 peer 10.100.0.10/32 scope global ppp1`
)

// The four files both halves write for the two daemons to read.
var pppInputFiles = []string{"xl2tpd.conf", "l2tp-secrets", "ppp-options", "ze.conf"}

// pppRecordFn is the shell function every stand-in records with. The words are
// separated by the unit separator so an argument carrying a space round-trips
// exactly.
const pppRecordFn = `sep=$(printf '\037')
record() {
  target="$1"
  shift
  line=""
  for a in "$@"; do
    if [ -z "$line" ]; then line="$a"; else line="$line$sep$a"; fi
  done
  printf '%s\n' "$line" >> "$target"
}
`

// pppIPStub records every ip call and plays the kernel.
//
// The link listing is empty until the ze stand-in says the session is up, and
// empty again once it says the routes are withdrawn. That is what makes the
// teardown assertion assert something: a port that never waited for the
// interfaces to go away would read them still there.
const pppIPStub = `#!/bin/bash
` + pppRecordFn + `
record "$ZE_RECORD_IP" "$@"
state="$ZE_PPP_STATE"

if [ "$1" = "netns" ] && [ "$2" = "exec" ]; then
  ns="$3"
  shift 3
  case "$1" in
    ip)
      case "$*" in
        "ip -o link show type ppp")
          if [ -f "$state/ppp-up" ]; then
            case "$ns" in
              *-ze-*) echo "3: ppp0: <POINTOPOINT,MULTICAST,NOARP,UP,LOWER_UP> mtu 1400 state UNKNOWN" ;;
              *) echo "4: ppp1: <POINTOPOINT,MULTICAST,NOARP,UP,LOWER_UP> mtu 1400 state UNKNOWN" ;;
            esac
          fi
          exit 0 ;;
        "ip -o addr show dev "*)
          case "$ns" in
            *-ze-*) printf '%s\n' "$ZE_PPP_ADDR_ZE" ;;
            *) printf '%s\n' "$ZE_PPP_ADDR_LAC" ;;
          esac
          exit 0 ;;
        *) exit 0 ;;
      esac ;;
    ping) exit 0 ;;
    cat) exit 0 ;;
    *) exec "$@" ;;
  esac
fi
exit 0
`

// pppModprobeStub records the module request and answers as a kernel with the
// code built in does.
const pppModprobeStub = `#!/bin/bash
` + pppRecordFn + `
record "$ZE_RECORD_MODPROBE" "$@"
exit 0
`

// pppToolStub stands in for a command the run only tests the presence of.
const pppToolStub = "#!/bin/bash\nexit 0\n"

// pppPeerStub records the peer's argv, tells the ze stand-in it has dialed,
// and tells it again when it is asked to leave.
//
// The wait is a backgrounded sleep rather than a plain one, because a shell
// does not run a trap while a foreground child is running. Without that the
// SIGTERM the proof sends would not be seen until the sleep ended, and the
// teardown the proof asserts about would never start.
const pppPeerStub = `#!/bin/bash
` + pppRecordFn + `
record "$ZE_RECORD_PEER" "$@"
state="$ZE_PPP_STATE"
mkdir -p "$state"
trap 'touch "$state/peer-stopped"; exit 0' TERM INT
: > "$state/peer-started"
i=0
while [ $i -lt 1200 ]; do
  sleep 0.1 &
  wait $!
  i=$((i+1))
done
exit 0
`

// pppDaemonStub is the ze stand-in the go stand-in writes.
//
// It copies the four input files into the recording BEFORE it announces
// anything. A passing run removes its scratch directory. Without these copies,
// both halves would lose all four files, and the test would compare nothing.
//
// It then plays the session IN ORDER.
// Each step waits for its real cause.
// The tunnel line waits for the peer to dial, and the withdraw line waits for the peer to leave.
// An immediate announcement CAN let both halves finish before the peer records its argv.
// Both recordings would then match and omit the peer.
// This area has produced that defect twice.
const pppDaemonStub = `#!/bin/bash
` + pppRecordFn + `
record "$ZE_RECORD_ZE" "$@"
env | grep -E '^(ZE_LOG_L2TP|ZE_STORAGE_BLOB|ZE_CONFIG_DIR|ZE_L2TP_SKIP_KERNEL_PROBE|ze\.l2tp\.)' | sort >> "$ZE_RECORD_ZE_ENV"
work=$(dirname "$ZE_CONFIG_DIR")
mkdir -p "$ZE_RECORD_INPUTS"
for f in xl2tpd.conf l2tp-secrets ppp-options ze.conf; do
  [ -f "$work/$f" ] && cp "$work/$f" "$ZE_RECORD_INPUTS/$f"
done
state="$ZE_PPP_STATE"
mkdir -p "$state"
echo "L2TP listener bound addr=172.30.0.1:1701" >&2
echo "l2tp-pool: configured gateway=10.100.0.1 start=10.100.0.2" >&2
n=0
while [ ! -f "$state/peer-started" ] && [ $n -lt 400 ]; do sleep 0.05; n=$((n+1)); done
echo "l2tp: session established (incoming LNS) tunnel-id=1 session-id=1" >&2
: > "$state/ppp-up"
echo "l2tp: session IP assigned tunnel-id=1 session-id=1 username=alice address=$ZE_PPP_ASSIGNED" >&2
echo "l2tp: subscriber route inject prefix=10.100.0.2/32 nexthop=ppp0" >&2
echo "l2tp: PPP session up tunnel-id=1 session-id=1 interface=ppp0" >&2
n=0
while [ ! -f "$state/peer-stopped" ] && [ $n -lt 1200 ]; do sleep 0.05; n=$((n+1)); done
rm -f "$state/ppp-up"
echo "l2tp: subscriber routes withdrawn tunnel-id=1 session-id=1" >&2
sleep 120
`

// pppGoStub records the build and writes the ze stand-in to the requested path.
// The next run therefore finds the daemon that it expects to have built.
const pppGoStub = `#!/bin/bash
` + pppRecordFn + `
record "$ZE_RECORD_GO" "$@"
prev=""
out=""
for a in "$@"; do
  if [ "$prev" = "-o" ]; then out="$a"; fi
  prev="$a"
done
[ -z "$out" ] && exit 0
mkdir -p "$(dirname "$out")"
cp "$ZE_PPP_DAEMON_STUB" "$out"
chmod +x "$out"
exit 0
`

// pppRun is what one half of the comparison did.
type pppRun struct {
	code      int
	ip        []string
	build     []string
	modprobe  []string
	daemon    []string
	daemonEnv []string
	peer      []string
	inputs    map[string]string
}

// pppFixture builds a checkout for each half. It contains the manifest used for
// build tags and a go.mod that defines the module. It also puts a script copy
// beside the imported module.
//
// The script is COPIED rather than run in place because it finds the tree by
// walking up from its own file. A run in place would build into the real
// checkout's tmp/evidence.
func pppFixture(t *testing.T) string {
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
	for _, name := range []string{"effective-l2tp-ppp.py", "feature_tags.py"} {
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

// pppStubs writes the recording programs and answers their directory, together
// with the path of the daemon stand-in the go stub copies into place.
func pppStubs(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	stubs := map[string]string{
		"ip":       pppIPStub,
		"go":       pppGoStub,
		"modprobe": pppModprobeStub,
		"xl2tpd":   pppPeerStub,
		"ping":     pppToolStub,
		"pppd":     pppToolStub,
	}
	for name, body := range stubs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
			t.Fatalf("write the %s stub: %v", name, err)
		}
	}

	daemon := filepath.Join(dir, "ze-daemon-stub")
	if err := os.WriteFile(daemon, []byte(pppDaemonStub), 0o755); err != nil { //nolint:gosec // the go stub copies this into place and runs it
		t.Fatalf("write the daemon stub: %v", err)
	}
	return dir, daemon
}

// pppEnv is the environment both halves are run under, with the stub directory
// in front of PATH and every recording named.
func pppEnv(stubDir, daemonStub, record, state string, extra ...string) []string {
	base := []string{
		"PATH=" + stubDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"ZE_RECORD_IP=" + filepath.Join(record, "ip"),
		"ZE_RECORD_GO=" + filepath.Join(record, "go"),
		"ZE_RECORD_MODPROBE=" + filepath.Join(record, "modprobe"),
		"ZE_RECORD_ZE=" + filepath.Join(record, "ze"),
		"ZE_RECORD_ZE_ENV=" + filepath.Join(record, "ze-env"),
		"ZE_RECORD_PEER=" + filepath.Join(record, "peer"),
		"ZE_RECORD_INPUTS=" + filepath.Join(record, "inputs"),
		"ZE_PPP_STATE=" + state,
		"ZE_PPP_DAEMON_STUB=" + daemonStub,
		"ZE_PPP_ADDR_ZE=" + pppZeAddrLine,
		"ZE_PPP_ADDR_LAC=" + pppLACAddrLine,
		"ZE_PPP_ASSIGNED=10.100.0.2",
	}
	return append(base, extra...)
}

// pppNamespaceArgv answers the argv that runs one command inside a user
// namespace where the proof's prerequisites are met.
//
// Two bind mounts supply the required data. /sys/module contains the module
// directory used by the PPPoL2TP check. /run contains the namespace directory
// used by `ip netns`, which is not writable outside the namespace.
//
// The wrapper and all stand-ins below use BASH instead of sh. dash DISCARDS an
// inherited variable when its name is not a shell identifier. This proof gives
// ze three such variables: ze.l2tp.ncp.ip-timeout and its two siblings. A dash
// wrapper removed all three from the daemon. Python then missed
// ze.l2tp.skip-kernel-probe and did not refuse the run. The real `ip` binary
// preserves these variables, so bash reproduces the gate environment.
func pppNamespaceArgv(fake string, argv ...string) []string {
	const script = `mount --bind "$1" /sys/module && mount --bind "$2" /run && shift 2 && exec "$@"`
	full := []string{
		"unshare", "--user", "--map-root-user", "--mount",
		"bash", "-c", script, "bash",
		filepath.Join(fake, "module"), filepath.Join(fake, "run"),
	}
	return append(full, argv...)
}

// pppFakeMounts answers a directory holding what the two bind mounts carry.
func pppFakeMounts(t *testing.T) string {
	t.Helper()

	fake := t.TempDir()
	for _, rel := range []string{filepath.Join("module", "l2tp_ppp"), "run"} {
		if err := os.MkdirAll(filepath.Join(fake, rel), 0o750); err != nil {
			t.Fatalf("build the fixture mounts: %v", err)
		}
	}
	return fake
}

// requireNamespaceSupport skips when this machine cannot carry the comparison.
//
// The proof needs two real facilities: an unprivileged user namespace and the
// PPP character device. Some distributions refuse the namespace, and creating
// the device requires real privilege. A machine without these facilities also
// cannot run the gate, so this test has nothing to prove there.
func requireNamespaceSupport(t *testing.T) {
	t.Helper()

	info, err := os.Stat("/dev/ppp")
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		t.Skip("this machine has no /dev/ppp, so neither half of the proof can start")
	}
	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skip("this machine has no unshare, so the proof's own prerequisites cannot be met")
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	probe := exec.CommandContext(ctx, "unshare", "--user", "--map-root-user", "--mount", "true")
	if out, err := probe.CombinedOutput(); err != nil {
		t.Skipf("this machine refuses an unprivileged user namespace: %v\n%s", err, out)
	}
}

// runPPPScript runs the Python original inside the namespace, over its own
// fixture.
func runPPPScript(t *testing.T, extra ...string) pppRun {
	t.Helper()

	root := pppFixture(t)
	stubDir, daemonStub := pppStubs(t)
	record := t.TempDir()

	argv := pppNamespaceArgv(pppFakeMounts(t), "python3",
		filepath.Join(root, "scripts", "evidence", "effective-l2tp-ppp.py"))
	return pppExecute(t, root, record, stubDir, daemonStub, argv, extra, nil)
}

// runPPPCommand runs the ported command inside the namespace, by re-executing
// this test binary there.
//
// The re-execution is what puts the port under the same capability and the same
// module evidence as the script. The child still calls the port as a function
// (TestTheL2TPPPPParityChildRunsThePort), so nothing here shells out to `go run`.
func runPPPCommand(t *testing.T, extra ...string) pppRun {
	t.Helper()

	root := pppFixture(t)
	stubDir, daemonStub := pppStubs(t)
	record := t.TempDir()

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("find this test binary: %v", err)
	}
	argv := pppNamespaceArgv(pppFakeMounts(t), self, "-test.run=^TestTheL2TPPPPParityChildRunsThePort$")
	child := []string{pppChildEnv + "=1", pppRootEnv + "=" + root}
	return pppExecute(t, root, record, stubDir, daemonStub, argv, extra, child)
}

// pppExecute runs one half and answers what it left behind.
func pppExecute(t *testing.T, root, record, stubDir, daemonStub string, argv, extra, child []string) pppRun {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), pppHalfTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // an argv this test built
	cmd.Dir = root
	cmd.Env = append(os.Environ(), pppEnv(stubDir, daemonStub, record, t.TempDir(), extra...)...)
	cmd.Env = append(cmd.Env, child...)
	out, err := cmd.CombinedOutput()

	return readPPPRun(t, root, record, pppExitCode(t, err, out))
}

// TestTheL2TPPPPParityChildRunsThePort is the re-executed half of the
// comparison. Outside the namespace it does nothing at all, which is what keeps
// it out of an ordinary run of this package's tests.
func TestTheL2TPPPPParityChildRunsThePort(t *testing.T) { //nolint:unparam // the signature is the framework's, and this half never reports through it
	if os.Getenv(pppChildEnv) != "1" {
		return
	}

	proof := deployment.NewL2TPPPP(os.Getenv(pppRootEnv))
	proof.Progress = io.Discard
	// The stand-ins answer within milliseconds, so the real bounds would only
	// slow a broken run down. Twenty seconds is far longer than any stand-in
	// needs and short enough that a broken port reports rather than hangs.
	const bound = 20 * time.Second
	proof.ListenerWait = bound
	proof.PoolWait = bound
	proof.SessionWait = bound
	proof.NCPWait = bound
	proof.WithdrawWait = bound
	proof.CleanupWait = bound

	report, err := proof.Run()
	code := 0
	if err != nil || !report.Proven {
		code = 1
	}
	os.Exit(code)
}

// pppExitCode answers what a half exited with, failing the test when it did not
// exit at all.
func pppExitCode(t *testing.T, err error, out []byte) int {
	t.Helper()

	if err == nil {
		return 0
	}
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("run one half of the comparison: %v\n%s", err, out)
	}
	return exit.ExitCode()
}

// readPPPRun answers what one half recorded.
func readPPPRun(t *testing.T, root, record string, code int) pppRun {
	t.Helper()

	return pppRun{
		code:      code,
		ip:        pppNormalize(calls(t, filepath.Join(record, "ip")), root),
		build:     pppNormalize(calls(t, filepath.Join(record, "go")), root),
		modprobe:  pppNormalize(calls(t, filepath.Join(record, "modprobe")), root),
		daemon:    pppNormalize(calls(t, filepath.Join(record, "ze")), root),
		daemonEnv: pppNormalize(calls(t, filepath.Join(record, "ze-env")), root),
		peer:      pppNormalize(calls(t, filepath.Join(record, "peer")), root),
		inputs:    pppInputs(t, root, filepath.Join(record, "inputs")),
	}
}

// pppInputs answers the four files copied by the daemon stand-in. The copy occurs
// before the run removes its scratch directory.
//
// A missing write appears as an absent file, and the comparison includes that
// result. A half that writes three files differs from a half that writes four.
func pppInputs(t *testing.T, root, dir string) map[string]string {
	t.Helper()

	files := map[string]string{}
	for _, name := range pppInputFiles {
		body, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // a path this test's own run wrote
		if err != nil {
			files[name] = "<absent>"
			continue
		}
		files[name] = strings.Join(pppNormalize([]string{string(body)}, root), "")
	}
	return files
}

// Each half derives names from its own process ID and scratch directory.
// Those names cannot match between two runs.
// Normalization removes every variable name and leaves the sequence of actions.
var pppDerivedNames = []struct {
	pattern *regexp.Regexp
	with    string
}{
	{regexp.MustCompile(`ze-l2tp-ppp-ze-\d+`), "<ze-ns>"},
	{regexp.MustCompile(`ze-l2tp-ppp-lac-\d+`), "<lac-ns>"},
	{regexp.MustCompile(`zpppz\d+`), "<ze-veth>"},
	{regexp.MustCompile(`zpppl\d+`), "<lac-veth>"},
	{regexp.MustCompile(`effective-l2tp-ppp-[A-Za-z0-9_]+`), "<work>"},
}

// pppNormalize removes each checkout and scratch directory.
// It also removes the namespaces and links named for each process ID.
func pppNormalize(lines []string, root string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.ReplaceAll(line, root, "<root>")
		for _, one := range pppDerivedNames {
			line = one.pattern.ReplaceAllString(line, one.with)
		}
		out = append(out, line)
	}
	return out
}

// The absolute counts a full run owes, stated rather than only compared.
//
// A full run makes 50 ip calls.
// They include one Generic Netlink probe and two cleanups of eight calls each.
// They also include eleven namespace and link commands, one underlay ping, and two per-namespace L2TP probes.
// The remaining calls are six baseline questions, two daemon starts, five kernel assertions, and six teardown questions.
// The run also makes five modprobe calls, one for each module.
// It makes one call each to the build, ze process, and peer process.
//
// The counts are STATED because two recordings that agree on nothing compare equal.
// This area has produced that failure twice: at 13 calls against 14, and at five against six.
// The two recordings were byte-identical each time.
const (
	pppIPCalls       = 50
	pppModprobeCalls = 5
	pppBuildCalls    = 1
	pppDaemonCalls   = 1
	pppPeerCalls     = 1
)

// VALIDATES: both halves reach the same verdict and request the same kernel modules.
// They build the daemon with the same tags and make the same namespaces and links.
// They start the same two daemons in the same environment.
// They ask the kernel the same questions in the same order.
// Both halves also write the same four files for the two daemons.
// PREVENTS: a port that changes what the proof does when the one OK line does not show the change.
func TestScriptAndCommandProveTheL2TPPPPPathTheSameWay(t *testing.T) {
	requireNamespaceSupport(t)

	script := runPPPScript(t)
	command := runPPPCommand(t)

	if script.code != 0 || command.code != 0 {
		t.Fatalf("the run answered script=%d command=%d, want 0 and 0:\nscript ip calls:\n  %s\ncommand ip calls:\n  %s",
			script.code, command.code, strings.Join(script.ip, "\n  "), strings.Join(command.ip, "\n  "))
	}

	for _, one := range []struct {
		half string
		run  pppRun
	}{{"script", script}, {"command", command}} {
		counted := []struct {
			what string
			got  int
			want int
		}{
			{"ip", len(one.run.ip), pppIPCalls},
			{"modprobe", len(one.run.modprobe), pppModprobeCalls},
			{"go", len(one.run.build), pppBuildCalls},
			{"ze", len(one.run.daemon), pppDaemonCalls},
			{"xl2tpd", len(one.run.peer), pppPeerCalls},
		}
		for _, count := range counted {
			if count.got == count.want {
				continue
			}
			t.Errorf("the %s made %d %s calls, want %d:\n  %s",
				one.half, count.got, count.what, count.want, strings.Join(one.run.ip, "\n  "))
		}
	}
	if t.Failed() {
		return
	}

	sameCalls(t, "ip", script.ip, command.ip)
	sameCalls(t, "modprobe", script.modprobe, command.modprobe)
	sameCalls(t, "go", script.build, command.build)
	sameCalls(t, "ze", script.daemon, command.daemon)
	sameCalls(t, "ze environment", script.daemonEnv, command.daemonEnv)
	sameCalls(t, "xl2tpd", script.peer, command.peer)

	for _, name := range pppInputFiles {
		if script.inputs[name] == "<absent>" {
			t.Errorf("neither half can be compared on %s: the script wrote none", name)
			continue
		}
		if script.inputs[name] != command.inputs[name] {
			t.Errorf("the input file %s differs:\nscript:\n%s\ncommand:\n%s",
				name, script.inputs[name], command.inputs[name])
		}
	}
}

// VALIDATES: every step of the L2TP path is actually walked, by naming the
// kernel question each of them owes.
// PREVENTS: the trap this comparison is most exposed to. Two recordings that
// agree on nothing prove nothing, and a port that stopped after the listener
// would still match a script that did the same.
func TestBothHalvesWalkEveryStepOfTheL2TPPath(t *testing.T) {
	requireNamespaceSupport(t)

	script := runPPPScript(t)
	command := runPPPCommand(t)

	owed := []string{
		"netns add <ze-ns>",
		"netns add <lac-ns>",
		"link add <ze-veth> type veth peer name <lac-veth>",
		"netns exec <lac-ns> ping -c 1 -W 2 172.30.0.1",
		"netns exec <ze-ns> ip l2tp show tunnel",
		"netns exec <ze-ns> ip l2tp show session",
		"netns exec <ze-ns> ip -o link show type ppp",
		"netns exec <ze-ns> <root>/tmp/evidence/bin/ze-l2tp-ppp start",
		"netns exec <lac-ns> xl2tpd -D -c",
		"netns exec <ze-ns> ip -o addr show dev ppp0",
		"netns exec <lac-ns> ip -o addr show dev ppp1",
		"netns exec <lac-ns> ping -c 2 -W 3 10.100.0.1",
		"netns delete <ze-ns>",
	}
	for _, one := range []struct {
		half  string
		calls []string
	}{{"script", script.ip}, {"command", command.ip}} {
		for _, want := range owed {
			if anyPPPCall(one.calls, want) {
				continue
			}
			t.Errorf("the %s never made an ip call carrying %q:\n  %s",
				one.half, want, strings.Join(one.calls, "\n  "))
		}
	}
}

// anyPPPCall reports whether any recorded call carries needle.
func anyPPPCall(calls []string, needle string) bool {
	for _, call := range calls {
		if strings.Contains(call, needle) {
			return true
		}
	}
	return false
}

// VALIDATES: the daemon each half builds carries every gate feature-gates.txt
// declares, and is built for THIS machine rather than cross-compiled.
// PREVENTS: the regression this derivation exists for, in either half.
// ze_l2tp became a gate on 2026-07-24, but no evidence script was updated.
// For a month, every script built a daemon with the feature compiled out.
// This gate proves L2TP.
func TestBothHalvesBuildTheL2TPPPPDaemonWithEveryGate(t *testing.T) {
	requireNamespaceSupport(t)

	script := runPPPScript(t)
	command := runPPPCommand(t)

	for _, one := range []struct {
		half  string
		build []string
	}{{"script", script.build}, {"command", command.build}} {
		if len(one.build) != pppBuildCalls {
			t.Fatalf("the %s ran go %d times, want %d: %v", one.half, len(one.build), pppBuildCalls, one.build)
		}
		for _, tag := range []string{"ze_core", "ze_distro", "ze_bgp", "ze_l2tp"} {
			if !strings.Contains(one.build[0], tag) {
				t.Errorf("the %s built without %s: %s", one.half, tag, one.build[0])
			}
		}
		if !strings.Contains(one.build[0], "<root>/tmp/evidence/bin/ze-l2tp-ppp") {
			t.Errorf("the %s built somewhere else: %s", one.half, one.build[0])
		}
	}
}

// VALIDATES: both halves start ze under the five settings the proof configures
// it with, three of which are dot-notation names a shell cannot hold.
// PREVENTS: a port that drops one of the settings and changes what the test proves.
// Without the NCP timeouts, the test would wait out a stalled IPCP instead of reporting it.
// Without ipv6cp off, the run would wait for a protocol that its address pool never configures.
//
// This case deliberately says NOTHING about the kernel-probe escape. Both
// halves refuse a run with the escape before they start the daemon.
// TestBothHalvesRefuseTheKernelProbeEscape proves that behavior. The STRIP after
// that refusal is unreachable from a full run. Thus, the case that tests it
// calls the builder directly:
// TestTheDaemonEnvironmentStripsTheEscapeAndCarriesTheSettings in
// internal/le/deployment. This run never sets the escape, so an absence assertion
// here cannot fail.
func TestBothHalvesHandTheDaemonTheSameSettings(t *testing.T) {
	requireNamespaceSupport(t)

	script := runPPPScript(t)
	command := runPPPCommand(t)

	for _, one := range []struct {
		half    string
		entries []string
	}{{"script", script.daemonEnv}, {"command", command.daemonEnv}} {
		joined := strings.Join(one.entries, "\n")
		if joined == "" {
			t.Fatalf("the %s never started ze, so this case proves nothing", one.half)
		}
		for _, want := range []string{
			"ZE_LOG_L2TP=debug",
			"ZE_STORAGE_BLOB=false",
			"ze.l2tp.ncp.enable-ipv6cp=false",
			"ze.l2tp.ncp.ip-timeout=15s",
			"ze.l2tp.auth.timeout=15s",
		} {
			if !strings.Contains(joined, want) {
				t.Errorf("the %s did not hand the daemon %q:\n%s", one.half, want, joined)
			}
		}
	}
}

// VALIDATES: a run started with either spelling of the kernel-probe escape is
// refused by both halves, before anything is built.
// PREVENTS: a machine-wide export making every run of this gate pass over a
// user-space path.
func TestBothHalvesRefuseTheKernelProbeEscape(t *testing.T) {
	requireNamespaceSupport(t)

	for _, key := range []string{"ZE_L2TP_SKIP_KERNEL_PROBE", "ze.l2tp.skip-kernel-probe"} {
		script := runPPPScript(t, key+"=true")
		if script.code == 0 {
			t.Errorf("the script ran with %s set", key)
		}
		if len(script.build) != 0 {
			t.Errorf("the script built a daemon before refusing %s: %v", key, script.build)
		}

		command := runPPPCommand(t, key+"=true")
		if command.code == 0 {
			t.Errorf("the port ran with %s set", key)
		}
		if len(command.build) != 0 {
			t.Errorf("the port built a daemon before refusing %s: %v", key, command.build)
		}
	}
}

// VALIDATES: the port refuses an interface with the WRONG addresses, but the
// script accepts it.
// PREVENTS: loss of this difference. verify_ppp_address in
// effective-l2tp-ppp.py tests each address as a SUBSTRING of the listing. The
// proof pool reaches 10.100.0.10, which contains gateway 10.100.0.1 as a
// substring. Thus, an interface on the wrong end of the pool satisfies the
// script assertion. That assertion cannot fail alone. Only the port fixes this
// defect (plan/journal/green-that-could-not-have-been-red.md). When somebody
// fixes the script, this case fails and must be deleted with the script.
func TestThePortRefusesTheAddressPairTheScriptAcceptsAsASubstring(t *testing.T) {
	requireNamespaceSupport(t)

	shifted := []string{
		"ZE_PPP_ADDR_ZE=" + pppZeShiftedAddrLine,
		"ZE_PPP_ADDR_LAC=" + pppLACShiftedAddrLine,
	}

	script := runPPPScript(t, shifted...)
	if script.code != 0 {
		t.Fatalf("the script now refuses an address pair it never had (exit %d); delete this case with the script", script.code)
	}
	if !anyPPPCall(script.ip, "ip -o addr show dev ppp0") {
		t.Fatal("the script no longer asks for the address listing, so this case proves nothing")
	}

	command := runPPPCommand(t, shifted...)
	if command.code == 0 {
		t.Error("the port reported a pass over an interface carrying neither address")
	}
	if anyPPPCall(command.ip, "ping -c 2 -W 3") {
		t.Error("the port went on to the dataplane ping after an address assertion it should have failed")
	}
}
