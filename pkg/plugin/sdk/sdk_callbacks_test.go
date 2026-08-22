package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// executeCommand asks a running plugin to run one command and returns the value
// the engine rebuilds from its answer.
//
// Every execute-command answer a plugin writes is a head, its records and a
// terminator. This reads it the way the engine reads it
// (PluginConn.SendExecuteCommand, internal/component/plugin/ipc/rpc.go): read
// the answer, collapse it, and rebuild the status from the terminator. The
// engine tree cannot be imported from here, so the three steps are spelled
// rather than shared.
func executeCommand(t *testing.T, ctx context.Context, mux *rpc.MuxConn, input rpc.ExecuteCommandInput) *rpc.ExecuteCommandOutput {
	t.Helper()

	answer, err := mux.CallAnswer(ctx, "ze-plugin-callback:execute-command", input)
	require.NoError(t, err)

	document, collapseErr := rpc.CollapseAnswer(answer)
	require.NoError(t, answer.Err(), "the answer must reach its terminator")
	require.NoError(t, collapseErr)
	if message := answer.Message(); message != "" {
		return &rpc.ExecuteCommandOutput{Status: rpc.StatusError, Data: json.RawMessage(message)}
	}
	return &rpc.ExecuteCommandOutput{Status: rpc.StatusDone, Data: document}
}

func TestOnExecuteCommandAnyMap(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", map[string]any{"running": true, "peers": 5}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	out := executeCommand(t, ctx, engine.mux,
		rpc.ExecuteCommandInput{Command: "test"})
	assert.Equal(t, "done", out.Status)

	var data map[string]any
	require.NoError(t, json.Unmarshal(out.Data, &data))
	assert.Equal(t, true, data["running"])
	assert.Equal(t, float64(5), data["peers"])

	cancel()
	<-errCh
}

func TestOnExecuteCommandAnyStruct(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	type entry struct {
		Prefix  string `json:"prefix"`
		NextHop string `json:"next-hop"`
	}

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", []entry{{Prefix: "10.0.0.0/24", NextHop: "10.0.0.1"}}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	out := executeCommand(t, ctx, engine.mux,
		rpc.ExecuteCommandInput{Command: "show"})
	assert.Equal(t, "done", out.Status)

	var entries []entry
	require.NoError(t, json.Unmarshal(out.Data, &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "10.0.0.0/24", entries[0].Prefix)

	cancel()
	<-errCh
}

func TestOnExecuteCommandAnyNil(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", nil, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	out := executeCommand(t, ctx, engine.mux,
		rpc.ExecuteCommandInput{Command: "noop"})
	assert.Equal(t, "done", out.Status)
	assert.Empty(t, out.Data)

	cancel()
	<-errCh
}

func TestOnExecuteCommandAnySlice(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", []string{"cache", "route", "peer"}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	out := executeCommand(t, ctx, engine.mux,
		rpc.ExecuteCommandInput{Command: "events"})
	assert.Equal(t, "done", out.Status)

	var events []string
	require.NoError(t, json.Unmarshal(out.Data, &events))
	assert.Equal(t, []string{"cache", "route", "peer"}, events)

	cancel()
	<-errCh
}

// TestOnExecuteCommandRawMessagePassthrough locks in the contract that a handler
// returning json.RawMessage is embedded verbatim (single marshal), not re-quoted.
// Pipeline terminals and the RPKI handler rely on this to ship hand-built JSON.
func TestOnExecuteCommandRawMessagePassthrough(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", json.RawMessage(`{"running":true,"peers":5}`), nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	out := executeCommand(t, ctx, engine.mux,
		rpc.ExecuteCommandInput{Command: "raw"})
	assert.Equal(t, "done", out.Status)

	// Data is an object, NOT a JSON string -- the RawMessage passed through unquoted.
	var data map[string]any
	require.NoError(t, json.Unmarshal(out.Data, &data))
	assert.Equal(t, true, data["running"])

	cancel()
	<-errCh
}

