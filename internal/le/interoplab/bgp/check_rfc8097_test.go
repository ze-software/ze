// Design: docs/architecture/testing/interop.md -- unit polarity proofs for RFC 8097 interop oracle and GoBGP RIB parser.
// Related: check_rfc8097.go -- requireGoBGPRPKIOVPaths, applyRFC8097EBGPStripOracle, applyRFC8097EBGPOptInOracle, requireRFC8097SpeakerReport.
package bgp

import (
	"encoding/hex"
	"strings"
	"testing"
)

// VALIDATES: requireGoBGPRPKIOVPaths accepts the exact inbound polarity pair
// (10.97.1.0/24 without 0x43/0x00 and 10.97.2.0/24 with 0x43/0x00 state 0) and
// rejects a leaked default-eBGP 0x43/0x00, a missing opted-in 0x43/0x00, or a
// missing sibling Route Target.
// PREVENTS: vacuous GoBGP RIB inspection passing when inbound boundary enforcement regresses.
func TestRequireGoBGPRPKIOVPathsPolarity(t *testing.T) {
	clean := `10.97.1.0/24 {Origin: i} ExtcommsJSON: {"type":16,"value":[{"type":0,"subtype":2,"value":"65001:7"}]}` + "\n" +
		`10.97.2.0/24 {Origin: i} ExtcommsJSON: {"type":16,"value":[{"type":0,"subtype":2,"value":"65001:7"},{"type":67,"subtype":0,"value":0}]}`
	if err := requireGoBGPRPKIOVPaths(clean); err != nil {
		t.Fatalf("clean inbound polarity rejected: %v", err)
	}

	cases := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "default eBGP leaks 0x43/0x00",
			output: `10.97.1.0/24 ExtcommsJSON: {"type":16,"value":[{"type":0,"subtype":2,"value":"65001:7"},{"type":67,"subtype":0,"value":0}]}` + "\n" +
				`10.97.2.0/24 ExtcommsJSON: {"type":16,"value":[{"type":0,"subtype":2,"value":"65001:7"},{"type":67,"subtype":0,"value":0}]}`,
			want: "leaked RFC 8097",
		},
		{
			name: "opted-in eBGP lost 0x43/0x00",
			output: `10.97.1.0/24 ExtcommsJSON: {"type":16,"value":[{"type":0,"subtype":2,"value":"65001:7"}]}` + "\n" +
				`10.97.2.0/24 ExtcommsJSON: {"type":16,"value":[{"type":0,"subtype":2,"value":"65001:7"}]}`,
			want: "lost RFC 8097",
		},
		{
			name: "missing sibling route target",
			output: `10.97.1.0/24 ExtcommsJSON: {"type":16,"value":[]}` + "\n" +
				`10.97.2.0/24 ExtcommsJSON: {"type":16,"value":[{"type":0,"subtype":2,"value":"65001:7"},{"type":67,"subtype":0,"value":0}]}`,
			want: "lacks sibling route target",
		},
		{
			name: "non-integer RFC 8097 state value",
			output: `10.97.1.0/24 ExtcommsJSON: {"type":16,"value":[{"type":0,"subtype":2,"value":"65001:7"}]}` + "\n" +
				`10.97.2.0/24 ExtcommsJSON: {"type":16,"value":[{"type":0,"subtype":2,"value":"65001:7"},{"type":67,"subtype":0,"value":"valid"}]}`,
			want: "decode GoBGP RFC 8097 state",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := requireGoBGPRPKIOVPaths(tc.output)
			if err == nil {
				t.Fatalf("err = nil, want substring %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}

// VALIDATES: the default eBGP wire oracle rejects any transmitted 0x43/0x00
// (counterfactual red if egress stripping is reverted), the opted-in eBGP wire
// oracle rejects non-zero Reserved octets [2..6], and requireRFC8097SpeakerReport
// enforces all three prefixes and their sibling Route Targets on both peers.
// PREVENTS: a silent egress strip regression or vacuous speaker pass.
func TestRFC8097WireOraclesAndReportPolarity(t *testing.T) {
	// UPDATE bodies (after 19-byte BGP header):
	// 10.97.3.0/24 with target:65000:9 only (stripped)
	strippedBody := mustHex(t, "0000001F4001010040020602010000FDE9400304AC1E0002C010080002FDE800000009180A6103")
	// 10.97.3.0/24 with target:65000:9 + 0x43/0x00:valid (clean Reserved)
	optInBody := mustHex(t, "000000274001010040020602010000FDE9400304AC1E0002C010100002FDE8000000094300000000000000180A6103")
	// 10.97.3.0/24 with target:65000:9 + 0x43/0x00:valid (dirty Reserved 0xAA)
	dirtyReservedBody := mustHex(t, "000000274001010040020602010000FDE9400304AC1E0002C010100002FDE8000000094300AA0000000000180A6103")

	stripPass := &speakerVerdict{plugin: speakerOracleRFC8097EBGPStrip}
	applyRFC8097EBGPStripOracle(decodeSpeakerUpdate(strippedBody), stripPass)
	if len(stripPass.failures) != 0 {
		t.Fatalf("clean stripped UPDATE failed strip oracle: %v", stripPass.failures)
	}

	stripLeak := &speakerVerdict{plugin: speakerOracleRFC8097EBGPStrip}
	applyRFC8097EBGPStripOracle(decodeSpeakerUpdate(optInBody), stripLeak)
	if len(stripLeak.failures) == 0 {
		t.Fatal("leaked 0x43/0x00 passed default eBGP strip oracle")
	}

	optInPass := &speakerVerdict{plugin: speakerOracleRFC8097EBGPOptIn}
	applyRFC8097EBGPOptInOracle(decodeSpeakerUpdate(optInBody), optInPass)
	if len(optInPass.failures) != 0 {
		t.Fatalf("clean opted-in UPDATE failed opt-in oracle: %v", optInPass.failures)
	}

	optInDirty := &speakerVerdict{plugin: speakerOracleRFC8097EBGPOptIn}
	applyRFC8097EBGPOptInOracle(decodeSpeakerUpdate(dirtyReservedBody), optInDirty)
	if len(optInDirty.failures) == 0 {
		t.Fatal("dirty Reserved octet passed opt-in oracle")
	}

	defaultReport := strings.Join([]string{
		"result: PASS",
		"plugin: " + speakerOracleRFC8097EBGPStrip,
		"note: established: yes",
		"note: rfc8097-route: 10.97.1.0/24 rt=65001:7 ov=absent",
		"note: rfc8097-route: 10.97.2.0/24 rt=65001:7 ov=absent",
		"note: rfc8097-route: 10.97.3.0/24 rt=65000:9 ov=absent",
	}, "\n")
	if err := requireRFC8097SpeakerReport(defaultReport, speakerOracleRFC8097EBGPStrip, false); err != nil {
		t.Fatalf("valid default report rejected: %v", err)
	}

	optInReport := strings.Join([]string{
		"result: PASS",
		"plugin: " + speakerOracleRFC8097EBGPOptIn,
		"note: established: yes",
		"note: rfc8097-route: 10.97.1.0/24 rt=65001:7 ov=absent",
		"note: rfc8097-route: 10.97.2.0/24 rt=65001:7 ov=0",
		"note: rfc8097-route: 10.97.3.0/24 rt=65000:9 ov=0",
	}, "\n")
	if err := requireRFC8097SpeakerReport(optInReport, speakerOracleRFC8097EBGPOptIn, true); err != nil {
		t.Fatalf("valid opt-in report rejected: %v", err)
	}

	// Unconditional strip (opt-in peer loses 0x43/0x00 on 10.97.3.0/24) must fail.
	optInMissingOV := strings.Replace(optInReport, "10.97.3.0/24 rt=65000:9 ov=0", "10.97.3.0/24 rt=65000:9 ov=absent", 1)
	if err := requireRFC8097SpeakerReport(optInMissingOV, speakerOracleRFC8097EBGPOptIn, true); err == nil {
		t.Fatal("opt-in report with stripped 0x43/0x00 passed")
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}
	return b
}
