//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the application override guard
// Overview: override_guard.go -- the backend seam and the state it reports
// Related: espform_linux.go -- the one socket policy the guard lets through
// RFC: rfc/short/rfc4301.md -- RFC4301-4.4.1-8, the administrator's override control

package dataplane

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/link"
	"golang.org/x/sys/unix"
)

// The struct bpf_sockopt context a cgroup setsockopt program reads
// (include/uapi/linux/bpf.h): three 8-byte pointers (sk, optval, optval_end), then
// level and optname as 32-bit integers.
//
//	 0                   8                  16                  24        28        32
//	+-------------------+-------------------+-------------------+---------+---------+
//	| sk                | optval            | optval_end        | level   | optname | ...
//	+-------------------+-------------------+-------------------+---------+---------+
const (
	sockoptLevelOffset   = 24
	sockoptOptnameOffset = 28
)

// The struct bpf_pidns_info bpf_get_ns_current_pid_tgid fills (include/uapi/linux/bpf.h),
// kept on the program's stack below the counter key at fp-4.
//
//	fp-16     fp-12     fp-8
//	+---------+---------+
//	| pid     | tgid    |
//	+---------+---------+
const (
	pidnsInfoOffset     = -16
	pidnsInfoSize       = 8
	pidnsInfoTgidOffset = 4
)

// Names of the guard's kernel objects. A BPF object name is at most 15 characters.
const (
	overrideGuardProgName = "ze_ipsec_guard"
	overrideGuardMapName  = "ze_ipsec_refuse"
	// overrideGuardLicense is GPL because bpf_get_netns_cookie and the map helpers
	// are offered to any license, but the kernel refuses a GPL-only helper to any
	// other, and one license keeps the program loadable as helpers are added.
	overrideGuardLicense = "GPL"
)

// overrideGuard holds the loaded program, its refusal counter and the link that
// attaches it. The zero value holds nothing. Safe for concurrent use.
type overrideGuard struct {
	mu      sync.Mutex
	program *ebpf.Program
	counter *ebpf.Map
	cgroup  link.Link
}

// AttachOverrideGuard loads and attaches the guard once; a second call while it is
// attached does nothing.
func (b *xfrmBackend) AttachOverrideGuard() error {
	return b.guard.attach()
}

// DetachOverrideGuard closes the link, then the program and the counter.
func (b *xfrmBackend) DetachOverrideGuard() error {
	return b.guard.detach()
}

// OverrideGuardState reads whether the link still reaches its cgroup, and the
// refusal counter.
func (b *xfrmBackend) OverrideGuardState() (OverrideGuardState, error) {
	return b.guard.state()
}

// attach puts the guard in force.
//
// RFC 4301 Section 4.4.1: "However, the system administrator MUST be able to specify
// whether or not a user or application can override (default) system policies."
// On Linux an application overrides the SPD by setting a per-socket policy, which
// xfrm_lookup_with_ifid and __xfrm_policy_check consult before any SPD entry. The
// program refuses that setsockopt (do_sock_setsockopt runs it before the protocol
// handler) to every process in Ze's network namespace except this one, whose
// dataplane sets the ESP form reader's policy. The exemption names this process
// by its tgid in its own PID namespace, read through that namespace's device and
// inode: in a container the tgid the initial namespace sees is another number.
func (g *overrideGuard) attach() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cgroup != nil {
		return nil
	}

	cookie, err := netnsCookie()
	if err != nil {
		return err
	}
	pidns, err := currentPIDNamespace()
	if err != nil {
		return err
	}
	root, err := cgroup2Root()
	if err != nil {
		return err
	}

	counter, err := ebpf.NewMap(&ebpf.MapSpec{
		Name:       overrideGuardMapName,
		Type:       ebpf.PerCPUArray,
		KeySize:    4,
		ValueSize:  8,
		MaxEntries: 1,
	})
	if err != nil {
		return fmt.Errorf("create the refusal counter: %w", err)
	}
	program, err := ebpf.NewProgram(&ebpf.ProgramSpec{
		Name:         overrideGuardProgName,
		Type:         ebpf.CGroupSockopt,
		AttachType:   ebpf.AttachCGroupSetsockopt,
		Instructions: overrideGuardInstructions(cookie, pidns, uint32(os.Getpid()), counter.FD()),
		License:      overrideGuardLicense,
	})
	if err != nil {
		counter.Close() //nolint:errcheck // the load already failed; that error is the one returned
		return fmt.Errorf("load the setsockopt guard: %w", err)
	}
	cgroup, err := link.AttachCgroup(link.CgroupOptions{
		Path:    root,
		Attach:  ebpf.AttachCGroupSetsockopt,
		Program: program,
	})
	if err != nil {
		program.Close() //nolint:errcheck // the attach already failed; that error is the one returned
		counter.Close() //nolint:errcheck // the attach already failed; that error is the one returned
		return fmt.Errorf("attach the setsockopt guard to %s: %w", root, err)
	}

	g.program, g.counter, g.cgroup = program, counter, cgroup
	return nil
}

