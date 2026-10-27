// Design: docs/architecture/testing/interop.md -- subscriber D-8 interop observations.
// Related: check_coa_disconnect.go -- exact radclient replies and kernel shaping rates.
package radiussubscriber

import "testing"

// TestDynamicReply checks the received packet, never radclient's sent attributes.
func TestDynamicReply(t *testing.T) {
	good := "Sent CoA-Request Id 1\nReceived CoA-NAK Id 1\n\tError-Cause = Invalid-Attribute-Value\n"
	if err := dynamicReply(good, "CoA-NAK", "Invalid-Attribute-Value"); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{
		"Sent CoA-NAK Id 1\nError-Cause = Invalid-Attribute-Value\n",
		"Received CoA-ACK Id 1\nError-Cause = Invalid-Attribute-Value\n",
		"Error-Cause = Invalid-Attribute-Value\nReceived CoA-NAK Id 1\n",
		"Received CoA-NAK Id 1\nError-Cause = Unsupported-Attribute\n",
	} {
		if err := dynamicReply(output, "CoA-NAK", "Invalid-Attribute-Value"); err == nil {
			t.Errorf("accepted wrong reply %q", output)
		}
	}
}

// TestTBFRate refuses absent, ambiguous and malformed kernel observations.
func TestTBFRate(t *testing.T) {
	good := `[{"kind":"tbf","dev":"ppp0","root":true,"options":{"rate":1250000}},{"kind":"clsact","dev":"ppp0"}]`
	if rate, err := tbfRate(good); err != nil || rate != 10_000_000 {
		t.Fatalf("rate %d, error %v", rate, err)
	}
	for _, output := range []string{
		`[]`, `broken`, `[{"kind":"tbf","options":{}}]`,
		`[{"kind":"tbf","dev":"eth0","root":true,"options":{"rate":1250000}}]`,
		`[{"kind":"tbf","dev":"ppp0","root":true,"options":{"rate":1}},{"kind":"tbf","dev":"ppp1","root":true,"options":{"rate":1}}]`,
	} {
		if _, err := tbfRate(output); err == nil {
			t.Errorf("accepted unmeasured rate %q", output)
		}
	}
}

// TestPolicerRate rejects absent and ambiguous upload state, rather than
// interpreting either as an unchanged rate after a refused request.
func TestPolicerRate(t *testing.T) {
	good := "action order 1: police 0x1 rate 5Mbit burst 16Kb mtu 2Kb action drop/pipe\n"
	if rate, err := policerRate(good); err != nil || rate != 5_000_000 {
		t.Fatalf("rate %d, error %v", rate, err)
	}
	for _, output := range []string{"", "police", "police 0x1 burst 16Kb", "police 0x1 rate 0Mbit", "police 0x1 rate badMbit", "police 0x1 rate 5Kbit", good + good} {
		if _, err := policerRate(output); err == nil {
			t.Errorf("accepted unmeasured police rate %q", output)
		}
	}
}
