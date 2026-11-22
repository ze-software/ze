// Design: docs/architecture/system-architecture.md -- capabilities without root
// Overview: drop.go -- privilege package
// Related: capabilities.go -- DaemonCapabilities and DaemonCapabilitiesFix, the set and the unit repair
// Related: membership_linux.go -- readServiceMembership, the /proc/self/cgroup read
// Related: membership_other.go -- readServiceMembership off Linux

package privilege

import (
	"errors"
	"path"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ServiceUnit is the systemd unit the Ze installer writes
// (internal/plugins/systemd, serviceName). It is declared here, once, because
// two sides need the same name: the installer that writes the unit, and
// DaemonServiceMembership, which decides from it whether the systemd repair
// applies to this process.
const ServiceUnit = "ze.service"

// selfCgroupPath is the file the kernel answers this process's cgroups from
// (cgroups(7), "/proc/pid/cgroup").
const selfCgroupPath = "/proc/self/cgroup"

// ServiceMembership says whether this process runs in the cgroup of the unit
// the Ze installer writes, which decides whether a systemd command can repair
// what it lacks.
type ServiceMembership uint8

const (
	// MembershipUnspecified is the zero value: no probe answered it.
	MembershipUnspecified ServiceMembership = iota
	// MembershipZeService is a process whose cgroup v2 path ends in ServiceUnit:
	// systemd started it from the unit the installer writes.
	MembershipZeService
	// MembershipOther is a process whose cgroup v2 path was read and is not
	// ServiceUnit's: a shell, another unit, a container, a test harness.
	MembershipOther
	// MembershipUnknown is a process whose membership could not be read: no
	// cgroup v2 line, an unreadable file, or a platform without cgroups.
	MembershipUnknown
)

// String names the membership for a human. Never compare with it.
func (m ServiceMembership) String() string {
	switch m {
	case MembershipUnspecified:
		return "unspecified"
	case MembershipZeService:
		return ServiceUnit
	case MembershipOther:
		return "other"
	case MembershipUnknown:
		return "unknown"
	}
	return "invalid"
}

// membershipProbe answers DaemonServiceMembership. It is a variable so a test
// in any package fixes the answer through SetServiceMembershipForTest instead
// of depending on how the test process was started.
var membershipProbe = readServiceMembership

// DaemonServiceMembership answers whether this process runs under ServiceUnit,
// read from the cgroup v2 line of /proc/self/cgroup. It answers
// MembershipUnknown with the reason whenever it cannot tell, and never
// MembershipUnspecified. Safe for concurrent use.
func DaemonServiceMembership() (ServiceMembership, error) {
	return membershipProbe()
}

// SetServiceMembershipForTest makes DaemonServiceMembership, and so
// CapabilityRemedy, answer membership and err until restore runs. A test MUST
// call restore (t.Cleanup(restore)), and MUST NOT run in parallel with another
// test that reads the membership.
func SetServiceMembershipForTest(membership ServiceMembership, err error) (restore func()) {
	if membership == MembershipUnspecified {
		panic("BUG: SetServiceMembershipForTest given MembershipUnspecified, which DaemonServiceMembership never answers")
	}
	saved := membershipProbe
	membershipProbe = func() (ServiceMembership, error) { return membership, err }
	return func() { membershipProbe = saved }
}

// membershipFromRead turns the result of reading the cgroup file at file into a
// ServiceMembership, formatting a read failure with file so the operator sees
// why membership is unknown.
func membershipFromRead(file string, data []byte, err error) (ServiceMembership, error) {
	if err != nil {
		var tb textbuf.Buffer
		return MembershipUnknown, errors.New(tb.Str("cannot read ").Str(file).Str(": ").Err(err).String())
	}
	return membershipFromCgroup(data)
}

// membershipFromCgroup answers the membership the cgroup v2 line ("0::<path>")
// of a /proc/<pid>/cgroup file names: ZeService when the path's last element is
// ServiceUnit, Other for any other path. A file with no cgroup v2 line, a host
// on cgroup v1 alone, answers Unknown, because a v1 hierarchy line does not say
// which unit systemd started the process from.
//
// The loop is bounded by the file the kernel wrote: one line per hierarchy.
func membershipFromCgroup(data []byte) (ServiceMembership, error) {
	for line := range strings.SplitSeq(string(data), "\n") {
		cgroup, found := strings.CutPrefix(line, "0::")
		if !found {
			continue
		}
		if path.Base(strings.TrimSpace(cgroup)) == ServiceUnit {
			return MembershipZeService, nil
		}
		return MembershipOther, nil
	}
	return MembershipUnknown, errors.New(selfCgroupPath + " holds no cgroup v2 line (0::<path>), so the unit ze runs under cannot be read")
}

// ServiceRemedy answers service, a remedy that runs through the ze systemd
// unit, only when DaemonServiceMembership establishes that this process runs
// under ServiceUnit. Any other process is answered other, because a command on
// the unit changes nothing for a ze the unit did not start, and one whose
// membership cannot be read is told that first, then other. Safe for
// concurrent use.
func ServiceRemedy(service, other string) string {
	membership, err := DaemonServiceMembership()
	return serviceRemedy(membership, err, service, other)
}

// serviceRemedy is ServiceRemedy for a membership already read.
func serviceRemedy(membership ServiceMembership, membershipErr error, service, other string) string {
	switch membership {
	case MembershipZeService:
		return service
	case MembershipOther:
		return other
	case MembershipUnknown:
		var tb textbuf.Buffer
		tb.Str("Ze could not tell whether it runs under ").Str(ServiceUnit)
		if membershipErr != nil {
			tb.Str(" (").Err(membershipErr).Byte(')')
		}
		tb.Str("; ").Str(other)
		return tb.String()
	case MembershipUnspecified:
		panic("BUG: serviceRemedy given MembershipUnspecified, which DaemonServiceMembership never answers")
	}
	panic("BUG: serviceRemedy given an undeclared ServiceMembership")
}

// The two daemon-restart remedies RestartRemedy chooses between.
// `ze signal restart` exits with status 75 without starting a new process
// (cmd/ze/hub, runYANGConfig; pkg/plugin/sdk/restart_text.go), so no single
// command restarts an unsupervised ze.
const (
	restartServiceRemedy = "sudo systemctl restart ze"
	restartOtherRemedy   = "no single command restarts an unsupervised ze: run ze signal stop, then start ze again the way it was started (ze start, or ze start <config-file>)"
)

// RestartRemedy answers how to restart this ze process: sudo systemctl restart
// ze only when DaemonServiceMembership establishes that it runs under
// ServiceUnit, and the stop-and-start steps otherwise. Safe for concurrent use.
func RestartRemedy() string {
	return ServiceRemedy(restartServiceRemedy, restartOtherRemedy)
}

// CapabilityRemedy answers the one operator remedy for a ze process that lacks
// the capabilities named (as capabilities(7) spells them: "CAP_BPF"), so every
// site that reports a missing capability words it the same. With no name it
// answers for the whole set DaemonCapabilities declares.
//
// The systemd repair is offered only when DaemonServiceMembership establishes
// that this process runs under ServiceUnit: DaemonCapabilitiesFix when the unit
// grants every capability named, a drop-in edit of the unit when one is outside
// the set it grants. Any other process, and one whose membership cannot be
// read, is told to restart ze as root or holding those capabilities, with no
// systemd command, because a unit reinstall would change nothing for it.
// Safe for concurrent use.
func CapabilityRemedy(missing ...string) string {
	names := missing
	if len(names) == 0 {
		names = daemonCapabilityNames()
	}
	return ServiceRemedy(serviceCapabilityRemedy(names), processRemedy(names))
}

// serviceCapabilityRemedy is the systemd-unit remedy for missing capabilities:
// DaemonCapabilitiesFix when the unit grants all of names, or a drop-in edit
// naming the capabilities outside the unit's set.
func serviceCapabilityRemedy(names []string) string {
	outside := namesOutsideDaemonSet(names)
	if len(outside) == 0 {
		return DaemonCapabilitiesFix
	}
	var tb textbuf.Buffer
	tb.Str(ServiceUnit).Str(" does not grant ")
	writeNames(&tb, outside)
	tb.Str(": add ")
	writeNames(&tb, outside)
	tb.Str(" to AmbientCapabilities= and CapabilityBoundingSet= with sudo systemctl edit ze, then run sudo systemctl restart ze")
	return tb.String()
}

// processRemedy is the remedy for a ze process systemd did not start from
// ServiceUnit: capabilities reach a process only when it starts, so it names the
// restart and the two ways to hand the capabilities over.
func processRemedy(names []string) string {
	var tb textbuf.Buffer
	tb.Str("restart ze as root, or holding ")
	writeNames(&tb, names)
	if len(names) == 1 {
		tb.Str(": grant it as an ambient capability")
	} else {
		tb.Str(": grant them as ambient capabilities")
	}
	tb.Str(" from whatever starts ze, or run sudo setcap ")
	for i, name := range names {
		if i > 0 {
			tb.Byte(',')
		}
		tb.Str(strings.ToLower(name))
	}
	tb.Str("+ep on the ze binary")
	return tb.String()
}

// writeNames writes names as an English list: "A", "A and B", "A, B and C".
func writeNames(tb *textbuf.Buffer, names []string) {
	for i, name := range names {
		switch {
		case i == 0:
		case i == len(names)-1:
			tb.Str(" and ")
		default:
			tb.Str(", ")
		}
		tb.Str(name)
	}
}

// daemonCapabilityNames answers the names of DaemonCapabilities, in its order.
func daemonCapabilityNames() []string {
	names := make([]string, 0, len(daemonCapabilities))
	for _, c := range daemonCapabilities {
		names = append(names, c.Name)
	}
	return names
}

// namesOutsideDaemonSet answers the names the unit's set does not hold.
func namesOutsideDaemonSet(names []string) []string {
	var outside []string
	for _, name := range names {
		held := false
		for _, c := range daemonCapabilities {
			if c.Name == name {
				held = true
				break
			}
		}
		if !held {
			outside = append(outside, name)
		}
	}
	return outside
}
