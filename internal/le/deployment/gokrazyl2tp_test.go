package deployment

// The appliance L2TP PPP proof, driven as functions.
//
// Goal: pin down what the argv comparison beside the script cannot reach. This
// includes the kernel package gate, the instance patch, the virtual machine's
// own arguments, and the lab's names. Method: build fixture packages and
// configurations on disk, then read what each judgement answers. The tests use
// no QEMU and no appliance anywhere.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// The bytes each architecture's kernel image is recognized by, and where.
const (
	amd64Magic  = "HdrS"
	amd64Offset = 0x202
	arm64Magic  = "ARMd"
	arm64Offset = 0x38
)

// kernelFixture writes a kernel package and answers it.
//
// release names the module directory, builtin is what modules.builtin holds,
// and loadable asks for a compressed l2tp_ppp module beside it. A magic of the
// empty string writes a vmlinuz that is not a kernel at all.
func kernelFixture(t *testing.T, release, builtin, magic string, offset int64, loadable bool) string {
	t.Helper()

	pkg := t.TempDir()
	if magic != "" {
		image := make([]byte, offset+int64(len(magic)))
		copy(image[offset:], magic)
		if err := os.WriteFile(filepath.Join(pkg, "vmlinuz"), image, 0o600); err != nil {
			t.Fatalf("write the fixture vmlinuz: %v", err)
		}
	}

	modules := filepath.Join(pkg, "lib", "modules", release)
	if err := os.MkdirAll(modules, 0o750); err != nil {
		t.Fatalf("write the fixture module tree: %v", err)
	}
	if builtin != "" {
		if err := os.WriteFile(filepath.Join(modules, modulesBuiltin), []byte(builtin), 0o600); err != nil {
			t.Fatalf("write the fixture modules.builtin: %v", err)
		}
	}
	if loadable {
		deep := filepath.Join(modules, "kernel", "net", "l2tp")
		if err := os.MkdirAll(deep, 0o750); err != nil {
			t.Fatalf("write the fixture module directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(deep, "l2tp_ppp.ko.zst"), []byte("module"), 0o600); err != nil {
			t.Fatalf("write the fixture module: %v", err)
		}
	}
	return pkg
}

// VALIDATES: a package with the right architecture, PPPoL2TP built in, and the
// pinned release is accepted.
// PREVENTS: a gate so strict that the package this repository builds is refused
// by it, which would make every run of the proof fail on a working kernel.
func TestAKernelPackageThatCanCarryTheProofIsAccepted(t *testing.T) {
	pkg := kernelFixture(t, "6.12.1", "kernel/net/l2tp/l2tp_ppp.ko\n", amd64Magic, amd64Offset, false)

	problems, err := kernelPackageProblems(pkg, ArchAMD64, "6.12.1")
	if err != nil {
		t.Fatalf("judge the package: %v", err)
	}
	if len(problems) != 0 {
		t.Errorf("a usable package was refused: %v", problems)
	}
}

// VALIDATES: The gate reports each of the three things that make a package
// unusable, and it reports each one by name.
// PREVENTS: The failure that this gate exists to stop. ze's L2TP probe is
// fail-closed, so an appliance on a kernel with no PPPoL2TP crash-loops. Without
// this gate, the proof reports "the web server did not start". That report sends
// the reader to the appliance rather than the kernel.
func TestAKernelPackageIsRefusedForEachReasonSeparately(t *testing.T) {
	cases := []struct {
		name string
		pkg  string
		want string
	}{
		{
			"an arm64 kernel under an amd64 pin",
			kernelFixture(t, "6.12.1", "l2tp_ppp\n", arm64Magic, arm64Offset, false),
			"is not an amd64 kernel",
		},
		{
			"no PPPoL2TP anywhere",
			kernelFixture(t, "6.12.1", "kernel/net/ipv4/tunnel4.ko\n", amd64Magic, amd64Offset, false),
			"has no PPPoL2TP support",
		},
		{
			"a module tree for another release",
			kernelFixture(t, "6.10.9", "l2tp_ppp\n", amd64Magic, amd64Offset, false),
			"not the pinned kernel version",
		},
		{
			"no kernel image at all",
			kernelFixture(t, "6.12.1", "l2tp_ppp\n", "", 0, false),
			"no vmlinuz at",
		},
	}
	for _, one := range cases {
		problems, err := kernelPackageProblems(one.pkg, ArchAMD64, "6.12.1")
		if err != nil {
			t.Fatalf("%s: judge the package: %v", one.name, err)
		}
		if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, one.want) }) {
			t.Errorf("%s: the problems do not carry %q: %v", one.name, one.want, problems)
		}
	}
}

