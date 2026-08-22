package rpc

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseLine verifies parsing of the #<id> <verb> [<payload>] wire format.
//
// VALIDATES: ParseLine correctly extracts id, verb, and optional payload.
// PREVENTS: Incorrect parsing of the unified line format.
func TestParseLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		line        string
		wantID      uint64
		wantVerb    string
		wantPayload string
		wantErr     bool
	}{
		{"request with params", `#1 test-method {"key":"value"}`, 1, "test-method", `{"key":"value"}`, false},
		{"request no params", "#42 ping", 42, "ping", "", false},
		{"ok with payload", `#5 ok {"result":"done"}`, 5, "ok", `{"result":"done"}`, false},
		{"ok no payload", "#3 ok", 3, "ok", "", false},
		{"error with payload", `#7 error {"code":"not-found","message":"peer not found"}`, 7, "error", `{"code":"not-found","message":"peer not found"}`, false},
		{"error no payload", "#9 error", 9, "error", "", false},
		{"large id", "#18446744073709551615 method", 18446744073709551615, "method", "", false},
		{"missing hash prefix", "1 method", 0, "", "", true},
		{"no verb", "#1", 0, "", "", true},
		{"invalid id", "#3:abc method", 0, "", "", true},
		{"empty after hash", "#", 0, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id, verb, payload, err := ParseLine([]byte(tt.line))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantVerb, verb)
			if tt.wantPayload == "" {
				assert.Nil(t, payload)
			} else {
				assert.Equal(t, tt.wantPayload, string(payload))
			}
		})
	}
}

// TestFormatRequest verifies request line formatting: #<id> <method> [<json>]
//
// VALIDATES: FormatRequest produces correct wire format for requests.
// PREVENTS: Malformed request lines on the wire.
func TestFormatRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		id     uint64
		method string
		params json.RawMessage
		want   string
	}{
		{"with params", 1, "test-method", json.RawMessage(`{"key":"value"}`), `#1 test-method {"key":"value"}`},
		{"no params", 42, "ping", nil, "#42 ping"},
		{"null params", 5, "ping", json.RawMessage("null"), "#5 ping"},
		{"empty params", 3, "method", json.RawMessage(""), "#3 method"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatRequest(tt.id, tt.method, tt.params)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

// TestFormatOK verifies empty success response formatting: #<id> ok
//
// VALIDATES: FormatOK produces correct wire format.
// PREVENTS: Malformed empty ok responses.
func TestFormatOK(t *testing.T) {
	t.Parallel()

	got := FormatOK(42)
	assert.Equal(t, "#42 ok", string(got))
}

// TestFormatError verifies error response formatting: #<id> error [<json>]
//
// VALIDATES: FormatError produces correct wire format for error responses.
// PREVENTS: Malformed error responses on the wire.
func TestFormatError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      uint64
		payload json.RawMessage
		want    string
	}{
		{"with payload", 1, json.RawMessage(`{"code":"not-found","message":"peer not found"}`), `#1 error {"code":"not-found","message":"peer not found"}`},
		{"empty payload", 2, nil, "#2 error"},
		{"empty bytes", 3, json.RawMessage(""), "#3 error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatError(tt.id, tt.payload)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

// TestNewErrorPayload verifies error payload construction.
//
// VALIDATES: NewErrorPayload produces valid JSON with code and message fields.
// PREVENTS: Unreadable error payloads reaching the CLI display.
func TestNewErrorPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		code        string
		message     string
		wantCode    string
		wantMessage string
	}{
		{"with code", "peer-not-found", "peer not found", "peer-not-found", "peer not found"},
		{"empty code", "", "some error", "", "some error"},
		{"command error", "command-not-available", `command "bgp rib routes" not available`, "command-not-available", `command "bgp rib routes" not available`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := NewErrorPayload(tt.code, tt.message)

			var detail struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(payload, &detail))
			assert.Equal(t, tt.wantCode, detail.Code)
			assert.Equal(t, tt.wantMessage, detail.Message)
		})
	}
}

// TestRPCCallError verifies RPCCallError.Error() message formatting.
//
// VALIDATES: RPCCallError implements error interface with informative messages.
// PREVENTS: Uninformative error messages when RPC calls fail.
func TestRPCCallError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     RPCCallError
		wantMsg string
	}{
		{"message only", RPCCallError{Message: "peer not found"}, "rpc error: peer not found"},
		{"code only", RPCCallError{Code: "not-found"}, "rpc error: not-found"},
		{"both", RPCCallError{Code: "not-found", Message: "peer not found"}, "rpc error: peer not found"},
		{"neither", RPCCallError{}, "rpc error: (no message)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantMsg, tt.err.Error())
		})
	}
}

// TestCodedError verifies CodedError carries code through error chains.
//
// VALIDATES: CodedError implements error interface and preserves code.
// PREVENTS: Loss of error codes when errors pass through dispatch layers.
func TestCodedError(t *testing.T) {
	err := NewCodedError("unknown-command", "command not found")
	assert.Equal(t, "unknown-command", err.Code)
	assert.Equal(t, "command not found", err.Error())
}

// TestExtractErrorMessage verifies human-readable message extraction from error payload JSON.
//
// VALIDATES: ExtractErrorMessage returns message when present, empty string otherwise.
// PREVENTS: Consumers falling through to kebab-case code for display.
func TestExtractErrorMessage(t *testing.T) {
	tests := []struct {
		name   string
		params json.RawMessage
		want   string
	}{
		{"with_message", json.RawMessage(`{"message":"peer not found"}`), "peer not found"},
		{"empty_message", json.RawMessage(`{"message":""}`), ""},
		{"no_message_field", json.RawMessage(`{"code":"err"}`), ""},
		{"nil_params", nil, ""},
		{"empty_params", json.RawMessage(``), ""},
		{"invalid_json", json.RawMessage(`{broken`), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractErrorMessage(tt.params)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseRPCError verifies parseRPCError handles empty, valid JSON,
// non-JSON, and partial field payloads.
//
// VALIDATES: parseRPCError correctly populates RPCCallError from various payloads.
// PREVENTS: Lost error details when remote side sends structured or unstructured errors.
func TestParseRPCError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		payload     string
		wantCode    string
		wantMessage string
	}{
		{
			name:        "empty payload",
			payload:     "",
			wantCode:    "",
			wantMessage: "",
		},
		{
			name:        "valid json",
			payload:     `{"code":"x","message":"y"}`,
			wantCode:    "x",
			wantMessage: "y",
		},
		{
			name:        "non-json",
			payload:     "plain text",
			wantCode:    "",
			wantMessage: "plain text",
		},
		{
			name:        "partial fields message only",
			payload:     `{"message":"only msg"}`,
			wantCode:    "",
			wantMessage: "only msg",
		},
		{
			name:        "partial fields code only",
			payload:     `{"code":"only-code"}`,
			wantCode:    "only-code",
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var input []byte
			if tt.payload != "" {
				input = []byte(tt.payload)
			}
			got := parseRPCError(input)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantCode, got.Code)
			assert.Equal(t, tt.wantMessage, got.Message)
		})
	}
}

// TestParseLineFormatRoundTrip verifies that Format*/ParseLine round-trip correctly.
//
// VALIDATES: Lines formatted with Format* can be parsed back by ParseLine.
// PREVENTS: Formatting/parsing mismatch causing protocol errors.
func TestParseLineFormatRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("request round-trip", func(t *testing.T) {
		t.Parallel()
		params := json.RawMessage(`{"key":"value"}`)
		line := FormatRequest(42, "test-method", params)

		id, verb, payload, err := ParseLine(line)
		require.NoError(t, err)
		assert.Equal(t, uint64(42), id)
		assert.Equal(t, "test-method", verb)
		assert.Equal(t, string(params), string(payload))
	})

	t.Run("ok round-trip", func(t *testing.T) {
		t.Parallel()
		result := json.RawMessage(`{"status":"done"}`)
		line := AppendResult(nil, 7, result)

		id, verb, payload, err := ParseLine(line)
		require.NoError(t, err)
		assert.Equal(t, uint64(7), id)
		assert.Equal(t, "ok", verb)
		assert.Equal(t, string(result), string(payload))
	})

	t.Run("error round-trip", func(t *testing.T) {
		t.Parallel()
		errPayload := NewErrorPayload("not-found", "peer not found")
		line := FormatError(3, errPayload)

		id, verb, payload, err := ParseLine(line)
		require.NoError(t, err)
		assert.Equal(t, uint64(3), id)
		assert.Equal(t, "error", verb)
		assert.JSONEq(t, string(errPayload), string(payload))
	})

	t.Run("ok no payload round-trip", func(t *testing.T) {
		t.Parallel()
		line := FormatOK(99)

		id, verb, payload, err := ParseLine(line)
		require.NoError(t, err)
		assert.Equal(t, uint64(99), id)
		assert.Equal(t, "ok", verb)
		assert.Nil(t, payload)
	})
}

