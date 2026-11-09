package engine

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// socketPolicyDP answers a fixed SPD dump and a fixed guard state.
type socketPolicyDP struct {
	bypassDP
	policies []dataplane.PolicyInfo
	state    dataplane.OverrideGuardState
	stateErr error
}

func (d *socketPolicyDP) ListPolicies() ([]dataplane.PolicyInfo, error) { return d.policies, nil }

func (d *socketPolicyDP) OverrideGuardState() (dataplane.OverrideGuardState, error) {
	return d.state, d.stateErr
}

func setAppliedOverride(t *testing.T, value ipsec.ApplicationOverride) {
	t.Helper()
	previous := appliedOverride.Load()
	appliedOverride.Store(&value)
	t.Cleanup(func() { appliedOverride.Store(previous) })
}

func diagnosticCodes(diags []rpc.DoctorCheckDiagnostic) string {
	codes := make([]string, 0, len(diags))
	for _, d := range diags {
		codes = append(codes, d.Severity+":"+d.Code)
	}
	return strings.Join(codes, ",")
}

// TestDoctorApplicationOverride drives the check through each state the guard and the
// SPD can be in.
//
// VALIDATES: AC-21 and AC-17: under deny a detached guard is an error and a socket
// policy not attributable to Ze a warning; Ze's own ESP form policy is not reported;
// under allow nothing about socket policies is reported.
// PREVENTS: a guard detached behind the administrator's back going unreported, and a
// warning the administrator already accepted by choosing allow.
func TestDoctorApplicationOverride(t *testing.T) {
	_, any4, _ := net.ParseCIDR("0.0.0.0/0")
	zeOwn := dataplane.PolicyInfo{PerSocket: true, Dir: dataplane.SADirIn, Src: any4, Dst: any4, Role: dataplane.PolicyRoleESPFormSocket}
	foreign := dataplane.PolicyInfo{PerSocket: true, Dir: dataplane.SADirOut, Src: any4, Dst: any4, Role: dataplane.PolicyRoleSocketOverride}
	spdEntry := dataplane.PolicyInfo{Dir: dataplane.SADirOut, Src: any4, Dst: any4}
	attached := dataplane.OverrideGuardState{Status: dataplane.OverrideGuardAttached}
	detached := dataplane.OverrideGuardState{Status: dataplane.OverrideGuardDetached}

	warn := "warning:" + diagnostic.CodeDoctorIPsecSocketPolicy
	fail := "error:" + diagnostic.CodeDoctorIPsecOverrideGuard
	for _, tc := range []struct {
		name     string
		override ipsec.ApplicationOverride
		dp       *socketPolicyDP
		want     string
	}{
		{"deny, attached, Ze's own only", ipsec.ApplicationOverrideDeny,
			&socketPolicyDP{policies: []dataplane.PolicyInfo{zeOwn, spdEntry}, state: attached}, ""},
		{"deny, attached, a foreign socket policy", ipsec.ApplicationOverrideDeny,
			&socketPolicyDP{policies: []dataplane.PolicyInfo{zeOwn, foreign, spdEntry}, state: attached}, warn},
		{"deny, detached", ipsec.ApplicationOverrideDeny,
			&socketPolicyDP{policies: []dataplane.PolicyInfo{zeOwn}, state: detached}, fail},
		{"deny, detached, a foreign socket policy", ipsec.ApplicationOverrideDeny,
			&socketPolicyDP{policies: []dataplane.PolicyInfo{foreign}, state: detached}, fail + "," + warn},
		{"deny, the state cannot be read", ipsec.ApplicationOverrideDeny,
			&socketPolicyDP{stateErr: errors.New("link info: bad file descriptor")}, fail},
		{"deny, VPP", ipsec.ApplicationOverrideDeny,
			&socketPolicyDP{policies: []dataplane.PolicyInfo{foreign}, state: dataplane.OverrideGuardState{Status: dataplane.OverrideGuardNotApplicable}}, ""},
		{"allow, a foreign socket policy", ipsec.ApplicationOverrideAllow,
			&socketPolicyDP{policies: []dataplane.PolicyInfo{foreign}, state: detached}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loadFailClosedBackend(t, tc.dp)
			setAppliedOverride(t, tc.override)
			diags := checkIPsecApplicationOverride(registry.DoctorCheckContext{})
			if got := diagnosticCodes(diags); got != tc.want {
				t.Fatalf("diagnostics = %q, want %q (%v)", got, tc.want, diags)
			}
			if tc.want == warn && !strings.Contains(diags[0].Message, "(out, 0.0.0.0/0 to 0.0.0.0/0)") {
				t.Errorf("the warning does not name the policy: %q", diags[0].Message)
			}
		})
	}
}

