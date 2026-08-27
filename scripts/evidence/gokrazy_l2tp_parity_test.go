package main

// AC-11 for the appliance L2TP PPP gate: effective-gokrazy-l2tp-ppp.py and
// `le deployment gokrazy-l2tp-ppp-test` do the same thing.
//
// A Python script and a Go command share no process, so one test cannot call
// both directly. This proof instead compares their effects on the machine. The
// effects cover the kernel package, gokrazy instance, image build, and virtual
// machine. They also cover the bridge, TAP, veth pair, namespace, DHCP server,
// peer, host-kernel queries, and firewall hole. Recording stand-ins replace ip,
// make, qemu, dnsmasq, ufw, modprobe, and xl2tpd over a fixture checkout. The
// test compares the argv sent to each stand-in and the bytes that each wrote.
//
// THE STAND-INS PLAY THE APPLIANCE.
// make writes an image at the requested build path.
// qemu then prints the appliance serial console in order.
// The appliance boots, binds its listener, waits for the peer, and reports the session.
//
// It reports the withdraw after the peer leaves.
// Without this sequence, both halves stop at the first wait.
// A test that compares two runs stopped at the build proves nothing.
//
// Both halves run inside a user namespace for the reason stated by the on-host
// proof in l2tp_ppp_parity_test.go. The proof correctly refuses to start without
// CAP_NET_ADMIN and PPPoL2TP evidence.
//
// This file lives beside the script rather than beside the port, so that step 14
// deletes the script and its parity proof in one commit.

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/deployment"
)

// gokChildEnv marks the re-executed test binary that runs the port inside the
// namespace, and gokRootEnv names the fixture it is pointed at.
const (
	gokChildEnv = "ZE_GOKRAZY_L2TP_PARITY_CHILD"
	gokRootEnv  = "ZE_GOKRAZY_L2TP_PARITY_ROOT"
)

// gokKernelVersion is the release the fixture checkout pins and the fixture
// kernel package carries. The two must agree, or the run leaves the staged
// package and starts a kernel build.
const gokKernelVersion = "6.12.1"

// The recording ip returns this listing for the peer namespace after the
// appliance reports an active session. It contains the address pair on the peer
// interface.
const (
	gokPeerLink = "4: ppp1: <POINTOPOINT,MULTICAST,NOARP,UP,LOWER_UP> mtu 1400 state UNKNOWN"
	gokPeerAddr = `4: ppp1    inet 10.100.0.2 peer 10.100.0.1/32 scope global ppp1`
)

// The three files the peer reads, and the appliance configuration the image is
// built carrying.
var (
	gokPeerFiles    = []string{"xl2tpd.conf", "l2tp-secrets", "ppp-options"}
	gokTemplateFile = "ze-gokrazy-l2tp.conf"
	gokInstanceFile = "config.json"
)

// gokIPStub records every ip call and plays the host kernel.
//
// It replaces the on-host proof with one namespace instead of two. The virtual
// machine contains the appliance kernel, which this process cannot inspect.
// Thus, only the peer link listing returns data.
const gokIPStub = `#!/bin/bash
` + pppRecordFn + `
record "$ZE_RECORD_IP" "$@"
state="$ZE_PPP_STATE"

if [ "$1" = "netns" ] && [ "$2" = "exec" ]; then
  shift 3
  case "$1" in
    ip)
      case "$*" in
        "ip -o link show type ppp")
          if [ -f "$state/ppp-up" ]; then echo "$ZE_GOK_PEER_LINK"; fi
          exit 0 ;;
        "ip -o addr show dev "*) printf '%s\n' "$ZE_GOK_PEER_ADDR" ; exit 0 ;;
        *) exit 0 ;;
      esac ;;
    ping) exit 0 ;;
    cat) exit 0 ;;
    *) exec "$@" ;;
  esac
fi
exit 0
`

// gokUfwStub records the firewall calls and reports a firewall that is off.
//
// The value is off because that is the answer from a build machine.
// Both halves must agree on it.
// A stand-in that returned "active" would add a rule.
// This test would not then observe whether the system honored that rule.
const gokUfwStub = `#!/bin/bash
` + pppRecordFn + `
record "$ZE_RECORD_UFW" "$@"
case "$1" in
  status) echo "Status: inactive" ;;
esac
exit 0
`

