// Design: docs/architecture/testing/qemu-integration.md -- ze proven on the image that ships
// Related: gokrazylab.go -- the network this proof builds
// Related: gokrazyimage.go -- the image this proof boots
// Related: gokrazykernel.go -- the kernel that image must carry
// Related: pppstate.go -- the kernel state this proof asserts about
// Related: l2tpppp.go -- the same proof on a developer host rather than an appliance
//
// gokrazyl2tp.go proves the whole L2TP PPP path against the APPLIANCE rather
// than against a developer's checkout. A real xl2tpd dials a gokrazy image in
// QEMU. A real pppd negotiates LCP and IPCP over the tunnel. The host kernel
// creates the peer's pppN interface, traffic crosses it, and every object goes
// away again when the peer leaves.
//
// This proof provides evidence that the on-host proof cannot provide. The
// on-host proof runs a daemon built from the working tree. This proof runs the
// shipped daemon, in its image, on the kernel that the image carries.
//
// Three failures have occurred at that boundary. No developer host detected
// them. One kernel had no PPPoL2TP. In one instance, the environment did not
// reach the daemon. One image did not receive an inbound tunnel request.
//
// The appliance's own account is the SERIAL CONSOLE. Nothing on the host can
// look inside the virtual machine. Every claim about what the daemon did comes
// from the console. Every claim about what the kernel did comes from the host
// side of the tunnel.

package deployment

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The dot-notation spellings of the ZE_GOKRAZY_* variables. env.Get matches
// case-insensitively and treats a dot and an underscore as the same character,
// so these keys read the variables the Python original read.
const (
	GokrazyHostIPKey       = "ze.gokrazy.l2tp.host.ip"
	GokrazyPeerIPKey       = "ze.gokrazy.l2tp.lac.ip"
	GokrazyPrefixKey       = "ze.gokrazy.l2tp.prefix"
	GokrazyPeerPortKey     = "ze.gokrazy.l2tp.xl2tpd.port"
	GokrazyApplianceIPKey  = "ze.gokrazy.l2tp.appliance.ip"
	GokrazyApplianceMACKey = "ze.gokrazy.l2tp.appliance.mac"
	GokrazyArchKey         = "ze.gokrazy.arch"
	GokrazyAccelKey        = "ze.gokrazy.qemu.accel"
	GokrazyBiosKey         = "ze.gokrazy.aarch64.bios"
	GokrazyCPUKey          = "ze.gokrazy.aarch64.cpu"
)

// What the run uses when the operator names nothing.
//
// The underlay is a private /24 nobody routes, carried by a bridge the
// appliance and the peer both attach to. It differs from the on-host proof's
// underlay so the two can run on one machine at the same time.
const (
	GokrazyHostIP       = "172.31.0.1"
	GokrazyPeerIP       = "172.31.0.2"
	GokrazyPrefix       = "24"
	GokrazyPeerPort     = "1702"
	GokrazyApplianceIP  = "172.31.0.10"
	GokrazyApplianceMAC = "52:54:00:12:34:56"
	GokrazyBios         = "/usr/share/qemu-efi-aarch64/QEMU_EFI.fd"
	GokrazyCPU          = "max"
)

// GokrazyArchEnv is the environment variable that supplies the architecture for
// an appliance build. The proof reads it as a fallback, so an operator does not
// have to set a second variable for the proof.
const GokrazyArchEnv = "GOKRAZY_ARCH"

