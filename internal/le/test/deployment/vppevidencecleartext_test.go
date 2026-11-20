package testdeployment

import (
	"encoding/binary"
	"strings"
	"testing"
)

// The readers of the cleartext probe judge text VPP prints. These tests feed them
// that text in the layout VPP v26.06's formatters write (show_errors in
// src/vlib/error.c, the trace node lines traceNodes reads), so a reader that
// confuses two reasons or two nodes goes red here rather than in a real run.

const cleartextShowErrors = `   Count                  Node                              Reason               Severity
         1            ipsec4-input-feature              IPSec pkts received            error
         1            ipsec4-input-feature           IPSec policy not matched          error
         2            ipsec4-input-feature              IPSec pkts received            error
         7                 ip4-input                    IPSec pkts received            error
         4            ipsec6-input-feature              IPSec policy discard           error
`

func TestVPPErrorCountSumsOneNodesRowsForOneReason(t *testing.T) {
	// Two threads' rows for the same counter are summed; another node's row with the
	// same reason, and a different reason of the same node, are not.
	if got := vppErrorCount(cleartextShowErrors, vppIPsecInputReceived); got != 3 {
		t.Fatalf("received = %d, want 3", got)
	}
	if got := vppErrorCount(cleartextShowErrors, vppIPsecInputNoMatch); got != 1 {
		t.Fatalf("not matched = %d, want 1", got)
	}
	// "IPSec policy match" is a prefix of no row here: "IPSec policy not matched"
	// must not be read as it.
	if got := vppErrorCount(cleartextShowErrors, vppIPsecInputMatch); got != 0 {
		t.Fatalf("match = %d, want 0", got)
	}
	if got := vppErrorCount(cleartextShowErrors, vppIPsecInputDiscard); got != 0 {
		t.Fatalf("discard = %d, want 0: the ipsec6 row is another node", got)
	}
}

func cleartextTrace(nodes ...string) string {
	var b strings.Builder
	b.WriteString("------------------- Start of thread 0 vpp_main -------------------\nPacket 1\n\n")
	for _, node := range nodes {
		b.WriteString("00:00:12:345678: ")
		b.WriteString(node)
		b.WriteString("\n  detail line\n")
	}
	return b.String()
}

