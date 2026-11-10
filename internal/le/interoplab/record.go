// Design: docs/architecture/testing/interop.md -- checker recording and replay
// Related: replay.go -- answers a checker from a recording this file wrote.
// Related: lab.go -- runScenario wraps the checker's Lab in a recorder.
package interoplab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// RecordingDirectory is the directory, under the session scratch, that holds
// one subdirectory of checker recordings per lab.
const RecordingDirectory = "interop-recordings"

// recordingVersion is the format of a recording. replay.go refuses any other
// version, so a change to a line's fields MUST raise it.
const recordingVersion = 1

// recordingAnswerOctetsMax bounds each answer field a recording keeps: a
// relay dump or a log tail can be megabytes, and a full BGP run records 234
// scenarios. An answer longer than this keeps its first octets and is marked
// Truncated, and replay refuses it, because a checker judged on part of an
// answer may change its verdict on the octets it never saw.
const recordingAnswerOctetsMax = 1 << 20

// labMethod names one CheckerLab method in a recording.
type labMethod string

const (
	methodExec         labMethod = "exec"
	methodExecDetached labMethod = "exec-detached"
	methodQuery        labMethod = "query"
	methodLogs         labMethod = "logs"
	methodPeerPID      labMethod = "peer-pid"
	methodSignal       labMethod = "signal"
	methodPause        labMethod = "pause"
	methodUnpause      labMethod = "unpause"
	methodStart        labMethod = "start"
	methodStop         labMethod = "stop"
)

// RecordedCall is what a checker asked of the lab: everything replay matches
// a call on. Environment holds the variable NAMES only. A value can be a PSK
// or a RADIUS secret, and a recording is copied into testdata/, so no value
// is ever written.
type RecordedCall struct {
	Method      labMethod `json:"method,omitempty"`
	Peer        string    `json:"peer,omitempty"`
	Argv        []string  `json:"argv,omitempty"`
	Environment []string  `json:"environment,omitempty"`
	// Lines is the log tail a logs call asked for.
	Lines int `json:"lines,omitempty"`
	// Signal is the signal a signal call sent.
	Signal string `json:"signal,omitempty"`
	// TimeoutSeconds is the grace period a stop call gave.
	TimeoutSeconds int `json:"timeout-seconds,omitempty"`
}

// RecordedAnswer is what the lab answered one call.
type RecordedAnswer struct {
	// Stdout is an exec's stdout, a query's answer, or a logs call's text.
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit-code,omitempty"`
	// Available is a logs call's LogResult.Available.
	Available bool `json:"available,omitempty"`
	PID       int  `json:"pid,omitempty"`
	// Truncated marks an answer field cut at recordingAnswerOctetsMax.
	Truncated bool `json:"truncated,omitempty"`
	// NotText marks an answer field that is not UTF-8: JSON cannot carry its
	// octets unchanged, so replay could not hand the checker what it read.
	NotText bool `json:"not-text,omitempty"`
	// Error is the text of the error the call returned, and ErrorKind the
	// typed error a checker can test for (errorKindOf).
	Error     string          `json:"error,omitempty"`
	ErrorKind recordedErrKind `json:"error-kind,omitempty"`
}

// recordedErrKind names the typed error behind a recorded error, so replay
// hands back an error errors.Is and errors.As still recognize.
type recordedErrKind string

const (
	errKindUnknownPeer recordedErrKind = "unknown-peer"
	errKindEmptyAnswer recordedErrKind = "empty-answer"
	errKindDeadline    recordedErrKind = "deadline"
	errKindCanceled    recordedErrKind = "canceled"
	errKindOther       recordedErrKind = "other"
)

