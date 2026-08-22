// Design: docs/architecture/testing/ci-format.md -- record-answer test plugin
// Related: cmd_engine_steps.go -- the other SDK plugin a .ci spawns
//          ../../../pkg/plugin/records.go -- Records, the walk a handler answers with
//          ../../../pkg/plugin/sdk/sdk_engine.go -- DispatchCommandAnswer, the streamed read
//
// cmdRecordPlugin is a Go SDK plugin that answers commands with a walk and
// reads an engine answer as one. It exists because the record path has two
// halves and only a running daemon joins them. A plugin's own rows travel to an
// operator through execute-command. An engine command's rows travel back
// through dispatch-command. Four .ci files under test/plugin/ drive it.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// recordPluginName is the plugin name a .ci file must declare:
//
//	plugin { external record-plugin { run "ze-test record-plugin" } }
//
// The spawn env binds the connect-back token to this exact name.
const recordPluginName = "record-plugin"

// The commands this plugin registers. Each one is one property of the record
// path, so a .ci that fails names the half that broke.
const (
	recordWalkCommand     = "show test records walk"
	recordFaultCommand    = "show test records fault"
	recordDocumentCommand = "show test records document"
	recordEngineCommand   = "show test engine answer"
)

// engineWalkCommand is the engine command the plugin reads as a streamed
// answer. It is the longest answer the daemon produces, so it passes
// rpc.AnswerBufferThreshold on any registry the tree can have.
const engineWalkCommand = "system command list"

// recordWalkRows and recordWalkRowBytes size the walk recordWalkCommand
// produces, and both numbers are load-bearing.
//
// The row count passes rpc.AnswerBufferThreshold, so the answer streams rather
// than collapsing to one document. The total width passes rpc.MaxMessageSize,
// so the collection has no form as a single wire message. A producer that built
// the whole answer and wrote it on one line cannot carry this walk at all.
// That is what makes test/plugin/plugin-owned-command-streams.ci discriminate.
// The two renderings of a walk are otherwise the same document by design
// (Records.MarshalJSON, pkg/plugin/records.go).
const (
	recordWalkRows     = 300
	recordWalkRowBytes = 60000
)

// recordFaultRows, recordFaultRowBytes and recordFaultIndex size the walk
// recordFaultCommand produces. It is a short walk, so the answer is one
// document. One row of it is wider than any line can carry.
//
// The wide row is rejected and the walk goes on. The answer therefore reports
// the rows it applied beside the row it refused
// (boundedRecord, pkg/plugin/rpc/answer_write.go).
const (
	recordFaultRows      = 12
	recordFaultRowBytes  = 64
	recordFaultIndex     = 6
	recordFaultWideBytes = rpc.MaxMessageSize + 1
)

// recordDocumentRows and recordDocumentRowBytes size the walk
// recordDocumentCommand produces, and both numbers are load-bearing.
//
// The row count is inside rpc.AnswerBufferThreshold, so the answer is one
// document rather than a stream. Each row carries five eighths of
// rpc.MaxMessageSize, so every row fits a line on its own and the two of them
// collapse to a document a quarter wider than any line can carry. What this
// answer rejects is therefore the DOCUMENT and never a row, which is what
// separates it from recordFaultCommand.
const (
	recordDocumentRows     = 2
	recordDocumentRowBytes = rpc.MaxMessageSize * 5 / 8
)

// engineAnswerWait bounds how long recordEngineCommand waits for the startup
// read to finish. The read starts when every plugin is ready, and an operator
// reaches this command later, so the wait is usually zero.
const engineAnswerWait = 15 * time.Second

// cmdRecordPlugin runs the plugin. It answers recordWalkCommand and
// recordFaultCommand with a walk, and recordEngineCommand with what it read
// from the engine's own streamed answer.
func cmdRecordPlugin(_ []string) int {
	p, err := sdk.NewFromEnv(recordPluginName)
	if err != nil {
		slog.Error("record-plugin: connect back to the engine", "error", err)
		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader := &engineAnswerReader{done: make(chan struct{})}

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		switch command {
		case recordWalkCommand:
			return rpc.StatusDone, sdk.Records{
				Key:  "rows",
				Rows: recordRows(recordWalkRows, recordWalkRowBytes, -1),
			}, nil
		case recordFaultCommand:
			return rpc.StatusDone, sdk.Records{
				Key:  "rows",
				Rows: recordRows(recordFaultRows, recordFaultRowBytes, recordFaultIndex),
			}, nil
		case recordDocumentCommand:
			return rpc.StatusDone, sdk.Records{
				Key:  "rows",
				Rows: recordRows(recordDocumentRows, recordDocumentRowBytes, -1),
			}, nil
		case recordEngineCommand:
			return reader.answer()
		}
		return rpc.StatusError, nil, fmt.Errorf("record-plugin: unknown command %q", command)
	})

	// The engine answers dispatch-command only after every plugin is ready, and
	// this handler answers an engine RPC, so it must return promptly. The read
	// therefore runs in its own goroutine: one goroutine for the life of one
	// plugin, ended by the walk it was started for.
	p.OnAllPluginsReady(func() error {
		go reader.read(ctx, p)
		return nil
	})

	registration := sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: recordWalkCommand, Description: "Walk that streams its rows"},
			{Name: recordFaultCommand, Description: "Walk with one row no line can carry"},
			{Name: recordDocumentCommand, Description: "Walk whose collapsed document no line can carry"},
			{Name: recordEngineCommand, Description: "What the plugin read from a streamed engine answer"},
		},
	}
	if runErr := p.Run(ctx, registration); runErr != nil {
		slog.Error("record-plugin: plugin loop ended", "error", runErr)
		return 1
	}
	return 0
}