// TestDoctorReportsRefusedDeny proves the doctor names a deny the apply could not
// put in place, with the apply's own refusal, and stops once an apply succeeds.
//
// VALIDATES: overrideRefusal, set by applyApplicationOverride and read by
// checkIPsecApplicationOverride; the first apply on a host with no cgroup2 mounted
// records no appliedOverride, so only the refusal can reach the doctor.
// PREVENTS: IPsec down at boot on such a host with `ze doctor` silent about why.
func TestDoctorReportsRefusedDeny(t *testing.T) {
	previous := appliedOverride.Load()
	appliedOverride.Store(nil)
	t.Cleanup(func() {
		appliedOverride.Store(previous)
		overrideRefusal.Store(nil)
	})
	dp := &socketPolicyDP{guardErr: errors.New("no cgroup2 file system is mounted (mount -t cgroup2 none /sys/fs/cgroup)")}
	loadFailClosedBackend(t, dp)

	if err := applyApplicationOverride(dp, ipsec.ApplicationOverrideDeny); err == nil {
		t.Fatal("deny applied with a guard the backend refused")
	}
	diags := checkIPsecApplicationOverride(registry.DoctorCheckContext{})
	if got, want := diagnosticCodes(diags), "error:"+diagnostic.CodeDoctorIPsecOverrideGuard; got != want {
		t.Fatalf("diagnostics = %q, want %q (%v)", got, want, diags)
	}
	for _, part := range []string{"no cgroup2 file system is mounted", "mount -t cgroup2 none /sys/fs/cgroup", "set `vpn ipsec application-override allow`"} {
		if !strings.Contains(diags[0].Message, part) {
			t.Errorf("the diagnostic does not name %q: %q", part, diags[0].Message)
		}
	}

	if err := applyApplicationOverride(dp, ipsec.ApplicationOverrideAllow); err != nil {
		t.Fatalf("allow apply: %v", err)
	}
	if diags := checkIPsecApplicationOverride(registry.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("the refusal outlived a successful apply: %v", diags)
	}
}