// gokDNSMasqStub records the DHCP server and exits, because the appliance it
// would serve is itself a stand-in.
const gokDNSMasqStub = `#!/bin/bash
` + pppRecordFn + `
record "$ZE_RECORD_DNSMASQ" "$@"
exit 0
`

// gokMakeStub records the build, keeps what the build was handed, and writes an
// image where it was asked for.
//
// The two copies let the comparison inspect the appliance configuration. A
// successful run removes its scratch directory. Without the copies, the
// template and patched instance would be gone before this test inspected them.
const gokMakeStub = `#!/bin/bash
` + pppRecordFn + `
record "$ZE_RECORD_MAKE" "$@"
mkdir -p "$ZE_RECORD_INPUTS"
img=""
for a in "$@"; do
  case "$a" in
    GOKRAZY_IMG=*) img=${a#GOKRAZY_IMG=} ;;
    GOKRAZY_TEMPLATE=*) cp "${a#GOKRAZY_TEMPLATE=}" "$ZE_RECORD_INPUTS/ze-gokrazy-l2tp.conf" ;;
    GOKRAZY_DIR=*) cp "${a#GOKRAZY_DIR=}/ze/config.json" "$ZE_RECORD_INPUTS/config.json" ;;
  esac
done
[ -n "$img" ] && { mkdir -p "$(dirname "$img")"; : > "$img"; }
exit 0
`

// gokQemuStub records the virtual machine and plays the appliance's serial
// console.
//
// It announces each step only after its cause occurs.
// The session line waits for the peer to dial, and the withdraw line waits for the peer to leave.
// A stand-in that prints all lines immediately CAN let both halves finish before the peer records its argv.
// Both recordings would then match and omit the peer.
const gokQemuStub = `#!/bin/bash
` + pppRecordFn + `
record "$ZE_RECORD_QEMU" "$@"
state="$ZE_PPP_STATE"
mkdir -p "$state"
echo "web server listening on 0.0.0.0:8080"
echo "l2tp: L2TP listener bound on 0.0.0.0:1701"
n=0
while [ ! -f "$state/peer-started" ] && [ $n -lt 400 ]; do sleep 0.05; n=$((n+1)); done
echo "l2tp: session established (incoming LNS) tunnel-id=1 session-id=1"
: > "$state/ppp-up"
echo "l2tp: session IP assigned tunnel-id=1 session-id=1 username=alice address=10.100.0.2"
echo "l2tp: subscriber route inject prefix=10.100.0.2/32 nexthop=ppp0"
echo "l2tp: PPP session up tunnel-id=1 session-id=1 interface=ppp0"
n=0
while [ ! -f "$state/peer-stopped" ] && [ $n -lt 1200 ]; do sleep 0.05; n=$((n+1)); done
rm -f "$state/ppp-up"
echo "l2tp: subscriber routes withdrawn tunnel-id=1 session-id=1"
sleep 120
`

