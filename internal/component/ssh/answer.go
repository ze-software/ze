// Design: docs/architecture/api/ipc_protocol.md -- the answer grammar
// Overview: ssh.go -- execMiddleware, the exec channel this frames an answer on
//
// answer.go carries the SSH exec channel's half of the answer protocol. The
// rendering itself belongs to command.RenderRecords.
//
// The channel has two streams and the answer uses both, because the daemon
// renders. It has to: the format an operator sees comes from `ze.cli.format`,
// which lives in the daemon's configuration, and four of the six renderings run
// to several lines. Rendered text can therefore not travel inside an item=,
// which is one line by construction (AppendAnswerItem, pkg/plugin/rpc).
//
//	stdout  the rendering, written as the records arrive. For `ssh <host>
//	        <command>` this is the whole answer and it is what it always was.
//	stderr  the head, the terminator and the not-understood line, for every
//	        session. One answer has one encoding, so nothing is declared and
//	        nothing is negotiated.
//
// Truncation is then a missing terminator, which is what AC-9 asks for: a
// connection that dies part way leaves the records that arrived and no line
// saying how many there should have been.

package ssh

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"charm.land/ssh"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// answerFrameCapacity is the initial byte capacity of the one line buffer a
// frame reuses. A head naming many columns grows it once, and the terminator
// then reuses the grown slice.
const answerFrameCapacity = 256

// answerFrame writes the frame of one exec-channel answer.
//
// One answer owns the channel, so every line is written with rpc.AnswerNoID and
// carries no #<len>:<id>. Everything after the verb is the grammar the plugin
// connection uses, so a reader parses one tail whichever channel it came from.
type answerFrame struct {
	w   io.Writer
	buf []byte
}

// newAnswerFrame returns the frame for this session. Every session gets one:
// the frame is what says how many records the answer holds and whether it
// finished, and a client that cannot ask for it still needs to be told.
func newAnswerFrame(sess ssh.Session) *answerFrame {
	return &answerFrame{w: sess.Stderr(), buf: make([]byte, 0, answerFrameCapacity)}
}

// head writes the line that opens the answer: what the daemon knows about the
// command, and which shape the rendering on stdout was produced from.
//
// It is written after the body rather than before it, because the type is read
// from the walk and the walk is what produces the body. The two streams are
// read independently, so a client cannot order stderr against stdout anyway;
// what it needs from the head is the status and the shape, and both are true
// whenever it arrives.
func (f *answerFrame) head(status, answerType, key string, fields []string) error {
	encoded, err := marshalAnswerFields(fields)
	if err != nil {
		return err
	}
	return f.write(rpc.AppendAnswerHead(f.buf[:0], rpc.AnswerNoID, status, answerType, key, encoded))
}

// terminator writes the line that ends the answer. Its absence is what states
// truncation, so nothing else may be written after it.
func (f *answerFrame) terminator(count, faults uint64, message string) error {
	return f.write(rpc.AppendAnswerTerminator(f.buf[:0], rpc.AnswerNoID, count, faults, message))
}

// notUnderstood writes the whole answer to a command text that names no
// command: the verb says the conversation was valid and the command was not, so
// a client can offer completion here rather than reporting an operational
// failure the daemon never attempted.
func (f *answerFrame) notUnderstood(message string) error {
	return f.write(rpc.AppendAnswerNotUnderstood(f.buf[:0], rpc.AnswerNoID, "", message))
}

// write frames one line with the newline that ends it and writes it in one
// call, so a reader never takes delivery of half a line. The framed slice is
// kept for the next line of this answer.
func (f *answerFrame) write(line []byte) error {
	line = append(line, '\n')
	f.buf = line
	_, err := f.w.Write(line)
	return err
}

// writeExecAnswer writes one exec-channel answer: the rendering on stdout, and
// the frame on stderr.
//
// input is the operator's whole text, pipe chain included, because the chain
// decides the rendering. formatOutput renders a payload the handler built
// before the answer opened; a payload that is a row generator is rendered by
// command.RenderRecords instead, which pulls the rows as it writes them.
func writeExecAnswer(sess ssh.Session, frame *answerFrame, input string, formatOutput func(string) string, result *plugin.RenderedResponse) error {
	if result == nil {
		return writeExecDocument(sess, frame, formatOutput, "")
	}
	if records, generated := plugin.RecordRows(result.Response); generated {
		return writeExecRecords(sess, frame, input, records)
	}
	return writeExecDocument(sess, frame, formatOutput, result.Output)
}

