// Design: docs/architecture/system-architecture.md -- capabilities without root
// Related: membership.go -- DaemonServiceMembership and CapabilityRemedy, under test

package privilege

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMembershipFromCgroup reads the membership from /proc/<pid>/cgroup texts.
//
// VALIDATES: a cgroup v2 line whose path ends in ze.service is MembershipZeService;
// another unit, a session scope and the namespace root are MembershipOther; a file
// with only cgroup v1 lines, or none, is MembershipUnknown with a reason.
// PREVENTS: a systemd command offered to a process systemd did not start from
// ze.service, or a process that could not be read treated as one that was.
// METHOD: membershipFromCgroup over literal file bodies, hybrid hierarchies included.
func TestMembershipFromCgroup(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want ServiceMembership
	}{
		{name: "ze.service", body: "0::/system.slice/ze.service\n", want: MembershipZeService},
		{name: "hybrid with ze.service", body: "12:pids:/system.slice/ze.service\n1:name=systemd:/system.slice/ze.service\n0::/system.slice/ze.service\n", want: MembershipZeService},
		{name: "another unit", body: "0::/system.slice/frr.service\n", want: MembershipOther},
		{name: "a unit whose name only ends like it", body: "0::/system.slice/notze.service\n", want: MembershipOther},
		{name: "ze.service as a parent of the leaf", body: "0::/system.slice/ze.service/payload\n", want: MembershipOther},
		{name: "a login session", body: "0::/user.slice/user-1000.slice/session-3.scope\n", want: MembershipOther},
		{name: "the namespace root", body: "0::/\n", want: MembershipOther},
		{name: "cgroup v1 only", body: "12:pids:/system.slice/ze.service\n1:name=systemd:/system.slice/ze.service\n", want: MembershipUnknown},
		{name: "empty", body: "", want: MembershipUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := membershipFromCgroup([]byte(tc.body))
			if got != tc.want {
				t.Fatalf("membership = %s, want %s", got, tc.want)
			}
			if tc.want == MembershipUnknown && err == nil {
				t.Errorf("an Unknown membership carries no reason")
			}
			if tc.want != MembershipUnknown && err != nil {
				t.Errorf("membership %s carries an error: %v", got, err)
			}
		})
	}
}

// TestMembershipFromFileUnreadable asks for the membership from a file that
// does not exist.
//
// VALIDATES: an unreadable cgroup file is MembershipUnknown, with a reason that
// names the file.
// PREVENTS: a read failure answered as MembershipOther, a guess presented as a fact.
func TestMembershipFromFileUnreadable(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cgroup")
	data, readErr := os.ReadFile(file)
	got, err := membershipFromRead(file, data, readErr)
	if got != MembershipUnknown {
		t.Fatalf("membership = %s, want %s", got, MembershipUnknown)
	}
	if err == nil || !strings.Contains(err.Error(), file) {
		t.Errorf("error = %v, want one naming %s", err, file)
	}
}

// TestCapabilityRemedy reads the remedy each membership gets.
//
// VALIDATES: under ze.service the remedy for capabilities the unit grants is
// DaemonCapabilitiesFix, and for one it does not grant a drop-in edit of the
// unit; any other process, and one whose membership is unknown, is told to
// restart ze as root or holding the named capabilities, with no systemd command,
// and the unknown one also says Ze could not tell and why; no name means the
// whole set.
// PREVENTS: an operator running ze from a shell or another supervisor told to
// reinstall a systemd unit that never started it.
// METHOD: capabilityRemedy per membership, and CapabilityRemedy through the
// SetServiceMembershipForTest seam.
func TestCapabilityRemedy(t *testing.T) {
	cgroupErr := errors.New("cannot read /proc/self/cgroup: permission denied")
	for _, tc := range []struct {
		name       string
		membership ServiceMembership
		err        error
		missing    []string
		want       string
		contains   []string
	}{
		{name: "ze.service, one of the set", membership: MembershipZeService, missing: []string{"CAP_BPF"}, want: DaemonCapabilitiesFix},
		{name: "ze.service, the whole set", membership: MembershipZeService, want: DaemonCapabilitiesFix},
		{name: "ze.service, outside the set", membership: MembershipZeService, missing: []string{"CAP_BPF", "CAP_SYS_ADMIN"}, contains: []string{
			"ze.service does not grant CAP_SYS_ADMIN", "sudo systemctl edit ze", "AmbientCapabilities=", "sudo systemctl restart ze",
		}},
		{name: "other, two named", membership: MembershipOther, missing: []string{"CAP_BPF", "CAP_NET_ADMIN"}, contains: []string{
			"restart ze as root, or holding CAP_BPF and CAP_NET_ADMIN", "ambient capabilities", "sudo setcap cap_bpf,cap_net_admin+ep",
		}},
		{name: "other, the whole set", membership: MembershipOther, contains: []string{
			"CAP_NET_ADMIN, CAP_NET_RAW, CAP_NET_BIND_SERVICE and CAP_BPF",
		}},
		{name: "unknown", membership: MembershipUnknown, err: cgroupErr, missing: []string{"CAP_NET_RAW"}, contains: []string{
			"Ze could not tell whether it runs under ze.service", "permission denied", "restart ze as root, or holding CAP_NET_RAW",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := SetServiceMembershipForTest(tc.membership, tc.err)
			t.Cleanup(restore)
			got := CapabilityRemedy(tc.missing...)
			if tc.want != "" && got != tc.want {
				t.Errorf("remedy = %q, want %q", got, tc.want)
			}
			for _, text := range tc.contains {
				if !strings.Contains(got, text) {
					t.Errorf("remedy does not carry %q: %q", text, got)
				}
			}
			if tc.membership != MembershipZeService && strings.Contains(got, "systemctl") {
				t.Errorf("membership %s is offered a systemd command: %q", tc.membership, got)
			}
			if tc.membership != MembershipZeService && strings.Contains(got, "ze install systemd") {
				t.Errorf("membership %s is told to reinstall the unit: %q", tc.membership, got)
			}
		})
	}
}