// VALIDATES: a loadable module satisfies the PPPoL2TP requirement, whatever it
// is compressed with, and a release suffixed by a local build still matches the
// pin.
// PREVENTS: a gate that only accepts the kernel this repository builds. A
// distribution kernel ships l2tp_ppp as l2tp_ppp.ko.zst and names its release
// with a local suffix, and refusing it would forbid an operator's own package.
func TestALoadableModuleAndASuffixedReleaseAreAccepted(t *testing.T) {
	pkg := kernelFixture(t, "6.12.1-zelocal", "", amd64Magic, amd64Offset, true)

	problems, err := kernelPackageProblems(pkg, ArchAMD64, "6.12.1")
	if err != nil {
		t.Fatalf("judge the package: %v", err)
	}
	if len(problems) != 0 {
		t.Errorf("a package with a loadable module and a suffixed release was refused: %v", problems)
	}
}

// VALIDATES: an architecture this proof cannot boot is an error rather than a
// list of problems.
// PREVENTS: a caller reading "no problems" for a package nobody looked at,
// which is what an unknown architecture would otherwise produce.
func TestAnUnknownArchitectureIsAnErrorRatherThanASilentPass(t *testing.T) {
	pkg := kernelFixture(t, "6.12.1", "l2tp_ppp\n", amd64Magic, amd64Offset, false)

	if _, err := kernelPackageProblems(pkg, "riscv64", ""); err == nil {
		t.Error("an architecture with no known kernel magic was judged rather than refused")
	}
}

