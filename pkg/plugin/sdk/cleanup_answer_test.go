// Design: docs/architecture/api/process-protocol.md -- retained record outcomes.
package sdk

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: buffered dispatch keeps valid rows, every refused row, warning lines
// and the actual outcome. Method: use real rpc.Answer walks, including the
// document produced by a short record walk. These are projection questions;
// the independent wire/bridge fixture proves transport delivery.
func TestCleanupAnswerBufferedDispatchKeepsFaultOutcome(t *testing.T) {
	for _, test := range []struct {
		name          string
		items, faults uint64
		warning       bool
		want          string
	}{
		{name: "empty", want: rpc.VerdictDone},
		{name: "healthy", items: 1, want: rpc.VerdictDone},
		{name: "partial", items: 1, faults: 2, want: rpc.VerdictPartial},
		{name: "all-refused", faults: 2, want: rpc.VerdictError},
		{name: "warning", items: 1, warning: true, want: rpc.VerdictWarning},
		{name: "partial-warning", items: 1, faults: 1, warning: true, want: rpc.VerdictPartial},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := []rpc.Record{}
			for range test.items {
				rows = append(rows, rpc.Record{Item: json.RawMessage(`{"owner":"original"}`)})
			}
			for range test.faults {
				rows = append(rows, rpc.Record{Fault: json.RawMessage(`{"cause":"original rejected owner"}`)})
			}
			warnings := []string(nil)
			if test.warning {
				warnings = []string{"original warning one", "original warning two"}
			}
			answer := rpc.NewAnswer(rpc.AnswerTail{Type: rpc.AnswerTypeMap, Key: "owners"}, rpc.AnswerTail{Count: test.items, Faults: test.faults, Warning: test.warning, Warnings: warnings}, slices.Values(rows))
			result, err := answerValue(answer, "cleanup-answer-test")
			if result.Status != test.want {
				t.Fatalf("status=%s want %s", result.Status, test.want)
			}
			if test.faults > 0 {
				if err == nil {
					t.Fatal("refused records became a successful buffered dispatch")
				}
			}
			if test.faults == 0 {
				if err != nil {
					t.Fatal(err)
				}
			}
			if !slices.Equal(result.Warnings, warnings) {
				t.Fatalf("warning lines changed: %#v", result.Warnings)
			}
			var document struct {
				Owners []json.RawMessage `json:"owners"`
				Errors []json.RawMessage `json:"errors"`
			}
			if err := json.Unmarshal(result.Data, &document); err != nil {
				t.Fatal("failure discarded its structured data:", err)
			}
			if uint64(len(document.Owners)) != test.items {
				t.Fatal("valid rows were lost")
			}
			if uint64(len(document.Errors)) != test.faults {
				t.Fatal("refused rows were lost")
			}
		})
	}
}

// Goal: the existing execute-command value DTO carries exact record counts
// after its single collapse. Method: drive the SDK producer with short and
// streamed-size walks; it must walk each producer once, including the empty
// case. The metadata is read by ipc.valueAnswer, not inferred from JSON keys.
func TestCleanupAnswerExecuteValueKeepsRecordMetadata(t *testing.T) {
	for _, size := range []int{0, 1, rpc.AnswerBufferThreshold + 1} {
		t.Run(strings.Repeat("r", min(size, 3)), func(t *testing.T) {
			walks := 0
			produced := 0
			records := Records{Key: "owners", Rows: func(yield func(Record) bool) {
				walks++
				var row rpc.RawRow
				for index := range size {
					row = rpc.RawRow(`{"owner":"original"}`)
					produced++
					if index%2 == 0 {
						if !yield(Record{Fault: &row}) {
							return
						}
						continue
					}
					if !yield(Record{Item: &row}) {
						return
					}
				}
			}}
			out, err := executeCommandOutput(rpc.StatusDone, records)
			if err != nil {
				t.Fatal(err)
			}
			if walks != 1 {
				t.Fatalf("walks=%d want one", walks)
			}
			if produced != size {
				t.Fatal("producer stopped early")
			}
			encoded, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			var document struct {
				RecordAnswer *struct {
					Count  uint64 `json:"count"`
					Faults uint64 `json:"faults"`
				} `json:"record-answer"`
			}
			if err := json.Unmarshal(encoded, &document); err != nil {
				t.Fatal(err)
			}
			if document.RecordAnswer == nil {
				t.Fatal("bridge lost record outcome metadata")
			}
			if document.RecordAnswer.Count != uint64(size/2) {
				t.Fatal("bridge changed successful record count")
			}
			if document.RecordAnswer.Faults != uint64((size+1)/2) {
				t.Fatal("bridge changed rejected record count")
			}
			tail := rpc.AnswerTail{Kind: rpc.AnswerKindTerminator, Count: document.RecordAnswer.Count, Faults: document.RecordAnswer.Faults}
			if out.Status != rpc.Verdict(&tail) {
				t.Fatal("status contradicts the original walk")
			}
			if !json.Valid(out.Data) {
				t.Fatal("bridge discarded the complete collapsed document")
			}
		})
	}
}

