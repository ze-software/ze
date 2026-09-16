// Design: docs/architecture/diagnostics/path-mtu.md -- the verdict tests
// Related: verdict.go -- classifyTunnel, runVerdictOf, adviseUnderlay
//
// VALIDATES: each tunnel verdict at its exact boundary (A-5), the order that
// tests "no usable MTU" before "oversized", the run ladder, and every row of
// the underlay advice matrix (AC-12, AC-13, AC-14).
// PREVENTS: a threshold drifting by one octet from the tool operators already
// trust, and a tunnel with no safe value being reported as merely oversized.

package cmd

import (
	"net/netip"
	"strings"
	"testing"
)

// TestVerdictThresholds pins each boundary at its exact value against a
// ceiling of 1446 and a recommended value of 1414 (AES-GCM over IPv4 at
// 1500): now = ceiling, ceiling+1, ceiling-32, ceiling-31, recommended and
// recommended±1, plus down and no-usable-mtu.
func TestVerdictThresholds(t *testing.T) {
	const ceil, rec = 1446, 1414
	sized := func(current uint16) tunnelSizing {
		return tunnelSizing{up: true, current: current, ceiling: ceil, recommended: rec, hasRecommended: true}
	}
	cases := []struct {
		name   string
		sizing tunnelSizing
		want   tunnelVerdict
		octets int
	}{
		{"interface down", tunnelSizing{up: false, current: 1500, ceiling: ceil, recommended: rec, hasRecommended: true}, tunnelVerdictDown, 0},
		{"no usable mtu", tunnelSizing{up: true, current: 1500, ceiling: ceil}, tunnelVerdictNoUsableMTU, 0},
		{"now = ceiling+1 is oversized by 1", sized(ceil + 1), tunnelVerdictOversized, 1},
		{"now = 1500 is oversized by 54", sized(1500), tunnelVerdictOversized, 54},
		{"now = ceiling is tight with 0 spare", sized(ceil), tunnelVerdictTight, 0},
		{"now = ceiling-31 is tight with 31 spare", sized(ceil - 31), tunnelVerdictTight, 31},
		{"now = ceiling-32 = recommended is ok", sized(ceil - 32), tunnelVerdictOK, 0},
		{"now = recommended is ok", sized(rec), tunnelVerdictOK, 0},
		{"now = recommended-1 could gain 1", sized(rec - 1), tunnelVerdictUnderUtilized, 1},
		{"now = 1280 could gain 134", sized(1280), tunnelVerdictUnderUtilized, 134},
	}
	for _, c := range cases {
		got, octets := classifyTunnel(&c.sizing)
		if got != c.want {
			t.Errorf("%s: verdict %s, want %s", c.name, got, c.want)
		}
		if octets != c.octets {
			t.Errorf("%s: octets %d, want %d", c.name, octets, c.octets)
		}
	}
	// recommended+1 sits inside the margin and is tight, never under-utilized:
	// the margin is where "tight" begins.
	if got, _ := classifyTunnel(&tunnelSizing{up: true, current: rec + 1, ceiling: ceil, recommended: rec, hasRecommended: true}); got != tunnelVerdictTight {
		t.Errorf("now = recommended+1: verdict %s, want tight", got)
	}
}

// TestVerdictOrderNoUsableMTUBeforeOversized proves a tunnel with no usable
// value is never classified as merely oversized, however far above the
// ceiling it sits, and that no command is owed for it (AC-6).
func TestVerdictOrderNoUsableMTUBeforeOversized(t *testing.T) {
	s := tunnelSizing{up: true, current: 9000, ceiling: 100, hasRecommended: false}
	got, octets := classifyTunnel(&s)
	if got != tunnelVerdictNoUsableMTU {
		t.Fatalf("verdict %s, want no-usable-mtu", got)
	}
	if octets != 0 {
		t.Errorf("octets %d, want 0", octets)
	}
	if got.needsCommand() {
		t.Error("no-usable-mtu must not earn a remediation command")
	}
	for _, v := range []tunnelVerdict{tunnelVerdictOversized, tunnelVerdictTight, tunnelVerdictUnderUtilized} {
		if !v.needsCommand() {
			t.Errorf("%s must earn a remediation command", v)
		}
	}
	for _, v := range []tunnelVerdict{tunnelVerdictDown, tunnelVerdictOK} {
		if v.needsCommand() {
			t.Errorf("%s must not earn a remediation command", v)
		}
	}
}

