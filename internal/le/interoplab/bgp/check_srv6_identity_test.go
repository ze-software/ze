// Design: docs/architecture/testing/interop.md -- oracle discrimination, not live proof.
// Related: check_srv6_identity.go -- actual daemon and FRR execution stays native.
package bgp

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestSRv6IdentityWireOracle checks both positive populations and the precise
// old-code mistakes. The separate control and FIFO fence cannot be omitted.
func TestSRv6IdentityWireOracle(t *testing.T) {
	retained := srv6IdentityLabel + srv6IdentityServices + srv6IdentityOther
	stripped := srv6IdentityLabel + srv6IdentityOther
	cases := []struct {
		name      string
		changed   bool
		hop       string
		sid       string
		prefixes  []int
		wantError bool
	}{
		{"equal-retains", false, srv6IdentityA, retained, []int{1, 2, 3}, false},
		{"changed-strips-only-services", true, srv6IdentityB, stripped, []int{1, 2, 3}, false},
		{"old-explicit-equal-strips", false, srv6IdentityA, stripped, []int{1, 2, 3}, true},
		{"old-policy-change-retains", true, srv6IdentityB, retained, []int{1, 2, 3}, true},
		{"wrong-next-hop", true, srv6IdentityA, stripped, []int{1, 2, 3}, true},
		{"route-loss", true, srv6IdentityB, stripped, []int{2, 3}, true},
		{"control-loss", true, srv6IdentityB, stripped, []int{1, 3}, true},
		{"fence-loss", true, srv6IdentityB, stripped, []int{1, 2}, true},
		{"fence-before-subject", true, srv6IdentityB, stripped, []int{3, 1, 2}, true},
		{"duplicate-subject", true, srv6IdentityB, stripped, []int{1, 1, 2, 3}, true},
		{"unknown-top-level-loss", true, srv6IdentityB, srv6IdentityLabel, []int{1, 2, 3}, true},
		{"service-reserved-zeroed", false, srv6IdentityA, strings.Replace(retained, "05002da5", "05002d00", 1), []int{1, 2, 3}, true},
		{"dirty-label-output", false, srv6IdentityA, srv6IdentityDirtyLabel + srv6IdentityServices + srv6IdentityOther, []int{1, 2, 3}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			capture := make([]string, 0, len(tc.prefixes)+2)
			capture = append(capture, srv6IdentityOracleFrame(t, 1, nil), srv6IdentityOracleFrame(t, 4, nil))
			for _, number := range tc.prefixes {
				sid := ""
				if number == 1 {
					sid = tc.sid
				}
				body := srv6IdentityOracleBody(t, number, tc.hop, sid)
				capture = append(capture, srv6IdentityOracleFrame(t, 2, body))
			}
			err := requireSRv6IdentityWire(strings.Join(capture, ""), tc.changed)
			if (err != nil) != tc.wantError {
				t.Fatalf("oracle error=%v, wantError=%t", err, tc.wantError)
			}
		})
	}
	// A selective mismatch is evidence only after the complete capture passes
	// its independent population, control and session checks. Old output plus
	// another fault MUST report that fault, not selective-removal semantics.
	for _, mutant := range []struct {
		name    string
		changed bool
		hop     string
		sid     string
	}{
		{"old-equal", false, srv6IdentityA, stripped},
		{"old-changed", true, srv6IdentityB, retained},
	} {
		open := srv6IdentityOracleFrame(t, 1, nil)
		keepalive := srv6IdentityOracleFrame(t, 4, nil)
		subject := srv6IdentityOracleFrame(t, 2, srv6IdentityOracleBody(t, 1, mutant.hop, mutant.sid))
		control := srv6IdentityOracleFrame(t, 2, srv6IdentityOracleBody(t, 2, mutant.hop, ""))
		fence := srv6IdentityOracleFrame(t, 2, srv6IdentityOracleBody(t, 3, mutant.hop, ""))
		alteredControl := srv6IdentityOracleFrame(t, 2, srv6IdentityOracleBody(t, 2, mutant.hop, stripped))
		withdrawal := srv6IdentityOracleFrame(t, 2, []byte{
			0, 0, 0, 15, 0x80, 15, 12, 0, 2, 1, 64,
			0x20, 1, 0x0d, 0xb8, 0x92, 0x52, 0, 1,
		})
		for _, tc := range []struct {
			name          string
			capture       string
			wantSelective bool
		}{
			{"complete-selective-red", open + keepalive + subject + control + fence, true},
			{"truncated-frame-after-subject", open + keepalive + subject + "{\"original\":\"ff\"}\n", false},
			{"truncated-capture-after-subject", open + keepalive + subject, false},
			{"withdrawal-after-subject", open + keepalive + subject + withdrawal + control + fence, false},
			{"duplicate-after-subject", open + keepalive + subject + subject + control + fence, false},
			{"altered-control-after-subject", open + keepalive + subject + alteredControl + fence, false},
			{"missing-control", open + keepalive + subject + fence, false},
			{"missing-fence", open + keepalive + subject + control, false},
			{"missing-open", keepalive + subject + control + fence, false},
			{"missing-keepalive", open + subject + control + fence, false},
			{"notification-after-fence", open + keepalive + subject + control + fence + srv6IdentityOracleFrame(t, 3, []byte{3, 5}), false},
		} {
			t.Run(mutant.name+"/"+tc.name, func(t *testing.T) {
				err := requireSRv6IdentityWire(tc.capture, mutant.changed)
				if err == nil {
					t.Fatal("faulty capture passed")
				}
				selective := strings.Contains(err.Error(), "RFC9252 selective semantics")
				if selective != tc.wantSelective {
					t.Fatalf("selective diagnostic=%t, want %t: %v", selective, tc.wantSelective, err)
				}
			})
		}
	}
	t.Run("policy-attempt-evidence", func(t *testing.T) {
		const peer = "172.30.0.11"
		calls := make([]srv6IdentityCall, 0, 4)
		for number := 1; number <= 3; number++ {
			sid := ""
			if number == 1 {
				sid = srv6IdentityDirtyLabel + srv6IdentityServices + srv6IdentityOther
			}
			input := srv6IdentityOracleBody(t, number, srv6IdentityA, sid)
			output, err := srv6IdentityPolicyOutput(&sdk.FilterUpdateInput{
				Direction: "export", Filter: "change-hop", Peer: peer, Raw: input,
			})
			if err != nil {
				t.Fatal(err)
			}
			calls = append(calls, srv6IdentityCall{
				Peer: peer, Input: hex.EncodeToString(input), Output: hex.EncodeToString(output),
				Action: sdk.FilterModify,
			})
		}
		calls = append(calls, srv6IdentityCall{
			Peer: peer, Input: "00000006800f03000201", Output: "00000006800f03000201", Action: sdk.FilterAccept,
		})
		for _, tc := range []struct {
			name      string
			state     any
			wantError bool
		}{
			{"complete-attempt-receipt", map[string]any{"attempts": 4, "failure": "", "calls": calls}, false},
			{"success-only-array-hides-attempts", calls, true},
			{"missing-attempt-count", map[string]any{"failure": "", "calls": calls}, true},
			{"excess-attempt-after-four-successes", map[string]any{"attempts": 5, "failure": "excess callback", "calls": calls}, true},
			{"validation-failure-before-four-successes", map[string]any{"attempts": 5, "failure": "unexpected direction", "calls": calls}, true},
			{"failure-with-exact-attempt-count", map[string]any{"attempts": 4, "failure": "validation failed", "calls": calls}, true},
			{"missing-eor", map[string]any{"attempts": 4, "failure": "", "calls": calls[:3]}, true},
			{"three-announcements-only", map[string]any{"attempts": 3, "failure": "", "calls": calls[:3]}, true},
			{"rejected-input-without-failure-text", srv6IdentityPolicyReceipt{Attempts: 4, Calls: calls, Rejected: srv6IdentityRejected{Input: "00", Octets: 1}}, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				output, err := json.Marshal(tc.state)
				if err != nil {
					t.Fatal(err)
				}
				err = requireSRv6IdentityPolicy(string(output), peer)
				if (err != nil) != tc.wantError {
					t.Fatalf("policy receipt error=%v, wantError=%t", err, tc.wantError)
				}
			})
		}
		for _, tc := range []struct {
			name   string
			change func([]srv6IdentityCall)
		}{
			{"duplicate-announcement", func(c []srv6IdentityCall) { c[1] = c[0] }},
			{"out-of-order-announcements", func(c []srv6IdentityCall) { c[1], c[2] = c[2], c[1] }},
			{"early-eor", func(c []srv6IdentityCall) { c[2], c[3] = c[3], c[2] }},
			{"fourth-announcement", func(c []srv6IdentityCall) { c[3] = c[2] }},
			{"wrong-family-eor", func(c []srv6IdentityCall) { c[3].Input = "00000006800f03000101" }},
			{"malformed-eor", func(c []srv6IdentityCall) { c[3].Input = "00000006800f030002" }},
			{"modified-eor", func(c []srv6IdentityCall) { c[3].Output = "00000006800f03000101" }},
			{"eor-modify-action", func(c []srv6IdentityCall) { c[3].Action = sdk.FilterModify }},
			{"eor-wrong-peer", func(c []srv6IdentityCall) { c[3].Peer = "172.30.0.10" }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				mutated := append([]srv6IdentityCall(nil), calls...)
				tc.change(mutated)
				encoded, err := json.Marshal(srv6IdentityPolicyReceipt{Attempts: 4, Calls: mutated})
				if err != nil {
					t.Fatal(err)
				}
				if err := requireSRv6IdentityPolicy(string(encoded), peer); err == nil {
					t.Fatal("invalid callback population passed")
				}
			})
		}
	})
	t.Run("registered-callback-ledger", func(t *testing.T) {
		const peer = "172.30.0.11"
		var ledger srv6IdentityPolicyLedger
		for number := 1; number <= 3; number++ {
			sid := ""
			if number == 1 {
				sid = srv6IdentityDirtyLabel + srv6IdentityServices + srv6IdentityOther
			}
			_, err := ledger.filter(&sdk.FilterUpdateInput{
				Direction: "export", Filter: "change-hop", Peer: peer,
				Raw: srv6IdentityOracleBody(t, number, srv6IdentityA, sid),
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		eor, err := ledger.filter(&sdk.FilterUpdateInput{
			Direction: "export", Filter: "change-hop", Peer: peer,
			Raw: []byte{0, 0, 0, 6, 0x80, 15, 3, 0, 2, 1},
		})
		if err != nil {
			t.Fatal(err)
		}
		if eor.Action != sdk.FilterAccept || len(eor.Raw) != 0 {
			t.Fatal("exact fourth EOR must be accepted without a raw replacement")
		}
		before := ledger.snapshot()
		encoded, err := json.Marshal(before)
		if err != nil {
			t.Fatal(err)
		}
		if err := requireSRv6IdentityPolicy(string(encoded), peer); err != nil {
			t.Fatal(err)
		}
		if _, err := ledger.filter(&sdk.FilterUpdateInput{
			Direction: "export", Filter: "change-hop", Peer: peer,
			Raw: []byte{0, 0, 0, 6, 0x80, 15, 3, 0, 2, 1},
		}); err == nil {
			t.Fatal("duplicate EOR callback succeeded")
		}
		for range 1000 {
			if _, err := ledger.filter(&sdk.FilterUpdateInput{}); err == nil {
				t.Fatal("excess callback succeeded")
			}
		}
		after := ledger.snapshot()
		if after.Attempts != 5 {
			t.Fatalf("saturated attempts=%d, want five-or-more", after.Attempts)
		}
		if after.Failure == "" {
			t.Fatal("excess callback failure disappeared")
		}
		if after.Rejected.Input != "00000006800f03000201" || after.Rejected.Octets != 10 {
			t.Fatal("later attempts erased the rejected duplicate EOR receipt")
		}
		if len(after.Calls) != 4 {
			t.Fatalf("successful receipt storage grew to %d", len(after.Calls))
		}
		if before.Attempts != 4 {
			t.Fatal("old snapshot changed its attempt count")
		}
		before.Calls[0].Input = "snapshot-owned"
		if ledger.snapshot().Calls[0].Input == "snapshot-owned" {
			t.Fatal("snapshot aliases the ledger's receipt slice")
		}
		var failedFirst srv6IdentityPolicyLedger
		rejected := &sdk.FilterUpdateInput{
			Direction: strings.Repeat("x", 1000), Filter: strings.Repeat("f", 1000),
			Peer: strings.Repeat("p", 1000), Raw: []byte(strings.Repeat("r", 5000)),
		}
		if _, err := failedFirst.filter(rejected); err == nil {
			t.Fatal("invalid first callback succeeded")
		}
		failed := failedFirst.snapshot()
		if failed.Attempts != 1 {
			t.Fatal("validation failure was not counted")
		}
		if len(failed.Failure) != 512 {
			t.Fatalf("first failure storage=%d bytes, want bound512", len(failed.Failure))
		}
		if failed.Rejected.Octets != 5000 || failed.Rejected.Input != hex.EncodeToString(rejected.Raw[:4096]) {
			t.Fatal("first rejected input was not retained at its fixed byte bound")
		}
		if len(failed.Rejected.Direction) != 128 || len(failed.Rejected.Filter) != 128 || len(failed.Rejected.Peer) != 256 {
			t.Fatal("rejected callback metadata is unbounded")
		}
		for number := 2; number <= 3; number++ {
			if _, err := failedFirst.filter(&sdk.FilterUpdateInput{
				Direction: "export", Filter: "change-hop", Peer: peer,
				Raw: srv6IdentityOracleBody(t, number, srv6IdentityA, ""),
			}); err != nil {
				t.Fatal(err)
			}
		}
		final := failedFirst.snapshot()
		if final.Failure != failed.Failure || final.Rejected != failed.Rejected {
			t.Fatal("later successes erased the first failure receipt")
		}
		encoded, err = json.Marshal(final)
		if err != nil {
			t.Fatal(err)
		}
		if err := requireSRv6IdentityPolicy(string(encoded), peer); err == nil {
			t.Fatal("failed attempt was hidden by later successes")
		}
	})
	t.Run("registered-callback-eor-contract", func(t *testing.T) {
		for _, tc := range []struct {
			name          string
			announcements int
			raw           []byte
			direction     string
			filter        string
		}{
			{"early-eor", 2, []byte{0, 0, 0, 6, 0x80, 15, 3, 0, 2, 1}, "export", "change-hop"},
			{"wrong-family-eor", 3, []byte{0, 0, 0, 6, 0x80, 15, 3, 0, 1, 1}, "export", "change-hop"},
			{"malformed-eor", 3, []byte{0, 0, 0, 6, 0x80, 15, 3, 0, 2}, "export", "change-hop"},
			{"legacy-empty-update", 3, []byte{0, 0, 0, 0}, "export", "change-hop"},
			{"eor-extra-attribute", 3, []byte{0, 0, 0, 10, 0x80, 15, 3, 0, 2, 1, 0x40, 1, 1, 0}, "export", "change-hop"},
			{"fourth-announcement", 3, srv6IdentityOracleBody(t, 3, srv6IdentityA, ""), "export", "change-hop"},
			{"wrong-direction-eor", 3, []byte{0, 0, 0, 6, 0x80, 15, 3, 0, 2, 1}, "import", "change-hop"},
			{"wrong-filter-eor", 3, []byte{0, 0, 0, 6, 0x80, 15, 3, 0, 2, 1}, "export", "other"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var ledger srv6IdentityPolicyLedger
				for number := 1; number <= tc.announcements; number++ {
					if _, err := ledger.filter(&sdk.FilterUpdateInput{
						Direction: "export", Filter: "change-hop", Peer: "172.30.0.11",
						Raw: srv6IdentityOracleBody(t, number, srv6IdentityA, ""),
					}); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := ledger.filter(&sdk.FilterUpdateInput{
					Direction: tc.direction, Filter: tc.filter, Peer: "172.30.0.11", Raw: tc.raw,
				}); err == nil {
					t.Fatal("invalid terminal callback succeeded")
				}
				state := ledger.snapshot()
				if state.Attempts != uint8(tc.announcements+1) || state.Failure == "" ||
					state.Rejected.Input != hex.EncodeToString(tc.raw) || state.Rejected.Octets != len(tc.raw) {
					t.Fatalf("failed callback receipt lost: %+v", state)
				}
			})
		}
	})
}

func srv6IdentityOracleBody(t *testing.T, number int, nextHop, sid string) []byte {
	t.Helper()
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 10, 2, 2, 0, 0, 0xfd, 0xe9, 0, 0, 0xfd, 0xec}
	attrs = append(attrs, 0x80, 14, 30, 0, 2, 1, 16)
	hop := netip.MustParseAddr(nextHop).As16()
	attrs = append(attrs, hop[:]...)
	attrs = append(attrs, 0, 64, 0x20, 1, 0x0d, 0xb8, 0x92, 0x52, 0, byte(number))
	if sid != "" {
		value, err := hex.DecodeString(sid)
		if err != nil {
			t.Fatal(err)
		}
		attrs = append(attrs, 0xc0, 40, byte(len(value)))
		attrs = append(attrs, value...)
	}
	body := make([]byte, 4, 4+len(attrs))
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	return append(body, attrs...)
}

func srv6IdentityOracleFrame(t *testing.T, kind byte, body []byte) string {
	t.Helper()
	frame := make([]byte, 19, 19+len(body))
	for index := range 16 {
		frame[index] = 0xff
	}
	binary.BigEndian.PutUint16(frame[16:18], uint16(19+len(body)))
	frame[18] = kind
	frame = append(frame, body...)
	row, err := json.Marshal(extendedRelayFrame{Original: hex.EncodeToString(frame)})
	if err != nil {
		t.Fatal(err)
	}
	return string(row) + "\n"
}
