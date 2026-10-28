// Design: docs/architecture/core-design.md -- machine state that is not a binary
//
// This file handles a kernel tunable, a group, and fixture loopback addresses.
// These items cannot be installed, so they do not belong in the tool table.
// Each item has three questions: What is its state? Which command changes it?
// Can that command run now?

package setup

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"os/user"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// --- Unprivileged user namespaces -----------------------------------------
//
// Ubuntu 23.10+ ships kernel.apparmor_restrict_unprivileged_userns=1, which
// blocks the user-namespace sandbox Chrome relies on. The agent-browser web
// functional tests then cannot launch Chrome ("No usable sandbox!"), so the
// restriction must be lifted globally.

// usernsSysctl is the tunable, usernsProcDefault is where the running kernel
// publishes it, and usernsConf is the drop-in that survives a reboot.
const usernsSysctl = "kernel.apparmor_restrict_unprivileged_userns"

const usernsProcDefault = "/proc/sys/kernel/apparmor_restrict_unprivileged_userns"

const usernsConf = "/etc/sysctl.d/60-ze-userns.conf"

// userns is the unprivileged-userns restriction state.
type userns string

const (
	// usernsOK means unprivileged user namespaces are allowed (value 0).
	usernsOK userns = "ok"
	// usernsRestricted means they are blocked (value 1).
	usernsRestricted userns = "restricted"
	// usernsNA means the kernel has no such knob, so there is nothing to do.
	// A non-AppArmor host reaches this.
	usernsNA userns = "na"
)

// usernsState reports whether unprivileged user namespaces are permitted. It
// also reports whether the question applies.
//
// If a knob EXISTS but cannot be READ, the result is an error, not usernsNA.
// The script returned NA in this state. That result means "no such knob, nothing
// to do", although the knob is present. The unreadable value is unknown and CAN
// be 1. The restriction then reports [present], and the run passes on a machine
// where Chrome cannot start
// (plan/journal/zero-value-as-valid-answer.md, 2026-08-26).
func (s *Setup) usernsState() (userns, error) {
	raw, err := os.ReadFile(s.usernsProc())
	if os.IsNotExist(err) {
		return usernsNA, nil
	}
	if err != nil {
		return usernsNA, err
	}
	if strings.TrimSpace(string(raw)) == "1" {
		return usernsRestricted, nil
	}
	return usernsOK, nil
}

// privilegedStep is one root command, what to feed it on stdin, and the line a
// human would copy instead of the argv.
type privilegedStep struct {
	argv  []string
	stdin []byte
	shown string
}

// usernsSteps returns the commands that remove the restriction persistently.
//
// The drop-in under /etc/sysctl.d remains after a reboot. The `sysctl -w`
// command applies the value to the running kernel. Both commands are necessary.
// One command corrects the current state, and the other corrects the state after
// the next boot.
func usernsSteps() []privilegedStep {
	var body textbuf.Buffer
	body.Str(usernsSysctl).Str(" = 0").Byte('\n')

	var shown textbuf.Buffer
	shown.Str(`echo "`).Str(usernsSysctl).Str(` = 0" | `).
		Str(sudoPlaceholder).Str("tee ").Str(usernsConf)

	var write textbuf.Buffer
	return []privilegedStep{
		{argv: []string{"tee", usernsConf}, stdin: []byte(body.String()), shown: shown.String()},
		{argv: []string{"sysctl", "-w", write.Str(usernsSysctl).Str("=0").String()}},
	}
}

// noteUsernsFix records the commands, for when root is out of reach.
func noteUsernsFix(report *Report) {
	var drop textbuf.Buffer
	report.Note(drop.Str(`  Run: echo "`).Str(usernsSysctl).Str(` = 0" | sudo tee `).Str(usernsConf).String())
	var apply textbuf.Buffer
	report.Note(apply.Str("  Run: sudo sysctl -w ").Str(usernsSysctl).Str("=0").String())
}