var (
	gokrazyHostIPEntry = stringSetting(GokrazyHostIPKey, GokrazyHostIP,
		"the underlay address the bridge holds in the appliance L2TP PPP proof")
	gokrazyPeerIPEntry = stringSetting(GokrazyPeerIPKey, GokrazyPeerIP,
		"the underlay address the xl2tpd peer holds in the appliance L2TP PPP proof")
	gokrazyPrefixEntry = stringSetting(GokrazyPrefixKey, GokrazyPrefix,
		"the prefix length of the underlay the appliance L2TP PPP proof builds")
	gokrazyPeerPortEntry = stringSetting(GokrazyPeerPortKey, GokrazyPeerPort,
		"the port the xl2tpd peer binds in the appliance L2TP PPP proof")
	gokrazyApplianceIPEntry = stringSetting(GokrazyApplianceIPKey, GokrazyApplianceIP,
		"the address the appliance is given by DHCP, and the address the peer dials")
	gokrazyApplianceMACEntry = stringSetting(GokrazyApplianceMACKey, GokrazyApplianceMAC,
		"the MAC the appliance NIC carries, which the DHCP reservation is keyed on")
	gokrazyArchEntry = stringSetting(GokrazyArchKey, "",
		"the architecture the appliance image is built and booted for")
	gokrazyAccelEntry = stringSetting(GokrazyAccelKey, "",
		"the QEMU accelerator the appliance is booted with; derived from /dev/kvm by default")
	gokrazyBiosEntry = stringSetting(GokrazyBiosKey, GokrazyBios,
		"the aarch64 firmware the appliance is booted with")
	gokrazyCPUEntry = stringSetting(GokrazyCPUKey, GokrazyCPU,
		"the aarch64 CPU model the appliance is booted with")
)

// gokrazyProofName is what this proof is called in the sentences it refuses
// with. It is the Python original's wording.
const gokrazyProofName = "gokrazy L2TP PPP appliance evidence"

// The run waits for these appliance console lines. The first line confirms that
// the appliance finished its boot. The remaining lines confirm the steps of the
// L2TP path. They are the on-host proof's own lines because the same daemon
// prints them.
const (
	gokrazyWebLine = "web server listening"
)

// gokrazyFatalLines are the appliance's own reports that it cannot carry the
// proof.
//
// The first report distinguishes this list from the on-host proof's list. A
// fail-closed module probe that refuses startup puts the appliance into a crash
// loop. Without this check, the proof waits out the ninety-second boot bound and
// reports "the web server did not start" for a kernel problem that the appliance
// already named. A REJECTED IPv6CP result is fatal here. A DECLINED IPv6CP
// result is not. The appliance reports DECLINED in a different sentence because
// the pool this proof configures is v4 only.
var gokrazyFatalLines = []string{
	"failed to load kernel modules",
	"skipping kernel module probe",
	"genl family resolve failed",
	"kernel integration disabled",
	"kernel session ready but no PPP driver wired",
	"ipcp: handler rejected",
	"ipv6cp: handler rejected",
	"ncp: timeout",
	"ip-response timeout",
}

// These waits bound the run. Boot has the longest wait because a whole virtual
// machine must start under TCG, which provides no hardware acceleration. The
// remaining waits use the on-host proof's bounds and allow more time for the
// appliance's slower path.
const (
	GokrazyBootWait     = 90 * time.Second
	GokrazyListenerWait = 30 * time.Second
	GokrazyAddressWait  = 30 * time.Second
	GokrazySessionWait  = 30 * time.Second
	GokrazyNCPWait      = 60 * time.Second
	GokrazyWithdrawWait = 20 * time.Second
	GokrazyCleanupWait  = 30 * time.Second
)

// gokrazyL2TP is one run of the appliance L2TP PPP proof.
type gokrazyL2TP struct {
	// Tree is the checkout the image is built from.
	Tree string
	// Arch is what the image is built and booted for, and Accel is how QEMU
	// runs it.
	Arch  string
	Accel string
	Bios  string
	CPU   string
	// PeerPort is the port the peer binds to dial from.
	PeerPort string
	// Lab is the network the appliance and the peer meet on.
	Lab *gokrazyLab
	// The bounds on each step of the run.
	BootWait     time.Duration
	ListenerWait time.Duration
	AddressWait  time.Duration
	SessionWait  time.Duration
	NCPWait      time.Duration
	WithdrawWait time.Duration
	CleanupWait  time.Duration
	// Progress receives the appliance's console and the peer's output as they
	// arrive. It MUST be safe for concurrent use.
	Progress io.Writer
}