// TestOnDoctorCheckCallback verifies the doctor-check callback dispatches correctly.
func TestOnDoctorCheckCallback(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnDoctorCheck(func(name string) ([]rpc.DoctorCheckDiagnostic, error) {
		return []rpc.DoctorCheckDiagnostic{
			{Code: "doctor-rpki-cache-unreachable", Severity: "warning", Message: "cache at 10.0.0.1:8282 not responding"},
		}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:doctor-check",
		rpc.DoctorCheckInput{Name: "rpki-cache-reachable"})
	require.NoError(t, err)

	var out rpc.DoctorCheckOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Len(t, out.Diagnostics, 1)
	assert.Equal(t, "doctor-rpki-cache-unreachable", out.Diagnostics[0].Code)
	assert.Equal(t, "warning", out.Diagnostics[0].Severity)

	cancel()
	<-errCh
}

// TestOnDoctorCheckDefaultNoOp verifies empty diagnostics when no handler registered.
func TestOnDoctorCheckDefaultNoOp(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:doctor-check",
		rpc.DoctorCheckInput{Name: "any-check"})
	require.NoError(t, err)

	var out rpc.DoctorCheckOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Empty(t, out.Diagnostics)

	cancel()
	<-errCh
}

// TestOnEnrichShowCallback verifies the enrich-show callback dispatches correctly.
//
// VALIDATES: AC-2 (external plugin receives base map, returns enrichment)
// PREVENTS: SDK callback wiring regression for enrich-show RPC.
func TestOnEnrichShowCallback(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnEnrichShow(func(command, key, mode string, base map[string]any) (map[string]any, error) {
		return map[string]any{
			"cos-profile": "residential",
			"speed":       float64(1000),
			"mode":        mode,
		}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:enrich-show",
		rpc.EnrichShowInput{
			Command: "show subscriber detail",
			Key:     "cos",
			Mode:    "detail",
			Base:    map[string]any{"id": "s1", "state": "active"},
		})
	require.NoError(t, err)

	var out rpc.EnrichShowOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "residential", out.Data["cos-profile"])
	assert.Equal(t, float64(1000), out.Data["speed"])
	assert.Equal(t, "detail", out.Data["mode"])

	cancel()
	<-errCh
}

// TestOnEnrichShowDefaultNoOp verifies empty data when no handler registered.
func TestOnEnrichShowDefaultNoOp(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:enrich-show",
		rpc.EnrichShowInput{
			Command: "show subscriber detail",
			Key:     "cos",
			Base:    map[string]any{"id": "s1"},
		})
	require.NoError(t, err)

	var out rpc.EnrichShowOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Empty(t, out.Data)

	cancel()
	<-errCh
}

// TestOnExecuteCommandStringIsDoubleEncoded documents the hazard that motivated
// the single-marshal sweep: a handler that returns a pre-marshaled JSON *string*
// (instead of a Go value) is re-quoted by the SDK, so the wire Data is a JSON
// string, not the intended object. Handlers must return structs/maps/RawMessage.
func TestOnExecuteCommandStringIsDoubleEncoded(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", `{"running":true}`, nil // BUG shape: pre-marshaled string
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	out := executeCommand(t, ctx, engine.mux,
		rpc.ExecuteCommandInput{Command: "str"})

	// Data decodes to a STRING, not an object -- proving the double-encode.
	// A handler must NOT return a JSON string; this asserts the failure mode.
	var asString string
	require.NoError(t, json.Unmarshal(out.Data, &asString))
	assert.Equal(t, `{"running":true}`, asString)

	var asObject map[string]any
	assert.Error(t, json.Unmarshal(out.Data, &asObject),
		"a pre-marshaled string must not decode as an object (it is double-encoded)")

	cancel()
	<-errCh
}