// detach closes the link first, so no call meets a program whose counter is gone.
func (g *overrideGuard) detach() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cgroup == nil {
		return nil
	}
	linkErr := g.cgroup.Close()
	programErr := g.program.Close()
	counterErr := g.counter.Close()
	g.program, g.counter, g.cgroup = nil, nil, nil
	return errors.Join(linkErr, programErr, counterErr)
}

// state answers Detached when no link is held, or when the held link no longer
// reaches a cgroup: the kernel reports cgroup id 0 for a cgroup link something
// detached (bpf_cgroup_link_fill_link_info). A process holding CAP_SYS_ADMIN can do
// that, which is why doctor reads this rather than trusting the attach.
func (g *overrideGuard) state() (OverrideGuardState, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cgroup == nil {
		return OverrideGuardState{Status: OverrideGuardDetached}, nil
	}
	refusals, err := sumPerCPUCounter(g.counter)
	if err != nil {
		return OverrideGuardState{}, err
	}
	info, err := g.cgroup.Info()
	if err != nil {
		return OverrideGuardState{}, fmt.Errorf("read the setsockopt guard link: %w", err)
	}
	cgroupInfo := info.Cgroup()
	if cgroupInfo == nil {
		return OverrideGuardState{}, errors.New("the setsockopt guard link is not a cgroup link")
	}
	if cgroupInfo.CgroupId == 0 {
		return OverrideGuardState{Status: OverrideGuardDetached, Refusals: refusals}, nil
	}
	return OverrideGuardState{Status: OverrideGuardAttached, Refusals: refusals}, nil
}

// sumPerCPUCounter reads slot 0 of the per-CPU refusal counter on every CPU.
func sumPerCPUCounter(counter *ebpf.Map) (uint64, error) {
	var perCPU []uint64
	if err := counter.Lookup(uint32(0), &perCPU); err != nil {
		return 0, fmt.Errorf("read the setsockopt guard counter: %w", err)
	}
	var total uint64
	for _, n := range perCPU {
		total += n
	}
	return total, nil
}

// netnsCookie answers the cookie of this process's network namespace, the value
// bpf_get_netns_cookie returns for a socket in it.
func netnsCookie() (uint64, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return 0, fmt.Errorf("open a socket to read the network namespace cookie: %w", err)
	}
	defer unix.Close(fd) //nolint:errcheck // a probe socket; nothing was written through it
	cookie, err := unix.GetsockoptUint64(fd, unix.SOL_SOCKET, unix.SO_NETNS_COOKIE)
	if err != nil {
		return 0, fmt.Errorf("read the network namespace cookie: %w", err)
	}
	return cookie, nil
}

// pidNamespace names a PID namespace the way bpf_get_ns_current_pid_tgid takes it:
// the device of the nsfs mount in the kernel's own dev_t encoding, and the
// namespace's inode.
type pidNamespace struct {
	dev   uint64
	inode uint64
}

// currentPIDNamespace answers the PID namespace this process runs in, read from
// /proc/self/ns/pid.
func currentPIDNamespace() (pidNamespace, error) {
	var st unix.Stat_t
	if err := unix.Stat("/proc/self/ns/pid", &st); err != nil {
		return pidNamespace{}, fmt.Errorf("read this process's PID namespace from /proc/self/ns/pid: %w", err)
	}
	return pidNamespace{dev: kernelDev(st.Dev), inode: st.Ino}, nil
}

// kernelDev converts a device number as stat(2) reports it to the kernel's dev_t
// (include/linux/kdev_t.h MKDEV: the major above bit 20, the minor below), which
// ns_match compares against the nsfs superblock. The two encodings agree only while
// the minor is below 256, and the nsfs minor is allocated at boot, so the
// conversion is spelled out rather than assumed.
func kernelDev(statDev uint64) uint64 {
	return uint64(unix.Major(statDev))<<20 | uint64(unix.Minor(statDev))
}

// errNoCgroup2 is the answer when no cgroup2 file system is mounted.
var errNoCgroup2 = errors.New("no cgroup2 file system is mounted (mount -t cgroup2 none /sys/fs/cgroup)")

// selfMountinfo is the calling process's mount table (proc(5)).
const selfMountinfo = "/proc/self/mountinfo"

// cgroup2Root answers the mount point of the cgroup2 hierarchy, read from
// selfMountinfo rather than assumed: a distribution host with a hybrid cgroup layout
// mounts it at /sys/fs/cgroup/unified, and gokrazy at /sys/fs/cgroup.
func cgroup2Root() (string, error) {
	f, err := os.Open(selfMountinfo)
	if err != nil {
		return "", fmt.Errorf("read %s to find the cgroup2 mount: %w", selfMountinfo, err)
	}
	defer f.Close() //nolint:errcheck // read only
	return cgroup2RootFrom(f)
}