// VALIDATES: the instance patch adds this proof's three settings to ze's own
// package entry and leaves everything else in the file alone.
// PREVENTS: the boundary defect this proof exists to catch. A setting that does
// not reach the appliance's daemon makes the proof fail deep inside a
// negotiation, where the reason looks like a protocol failure.
func TestTheInstancePatchAddsTheProofSettingsAndNothingElse(t *testing.T) {
	source := filepath.Join(t.TempDir(), "config.json")
	original := `{
    "Hostname": "ze",
    "PackageConfig": {
        "github.com/ze-software/ze/cmd/ze": {
            "Environment": ["ZE_LOG_LEVEL=info"]
        },
        "github.com/gokrazy/serial-busybox": {}
    }
}`
	if err := os.WriteFile(source, []byte(original), 0o600); err != nil {
		t.Fatalf("write the fixture configuration: %v", err)
	}

	patched, err := instanceConfig(source)
	if err != nil {
		t.Fatalf("patch the configuration: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(patched, &config); err != nil {
		t.Fatalf("the patched configuration is not JSON: %v\n%s", err, patched)
	}
	if config["Hostname"] != "ze" {
		t.Errorf("the patch changed the hostname: %v", config["Hostname"])
	}

	packages, _ := config["PackageConfig"].(map[string]any)
	if _, kept := packages["github.com/gokrazy/serial-busybox"]; !kept {
		t.Error("the patch dropped another package's entry")
	}

	entry, _ := packages[ZePackage].(map[string]any)
	environment, _ := entry["Environment"].([]any)
	var got []string
	for _, item := range environment {
		if text, ok := item.(string); ok {
			got = append(got, text)
		}
	}
	if !slices.Contains(got, "ZE_LOG_LEVEL=info") {
		t.Errorf("the patch dropped a setting the instance already carried: %v", got)
	}
	for _, want := range ProofDaemonEnv {
		if !slices.Contains(got, want) {
			t.Errorf("the patch did not add %q: %v", want, got)
		}
	}
	if !strings.HasSuffix(string(patched), "\n") {
		t.Error("the patched configuration has no trailing newline")
	}
}

// VALIDATES: a setting the instance already carries is not added a second time.
// PREVENTS: two entries for one key, where which of them the daemon reads is
// the environment's own last-wins rule rather than anybody's decision.
func TestTheInstancePatchDoesNotDuplicateASettingAlreadyThere(t *testing.T) {
	source := filepath.Join(t.TempDir(), "config.json")
	original := `{"PackageConfig": {"github.com/ze-software/ze/cmd/ze": {"Environment": ["ze.l2tp.ncp.ip-timeout=90s"]}}}`
	if err := os.WriteFile(source, []byte(original), 0o600); err != nil {
		t.Fatalf("write the fixture configuration: %v", err)
	}

	patched, err := instanceConfig(source)
	if err != nil {
		t.Fatalf("patch the configuration: %v", err)
	}
	if count := strings.Count(string(patched), "ze.l2tp.ncp.ip-timeout="); count != 1 {
		t.Errorf("the key appears %d times, want 1:\n%s", count, patched)
	}
	if !strings.Contains(string(patched), "ze.l2tp.ncp.ip-timeout=90s") {
		t.Errorf("the patch overwrote the instance's own value:\n%s", patched)
	}
}

// VALIDATES: The virtual machine attaches its NIC to the lab's TAP. It boots the
// image that was built and puts its console on this process's streams.
// PREVENTS: The regression that the TAP exists to prevent. QEMU's user-mode
// networking does not deliver inbound UDP to the guest. An L2TP tunnel starts
// with an inbound SCCRQ. Therefore, a machine built with user-mode networking
// can never pass this proof.
func TestTheApplianceIsBootedOnTheLabsTap(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	run := newGokrazyL2TP(t.TempDir())
	argv, err := run.qemuArgs("/tmp/ze.img")
	if err != nil {
		t.Skipf("this machine has no qemu for %s: %v", run.Arch, err)
	}

	joined := strings.Join(argv, " ")
	for _, want := range []string{
		"tap,id=net0,ifname=" + run.Lab.Tap + ",script=no,downscript=no",
		"e1000,netdev=net0,mac=" + run.Lab.ApplianceMAC,
		"file=/tmp/ze.img,format=raw",
		"-serial mon:stdio",
		"-nographic",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the machine does not carry %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "hostfwd") || strings.Contains(joined, "user,") {
		t.Errorf("the machine uses user-mode networking, which cannot receive an SCCRQ: %s", joined)
	}
}

// VALIDATES: an architecture with no QEMU of its own is refused.
// PREVENTS: an argv built for a machine nobody can start, whose failure arrives
// as an exec error with no reason in it.
func TestAnUnbootableArchitectureIsRefused(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	run := newGokrazyL2TP(t.TempDir())
	run.Arch = "riscv64"
	if _, err := run.qemuArgs("/tmp/ze.img"); err == nil {
		t.Error("an architecture this proof cannot boot produced a machine")
	}
}

// applianceConsole answers a collector fed the appliance's session lines, with
// the session-up line the case wants.
func applianceConsole(t *testing.T, sessionUp string) *collector {
	t.Helper()

	console := newCollector(append([]string{
		gokrazyWebLine, pppListenerLine, pppSessionLine, pppIPLine,
		pppRouteLine, pppUpLine, pppWithdrawLine, pppTeardownLine,
	}, gokrazyFatalLines...)...)
	feed(t, console,
		"web server listening on 0.0.0.0:8080",
		"l2tp: L2TP listener bound on 0.0.0.0:1701",
		"l2tp: session established (incoming LNS) tunnel-id=1",
		"l2tp: session IP assigned tunnel-id=1 username=alice address=10.100.0.2",
		"l2tp: subscriber route inject prefix=10.100.0.2/32",
		sessionUp,
	)
	return console
}

// VALIDATES: The proof reads the appliance's own interface from its serial
// console. A console that names none is a failure rather than a blank field.
// PREVENTS: Without this validation, a report can name the appliance's interface
// as nothing at all over a session that the appliance never programmed. Nothing
// on the host can look inside the virtual machine. This line is the ONLY
// evidence about the appliance's own kernel objects. Its absence would leave
// that half of the proof unasserted.
func TestTheApplianceInterfaceIsReadOutOfItsConsole(t *testing.T) {
	withStateStub(t)
	t.Setenv("ZE_TEST_LINKS", oneNewLink)
	t.Setenv("ZE_TEST_ADDRS", "4: ppp0    inet 10.100.0.2 peer 10.100.0.1/32 scope global ppp0")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	run := newGokrazyL2TP(t.TempDir())
	run.Progress = nil
	base := pppBaseline{ns: run.Lab.Namespace, links: map[string]bool{}}

	named := applianceConsole(t, "l2tp: PPP session up tunnel-id=1 interface=ppp7")
	report, proven := run.assertKernelState(GokrazyL2TPReport{}, named, base)
	if !proven {
		t.Fatalf("a complete console was refused: %s", report.Reason)
	}
	if report.ApplianceInterface != "ppp7" {
		t.Errorf("the appliance interface is %q, want the one its console named", report.ApplianceInterface)
	}

	unnamed := applianceConsole(t, "l2tp: PPP session up tunnel-id=1 session-id=1")
	report, proven = run.assertKernelState(GokrazyL2TPReport{}, unnamed, base)
	if proven {
		t.Fatalf("a console naming no interface was accepted, as %q", report.ApplianceInterface)
	}
	if !strings.Contains(report.Reason, "interface=pppN") {
		t.Errorf("the refusal does not say what was missing: %s", report.Reason)
	}
}

// VALIDATES: the lab names every object after this process, and the bridge, tap
// and veth names fit the kernel's bound.
// PREVENTS: two runs on one machine colliding on a bridge, and a link name the
// kernel refuses, which reads as "cannot create bridge" with no reason.
func TestTheLabNamesEveryObjectAfterThisRun(t *testing.T) {
	lab := newGokrazyLab(namespaceSuffix())

	const linkMax = 15
	names := []string{lab.Bridge, lab.Tap, lab.HostVeth, lab.PeerVeth}
	for _, name := range names {
		if len(name) > linkMax {
			t.Errorf("the link name %q is %d characters, over the kernel's %d", name, len(name), linkMax)
		}
	}
	for i, name := range names {
		if slices.Index(names, name) != i {
			t.Errorf("two objects are both called %q", name)
		}
	}
	suffix := namespaceSuffix()
	for _, path := range []string{lab.PidFile, lab.LeaseFile, lab.LogFile} {
		if !strings.Contains(path, suffix) {
			t.Errorf("the dnsmasq file %q is not named after this run (%s)", path, suffix)
		}
	}
}

// VALIDATES: The DHCP server hands out ONE address, keyed on the appliance's
// MAC, and answers no DNS.
// PREVENTS: Two failures that led to this configuration. A range rather than a
// reservation means the peer cannot be told the address before the appliance
// boots. A DNS listener clashes with the host's own resolver on port 53.
func TestTheDHCPServerReservesTheApplianceAddress(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	run := newGokrazyL2TP(t.TempDir())
	lab := run.Lab

	// The test reads the argv from the failure because the code starts the server
	// rather than only describing it. There is no dnsmasq on a test machine, so
	// the start fails. The test asserts the argv that it would have used.
	lab.Progress = nil
	got := dnsmasqArgv(lab)
	joined := strings.Join(got, " ")
	for _, want := range []string{
		"--interface=" + lab.Bridge,
		"--port=0",
		"--dhcp-range=" + lab.ApplianceIP + "," + lab.ApplianceIP + ",255.255.255.0,2m",
		"--dhcp-host=" + lab.ApplianceMAC + "," + lab.ApplianceIP,
		"--pid-file=" + lab.PidFile,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the DHCP server is not started with %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "--bind-interfaces") {
		t.Error("the DHCP socket is bound to the interface address, where it cannot receive a broadcast DISCOVER")
	}
}

// VALIDATES: the appliance template configures the pool the peer negotiates
// against, and turns on the two servers the proof watches for.
// PREVENTS: an image whose daemon has no L2TP at all, which fails at the
// listener wait with nothing to say why.
func TestTheApplianceTemplateConfiguresTheProofsPool(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	template := newGokrazyL2TP(t.TempDir()).applianceTemplate()
	for _, want := range []string{
		"set l2tp enabled true",
		"set l2tp pool ipv4 gateway " + L2TPPPPLocalAddr,
		"set l2tp pool ipv4 start " + L2TPPPPPeerAddr,
		"set l2tp pool ipv4 end " + L2TPPPPPoolEnd,
		"set environment l2tp server main ip 0.0.0.0",
		"set environment web enabled true",
		"set interface dhcp-auto true",
	} {
		if !strings.Contains(template, want) {
			t.Errorf("the template lacks %q:\n%s", want, template)
		}
	}
}

// VALIDATES: The peer dials the APPLIANCE rather than a loopback daemon. It uses
// its own short directory.
// PREVENTS: Two real failures. A peer pointed at the loopback dials the host's
// own ze if one is running. xl2tpd 1.3.18 silently truncates a configuration
// path over about ninety characters. It then reports that it cannot open the
// file.
func TestThePeerDialsTheApplianceFromAShortPath(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	run := newGokrazyL2TP(t.TempDir())
	dir := t.TempDir()
	if err := run.writePeerInputs(dir); err != nil {
		t.Fatalf("write the peer inputs: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, "xl2tpd.conf")) //nolint:gosec // a path this test wrote
	if err != nil {
		t.Fatalf("read the peer configuration: %v", err)
	}
	config := string(body)
	if !strings.Contains(config, "lns = "+run.Lab.ApplianceIP) {
		t.Errorf("the peer does not dial the appliance:\n%s", config)
	}
	if !strings.Contains(config, "port = "+run.PeerPort) {
		t.Errorf("the peer does not bind its own port:\n%s", config)
	}

	short, err := lacScratch()
	if err != nil {
		t.Fatalf("make the peer's own directory: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(short) }) //nolint:errcheck // a test's own directory
	const xl2tpdPathMax = 90
	full := filepath.Join(short, "xl2tpd.conf")
	if len(full) > xl2tpdPathMax {
		t.Errorf("the peer's configuration path is %d characters, over xl2tpd's %d: %s",
			len(full), xl2tpdPathMax, full)
	}
}

// VALIDATES: The accelerator is derived from whether /dev/kvm can be OPENED.
// An operator's own choice wins.
// PREVENTS: The failure that the read-write test exists to prevent. A group owns
// the KVM node, so a user outside it sees the node while QEMU cannot open it.
// QEMU then fails outright rather than falling back to software.
func TestTheAcceleratorIsDerivedFromWhatCanBeOpened(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	if got := applianceAccel(); got != "kvm" && got != "tcg" {
		t.Errorf("the accelerator is %q, want kvm or tcg", got)
	}

	t.Setenv("ZE_GOKRAZY_QEMU_ACCEL", "hvf")
	env.ResetCache()
	if got := applianceAccel(); got != "hvf" {
		t.Errorf("an operator's own accelerator answered %q", got)
	}
}

// VALIDATES: the architecture comes from this proof's own variable, falls back
// to the build's, and defaults to amd64.
// PREVENTS: an image built for one architecture and booted for another, which
// is a QEMU that prints nothing at all.
func TestTheArchitectureFallsBackToTheBuildsOwnVariable(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	if got := applianceArch(); got != ArchAMD64 {
		t.Errorf("with nothing set the architecture is %q, want %s", got, ArchAMD64)
	}

	t.Setenv(GokrazyArchEnv, ArchARM64)
	env.ResetCache()
	if got := applianceArch(); got != ArchARM64 {
		t.Errorf("with only the build variable set the architecture is %q", got)
	}

	t.Setenv("ZE_GOKRAZY_ARCH", ArchAMD64)
	env.ResetCache()
	if got := applianceArch(); got != ArchAMD64 {
		t.Errorf("this proof's own variable did not win: %q", got)
	}
}
