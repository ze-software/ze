// Design: docs/architecture/api/process-protocol.md -- identical value outcomes.
package ipc

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: the existing bridge value reconstructs the original record walk's
// outcome even though a short walk is one document. Method: use exact producer
// metadata, then the actual valueAnswer -> ExecuteCommandValue round trip.
func TestCleanupAnswerBridgeValueRetainsOriginalCounts(t *testing.T) {
	for _, test := range []struct {
		name          string
		count, faults uint64
		warning       bool
		message       string
		want          string
	}{
		{name: "empty", want: rpc.VerdictDone},
		{name: "one", count: 1, want: rpc.VerdictDone},
		{name: "many", count: 257, want: rpc.VerdictDone},
		{name: "partial", count: 257, faults: 2, want: rpc.VerdictPartial},
		{name: "all-refused", faults: 257, want: rpc.VerdictError},
		{name: "warning", count: 1, warning: true, want: rpc.VerdictWarning},
		{name: "failed-tail", count: 1, message: "original command stopped", want: rpc.VerdictAborted},
	} {
		t.Run(test.name, func(t *testing.T) {
			warnings := []string(nil)
			if test.warning {
				warnings = []string{"original warning"}
			}
			metadata := map[string]any{"count": test.count, "faults": test.faults, "message": test.message, "warning": test.warning}
			if len(warnings) > 0 {
				metadata["warnings"] = warnings
			}
			wire, err := json.Marshal(map[string]any{"status": test.want, "data": json.RawMessage(`{"owners":[],"errors":[{"cause":"retained original"}]}`), "record-answer": metadata})
			if err != nil {
				t.Fatal(err)
			}
			var original rpc.ExecuteCommandOutput
			if err := json.Unmarshal(wire, &original); err != nil {
				t.Fatal(err)
			}
			answer := valueAnswer(&original)
			out, err := ExecuteCommandValue(answer)
			if err != nil {
				t.Fatal(err)
			}
			if out.Status != test.want {
				t.Fatalf("status=%s want %s", out.Status, test.want)
			}
			if !slices.Equal(out.Data, original.Data) {
				t.Fatal("buffered execute lost its original complete data")
			}
			encoded, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			var document struct {
				RecordAnswer *struct {
					Count    uint64   `json:"count"`
					Faults   uint64   `json:"faults"`
					Message  string   `json:"message"`
					Warnings []string `json:"warnings"`
				} `json:"record-answer"`
			}
			if err := json.Unmarshal(encoded, &document); err != nil {
				t.Fatal(err)
			}
			if document.RecordAnswer == nil {
				t.Fatal("buffered execute discarded original outcome metadata")
			}
			if document.RecordAnswer.Count != test.count {
				t.Fatal("one document was counted as one original row")
			}
			if document.RecordAnswer.Faults != test.faults {
				t.Fatal("refused original rows became successful")
			}
			if document.RecordAnswer.Message != test.message {
				t.Fatal("original command's tail message changed")
			}
			if !slices.Equal(document.RecordAnswer.Warnings, warnings) {
				t.Fatal("warning lines changed")
			}
		})
	}
}

// Goal: foreign value metadata cannot contradict its declared result or wrap
// row counts. Method: each malformed boundary DTO must become a failed Answer.
func TestCleanupAnswerBridgeRefusesInconsistentMetadata(t *testing.T) {
	for _, metadata := range []map[string]any{
		{"count": 1, "faults": 1},
		{"count": ^uint64(0), "faults": 1},
		{"count": 1, "faults": 0, "warnings": []string{"undeclared warning"}},
	} {
		wire, err := json.Marshal(map[string]any{"status": rpc.StatusDone, "data": json.RawMessage(`[]`), "record-answer": metadata})
		if err != nil {
			t.Fatal(err)
		}
		var out rpc.ExecuteCommandOutput
		if err := json.Unmarshal(wire, &out); err != nil {
			t.Fatal(err)
		}
		answer := valueAnswer(&out)
		for range answer.Records {
		}
		if answer.Verdict() != rpc.VerdictError {
			t.Fatal("inconsistent metadata authorized successful value")
		}
		if answer.Message() == "" {
			t.Fatal("inconsistent metadata lost its refusal")
		}
	}
}

// Goal: a raw plugin value cannot omit, null, repeat or extend the original
// outcome metadata. Method: decode boundary JSON before valueAnswer reads it.
func TestCleanupAnswerBridgeMetadataDecoderIsClosed(t *testing.T) {
	for _, metadata := range []string{
		`{}`, `{"count":null,"faults":0}`, `{"count":0,"faults":null}`,
		`{"count":0,"faults":0,"unknown":true}`,
		`{"count":0,"count":1,"faults":0}`,
		`{"count":0,"faults":0,"warning":null}`,
	} {
		var output rpc.ExecuteCommandOutput
		if err := json.Unmarshal([]byte(`{"status":"done","data":[],"record-answer":`+metadata+`}`), &output); err == nil {
			t.Fatalf("open metadata was accepted: %s", metadata)
		}
	}
}
