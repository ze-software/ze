// Design: docs/architecture/testing/interop.md -- exact AIGP recipient evidence.
package bgp

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	aigpTestOpen     = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001D0104FDE9005AAC1F160200"
	aigpTestEOR      = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000"
	aigpTestDirect   = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003D02000000224001010040020602010000FDEC400304AC1F1602801A0B01000B000000000000006B180A0A02"
	aigpTestWithdraw = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020004180A0A030000"
	aigpTestFence    = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1F1602180A0A04"
	aigpTestRecovery = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003D02000000224001010040020602010000FDEC400304AC1F1602801A0B01000B000000000000006F180A0A03"
)

func aigpTestCapture(t *testing.T, frames ...string) string {
	t.Helper()
	var capture strings.Builder
	encoder := json.NewEncoder(&capture)
	for _, frame := range frames {
		if err := encoder.Encode(extendedRelayFrame{Original: frame}); err != nil {
			t.Fatal(err)
		}
	}
	return capture.String()
}

// TestAIGPWireHistory requires actual synchronized wire receipt, not a table
// row, a substring metric or replay through a replacement recipient session.
func TestAIGPWireHistory(t *testing.T) {
	initial := aigpTestCapture(t, aigpTestOpen, aigpTestEOR, aigpTestDirect, aigpTestWithdraw, aigpTestFence)
	recovered := initial + aigpTestCapture(t, aigpTestRecovery)
	metric107 := strings.Replace(aigpTestRecovery, "006F180A0A03", "006B180A0A03", 1)
	full := recovered + aigpTestCapture(t, metric107, aigpTestWithdraw, metric107)
	for _, positive := range []struct {
		capture     string
		transitions int
	}{{initial, 1}, {recovered, 2}, {full, 5}} {
		if err := requireAIGPWire(positive.capture, "172.31.22.2", positive.transitions); err != nil {
			t.Fatal(err)
		}
	}
	for name, capture := range map[string]string{
		"empty":                  "",
		"destination-cost":       strings.Replace(recovered, "006B180A0A02", "008F180A0A02", 1),
		"unchanged-metric":       strings.Replace(recovered, "006B180A0A02", "0064180A0A02", 1),
		"missing-direct":         strings.Replace(recovered, aigpTestDirect, aigpTestFence, 1),
		"wrong-next-hop":         strings.ReplaceAll(recovered, "AC1F1602", "AC1F1609"),
		"wrong-recovery-cost":    strings.Replace(recovered, "006F180A0A03", "006B180A0A03", 1),
		"double-accumulated":     strings.Replace(recovered, "006F180A0A03", "0076180A0A03", 1),
		"different-withdrawal":   strings.Replace(recovered, "001B020004180A0A030000", "001B020004180A0A020000", 1),
		"missing-withdrawal":     strings.Replace(recovered, aigpTestWithdraw, aigpTestEOR, 1),
		"no-eor":                 strings.Replace(recovered, aigpTestEOR, aigpTestOpen, 1),
		"no-fifo-sentinel":       strings.Replace(recovered, aigpTestFence, aigpTestEOR, 1),
		"reconnected":            recovered + aigpTestCapture(t, aigpTestOpen),
		"notification":           recovered + aigpTestCapture(t, "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0015030602"),
		"direct-later-withdrawn": recovered + aigpTestCapture(t, strings.Replace(aigpTestWithdraw, "180A0A03", "180A0A02", 1)),
		"malformed-header":       strings.Replace(recovered, "003D02", "003C02", 1),
		"duplicate-recovery":     recovered + aigpTestCapture(t, aigpTestRecovery),
		"partial-capture":        recovered + `{"original":"FF`,
		"packed-subject":         strings.Replace(recovered, aigpTestDirect, strings.Replace(aigpTestDirect, "003D02", "004102", 1)+"180A0A05", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if requireAIGPWire(capture, "172.31.22.2", 2) == nil {
				t.Fatal("accepted invalid received-wire history")
			}
		})
	}
}

// TestAIGPWireReadiness refuses absence of the actual initial EOR, even when
// the peer already reports Established.
func TestAIGPWireReadiness(t *testing.T) {
	const ready = `{"peers":{"172.31.22.10":{"state":"established","eor-sent":1,"connections-established":1,"connections-dropped":0}}}`
	if !aigpPeerSynchronized(ready, "172.31.22.10") {
		t.Fatal("valid initial-sync receipt rejected")
	}
	for _, output := range []string{"", "{}", "null",
		strings.Replace(ready, `"eor-sent":1`, `"eor-sent":0`, 1),
		strings.Replace(ready, `"state":"established"`, `"state":"idle"`, 1),
		strings.Replace(ready, `"connections-dropped":0`, `"unrelated":0`, 1)} {
		if aigpPeerSynchronized(output, "172.31.22.10") {
			t.Fatal("accepted incomplete initial-sync receipt")
		}
	}
}
