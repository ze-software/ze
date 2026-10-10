//go:build linux

// Design: docs/architecture/mpls/mpls-kernel.md -- Path MTU and label overhead
// Related: probe_linux.go -- MPLSIPMTU and MPLSIPMTUInThisNamespace, the probes these tests drive
//
// The transit MTU probe's classification, driven by errno through the request
// seam rather than by whichever kernel the host runs. The live answer on a stock
// and a patched kernel is TestMPLSIntegration_TransitPathMTUFollowsTheProbe in
// internal/plugins/fib/kernel.

package kernelcap

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// withMPLSIPMTUProbe swaps the label space read and the route request for the
// duration of a test. The request records whether it carried the metric.
func withMPLSIPMTUProbe(t *testing.T, labelSpace string, labelSpaceErr, control, metric error) *[]bool {
	t.Helper()
	originalRead, originalProbe := readFile, mplsRouteProbe
	t.Cleanup(func() { readFile, mplsRouteProbe = originalRead, originalProbe })

	readFile = func(string) ([]byte, error) { return []byte(labelSpace), labelSpaceErr }
	var sent []bool
	mplsRouteProbe = func(withMetric bool) error {
		sent = append(sent, withMetric)
		if withMetric {
			return metric
		}
		return control
	}
	return &sent
}

// VALIDATES: the probe reports the patched capability only when a request
// carrying RTA_METRICS reaches the label lookup, reports absence only when the
// metric alone is refused, and reports cannot-determine for every other answer.
// PREVENTS: a stock kernel read as patched (every transit install then fails
// with EINVAL), and a privilege or label-space fault read as an unpatched kernel
// (a patched kernel then forwards with no MTU bound).
func TestMPLSIPMTUProbeClassification(t *testing.T) {
	for name, tc := range map[string]struct {
		control, metric error
		state           State
	}{
		"patched, label free":        {unix.ENOENT, unix.ENOENT, StatePresent},
		"patched, label taken":       {unix.EEXIST, unix.EEXIST, StatePresent},
		"stock kernel":               {unix.ENOENT, unix.EINVAL, StateAbsent},
		"stock kernel, label taken":  {unix.EEXIST, unix.EINVAL, StateAbsent},
		"unprivileged":               {unix.EPERM, unix.EPERM, StateUnknown},
		"control refused":            {unix.EINVAL, unix.EINVAL, StateUnknown},
		"no AF_MPLS":                 {unix.EAFNOSUPPORT, unix.EAFNOSUPPORT, StateUnknown},
		"metric answered otherwise":  {unix.ENOENT, unix.ENOBUFS, StateUnknown},
		"control created a route":    {nil, unix.ENOENT, StateUnknown},
		"metric request created one": {unix.ENOENT, nil, StateUnknown},
	} {
		t.Run(name, func(t *testing.T) {
			sent := withMPLSIPMTUProbe(t, "100000\n", nil, tc.control, tc.metric)

			result := MPLSIPMTUInThisNamespace()
			if result.State != tc.state {
				t.Fatalf("control %v, metric %v: got %v (%v), want %v", tc.control, tc.metric, result.State, result.Reason, tc.state)
			}
			if result.State == StatePresent {
				if result.Reason != nil {
					t.Errorf("a present verdict carries a reason: %v", result.Reason)
				}
				return
			}
			if result.Reason == nil {
				t.Errorf("a %v verdict carries no reason", result.State)
			}
			if len(*sent) == 0 || (*sent)[0] {
				t.Errorf("the control request did not go first: %v", *sent)
			}
		})
	}
}

// VALIDATES: the probe sends nothing when the label space cannot hold its probe
// label, and says why rather than guessing.
// PREVENTS: a label-range EINVAL, which both kernels return before they look at
// RTA_METRICS, being read as the stock kernel's refusal of the metric.
func TestMPLSIPMTUProbeNeedsALabelSpace(t *testing.T) {
	for name, tc := range map[string]struct {
		labelSpace string
		err        error
	}{
		"label space is zero":         {"0\n", nil},
		"label space below the label": {"16\n", nil},
		"no AF_MPLS table":            {"", fs.ErrNotExist},
		"unreadable":                  {"", errors.New("permission denied")},
		"garbage":                     {"many\n", nil},
	} {
		t.Run(name, func(t *testing.T) {
			sent := withMPLSIPMTUProbe(t, tc.labelSpace, tc.err, unix.ENOENT, unix.ENOENT)

			result := MPLSIPMTUInThisNamespace()
			if result.State != StateUnknown {
				t.Fatalf("got %v (%v), want unknown", result.State, result.Reason)
			}
			if result.Reason == nil {
				t.Error("the verdict carries no reason")
			}
			if len(*sent) != 0 {
				t.Errorf("the probe sent %d requests with no label to address", len(*sent))
			}
		})
	}
}