// newGokrazyL2TP answers the run the command performs over tree, with every
// setting taken from the environment or from its default.
func newGokrazyL2TP(tree string) *gokrazyL2TP {
	lab := newGokrazyLab(namespaceSuffix())
	lab.HostIP = setting(gokrazyHostIPEntry.Key, GokrazyHostIP)
	lab.PeerIP = setting(gokrazyPeerIPEntry.Key, GokrazyPeerIP)
	lab.Prefix = setting(gokrazyPrefixEntry.Key, GokrazyPrefix)
	lab.ApplianceIP = setting(gokrazyApplianceIPEntry.Key, GokrazyApplianceIP)
	lab.ApplianceMAC = setting(gokrazyApplianceMACEntry.Key, GokrazyApplianceMAC)

	return &gokrazyL2TP{
		Tree:         tree,
		Arch:         applianceArch(),
		Accel:        applianceAccel(),
		Bios:         setting(gokrazyBiosEntry.Key, GokrazyBios),
		CPU:          setting(gokrazyCPUEntry.Key, GokrazyCPU),
		PeerPort:     setting(gokrazyPeerPortEntry.Key, GokrazyPeerPort),
		Lab:          lab,
		BootWait:     GokrazyBootWait,
		ListenerWait: GokrazyListenerWait,
		AddressWait:  GokrazyAddressWait,
		SessionWait:  GokrazySessionWait,
		NCPWait:      GokrazyNCPWait,
		WithdrawWait: GokrazyWithdrawWait,
		CleanupWait:  GokrazyCleanupWait,
		Progress:     os.Stderr,
	}
}

// applianceArch answers the architecture that the image build and boot use.
//
// The function reads the appliance build's environment variable as a fallback
// because an operator intended the same architecture for this proof. The
// fallback avoids a second variable that CAN make the two architectures
// disagree.
func applianceArch() string {
	if named := env.Get(gokrazyArchEntry.Key); named != "" {
		return named
	}
	if named := os.Getenv(GokrazyArchEnv); named != "" {
		return named
	}
	return ArchAMD64
}

// applianceAccel answers how QEMU runs the appliance.
//
// The test checks whether the KVM device permits READ AND WRITE. It does not
// merely check whether the device exists. A group owns the device, so a user
// outside that group sees the node while QEMU cannot open it. QEMU then fails
// outright rather than falling back.
func applianceAccel() string {
	if named := env.Get(gokrazyAccelEntry.Key); named != "" {
		return named
	}
	if err := unix.Access("/dev/kvm", unix.R_OK|unix.W_OK); err == nil {
		return "kvm"
	}
	return "tcg"
}

// Run performs the proof and answers what happened.
//
// A step that cannot be performed is an error. The operator has something to
// fix, and the proof reached no verdict. A step of the L2TP path that does not
// happen is NOT an error. It is the verdict. The report contains the verdict,
// the reason, and the appliance's last console lines.
func (g *gokrazyL2TP) Run() (GokrazyL2TPReport, error) {
	report := GokrazyL2TPReport{
		Peer:             PeerName,
		Arch:             g.Arch,
		Accel:            g.Accel,
		Namespace:        g.Lab.Namespace,
		ApplianceAddress: g.Lab.ApplianceIP,
		LocalAddress:     L2TPPPPLocalAddr,
		PeerAddress:      L2TPPPPPeerAddr,
	}

	if err := refuseSkipKernelProbe(); err != nil {
		return report, err
	}
	if err := look("ip", "ping", PeerName, "pppd", "dnsmasq"); err != nil {
		return report, err
	}
	if err := ensureKernelSupport(gokrazyProofName); err != nil {
		return report, err
	}

	work, err := scratchDir(g.Tree, "gokrazy-l2tp-ppp-")
	if err != nil {
		return report, err
	}
	template := filepath.Join(work, "ze-gokrazy-l2tp.conf")
	if err := os.WriteFile(template, []byte(g.applianceTemplate()), inputMode); err != nil {
		return report, err
	}

	peerDir, err := lacScratch()
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(peerDir) //nolint:errcheck // the peer's own files, outside the checkout
	if err := g.writePeerInputs(peerDir); err != nil {
		return report, err
	}

	image, err := gokrazyImage(g.Tree, work, template, g.Arch, g.Progress)
	if err != nil {
		return report, err
	}
	report.Image = image

	defer g.Lab.remove()
	if err := g.Lab.setup(); err != nil {
		return report, err
	}

	report, err = g.observe(report, image, peerDir)
	if err != nil {
		return report, err
	}
	if report.Proven {
		os.RemoveAll(work) //nolint:errcheck // the run passed; a scratch directory left behind is not a verdict
	}
	return report, nil
}

