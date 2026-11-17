// Design: docs/architecture/api/ipc_protocol.md -- absence and optional metadata.
package ipc

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: a completed no-data answer survives the actual value projection and
// reconstruction without inventing a null document or losing its original
// zero counts or warning lines. Method: start with the real
// Answer producer, project twice with valueAnswer between the projections.
func TestCleanupAnswerAbsentDocumentRoundTrip(t *testing.T) {
	for _, test := range []struct {
		name string
		tail rpc.AnswerTail
		want string
	}{
		{name: "done", want: rpc.VerdictDone},
		{name: "warning", tail: rpc.AnswerTail{Warning: true, Warnings: []string{"original warning"}}, want: rpc.VerdictWarning},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := rpc.NewAnswer(rpc.AnswerTail{Type: rpc.AnswerTypeDocument}, test.tail, nil)
			first, err := ExecuteCommandValue(original)
			if err != nil {
				t.Fatal(err)
			}
			second, err := ExecuteCommandValue(valueAnswer(first))
			if err != nil {
				t.Fatal(err)
			}
			for _, out := range []*rpc.ExecuteCommandOutput{first, second} {
				if out.Status != test.want {
					t.Fatalf("status=%s want=%s", out.Status, test.want)
				}
				if len(out.Data) != 0 {
					t.Fatalf("absence became document %s", out.Data)
				}
				encoded, err := json.Marshal(out)
				if err != nil {
					t.Fatal(err)
				}
				var value struct {
					Data     json.RawMessage `json:"data"`
					Metadata *struct {
						Count    uint64   `json:"count"`
						Faults   uint64   `json:"faults"`
						Message  string   `json:"message"`
						Warning  bool     `json:"warning"`
						Warnings []string `json:"warnings"`
						Absent   bool     `json:"document-absent"`
					} `json:"record-answer"`
				}
				if err := json.Unmarshal(encoded, &value); err != nil {
					t.Fatal(err)
				}
				if value.Data != nil {
					t.Fatal("absent payload was published as a document")
				}
				if value.Metadata == nil {
					t.Fatal("completed original terminator was lost")
				}
				if !value.Metadata.Absent {
					t.Fatal("actual absent document lost its provenance")
				}
				if value.Metadata.Count != 0 {
					t.Fatal("absent document acquired a produced row")
				}
				if value.Metadata.Faults != 0 {
					t.Fatal("absent document acquired a refused row")
				}
				if value.Metadata.Message != test.tail.Message {
					t.Fatal("original failure text changed")
				}
				if value.Metadata.Warning != test.tail.Warning {
					t.Fatal("original warning kind changed")
				}
				if !slices.Equal(value.Metadata.Warnings, test.tail.Warnings) {
					t.Fatal("original warning lines changed")
				}
			}
		})
	}
}

// Goal: a no-record failure retains its existing public Data reason and exact
// terminator without manufacturing a row. Method: include a reason that is
// itself JSON, so bytes alone cannot distinguish it from an empty collection.
func TestCleanupAnswerAbsentFailureKeepsPublicReason(t *testing.T) {
	for _, message := range []string{"[]", "original public failure"} {
		t.Run(message, func(t *testing.T) {
			tail := rpc.AnswerTail{Message: message, Warning: true, Warnings: []string{"original warning"}}
			original := rpc.NewAnswer(rpc.AnswerTail{Type: rpc.AnswerTypeDocument}, tail, nil)
			first, err := ExecuteCommandValue(original)
			if err != nil {
				t.Fatal(err)
			}
			if string(first.Data) != tail.Message {
				t.Fatal("existing public failure reason changed")
			}
			answer := valueAnswer(first)
			rows := 0
			for range answer.Records {
				rows++
			}
			if rows != 0 {
				t.Fatal("public failure reason became an answer row")
			}
			if answer.Message() != tail.Message {
				t.Fatal("original failure text changed")
			}
			if !slices.Equal(answer.Warnings(), tail.Warnings) {
				t.Fatal("original warning lines changed")
			}
			if answer.Verdict() != rpc.VerdictError {
				t.Fatal("original failure outcome changed")
			}
			second, err := ExecuteCommandValue(answer)
			if err != nil {
				t.Fatal(err)
			}
			if string(second.Data) != tail.Message {
				t.Fatal("public failure reason changed after reconstruction")
			}
		})
	}
}