// recordRow is one row of a walk: its position, and a filler that gives the row
// the width the walk was sized for.
//
// The filler is one string the whole walk shares, so a wide walk costs one
// allocation rather than one for each row.
type recordRow struct {
	index int
	fill  string
}

// AppendTo appends the row's JSON to buf. The filler is ASCII letters only, so
// nothing in it needs a JSON escape and the row can be written without a
// marshaler.
func (r recordRow) AppendTo(buf []byte) []byte {
	buf = append(buf, `{"index":`...)
	buf = strconv.AppendInt(buf, int64(r.index), 10)
	buf = append(buf, `,"fill":"`...)
	buf = append(buf, r.fill...)
	return append(buf, `"}`...)
}

// recordRows returns a walk of rows rows, each carrying fill bytes of filler.
// The row at wide, when it is a real index, carries recordFaultWideBytes
// instead, which is more than one wire message can hold.
//
// The two filler strings are built once for the walk and shared by every row,
// which is what the appender form of plugin.Row makes possible.
func recordRows(rows, fill, wide int) iter.Seq[plugin.Record] {
	return func(yield func(plugin.Record) bool) {
		narrow := strings.Repeat("x", fill)
		var wideFill string
		if wide >= 0 && wide < rows {
			wideFill = strings.Repeat("x", recordFaultWideBytes)
		}
		for index := range rows {
			row := recordRow{index: index, fill: narrow}
			if index == wide {
				row.fill = wideFill
			}
			if !yield(plugin.Record{Item: row}) {
				return
			}
		}
	}
}

// engineAnswerReading is what one streamed engine answer turned out to be, in
// the shape recordEngineCommand answers with.
//
// Type is the head's item type, and it is the fact that says the answer streamed:
// rpc.AnswerTypeMap for a walk that passed the threshold, rpc.AnswerTypeDocument
// for one that did not. Rows is what this plugin counted, and Verdict is what
// the terminator derived, so the two together say the whole answer arrived.
type engineAnswerReading struct {
	Type    string `json:"type"`
	Key     string `json:"key"`
	Rows    int    `json:"rows"`
	Verdict string `json:"verdict"`
	First   string `json:"first"`
	Last    string `json:"last"`
}

// engineAnswerReader holds the one reading of the engine's answer and the
// channel that says it is finished.
//
// read is the only writer of reading and err, and it closes done when it has
// written them. A reader MUST wait on done before it reads either field, which
// answer does. Not safe for concurrent use in any other pattern.
type engineAnswerReader struct {
	done    chan struct{}
	reading engineAnswerReading
	err     error
}

// read walks one streamed engine answer and keeps what it saw. It holds no row.
// The count, the first name and the last name are all it carries out of a walk
// of any length. That is the property a streamed read exists for.
func (r *engineAnswerReader) read(ctx context.Context, p *sdk.Plugin) {
	defer close(r.done)

	answer, err := p.DispatchCommandAnswer(ctx, engineWalkCommand)
	if err != nil {
		r.err = fmt.Errorf("dispatch %q: %w", engineWalkCommand, err)
		return
	}

	reading := engineAnswerReading{Type: answer.Type, Key: answer.Key}
	for record := range answer.Records {
		var row struct {
			Value string `json:"value"`
		}
		if unmarshalErr := json.Unmarshal(record.Item, &row); unmarshalErr != nil {
			r.err = fmt.Errorf("record %d of %q: %w", reading.Rows, engineWalkCommand, unmarshalErr)
			return
		}
		if reading.Rows == 0 {
			reading.First = row.Value
		}
		reading.Last = row.Value
		reading.Rows++
	}

	// Read after the range, never before: the range is what fills them.
	if walkErr := answer.Err(); walkErr != nil {
		r.err = fmt.Errorf("walk %q: %w", engineWalkCommand, walkErr)
		return
	}
	reading.Verdict = answer.Verdict()
	r.reading = reading
	slog.Info("record-plugin: read the engine answer",
		"type", reading.Type, "rows", reading.Rows, "verdict", reading.Verdict)
}

// answer reports the reading as the command's payload. It waits for the read
// that started at plugin readiness. When that wait runs out it names the wait,
// rather than answering an empty reading.
func (r *engineAnswerReader) answer() (string, any, error) {
	select {
	case <-r.done:
	case <-time.After(engineAnswerWait):
		return rpc.StatusError, nil, errors.New("record-plugin: the engine answer has not been read yet")
	}
	if r.err != nil {
		return rpc.StatusError, nil, r.err
	}
	return rpc.StatusDone, map[string]any{"engine-answer": r.reading}, nil
}