// observe boots the appliance, walks the L2TP path, and answers the verdict.
//
// The proof stops the peer before the appliance. Only the peer's departure can
// test the teardown path. If the appliance stops first, it ends the session
// from the wrong end and proves nothing about the withdraw path.
func (g *gokrazyL2TP) observe(report GokrazyL2TPReport, image, peerDir string) (GokrazyL2TPReport, error) {
	baselines, err := readPPPBaselines([]string{g.Lab.Namespace})
	if err != nil {
		return report, err
	}
	base := baselines[0]

	console := newCollector(append([]string{
		gokrazyWebLine, pppListenerLine, pppSessionLine, pppIPLine,
		pppRouteLine, pppUpLine, pppWithdrawLine, pppTeardownLine,
	}, gokrazyFatalLines...)...)

	argv, err := g.qemuArgs(image)
	if err != nil {
		return report, err
	}
	machine := exec.CommandContext(g.context(), argv[0], argv[1:]...) //nolint:gosec // the argv is built below, never by an operator
	machine.Dir = g.Tree

	appliance, err := startWatched(machine, "qemu> ", console, g.Progress)
	if err != nil {
		return report, err
	}
	defer console.wait()
	defer appliance.stop()

	preSession := append(slices.Clone(gokrazyFatalLines), pppTeardownLine)
	steps := []struct {
		wanted []string
		fatal  []string
		wait   time.Duration
		missed string
	}{
		{[]string{gokrazyWebLine}, preSession, g.BootWait, "gokrazy appliance web server did not start"},
		{[]string{pppListenerLine}, preSession, g.ListenerWait, "gokrazy appliance L2TP listener did not start"},
	}
	for _, step := range steps {
		if verdict, ok := g.step(console, report, appliance, step.wanted, step.fatal, step.wait, step.missed); !ok {
			return verdict, nil
		}
	}

	if err := g.Lab.awaitAppliance(g.AddressWait); err != nil {
		writeProgress(g.Progress, g.Lab.dnsmasqLog())
		return g.fail(report, console, err.Error()), nil
	}

	peer := nsCommand(g.Lab.Namespace, PeerName,
		"-D",
		"-c", filepath.Join(peerDir, PeerConfigFile),
		"-s", filepath.Join(peerDir, PeerSecretsFile),
		"-p", filepath.Join(peerDir, "xl2tpd.pid"),
		"-C", filepath.Join(peerDir, "l2tp-control"))

	said := newCollector()
	dialer, err := startWatched(peer, "xl2tpd> ", said, g.Progress)
	if err != nil {
		return report, err
	}
	defer said.wait()
	defer dialer.stop()

	if verdict, ok := g.step(console, report, appliance, []string{pppSessionLine}, preSession,
		g.SessionWait, "xl2tpd did not establish an incoming L2TP session with the appliance"); !ok {
		return verdict, nil
	}
	if verdict, ok := g.step(console, report, appliance,
		[]string{pppIPLine, pppRouteLine, pppUpLine}, gokrazyFatalLines, g.NCPWait,
		"appliance PPP LCP/IPCP completion and route injection were not observed"); !ok {
		return verdict, nil
	}

	report, proven := g.assertKernelState(report, console, base)
	if !proven {
		return report, nil
	}

	dialer.stop()
	said.wait()
	if verdict, ok := g.step(console, report, appliance, []string{pppWithdrawLine}, nil,
		g.WithdrawWait, "appliance subscriber route withdraw was not observed during teardown"); !ok {
		return verdict, nil
	}

	base.iface = report.PeerInterface
	if err := awaitTeardown([]pppBaseline{base}, g.CleanupWait); err != nil {
		return g.fail(report, console, err.Error()), nil
	}

	report.Proven = true
	return report, nil
}