// cgroup2RootFrom scans mountinfo lines (proc(5)): field 5 is the mount point, and
// the file system type is the first field after the " - " separator. The scan is
// bounded by the file, which the kernel writes.
func cgroup2RootFrom(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		before, after, found := strings.Cut(line, " - ")
		if !found {
			continue
		}
		fsType, _, _ := strings.Cut(after, " ")
		if fsType != "cgroup2" {
			continue
		}
		fields := strings.Fields(before)
		if len(fields) < 5 {
			continue
		}
		return fields[4], nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan mountinfo: %w", err)
	}
	return "", errNoCgroup2
}

// overrideGuardInstructions is the guard program. It answers 1 (let the call
// through) for every setsockopt but the four per-socket IPsec policy options, and
// for those when the socket is outside Ze's network namespace or the caller is the
// process that loaded the guard, named by its tgid within its own PID namespace. Otherwise it counts the refusal and answers 0,
// which the kernel turns into EPERM (kernel/bpf/cgroup.c bpf_prog_run_array_cg).
// The level compare comes first, so every other setsockopt on the host costs two
// loads and two compares.
func overrideGuardInstructions(cookie uint64, pidns pidNamespace, tgid uint32, counterFD int) asm.Instructions {
	return asm.Instructions{
		asm.Mov.Reg(asm.R6, asm.R1),
		asm.LoadMem(asm.R2, asm.R6, sockoptLevelOffset, asm.Word),
		asm.LoadMem(asm.R3, asm.R6, sockoptOptnameOffset, asm.Word),
		asm.JEq.Imm(asm.R2, unix.SOL_IP, "ipv4"),
		asm.JEq.Imm(asm.R2, unix.SOL_IPV6, "ipv6"),
		asm.Ja.Label("allow"),

		asm.JEq.Imm(asm.R3, unix.IP_IPSEC_POLICY, "guarded").WithSymbol("ipv4"),
		asm.JEq.Imm(asm.R3, unix.IP_XFRM_POLICY, "guarded"),
		asm.Ja.Label("allow"),

		asm.JEq.Imm(asm.R3, unix.IPV6_IPSEC_POLICY, "guarded").WithSymbol("ipv6"),
		asm.JEq.Imm(asm.R3, unix.IPV6_XFRM_POLICY, "guarded"),
		asm.Ja.Label("allow"),

		// Another network namespace has its own SPD, which the guard does not govern.
		asm.Mov.Reg(asm.R1, asm.R6).WithSymbol("guarded"),
		asm.FnGetNetnsCookie.Call(),
		asm.LoadImm(asm.R1, int64(cookie), asm.DWord),
		asm.JNE.Reg(asm.R0, asm.R1, "allow"),

		// The process that loaded the guard is Ze's IPsec dataplane: its ESP form
		// reader's socket policy is the implementation's own receive path. The tgid
		// is read in Ze's PID namespace (struct bpf_pidns_info {u32 pid; u32 tgid;}
		// at fp-16), because bpf_get_current_pid_tgid answers it in the initial
		// namespace, which a container's os.Getpid never equals. A caller in any
		// other PID namespace gets a non-zero answer and is refused.
		asm.LoadImm(asm.R1, int64(pidns.dev), asm.DWord),
		asm.LoadImm(asm.R2, int64(pidns.inode), asm.DWord),
		asm.Mov.Reg(asm.R3, asm.RFP),
		asm.Add.Imm(asm.R3, pidnsInfoOffset),
		asm.Mov.Imm(asm.R4, pidnsInfoSize),
		asm.FnGetNsCurrentPidTgid.Call(),
		asm.JNE.Imm(asm.R0, 0, "count"),
		asm.LoadMem(asm.R0, asm.RFP, pidnsInfoOffset+pidnsInfoTgidOffset, asm.Word),
		asm.JEq.Imm(asm.R0, int32(tgid), "allow"),

		asm.StoreImm(asm.RFP, -4, 0, asm.Word).WithSymbol("count"),
		asm.LoadMapPtr(asm.R1, counterFD),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, -4),
		asm.FnMapLookupElem.Call(),
		asm.JEq.Imm(asm.R0, 0, "deny"),
		asm.LoadMem(asm.R1, asm.R0, 0, asm.DWord),
		asm.Add.Imm(asm.R1, 1),
		asm.StoreMem(asm.R0, 0, asm.R1, asm.DWord),

		asm.Mov.Imm(asm.R0, 0).WithSymbol("deny"),
		asm.Return(),

		asm.Mov.Imm(asm.R0, 1).WithSymbol("allow"),
		asm.Return(),
	}
}