// executeCommandValueCases are the answers a command handler builds before its
// answer opens, each with the status its head declares and the exact bytes its
// one item= line carries. The bytes are literals rather than a second marshal of
// the same value: a golden derived from the code under test agrees with that
// code whatever it does.
//
// They are the shapes the tree already produces. plugin.Map is a map, the RPKI
// handler and the pipeline terminals ship json.RawMessage, and a handler that
// produced nothing answers with none.
var executeCommandValueCases = []struct {
	name        string
	command     string
	status      string
	data        any
	wantMessage string
	wantItem    string
}{
	{
		name:     "a map",
		command:  "status",
		status:   "done",
		data:     map[string]any{"running": true, "peers": 5},
		wantItem: `{"peers":5,"running":true}`,
	},
	{
		name:     "a slice of structs",
		command:  "show",
		status:   "done",
		data:     []executeCommandEntry{{Prefix: "10.0.0.0/24", NextHop: "10.0.0.1"}},
		wantItem: `[{"prefix":"10.0.0.0/24","next-hop":"10.0.0.1"}]`,
	},
	{
		name:     "a slice of strings",
		command:  "events",
		status:   "done",
		data:     []string{"cache", "route", "peer"},
		wantItem: `["cache","route","peer"]`,
	},
	{
		name:     "hand-built JSON",
		command:  "raw",
		status:   "done",
		data:     json.RawMessage(`{"running":true,"peers":5}`),
		wantItem: `{"running":true,"peers":5}`,
	},
	{
		name:     "no data at all",
		command:  "noop",
		status:   "done",
		data:     nil,
		wantItem: "",
	},
	{
		// A failure rides the TERMINATOR, which is the one line an answer
		// states its outcome on, and it carries no record: the reason is the
		// answer.
		name:        "a failure the handler reported as a status",
		command:     "broken",
		status:      "error",
		data:        map[string]any{"reason": "no cache"},
		wantMessage: `{"reason":"no cache"}`,
		wantItem:    "",
	},
}

// executeCommandEntry is one row of the struct-slice answer above.
type executeCommandEntry struct {
	Prefix  string `json:"prefix"`
	NextHop string `json:"next-hop"`
}

// TestExecuteCommandValueAnswerIsUnchanged checks that a command handler
// answering with a value it built reaches the engine as exactly the bytes it
// reached the engine as before a handler could answer with rows. The method:
// one plugin serves every shape a handler answers with today, and each answer's
// item is compared against the literal bytes, byte for byte.
//
// The comparison is against a literal and not against a second marshal of the
// same value, because a golden the code under test computes agrees with that
// code whatever it does.
//
// The FRAME around those bytes is the only frame there is: this plugin's answer
// is a head, one item= line and a terminator, whether the handler built a value
// or produced rows. Nothing chooses it from the payload, because the engine must
// know which frame is arriving before it reads the first line (AC-8). What AC-5
// holds fixed is the VALUE inside it.
//
// VALIDATES: AC-5 of spec-record-answers-1-sdk-path -- a handler answering with
// a built value is unchanged from today, byte for byte.
// PREVENTS: the record answer form changing the payload of the 219 plugin.Map
// call sites that answer with a built value, none of which produce rows.
func TestExecuteCommandValueAnswerIsUnchanged(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	answers := make(map[string]struct {
		status string
		data   any
	}, len(executeCommandValueCases))
	for _, tc := range executeCommandValueCases {
		answers[tc.command] = struct {
			status string
			data   any
		}{status: tc.status, data: tc.data}
	}

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		answer, known := answers[command]
		if !known {
			return "", nil, fmt.Errorf("unknown command %q", command)
		}
		return answer.status, answer.data, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	for _, tc := range executeCommandValueCases {
		answer, err := engine.mux.CallAnswer(ctx, "ze-plugin-callback:execute-command",
			rpc.ExecuteCommandInput{Command: tc.command})
		require.NoError(t, err, tc.name)
		assert.Equal(t, rpc.AnswerTypeDocument, answer.Type,
			"%s: a value the handler built is one document", tc.name)

		var items []string
		for record := range answer.Records {
			require.Empty(t, record.Fault, "%s: a built value rejects no row", tc.name)
			items = append(items, string(record.Item))
		}
		require.NoError(t, answer.Err(), tc.name)
		assert.Equal(t, tc.wantMessage, answer.Message(),
			"%s: the terminator is the one line an answer states its outcome on", tc.name)

		if tc.wantItem == "" {
			assert.Empty(t, items, "%s: a handler that produced nothing carries no item line", tc.name)
			continue
		}
		require.Len(t, items, 1, "%s: a built value is one record", tc.name)
		assert.Equal(t, tc.wantItem, items[0], "%s: the value must be byte for byte what it was", tc.name)
	}

	cancel()
	<-errCh
}