// Goal: an empty collection whose failure message is JSON stays a document.
// Method: use the exact bytes of the preceding no-record failure as the real
// collapsed collection too, so only original absence provenance can decide.
func TestCleanupAnswerFailedEmptyCollectionKeepsDocument(t *testing.T) {
	original := rpc.NewAnswer(rpc.AnswerTail{Type: rpc.AnswerTypeMap}, rpc.AnswerTail{Message: "[]"}, nil)
	first, err := ExecuteCommandValue(original)
	if err != nil {
		t.Fatal(err)
	}
	answer := valueAnswer(first)
	document, err := rpc.CollapseAnswer(answer)
	if err != nil {
		t.Fatal(err)
	}
	if string(document) != "[]" {
		t.Fatalf("real empty collection became %s", document)
	}
	if answer.Message() != "[]" {
		t.Fatal("original failure message changed")
	}
	if answer.Verdict() != rpc.VerdictError {
		t.Fatal("failed empty collection outcome changed")
	}
}

// Goal: an empty collection remains a real document, distinct from absence.
// Method: collapse an actual zero-row map Answer through both value projections.
func TestCleanupAnswerEmptyCollectionKeepsDocument(t *testing.T) {
	original := rpc.NewAnswer(rpc.AnswerTail{Type: rpc.AnswerTypeMap, Key: "owners"}, rpc.AnswerTail{}, nil)
	first, err := ExecuteCommandValue(original)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExecuteCommandValue(valueAnswer(first))
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range []*rpc.ExecuteCommandOutput{first, second} {
		if out.Status != rpc.VerdictDone {
			t.Fatalf("empty walk status=%s", out.Status)
		}
		if string(out.Data) != `{"owners":[]}` {
			t.Fatalf("empty collection became %s", out.Data)
		}
	}
}

// Goal: explicit null metadata cannot impersonate absent optional metadata.
// Method: decode the enclosing real DTO and require refusal, including JSON's
// case-insensitive field spelling, while an omitted field remains usable.
func TestCleanupAnswerOuterNullMetadataRefused(t *testing.T) {
	for _, wire := range []string{
		`{"status":"done","data":[],"record-answer":null}`,
		`{"status":"done","data":[],"RECORD-ANSWER":null}`,
		`{"status":"done","data":[],"record-answer":null,"record-answer":{"count":0,"faults":0}}`,
		`{"status":"done","data":[],"record-answer":{"count":0,"faults":0},"record-answer":null}`,
	} {
		var out rpc.ExecuteCommandOutput
		if err := json.Unmarshal([]byte(wire), &out); err == nil {
			t.Fatalf("explicit null metadata accepted: %s", wire)
		}
	}
}

// Goal: ordinary built-value DTOs still allow the optional metadata to be
// absent. Method: read current successful, absent and failed value forms.
func TestCleanupAnswerBuiltValueWithoutMetadata(t *testing.T) {
	for _, test := range []struct {
		wire string
		want string
		data string
	}{
		{wire: `{"status":"done","data":{"owner":"original"}}`, want: rpc.VerdictDone, data: `{"owner":"original"}`},
		{wire: `{"status":"done"}`, want: rpc.VerdictDone},
		{wire: `{"status":"error","data":"original failure"}`, want: rpc.VerdictError},
	} {
		var out rpc.ExecuteCommandOutput
		if err := json.Unmarshal([]byte(test.wire), &out); err != nil {
			t.Fatal(err)
		}
		answer := valueAnswer(&out)
		document, err := rpc.CollapseAnswer(answer)
		if err != nil {
			t.Fatal(err)
		}
		if err := answer.Err(); err != nil {
			t.Fatal(err)
		}
		if answer.Verdict() != test.want {
			t.Fatalf("built value status=%s want=%s", answer.Verdict(), test.want)
		}
		if string(document) != test.data {
			t.Fatalf("built value data=%s want=%s", document, test.data)
		}
		if test.want == rpc.VerdictError {
			if answer.Message() != string(out.Data) {
				t.Fatal("built failure text changed")
			}
		}
	}
}