// TestParseLineCarriesKeyValueTailWhole checks that the frame layer hands a
// key=value tail to the answer reader unsplit. The method: one line of each
// answer shape is parsed, and the payload is compared with everything the line
// carries after the verb.
//
// VALIDATES: A-1 of spec-streaming-answer-protocol -- the frame layer
// needs no change to carry a tail, because ParseLine cuts the verb at the first
// space and returns the remainder as one payload.
// PREVENTS: the tail migration reaching the frame reader, which would break
// every plugin at once rather than at the payload boundary.
func TestParseLineCarriesTheAnswerTailWhole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		line        string
		wantVerb    string
		wantPayload string
	}{
		{
			name:        "head",
			line:        "#7 top map 5:peers 0:",
			wantVerb:    AnswerKindHead,
			wantPayload: "map 5:peers 0:",
		},
		{
			name:        "a record holding = and spaces",
			line:        `#7 row {"peer":"10.0.0.1","note":"a=b c"}`,
			wantVerb:    AnswerKindRecord,
			wantPayload: `{"peer":"10.0.0.1","note":"a=b c"}`,
		},
		{
			name:        "terminator",
			line:        "#7 end 97 3 0:",
			wantVerb:    AnswerKindTerminator,
			wantPayload: "97 3 0:",
		},
		{
			name:        "not understood",
			line:        "#7 nay 0: 31:unknown command: shwo bgp peers",
			wantVerb:    AnswerKindNotUnderstood,
			wantPayload: "0: 31:unknown command: shwo bgp peers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id, verb, payload, err := ParseLine([]byte(tt.line))
			require.NoError(t, err)
			assert.Equal(t, uint64(7), id)
			assert.Equal(t, tt.wantVerb, verb)
			assert.Equal(t, tt.wantPayload, string(payload))
		})
	}
}

// TestTailTokenizerNeedsNoJSONDecoder checks that a reader decides how to read
// an answer from the kind and the tail's positional fields alone. The method:
// tails whose payload is not JSON at all are parsed, and the control fields are
// asserted.
//
// VALIDATES: the verdict is the kind and its fields, never a payload field.
// PREVENTS: a consumer parsing a record payload to learn what the line is,
// which is the whole-answer materialization this protocol exists to remove.
func TestTailTokenizerNeedsNoJSONDecoder(t *testing.T) {
	t.Parallel()

	head, err := ParseAnswerTail(AnswerKindHead, []byte("map 5:peers 0:"))
	require.NoError(t, err)
	assert.Equal(t, AnswerTypeMap, head.Type)
	assert.Equal(t, "peers", head.Key)
	assert.Equal(t, AnswerKindHead, head.Kind)

	record, err := ParseAnswerTail(AnswerKindRecord, []byte("20:<<this is not json>>"))
	require.NoError(t, err)
	assert.Equal(t, "<<this is not json>>", string(record.Item))
	assert.Equal(t, AnswerKindRecord, record.Kind)

	terminator, err := ParseAnswerTail(AnswerKindTerminator, []byte("97 3 0:"))
	require.NoError(t, err)
	assert.Equal(t, uint64(97), terminator.Count)
	assert.Equal(t, uint64(3), terminator.Faults)
	assert.Equal(t, AnswerKindTerminator, terminator.Kind)
}

// TestAnswerValuesNeedNoEscaping checks that a value holding an = or a space
// reaches the wire unchanged. The method: a record payload, a rejected row and a
// terminator message each carry an = and a space, are written, parsed back
// through the frame layer and compared byte for byte; then a value holding a
// newline is written and the line count is asserted.
//
// VALIDATES: AC-11 of spec-streaming-answer-protocol -- a value containing = and
// spaces round-trips with no escaping and no quoting, which is what a counted
// field and a trailing payload each buy.
// PREVENTS: an escaping scheme being invented for the tail, and a value
// splitting one answer line into two.
func TestAnswerValuesNeedNoEscaping(t *testing.T) {
	t.Parallel()

	t.Run("a record holding = and spaces", func(t *testing.T) {
		t.Parallel()
		item := json.RawMessage(`{"filter":"community = 65000:1","note":"a b c"}`)

		tail := parseAnswerLine(t, AppendAnswerItem(nil, 7, item))
		assert.Equal(t, string(item), string(tail.Item))
		assert.Empty(t, tail.Fault)
	})

	t.Run("a rejected row holding = and spaces", func(t *testing.T) {
		t.Parallel()
		fault := json.RawMessage(`{"path":"bgp/peer/10.0.0.2","message":"nexthop = unreachable"}`)

		tail := parseAnswerLine(t, AppendAnswerFault(nil, 7, fault))
		assert.Equal(t, string(fault), string(tail.Fault))
		assert.Empty(t, tail.Item)
	})

	t.Run("a message holding = and spaces", func(t *testing.T) {
		t.Parallel()

		tail := parseAnswerLine(t, AppendAnswerTerminator(nil, 7, 417, 0, "rib snapshot expired: age = 4h"))
		assert.Equal(t, uint64(417), tail.Count)
		assert.Equal(t, "rib snapshot expired: age = 4h", tail.Message)
	})

	t.Run("a newline in a value is data, not a boundary", func(t *testing.T) {
		t.Parallel()

		// The message states its own byte count, so the newline inside it is
		// part of the value the count names. It reaches the wire unchanged and
		// reads back unchanged, which is what deleting the rewriting pass over
		// operator data buys.
		const message = "peer rejected\nthe route"

		line := AppendAnswerTerminator(nil, 7, 0, 0, message)
		assert.Contains(t, string(line), message, "the newline was rewritten on the way to the wire")

		tail := parseAnswerLine(t, line)
		assert.Equal(t, message, tail.Message)
	})
}

// TestTerminatorIsTheLineStatingTheEndKind checks that a reader tells the head
// from the terminator by the kind token rather than by a key or by position.
// The method: a head, a record and a terminator are written and parsed, and
// each is asked what it is with no other line in hand.
//
// VALIDATES: the first of the four structural rules -- a reader needs no
// lookahead and no state at all, because each line says what it is.
// PREVENTS: a reader buffering the answer to find out where it ends, and a
// reader deriving the terminator from count= after the key names leave.
func TestTerminatorIsTheLineStatingTheEndKind(t *testing.T) {
	t.Parallel()

	head := parseAnswerLine(t, AppendAnswerHead(nil, 7, AnswerTypeMap, "peers", nil))
	assert.Equal(t, AnswerKindHead, head.Kind)
	assert.Equal(t, AnswerTypeMap, head.Type)
	assert.Equal(t, "peers", head.Key)

	record := parseAnswerLine(t, AppendAnswerItem(nil, 7, json.RawMessage(`{"peer":"10.0.0.1"}`)))
	assert.Equal(t, AnswerKindRecord, record.Kind)

	fault := parseAnswerLine(t, AppendAnswerFault(nil, 7, json.RawMessage(`{"message":"nexthop unreachable"}`)))
	assert.Equal(t, AnswerKindFault, fault.Kind)

	terminator := parseAnswerLine(t, AppendAnswerTerminator(nil, 7, 2, 0, ""))
	assert.Equal(t, AnswerKindTerminator, terminator.Kind)
	assert.Equal(t, uint64(2), terminator.Count)
}