// TestVerdictZeroValuesNeverRender proves the zero value of each enum this
// file declares is Unspecified and panics when written to the payload, so an
// unset verdict or outcome can never pass for one.
func TestVerdictZeroValuesNeverRender(t *testing.T) {
	expectPanic := func(name string, f func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s rendered its zero value", name)
			}
		}()
		f()
	}
	expectPanic("tunnelVerdict", func() { _ = tunnelVerdictUnspecified.String() })
	expectPanic("underlayOutcome", func() { _ = underlayUnspecified.String() })
	expectPanic("underlayOutcome severity", func() { _ = underlayUnspecified.severity() })
	expectPanic("ipFamily", func() { _ = ipFamilyUnspecified.String() })
	expectPanic("tcpMTUProbing", func() { _ = tcpMTUProbingUnspecified.String() })
	expectPanic("runVerdictOf", func() { _ = runVerdictOf([]tunnelVerdict{tunnelVerdictUnspecified}, false) })
	expectPanic("runStatus", func() { _ = runStatusUnspecified.String() })
	expectPanic("runVerdict", func() { _ = runVerdictUnspecified.String() })
	expectPanic("noteSeverity", func() { _ = noteSeverityUnspecified.String() })
	expectPanic("inventoryState", func() { _ = inventoryUnspecified.String() })
	expectPanic("probeOutcome", func() { _ = probeOutcomeUnspecified.String() })
	expectPanic("underlayOutcome text", func() { _ = underlayUnspecified.text(&underlayInput{}, 0) })
	expectPanic("adviseUnderlay without a measured path", func() { _ = adviseUnderlay(&underlayInput{iface: "eth0", current: 1500}) })
}