// writeExecRecords renders a row generator to the session and frames what it
// turned out to be.
//
// The head names the envelope only for a streamed answer. A bounded answer is
// one document and the document carries the envelope already, so stating it
// twice would give a reader two facts that can disagree. The columns are named
// on the same condition and for the same reason: they are how a positional row
// is read, and a document has no positional rows in it.
func writeExecRecords(sess ssh.Session, frame *answerFrame, input string, records plugin.Records) error {
	// An exec channel holds no per-session `set cli format` override, so the
	// session format is empty and the configured default applies.
	answer, err := command.RenderRecords(sess, input, "", records.Key, records.Fields, records.Rows)
	if err != nil {
		return err
	}

	key := records.Key
	var fields []string
	switch answer.Type {
	case rpc.AnswerTypeJSON:
		key = ""
	case rpc.AnswerTypeStream:
		fields = records.Fields
	}
	if headErr := frame.head(plugin.StatusDone, answer.Type, key, fields); headErr != nil {
		return headErr
	}
	return frame.terminator(answer.Count, answer.Faults, "")
}

// writeExecDocument renders a payload the handler built whole and frames it as
// the one document it is.
//
// An empty output is a command that reported nothing, and it writes no body:
// nothing is not the same answer as an empty collection, and the terminator
// says which by carrying count=0. The client is what turns that into the OK an
// operator reads, because only the client knows whether the chain named a
// format.
func writeExecDocument(sess ssh.Session, frame *answerFrame, formatOutput func(string) string, output string) error {
	if err := frame.head(plugin.StatusDone, rpc.AnswerTypeJSON, "", nil); err != nil {
		return err
	}

	var count uint64
	if output != "" {
		count = 1
	}
	// The formatter can end its rendering with a newline; Fprintln adds the
	// only one the caller needs.
	if rendered := strings.TrimRight(formatOutput(output), "\n"); rendered != "" {
		if _, err := fmt.Fprintln(sess, rendered); err != nil {
			return err
		}
	}
	return frame.terminator(count, 0, "")
}

// writeExecFailure reports a command that did not produce an answer.
//
// The frame reads the two failures apart: a command text that names no command
// earns the error verb, which says the conversation was valid and re-sending is
// pointless, and a command that ran and failed earns a head stating
// status=error with the operational text on its terminator.
//
// The operator's plain line is written first and the frame follows it. Both
// audiences read one stream, and the terminator ends the answer, so nothing may
// come after it. A client reading the frame keeps the plain line as the daemon
// talking to a person rather than to it (readAnswerFrame,
// internal/core/ssh/client/answer.go), and the message it reports comes from
// the frame, so the two cannot disagree.
//
// The frame writes are best-effort: the session is ending with exit code 1
// whatever they do, and a client whose terminator never arrives reads the
// answer as truncated, which is the right verdict for one that never finished.
func writeExecFailure(sess ssh.Session, frame *answerFrame, err error) {
	writeExecError(sess, err)
	if errors.Is(err, pluginserver.ErrUnknownCommand) {
		frame.notUnderstood(err.Error()) //nolint:errcheck // the session is ending; a lost line reads as truncated
		return
	}
	frame.head(plugin.StatusError, rpc.AnswerTypeJSON, "", nil) //nolint:errcheck // as above
	frame.terminator(0, 0, err.Error())                         //nolint:errcheck // as above
}

// writeExecError writes the plain-text failure an operator reads, in the shape
// `ssh <host> <command>` has always answered with. sshclient.trimErrorPrefix is
// its reader.
func writeExecError(sess ssh.Session, err error) {
	var tb textbuf.Buffer
	io.WriteString(sess.Stderr(), tb.Str("error: ").Err(err).Byte('\n').String()) //nolint:errcheck // best-effort
}

// marshalAnswerFields encodes the column names a streamed answer's head
// carries, and encodes nothing when the answer declares none. It runs before
// any line is written, so a schema that cannot be encoded is named instead of
// half a frame reaching the client.
func marshalAnswerFields(fields []string) (json.RawMessage, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("marshal answer fields: %w", err)
	}
	return encoded, nil
}