// TestVerdictDerivesFromTheCounts checks every row of the derivation table. The
// method: each terminator shape is written, parsed back, and its derived
// verdict compared with the row.
//
// VALIDATES: AC-12 -- the verdict a consumer computes matches the table.
// PREVENTS: a consumer inventing its own reading of the counts, and a
// truncated answer being read as a short one.
func TestVerdictDerivesFromTheCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		count   uint64
		faults  uint64
		message string
		want    string
	}{
		{name: "records and no fault", count: 2, want: VerdictDone},
		{name: "no records and no fault", count: 0, want: VerdictDone},
		{name: "records and faults", count: 97, faults: 3, want: VerdictPartial},
		{name: "no records and faults", count: 0, faults: 3, want: VerdictError},
		{name: "aborted walk", count: 417, message: "rib snapshot expired", want: VerdictAborted},
		{name: "failed before any record", count: 0, message: "peer 10.0.0.1 not configured", want: VerdictError},
		{name: "aborted with faults", count: 97, faults: 3, message: "rib snapshot expired", want: VerdictAborted},
		{name: "aborted after rejecting every row", count: 0, faults: 3, message: "rib snapshot expired", want: VerdictAborted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			terminator := parseAnswerLine(t, AppendAnswerTerminator(nil, 7, tt.count, tt.faults, tt.message))
			assert.Equal(t, tt.want, Verdict(&terminator))
		})
	}

	t.Run("no terminator", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, VerdictTruncated, Verdict(nil))
	})

	t.Run("last line is not a terminator", func(t *testing.T) {
		t.Parallel()
		head := parseAnswerLine(t, AppendAnswerHead(nil, 7, AnswerTypeMap, "peers", nil))
		assert.Equal(t, VerdictTruncated, Verdict(&head))
	})
}

// TestVerdictTellsFailedFromAbortedFromPartial checks that the terminator ALONE
// separates the three outcomes an answer can end with. The method: a command
// that failed before it produced anything, a walk that stopped part way, and a
// walk that produced rows and rejected some are each written by the terminator
// appender, read back by the parser, and put to Verdict; the three verdicts are
// then required to differ from each other.
//
// VALIDATES: AC-11 and A-5 -- the distinction lives in the counts and the
// message the terminator already carries, so no second line has to state it.
// PREVENTS: the head's status returning as a positional token. Two lines
// stating one outcome can disagree, which is the defect this frame exists to
// remove; this test is what says the terminator is enough on its own.
func TestVerdictTellsFailedFromAbortedFromPartial(t *testing.T) {
	t.Parallel()

	failed := parseAnswerLine(t, AppendAnswerTerminator(nil, 7, 0, 0, "peer 10.0.0.1 not configured"))
	aborted := parseAnswerLine(t, AppendAnswerTerminator(nil, 7, 417, 0, "rib snapshot expired"))
	partial := parseAnswerLine(t, AppendAnswerTerminator(nil, 7, 97, 3, ""))

	assert.Equal(t, VerdictError, Verdict(&failed), "a command that produced nothing and stated a reason is a failure")
	assert.Equal(t, VerdictAborted, Verdict(&aborted), "a walk that produced rows and then stopped is aborted")
	assert.Equal(t, VerdictPartial, Verdict(&partial), "a walk that produced rows and rejected rows is partial")

	verdicts := map[string]string{
		Verdict(&failed):  "failed",
		Verdict(&aborted): "aborted",
		Verdict(&partial): "partial",
	}
	assert.Len(t, verdicts, 3, "two of the three outcomes derive to one verdict, so a consumer cannot tell them apart")

	// The rows an aborted walk did produce are real, and the count is what says
	// so. A consumer reading the terminator alone learns how far it got.
	assert.Equal(t, uint64(0), failed.Count)
	assert.Equal(t, uint64(417), aborted.Count)
	assert.Equal(t, "peer 10.0.0.1 not configured", failed.Message)
	assert.Equal(t, "rib snapshot expired", aborted.Message)
}

// TestHeadStatesNoStatus checks that the head writes the kind, the id, the item
// type, the envelope name and the column names, and nothing else. The method:
// every head the writers can produce is written and searched for the words the
// status vocabulary used to spell, its fields are counted against the grammar,
// and the reader's own view of the line is compared with the writer's.
//
// VALIDATES: AC-15 -- a head states no outcome, so nothing on it can contradict
// the terminator.
// PREVENTS: the status returning as a positional token. The head is written
// AFTER the body on the SSH exec channel, so a status there was never a fact a
// consumer could commit to on the first line.
func TestHeadStatesNoStatus(t *testing.T) {
	t.Parallel()

	heads := []string{
		string(AppendAnswerHead(nil, 7, AnswerTypeDocument, "", nil)),
		string(AppendAnswerHead(nil, 7, AnswerTypeMap, "peers", nil)),
		string(AppendAnswerHead(nil, AnswerNoID, AnswerTypeMap, "peers", nil)),
		string(AppendAnswerHead(nil, 7, AnswerTypeTable, "peers", json.RawMessage(`["peer","state"]`))),
	}
	for _, head := range heads {
		assert.NotContains(t, head, StatusDone, "the head states a status: %q", head)
		assert.NotContains(t, head, StatusError, "the head states a status: %q", head)
		assert.NotContains(t, head, "=", "the head carries a key=value pair: %q", head)
	}

	// The head's fields, in the order the grammar states them: the kind, the
	// item type, the envelope name and the column names. Four, whatever the
	// answer turned out to be.
	for _, head := range heads {
		_, tail, err := ParseAnswerLine([]byte(strings.TrimPrefix(head, "#7 ")))
		require.NoError(t, err, "the head does not read back: %q", head)
		assert.Equal(t, AnswerKindHead, tail.Kind)
		assert.NotEmpty(t, tail.Type, "the head states no item type: %q", head)

		fields := strings.Split(strings.TrimPrefix(head, "#7 "), " ")
		assert.Len(t, fields, 4, "the head has %d fields rather than four: %q", len(fields), head)
	}

	// A head that states an outcome is a line this build cannot read, rather
	// than a line whose extra field is ignored.
	_, err := ParseAnswerTail(AnswerKindHead, []byte("map done 5:peers 0:"))
	require.Error(t, err, "a head stating an outcome must be refused")
}

// TestAnswerTerminatorCountBoundaries checks the numeric edges of the
// terminator's counts. The method: the lowest count, the highest, and one past
// the highest are each written or fed to the reader.
//
// VALIDATES: the count= boundary row of the spec's Boundary Tests table -- 0
// and max uint64 are carried, and an overflow is refused.
// PREVENTS: a count wrapping to a small number, which would report a large
// answer as a short one.
func TestAnswerTerminatorCountBoundaries(t *testing.T) {
	t.Parallel()

	lowest := parseAnswerLine(t, AppendAnswerTerminator(nil, 7, 0, 0, ""))
	assert.Equal(t, uint64(0), lowest.Count)
	assert.Equal(t, AnswerKindTerminator, lowest.Kind)

	highest := parseAnswerLine(t, AppendAnswerTerminator(nil, 7, math.MaxUint64, math.MaxUint64, ""))
	assert.Equal(t, uint64(math.MaxUint64), highest.Count)
	assert.Equal(t, uint64(math.MaxUint64), highest.Faults)

	_, err := ParseAnswerTail(AnswerKindTerminator, []byte("18446744073709551616 0 0:"))
	require.Error(t, err, "a count past max uint64 must be refused")

	_, err = ParseAnswerTail(AnswerKindTerminator, []byte("-1 0 0:"))
	require.Error(t, err, "a negative count must be refused")

	// A message stating more bytes than arrived. It is the last field, so a
	// reader that clamped the length to what it got would accept the line.
	_, err = ParseAnswerTail(AnswerKindTerminator, []byte("0 0 99:short"))
	require.Error(t, err, "a message wider than the line must be refused")

	// The same for the not-understood answer, whose message is also last.
	_, err = ParseAnswerTail(AnswerKindNotUnderstood, []byte("0: 99:short"))
	require.Error(t, err, "a not-understood message wider than the line must be refused")

	_, err = ParseAnswerTail(AnswerKindTerminator, []byte(""))
	require.Error(t, err, "a terminator stating no count at all must be refused")
}

// TestParseRPCErrorReadsMessageKey checks that a not-understood answer's text
// reaches the caller without its key. The method: the writer produces the
// answer, the frame layer hands over the payload, and the decoded error is
// compared with the text that was written.
//
// VALIDATES: R-4 of spec-streaming-answer-protocol -- parseRPCError
// reads the tail rather than treating it as message text.
// PREVENTS: an operator reading `message=unknown command: shwo bgp peers` with
// the key still in front of it.
func TestParseRPCErrorReadsMessageKey(t *testing.T) {
	t.Parallel()

	t.Run("message only", func(t *testing.T) {
		t.Parallel()

		_, _, payload, err := ParseLine(AppendAnswerNotUnderstood(nil, 7, "", "unknown command: shwo bgp peers"))
		require.NoError(t, err)

		got := parseRPCError(payload)
		assert.Empty(t, got.Code)
		assert.Equal(t, "unknown command: shwo bgp peers", got.Message)
	})

	t.Run("code and message", func(t *testing.T) {
		t.Parallel()

		_, _, payload, err := ParseLine(AppendAnswerNotUnderstood(nil, 7, "unknown-command", "unknown command: shwo bgp peers"))
		require.NoError(t, err)

		got := parseRPCError(payload)
		assert.Equal(t, "unknown-command", got.Code)
		assert.Equal(t, "unknown command: shwo bgp peers", got.Message)
	})

	t.Run("text that is neither json nor a tail", func(t *testing.T) {
		t.Parallel()

		got := parseRPCError([]byte("plain text"))
		assert.Empty(t, got.Code)
		assert.Equal(t, "plain text", got.Message)
	})
}

