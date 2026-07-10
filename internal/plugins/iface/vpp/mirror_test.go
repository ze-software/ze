package ifacevpp

import (
	"strings"
	"testing"

	"go.fd.io/govpp/binapi/span"
)

// newMirrorBackend returns a backend wired to a programmable channel with two
// pre-registered interfaces (a source and a destination) so SetupMirror can
// resolve both names to SwIfIndex without a live VPP.
func newMirrorBackend(ch *progChannel) *vppBackendImpl {
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {}) // short-circuit ensureChannel populate
	b.names.Add("xe0", 4, "xe0")
	b.names.Add("xe1", 9, "xe1")
	return b
}

// TestSetupMirrorSpanIngressEgress verifies AC-4: mirror with both ingress and
// egress issues a sw_interface_span_enable_disable with state RX_TX, the
// resolved from/to indices, and device-level SPAN (is_l2=false, netlink parity).
// VALIDATES: AC-4 -- SPAN programmed with the RX_TX state per A-6.
// PREVENTS: regression to the errNotSupported stub.
func TestSetupMirrorSpanIngressEgress(t *testing.T) {
	ch := &progChannel{}
	b := newMirrorBackend(ch)

	if err := b.SetupMirror("xe0", "xe1", true, true); err != nil {
		t.Fatalf("SetupMirror: %v", err)
	}
	req, ok := ch.requests[0].(*span.SwInterfaceSpanEnableDisable)
	if !ok {
		t.Fatalf("request type: got %T, want *span.SwInterfaceSpanEnableDisable", ch.requests[0])
	}
	if req.SwIfIndexFrom != 4 {
		t.Errorf("SwIfIndexFrom: got %d, want 4", req.SwIfIndexFrom)
	}
	if req.SwIfIndexTo != 9 {
		t.Errorf("SwIfIndexTo: got %d, want 9", req.SwIfIndexTo)
	}
	if req.State != span.SPAN_STATE_API_RX_TX {
		t.Errorf("State: got %v, want RX_TX", req.State)
	}
	if req.IsL2 {
		t.Error("IsL2: got true, want false (device SPAN, netlink parity per A-6)")
	}
}

// TestSetupMirrorSpanStateMapping verifies the ingress/egress -> SpanState map
// covers each direction. VALIDATES: AC-4 -- rx/tx flag mapping per A-6.
func TestSetupMirrorSpanStateMapping(t *testing.T) {
	cases := []struct {
		name            string
		ingress, egress bool
		want            span.SpanState
	}{
		{"ingress-only", true, false, span.SPAN_STATE_API_RX},
		{"egress-only", false, true, span.SPAN_STATE_API_TX},
		{"both", true, true, span.SPAN_STATE_API_RX_TX},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch := &progChannel{}
			b := newMirrorBackend(ch)
			if err := b.SetupMirror("xe0", "xe1", tc.ingress, tc.egress); err != nil {
				t.Fatalf("SetupMirror: %v", err)
			}
			req, ok := ch.requests[0].(*span.SwInterfaceSpanEnableDisable)
			if !ok {
				t.Fatalf("request type: got %T", ch.requests[0])
			}
			if req.State != tc.want {
				t.Errorf("State: got %v, want %v", req.State, tc.want)
			}
		})
	}
}

// TestSetupMirrorRejectsNoDirection verifies at-least-one-of ingress/egress,
// matching the netlink backend's errIfaceMirrorAtLeastOneOf.
// VALIDATES: AC-4 -- no silent no-op mirror.
func TestSetupMirrorRejectsNoDirection(t *testing.T) {
	ch := &progChannel{}
	b := newMirrorBackend(ch)
	if err := b.SetupMirror("xe0", "xe1", false, false); err == nil {
		t.Fatal("expected error when neither ingress nor egress set, got nil")
	}
	if len(ch.requests) != 0 {
		t.Errorf("no VPP request expected on rejection, got %d", len(ch.requests))
	}
}

// TestSetupMirrorUnknownInterface verifies an unresolved source or destination
// name is rejected (no partial SPAN). VALIDATES: AC-4 boundary.
func TestSetupMirrorUnknownInterface(t *testing.T) {
	ch := &progChannel{}
	b := newMirrorBackend(ch)
	if err := b.SetupMirror("nope", "xe1", true, false); err == nil {
		t.Fatal("expected error for unknown source, got nil")
	}
	if err := b.SetupMirror("xe0", "nope", true, false); err == nil {
		t.Fatal("expected error for unknown destination, got nil")
	}
}

// TestRemoveMirrorSpan verifies RemoveMirror disables every SPAN destination
// recorded for a source, replaying the (from,to,is_l2) triple with state
// DISABLED that VPP requires to delete the entry.
// VALIDATES: AC-4 -- RemoveMirror disables SPAN.
// PREVENTS: a stale SPAN entry after the mirror config is removed.
func TestRemoveMirrorSpan(t *testing.T) {
	ch := &progChannel{}
	b := newMirrorBackend(ch)

	if err := b.SetupMirror("xe0", "xe1", true, true); err != nil {
		t.Fatalf("SetupMirror: %v", err)
	}
	if err := b.RemoveMirror("xe0"); err != nil {
		t.Fatalf("RemoveMirror: %v", err)
	}
	last, ok := ch.requests[len(ch.requests)-1].(*span.SwInterfaceSpanEnableDisable)
	if !ok {
		t.Fatalf("disable request type: got %T", ch.requests[len(ch.requests)-1])
	}
	if last.State != span.SPAN_STATE_API_DISABLED {
		t.Errorf("disable State: got %v, want DISABLED", last.State)
	}
	if last.SwIfIndexFrom != 4 || last.SwIfIndexTo != 9 {
		t.Errorf("disable from/to: got %d/%d, want 4/9", last.SwIfIndexFrom, last.SwIfIndexTo)
	}
}

// TestRemoveMirrorNoRecordIsNoop verifies RemoveMirror is idempotent when no
// SPAN was recorded (mirrors netlink's isNotFound tolerance).
func TestRemoveMirrorNoRecordIsNoop(t *testing.T) {
	ch := &progChannel{}
	b := newMirrorBackend(ch)
	if err := b.RemoveMirror("xe0"); err != nil {
		t.Fatalf("RemoveMirror with no record: %v", err)
	}
	if len(ch.requests) != 0 {
		t.Errorf("no VPP request expected, got %d", len(ch.requests))
	}
}

// TestSetupMirrorRetvalError verifies a non-zero VPP retval surfaces as an
// error rather than a silent success.
func TestSetupMirrorRetvalError(t *testing.T) {
	ch := &progChannel{retval: -1}
	b := newMirrorBackend(ch)
	err := b.SetupMirror("xe0", "xe1", true, false)
	if err == nil {
		t.Fatal("expected error on non-zero retval, got nil")
	}
	if !strings.Contains(err.Error(), "retval") {
		t.Errorf("expected 'retval' in error, got: %v", err)
	}
}