// jsonRow is a row a test handler already holds as JSON, appended into whatever
// buffer the SDK offers it.
type jsonRow string

// AppendTo appends the row's JSON to buf and returns the extended slice.
func (r jsonRow) AppendTo(buf []byte) []byte { return append(buf, r...) }

// peerRows is the walk a test command answers with: one self-describing row for
// each of count peers, produced one at a time.
func peerRows(count int, produced *int) iter.Seq[Record] {
	return func(yield func(Record) bool) {
		for i := range count {
			if produced != nil {
				*produced++
			}
			row := jsonRow(`{"peer":"10.0.0.` + strconv.Itoa(i) + `"}`)
			if !yield(Record{Item: row}) {
				return
			}
		}
	}
}

// TestExecuteCommandRecordResult checks that a command handler answering with a
// row generator reaches the engine as a record answer rather than as one
// marshaled value. The method: the engine asks the plugin for a command through
// the answer-reading call and reads the head, every record and the terminator
// that come back.
//
// Two row counts are driven, because the count is what decides the shape. A
// walk past rpc.AnswerBufferThreshold is streamed, so the engine reads one row
// at a time and never holds the collection. A walk under it is the one document
// a command answered with before it could produce rows, so no consumer of an
// existing command meets a new shape.
//
// VALIDATES: AC-4 of spec-record-answers-1-sdk-path -- a handler answers with a
// row generator and the rows reach the engine as records, in walk order, with
// the terminator's verdict.
// PREVENTS: executeCommandOutput marshaling the walk whole, which is the
// double materialization this spec exists to remove.
func TestExecuteCommandRecordResult(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		command  string
		rows     int
		wantType string
	}{
		{
			name:     "a walk past the threshold is streamed",
			command:  "show peers",
			rows:     rpc.AnswerBufferThreshold + 3,
			wantType: rpc.AnswerTypeMap,
		},
		{
			name:     "a walk under the threshold is one document",
			command:  "show peers brief",
			rows:     2,
			wantType: rpc.AnswerTypeDocument,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, engine := newTestPair(t)

			produced := 0
			p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
				return "done", Records{Key: "peers", Rows: peerRows(tc.rows, &produced)}, nil
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			errCh := make(chan error, 1)
			go func() { errCh <- p.Run(ctx, Registration{}) }()
			completeStartup(t, ctx, engine)

			answer, err := engine.mux.CallAnswer(ctx, "ze-plugin-callback:execute-command",
				rpc.ExecuteCommandInput{Command: tc.command})
			require.NoError(t, err, "the plugin must answer a command that walks with the answer sequence")
			assert.Equal(t, tc.wantType, answer.Type)

			var items []string
			for record := range answer.Records {
				require.Empty(t, record.Fault, "no row of this walk is rejected")
				items = append(items, string(record.Item))
			}
			assert.Equal(t, rpc.VerdictDone, answer.Verdict())
			assert.NoError(t, answer.Err())
			assert.Equal(t, tc.rows, produced, "every row of the walk was produced")

			if tc.wantType == rpc.AnswerTypeDocument {
				assert.Empty(t, answer.Key,
					"a document answer names no envelope on its head: the document already carries it")
				require.Len(t, items, 1, "a bounded walk is one record carrying the whole document")
				assert.Equal(t, `{"peers":[{"peer":"10.0.0.0"},{"peer":"10.0.0.1"}]}`, items[0])
			} else {
				assert.Equal(t, "peers", answer.Key, "the head names the envelope the rows belong under")
				require.Len(t, items, tc.rows, "every row of the walk reaches the engine as its own record")
				assert.Equal(t, `{"peer":"10.0.0.0"}`, items[0], "the rows arrive in walk order")
				assert.Equal(t, `{"peer":"10.0.0.`+strconv.Itoa(tc.rows-1)+`"}`, items[len(items)-1])
			}

			cancel()
			<-errCh
		})
	}
}