// applyUserns records, then runs, the commands that lift the restriction.
//
// It answers true only when the restriction is actually cleared. On any failure
// the caller falls back to naming the manual commands.
func (s *Setup) applyUserns(report *Report) bool {
	for _, step := range usernsSteps() {
		ok, detail := s.Shell.runPrivileged(report, step.argv, step.stdin, step.shown)
		if !ok {
			var tb textbuf.Buffer
			report.Note(tb.Str("  FAIL: ").Str(detail).String())
			return false
		}
	}
	state, err := s.usernsState()
	return err == nil && state == usernsOK
}

// --- KVM device access ----------------------------------------------------
//
// QEMU-backed evidence includes appliance boot proofs and the ze-qemu-* targets.
// It uses KVM when KVM is available. /dev/kvm is root:kvm 0660. Therefore, the
// invoking user must be in the kvm group. QEMU does not use an alternative
// without this membership. It stops because access to the KVM kernel module is
// denied, and the caller reports a timeout.
//
// This requirement applies only to Linux. macOS has no /dev/kvm. QEMU uses the
// Apple hypervisor (hvf), which does not require a group.

// kvmDevDefault is the device. KVMGroup is the group permitted to open it.
const kvmDevDefault = "/dev/kvm"

// KVMGroup is the group /dev/kvm belongs to.
const KVMGroup = "kvm"

// usermodBin is the program that adds the user to a group, for the kvm and the
// /dev/ppp steps.
const usermodBin = "usermod"

// kvm is whether QEMU can use KVM as this user.
type kvm string

const (
	// kvmOK means /dev/kvm is readable and writable in this process now.
	kvmOK kvm = "ok"
	// kvmPendingLogin means that the user IS in the kvm group, but the session
	// started before that membership changed. Group membership is fixed at login.
	kvmPendingLogin kvm = "pending-login"
	// kvmNoGroup means the device exists and the user is not in the group.
	kvmNoGroup kvm = "no-group"
	// kvmNA means no /dev/kvm at all (no hardware virt, or a VM without nested
	// virt). QEMU still runs under tcg, only slower, so there is nothing to
	// fix.
	kvmNA kvm = "na"
)

// inKvmGroup reports whether the group database lists this user in the kvm
// group.
//
// This function intentionally does not test device access. After `usermod -aG`,
// the database includes the user, but processes that are already running do not.
// This distinction determines whether the instruction is "run this command" or
// "log back in".
//
// The script read the supplementary member list of the group. Therefore, it did
// not identify a user whose PRIMARY group is kvm as a member. The group IDs of
// the current user include primary and supplementary groups. The two methods can
// differ only for a user who cannot reach this branch. A primary-group member
// can already open the device.
func (s *Setup) inKvmGroup() bool {
	if s.KvmGroupMember != nil {
		return s.KvmGroupMember()
	}
	return groupDatabaseLists(KVMGroup)
}

// groupDatabaseLists reports whether the group database lists the invoking
// user in the named group, as its primary or a supplementary group. A group or
// user it cannot look up answers false: the caller then names the command that
// adds the membership, which is the fix in either case.
func groupDatabaseLists(name string) bool {
	group, err := user.LookupGroup(name)
	if err != nil {
		return false
	}
	current, err := user.Current()
	if err != nil {
		return false
	}
	ids, err := current.GroupIds()
	if err != nil {
		return false
	}
	return slices.Contains(ids, group.Gid)
}

// kvmState answers whether QEMU can use KVM as this user.
func (s *Setup) kvmState() kvm {
	device := s.kvmDev()
	if _, err := os.Stat(device); err != nil {
		return kvmNA
	}
	if deviceOpenable(device) {
		return kvmOK
	}
	if s.inKvmGroup() {
		return kvmPendingLogin
	}
	return kvmNoGroup
}

