// Design: docs/architecture/core-design.md -- machine state that is not a binary
//
// system.go is a Go port of scripts/le/devtools/system.py. It handles a kernel
// tunable, a group, and two loopback addresses. These items cannot be installed,
// so they do not belong in the tool table. Each item has three questions: What
// is its state? Which command changes it? Can that command run now?

package devsetup

import (
	"net"
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

// Userns is the unprivileged-userns restriction state.
type Userns string

const (
	// UsernsOK means unprivileged user namespaces are allowed (value 0).
	UsernsOK Userns = "ok"
	// UsernsRestricted means they are blocked (value 1).
	UsernsRestricted Userns = "restricted"
	// UsernsNA means the kernel has no such knob, so there is nothing to do.
	// A non-AppArmor host reaches this.
	UsernsNA Userns = "na"
)

// UsernsState reports whether unprivileged user namespaces are permitted. It
// also reports whether the question applies.
//
// If a knob EXISTS but cannot be READ, the result is an error, not UsernsNA.
// The script returned NA in this state. That result means "no such knob, nothing
// to do", although the knob is present. The unreadable value is unknown and CAN
// be 1. The restriction then reports [present], and the run passes on a machine
// where Chrome cannot start
// (plan/journal/zero-value-as-valid-answer.md, 2026-08-26).
func (s *Setup) UsernsState() (Userns, error) {
	raw, err := os.ReadFile(s.usernsProc())
	if os.IsNotExist(err) {
		return UsernsNA, nil
	}
	if err != nil {
		return UsernsNA, err
	}
	if strings.TrimSpace(string(raw)) == "1" {
		return UsernsRestricted, nil
	}
	return UsernsOK, nil
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

// ApplyUserns records, then runs, the commands that lift the restriction.
//
// It answers true only when the restriction is actually cleared. On any failure
// the caller falls back to naming the manual commands.
func (s *Setup) ApplyUserns(report *Report) bool {
	for _, step := range usernsSteps() {
		ok, detail := s.Shell.RunPrivileged(report, step.argv, step.stdin, step.shown)
		if !ok {
			var tb textbuf.Buffer
			report.Note(tb.Str("  FAIL: ").Str(detail).String())
			return false
		}
	}
	state, err := s.UsernsState()
	return err == nil && state == UsernsOK
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

// Kvm is whether QEMU can use KVM as this user.
type Kvm string

const (
	// KvmOK means /dev/kvm is readable and writable in this process now.
	KvmOK Kvm = "ok"
	// KvmPendingLogin means that the user IS in the kvm group, but the session
	// started before that membership changed. Group membership is fixed at login.
	KvmPendingLogin Kvm = "pending-login"
	// KvmNoGroup means the device exists and the user is not in the group.
	KvmNoGroup Kvm = "no-group"
	// KvmNA means no /dev/kvm at all (no hardware virt, or a VM without nested
	// virt). QEMU still runs under tcg, only slower, so there is nothing to
	// fix.
	KvmNA Kvm = "na"
)

// InKvmGroup reports whether the group database lists this user in the kvm
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
func (s *Setup) InKvmGroup() bool {
	if s.KvmGroupMember != nil {
		return s.KvmGroupMember()
	}
	group, err := user.LookupGroup(KVMGroup)
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

// KvmState answers whether QEMU can use KVM as this user.
func (s *Setup) KvmState() Kvm {
	device := s.kvmDev()
	if _, err := os.Stat(device); err != nil {
		return KvmNA
	}
	if deviceOpenable(device) {
		return KvmOK
	}
	if s.InKvmGroup() {
		return KvmPendingLogin
	}
	return KvmNoGroup
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

// ApplyKvm adds the invoking user to the kvm group.
//
// It answers true when the group database lists the user afterwards. That is
// NOT the same as usable: this process keeps the groups it started with, so the
// caller must still say to log back in.
func (s *Setup) ApplyKvm(report *Report) bool {
	ok, detail := s.Shell.RunPrivileged(report, []string{"usermod", "-aG", KVMGroup, s.userName()}, nil, "")
	if !ok {
		var tb textbuf.Buffer
		report.Note(tb.Str("  FAIL: ").Str(detail).String())
		return false
	}
	return s.InKvmGroup()
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
// IPv6 has exactly one loopback address, ::1, on every platform. A second address
// requires configuration. The suite uses fd00::2. The fd00::/8 range is
// unique-local (RFC 4193) and is never globally routable. Thus, a fixture that
// sends a packet toward fd00::2 cannot reach an actual destination on an actual
// network. A documentation prefix (2001:db8::/32) is globally scoped and does
// not have this property.
//
// This configuration belongs to setup because the runner cannot create it.
// SIOCAIFADDR_IN6 returns EPERM to an unprivileged process on darwin. The Linux
// route requires CAP_NET_ADMIN, but the verify gate runs as an ordinary user.
// internal/test/runner/loopback.go reports the missing address and names setup.
//
// Neither addition remains after a reboot on either platform. This behavior is
// intentional. The persistent methods use a launchd plist, netplan, or a
// systemd-networkd unit. These methods modify files that a developer's machine
// can use for other purposes. Setup can run again quickly, and check mode reports
// when the configuration is necessary.

// LoopbackIPv6 is the second IPv6 loopback address the suite binds.
const LoopbackIPv6 = "fd00::2"

// loopbackIPv4Darwin contains 127.0.0.2 through 127.0.0.5. Current multi-peer
// fixtures bind these addresses. docs/guide/chaos-testing.md also instructs a
// user to add them manually for FRR and BIRD chaos runs. Those daemons identify
// peers by source address, not by port.
var loopbackIPv4Darwin = []string{"127.0.0.2", "127.0.0.3", "127.0.0.4", "127.0.0.5"}

// LoopbackAddresses answers the addresses this host must carry for the
// functional suite to run.
//
// Linux is IPv6-only here: 127.0.0.0/8 already routes to lo, so an IPv4 alias
// would be work with no effect.
func (s *Setup) LoopbackAddresses() []string {
	if s.goos() == osDarwin {
		return append(slices.Clone(loopbackIPv4Darwin), LoopbackIPv6)
	}
	return []string{LoopbackIPv6}
}

// LoopbackBindable reports whether a socket can bind addr now.
//
// This function attempts a bind instead of scanning the interface list. Every
// fixture must bind successfully, and the two methods can give different results.
// An IPv6 address can be listed while duplicate-address detection still rejects
// it. The test runner uses the same method (loopbackBindable,
// internal/test/runner/loopback.go).
func (s *Setup) LoopbackBindable(addr string) bool {
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

// MissingLoopback answers the subset of LoopbackAddresses this host does not
// carry.
func (s *Setup) MissingLoopback() []string {
	var missing []string
	for _, addr := range s.LoopbackAddresses() {
		if !s.LoopbackBindable(addr) {
			missing = append(missing, addr)
		}
	}
	return missing
}

// loopbackAddArgv answers the root command that puts addr on the loopback
// interface.
func (s *Setup) loopbackAddArgv(addr string) []string {
	var tb textbuf.Buffer
	host := tb.Str(addr).Str("/128").String()
	if s.goos() == osDarwin {
		if strings.Contains(addr, ":") {
			return []string{"ifconfig", "lo0", "inet6", host, "alias"}
		}
		return []string{"ifconfig", "lo0", "alias", addr}
	}
	// Linux reaches this point only for IPv6. The /128 prefix keeps the address
	// as a host address and prevents a route to the remainder of fd00::/8.
	return []string{"ip", "-6", "addr", "add", host, "dev", "lo"}
}

// noteLoopbackFix records the commands, for when root is out of reach.
func (s *Setup) noteLoopbackFix(report *Report, missing []string) {
	for _, addr := range missing {
		var tb textbuf.Buffer
		report.Note(tb.Str("  Run: sudo ").Join(s.loopbackAddArgv(addr), " ").String())
	}
}

// ApplyLoopback records, then runs, the commands that add the missing
// addresses.
//
// Idempotent by construction: only addresses that failed the bind probe are
// passed in, so a re-run on a configured host runs nothing. It answers true only
// when every address binds afterwards.
func (s *Setup) ApplyLoopback(report *Report, missing []string) bool {
	for _, addr := range missing {
		ok, detail := s.Shell.RunPrivileged(report, s.loopbackAddArgv(addr), nil, "")
		if !ok {
			var tb textbuf.Buffer
			report.Note(tb.Str("  FAIL: ").Str(detail).String())
			return false
		}
	}
	return len(s.MissingLoopback()) == 0
}