// Goal: missing data cannot hide nonzero original row counts. Method: supply
// valid metadata with either successful or refused rows and no document.
func TestCleanupAnswerNonemptyMetadataRequiresDocument(t *testing.T) {
	for _, wire := range []string{
		`{"status":"done","record-answer":{"count":1,"faults":0}}`,
		`{"status":"error","record-answer":{"count":0,"faults":1}}`,
	} {
		var out rpc.ExecuteCommandOutput
		if err := json.Unmarshal([]byte(wire), &out); err != nil {
			t.Fatal(err)
		}
		answer := valueAnswer(&out)
		for range answer.Records {
		}
		if answer.Verdict() != rpc.VerdictError {
			t.Fatal("absent document hid nonzero counts")
		}
		if answer.Message() == "" {
			t.Fatal("absent nonempty answer lost its refusal")
		}
	}
}

// Goal: an explicit absence variant cannot claim original successful or
// refused rows. Method: supply each nonzero count with document-absent true
// and require refusal before any result is accepted as completed evaluation.
func TestCleanupAnswerAbsentMetadataRefusesNonzeroCounts(t *testing.T) {
	for _, wire := range []string{
		`{"status":"done","record-answer":{"count":1,"faults":0,"document-absent":true}}`,
		`{"status":"error","record-answer":{"count":0,"faults":1,"document-absent":true}}`,
	} {
		var out rpc.ExecuteCommandOutput
		if err := json.Unmarshal([]byte(wire), &out); err != nil {
			t.Fatal(err)
		}
		answer := valueAnswer(&out)
		for range answer.Records {
		}
		if answer.Verdict() != rpc.VerdictError {
			t.Fatal("absent document retained nonzero counts")
		}
		if answer.Message() == "" {
			t.Fatal("invalid absence lost its refusal")
		}
	}
}

// Goal: the explicit absent variant retains only its original public reason.
// Method: supply a different Data reason with otherwise valid zero metadata.
func TestCleanupAnswerAbsentMetadataRefusesForeignReason(t *testing.T) {
	var out rpc.ExecuteCommandOutput
	wire := `{"status":"error","data":"foreign","record-answer":{"count":0,"faults":0,"message":"original","document-absent":true}}`
	if err := json.Unmarshal([]byte(wire), &out); err != nil {
		t.Fatal(err)
	}
	answer := valueAnswer(&out)
	for range answer.Records {
	}
	if answer.Message() == "original" {
		t.Fatal("foreign data acquired the original failure identity")
	}
	if answer.Verdict() != rpc.VerdictError {
		t.Fatal("inconsistent absent representation became success")
	}
	if answer.Message() == "" {
		t.Fatal("inconsistent absent representation lost its refusal")
	}
}

// Goal: zero counts alone cannot invent document absence on an untrusted
// value. Method: supply no Data and omit the explicit absence provenance.
func TestCleanupAnswerAbsentMetadataRequiresProvenance(t *testing.T) {
	var out rpc.ExecuteCommandOutput
	wire := `{"status":"done","record-answer":{"count":0,"faults":0}}`
	if err := json.Unmarshal([]byte(wire), &out); err != nil {
		t.Fatal(err)
	}
	answer := valueAnswer(&out)
	for range answer.Records {
	}
	if answer.Verdict() != rpc.VerdictError {
		t.Fatal("zero counts invented absent document provenance")
	}
	if answer.Message() == "" {
		t.Fatal("unproven absence lost its refusal")
	}
}