// TestDoctorSPDWritersReadsProc proves the writer scan over a fake procfs.
//
// VALIDATES: AC-21: another CAP_NET_ADMIN holder in Ze's network namespace is named by
// pid and command; one in another namespace, one without the capability, a kernel
// thread and Ze itself are not.
// PREVENTS: a warning that names nobody, or names Ze or every kernel thread.
func TestDoctorSPDWritersReadsProc(t *testing.T) {
	root := t.TempDir()
	proc := func(pid, ns, capEff, cmdline, comm string) {
		dir := filepath.Join(root, pid)
		if err := os.MkdirAll(filepath.Join(dir, "ns"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(ns, filepath.Join(dir, "ns", "net")); err != nil {
			t.Fatal(err)
		}
		files := map[string]string{"status": "Name:\t" + comm + "\nCapEff:\t" + capEff + "\n", "cmdline": cmdline, "comm": comm + "\n"}
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	const zeNS, otherNS = "net:[4026531840]", "net:[4026532999]"
	self := strconv.Itoa(os.Getpid())
	proc(self, zeNS, "000001ffffffffff", "ze\x00start", "ze")     // Ze itself
	proc("200", zeNS, "0000000000001000", "charon\x00", "charon") // CAP_NET_ADMIN only
	proc("300", zeNS, "0000000000000000", "sh\x00", "sh")         // no capability
	proc("400", otherNS, "000001ffffffffff", "bird\x00", "bird")  // another namespace
	proc("2", zeNS, "000001ffffffffff", "", "kthreadd")           // kernel thread
	proc("150", zeNS, "000001ffffffffff", "zebra\x00", "zebra")   // full set
	if err := os.WriteFile(filepath.Join(root, "uptime"), []byte("1 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scan, err := spdWriters(root, os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if scan.unreadable != 0 {
		t.Fatalf("unreadable = %d, want 0: every fake process is readable", scan.unreadable)
	}
	writers := scan.writers
	if len(writers) != 2 || writers[0] != (spdWriter{150, "zebra"}) || writers[1] != (spdWriter{200, "charon"}) {
		t.Fatalf("writers = %v, want [{150 zebra} {200 charon}]", writers)
	}

	previous := procRoot
	procRoot = root
	t.Cleanup(func() { procRoot = previous })
	setAppliedOverride(t, ipsec.ApplicationOverrideAllow)
	diags := checkIPsecSPDWriters(registry.DoctorCheckContext{})
	if len(diags) != 1 || diags[0].Code != diagnostic.CodeDoctorIPsecSPDWriter || diags[0].Severity != severityWarning ||
		!strings.Contains(diags[0].Message, "Processes 150 (zebra), 200 (charon) hold CAP_NET_ADMIN") {
		t.Fatalf("the check answered %v, want one warning naming 150 (zebra) and 200 (charon)", diags)
	}
	if _, err := spdWriters(filepath.Join(root, "absent"), os.Getpid()); err == nil {
		t.Fatal("a tree with no procfs answered no error")
	}
}

// TestDoctorSPDWritersReportsUnreadableProc proves the writer check says it cannot tell
// when /proc hides processes from Ze.
//
// VALIDATES: a missing /proc, and a process whose namespace Linux refuses to show, each
// raise doctor-ipsec-spd-writer-unknown naming the cause and the fix; a process that
// exited during the scan does not.
// PREVENTS: an unreadable /proc reported as "no other process can write the SPD".
func TestDoctorSPDWritersReportsUnreadableProc(t *testing.T) {
	previous := procRoot
	t.Cleanup(func() { procRoot = previous })
	setAppliedOverride(t, ipsec.ApplicationOverrideDeny)

	procRoot = filepath.Join(t.TempDir(), "absent")
	diags := checkIPsecSPDWriters(registry.DoctorCheckContext{})
	if len(diags) != 1 || diags[0].Code != diagnostic.CodeDoctorIPsecSPDWriterUnknown || diags[0].Severity != severityWarning {
		t.Fatalf("a missing /proc answered %v, want one %s warning", diags, diagnostic.CodeDoctorIPsecSPDWriterUnknown)
	}
	for _, want := range []string{"reading " + procRoot + " failed", "cannot tell", "mount -o remount,hidepid=0 /proc"} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("the warning does not say %q: %q", want, diags[0].Message)
		}
	}

	// One process whose ns/net is no symlink (readlink answers EINVAL, as a refused
	// read answers EACCES), and one with no ns/net at all (it exited during the scan).
	root := t.TempDir()
	self := strconv.Itoa(os.Getpid())
	for _, dir := range []string{filepath.Join(root, self, "ns"), filepath.Join(root, "500", "ns"), filepath.Join(root, "600")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("net:[4026531840]", filepath.Join(root, self, "ns", "net")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "500", "ns", "net"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	procRoot = root
	diags = checkIPsecSPDWriters(registry.DoctorCheckContext{})
	if len(diags) != 1 || diags[0].Code != diagnostic.CodeDoctorIPsecSPDWriterUnknown ||
		!strings.Contains(diags[0].Message, "Linux hides under "+root+": 1)") {
		t.Fatalf("one hidden process answered %v, want one %s warning counting 1", diags, diagnostic.CodeDoctorIPsecSPDWriterUnknown)
	}
}