// deviceOpenable reports whether this process can open the device for reading
// and writing. QEMU uses /dev/kvm in this way.
//
// The script used os.access, which tests permission bits against the REAL uid.
// It does not perform the required operation. Opening the device performs that
// operation and is portable where a raw access(2) is not. Opening /dev/kvm does
// not allocate resources. A VM requires an ioctl on the fd tested here.
func deviceOpenable(path string) bool {
	handle, err := os.OpenFile(path, os.O_RDWR, 0) //nolint:gosec // the path is the device this tool is asked about, and opening it is the question
	if err != nil {
		return false
	}
	handle.Close() //nolint:errcheck // probe handle, result irrelevant
	return true
}

// userName answers the invoking user's login name, which is what usermod takes.
func (s *Setup) userName() string {
	if s.User != nil {
		return s.User()
	}
	current, err := user.Current()
	if err != nil {
		return ""
	}
	return current.Username
}

// noteKvmFix records the commands, for when root is out of reach.
func (s *Setup) noteKvmFix(report *Report) {
	var add textbuf.Buffer
	report.Note(add.Str("  Run: sudo usermod -aG ").Str(KVMGroup).Byte(' ').Str(s.userName()).String())
	var then textbuf.Buffer
	report.Note(then.Str("  Then log out and back in, or prefix a command with: sg ").
		Str(KVMGroup).Str(" -c '<command>'").String())
}

// applyKvm adds the invoking user to the kvm group.
//
// It answers true when the group database lists the user afterwards. That is
// NOT the same as usable: this process keeps the groups it started with, so the
// caller must still say to log back in.
func (s *Setup) applyKvm(report *Report) bool {
	ok, detail := s.Shell.runPrivileged(report, []string{usermodBin, "-aG", KVMGroup, s.userName()}, nil, "")
	if !ok {
		var tb textbuf.Buffer
		report.Note(tb.Str("  FAIL: ").Str(detail).String())
		return false
	}
	return s.inKvmGroup()
}

// --- Loopback addresses the functional suite binds ------------------------
//
// A .ci fixture with two BGP speakers assigns a separate address to each end.
// RFC 4271 Section 5.1.3 forbids a peer from using its own address as NEXT_HOP.
// Therefore, a session with one address at both ends withholds every originated
// route (originatedNextHopIsPeerOwn,
// internal/component/bgp/reactor/forward_next_hop.go).
//
// IPv4 provides 127.0.0.0/8. Linux routes the complete range to lo. Therefore,
// only macOS needs aliases, and only for addresses that the suite uses.
//
// IPv6 needs explicit loopback addresses. fd00::2/128 is the speaker and next-hop
// owner; fd00::3/127 supplies the recipient and their one common fd00::2/127
// subnet for RFC 2545. fe80::1 belongs to that same interface and owner, as the
// link-local next hop and bind address; its prefix length is immaterial to the
// fixtures, and macOS assigns fe80::1/64 to lo0 at boot. Setup adds it as /128
// where it is absent (Linux). The /127 connects exactly those two unique-local
// addresses, not all of fd00::/64.
//
// This configuration belongs to setup because the runner cannot create it.
// SIOCAIFADDR_IN6 returns EPERM to an unprivileged process on darwin. The Linux
// route requires CAP_NET_ADMIN, but the verify gate runs as an ordinary user.
// internal/test/runner/loopback.go reports the missing address and names setup.
//
// These additions do not remain after a reboot on either platform. This behavior is
// intentional. The persistent methods use a launchd plist, netplan, or a
// systemd-networkd unit. These methods modify files that a developer's machine
// can use for other purposes. Setup can run again quickly, and check mode reports
// when the configuration is necessary.

// LoopbackIPv6 is the second IPv6 loopback address the suite binds.
const LoopbackIPv6 = "fd00::2"

const loopbackPeerIPv6 = "fd00::3"
const loopbackLinkLocalIPv6 = "fe80::1"