func TestTraceFateIsDecidedByTheFirstNodeAfterIPv4Input(t *testing.T) {
	cases := []struct {
		name  string
		nodes []string
		want  vppTraceFate
	}{
		{"forwarded, reply dropped later", []string{"pg-input", "ethernet-input", "ip4-input", "ip4-lookup",
			"ip4-local", "ip4-icmp-input", "ip4-icmp-echo-request", "ip4-load-balance", "error-drop"}, vppTraceFateForwarded},
		{"dropped by ipsec input", []string{"pg-input", "ethernet-input", "ip4-input", "ipsec4-input-feature",
			"error-drop", "drop"}, vppTraceFateDropped},
		{"no-checksum input", []string{"pg-input", "ethernet-input", "ip4-input-no-checksum", "ip4-lookup"},
			vppTraceFateForwarded},
		{"drop before ip4-input decides nothing", []string{"pg-input", "ethernet-input", "error-drop"},
			vppTraceFateUnspecified},
		{"neither after ip4-input", []string{"pg-input", "ethernet-input", "ip4-input"}, vppTraceFateUnspecified},
	}
	for _, tc := range cases {
		if got := traceFate(traceNodes(cleartextTrace(tc.nodes...))); got != tc.want {
			t.Errorf("%s: fate = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestCleartextProbePacketsAreValidIPv4Datagrams(t *testing.T) {
	// Every probe frame must reach ip4-input intact: ip4-input drops a datagram
	// whose header checksum or length is wrong, and that drop would read as VPP's
	// IPsec verdict.
	for _, frame := range vppCleartextProbeFrames {
		var packet [vppCleartextFrameLen]byte
		if err := cleartextProbePacket(packet[:], frame); err != nil {
			t.Fatalf("%s: %v", frame.check, err)
		}
		if got := binary.BigEndian.Uint16(packet[12:14]); got != 0x0800 {
			t.Fatalf("%s: ethertype %#x, want 0x0800", frame.check, got)
		}
		ip := packet[14:34]
		if got := ipv4HeaderChecksum(ip); got != 0 {
			t.Fatalf("%s: IPv4 header does not verify, residual %#x", frame.check, got)
		}
		length := int(binary.BigEndian.Uint16(ip[2:4]))
		if frame.esp {
			if ip[9] != 50 {
				t.Fatalf("%s: protocol %d, want ESP 50", frame.check, ip[9])
			}
			if length != 44 {
				t.Fatalf("%s: length %d, want 44", frame.check, length)
			}
			if got := binary.BigEndian.Uint32(packet[34:38]); got != vppCleartextESPSPI {
				t.Fatalf("%s: SPI %#x, want %#x", frame.check, got, vppCleartextESPSPI)
			}
			continue
		}
		if ip[9] != 1 {
			t.Fatalf("%s: protocol %d, want ICMP 1", frame.check, ip[9])
		}
		if length != 36 {
			t.Fatalf("%s: length %d, want 36", frame.check, length)
		}
		if packet[34] != 8 {
			t.Fatalf("%s: ICMP type %d, want echo request 8", frame.check, packet[34])
		}
		if got := ipv4HeaderChecksum(packet[34 : 34+16]); got != 0 {
			t.Fatalf("%s: ICMP checksum does not verify, residual %#x", frame.check, got)
		}
	}
}

func TestJudgeCleartextFrameFailsOnlyWhenTheMeasurementDidNotHappen(t *testing.T) {
	frame := vppCleartextProbeFrames[0]
	forwarded := cleartextTrace("pg-input", "ethernet-input", "ip4-input", "ip4-lookup", "ip4-local")
	received := "         1            ipsec4-input-feature              IPSec pkts received            error\n" +
		"         1            ipsec4-input-feature           IPSec policy not matched          error\n"

	if got := judgeCleartextFrame(frame, false, "", forwarded, received); got.Verdict != VPPProofFail {
		t.Fatalf("a frame the generator never sent was judged %v", got.Verdict)
	}
	if got := judgeCleartextFrame(frame, true, "", forwarded, ""); got.Verdict != VPPProofFail {
		t.Fatalf("a frame ipsec4-input-feature never counted was judged %v", got.Verdict)
	}
	if got := judgeCleartextFrame(frame, true, "", cleartextTrace("pg-input"), received); got.Verdict != VPPProofFail {
		t.Fatalf("a frame with no readable fate was judged %v", got.Verdict)
	}

	got := judgeCleartextFrame(frame, true, "", forwarded, received)
	if got.Verdict != VPPProofPass {
		t.Fatalf("a measured frame was judged %v: %s", got.Verdict, got.Detail)
	}
	for _, want := range []string{"real VPP forwarded cleartext ICMP echo", "'IPSec policy not matched' 1", frame.owed} {
		if !strings.Contains(got.Detail, want) {
			t.Fatalf("detail %q lacks %q", got.Detail, want)
		}
	}
	dropped := judgeCleartextFrame(frame, true, "",
		cleartextTrace("pg-input", "ethernet-input", "ip4-input", "error-drop"), received)
	if !strings.Contains(dropped.Detail, "real VPP dropped") {
		t.Fatalf("a dropped frame's detail %q does not say so", dropped.Detail)
	}
}

func TestParseCleartextProbeAnswersOnlyAReportedSPDID(t *testing.T) {
	got, err := parseCleartextProbe("=== RUN TestVPPRealCleartextProbe\nze-vpp-cleartext: spd-id=4\n--- PASS")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != "4" {
		t.Fatalf("parse = %q, want 4", got)
	}
	for _, output := range []string{
		"--- SKIP: TestVPPRealCleartextProbe\nze-vpp-cleartext: spd-id=4",
		"ze-vpp-cleartext: spd-id=four",
		"ze-vpp-ipsec: spd-id=4",
		"",
	} {
		if got, err := parseCleartextProbe(output); err == nil {
			t.Fatalf("parse(%q) = %q, want an error", output, got)
		}
	}
}

func TestCleartextSPDMissingNamesWhatVPPLacks(t *testing.T) {
	all := "spd 4\n  ip4-outbound:\n  [3] priority -2000 action protect type ip4-inbound-protect protocol any sa 1\n" +
		"  [4] priority -100 action discard type ip4-inbound-discard protocol any\n" +
		"  [5] priority -100 action bypass type ip4-inbound-bypass protocol any\nSPD Bindings:\n 4 -> pg1\n"
	if got := cleartextSPDMissing(all, "4"); got != "" {
		t.Fatalf("a complete SPD was refused: %s", got)
	}
	if got := cleartextSPDMissing(all, "5"); got == "" {
		t.Fatal("an SPD bound elsewhere was accepted")
	}
	if got := cleartextSPDMissing(strings.Replace(all, "type ip4-inbound-discard", "", 1), "4"); got == "" {
		t.Fatal("an SPD without its DISCARD entry was accepted")
	}
}
