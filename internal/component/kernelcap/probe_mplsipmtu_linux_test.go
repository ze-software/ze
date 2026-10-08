//go:build linux

// Design: docs/architecture/mpls/mpls-kernel.md -- Path MTU and label overhead
// Related: probe_linux.go -- MPLSIPMTU, the probe these tests drive
//
// The transit MTU probe's classification, driven by errno through the request
// seam rather than by whichever kernel the host runs. The live answer on a stock
// and a patched kernel is TestMPLSIntegration_TransitPathMTUFollowsTheProbe in
// internal/plugins/fib/kernel.

package kernelcap

import (
	"errors"
	"io/fs"
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

			result := MPLSIPMTU()
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

			result := MPLSIPMTU()
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