// TestHeadStatesHowItsItemsAreRead checks that every head names a type and that
// the reader refuses one that does not. The method: each of the three types is
// written and parsed back, and then five heads that state the type wrongly are
// fed to the reader.
//
// VALIDATES: type= is required on the head, and a reader never guesses how to
// read the items that follow it.
// PREVENTS: a consumer inferring the answer's shape from the first byte of the
// first record, which is the heuristic type= exists to remove.
func TestHeadStatesHowItsItemsAreRead(t *testing.T) {
	t.Parallel()

	document := parseAnswerLine(t, AppendAnswerHead(nil, 7, AnswerTypeDocument, "", nil))
	assert.Equal(t, AnswerTypeDocument, document.Type)
	assert.Empty(t, document.Fields)

	objects := parseAnswerLine(t, AppendAnswerHead(nil, 7, AnswerTypeMap, "peers", nil))
	assert.Equal(t, AnswerTypeMap, objects.Type)
	assert.Equal(t, "peers", objects.Key)

	positional := parseAnswerLine(t, AppendAnswerHead(nil, 7, AnswerTypeTable, "peers", json.RawMessage(`["peer","as","state"]`)))
	assert.Equal(t, AnswerTypeTable, positional.Type)
	assert.Equal(t, []string{"peer", "as", "state"}, positional.Fields)

	refused := []struct {
		name string
		kind string
		tail string
	}{
		{name: "a head with no item type", kind: AnswerKindHead, tail: ""},
		{name: "a head whose type is two bytes", kind: AnswerKindHead, tail: "do 5:peers 0:"},
		{name: "a head whose type is four bytes", kind: AnswerKindHead, tail: "docs 5:peers 0:"},
		{name: "a head stating a type nobody implements", kind: AnswerKindHead, tail: "pbf 0: 0:"},
		{name: "a head stating only its type", kind: AnswerKindHead, tail: "map"},
		{name: "columns with a type that does not read them", kind: AnswerKindHead, tail: `map 0: 10:["peer"]`},
		{name: "a table with no schema to read against", kind: AnswerKindHead, tail: "tab 0: 0:"},
		{name: "a table with an empty schema", kind: AnswerKindHead, tail: "tab 0: 2:[]"},
		{name: "a schema that is not an array of names", kind: AnswerKindHead, tail: `tab 0: 11:{"peer":1}`},
		{name: "a head carrying bytes past its last field", kind: AnswerKindHead, tail: "map 0: 0: extra"},
		// A counted field that states more bytes than arrived. It sits LAST on
		// the line, so a reader that clamped the length to what it got would
		// accept the line rather than refuse it: a shorter field would be
		// caught by the separator the next field expects, for the wrong
		// reason. The stated length is what a reader slices on, so it is
		// checked against the bytes that arrived before the slice.
		{name: "column names wider than the line", kind: AnswerKindHead, tail: `tab 0: 99:["peer"]`},
		{name: "an envelope name wider than the line", kind: AnswerKindHead, tail: "map 99:peers 0:"},
		// A counted text whose length is not closed by its colon. The bytes
		// after it are exactly the length it states, so a reader that took the
		// value without asking for the colon would accept the line.
		{name: "column names with no colon after their count", kind: AnswerKindHead, tail: `tab 0: 8["peer"]`},
		// Two fields with no space between them. Each field states its own
		// width, so a reader that did not ask for the separator would take
		// both and accept the line; the space is what says another field
		// follows rather than what says this one ended.
		{name: "a head whose item type is not separated", kind: AnswerKindHead, tail: "map0: 0:"},
		{name: "a head whose envelope name is not separated", kind: AnswerKindHead, tail: "map 0:0:"},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseAnswerTail(tt.kind, []byte(tt.tail))
			require.Error(t, err, "the reader accepted %q", tt.tail)
		})
	}
}

// TestCountedFieldsAreToldApartByTheirColon checks that the colon is what
// separates the two field types. A counted text carries one and a counted
// number does not, and each field refuses the other's spelling BY NAME.
//
// The method: a terminator whose counts are closed by a colon, a head whose two
// texts state a byte count with no colon after it, and a record whose payload
// does the same. Each is offered to the reader, and each refusal is required to
// name the byte.
//
// VALIDATES: the type rule -- a counted number is decimal digits and nothing
// else, and a counted text always carries its colon, an empty one included.
// PREVENTS: one field being read as the other, which takes the bytes after the
// count as a value and mis-slices every field that follows.
//
// The message is what is asserted, not the error. Every case here is also
// refused by the rest of the reader for a reason of its own: a separator that is
// not a space, or a head that ends where a field belongs. A test that required
// an error alone would therefore pass with both guards deleted.
func TestCountedFieldsAreToldApartByTheirColon(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind string
		tail string
		want string
	}{
		{
			name: "a terminator count closed by a colon",
			kind: AnswerKindTerminator,
			tail: "417:3 0 0:",
			want: "counted number is closed by ':'",
		},
		{
			name: "a terminator fault count closed by a colon",
			kind: AnswerKindTerminator,
			tail: "417 3:0 0:",
			want: "counted number is closed by ':'",
		},
		{
			name: "an envelope name whose count is not closed by a colon",
			kind: AnswerKindHead,
			tail: "map 5peers 0:",
			want: "counted text byte count is not closed by ':'",
		},
		{
			name: "empty column names with no colon at all",
			kind: AnswerKindHead,
			tail: "map 5:peers 0",
			want: "counted text byte count is not closed by ':'",
		},
		{
			name: "a record payload whose count is not closed by a colon",
			kind: AnswerKindRecord,
			tail: `19{"peer":"10.0.0.1"}`,
			want: "counted text byte count is not closed by ':'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseAnswerTail(tt.kind, []byte(tt.tail))
			require.Error(t, err, "the reader accepted %q", tt.tail)
			assert.ErrorContains(t, err, tt.want, "the refusal of %q names another byte", tt.tail)
		})
	}

	// The two spellings the guards exist to keep apart, each read as itself.
	terminator, err := ParseAnswerTail(AnswerKindTerminator, []byte("417 3 0:"))
	require.NoError(t, err)
	assert.Equal(t, uint64(417), terminator.Count, "a number closed by a space does not read back")
	assert.Empty(t, terminator.Message, "an empty text written 0: does not read back")
}

// TestColumnNamesAreCounted checks that the head's column names are a counted
// field rather than a run to the end of the line. The method: a schema holding a
// space and an = is written after an envelope name, the stated byte count is
// compared with the schema's length, and the line is parsed back name for name.
//
// VALIDATES: AC-13 -- the column names state their own width, so a column name
// needs no escaping and no field runs to the end of the line.
// PREVENTS: a later field being swallowed by the schema, which is what an
// open-ended value forced the writer's ordering to prevent.
func TestColumnNamesAreCounted(t *testing.T) {
	t.Parallel()

	schema := json.RawMessage(`["peer address","as=number","state"]`)
	line := AppendAnswerHead(nil, 7, AnswerTypeTable, "peers", schema)

	assert.Contains(t, string(line), strconv.Itoa(len(schema))+":"+string(schema),
		"the column names do not state their own byte count on %q", line)

	head := parseAnswerLine(t, line)
	assert.Equal(t, "peers", head.Key)
	assert.Equal(t, []string{"peer address", "as=number", "state"}, head.Fields)
}