// VALIDATES: inside its throwaway namespace the probe makes /proc/sys writable,
// sizes the label space to hold the probe label, and only then asks; a step
// that fails is reported as unknown with its reason and nothing is sent.
// Method: the remount, the sysctl write and the route request are faked, and
// the faked label space reads back what the probe wrote.
// PREVENTS: the probe asking a namespace whose label space is 0, which is every
// fresh namespace (the Docker kernel check read unknown on every kernel), and a
// failed remount or write being read as the unpatched kernel.
func TestMPLSIPMTUThrowawayNamespaceSizesTheLabelSpace(t *testing.T) {
	for name, tc := range map[string]struct {
		remountErr, writeErr error
		state                State
		written              string
	}{
		"sized, then asked": {nil, nil, StatePresent, "17"},
		"remount refused":   {unix.EPERM, nil, StateUnknown, ""},
		"write refused":     {nil, unix.EROFS, StateUnknown, ""},
	} {
		t.Run(name, func(t *testing.T) {
			sent := withMPLSIPMTUProbe(t, "0\n", nil, unix.ENOENT, unix.ENOENT)
			originalWrite, originalRemount := writeFile, mplsProbeWritableSysctl
			t.Cleanup(func() { writeFile, mplsProbeWritableSysctl = originalWrite, originalRemount })

			labelSpace := "0\n"
			readFile = func(string) ([]byte, error) { return []byte(labelSpace), nil }
			var writtenPath string
			writeFile = func(path string, data []byte, _ fs.FileMode) error {
				if tc.writeErr != nil {
					return tc.writeErr
				}
				writtenPath, labelSpace = path, string(data)
				return nil
			}
			mplsProbeWritableSysctl = func() error { return tc.remountErr }

			result := askMPLSIPMTUInThrowawayNamespace()
			if result.State != tc.state {
				t.Fatalf("got %v (%v), want %v", result.State, result.Reason, tc.state)
			}
			if tc.state == StatePresent {
				if writtenPath != MPLSPlatformLabelsPath() || labelSpace != tc.written {
					t.Errorf("wrote %q to %q, want %q to %q", labelSpace, writtenPath, tc.written, MPLSPlatformLabelsPath())
				}
				return
			}
			if result.Reason == nil {
				t.Error("an unknown verdict carries no reason")
			}
			if len(*sent) != 0 {
				t.Errorf("the probe sent %d requests after a failed step", len(*sent))
			}
		})
	}
}

// VALIDATES: AC-15 (D-7). A remount or a label-space write refused with EACCES
// while a security policy confines the process reads denied, naming the policy,
// the step and Ze's profile; EACCES with no confining label, and EPERM (a
// missing capability), stay unknown.
// Method: the remount, the write and the confining label are faked.
// PREVENTS: Docker's docker-default profile (deny mount, deny writes under
// /proc/sys/net) read as a bare unknown that names no policy and no fix, and a
// missing capability blamed on a policy.
func TestMPLSIPMTUPolicyDenialIsDenied(t *testing.T) {
	const confined = "AppArmor profile docker-default (enforce)"
	for name, tc := range map[string]struct {
		remountErr, writeErr error
		policy               string
		state                State
	}{
		"mount denied under a profile":  {unix.EACCES, nil, confined, StateDenied},
		"write denied under a profile":  {nil, unix.EACCES, confined, StateDenied},
		"mount EACCES while unconfined": {unix.EACCES, nil, "", StateUnknown},
		"mount EPERM under a profile":   {unix.EPERM, nil, confined, StateUnknown},
	} {
		t.Run(name, func(t *testing.T) {
			withMPLSIPMTUProbe(t, "0\n", nil, unix.ENOENT, unix.ENOENT)
			originalWrite, originalRemount, originalPolicy := writeFile, mplsProbeWritableSysctl, confiningPolicy
			t.Cleanup(func() {
				writeFile, mplsProbeWritableSysctl, confiningPolicy = originalWrite, originalRemount, originalPolicy
			})
			writeFile = func(string, []byte, fs.FileMode) error { return tc.writeErr }
			mplsProbeWritableSysctl = func() error { return tc.remountErr }
			confiningPolicy = func() string { return tc.policy }

			result := askMPLSIPMTUInThrowawayNamespace()
			if result.State != tc.state {
				t.Fatalf("got %v (%v), want %v", result.State, result.Reason, tc.state)
			}
			if result.Reason == nil {
				t.Fatal("the verdict carries no reason")
			}
			if tc.state != StateDenied {
				return
			}
			for _, want := range []string{confined, ProbeAppArmorProfileName, "permission denied"} {
				if !strings.Contains(result.Reason.Error(), want) {
					t.Errorf("the reason does not name %q: %v", want, result.Reason)
				}
			}
			if !errors.Is(result.Reason, unix.EACCES) {
				t.Errorf("the reason does not wrap EACCES: %v", result.Reason)
			}
		})
	}
}
