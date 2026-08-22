package rpc

import (
	"bytes"
	"encoding/json"
	"math"
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
		{"invalid id", "#abc method", 0, "", "", true},
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
func TestParseLineCarriesKeyValueTailWhole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		line        string
		wantVerb    string
		wantPayload string
	}{
		{
			name:        "head",
			line:        "#7 ok status=done key=peers",
			wantVerb:    "ok",
			wantPayload: "status=done key=peers",
		},
		{
			name:        "item holding = and spaces",
			line:        `#7 ok item={"peer":"10.0.0.1","note":"a=b c"}`,
			wantVerb:    "ok",
			wantPayload: `item={"peer":"10.0.0.1","note":"a=b c"}`,
		},
		{
			name:        "terminator",
			line:        "#7 ok count=97 faults=3",
			wantVerb:    "ok",
			wantPayload: "count=97 faults=3",
		},
		{
			name:        "not understood",
			line:        "#7 error message=unknown command: shwo bgp peers",
			wantVerb:    "error",
			wantPayload: "message=unknown command: shwo bgp peers",
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
// an answer from the tail's keys alone. The method: tails whose open-ended
// value is not JSON at all are parsed, and the control keys are asserted.
//
// VALIDATES: the verdict is a key, never a payload field.
// PREVENTS: a consumer parsing a record payload to learn what the line is,
// which is the whole-answer materialization this protocol exists to remove.
func TestTailTokenizerNeedsNoJSONDecoder(t *testing.T) {
	t.Parallel()

	head, err := ParseAnswerTail([]byte("status=error type=ndjson key=peers"))
	require.NoError(t, err)
	assert.Equal(t, StatusError, head.Status)
	assert.Equal(t, AnswerTypeNDJSON, head.Type)
	assert.Equal(t, "peers", head.Key)
	assert.False(t, head.IsTerminator())

	record, err := ParseAnswerTail([]byte("item=<<this is not json>>"))
	require.NoError(t, err)
	assert.Equal(t, "<<this is not json>>", string(record.Item))
	assert.False(t, record.IsTerminator())

	terminator, err := ParseAnswerTail([]byte("count=97 faults=3"))
	require.NoError(t, err)
	assert.Equal(t, uint64(97), terminator.Count)
	assert.Equal(t, uint64(3), terminator.Faults)
	assert.True(t, terminator.IsTerminator())
}

// TestOpenEndedKeyRunsToEndOfLine checks that item=, fault= and message= take
// the rest of the line verbatim. The method: values holding = and spaces are
// written, parsed back through the frame layer, and compared byte for byte;
// then a value holding a newline is written and the line count is asserted.
//
// VALIDATES: AC-11 -- a record whose JSON value contains = and spaces
// round-trips with no escaping and no quoting.
// PREVENTS: an escaping scheme being invented for the tail, and an open-ended
// value splitting one answer line into two.
func TestOpenEndedKeyRunsToEndOfLine(t *testing.T) {
	t.Parallel()

	t.Run("item holding = and spaces", func(t *testing.T) {
		t.Parallel()
		item := json.RawMessage(`{"filter":"community = 65000:1","note":"a b c"}`)

		tail := parseAnswerLine(t, AppendAnswerItem(nil, 7, item))
		assert.Equal(t, string(item), string(tail.Item))
		assert.Empty(t, tail.Fault)
	})

	t.Run("fault holding = and spaces", func(t *testing.T) {
		t.Parallel()
		fault := json.RawMessage(`{"path":"bgp/peer/10.0.0.2","message":"nexthop = unreachable"}`)

		tail := parseAnswerLine(t, AppendAnswerFault(nil, 7, fault))
		assert.Equal(t, string(fault), string(tail.Fault))
		assert.Empty(t, tail.Item)
	})

	t.Run("message holding = and spaces", func(t *testing.T) {
		t.Parallel()

		tail := parseAnswerLine(t, AppendAnswerTerminator(nil, 7, 417, 0, "rib snapshot expired: age = 4h"))
		assert.Equal(t, uint64(417), tail.Count)
		assert.Equal(t, "rib snapshot expired: age = 4h", tail.Message)
	})

	t.Run("newline in a value stays on one line", func(t *testing.T) {
		t.Parallel()

		line := AppendAnswerTerminator(nil, 7, 0, 0, "peer rejected\nthe route")
		assert.NotContains(t, string(line), "\n")

		tail := parseAnswerLine(t, line)
		assert.Equal(t, "peer rejected the route", tail.Message)
	})
}

// TestTerminatorIsTheLineCarryingCount checks that a reader tells the head from
// the terminator by a key rather than by position. The method: a head and a
// terminator are written and parsed, and each is asked what it is with no other
// line in hand.
//
// VALIDATES: the first of the four structural rules -- a reader needs no
// lookahead and no state beyond "have I seen the head".
// PREVENTS: a reader buffering the answer to find out where it ends.
func TestTerminatorIsTheLineCarryingCount(t *testing.T) {
	t.Parallel()

	head := parseAnswerLine(t, AppendAnswerHead(nil, 7, StatusDone, AnswerTypeNDJSON, "peers", nil))
	assert.False(t, head.IsTerminator())
	assert.Equal(t, StatusDone, head.Status)
	assert.Equal(t, AnswerTypeNDJSON, head.Type)
	assert.Equal(t, "peers", head.Key)

	record := parseAnswerLine(t, AppendAnswerItem(nil, 7, json.RawMessage(`{"peer":"10.0.0.1"}`)))
	assert.False(t, record.IsTerminator())

	terminator := parseAnswerLine(t, AppendAnswerTerminator(nil, 7, 2, 0, ""))
	assert.True(t, terminator.IsTerminator())
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
		{name: "aborted before any record", count: 0, message: "peer 10.0.0.1 not configured", want: VerdictAborted},
		{name: "aborted with faults", count: 97, faults: 3, message: "rib snapshot expired", want: VerdictAborted},
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
		head := parseAnswerLine(t, AppendAnswerHead(nil, 7, StatusDone, AnswerTypeNDJSON, "peers", nil))
		assert.Equal(t, VerdictTruncated, Verdict(&head))
	})
}