// TestAnswerRecordLineSizeMeasuresTheLineItsAppenderWrites checks that the size
// a producer refuses a record by is the size of the line it would have written.
// The method: for a result record and a rejected one, over three id shapes and
// values holding a space, an = and a newline, the reported size is compared
// with the length of the appended line itself.
//
// VALIDATES: AC-15 of the streaming answer protocol -- a record is measured by
// its line, so a refusal fires where the transport would have failed.
// PREVENTS: a second spelling of the line format drifting from the appenders,
// which would refuse a record that fits or write one that does not.
func TestAnswerRecordLineSizeMeasuresTheLineItsAppenderWrites(t *testing.T) {
	t.Parallel()

	values := []json.RawMessage{
		nil,
		json.RawMessage(`{"peer":"10.0.0.1"}`),
		json.RawMessage(`{"leaf":1,"message":"invalid = value"}`),
		json.RawMessage("{\n\"leaf\": 1\n}"),
	}
	for _, id := range []uint64{AnswerNoID, 7, math.MaxUint64} {
		for _, value := range values {
			item := AppendAnswerItem(nil, id, value)
			if size := AnswerRecordLineSize(id, Record{Item: value}); size != len(item) {
				t.Errorf("id %d: the item line is %d bytes and its size reads %d", id, len(item), size)
			}
			if len(value) == 0 {
				continue
			}
			fault := AppendAnswerFault(nil, id, value)
			if size := AnswerRecordLineSize(id, Record{Fault: value}); size != len(fault) {
				t.Errorf("id %d: the fault line is %d bytes and its size reads %d", id, len(fault), size)
			}
		}
	}
}

// TestAppendRequestSpellsTheSameIDField checks that a request line carries the
// same id field an answer line carries. The method: one id is written into a
// request line, into both response lines, and into a record line, and each is
// required to open with the `#<id> ` the table spells out.
//
// VALIDATES: every line that carries an id spells it the same way, requests
// included, so the protocol carries ONE id encoding rather than two.
// PREVENTS: one direction of the protocol spelling the id differently from the
// other, which leaves a reader computing the wrong offset for every later
// field.
func TestAppendRequestSpellsTheSameIDField(t *testing.T) {
	t.Parallel()

	const method = "ze-bgp:peer-list"

	tests := []struct {
		name    string
		id      uint64
		idField string
	}{
		{"one digit", 7, "#7 "},
		{"two digits", 42, "#42 "},
		{"ten digits", 1234567890, "#1234567890 "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request := string(AppendRequest(nil, tt.id, method, nil))
			assert.Equal(t, method, strings.TrimPrefix(request, tt.idField),
				"the request line opens with %q", tt.idField)

			ok := string(AppendOK(nil, tt.id))
			assert.Equal(t, StatusOK, strings.TrimPrefix(ok, tt.idField),
				"the ok line opens with %q", tt.idField)

			failure := string(AppendError(nil, tt.id, nil))
			assert.Equal(t, StatusError, strings.TrimPrefix(failure, tt.idField),
				"the error line opens with %q", tt.idField)

			record := string(AppendAnswerItem(nil, tt.id, json.RawMessage(`{"peer":"10.0.0.1"}`)))
			assert.Equal(t, fmt.Sprintf("row %s", countedTextFixture(`{"peer":"10.0.0.1"}`)), strings.TrimPrefix(record, tt.idField),
				"the record line opens with the same %q the request line does", tt.idField)
		})
	}
}

// TestAnswerIDRoundTrip checks that every id a uint64 can hold reaches the wire
// and reads back. The method: for each digit count from 1 to 20 the smallest
// and the largest id of that width are written into a request line, the field
// is required to be `#`, those digits, and the space that closes it, and the
// line is read back.
//
// VALIDATES: the whole uint64 range is expressible, so no counter has to wrap
// to keep its id readable, and the space that closes the field is what a reader
// stops its fused loop at.
// PREVENTS: an id that reaches the wire and reads back as another number, and a
// field a reader cannot tell the end of.
func TestAnswerIDRoundTrip(t *testing.T) {
	t.Parallel()

	const method = "ze-bgp:peer-list"

	smallest := uint64(1)
	for digits := 1; digits <= 20; digits++ {
		largest := smallest*10 - 1
		if digits == 20 {
			largest = math.MaxUint64
		}
		for _, id := range []uint64{smallest, largest} {
			line := string(AppendRequest(nil, id, method, nil))
			spelled := strconv.FormatUint(id, 10)
			require.Equal(t, digits, len(spelled),
				"the fixture id %d is not %d digits wide", id, digits)

			require.Greater(t, len(line), digits+1, "line %q is shorter than its id field", line)
			assert.Equal(t, byte('#'), line[0], "line %q does not open with #", line)
			assert.Equal(t, spelled, line[1:1+digits], "line %q does not spell its id in decimal", line)
			assert.Equal(t, byte(' '), line[1+digits],
				"line %q does not close its id field with a space", line)

			readID, verb, _, err := ParseLine([]byte(line))
			require.NoError(t, err, "line %q does not read back", line)
			assert.Equal(t, id, readID, "line %q reads back a different id", line)
			assert.Equal(t, method, verb, "line %q reaches the wrong field after the id", line)
		}
		if digits < 20 {
			smallest *= 10
		}
	}
}

// TestAnswerIDMaxUint64 checks that the whole id range reaches the wire. The
// method: the largest uint64 is written into a request line and into a record
// line, both are required to open with its twenty decimal digits, and the
// request is read back.
//
// VALIDATES: A-2 -- every id Ze can produce is expressible, so no counter has
// to wrap to keep its id readable.
// PREVENTS: an id at the top of the range being truncated or rejected by its
// own encoding, which is the failure a wrapping counter would have been
// introduced to avoid.
func TestAnswerIDMaxUint64(t *testing.T) {
	t.Parallel()

	const maxIDField = "#18446744073709551615 "

	request := string(AppendRequest(nil, math.MaxUint64, "ze-bgp:peer-list", nil))
	assert.Equal(t, "ze-bgp:peer-list", strings.TrimPrefix(request, maxIDField),
		"the widest id writes its twenty decimal digits")

	record := string(AppendAnswerItem(nil, math.MaxUint64, json.RawMessage(`{"peer":"10.0.0.1"}`)))
	assert.Equal(t, fmt.Sprintf("row %s", countedTextFixture(`{"peer":"10.0.0.1"}`)), strings.TrimPrefix(record, maxIDField),
		"an answer line spells the widest id the same way")

	readID, verb, _, err := ParseLine([]byte(request))
	require.NoError(t, err)
	assert.Equal(t, uint64(math.MaxUint64), readID, "the widest id reads back")
	assert.Equal(t, "ze-bgp:peer-list", verb)
}

// TestAnswerIDFieldRejected checks that a reader refuses an id field it cannot
// trust rather than slicing on it. The method: each malformed id field is read,
// and the reader is required to return an error.
//
// VALIDATES: the boundary table of the id field -- the `#` is required, every
// byte before the space is a decimal digit, the field ends at a space, and an
// id past the 20 digits or past the range of a uint64 is refused.
// PREVENTS: a reader taking a verb out of the middle of a number, and an id
// that wraps silently into another conversation's.
//
// Each refusal names the guard that made it, because a malformed line is
// refused by whatever meets it FIRST and the rest of the reader would refuse
// several of these for a reason of its own. `#42` is the case that proves it: a
// line with no verb is refused whether or not the id field checks that it ends
// at a space, so only the message says which check fired.
func TestAnswerIDFieldRejected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		line    string
		refusal string
	}{
		// The boundary of the digit run, last valid first.
		{name: "twenty digits, the widest a uint64 occupies", line: "#18446744073709551615 ok"},
		{name: "twenty-one digits", line: "#000000000000000000001 ok", refusal: "past the 20 digits a uint64 occupies"},
		{name: "twenty digits past the range of a uint64", line: "#99999999999999999999 ok", refusal: "past the range of a uint64"},

		{name: "no hash", line: "1 ok", refusal: "missing # prefix"},
		{name: "nothing after the hash", line: "#", refusal: "does not end at a space"},
		{name: "no digits before the space", line: "# ok", refusal: "states no id digits"},
		{name: "no space after the id", line: "#42ok", refusal: "not a decimal digit"},
		{name: "digits that run to the end of the line", line: "#42", refusal: "does not end at a space"},
		{name: "a byte that is not a decimal digit", line: "#4x ok", refusal: "not a decimal digit"},
		{name: "a letter where the digits belong", line: "#abc ok", refusal: "not a decimal digit"},
		{name: "a sign in front of the digits", line: "#-1 ok", refusal: "not a decimal digit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, _, err := ParseLine([]byte(tt.line))
			if tt.refusal == "" {
				require.NoError(t, err, "line %q was refused rather than read", tt.line)
				return
			}
			require.Error(t, err, "line %q was read rather than refused", tt.line)
			assert.ErrorContains(t, err, tt.refusal,
				"line %q was refused by another check than the one under test", tt.line)
		})
	}
}