// assertKernelState asks the host kernel what the appliance's console claims.
//
// Only the peer's kernel is visible because the appliance kernel is inside the
// virtual machine. The appliance reports its interface through the console. The
// host kernel reports the peer's interface. A successful dataplane ping proves
// that a packet crossed both interfaces.
func (g *gokrazyL2TP) assertKernelState(report GokrazyL2TPReport, console *collector, base pppBaseline) (GokrazyL2TPReport, bool) {
	var tb textbuf.Buffer
	wantAddress := tb.Str("address=").Str(L2TPPPPPeerAddr).String()
	if !anyLineCarrying(console.carrying(pppIPLine), wantAddress) {
		return g.fail(report, console, tb.Reset().Str("session IP assigned log missing expected address=").
			Str(L2TPPPPPeerAddr).String()), false
	}

	for _, line := range console.carrying(pppUpLine) {
		if candidate := interfaceField(line); strings.HasPrefix(candidate, pppPrefix) {
			report.ApplianceInterface = candidate
			break
		}
	}
	if report.ApplianceInterface == "" {
		return g.fail(report, console, "appliance PPP session up log missing interface=pppN field"), false
	}

	peerIface, err := discoverPPPIface(g.Lab.Namespace, base.links, nil, "LAC")
	if err != nil {
		return g.fail(report, console, err.Error()), false
	}
	report.PeerInterface = peerIface

	if err := verifyPPPAddress(g.Lab.Namespace, peerIface, L2TPPPPPeerAddr, L2TPPPPLocalAddr); err != nil {
		return g.fail(report, console, err.Error()), false
	}
	if _, ok := nsText(g.Lab.Namespace, "ping", "-c", "2", "-W", "3", L2TPPPPLocalAddr); !ok {
		return g.fail(report, console, tb.Reset().Str("dataplane ping to appliance LNS ").
			Str(L2TPPPPLocalAddr).Str(" through PPP tunnel failed").String()), false
	}
	return report, true
}

// step waits for one set of console lines and turns a miss into the verdict.
func (g *gokrazyL2TP) step(console *collector, report GokrazyL2TPReport, appliance *running,
	wanted, fatal []string, wait time.Duration, missed string,
) (GokrazyL2TPReport, bool) {
	arrived, err := awaitAll(console, wanted, fatal, appliance, wait)
	if err != nil {
		return g.fail(report, console, err.Error()), false
	}
	if !arrived {
		return g.fail(report, console, missed), false
	}
	return report, true
}

// fail answers the report for a proof that did not complete, with the reason
// and the appliance's last console lines in it.
func (g *gokrazyL2TP) fail(report GokrazyL2TPReport, console *collector, reason string) GokrazyL2TPReport {
	report.Proven = false
	report.Reason = reason
	report.LogTail = console.tailLines()
	return report
}

// context answers the context the virtual machine is started under. It is
// unbounded because the run's own waits are where it is bounded in time, and
// the stop path is what ends the machine.
func (g *gokrazyL2TP) context() context.Context { return context.Background() }

// qemuArgs answers the virtual machine.
//
// The NIC uses a TAP on the lab's bridge instead of QEMU user-mode networking.
// The lab exists because user-mode networking does not deliver inbound UDP to
// the guest. An L2TP tunnel starts with an inbound SCCRQ. The bridge makes the
// appliance reachable at its own address with no translation or forwarding.
func (g *gokrazyL2TP) qemuArgs(image string) ([]string, error) {
	var tb textbuf.Buffer
	netdev := tb.Str("tap,id=net0,ifname=").Str(g.Lab.Tap).Str(",script=no,downscript=no").String()
	drive := tb.Reset().Str("file=").Str(image).Str(",format=raw").String()
	device := tb.Reset().Str("e1000,netdev=net0,mac=").Str(g.Lab.ApplianceMAC).String()

	tail := []string{
		"-drive", drive,
		"-nographic",
		"-serial", "mon:stdio",
		"-netdev", netdev,
		"-device", device,
	}

	switch g.Arch {
	case ArchAMD64:
		if err := look("qemu-system-x86_64"); err != nil {
			return nil, err
		}
		head := make([]string, 0, 7+len(tail))
		head = append(head,
			"qemu-system-x86_64",
			"-machine", tb.Reset().Str("accel=").Str(g.Accel).String(),
			"-smp", "2",
			"-m", "512",
		)
		return append(head, tail...), nil
	case ArchARM64:
		if err := look("qemu-system-aarch64"); err != nil {
			return nil, err
		}
		if !isRegularFile(g.Bios) {
			return nil, errors.New(tb.Reset().Str("aarch64 QEMU firmware not found: ").Str(g.Bios).String())
		}
		head := make([]string, 0, 11+len(tail))
		head = append(head,
			"qemu-system-aarch64",
			"-machine", tb.Reset().Str("virt,highmem=off,accel=").Str(g.Accel).String(),
			"-cpu", g.CPU,
			"-smp", "2",
			"-m", "512",
			"-bios", g.Bios,
		)
		return append(head, tail...), nil
	default:
		return nil, errUnsupportedArch
	}
}

