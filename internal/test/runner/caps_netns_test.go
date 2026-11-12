// VALIDATES: the per-test netns mode's capability grant is the systemd unit's
// set (netnsDaemonCapBits, derived from privilege.DaemonCapabilities), its
// setcap text is derived from that declaration, and
// the caps= gate in that mode judges a test against the grant ze receives
// rather than the root runner's own CapEff.
// PREVENTS: the vacuous gate that admitted every XFRM IPsec test declaring
// caps=net-admin,bpf because the root runner held CAP_BPF, while the dropped
// ze held only cap_net_admin,cap_net_raw,cap_net_bind_service and failed
// BPF_MAP_CREATE with EPERM for its application-override deny guard.
// METHOD: drive netnsActive and hasCaps through their seams, so both
// polarities run on any host, and compare the setcap text with a literal
// written independently of the derivation.

package runner

import (
	"runtime"
	"slices"
	"strings"
	"testing"
)

// withNetns forces the parse-time netns-mode seam for one test.
func withNetns(t *testing.T, active bool) {
	t.Helper()
	original := netnsActive
	netnsActive = func() bool { return active }
	t.Cleanup(func() { netnsActive = original })
}

// The grant written on the netns-mode copies of ze, ze-stripped and le is the
// declaration's own text, and carries cap_bpf.
func TestNetnsDaemonSetcapNamesTheGrant(t *testing.T) {
	got, err := NetnsDaemonSetcap()
	if err != nil {
		t.Fatalf("NetnsDaemonSetcap: %v", err)
	}
	const want = "cap_net_admin,cap_net_raw,cap_net_bind_service,cap_bpf+ep"
	if got != want {
		t.Errorf("NetnsDaemonSetcap() = %q, want %q", got, want)
	}
}

// netnsDaemonMissing answers the setcap name of each required capability the
// grant lacks, nothing for a token set the grant covers, and an error for a
// token it does not know.
func TestNetnsDaemonMissingAgainstTheGrant(t *testing.T) {
	for _, tc := range []struct {
		tokens []string
		want   []string
	}{
		{[]string{capsNetAdmin, capsBPF}, nil},
		{[]string{capsNetAdmin, capsNetRaw, capsNetBind, capsBPF}, nil},
		{[]string{capsSysTime}, []string{"cap_sys_time"}},
		{[]string{capsNetAdmin, capsSysTime, capsSysTime}, []string{"cap_sys_time"}},
	} {
		got, err := netnsDaemonMissing(tc.tokens)
		if err != nil {
			t.Fatalf("netnsDaemonMissing(%v): %v", tc.tokens, err)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("netnsDaemonMissing(%v) = %v, want %v", tc.tokens, got, tc.want)
		}
	}
	if _, err := netnsDaemonMissing([]string{"net-admn"}); err == nil {
		t.Error("netnsDaemonMissing accepted an unknown token, answering an empty miss list for it")
	}
}

// In netns mode a test declaring a capability outside the grant is skipped
// with the missing name and the declaration to edit, even though the runner's
// own probe answers present; a test inside the grant still runs.
func TestNetnsModeGateRefusesACapOutsideTheGrant(t *testing.T) {
	withNetns(t, true)

	outside := newRecord("netns-sys-time")
	if err := applyNetnsDaemonCapsGate(outside, []string{capsNetAdmin, capsSysTime}); err != nil {
		t.Fatalf("applyNetnsDaemonCapsGate: %v", err)
	}
	if !strings.Contains(outside.SkipReason, "cap_sys_time") {
		t.Errorf("skip reason %q does not name the capability ze is not granted", outside.SkipReason)
	}
	if !strings.Contains(outside.SkipReason, "netnsDaemonCapBits") {
		t.Errorf("skip reason %q does not name the declaration that grants it", outside.SkipReason)
	}

	inside := newRecord("netns-bpf")
	if err := applyNetnsDaemonCapsGate(inside, []string{capsNetAdmin, capsBPF}); err != nil {
		t.Fatalf("applyNetnsDaemonCapsGate: %v", err)
	}
	if inside.SkipReason != "" {
		t.Errorf("caps=net-admin,bpf skipped in netns mode although the grant carries both: %q", inside.SkipReason)
	}

	if err := applyNetnsDaemonCapsGate(newRecord("netns-typo"), []string{"net-admn"}); err == nil {
		t.Error("an unknown token passed the netns-mode gate")
	}
}

// Through the parser on Linux: with the runner probe answering present, netns
// mode skips caps=sys-time and runs caps=net-admin,bpf, and host mode keeps
// its old answer (the runner and ze share one credential there, so the probe
// alone decides).
func TestNeedsLinuxCapsInNetnsModeJudgeTheDaemonGrant(t *testing.T) {
	if runtime.GOOS != goosLinux {
		t.Skip("the caps gate runs after the needs-linux GOOS skip, so it is reached only on Linux")
	}
	withCaps(t, true)

	withNetns(t, true)
	if r := parseNeedsLinux(t, "option=needs-linux:caps=sys-time"); !strings.Contains(r.SkipReason, "cap_sys_time") {
		t.Errorf("netns mode admitted caps=sys-time, which the dropped ze is not granted: skip reason %q", r.SkipReason)
	}
	if r := parseNeedsLinux(t, "option=needs-linux:caps=net-admin,bpf"); r.SkipReason != "" {
		t.Errorf("netns mode skipped caps=net-admin,bpf, which the grant carries: %q", r.SkipReason)
	}

	withNetns(t, false)
	if r := parseNeedsLinux(t, "option=needs-linux:caps=sys-time"); r.SkipReason != "" {
		t.Errorf("host mode skipped caps=sys-time although the probe answered present: %q", r.SkipReason)
	}
}