// TestRunVerdictLadder pins the run-level ladder, first match wins, and the
// rung where a fault note outside the tunnel table lands.
func TestRunVerdictLadder(t *testing.T) {
	cases := []struct {
		name     string
		verdicts []tunnelVerdict
		fault    bool
		want     runVerdict
	}{
		{"no tunnels", nil, false, runVerdictNoTunnels},
		{"no tunnels but a fault note", nil, true, runVerdictActionNeeded},
		{"all ok", []tunnelVerdict{tunnelVerdictOK, tunnelVerdictOK}, false, runVerdictOK},
		{"only under-utilized is ok", []tunnelVerdict{tunnelVerdictUnderUtilized}, false, runVerdictOK},
		{"one tight is check", []tunnelVerdict{tunnelVerdictOK, tunnelVerdictTight}, false, runVerdictCheck},
		{"one down is check", []tunnelVerdict{tunnelVerdictDown}, false, runVerdictCheck},
		{"one oversized is action-needed", []tunnelVerdict{tunnelVerdictTight, tunnelVerdictOversized}, false, runVerdictActionNeeded},
		{"one no-usable-mtu is action-needed", []tunnelVerdict{tunnelVerdictOK, tunnelVerdictNoUsableMTU}, false, runVerdictActionNeeded},
		{"sized correctly with a fault note is action-needed", []tunnelVerdict{tunnelVerdictOK}, true, runVerdictActionNeeded},
		{"tight outranks the fault note", []tunnelVerdict{tunnelVerdictTight}, true, runVerdictCheck},
	}
	for _, c := range cases {
		if got := runVerdictOf(c.verdicts, c.fault); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

// TestUnderlayAdviceMatrix table-tests every row of the ported tool's advice
// matrix, with the command only where the matrix produces one.
func TestUnderlayAdviceMatrix(t *testing.T) {
	ref := netip.MustParseAddr("1.1.1.1")
	cases := []struct {
		name    string
		in      underlayInput
		outcome underlayOutcome
		command string
	}{
		{"no interface found", underlayInput{current: 1500, tightest: 1400}, underlayUnreadable, ""},
		{"interface mtu unreadable", underlayInput{iface: "eth0", tightest: 1400}, underlayUnreadable, ""},
		{"capped by the interface below 1500", underlayInput{iface: "eth0", current: 1400, tightest: 1400}, underlayCappedByInterface, ""},
		{"interface at 1500 and the path carries it", underlayInput{iface: "eth0", current: 1500, tightest: 1500}, underlayAtStandard, ""},
		{"peers lower, no reference: undecidable (AC-14)", underlayInput{iface: "eth0", current: 1500, tightest: 1400}, underlayUndecidable, ""},
		{"reference reaches the full mtu: not clamped (AC-12)", underlayInput{iface: "eth0", current: 1500, tightest: 1400, reference: 1500, hasReference: true, referenceHost: ref}, underlayNotClamped, ""},
		{"reference above the interface counts as full", underlayInput{iface: "eth0", current: 1500, tightest: 1400, reference: 1600, hasReference: true, referenceHost: ref}, underlayNotClamped, ""},
		{"reference equals the peers: circuit clamped (AC-13)", underlayInput{iface: "eth0", kind: "ethernet", current: 1500, tightest: 1400, reference: 1400, hasReference: true, referenceHost: ref}, underlayCircuitClamped, "set interface ethernet eth0 mtu 1400"},
		{"reference between: two clamps, the lower wins", underlayInput{iface: "eth0", kind: "ethernet", current: 1500, tightest: 1400, reference: 1450, hasReference: true, referenceHost: ref}, underlayTwoClamps, "set interface ethernet eth0 mtu 1400"},
		{"reference below the peers: two clamps, the reference wins", underlayInput{iface: "eth0", kind: "ethernet", current: 1500, tightest: 1400, reference: 1300, hasReference: true, referenceHost: ref}, underlayTwoClamps, "set interface ethernet eth0 mtu 1300"},
		// The command is spelled with the underlay's own YANG list: a veth (the
		// functional tests' sr0) or a bridge is never written as `ethernet`,
		// and a link the schema does not configure earns the value in the note
		// and no command.
		{"circuit clamped on a veth: the veth list", underlayInput{iface: "sr0", kind: "veth", current: 1500, tightest: 1400, reference: 1400, hasReference: true, referenceHost: ref}, underlayCircuitClamped, "set interface veth sr0 mtu 1400"},
		{"two clamps on a bridge: the bridge list", underlayInput{iface: "br0", kind: "bridge", current: 1500, tightest: 1400, reference: 1450, hasReference: true, referenceHost: ref}, underlayTwoClamps, "set interface bridge br0 mtu 1400"},
		{"circuit clamped on a kind the schema lacks: no command", underlayInput{iface: "ppp0", current: 1500, tightest: 1400, reference: 1400, hasReference: true, referenceHost: ref}, underlayCircuitClamped, ""},
		{"circuit clamped on loopback: no command", underlayInput{iface: "lo", kind: loopbackKind, current: 1500, tightest: 1400, reference: 1400, hasReference: true, referenceHost: ref}, underlayCircuitClamped, ""},
	}
	for _, c := range cases {
		got := adviseUnderlay(&c.in)
		if got.outcome != c.outcome {
			t.Errorf("%s: outcome %s, want %s", c.name, got.outcome, c.outcome)
		}
		if got.command != c.command {
			t.Errorf("%s: command %q, want %q", c.name, got.command, c.command)
		}
		if c.command != "" && got.mtu == 0 {
			t.Errorf("%s: a command was produced with no mtu", c.name)
		}
	}
	// The ported tool's severities: only "not clamped" and "at standard" are
	// information; every other outcome is a caution the operator acts on.
	wantSeverity := map[underlayOutcome]noteSeverity{
		underlayUnreadable:        noteSeverityCaution,
		underlayCappedByInterface: noteSeverityCaution,
		underlayAtStandard:        noteSeverityInfo,
		underlayUndecidable:       noteSeverityCaution,
		underlayNotClamped:        noteSeverityInfo,
		underlayCircuitClamped:    noteSeverityCaution,
		underlayTwoClamps:         noteSeverityCaution,
	}
	for outcome, want := range wantSeverity {
		if got := outcome.severity(); got != want {
			t.Errorf("%s: severity %s, want %s", outcome, got, want)
		}
	}
	if got := tunnelCommand("vti-site-a", 1414); got != "set interface xfrm vti-site-a mtu 1414" {
		t.Errorf("tunnelCommand = %q", got)
	}
	// An outcome that names a value but earns no command says why in its note.
	unknown := underlayInput{iface: "ppp0", current: 1500, tightest: 1400, reference: 1400, hasReference: true, referenceHost: ref}
	if text := underlayCircuitClamped.text(&unknown, 1400); !strings.Contains(text, "no configuration command is listed because ppp0") {
		t.Errorf("circuit-clamped note for a kind the schema lacks does not say why no command is listed: %q", text)
	}
	known := underlayInput{iface: "eth0", kind: "ethernet", current: 1500, tightest: 1400, reference: 1400, hasReference: true, referenceHost: ref}
	if text := underlayCircuitClamped.text(&known, 1400); strings.Contains(text, "no configuration command") {
		t.Errorf("circuit-clamped note for an ethernet underlay disclaims a command it lists: %q", text)
	}
}