// loopbackIPv4Darwin contains 127.0.0.2 through 127.0.0.6. Current multi-peer
// fixtures bind these addresses: 127.0.0.6 is peer F of
// test/plugin/rfc7999-blackhole-readvertise-agreement.ci. docs/guide/chaos-testing.md also instructs a
// user to add them manually for FRR and BIRD chaos runs. Those daemons identify
// peers by source address, not by port.
var loopbackIPv4Darwin = []string{"127.0.0.2", "127.0.0.3", "127.0.0.4", "127.0.0.5", "127.0.0.6"}

// loopbackAddresses answers the addresses this host must carry for the
// functional suite to run.
//
// Linux is IPv6-only here: 127.0.0.0/8 already routes to lo, so an IPv4 alias
// would be work with no effect.
func (s *Setup) loopbackAddresses() []string {
	if s.goos() == osDarwin {
		return append(slices.Clone(loopbackIPv4Darwin), LoopbackIPv6, loopbackPeerIPv6, loopbackLinkLocalIPv6)
	}
	return []string{LoopbackIPv6, loopbackPeerIPv6, loopbackLinkLocalIPv6}
}

// loopbackInterfaceDarwin is the loopback interface name on Darwin.
const loopbackInterfaceDarwin = "lo0"

func (s *Setup) loopbackInterface() string {
	if s.goos() == osDarwin {
		return loopbackInterfaceDarwin
	}
	return "lo"
}