// Goal: the actual SDK callback carries the same empty, healthy, partial and
// refused outcomes on the real socket and typed bridge. Method: complete the
// existing five-stage SDK startup on each route and call its installed command
// handler, with both sides of the record-buffer threshold.
func TestCleanupAnswerSDKTransportsKeepRefusedRecords(t *testing.T) {
	for _, transport := range []string{"wire", "bridge"} {
		t.Run(transport, func(t *testing.T) {
			var p *Plugin
			var engine *engineSide
			var bridge *rpc.DirectBridge
			if transport == "bridge" {
				p, engine, bridge = newBridgedTestPair(t)
			} else {
				p, engine = newTestPair(t)
			}
			p.OnExecuteCommand(func(_ string, _ string, args []string, _ string) (string, any, error) {
				var input struct {
					Count   int  `json:"count"`
					Faults  int  `json:"faults"`
					Warning bool `json:"warning"`
				}
				if err := json.Unmarshal([]byte(args[0]), &input); err != nil {
					return "", nil, err
				}
				status := rpc.StatusDone
				if input.Warning {
					status = rpc.StatusWarning
				}
				return status, Records{Key: "owners", Rows: func(yield func(Record) bool) {
					var row rpc.RawRow
					for range input.Count {
						row = rpc.RawRow(`{"owner":"retained-original"}`)
						if !yield(Record{Item: &row}) {
							return
						}
					}
					for range input.Faults {
						row = rpc.RawRow(`{"cause":"original cleanup refused"}`)
						if !yield(Record{Fault: &row}) {
							return
						}
					}
				}}, nil
			})
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			finished := make(chan error, 1)
			go func() { finished <- p.Run(ctx, &Registration{}) }()
			defer func() {
				cancel()
				select {
				case err := <-finished:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(2 * time.Second):
					t.Error("SDK callback owner did not stop")
				}
			}()
			completeStartup(t, ctx, engine)
			if bridge != nil {
				require.Eventually(t, bridge.HasExecuteCommand, 2*time.Second, time.Millisecond)
			}
			for _, test := range []struct {
				Count, Faults int
				Warning       bool
				Want          string
			}{
				{Want: rpc.VerdictDone}, {Count: 1, Want: rpc.VerdictDone}, {Count: 257, Want: rpc.VerdictDone},
				{Count: 1, Faults: 1, Want: rpc.VerdictPartial}, {Count: 257, Faults: 2, Want: rpc.VerdictPartial},
				{Faults: 1, Want: rpc.VerdictError}, {Faults: 257, Want: rpc.VerdictError},
				{Count: 1, Warning: true, Want: rpc.VerdictWarning}, {Count: 257, Warning: true, Want: rpc.VerdictWarning},
			} {
				encoded, err := json.Marshal(map[string]any{"count": test.Count, "faults": test.Faults, "warning": test.Warning})
				if err != nil {
					t.Fatal(err)
				}
				var status string
				var document json.RawMessage
				if bridge != nil {
					out, err := bridge.ExecuteCommand(ctx, "cleanup-answer", "show cleanup-answer", []string{string(encoded)}, "*")
					if err != nil {
						t.Fatal(err)
					}
					status, document = out.Status, out.Data
				} else {
					answer, err := engine.mux.CallAnswer(ctx, "ze-plugin-callback:execute-command", rpc.ExecuteCommandInput{Command: "show cleanup-answer", Args: []string{string(encoded)}})
					if err != nil {
						t.Fatal(err)
					}
					document, err = rpc.CollapseAnswer(answer)
					if err != nil {
						t.Fatal(err)
					}
					if err := answer.Err(); err != nil {
						t.Fatal(err)
					}
					status = answer.Verdict()
				}
				if status != test.Want {
					t.Fatalf("count=%d faults=%d status=%s want=%s", test.Count, test.Faults, status, test.Want)
				}
				var got struct {
					Owners []json.RawMessage `json:"owners"`
					Errors []json.RawMessage `json:"errors"`
				}
				if err := json.Unmarshal(document, &got); err != nil {
					t.Fatal(err)
				}
				if len(got.Owners) != test.Count {
					t.Fatal("actual callback discarded completed rows")
				}
				if len(got.Errors) != test.Faults {
					t.Fatal("actual callback discarded refused rows")
				}
			}
		})
	}
}

// Goal: interrupted evaluation preserves observed good rows while refusing a
// complete result. Method: the real Answer reader encounters an invalid empty
// record after one good row and therefore has no terminator.
func TestCleanupAnswerBufferedInterruptKeepsObservedProgress(t *testing.T) {
	answer := rpc.NewAnswer(rpc.AnswerTail{Type: rpc.AnswerTypeMap, Key: "owners"}, rpc.AnswerTail{Count: 2}, slices.Values([]rpc.Record{{Item: json.RawMessage(`{"recovered":1}`)}, {}}))
	result, err := answerValue(answer, "cleanup-answer-interrupt")
	if err == nil {
		t.Fatal("interrupted evaluation became success")
	}
	if result.Status != rpc.VerdictTruncated {
		t.Fatal("interrupted evaluation became complete")
	}
	if !strings.Contains(string(result.Data), `"recovered":1`) {
		t.Fatal("interrupted evaluation discarded observed progress")
	}
}

// Goal: a command's own failed tail keeps its successful payload and warning
// lines beside a failing result. Method: project the real completed Answer.
func TestCleanupAnswerBufferedFailureKeepsDataAndWarnings(t *testing.T) {
	answer := rpc.NewAnswer(rpc.AnswerTail{Type: rpc.AnswerTypeDocument}, rpc.AnswerTail{Count: 1, Message: "original evaluation stopped", Warning: true, Warnings: []string{"original warning"}}, slices.Values([]rpc.Record{{Item: json.RawMessage(`{"recovered":1}`)}}))
	result, err := answerValue(answer, "cleanup-answer-test")
	if err == nil {
		t.Fatal("failed tail became success")
	}
	if err.Error() != "original evaluation stopped" {
		t.Fatal("public command failure changed:", err)
	}
	if result.Status != rpc.VerdictAborted {
		t.Fatal("partial progress lost its aborted outcome")
	}
	if string(result.Data) != `{"recovered":1}` {
		t.Fatal("failed tail discarded real partial progress")
	}
	if !slices.Equal(result.Warnings, []string{"original warning"}) {
		t.Fatal("failed tail discarded warnings")
	}
}