// recordingHeader is a recording's first line: its version and everything a
// CheckContext carries besides the Lab, so replay hands the checker the
// network, links and memberships the lab run handed it.
type recordingHeader struct {
	Version         int                         `json:"version"`
	Lab             string                      `json:"lab"`
	Scenario        string                      `json:"scenario"`
	Network         Network                     `json:"network"`
	Links           []Link                      `json:"links,omitempty"`
	Memberships     map[string][]LinkMembership `json:"memberships,omitempty"`
	AnswerOctetsMax int                         `json:"answer-octets-max"`
}

// recordingLine is every line after the header: one call and its answer, or,
// as the last line, the lab's verdict on the scenario.
type recordingLine struct {
	RecordedCall
	RecordedAnswer
	Verdict *Verdict `json:"verdict,omitempty"`
}

// Verdict is a checker's answer on one scenario.
type Verdict struct {
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
}

// recordingPlan is where one Suite.Run writes its checkers' recordings.
type recordingPlan struct {
	// directory is <session scratch>/interop-recordings/<lab>.
	directory string
	lab       string
	runID     string
	// keepPasses keeps a passing scenario's recording: a selector run keeps
	// all, a full run keeps only the failed ones (spec R-1, scratch size).
	keepPasses bool
	// keepAllReason, when not nil, is why the catalog could not be counted:
	// recordChecker then keeps every recording whatever keepPasses says, and
	// Suite.Run prints it once, so a full run that kept its passes says why.
	keepAllReason error
}

// recordingPlan resolves where this run records. Recordings sits under
// Recordings when the caller set it, else under Checkout's session scratch;
// the lab is the registered catalog whose scenario root holds the first
// scenario. A run is a selector run when it holds fewer scenarios than its
// catalog answers.
func (s Suite) recordingPlan() (*recordingPlan, error) {
	catalog, found := s.catalog()
	if !found {
		return nil, errors.New("no registered catalog declares this suite's scenario directory, so the recordings have no lab to file under")
	}
	root := s.Recordings
	if root == "" {
		if s.Checkout == "" {
			return nil, errors.New("the suite declares no checkout, so there is no session scratch to record under")
		}
		paths, err := lepath.ResolveSession(s.Checkout, true)
		if err != nil {
			return nil, err
		}
		root = filepath.Join(s.Checkout, paths.Scratch, RecordingDirectory)
	}
	plan := &recordingPlan{directory: filepath.Join(root, catalog.Suite), lab: catalog.Suite, runID: s.Docker.owner.run}
	if plan.runID == "" {
		return nil, errors.New("the Docker client carries no run id to name the recordings by")
	}
	// A catalog that cannot list its scenarios keeps every recording: that
	// loses nothing, while reading the run as a full one would delete the
	// passing recordings a selector run asked for. The listing error is not
	// this function's failure; it travels on the plan as keepAllReason, and
	// Suite.Run prints it.
	all, listErr := catalog.Scenarios(s.Checkout)
	plan.keepAllReason = listErr
	plan.keepPasses = len(s.Scenarios) < len(all)
	return plan, nil
}

// path answers the recording file of one scenario.
func (p *recordingPlan) path(scenario string) string {
	return filepath.Join(p.directory, scenario+"-"+p.runID+".jsonl")
}

// checkerRecorder is a CheckerLab that forwards every call to lab and writes
// it, in call order, as one jsonl line to out. The first write error stops
// every later write and is kept in err. Safe for concurrent use: a mutex
// orders each call's line, so concurrent calls are written in the order they
// finished, and replay refuses a checker that issues them in another order.
type checkerRecorder struct {
	lab CheckerLab
	mu  sync.Mutex
	out *json.Encoder
	err error
}

var _ CheckerLab = (*checkerRecorder)(nil)

// newCheckerRecorder writes header to out and answers the recorder. The
// caller MUST call finish with the verdict after the checker returns.
func newCheckerRecorder(lab CheckerLab, out io.Writer, header *recordingHeader) *checkerRecorder {
	recorder := &checkerRecorder{lab: lab, out: json.NewEncoder(out)}
	recorder.out.SetEscapeHTML(false)
	header.Version = recordingVersion
	header.AnswerOctetsMax = recordingAnswerOctetsMax
	recorder.err = recorder.out.Encode(header)
	return recorder
}

