// Design: docs/architecture/system-architecture.md -- the capabilities the daemon runs with
// Overview: drop.go -- privilege package
// Related: check_linux.go -- the warning for each capability a non-root daemon lacks

package privilege

// Capability is one Linux capability the daemon needs when it does not run as
// root.
type Capability struct {
	// Name is the capability as capabilities(7) and a systemd unit spell it.
	Name string
	// Bit is its position in the kernel's capability sets: the number
	// include/uapi/linux/capability.h gives it, which /proc/<pid>/status reports
	// as a bit of CapEff and syscall.SysProcAttr.AmbientCaps takes.
	Bit uint
	// Use says what Ze needs it for, in the warning a daemon without it prints.
	Use string
}

// daemonCapabilities is the one declaration of the capabilities a non-root
// daemon runs with. The systemd unit Ze installs grants exactly this set,
// ambient and bounding alike (internal/plugins/systemd, buildUnitFile);
// CheckPrivileges warns for each one a non-root daemon lacks; and a unit
// capability test (unit_capabilities_integration_linux_test.go in the package
// that owns an eBPF program) loads it from a process holding exactly this set.
// The order is the order the unit lists them in.
//
// Each Bit is the kernel's number, written out because golang.org/x/sys/unix
// declares the CAP_ constants on Linux alone and the unit is written on every
// platform. TestDaemonCapabilityBitsAreTheKernels (capabilities_linux_test.go)
// compares each one with the unix constant of its name.
//
// CAP_BPF is there for the eBPF programs Ze loads: the IPsec ESP form tc fast
// path (SchedCLS programs, hash and per-CPU array maps), the IPsec application
// override guard (a CGroupSockopt program and a per-CPU array) and
// traffic-usage (SchedCLS programs and LRU hash maps). Read in Linux v7.0
// kernel/bpf/syscall.c: bpf_prog_load refuses every program type but a socket
// filter and cgroup skb without CAP_BPF, and its is_net_admin_prog_type check
// then asks CAP_NET_ADMIN for SchedCLS and CGroupSockopt. map_create asks
// CAP_BPF for an LRU hash, and for every map while
// kernel.unprivileged_bpf_disabled is set, as it is from boot on a kernel built
// with CONFIG_BPF_UNPRIV_DEFAULT_OFF. The attach asks for nothing more:
// link_create hands a SchedCLS program to tcx_link_attach and a CGroupSockopt
// one to cgroup_bpf_link_attach, and neither checks a capability beyond the
// descriptor it is given.
//
// CAP_PERFMON is not in the set, so the verifier judges every program by its
// unprivileged rules: bpf_check sets "env->allow_ptr_leaks =
// bpf_allow_ptr_leaks(env->prog->aux->token);", and bpf_allow_ptr_leaks,
// bpf_allow_uninit_stack, bpf_bypass_spec_v1 and bpf_bypass_spec_v4 each ask
// "bpf_token_capable(token, CAP_PERFMON)" (kernel/bpf/verifier.c,
// include/linux/bpf.h, v7.0). The unit capability tests prove each program
// passes those rules.
var daemonCapabilities = [...]Capability{
	{Name: "CAP_NET_ADMIN", Bit: 12, Use: "FIB, interfaces, eBPF tc and cgroup programs"},
	{Name: "CAP_NET_RAW", Bit: 13, Use: "ping, traceroute"},
	{Name: "CAP_NET_BIND_SERVICE", Bit: 10, Use: "port 179"},
	{Name: "CAP_BPF", Bit: 39, Use: "eBPF: IPsec ESP form fast path, IPsec override guard, traffic-usage"},
}

// DaemonCapabilitiesFix is the one command line that gives a systemd-run daemon the
// whole set: it stops ze, because the install refuses while the daemon holds the
// store, then rewrites the ze unit from DaemonCapabilities, reloads systemd and starts
// ze. The && runs the install only once the stop succeeded, so the line works as it is.
const DaemonCapabilitiesFix = "sudo systemctl stop ze && sudo ze install systemd --force --start"

// DaemonCapabilitiesAdvice says how a daemon that lacks a capability of the set gets
// it, under systemd and without it. The start warning (cmd/ze/hub) and the
// daemon-capabilities doctor check both carry it.
const DaemonCapabilitiesAdvice = "Under systemd, a ze unit written by an older ze does not " +
	"grant CAP_BPF: " + DaemonCapabilitiesFix + " stops ze with sudo systemctl stop ze, " +
	"because the install refuses while ze runs, then rewrites the unit (add the --config " +
	"<dir> the unit was first installed with to the install), reloads systemd and starts ze " +
	"again. Without systemd, start ze with the capabilities named."

// DaemonCapabilities answers the capabilities a non-root daemon runs with, in
// the order the systemd unit lists them. The slice is a fresh copy, so a caller
// may change it. Safe for concurrent use.
func DaemonCapabilities() []Capability {
	capabilities := daemonCapabilities
	return capabilities[:]
}