// parseAnswerLine takes one written answer line back through the frame layer
// and the tail reader, which is the route every consumer takes. The verb the
// frame layer cuts off IS the kind, so the tail reader is handed the token the
// writer put there rather than one the test names itself.
func parseAnswerLine(t *testing.T, line []byte) AnswerTail {
	t.Helper()

	_, kind, payload, err := ParseLine(line)
	require.NoError(t, err)

	tail, err := ParseAnswerTail(kind, payload)
	require.NoError(t, err)
	return tail
}

// answerKindLines writes one line of every kind under id, in the order the
// published line table lists them. The tests below read the wire the shipped
// writers produce rather than a spelling of their own.
func answerKindLines(id uint64) map[string]string {
	return map[string]string{
		AnswerKindHead:          string(AppendAnswerHead(nil, id, AnswerTypeMap, "peers", nil)),
		AnswerKindRecord:        string(AppendAnswerItem(nil, id, json.RawMessage(`{"peer":"10.0.0.1","state":"established"}`))),
		AnswerKindFault:         string(AppendAnswerFault(nil, id, json.RawMessage(`{"path":"bgp/peer/10.0.0.2","message":"nexthop unreachable"}`))),
		AnswerKindTerminator:    string(AppendAnswerTerminator(nil, id, 1, 1, "")),
		AnswerKindNotUnderstood: string(AppendAnswerNotUnderstood(nil, id, "", "unknown command: shwo bgp peers")),
	}
}

// TestParseAnswerLineFixedOffsets checks that every kind puts its token at the
// same offset, on both channels, and that a reader reaches it by arithmetic.
// The method: one line of each kind is written under three id widths and under
// AnswerNoID, and the token is read by computing the offset from the id's digit
// count rather than by searching the line for a space.
//
// VALIDATES: AC-3 -- on the mux channel the kind is reached from the id's
// length byte with one addition, identically for all five kinds.
// VALIDATES: AC-4 -- on the exec channel the kind starts at offset zero.
// PREVENTS: a reader scanning for the first space to find what a line is, which
// is the scan the fixed-width token exists to remove.
func TestParseAnswerLineFixedOffsets(t *testing.T) {
	t.Parallel()

	for _, id := range []uint64{7, 1234567890, math.MaxUint64} {
		// The id field is `#`, the decimal digits, and the space that closes
		// it, so the kind starts one byte past that space and nothing is
		// searched for.
		want := 2 + len(strconv.FormatUint(id, 10))
		for kind, line := range answerKindLines(id) {
			require.Greater(t, len(line), want+answerKindWidth, "line %q is shorter than its own prefix", line)
			assert.Equal(t, kind, line[want:want+answerKindWidth],
				"id %d: the %s line does not put its kind at offset %d", id, kind, want)
			assert.Equal(t, byte(' '), line[want+answerKindWidth],
				"id %d: the %s line does not close its kind with a space", id, kind)

			readID, readKind, payload, err := ParseLine([]byte(line))
			require.NoError(t, err, "line %q does not read back", line)
			assert.Equal(t, id, readID)
			assert.Equal(t, kind, readKind, "line %q reaches the wrong field after the id", line)

			tail, tailErr := ParseAnswerTail(readKind, payload)
			require.NoError(t, tailErr, "line %q carries a tail its kind refuses", line)
			assert.Equal(t, kind, tail.Kind, "the tail reader keeps the kind the wire stated")
		}
	}

	for kind, line := range answerKindLines(AnswerNoID) {
		assert.Equal(t, kind, line[:answerKindWidth],
			"the exec channel does not put the %s token at offset zero: %q", kind, line)
		assert.Equal(t, byte(' '), line[answerKindWidth],
			"the exec channel does not close the %s token with a space: %q", kind, line)

		readKind, tail, err := ParseAnswerLine([]byte(line))
		require.NoError(t, err, "line %q does not read back", line)
		assert.Equal(t, kind, readKind)
		assert.Equal(t, kind, tail.Kind)
	}
}

// TestParseAnswerLineUnknownKind checks that a reader refuses a kind it does
// not know rather than guessing one from the tail. The method: lines whose
// token is not a kind, and lines whose token merely starts with one, are fed to
// both readers.
//
// VALIDATES: AC-7 -- an unknown three-byte kind is refused with a named error,
// and the error names the vocabulary rather than echoing the line.
// PREVENTS: a reader falling back on the tail to decide what a line is, which
// would read a rejected row as a result once the keys leave the wire (R-5).
func TestParseAnswerLineUnknownKind(t *testing.T) {
	t.Parallel()

	refused := []struct {
		name string
		line string
	}{
		{name: "a token no kind claims", line: "xyz doc 0: 0:"},
		{name: "the verb an answer line used to open with", line: "ok doc 0: 0:"},
		{name: "the error verb", line: "error 15:unknown command"},
		{name: "a token whose tail would name a terminator", line: "xyz 2 0 0:"},
		{name: "a kind spelled in upper case", line: "END 2 0 0:"},
		{name: "a longer word opening with a kind", line: "topple doc 0: 0:"},
		{name: "a kind with no space after it", line: "end2 0 0:"},
		// Each of these carries a tail its kind would accept, so the byte that
		// refuses it is the one the token must be closed with and nothing else.
		{name: "a terminator tail behind a separator that is not a space", line: "endX2 0 0:"},
		{name: "a record tail behind a separator that is not a space", line: `rowX{"peer":"10.0.0.1"}`},
		{name: "a head tail behind a separator that is not a space", line: "top:doc 0: 0:"},
		{name: "a kind and no tail", line: "end"},
		{name: "a line shorter than one kind", line: "en"},
		{name: "an empty line", line: ""},
	}

	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := ParseAnswerLine([]byte(tt.line))
			require.Error(t, err, "the exec reader accepted %q", tt.line)
		})
	}

	_, err := ParseAnswerTail("xyz", []byte("2 0 0:"))
	require.Error(t, err, "the tail reader accepted an unknown kind")
	assert.Contains(t, err.Error(), answerKindWords, "the refusal names the kinds a reader accepts")
}

// TestAnswerLineTableMatchesDoc checks that the line table published in
// docs/architecture/api/ipc_protocol.md is the table the writers write. The
// method: one line of every kind is built by the shipped appenders and the
// document is required to carry each of them byte for byte.
//
// VALIDATES: the published grammar cannot drift from the writer
// (ai/rules/evidence.md), and every kind is a whole word with a distinct first
// byte (AC-16).
// PREVENTS: an operator reading a captured session against a line table that
// stopped being true, which is the whole cost of positional fields.
func TestAnswerLineTableMatchesDoc(t *testing.T) {
	t.Parallel()

	seen := make(map[byte]string, len(answerKinds))
	for _, kind := range answerKinds {
		require.Len(t, kind, answerKindWidth, "kind %q is not %d bytes", kind, answerKindWidth)
		assert.Equal(t, strings.ToLower(kind), kind, "kind %q is not lower case", kind)
		if other, clash := seen[kind[0]]; clash {
			t.Errorf("kinds %q and %q share their first byte, so a machine cannot switch on one load", other, kind)
		}
		seen[kind[0]] = kind
	}

	published, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "architecture", "api", "ipc_protocol.md"))
	require.NoError(t, err)

	for kind, line := range answerKindLines(7) {
		assert.Contains(t, string(published), line,
			"the published line table carries no %s line spelled the way the writer spells one", kind)
	}
}