// TestDaemonServiceMembershipSeam checks the test seam answers through the
// public probe and restores the real one.
//
// VALIDATES: DaemonServiceMembership answers what SetServiceMembershipForTest
// set, and the probe in force before it after restore.
// PREVENTS: a seam that a test in another package sets and that leaks into the
// next test.
func TestDaemonServiceMembershipSeam(t *testing.T) {
	before, beforeErr := DaemonServiceMembership()
	restore := SetServiceMembershipForTest(MembershipOther, nil)
	got, err := DaemonServiceMembership()
	if got != MembershipOther || err != nil {
		t.Errorf("membership = %s, %v; want %s, nil", got, err, MembershipOther)
	}
	restore()
	after, afterErr := DaemonServiceMembership()
	if after != before || (afterErr == nil) != (beforeErr == nil) {
		t.Errorf("after restore = %s, %v; want %s, %v", after, afterErr, before, beforeErr)
	}
	if before == MembershipUnspecified {
		t.Errorf("DaemonServiceMembership answered %s", before)
	}
}

// TestServiceRemedy reads ServiceRemedy across all three memberships.
//
// VALIDATES: MembershipZeService answers service; MembershipOther answers
// other; MembershipUnknown prefixes the uncertainty sentence (with the read
// error when present) before other, and never offers service.
// PREVENTS: a systemd command reaching a process outside ze.service, or a
// membership read failure presented as known.
func TestServiceRemedy(t *testing.T) {
	const service = "sudo systemctl stop ze && sudo modprobe -r esp4_offload && sudo systemctl start ze"
	const other = "stop ze, run sudo modprobe -r esp4_offload, then start ze again"
	readErr := errors.New("no cgroup v2 line")
	for _, tc := range []struct {
		name       string
		membership ServiceMembership
		err        error
		want       string
	}{
		{name: "ze.service", membership: MembershipZeService, want: service},
		{name: "other", membership: MembershipOther, want: other},
		{name: "unknown with error", membership: MembershipUnknown, err: readErr, want: "Ze could not tell whether it runs under ze.service (no cgroup v2 line); " + other},
		{name: "unknown without error", membership: MembershipUnknown, want: "Ze could not tell whether it runs under ze.service; " + other},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(SetServiceMembershipForTest(tc.membership, tc.err))
			if got := ServiceRemedy(service, other); got != tc.want {
				t.Errorf("ServiceRemedy = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRestartRemedy reads RestartRemedy across all three memberships.
//
// VALIDATES: MembershipZeService answers "sudo systemctl restart ze";
// MembershipOther and MembershipUnknown never offer systemctl and never tell
// the operator that `ze signal restart` alone restarts an unsupervised ze.
// PREVENTS: an operator running ze outside ze.service told to run systemctl,
// or told to run a command that stops ze without starting it again.
func TestRestartRemedy(t *testing.T) {
	t.Run("ze.service", func(t *testing.T) {
		t.Cleanup(SetServiceMembershipForTest(MembershipZeService, nil))
		if got := RestartRemedy(); got != "sudo systemctl restart ze" {
			t.Errorf("RestartRemedy = %q, want %q", got, "sudo systemctl restart ze")
		}
	})
	t.Run("other", func(t *testing.T) {
		t.Cleanup(SetServiceMembershipForTest(MembershipOther, nil))
		got := RestartRemedy()
		if strings.Contains(got, "systemctl") {
			t.Errorf("RestartRemedy outside ze.service carries systemctl: %q", got)
		}
		for _, want := range []string{"no single command restarts an unsupervised ze", "ze signal stop", "ze start"} {
			if !strings.Contains(got, want) {
				t.Errorf("RestartRemedy = %q, want %q", got, want)
			}
		}
	})
	t.Run("unknown", func(t *testing.T) {
		t.Cleanup(SetServiceMembershipForTest(MembershipUnknown, errors.New("cgroup unreadable")))
		got := RestartRemedy()
		if strings.Contains(got, "systemctl") {
			t.Errorf("RestartRemedy with unknown membership carries systemctl: %q", got)
		}
		for _, want := range []string{"Ze could not tell whether it runs under ze.service (cgroup unreadable)", "ze signal stop"} {
			if !strings.Contains(got, want) {
				t.Errorf("RestartRemedy = %q, want %q", got, want)
			}
		}
	})
}
