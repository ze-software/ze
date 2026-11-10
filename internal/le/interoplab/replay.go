// Design: docs/architecture/testing/interop.md -- checker recording and replay
// Related: record.go -- writes the recordings this file replays.
// Related: catalog.go -- the registered checker a replay runs.
package interoplab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ReplayResult is the lab's verdict on a recorded scenario beside the verdict
// its checker reaches when replayed from the recording. A replay that agrees
// with the lab has Lab equal to Replay.
type ReplayResult struct {
	Lab    Verdict `json:"lab"`
	Replay Verdict `json:"replay"`
}

// ReplayRefusal is a replay that could not answer the checker from the
// recording: the recorded lab never saw what the checker now asks, so any
// answer would be a guess, and only a lab run can judge the checker.
type ReplayRefusal struct {
	Recording string
	// Position is the 1-based number of the call refused; 0 for a refusal of
	// the recording as a whole.
	Position int
	// Expected is the call the recording holds at Position; nil when the
	// recording is exhausted or refused as a whole.
	Expected *RecordedCall
	// Actual is the call the checker made at Position; nil when the checker
	// stopped before it.
	Actual *RecordedCall
	Reason string
	// Rerun is the command that runs the scenario in the lab; empty when the
	// recording names no registered lab.
	Rerun string
}

func (r *ReplayRefusal) Error() string {
	var text textbuf.Buffer
	text.Str("replay of ").Str(r.Recording)
	if r.Position > 0 {
		text.Str(" refused call ").Int(int64(r.Position))
	} else {
		text.Str(" refused")
	}
	text.Str(": ").Str(r.Reason)
	text.Str("; expected ")
	describeCall(&text, r.Expected)
	text.Str(", actual ")
	describeCall(&text, r.Actual)
	text.Str(". The recording cannot answer this checker, so the checker must be judged by a lab run")
	if r.Rerun != "" {
		text.Str(": ").Str(r.Rerun)
	}
	text.Str(", and recorded again from that run")
	return text.String()
}

// describeCall writes call as `<method> <peer> [argv...]` and its other
// parameters, or `none` for a nil call.
func describeCall(text *textbuf.Buffer, call *RecordedCall) {
	if call == nil {
		text.Str("none")
		return
	}
	text.Str(string(call.Method)).Byte(' ').Str(call.Peer).Str(" [").Join(call.Argv, " ").Byte(']')
	if len(call.Environment) != 0 {
		text.Str(" environment ").Join(call.Environment, ",")
	}
	if call.Lines != 0 {
		text.Str(" lines ").Int(int64(call.Lines))
	}
	if call.Signal != "" {
		text.Str(" signal ").Str(call.Signal)
	}
	if call.TimeoutSeconds != 0 {
		text.Str(" timeout ").Int(int64(call.TimeoutSeconds)).Byte('s')
	}
}

// recording is one parsed recording file.
type recording struct {
	path    string
	header  recordingHeader
	calls   []recordingLine
	verdict Verdict
}

// ReplayScenario replays the recording at path through the checker its lab
// registers for the recorded scenario (Catalog.Scenarios under checkout), and
// answers the lab's verdict beside the replayed one. A checker call the
// recording does not hold at its position, a call past the recording's end,
// a truncated or non-text answer, a recording of another version, and a
// checker that stops before the recording's last call each answer a
// *ReplayRefusal and no result: the checker's verdict then rests on data the
// lab never produced.
func ReplayScenario(ctx context.Context, checkout, path string) (ReplayResult, error) {
	recorded, err := openRecording(path)
	if err != nil {
		return ReplayResult{}, err
	}
	catalog, held := CatalogNamed(recorded.header.Lab)
	if !held {
		return ReplayResult{}, &ReplayRefusal{Recording: path,
			Reason: "the recording names lab " + recorded.header.Lab + ", which no registered catalog declares"}
	}
	rerun := rerunScenario(catalog, recorded.header.Scenario)
	sources, err := catalog.Scenarios(checkout)
	if err != nil {
		return ReplayResult{}, err
	}
	index := slices.IndexFunc(sources, func(source ScenarioSource) bool { return source.Name == recorded.header.Scenario })
	if index < 0 {
		return ReplayResult{}, &ReplayRefusal{Recording: path, Rerun: rerun,
			Reason: "lab " + catalog.Suite + " declares no scenario " + recorded.header.Scenario}
	}
	source := sources[index]
	lab := &replayLab{recording: recorded, rerun: rerun}
	check := &CheckContext{Source: source, Network: recorded.header.Network, Lab: lab, Checkout: checkout,
		Links: recorded.header.Links, Memberships: recorded.header.Memberships}
	checkErr := source.Checker(ctx, check)
	if refusal := lab.finish(); refusal != nil {
		return ReplayResult{}, refusal
	}
	result := ReplayResult{Lab: recorded.verdict, Replay: Verdict{Passed: checkErr == nil}}
	if checkErr != nil {
		result.Replay.Error = checkErr.Error()
	}
	return result, nil
}