// finish writes the lab's verdict as the last line and answers the first
// write error. It MUST be called once, after the checker returned.
func (r *checkerRecorder) finish(verdict Verdict) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.err = r.out.Encode(recordingLine{Verdict: &verdict})
	return r.err
}

func (r *checkerRecorder) write(call *RecordedCall, answer *RecordedAnswer, err error) {
	if err != nil {
		answer.Error = err.Error()
		answer.ErrorKind = errorKindOf(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return
	}
	r.err = r.out.Encode(recordingLine{RecordedCall: *call, RecordedAnswer: *answer})
}

// errorKindOf names the typed error a checker can test err for.
func errorKindOf(err error) recordedErrKind {
	if _, empty := errors.AsType[*EmptyAnswerError](err); empty {
		return errKindEmptyAnswer
	}
	if errors.Is(err, ErrUnknownPeer) {
		return errKindUnknownPeer
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errKindDeadline
	}
	if errors.Is(err, context.Canceled) {
		return errKindCanceled
	}
	return errKindOther
}

// bound answers text cut at recordingAnswerOctetsMax and marks the answer
// when it was cut or is not UTF-8.
func (a *RecordedAnswer) bound(text string) string {
	if !utf8.ValidString(text) {
		a.NotText = true
	}
	if len(text) <= recordingAnswerOctetsMax {
		return text
	}
	a.Truncated = true
	return text[:recordingAnswerOctetsMax]
}

// environmentNames answers the names of environ, never a value.
func environmentNames(environ []EnvironmentVariable) []string {
	if len(environ) == 0 {
		return nil
	}
	names := make([]string, 0, len(environ))
	for _, variable := range environ {
		names = append(names, variable.Name)
	}
	return names
}

// Exec forwards to the lab and records the call and its streams.
func (r *checkerRecorder) Exec(ctx context.Context, peer string, argv []string, environ []EnvironmentVariable) (CommandResult, error) {
	result, err := r.lab.Exec(ctx, peer, argv, environ)
	call := RecordedCall{Method: methodExec, Peer: peer, Argv: argv, Environment: environmentNames(environ)}
	var answer RecordedAnswer
	answer.Stdout = answer.bound(result.Stdout)
	answer.Stderr = answer.bound(result.Stderr)
	answer.ExitCode = result.ExitCode
	r.write(&call, &answer, err)
	return result, err
}

// ExecDetached forwards to the lab and records the call.
func (r *checkerRecorder) ExecDetached(ctx context.Context, peer string, argv []string, environ []EnvironmentVariable) error {
	err := r.lab.ExecDetached(ctx, peer, argv, environ)
	call := RecordedCall{Method: methodExecDetached, Peer: peer, Argv: argv, Environment: environmentNames(environ)}
	r.write(&call, &RecordedAnswer{}, err)
	return err
}

// Query forwards to the lab and records the call and its answer.
func (r *checkerRecorder) Query(ctx context.Context, peer string, argv []string, environ []EnvironmentVariable) (string, error) {
	text, err := r.lab.Query(ctx, peer, argv, environ)
	call := RecordedCall{Method: methodQuery, Peer: peer, Argv: argv, Environment: environmentNames(environ)}
	var answer RecordedAnswer
	answer.Stdout = answer.bound(text)
	r.write(&call, &answer, err)
	return text, err
}

// Logs forwards to the lab and records the call and the tail it read.
func (r *checkerRecorder) Logs(ctx context.Context, peer string, lines int) (LogResult, error) {
	result, err := r.lab.Logs(ctx, peer, lines)
	call := RecordedCall{Method: methodLogs, Peer: peer, Lines: lines}
	var answer RecordedAnswer
	answer.Stdout = answer.bound(result.Text)
	answer.Available = result.Available
	r.write(&call, &answer, err)
	return result, err
}

// PeerPID forwards to the lab and records the call and the pid.
func (r *checkerRecorder) PeerPID(ctx context.Context, peer string) (int, error) {
	pid, err := r.lab.PeerPID(ctx, peer)
	r.write(&RecordedCall{Method: methodPeerPID, Peer: peer}, &RecordedAnswer{PID: pid}, err)
	return pid, err
}

// Signal forwards to the lab and records the call.
func (r *checkerRecorder) Signal(ctx context.Context, peer, signal string) error {
	err := r.lab.Signal(ctx, peer, signal)
	r.write(&RecordedCall{Method: methodSignal, Peer: peer, Signal: signal}, &RecordedAnswer{}, err)
	return err
}

// Pause forwards to the lab and records the call.
func (r *checkerRecorder) Pause(ctx context.Context, peer string) error {
	err := r.lab.Pause(ctx, peer)
	r.write(&RecordedCall{Method: methodPause, Peer: peer}, &RecordedAnswer{}, err)
	return err
}

// Unpause forwards to the lab and records the call.
func (r *checkerRecorder) Unpause(ctx context.Context, peer string) error {
	err := r.lab.Unpause(ctx, peer)
	r.write(&RecordedCall{Method: methodUnpause, Peer: peer}, &RecordedAnswer{}, err)
	return err
}

// Start forwards to the lab and records the call.
func (r *checkerRecorder) Start(ctx context.Context, peer string) error {
	err := r.lab.Start(ctx, peer)
	r.write(&RecordedCall{Method: methodStart, Peer: peer}, &RecordedAnswer{}, err)
	return err
}

// Stop forwards to the lab and records the call.
func (r *checkerRecorder) Stop(ctx context.Context, peer string, timeoutSeconds int) error {
	err := r.lab.Stop(ctx, peer, timeoutSeconds)
	r.write(&RecordedCall{Method: methodStop, Peer: peer, TimeoutSeconds: timeoutSeconds}, &RecordedAnswer{}, err)
	return err
}

// recordChecker runs checker on check with its Lab wrapped in a recorder
// writing to plan's file for the scenario, and answers the checker's error
// and one line, with no newline, that names the recording, or is empty when
// a passing scenario's recording was not kept. A failure to open or
// write the recording never changes the verdict: it is named in the
// diagnostic, so a failed scenario says why it has no recording.
func (p *recordingPlan) recordChecker(ctx context.Context, checker Checker, check *CheckContext) (diagnostic string, checkErr error) {
	path := p.path(check.Source.Name)
	var line textbuf.Buffer
	if err := os.MkdirAll(p.directory, 0o750); err != nil {
		return line.Str("checker recording not written: ").Err(err).String(), checker(ctx, check)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // the path is the session scratch joined with the scenario name and the run id
	if err != nil {
		return line.Str("checker recording not written: ").Err(err).String(), checker(ctx, check)
	}
	recorder := newCheckerRecorder(check.Lab, file, &recordingHeader{Lab: p.lab, Scenario: check.Source.Name,
		Network: check.Network, Links: check.Links, Memberships: check.Memberships})
	check.Lab = recorder
	checkErr = checker(ctx, check)
	verdict := Verdict{Passed: checkErr == nil}
	if checkErr != nil {
		verdict.Error = checkErr.Error()
	}
	writeErr := errors.Join(recorder.finish(verdict), file.Close())
	if writeErr != nil {
		return line.Str("checker recording ").Str(path).Str(" is incomplete: ").Err(writeErr).String(), checkErr
	}
	if checkErr == nil && !p.keepPasses && p.keepAllReason == nil {
		if err := os.Remove(path); err != nil {
			return line.Str("checker recording ").Str(path).Str(" of a passing scenario was not removed: ").Err(err).String(), checkErr
		}
		return "", checkErr
	}
	return line.Str("checker recording: ").Str(path).String(), checkErr
}
