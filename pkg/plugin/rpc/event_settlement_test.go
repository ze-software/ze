// Design: docs/architecture/api/process-protocol.md — applied event settlement.

package rpc

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestEventSettlementInputRequiresFutureBound pairs a valid future request with
// absent, null, malformed and expired deadline inputs at the actual decoder.
func TestEventSettlementInputRequiresFutureBound(t *testing.T) {
	now := time.UnixMilli(100)
	for _, test := range []struct {
		body  string
		valid bool
	}{
		{`{"deadline-ms":101}`, true},
		{`{"deadline-ms":9223372036854775807}`, true},
		{`{}`, false}, {`null`, false}, {`{"deadline-ms":null}`, false},
		{`{"deadline-ms":"101"}`, false}, {`{"deadline-ms":0}`, false},
		{`{"deadline-ms":-1}`, false}, {`{"deadline-ms":100}`, false},
	} {
		t.Run(test.body, func(t *testing.T) {
			var input EventSettlementInput
			err := json.Unmarshal([]byte(test.body), &input)
			if err == nil {
				err = input.Validate(now)
			}
			if (err == nil) != test.valid {
				t.Fatalf("valid=%t, error=%v", test.valid, err)
			}
		})
	}
}

// TestEventSettlementReceiptRequiresExactOutcome proves neither an empty
// successful result nor an unrelated request bound can acknowledge settlement.
func TestEventSettlementReceiptRequiresExactOutcome(t *testing.T) {
	input := EventSettlementInput{DeadlineMS: 101}
	for _, test := range []struct {
		body  string
		valid bool
	}{
		{`{"deadline-ms":101,"status":"ok"}`, true},
		{`{"deadline-ms":101,"status":"error","error":"writer refused"}`, true},
		{`{}`, false}, {`null`, false}, {`{"deadline-ms":null,"status":"ok"}`, false},
		{`{"deadline-ms":101}`, false}, {`{"deadline-ms":101,"status":null}`, false},
		{`{"deadline-ms":102,"status":"ok"}`, false},
		{`{"deadline-ms":101,"status":"queued"}`, false},
		{`{"deadline-ms":101,"status":"ok","error":"failed"}`, false},
		{`{"deadline-ms":101,"status":"error"}`, false},
		{`{"deadline-ms":101,"status":"error","error":"` + strings.Repeat("x", EventSettlementErrorBytesMax+1) + `"}`, false},
	} {
		var output EventSettlementOutput
		err := json.Unmarshal([]byte(test.body), &output)
		if err == nil {
			err = output.Validate(input)
		}
		if (err == nil) != test.valid {
			t.Fatalf("body=%q valid=%t, error=%v", test.body, test.valid, err)
		}
	}
}

// TestEventSettlementInputRejectsNonpositiveBeforeUnixEpoch checks the native
// YANG range independently of whether a test clock already passed the bound.
func TestEventSettlementInputRejectsNonpositiveBeforeUnixEpoch(t *testing.T) {
	for _, deadline := range []int64{-1, 0} {
		if err := (EventSettlementInput{DeadlineMS: deadline}).Validate(time.UnixMilli(-100)); err == nil {
			t.Fatalf("nonpositive deadline accepted: %d", deadline)
		}
	}
}

// FuzzEventSettlementInput exercises the external decoder and its future bound.
func FuzzEventSettlementInput(f *testing.F) {
	f.Add([]byte(`{"deadline-ms":101}`))
	f.Add([]byte(`{"deadline-ms":null}`))
	f.Fuzz(func(t *testing.T, body []byte) {
		var input EventSettlementInput
		if err := json.Unmarshal(body, &input); err != nil {
			return
		}
		if err := input.Validate(time.UnixMilli(100)); err != nil {
			return
		}
		if input.DeadlineMS <= 100 {
			t.Fatalf("expired request accepted: %+v", input)
		}
	})
}

// FuzzEventSettlementOutput exercises receipt decoding and exact correlation.
func FuzzEventSettlementOutput(f *testing.F) {
	f.Add([]byte(`{"deadline-ms":101,"status":"ok"}`))
	f.Add([]byte(`{"deadline-ms":101,"status":"error","error":"failed"}`))
	f.Fuzz(func(t *testing.T, body []byte) {
		var output EventSettlementOutput
		if err := json.Unmarshal(body, &output); err != nil {
			return
		}
		if err := output.Validate(EventSettlementInput{DeadlineMS: 101}); err != nil {
			return
		}
		if output.DeadlineMS != 101 {
			t.Fatalf("uncorrelated receipt accepted: %+v", output)
		}
	})
}