// TestTerminatorCarriesNoStatusKey checks that the counts stay the single
// source of the verdict. The method: every terminator shape is written and
// searched for a status key, and a terminator that states one is fed to the
// reader.
//
// VALIDATES: AC-12 -- no status= key is written on a terminator.
// PREVENTS: a second source of truth for a fact the counts already hold, which
// is a disagreement a consumer would have to resolve.
func TestTerminatorCarriesNoStatusKey(t *testing.T) {
	t.Parallel()

	shapes := [][]byte{
		AppendAnswerTerminator(nil, 7, 0, 0, ""),
		AppendAnswerTerminator(nil, 7, 2, 0, ""),
		AppendAnswerTerminator(nil, 7, 97, 3, ""),
		AppendAnswerTerminator(nil, 7, 0, 3, ""),
		AppendAnswerTerminator(nil, 7, 417, 0, "rib snapshot expired"),
		AppendAnswerTerminator(nil, 7, 0, 0, "peer 10.0.0.1 not configured"),
	}
	for _, shape := range shapes {
		assert.NotContains(t, string(shape), answerKeyStatus+"=", "terminator states a status")
	}

	_, err := ParseAnswerTail([]byte("count=2 status=done"))
	require.Error(t, err, "a terminator stating a status must be refused")
	assert.Contains(t, err.Error(), "derives")
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
	assert.True(t, lowest.IsTerminator())

	highest := parseAnswerLine(t, AppendAnswerTerminator(nil, 7, math.MaxUint64, math.MaxUint64, ""))
	assert.Equal(t, uint64(math.MaxUint64), highest.Count)
	assert.Equal(t, uint64(math.MaxUint64), highest.Faults)

	_, err := ParseAnswerTail([]byte("count=18446744073709551616"))
	require.Error(t, err, "a count past max uint64 must be refused")

	_, err = ParseAnswerTail([]byte("count=-1"))
	require.Error(t, err, "a negative count must be refused")
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

	document := parseAnswerLine(t, AppendAnswerHead(nil, 7, StatusDone, AnswerTypeJSON, "", nil))
	assert.Equal(t, AnswerTypeJSON, document.Type)
	assert.Empty(t, document.Fields)

	objects := parseAnswerLine(t, AppendAnswerHead(nil, 7, StatusDone, AnswerTypeNDJSON, "peers", nil))
	assert.Equal(t, AnswerTypeNDJSON, objects.Type)
	assert.Equal(t, "peers", objects.Key)

	positional := parseAnswerLine(t, AppendAnswerHead(nil, 7, StatusDone, AnswerTypeStream, "peers", json.RawMessage(`["peer","as","state"]`)))
	assert.Equal(t, AnswerTypeStream, positional.Type)
	assert.Equal(t, []string{"peer", "as", "state"}, positional.Fields)

	refused := []struct {
		name string
		tail string
	}{
		{name: "a head with no type", tail: "status=done key=peers"},
		{name: "a head stating a type nobody implements", tail: "status=done type=protobuf"},
		{name: "a type on a line that is not a head", tail: "type=json item=[1]"},
		{name: "fields with a type that does not read them", tail: `status=done type=ndjson fields=["peer"]`},
		{name: "a stream with no schema to read against", tail: "status=done type=stream"},
		{name: "a stream with an empty schema", tail: "status=done type=stream fields=[]"},
		{name: "a schema that is not an array of names", tail: `status=done type=stream fields={"peer":1}`},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseAnswerTail([]byte(tt.tail))
			require.Error(t, err, "the reader accepted %q", tt.tail)
		})
	}
}

// TestFieldsRunToEndOfLine checks that the column schema is the last key on the
// head. The method: a schema holding a space and an = is written after an
// envelope key, parsed back, and compared name for name.
//
// VALIDATES: fields= is open-ended, so a column name needs no escaping.
// PREVENTS: a key written after fields= being swallowed by it, which is what
// the writer's ordering exists to prevent.
func TestFieldsRunToEndOfLine(t *testing.T) {
	t.Parallel()

	schema := json.RawMessage(`["peer address","as=number","state"]`)
	line := AppendAnswerHead(nil, 7, StatusDone, AnswerTypeStream, "peers", schema)

	assert.True(t, bytes.HasSuffix(line, schema), "fields= is not last on %q", line)

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

// parseAnswerLine takes one written answer line back through the frame layer
// and the tail reader, which is the route every consumer takes.
func parseAnswerLine(t *testing.T, line []byte) AnswerTail {
	t.Helper()

	_, _, payload, err := ParseLine(line)
	require.NoError(t, err)

	tail, err := ParseAnswerTail(payload)
	require.NoError(t, err)
	return tail
}