// loopbackPrefixes reads only loopback, because an address on another interface
// does not prove that the global next hop and its link-local share an owner/link.
func (s *Setup) loopbackPrefixes() ([]netip.Prefix, error) {
	if s.LoopbackPrefixes != nil {
		return s.LoopbackPrefixes()
	}
	iface, err := net.InterfaceByName(s.loopbackInterface())
	if err != nil {
		return nil, err
	}
	addresses, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	prefixes := make([]netip.Prefix, 0, len(addresses))
	for _, address := range addresses {
		prefix, err := netip.ParsePrefix(address.String())
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

// loopbackReady checks IPv6 ownership as well as bindability.
//
// A unique-local address must sit at its exact prefix: a bind alone accepts a
// peer /128, which cannot prove the joint-subnet fixture. A link-local address
// is ready at any prefix length, because the fixtures use fe80::1 only as a
// next hop and bind address, and macOS owns it as fe80::1/64 from boot. Holding
// it to /128 left `./le setup install` pending forever on macOS (an alias does
// not replace the boot /64) and would make `ip -6 addr add` fail with EEXIST on
// a Linux lo that already holds it at another length.
func (s *Setup) loopbackReady(addr string, prefixes []netip.Prefix) bool {
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return false
	}
	if !ip.Is6() {
		return s.loopbackBindable(addr)
	}
	if ip.IsLinkLocalUnicast() {
		if !loopbackOwns(prefixes, ip) {
			return false
		}
		var tb textbuf.Buffer
		return s.loopbackBindable(tb.Str(addr).Byte('%').Str(s.loopbackInterface()).String())
	}
	bits := 128
	if addr == loopbackPeerIPv6 {
		bits = 127
	}
	if !slices.Contains(prefixes, netip.PrefixFrom(ip, bits)) {
		return false
	}
	return s.loopbackBindable(addr)
}

// loopbackOwns reports whether the loopback interface carries ip at any prefix
// length.
func loopbackOwns(prefixes []netip.Prefix, ip netip.Addr) bool {
	for _, prefix := range prefixes {
		if prefix.Addr() == ip {
			return true
		}
	}
	return false
}

// loopbackBindable reports whether a socket can bind addr now.
//
// This complements the IPv6 ownership/prefix check: every fixture must also bind
// successfully, and the two probes can give different results.
// An IPv6 address can be listed while duplicate-address detection still rejects
// it. The test runner uses the same method (loopbackBindable,
// internal/test/runner/loopback.go).
func (s *Setup) loopbackBindable(addr string) bool {
	if s.Bindable != nil {
		return s.Bindable(addr)
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(addr, "0")) //nolint:noctx // probe-only, no cancellation needed
	if err != nil {
		return false
	}
	listener.Close() //nolint:errcheck // probe listener, result irrelevant
	return true
}

// missingLoopback answers the addresses whose ownership, prefix or bindability
// does not satisfy the fixture requirements.
func (s *Setup) missingLoopback() []string {
	var missing []string
	prefixes, err := s.loopbackPrefixes()
	if err != nil {
		// An unreadable interface cannot establish IPv6 ownership or a subnet.
		prefixes = nil
	}
	for _, addr := range s.loopbackAddresses() {
		if !s.loopbackReady(addr, prefixes) {
			missing = append(missing, addr)
		}
	}
	return missing
}

// loopbackAddArgv answers the root command that puts addr on the loopback
// interface.
func (s *Setup) loopbackAddArgv(addr string) []string {
	suffix := "/128"
	if addr == loopbackPeerIPv6 {
		suffix = "/127"
	}
	var tb textbuf.Buffer
	host := tb.Str(addr).Str(suffix).String()
	if s.goos() == osDarwin {
		if strings.Contains(addr, ":") {
			return []string{"ifconfig", loopbackInterfaceDarwin, "inet6", host, "alias"}
		}
		return []string{"ifconfig", loopbackInterfaceDarwin, "alias", addr}
	}
	// The peer's /127 is the only non-host route these additions introduce.
	return []string{ipBin, "-6", "addr", "add", host, "dev", "lo"}
}

// noteLoopbackFix records the commands, for when root is out of reach.
func (s *Setup) noteLoopbackFix(report *Report, missing []string) {
	for _, addr := range missing {
		var tb textbuf.Buffer
		report.Note(tb.Str("  Run: sudo ").Join(s.loopbackAddArgv(addr), " ").String())
	}
}

// FunctionalHostPreflight answers nil when this host carries every loopback
// address the functional suites bind, and otherwise the refusal that names each
// missing address and the commands that add it.
//
// A functional run, a verification that holds one, and a cut sweep call it
// BEFORE any work starts. Without it, a host rebooted since its last `./le
// setup` ran a verification for two hours before the first fixture that binds
// fd00::2 failed (the runner's own per-test probe, ensureLoopbackAlias in
// internal/test/runner/loopback.go). The address set and the readiness test are
// the ones `./le setup` adds and `./le setup check` reports, so the preflight,
// setup and its check cannot disagree about what the host needs.
func FunctionalHostPreflight() error {
	var host Setup
	return host.functionalHostPreflight()
}

func (s *Setup) functionalHostPreflight() error {
	missing := s.missingLoopback()
	if len(missing) == 0 {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("this host is missing loopback addresses that the functional tests bind: ").Join(missing, ", ").
		Str("\nEvery test that binds one of them fails, so the run did not start." +
			"\nThese addresses are lost when the host reboots. Add them with:\n  ./le setup install" +
			"\nor add them by hand:")
	for _, addr := range missing {
		tb.Str("\n  sudo ").Join(s.loopbackAddArgv(addr), " ")
	}
	return errors.New(tb.String())
}

// applyLoopback records, then runs, the commands that add the missing
// addresses.
//
// Only addresses that failed the readiness probe are passed in, so a re-run on
// a configured host runs nothing. A conflicting assignment is reported, never
// silently removed. Success requires every ownership/prefix/bind probe to pass.
func (s *Setup) applyLoopback(report *Report, missing []string) bool {
	for _, addr := range missing {
		ok, detail := s.Shell.runPrivileged(report, s.loopbackAddArgv(addr), nil, "")
		if !ok {
			var tb textbuf.Buffer
			report.Note(tb.Str("  FAIL: ").Str(detail).String())
			return false
		}
	}
	return len(s.missingLoopback()) == 0
}