// applianceTemplate answers the configuration the image is built carrying.
//
// The image builder takes a command SCRIPT, not a configuration file. It types
// each line into the appliance's editor during the build. DHCP lets the
// appliance find the lab. The proof enables the web server and SSH for access.
// The web server reports when the boot is complete.
func (g *gokrazyL2TP) applianceTemplate() string {
	var tb textbuf.Buffer
	return tb.Str("set environment log level info\n").
		Str("set environment web enabled true\n").
		Str("set environment web server default ip 0.0.0.0\n").
		Str("set environment web server default port 8080\n").
		Str("set environment ssh enabled true\n").
		Str("set environment ssh server default ip 0.0.0.0\n").
		Str("set environment ssh server default port 22\n").
		Str("set environment ntp enabled false\n").
		Str("set interface dhcp-auto true\n").
		Str("set l2tp enabled true\n").
		Str("set l2tp auth-method none\n").
		Str("set l2tp allow-no-auth true\n").
		Str("set l2tp hello-interval 5\n").
		Str("set l2tp max-tunnels 4\n").
		Str("set l2tp max-sessions 4\n").
		Str("set l2tp pool ipv4 gateway ").Str(L2TPPPPLocalAddr).Byte('\n').
		Str("set l2tp pool ipv4 start ").Str(L2TPPPPPeerAddr).Byte('\n').
		Str("set l2tp pool ipv4 end ").Str(L2TPPPPPoolEnd).Byte('\n').
		Str("set l2tp pool ipv4 dns-primary 8.8.8.8\n").
		Str("set l2tp pool ipv4 dns-secondary 8.8.4.4\n").
		Str("set environment l2tp server main ip 0.0.0.0\n").
		Str("set environment l2tp server main port 1701\n").String()
}

// writePeerInputs writes the three files the peer reads.
//
// These files differ from the on-host proof's files in two ways. The peer dials
// the APPLIANCE instead of a daemon on the loopback. The peer also runs without
// a pppd log file because its output already reaches this process.
func (g *gokrazyL2TP) writePeerInputs(dir string) error {
	var tb textbuf.Buffer
	config := tb.Str("[global]\nport = ").Str(g.PeerPort).
		Str("\nauth file = ").Str(filepath.Join(dir, PeerSecretsFile)).
		Str("\ndebug tunnel = yes\ndebug state = yes\ndebug packet = yes\ndebug avp = yes\n\n").
		Str("[lac ze]\nlns = ").Str(g.Lab.ApplianceIP).
		Str("\nautodial = yes\nredial = yes\nredial timeout = 1\nmax redials = 5\n").
		Str("require authentication = no\nppp debug = yes\npppoptfile = ").
		Str(filepath.Join(dir, PeerOptionsFile)).
		Str("\nlength bit = yes\n").String()

	options := "noauth\nname alice\npassword s3cr3t\nrefuse-eap\nnodefaultroute\n" +
		"ipcp-accept-local\nipcp-accept-remote\nnoipv6\ndebug\nnodetach\n"

	files := []struct {
		name string
		body string
		mode os.FileMode
	}{
		{PeerConfigFile, config, inputMode},
		{PeerSecretsFile, L2TPPPPSecrets, secretsMode},
		{PeerOptionsFile, options, inputMode},
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file.name), []byte(file.body), file.mode); err != nil {
			return err
		}
	}
	return nil
}