// rerunScenario answers the command that runs one scenario of catalog's lab.
func rerunScenario(catalog Catalog, scenario string) string {
	var command textbuf.Buffer
	return command.Str(catalog.SelectorVariable).Byte('=').Str(scenario).Str(" ./le ").
		Str(strings.Replace(catalog.Action, "/", " ", 1)).String()
}

// openRecording parses the recording at path: a header of this version,
// calls, and the lab's verdict as the last line.
func openRecording(path string) (*recording, error) {
	file, err := os.Open(path) //nolint:gosec // a recording a test names, inside the checkout or the session scratch
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // a read-only file; every read error is already returned
	decoder := json.NewDecoder(file)
	var first json.RawMessage
	if err := decoder.Decode(&first); err != nil {
		return nil, &ReplayRefusal{Recording: path, Reason: "the recording has no readable header line: " + err.Error()}
	}
	var version struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(first, &version); err != nil {
		return nil, &ReplayRefusal{Recording: path, Reason: "the header line is not a recording header: " + err.Error()}
	}
	if version.Version != recordingVersion {
		return nil, &ReplayRefusal{Recording: path, Reason: "the recording is version " + strconv.Itoa(version.Version) +
			" and this replay reads version " + strconv.Itoa(recordingVersion)}
	}
	recorded := &recording{path: path}
	if err := strictUnmarshal(first, &recorded.header); err != nil {
		return nil, &ReplayRefusal{Recording: path, Reason: "the header line does not parse: " + err.Error()}
	}
	decoder.DisallowUnknownFields()
	for {
		var line recordingLine
		err := decoder.Decode(&line)
		if errors.Is(err, io.EOF) {
			return nil, &ReplayRefusal{Recording: path, Position: len(recorded.calls) + 1,
				Reason: "the recording ends with no verdict line, so the lab run that wrote it did not finish"}
		}
		if err != nil {
			return nil, &ReplayRefusal{Recording: path, Position: len(recorded.calls) + 1, Reason: "the line does not parse: " + err.Error()}
		}
		if line.Verdict != nil {
			recorded.verdict = *line.Verdict
			break
		}
		if (line.Error == "") != (line.ErrorKind == "") {
			return nil, &ReplayRefusal{Recording: path, Position: len(recorded.calls) + 1, Expected: &line.RecordedCall,
				Reason: "the line carries an error text and an error kind only together"}
		}
		recorded.calls = append(recorded.calls, line)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, &ReplayRefusal{Recording: path, Position: len(recorded.calls) + 1,
			Reason: "the recording carries lines after its verdict"}
	}
	return recorded, nil
}

func strictUnmarshal(data []byte, into any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(into)
}

// replayLab is a CheckerLab answering each call from a recording in order.
// The first refusal sticks: every later call is refused too, because a
// checker that swallowed one refusal (a poll counting it as a transient
// failure) has already left the recorded path. Safe for concurrent use.
type replayLab struct {
	recording *recording
	rerun     string
	mu        sync.Mutex
	next      int
	refusal   *ReplayRefusal
}

var _ CheckerLab = (*replayLab)(nil)

// answer matches call against the recording's next call and answers its
// recorded answer, or refuses.
func (l *replayLab) answer(call *RecordedCall) (*RecordedAnswer, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.refusal != nil {
		return nil, l.refusal
	}
	position := l.next + 1
	if l.next >= len(l.recording.calls) {
		return nil, l.refuse(position, nil, call, "the recording holds "+strconv.Itoa(len(l.recording.calls))+" calls and the checker made another")
	}
	recorded := &l.recording.calls[l.next]
	if !sameCall(&recorded.RecordedCall, call) {
		return nil, l.refuse(position, &recorded.RecordedCall, call, "the checker made a call the recording does not hold at this position")
	}
	if recorded.Truncated {
		return nil, l.refuse(position, &recorded.RecordedCall, call,
			"the recorded answer was cut at "+strconv.Itoa(l.recording.header.AnswerOctetsMax)+" octets")
	}
	if recorded.NotText {
		return nil, l.refuse(position, &recorded.RecordedCall, call, "the recorded answer is not UTF-8 text, so its octets were not kept")
	}
	l.next++
	return &recorded.RecordedAnswer, nil
}

func (l *replayLab) refuse(position int, expected, actual *RecordedCall, reason string) *ReplayRefusal {
	l.refusal = &ReplayRefusal{Recording: l.recording.path, Position: position, Expected: expected,
		Actual: actual, Reason: reason, Rerun: l.rerun}
	return l.refusal
}

// finish answers the replay's refusal, if any, once the checker returned: the
// first refused call, or the first recorded call the checker never made.
func (l *replayLab) finish() *ReplayRefusal {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.refusal != nil {
		return l.refusal
	}
	if l.next < len(l.recording.calls) {
		return l.refuse(l.next+1, &l.recording.calls[l.next].RecordedCall, nil,
			"the checker returned after "+strconv.Itoa(l.next)+" of "+strconv.Itoa(len(l.recording.calls))+" recorded calls")
	}
	return nil
}