// TestAnswerLineCarriesNoKeyNames checks that no key name reaches the wire. The
// method: one line of every kind is written by the shipped appenders, the id
// field and the payload are set aside, and what is left is required to carry no
// `=` at all; then each retired key name is offered to the reader and refused.
//
// The record kinds set their payload aside because a JSON value may hold an `=`
// inside a string. What is asserted for them is that the tail is the count and
// the payload and nothing else, so no key name sits between the kind and the
// count.
//
// VALIDATES: AC-13 and AC-14 -- every field is positional, and ParseAnswerTail
// dispatches on no key name.
// PREVENTS: a key creeping back onto the hot line. `item=` and `fault=` were
// five and six bytes on every line of a million-row walk, and each was a second
// statement of what the kind already says.
func TestAnswerLineCarriesNoKeyNames(t *testing.T) {
	t.Parallel()

	payloads := map[string]string{
		AnswerKindRecord: `{"peer":"10.0.0.1","state":"established"}`,
		AnswerKindFault:  `{"path":"bgp/peer/10.0.0.2","message":"nexthop unreachable"}`,
	}
	for kind, line := range answerKindLines(7) {
		tail := strings.TrimPrefix(line, "#7 "+kind+" ")
		require.NotEqual(t, line, tail, "the %s line does not open with its id and kind: %q", kind, line)

		if payload, carried := payloads[kind]; carried {
			assert.Equal(t, countedTextFixture(payload), tail,
				"the %s line puts something between its kind and its counted payload: %q", kind, line)
			continue
		}
		assert.NotContains(t, tail, "=", "the %s line carries a key=value pair: %q", kind, line)
	}

	// Each name a line used to spell. The reader takes none of them, so a peer
	// still writing one is refused rather than half-read.
	retired := []struct {
		kind string
		tail string
	}{
		{kind: AnswerKindHead, tail: "status=done type=ndjson key=peers"},
		{kind: AnswerKindTerminator, tail: "count=97 faults=3"},
		{kind: AnswerKindTerminator, tail: "count=0 message=rib snapshot expired"},
		{kind: AnswerKindNotUnderstood, tail: "code=unknown-command message=no such command"},
		// A well-formed positional tail with a key hung off the end. The
		// reader refuses the whole line rather than taking the fields it
		// understood and ignoring the rest, which is what would let a key
		// creep back on one kind at a time.
		{kind: AnswerKindHead, tail: "map 5:peers 0: status=done"},
		{kind: AnswerKindTerminator, tail: "2 0 0: status=done"},
		{kind: AnswerKindNotUnderstood, tail: "0: 2:no code=unknown-command"},
	}
	for _, tt := range retired {
		_, err := ParseAnswerTail(tt.kind, []byte(tt.tail))
		require.Error(t, err, "the %s reader accepted the key=value tail %q", tt.kind, tt.tail)
	}
}

// TestEnvelopeKeyLengthPrefixed checks that the head's envelope name states its
// own width and is never omitted. The method: names of several widths, and no
// name at all, are written and read back; the written bytes are compared against
// the length prefix the id uses; and a head that omits the field is refused.
//
// VALIDATES: AC-17 -- the name is length-prefixed like the id, and an absent
// name writes length zero rather than dropping the field.
// PREVENTS: an absent name shortening the line, which would move every field
// after it and make the head's field count depend on its content.
func TestEnvelopeKeyLengthPrefixed(t *testing.T) {
	t.Parallel()

	names := []string{"", "p", "peers", "bgp-peers-with-a-long-envelope-name", strings.Repeat("n", 300)}
	for _, name := range names {
		line := AppendAnswerHead(nil, 7, AnswerTypeMap, name, nil)
		assert.Contains(t, string(line), strconv.Itoa(len(name))+":"+name,
			"the envelope name does not state its own byte count on %q", line)

		head := parseAnswerLine(t, line)
		assert.Equal(t, name, head.Key, "the envelope name does not read back from %q", line)
	}

	// The absent name is present and empty, so the field count never varies.
	absent := string(AppendAnswerHead(nil, 7, AnswerTypeMap, "", nil))
	named := string(AppendAnswerHead(nil, 7, AnswerTypeMap, "peers", nil))
	assert.Len(t, strings.Split(absent, " "), len(strings.Split(named, " ")),
		"a head naming no envelope has a different field count: %q against %q", absent, named)
	assert.Contains(t, absent, " 0: ", "an absent envelope name is not written as length zero: %q", absent)

	// A head that omits the field is a line this build cannot read.
	_, err := ParseAnswerTail(AnswerKindHead, []byte("map 0:"))
	require.Error(t, err, "a head that omits its column names must be refused")
}

// TestAnswerRecordLineSizeAllocatesNothing checks that measuring a record line
// costs no allocation, which is what answerRecordPrefixMax is for. The method:
// the widest prefix the encoder can write (the widest id and the kind token) is
// measured under testing.AllocsPerRun.
//
// VALIDATES: the record path stays allocation-free after the kind token widened
// the prefix (ai/rules/performance.md).
// PREVENTS: answerRecordPrefixWidth being left behind by a wider prefix, which
// makes the stack scratch grow into the heap once per record of every walk.
func TestAnswerRecordLineSizeAllocatesNothing(t *testing.T) {
	item := Record{Item: json.RawMessage(`{"peer":"10.0.0.1","state":"established"}`)}
	fault := Record{Fault: json.RawMessage(`{"path":"bgp/peer/10.0.0.2","message":"nexthop unreachable"}`)}

	// The prefix the scratch must hold whole: the widest id, the kind token,
	// the space that closes it, and the counted number stating the payload's
	// byte count at its widest.
	widest := len(appendAnswerRecordPrefix(nil, math.MaxUint64, AnswerKindFault, math.MaxUint64))
	require.LessOrEqual(t, widest, answerRecordPrefixWidth,
		"the widest record prefix is %d bytes and the scratch holds %d", widest, answerRecordPrefixWidth)

	for _, record := range []Record{item, fault} {
		allocs := testing.AllocsPerRun(100, func() {
			if AnswerRecordLineSize(math.MaxUint64, record) == 0 {
				t.Error("the measured line is empty")
			}
		})
		assert.Zero(t, allocs, "measuring a record line allocated %v times", allocs)
	}
}

// TestAppendAnswerRecordNoKey checks that a record line carries its payload
// straight after the kind, behind nothing but the count that says how wide it
// is. The method: a result record and a rejected one are written under three id
// widths, each line is rebuilt from the grammar and compared byte for byte, the
// payload is reached by adding the widths the line states, and a line carrying
// one byte past its payload is offered to the reader.
//
// VALIDATES: AC-5 -- the payload's byte count follows the kind token and the
// payload follows the count, with no key between them and no separator searched
// for at either end.
// PREVENTS: a key creeping back onto the line that repeats, and a reader
// looking for the newline to find where the payload stops, which is the last
// scan the counted payload removes.
func TestAppendAnswerRecordNoKey(t *testing.T) {
	t.Parallel()

	appenders := map[string]func(uint64, json.RawMessage) []byte{
		AnswerKindRecord: func(id uint64, value json.RawMessage) []byte { return AppendAnswerItem(nil, id, value) },
		AnswerKindFault:  func(id uint64, value json.RawMessage) []byte { return AppendAnswerFault(nil, id, value) },
	}
	payloads := map[string]json.RawMessage{
		AnswerKindRecord: json.RawMessage(`{"peer":"10.0.0.1","state":"established"}`),
		AnswerKindFault:  json.RawMessage(`{"path":"bgp/peer/10.0.0.2","message":"nexthop = unreachable"}`),
	}

	for _, id := range []uint64{AnswerNoID, 7, math.MaxUint64} {
		for kind, appender := range appenders {
			payload := payloads[kind]
			line := string(appender(id, payload))

			// The whole line, spelled from the grammar rather than read off the
			// writer, so a key between the kind and the payload has to be added
			// in both places to pass.
			want := fmt.Sprintf("%s%s %s:%s", idFieldFixture(id), kind, countedNumberFixture(len(payload)), payload)
			require.Equal(t, want, line, "id %d: the %s line is not its prefix and its payload", id, kind)

			// The payload is reached by adding the widths the line states, and
			// the line is searched for nothing.
			start := recordPayloadStart(t, id, line)
			assert.Equal(t, string(payload), line[start:start+len(payload)],
				"id %d: the %s payload does not sit where the counted prefix says", id, kind)
			assert.Len(t, line, start+len(payload),
				"id %d: the %s line carries bytes past its payload", id, kind)

			// Nothing sits between the kind and the count that says how wide the
			// payload is.
			assert.NotContains(t, line[:start], "=",
				"id %d: the %s line carries a key in front of its payload: %q", id, kind, line)

			read := readAnswerLine(t, id, line)
			assert.Equal(t, kind, read.Kind)
			carried := read.Item
			if kind == AnswerKindFault {
				carried = read.Fault
			}
			assert.Equal(t, string(payload), string(carried), "id %d: the %s payload does not read back", id, kind)
		}
	}

	// The count is what says where the payload stops, so a byte past it is a
	// line this build cannot read whole rather than a payload one byte longer.
	overrun := AppendAnswerItem(nil, 7, json.RawMessage(`{"peer":"10.0.0.1"}`))
	_, _, payload, err := ParseLine(append(overrun, 'X'))
	require.NoError(t, err)
	_, err = ParseAnswerTail(AnswerKindRecord, payload)
	require.Error(t, err, "a record line carrying a byte past its counted payload was read rather than refused")
}