// gokPeerStub records the peer argv and keeps the three input files. It also
// tells the console stand-in when the peer dials and leaves.
const gokPeerStub = `#!/bin/bash
` + pppRecordFn + `
record "$ZE_RECORD_PEER" "$@"
mkdir -p "$ZE_RECORD_INPUTS"
prev=""
for a in "$@"; do
  if [ "$prev" = "-c" ]; then
    dir=$(dirname "$a")
    for f in xl2tpd.conf l2tp-secrets ppp-options; do
      [ -f "$dir/$f" ] && cp "$dir/$f" "$ZE_RECORD_INPUTS/$f"
    done
  fi
  prev="$a"
done
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

// gokRun is what one half of the comparison did.
type gokRun struct {
	code     int
	ip       []string
	modprobe []string
	make     []string
	qemu     []string
	peer     []string
	ufw      []string
	dnsmasq  []string
	inputs   map[string]string
}

// gokFixture builds a checkout for each half. It contains the module marker,
// pinned kernel version, kernel package, and gokrazy instance. The instance is
// the patch target, and the checkout also contains a script copy.
func gokFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.test/m\n\ngo 1.26\n",
		filepath.Join("internal", "appliance", "kernel.version"): gokKernelVersion + "\n",
		filepath.Join("gokrazy", "ze", "config.json"): `{
    "Hostname": "ze",
    "PackageConfig": {
        "github.com/ze-software/ze/cmd/ze": {
            "Environment": ["ZE_LOG_LEVEL=info"]
        }
    }
}
`,
		filepath.Join("gokrazy", "ze", "builddir", "keep"): "the build directory the instance links to\n",
	}
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create the fixture directory for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write the fixture %s: %v", rel, err)
		}
	}

	gokKernelPackage(t, filepath.Join(root, "tmp", "kernel", "pkg"))

	dir := filepath.Join(root, "scripts", "evidence")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create the fixture script directory: %v", err)
	}
	body, err := os.ReadFile("effective-gokrazy-l2tp-ppp.py")
	if err != nil {
		t.Fatalf("read the script: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "effective-gokrazy-l2tp-ppp.py"), body, 0o600); err != nil {
		t.Fatalf("copy the script into the fixture: %v", err)
	}
	return root
}

// gokKernelPackage writes a kernel package that both halves accept. It contains
// an amd64 image with the header bytes that both halves read. Its module tree
// has PPPoL2TP built into the pinned release.
//
// The package is VALID by design. A rejected package sends both halves to the
// cache probe and kernel build. That path tests different behavior and is much
// slower. Separate cases assert the rejection paths.
func gokKernelPackage(t *testing.T, pkg string) {
	t.Helper()

	const amd64Offset = 0x202
	const amd64Magic = "HdrS"
	image := make([]byte, amd64Offset+len(amd64Magic))
	copy(image[amd64Offset:], amd64Magic)

	if err := os.MkdirAll(pkg, 0o750); err != nil {
		t.Fatalf("create the fixture kernel package: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "vmlinuz"), image, 0o600); err != nil {
		t.Fatalf("write the fixture vmlinuz: %v", err)
	}

	modules := filepath.Join(pkg, "lib", "modules", gokKernelVersion)
	if err := os.MkdirAll(modules, 0o750); err != nil {
		t.Fatalf("create the fixture module tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modules, "modules.builtin"),
		[]byte("kernel/net/l2tp/l2tp_ppp.ko\n"), 0o600); err != nil {
		t.Fatalf("write the fixture modules.builtin: %v", err)
	}
}

// gokStubs writes the recording programs and answers their directory.
func gokStubs(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	stubs := map[string]string{
		"ip":                 gokIPStub,
		"make":               gokMakeStub,
		"qemu-system-x86_64": gokQemuStub,
		"xl2tpd":             gokPeerStub,
		"ufw":                gokUfwStub,
		"dnsmasq":            gokDNSMasqStub,
		"modprobe":           pppModprobeStub,
		"ping":               pppToolStub,
		"pppd":               pppToolStub,
	}
	for name, body := range stubs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
			t.Fatalf("write the %s stub: %v", name, err)
		}
	}
	return dir
}

// gokEnv is the environment both halves are run under.
func gokEnv(stubDir, record, state string, extra ...string) []string {
	base := []string{
		"PATH=" + stubDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"ZE_RECORD_IP=" + filepath.Join(record, "ip"),
		"ZE_RECORD_MAKE=" + filepath.Join(record, "make"),
		"ZE_RECORD_QEMU=" + filepath.Join(record, "qemu"),
		"ZE_RECORD_PEER=" + filepath.Join(record, "peer"),
		"ZE_RECORD_UFW=" + filepath.Join(record, "ufw"),
		"ZE_RECORD_DNSMASQ=" + filepath.Join(record, "dnsmasq"),
		"ZE_RECORD_MODPROBE=" + filepath.Join(record, "modprobe"),
		"ZE_RECORD_INPUTS=" + filepath.Join(record, "inputs"),
		"ZE_PPP_STATE=" + state,
		"ZE_GOK_PEER_LINK=" + gokPeerLink,
		"ZE_GOK_PEER_ADDR=" + gokPeerAddr,
		// The accelerator is FIXED instead of derived. Otherwise, the comparison
		// would depend on whether each half can open /dev/kvm.
		"ZE_GOKRAZY_QEMU_ACCEL=tcg",
		"ZE_GOKRAZY_ARCH=amd64",
	}
	return append(base, extra...)
}

// runGokScript runs the Python original inside the namespace.
func runGokScript(t *testing.T, extra ...string) gokRun {
	t.Helper()

	root := gokFixture(t)
	argv := pppNamespaceArgv(pppFakeMounts(t), "python3",
		filepath.Join(root, "scripts", "evidence", "effective-gokrazy-l2tp-ppp.py"))
	return gokExecute(t, root, argv, extra, nil)
}

// runGokCommand runs the ported command inside the namespace, by re-executing
// this test binary there.
func runGokCommand(t *testing.T, extra ...string) gokRun {
	t.Helper()

	root := gokFixture(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("find this test binary: %v", err)
	}
	argv := pppNamespaceArgv(pppFakeMounts(t), self, "-test.run=^TestTheGokrazyL2TPParityChildRunsThePort$")
	child := []string{gokChildEnv + "=1", gokRootEnv + "=" + root}
	return gokExecute(t, root, argv, extra, child)
}

// gokExecute runs one half and answers what it recorded.
func gokExecute(t *testing.T, root string, argv, extra, child []string) gokRun {
	t.Helper()

	record := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), pppHalfTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // an argv this test built
	cmd.Dir = root
	cmd.Env = append(os.Environ(), gokEnv(gokStubs(t), record, t.TempDir(), extra...)...)
	cmd.Env = append(cmd.Env, child...)
	out, err := cmd.CombinedOutput()

	return gokRun{
		code:     pppExitCode(t, err, out),
		ip:       gokNormalize(calls(t, filepath.Join(record, "ip")), root),
		modprobe: gokNormalize(calls(t, filepath.Join(record, "modprobe")), root),
		make:     gokNormalize(calls(t, filepath.Join(record, "make")), root),
		qemu:     gokNormalize(calls(t, filepath.Join(record, "qemu")), root),
		peer:     gokNormalize(calls(t, filepath.Join(record, "peer")), root),
		ufw:      gokNormalize(calls(t, filepath.Join(record, "ufw")), root),
		dnsmasq:  gokNormalize(calls(t, filepath.Join(record, "dnsmasq")), root),
		inputs:   gokInputs(t, root, filepath.Join(record, "inputs")),
	}
}

// TestTheGokrazyL2TPParityChildRunsThePort is the re-executed half of the
// comparison. Outside the namespace it does nothing at all.
func TestTheGokrazyL2TPParityChildRunsThePort(t *testing.T) { //nolint:unparam // the signature is the framework's, and this half never reports through it
	if os.Getenv(gokChildEnv) != "1" {
		return
	}

	proof := deployment.NewGokrazyL2TP(os.Getenv(gokRootEnv))
	proof.Progress = io.Discard
	proof.Lab.Progress = io.Discard
	// The stand-ins answer within milliseconds, so the real bounds would only
	// slow a broken run down.
	const bound = 20 * time.Second
	proof.BootWait = bound
	proof.ListenerWait = bound
	proof.AddressWait = bound
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

// gokInputs answers the appliance configuration, the patched instance and the
// peer's three files, out of what the stand-ins kept.
func gokInputs(t *testing.T, root, dir string) map[string]string {
	t.Helper()

	files := map[string]string{}
	wanted := append([]string{gokTemplateFile, gokInstanceFile}, gokPeerFiles...)
	for _, name := range wanted {
		body, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // a path this test's own run wrote
		if err != nil {
			files[name] = "<absent>"
			continue
		}
		files[name] = strings.Join(gokNormalize([]string{string(body)}, root), "")
	}
	return files
}

// The names each half derives from its own process id, and the two scratch
// directories each made.
var gokDerivedNames = []struct {
	pattern *regexp.Regexp
	with    string
}{
	{regexp.MustCompile(`ze-gokrazy-lac-\d+`), "<lac-ns>"},
	{regexp.MustCompile(`zebr\d+`), "<bridge>"},
	{regexp.MustCompile(`zetap\d+`), "<tap>"},
	{regexp.MustCompile(`zgokh\d+`), "<host-veth>"},
	{regexp.MustCompile(`zgokl\d+`), "<lac-veth>"},
	{regexp.MustCompile(`ze-l2tp-dnsmasq-\d+`), "<dnsmasq>"},
	{regexp.MustCompile(`gokrazy-l2tp-ppp-[A-Za-z0-9_]+`), "<work>"},
	{regexp.MustCompile(`zel2tp-[A-Za-z0-9_]+`), "<peer-dir>"},
}

// gokNormalize removes the tree each half was pointed at and every name each
// derived from its own process id.
func gokNormalize(lines []string, root string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.ReplaceAll(line, root, "<root>")
		for _, one := range gokDerivedNames {
			line = one.pattern.ReplaceAllString(line, one.with)
		}
		out = append(out, line)
	}
	return out
}

// The absolute counts a full run owes, stated rather than only compared.
//
// A full run makes 42 ip calls.
// They include one Generic Netlink probe and two lab removals of seven calls each.
// They also include sixteen lab-build commands, three baseline questions, one appliance ping, and one peer start.
// The last six calls are three kernel assertions and three teardown questions.
//
// The run also makes five modprobe calls.
// It makes one call each to the image build, virtual machine, peer, and DHCP server.
// And it makes three firewall calls.
//
// The counts are STATED because two recordings that agree on nothing compare
// equal: this area has produced that failure twice.
const (
	gokIPCalls       = 42
	gokModprobeCalls = 5
	gokMakeCalls     = 1
	gokQemuCalls     = 1
	gokPeerCalls     = 1
	gokUfwCalls      = 3
	gokDNSMasqCalls  = 1
)

// VALIDATES: both halves reach the same verdict and validate the same kernel package.
// They patch the instance identically and start the same image build.
// They build the same lab, boot the same virtual machine, and start the same peer.
// They ask the host kernel the same questions in the same order.
// Both halves also leave the same files.
// PREVENTS: a port that changes what the proof does when the one OK line does not show the change.
func TestScriptAndCommandProveTheApplianceL2TPPathTheSameWay(t *testing.T) {
	requireNamespaceSupport(t)

	script := runGokScript(t)
	command := runGokCommand(t)

	if script.code != 0 || command.code != 0 {
		t.Fatalf("the run answered script=%d command=%d, want 0 and 0:\nscript ip calls:\n  %s\ncommand ip calls:\n  %s",
			script.code, command.code, strings.Join(script.ip, "\n  "), strings.Join(command.ip, "\n  "))
	}

	for _, one := range []struct {
		half string
		run  gokRun
	}{{"script", script}, {"command", command}} {
		counted := []struct {
			what string
			got  int
			want int
		}{
			{"ip", len(one.run.ip), gokIPCalls},
			{"modprobe", len(one.run.modprobe), gokModprobeCalls},
			{"make", len(one.run.make), gokMakeCalls},
			{"qemu", len(one.run.qemu), gokQemuCalls},
			{"xl2tpd", len(one.run.peer), gokPeerCalls},
			{"ufw", len(one.run.ufw), gokUfwCalls},
			{"dnsmasq", len(one.run.dnsmasq), gokDNSMasqCalls},
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
	sameCalls(t, "make", script.make, command.make)
	sameCalls(t, "qemu", script.qemu, command.qemu)
	sameCalls(t, "xl2tpd", script.peer, command.peer)
	sameCalls(t, "ufw", script.ufw, command.ufw)
	sameCalls(t, "dnsmasq", script.dnsmasq, command.dnsmasq)

	// The appliance template and the peer's files are compared byte for byte.
	// The instance is compared as a DOCUMENT rather than as bytes.
	// A build parses the instance as JSON.
	// Go sorts the object's keys, but Python keeps the file's order.
	for _, name := range append([]string{gokTemplateFile}, gokPeerFiles...) {
		if script.inputs[name] == "<absent>" {
			t.Errorf("neither half can be compared on %s: the script wrote none", name)
			continue
		}
		if script.inputs[name] != command.inputs[name] {
			t.Errorf("the input file %s differs:\nscript:\n%s\ncommand:\n%s",
				name, script.inputs[name], command.inputs[name])
		}
	}
	sameDocument(t, gokInstanceFile, script.inputs[gokInstanceFile], command.inputs[gokInstanceFile])
}

// sameDocument compares two JSON documents by what they MEAN.
//
// A byte comparison would be stronger, but it would be wrong here.
// The consumer is a JSON parser.
// Both halves order an object's keys differently for a reason that neither half chose.
func sameDocument(t *testing.T, name, script, command string) {
	t.Helper()

	var left, right any
	if err := json.Unmarshal([]byte(script), &left); err != nil {
		t.Fatalf("the script's %s is not JSON: %v\n%s", name, err, script)
	}
	if err := json.Unmarshal([]byte(command), &right); err != nil {
		t.Fatalf("the command's %s is not JSON: %v\n%s", name, err, command)
	}
	if !reflect.DeepEqual(left, right) {
		t.Errorf("the %s differs:\nscript:\n%s\ncommand:\n%s", name, script, command)
	}
}

// VALIDATES: every step of the appliance path is actually walked, by naming the
// command each of them owes.
// PREVENTS: the trap this comparison is most exposed to. Two recordings that
// agree on nothing prove nothing, and a port that stopped at the image build
// would still match a script that did the same.
func TestBothHalvesWalkEveryStepOfTheApplianceL2TPPath(t *testing.T) {
	requireNamespaceSupport(t)

	script := runGokScript(t)
	command := runGokCommand(t)

	owed := []struct {
		what   string
		pick   func(gokRun) []string
		needle string
	}{
		{"the image build", func(r gokRun) []string { return r.make }, "ze-gokrazy-build"},
		{"the kernel handed to the build", func(r gokRun) []string { return r.make }, "KERNEL_PKG=<root>/tmp/evidence/<work>/kernel-pkg"},
		{"the bridge", func(r gokRun) []string { return r.ip }, "link add name <bridge> type bridge forward_delay 0"},
		{"the tap", func(r gokRun) []string { return r.ip }, "tuntap add dev <tap> mode tap"},
		{"the veth pair", func(r gokRun) []string { return r.ip }, "link add <host-veth> type veth peer name <lac-veth>"},
		{"the DHCP reservation", func(r gokRun) []string { return r.dnsmasq }, "--dhcp-host=52:54:00:12:34:56,172.31.0.10"},
		{"the virtual machine", func(r gokRun) []string { return r.qemu }, "tap,id=net0,ifname=<tap>,script=no,downscript=no"},
		{"the appliance ping", func(r gokRun) []string { return r.ip }, "ping -c 1 -W 1 172.31.0.10"},
		// The peer's directory is under the machine's temporary root, not under the checkout.
		// Thus, the needle names the directory and file instead of the whole argument.
		{"the peer in the foreground", func(r gokRun) []string { return r.peer }, "-D -c "},
		{"the peer's configuration", func(r gokRun) []string { return r.peer }, "<peer-dir>/xl2tpd.conf"},
		{"the peer's address listing", func(r gokRun) []string { return r.ip }, "ip -o addr show dev ppp1"},
		{"the dataplane ping", func(r gokRun) []string { return r.ip }, "ping -c 2 -W 3 10.100.0.1"},
		{"the namespace removal", func(r gokRun) []string { return r.ip }, "netns delete <lac-ns>"},
	}
	for _, one := range []struct {
		half string
		run  gokRun
	}{{"script", script}, {"command", command}} {
		for _, step := range owed {
			calls := step.pick(one.run)
			if anyPPPCall(calls, step.needle) {
				continue
			}
			t.Errorf("the %s never reached %s (%q):\n  %s",
				one.half, step.what, step.needle, strings.Join(calls, "\n  "))
		}
	}
}

// VALIDATES: the kernel package handed to the build is a per-run COPY rather
// than the shared staged path.
// PREVENTS: the race the copy exists for. The staged path is rewritten by any
// concurrent kernel build, starting with a removal, and the image build reads
// the package minutes after it was validated.
func TestBothHalvesHandTheBuildAPerRunKernelCopy(t *testing.T) {
	requireNamespaceSupport(t)

	script := runGokScript(t)
	command := runGokCommand(t)

	for _, one := range []struct {
		half  string
		calls []string
	}{{"script", script.make}, {"command", command.make}} {
		if len(one.calls) != gokMakeCalls {
			t.Fatalf("the %s ran make %d times, want %d: %v", one.half, len(one.calls), gokMakeCalls, one.calls)
		}
		if strings.Contains(one.calls[0], "KERNEL_PKG=<root>/tmp/kernel/pkg") {
			t.Errorf("the %s handed the build the shared staged package: %s", one.half, one.calls[0])
		}
		if !strings.Contains(one.calls[0], "kernel-pkg") {
			t.Errorf("the %s handed the build no per-run kernel copy: %s", one.half, one.calls[0])
		}
	}
}

// VALIDATES: both halves add this proof's three settings to the instance and
// keep the setting the checked-in instance already carried.
// PREVENTS: the boundary defect this proof exists to catch. A setting that does
// not reach the appliance's daemon makes the run fail inside a negotiation,
// where the reason reads as a protocol failure.
func TestBothHalvesPatchTheInstanceTheSameWay(t *testing.T) {
	requireNamespaceSupport(t)

	script := runGokScript(t)
	command := runGokCommand(t)

	for _, one := range []struct {
		half string
		body string
	}{{"script", script.inputs[gokInstanceFile]}, {"command", command.inputs[gokInstanceFile]}} {
		if one.body == "<absent>" {
			t.Fatalf("the %s patched no instance, so this case proves nothing", one.half)
		}
		for _, want := range []string{
			"ZE_LOG_LEVEL=info",
			"ze.l2tp.ncp.enable-ipv6cp=false",
			"ze.l2tp.ncp.ip-timeout=15s",
			"ze.l2tp.auth.timeout=15s",
		} {
			if !strings.Contains(one.body, want) {
				t.Errorf("the %s instance lacks %q:\n%s", one.half, want, one.body)
			}
		}
	}
}

// VALIDATES: the port refuses a peer interface with the WRONG addresses, but the
// script accepts it.
// PREVENTS: loss of this difference. verify_ppp_address in
// effective-gokrazy-l2tp-ppp.py tests each address as a SUBSTRING of the
// listing. The proof pool reaches 10.100.0.10, which contains gateway
// 10.100.0.1 as a substring. This is the SECOND script with that defect. Thus,
// the journal row identifies a script habit instead of one author's error. Only
// the port fixes it (plan/journal/green-that-could-not-have-been-red.md). This
// case fails when somebody fixes the script and must then be deleted with the
// script.
func TestTheAppliancePortRefusesTheAddressPairTheScriptAcceptsAsASubstring(t *testing.T) {
	requireNamespaceSupport(t)

	// The listing moved one step up the pool: neither address the proof asks
	// for is on the interface, and both are substrings of one that is.
	const shifted = `4: ppp1    inet 10.100.0.20 peer 10.100.0.10/32 scope global ppp1`

	script := runGokScript(t, "ZE_GOK_PEER_ADDR="+shifted)
	if script.code != 0 {
		t.Fatalf("the script now refuses an address pair it never had (exit %d); delete this case with the script", script.code)
	}
	if !anyPPPCall(script.ip, "ip -o addr show dev ppp1") {
		t.Fatal("the script no longer asks for the address listing, so this case proves nothing")
	}

	command := runGokCommand(t, "ZE_GOK_PEER_ADDR="+shifted)
	if command.code == 0 {
		t.Error("the port reported a pass over a peer interface carrying neither address")
	}
	if anyPPPCall(command.ip, "ping -c 2 -W 3") {
		t.Error("the port went on to the dataplane ping after an address assertion it should have failed")
	}
}

// VALIDATES: a run started with either spelling of the kernel-probe escape is
// refused by both halves, before anything is built.
// PREVENTS: a machine-wide export making every run of this gate pass over an
// appliance whose daemon never touched the kernel.
func TestBothHalvesOfTheApplianceProofRefuseTheKernelProbeEscape(t *testing.T) {
	requireNamespaceSupport(t)

	for _, key := range []string{"ZE_L2TP_SKIP_KERNEL_PROBE", "ze.l2tp.skip-kernel-probe"} {
		script := runGokScript(t, key+"=true")
		if script.code == 0 {
			t.Errorf("the script ran with %s set", key)
		}
		if len(script.make) != 0 {
			t.Errorf("the script started a build before refusing %s: %v", key, script.make)
		}

		command := runGokCommand(t, key+"=true")
		if command.code == 0 {
			t.Errorf("the port ran with %s set", key)
		}
		if len(command.make) != 0 {
			t.Errorf("the port started a build before refusing %s: %v", key, command.make)
		}
	}
}