// sameCall reports whether actual is the recorded call: every parameter the
// checker passed, environment names included.
func sameCall(recorded, actual *RecordedCall) bool {
	if recorded.Method != actual.Method {
		return false
	}
	if recorded.Peer != actual.Peer {
		return false
	}
	if !slices.Equal(recorded.Argv, actual.Argv) {
		return false
	}
	if !slices.Equal(recorded.Environment, actual.Environment) {
		return false
	}
	if recorded.Lines != actual.Lines {
		return false
	}
	if recorded.Signal != actual.Signal {
		return false
	}
	return recorded.TimeoutSeconds == actual.TimeoutSeconds
}

// replayedError is a recorded error: its text as the lab wrote it, and the
// typed cause a checker tests with errors.Is or errors.As.
type replayedError struct {
	text  string
	cause error
}

func (e *replayedError) Error() string { return e.text }
func (e *replayedError) Unwrap() error { return e.cause }

// recordedError rebuilds the error answer carries, nil when it carries none.
// The kind is read from a file, so the set is open: an unknown kind refuses.
func (l *replayLab) recordedError(answer *RecordedAnswer, call *RecordedCall) error {
	switch answer.ErrorKind {
	case "":
		return nil
	case errKindOther:
		return &replayedError{text: answer.Error}
	case errKindUnknownPeer:
		return &replayedError{text: answer.Error, cause: ErrUnknownPeer}
	case errKindEmptyAnswer:
		return &replayedError{text: answer.Error, cause: &EmptyAnswerError{Peer: call.Peer}}
	case errKindDeadline:
		return &replayedError{text: answer.Error, cause: context.DeadlineExceeded}
	case errKindCanceled:
		return &replayedError{text: answer.Error, cause: context.Canceled}
	default:
		// The recording is outside input, so an unknown kind is refused.
		l.mu.Lock()
		defer l.mu.Unlock()
		return l.refuse(l.next, call, call, "the recorded error kind "+string(answer.ErrorKind)+" is not one this replay knows")
	}
}

// reply answers call's recorded answer and error, or the refusal.
func (l *replayLab) reply(call *RecordedCall) (*RecordedAnswer, error) {
	answer, err := l.answer(call)
	if err != nil {
		return &RecordedAnswer{}, err
	}
	return answer, l.recordedError(answer, call)
}

// Exec answers the recorded streams.
func (l *replayLab) Exec(_ context.Context, peer string, argv []string, environ []EnvironmentVariable) (CommandResult, error) {
	answer, err := l.reply(&RecordedCall{Method: methodExec, Peer: peer, Argv: argv, Environment: environmentNames(environ)})
	return CommandResult{Stdout: answer.Stdout, Stderr: answer.Stderr, ExitCode: answer.ExitCode}, err
}

// ExecDetached answers the recorded error.
func (l *replayLab) ExecDetached(_ context.Context, peer string, argv []string, environ []EnvironmentVariable) error {
	_, err := l.reply(&RecordedCall{Method: methodExecDetached, Peer: peer, Argv: argv, Environment: environmentNames(environ)})
	return err
}

// Query answers the recorded answer.
func (l *replayLab) Query(_ context.Context, peer string, argv []string, environ []EnvironmentVariable) (string, error) {
	answer, err := l.reply(&RecordedCall{Method: methodQuery, Peer: peer, Argv: argv, Environment: environmentNames(environ)})
	return answer.Stdout, err
}

// Logs answers the recorded tail.
func (l *replayLab) Logs(_ context.Context, peer string, lines int) (LogResult, error) {
	answer, err := l.reply(&RecordedCall{Method: methodLogs, Peer: peer, Lines: lines})
	return LogResult{Text: answer.Stdout, Available: answer.Available}, err
}

// PeerPID answers the recorded pid.
func (l *replayLab) PeerPID(_ context.Context, peer string) (int, error) {
	answer, err := l.reply(&RecordedCall{Method: methodPeerPID, Peer: peer})
	return answer.PID, err
}

// PeerImage answers the recorded image ID.
func (l *replayLab) PeerImage(_ context.Context, peer string) (string, error) {
	answer, err := l.reply(&RecordedCall{Method: methodPeerImage, Peer: peer})
	return answer.Stdout, err
}

// Signal answers the recorded error.
func (l *replayLab) Signal(_ context.Context, peer, signal string) error {
	_, err := l.reply(&RecordedCall{Method: methodSignal, Peer: peer, Signal: signal})
	return err
}

// Pause answers the recorded error.
func (l *replayLab) Pause(_ context.Context, peer string) error {
	_, err := l.reply(&RecordedCall{Method: methodPause, Peer: peer})
	return err
}

// Unpause answers the recorded error.
func (l *replayLab) Unpause(_ context.Context, peer string) error {
	_, err := l.reply(&RecordedCall{Method: methodUnpause, Peer: peer})
	return err
}

// Start answers the recorded error.
func (l *replayLab) Start(_ context.Context, peer string) error {
	_, err := l.reply(&RecordedCall{Method: methodStart, Peer: peer})
	return err
}

// Stop answers the recorded error.
func (l *replayLab) Stop(_ context.Context, peer string, timeoutSeconds int) error {
	_, err := l.reply(&RecordedCall{Method: methodStop, Peer: peer, TimeoutSeconds: timeoutSeconds})
	return err
}