// readAnswerLine takes one written answer line back through the reader of the
// channel it was written for: the mux channel carries an id and the exec
// channel carries none, so each has its own entry point.
func readAnswerLine(t *testing.T, id uint64, line string) AnswerTail {
	t.Helper()

	if id != AnswerNoID {
		return parseAnswerLine(t, []byte(line))
	}
	_, tail, err := ParseAnswerLine([]byte(line))
	require.NoError(t, err)
	return tail
}

// idFieldFixture spells the `#<id> ` an answer line opens with, and nothing for
// the exec channel's AnswerNoID. It is written out here rather than read from
// appendID, so a change to the field has to be made in both places.
func idFieldFixture(id uint64) string {
	if id == AnswerNoID {
		return ""
	}
	return fmt.Sprintf("#%d ", id)
}

// countedNumberFixture spells the decimal digits a counted number is written
// as. It is written out here rather than read from the writer, so a change to
// the field has to be made in both places and cannot pass unnoticed.
func countedNumberFixture(value int) string {
	return strconv.Itoa(value)
}

// countedTextFixture spells the `<n>:<bytes>` a counted text is written as,
// where `<n>` is the BYTE count of the value. The colon is always there, so an
// empty text is `0:`.
func countedTextFixture(text string) string {
	return fmt.Sprintf("%s:%s", countedNumberFixture(len(text)), text)
}

// recordPayloadStart returns the offset a record line's payload starts at. It is
// computed from the widths the line states and from no search of the line: the
// id field, the three-byte kind and the space that closes it, then the digits
// stating the payload's byte count and the colon that closes them.
func recordPayloadStart(t *testing.T, id uint64, line string) int {
	t.Helper()

	at := len(idFieldFixture(id)) + answerKindWidth + 1
	require.Equal(t, byte(' '), line[at-1], "the kind of %q is not closed by a space", line)
	digits := 0
	for line[at+digits] >= '0' && line[at+digits] <= '9' {
		digits++
	}
	require.NotZero(t, digits, "the payload count of %q states no digits", line)
	require.Equal(t, byte(':'), line[at+digits], "the payload count of %q is not closed by its colon", line)
	return at + digits + 1
}

// TestAnswerRecordLineSizeExact checks that the size a producer refuses a record
// by is EXACTLY the size of the line the appender writes, and that the scratch
// constant it measures in is exactly the widest prefix. The method: the widest
// prefix is built and compared with the constant; then, for payload widths
// either side of every count that gains a digit, the measured size is compared
// with the length of the written line under four id widths.
//
// VALIDATES: the phase-6 deliverable -- the measured size equals the written
// size, and the prefix constant is exact rather than a maximum.
// PREVENTS: a size that forgot the counted header, which would let a record
// through that the transport then refuses; and a constant left loose, which
// costs nothing a running test can see and is why this states equality rather
// than a bound.
func TestAnswerRecordLineSizeExact(t *testing.T) {
	t.Parallel()

	widest := len(appendAnswerRecordPrefix(nil, math.MaxUint64, AnswerKindFault, math.MaxUint64))
	assert.Equal(t, answerRecordPrefixWidth, widest,
		"the widest record prefix is %d bytes and the constant states %d", widest, answerRecordPrefixWidth)

	for _, width := range []int{0, 1, 9, 10, 99, 100, 999, 1000, 9999, 10000} {
		value := json.RawMessage(strings.Repeat("v", width))
		for _, id := range []uint64{AnswerNoID, 1, 7, 1234567890, math.MaxUint64} {
			item := AppendAnswerItem(nil, id, value)
			assert.Equal(t, len(item), AnswerRecordLineSize(id, Record{Item: value}),
				"id %d: a %d-byte item line is %d bytes and its size reads %d", id, width, len(item), AnswerRecordLineSize(id, Record{Item: value}))
			if width == 0 {
				// A record carrying neither an item nor a fault measures as an
				// empty item, which the case above already covers.
				continue
			}
			fault := AppendAnswerFault(nil, id, value)
			assert.Equal(t, len(fault), AnswerRecordLineSize(id, Record{Fault: value}),
				"id %d: a %d-byte fault line is %d bytes and its size reads %d", id, width, len(fault), AnswerRecordLineSize(id, Record{Fault: value}))
		}
	}
}

// TestAnswerWordsComeFromTheirTokens checks that the integer a reader compares
// a three-letter word against is the word its token spells. The method: every
// kind's and every item type's constant is rebuilt here from its own string and
// its closing space, and compared with the one the package derived.
//
// VALIDATES: the tokens and the integers cannot drift apart, which is the whole
// risk a single-load compare takes on.
// PREVENTS: a hand-typed constant that silently disagrees with its token, which
// passes review and refuses a well-formed line in production.
func TestAnswerWordsComeFromTheirTokens(t *testing.T) {
	t.Parallel()

	words := map[string]answerWord{
		AnswerKindHead:          answerWordHead,
		AnswerKindRecord:        answerWordRecord,
		AnswerKindFault:         answerWordFault,
		AnswerKindTerminator:    answerWordTerminator,
		AnswerKindNotUnderstood: answerWordNotUnderstood,
		AnswerTypeDocument:      answerWordTypeDocument,
		AnswerTypeMap:           answerWordTypeMap,
		AnswerTypeTable:         answerWordTypeTable,
	}

	seen := make(map[answerWord]string, len(words))
	for token, word := range words {
		// Rebuilt byte by byte rather than through answerWordOf, so the writer
		// of the constants is not also the judge of them.
		spelled := token + " "
		want := answerWord(spelled[0]) |
			answerWord(spelled[1])<<8 |
			answerWord(spelled[2])<<16 |
			answerWord(spelled[3])<<24
		assert.Equal(t, want, word, "the word of %q is not the word %q spells", token, spelled)

		if other, clash := seen[word]; clash {
			t.Errorf("tokens %q and %q share one word, so one compare cannot tell them apart", other, token)
		}
		seen[word] = token

		kind, known := answerKindOfWord(word)
		itemType, typed := answerTypeOfWord(word)
		assert.True(t, known || typed, "the word of %q is read back as neither a kind nor an item type", token)
		if known {
			assert.Equal(t, token, kind)
		}
		if typed {
			assert.Equal(t, token, itemType)
		}
	}
}

// TestEveryThreeLetterWordIsSpaceClosed checks the grammar's MUST: a
// three-letter word on an answer line is always followed by a space, and never
// by the line terminator. The method: one line of every kind is built by the
// shipped appenders, under an id and without one, and the byte after each word
// is required to be a space; then a word sitting last on a line is offered to
// both readers.
//
// VALIDATES: the rule that makes a four-byte load unconditionally safe, so no
// reader needs a bounds case for a word that ends a line.
// PREVENTS: a writer putting a token last on a line, which would make every
// word compare in the protocol a special case, and a reader accepting one and
// loading a byte the line does not carry.
func TestEveryThreeLetterWordIsSpaceClosed(t *testing.T) {
	t.Parallel()

	for _, id := range []uint64{AnswerNoID, 7, math.MaxUint64} {
		for kind, line := range answerKindLines(id) {
			at := len(idFieldFixture(id))
			require.Greater(t, len(line), at+answerKindWidth,
				"id %d: the %s line ends at its kind: %q", id, kind, line)
			assert.Equal(t, byte(' '), line[at+answerKindWidth],
				"id %d: the %s line does not close its kind with a space: %q", id, kind, line)

			if kind != AnswerKindHead {
				continue
			}
			// The head's item type is the same shape and carries the same rule.
			at += answerWordWidth
			require.Greater(t, len(line), at+answerTypeWidth,
				"id %d: the head ends at its item type: %q", id, line)
			assert.Equal(t, byte(' '), line[at+answerTypeWidth],
				"id %d: the head does not close its item type with a space: %q", id, line)
		}
	}

	// A word sitting last on a line is not a line this build reads.
	for _, kind := range answerKinds {
		_, known := answerKindAt(kind)
		assert.False(t, known, "the kind %q was read with no space after it", kind)

		_, _, err := ParseAnswerLine([]byte(kind))
		require.Error(t, err, "an answer line that is nothing but the kind %q was read", kind)
	}
	_, err := ParseAnswerTail(AnswerKindHead, []byte(AnswerTypeMap))
	require.Error(t, err, "a head whose item type ends the line was read")
}
