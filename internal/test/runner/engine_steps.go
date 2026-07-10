// Design: docs/architecture/testing/ci-format.md -- engine-step directives
//
// Engine steps drive a live test daemon through CLI dispatch, first-class in
// .ci instead of an embedded Python observer:
//
//	command=<cli command text>
//	stream=<monitor command text>
//	expect=output:contains=<text>[:timeout=<dur>]
//	expect=event:namespace=<ns>:name=<name>[:timeout=<dur>]
//	expect=stream:contains=<text>[:timeout=<dur>]
//
// Execution model: the runner serializes the parsed steps to
// engine-steps.json in the test's tmpfs directory, and the .ci declares the
// executor as a regular external plugin:
//
//	plugin {
//		external engine-steps {
//			run "ze-test engine-steps ./engine-steps.json"
//			encoder json
//		}
//	}
//
// The spawned executor (internal/test/cli/cmd_engine_steps.go) runs the steps
// from OnAllPluginsReady -- the engine's sanctioned point for cross-plugin
// dispatch -- and reports failures through the ZE-OBSERVER-FAIL sentinel the
// runner already gates on. This deliberately reuses the connect-back plugin
// transport: the hub acceptor drops unsolicited connections
// (internal/component/plugin/ipc/tls.go handleConn), so a runner-side dial
// is not a supported path.
//
// command=/stream= lines keep their full raw text (colons included).
// expect=output re-dispatches the most recent command until its result text
// contains the needle or the timeout expires; expect=event/expect=stream
// match delivered events (event subscriptions are established up front so a
// trigger fired by an earlier step cannot race its expectation).

package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// EngineStepKind identifies one engine-step directive.
type EngineStepKind uint8

// Engine step kinds, in .ci-author terms.
const (
	EngineStepCommand      EngineStepKind = iota + 1 // command=<text>
	EngineStepStream                                 // stream=<text>
	EngineStepExpectOutput                           // expect=output:contains=
	EngineStepExpectEvent                            // expect=event:namespace=:name=
	EngineStepExpectStream                           // expect=stream:contains=
)

// EngineStep is one parsed engine-step directive, in file order. JSON tags
// shape engine-steps.json, the contract between the runner and the spawned
// `ze-test engine-steps` executor.
type EngineStep struct {
	Kind      EngineStepKind `json:"kind"`
	Text      string         `json:"text,omitempty"` // command/stream text, or contains= needle
	Namespace string         `json:"namespace,omitempty"`
	Name      string         `json:"name,omitempty"`
	Timeout   time.Duration  `json:"timeout,omitempty"`
}

// EngineStepsFileName is the tmpfs file the runner writes and the spawned
// executor reads.
const EngineStepsFileName = "engine-steps.json"

const engineStepDefaultTimeout = 10 * time.Second

// Directive action names shared between the parse dispatch and helpers.
const (
	engineActionCommand = "command"
	engineActionStream  = "stream"
)

// MarshalEngineSteps serializes steps for engine-steps.json.
func MarshalEngineSteps(steps []EngineStep) ([]byte, error) {
	return json.Marshal(steps)
}

// UnmarshalEngineSteps parses engine-steps.json content.
func UnmarshalEngineSteps(data []byte) ([]EngineStep, error) {
	var steps []EngineStep
	if err := json.Unmarshal(data, &steps); err != nil {
		return nil, fmt.Errorf("engine steps file: %w", err)
	}
	return steps, nil
}

// parseEngineTimeout accepts both bare seconds ("10") and Go durations ("10s").
func parseEngineTimeout(v string) (time.Duration, error) {
	if v == "" {
		return engineStepDefaultTimeout, nil
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q (want seconds or duration): %w", v, err)
	}
	return d, nil
}

// parseEngineCmd handles command=<text> and stream=<text> lines. The raw
// remainder after the first '=' is the command text (colons included), so
// the generic action:key=value splitter must not be applied here.
func parseEngineCmd(r *Record, action, line string) error {
	_, text, ok := strings.Cut(line, "=")
	if !ok || strings.TrimSpace(text) == "" {
		return fmt.Errorf("%s= requires a command text", action)
	}
	kind := EngineStepCommand
	if action == engineActionStream {
		kind = EngineStepStream
	}
	r.EngineSteps = append(r.EngineSteps, EngineStep{Kind: kind, Text: strings.TrimSpace(text)})
	return nil
}

// parseEngineExpectEvent handles expect=event:namespace=<ns>:name=<name>[:timeout=]
// directives, whose namespace/name never contain ':', so the generic
// action:key=value splitter (record_parse.go parseLine) parses them cleanly.
func parseEngineExpectEvent(r *Record, kv map[string]string) error {
	timeout, err := parseEngineTimeout(kv["timeout"])
	if err != nil {
		return fmt.Errorf("expect=event: %w", err)
	}
	if kv["namespace"] == "" || kv["name"] == "" {
		return fmt.Errorf("expect=event requires namespace= and name=")
	}
	r.EngineSteps = append(r.EngineSteps, EngineStep{
		Kind: EngineStepExpectEvent, Namespace: kv["namespace"], Name: kv["name"], Timeout: timeout,
	})
	return nil
}

// parseEngineExpectContains handles expect=output / expect=stream directives,
// whose contains= needle may itself hold ':' -- e.g. a compact-JSON fragment
// like "rekey-count":1 that a rekey test polls for. rest is everything after
// "expect=output:" / "expect=stream:". The optional trailing ":timeout=<dur>"
// is split off the end; the remainder after "contains=" is the needle verbatim,
// colons included. This is why parseLine routes these here before applying the
// generic ':' splitter (which would truncate the needle at its first colon),
// mirroring how command=/stream= keep their raw remainder (parseEngineCmd).
func parseEngineExpectContains(r *Record, expType, rest string) error {
	timeoutStr := ""
	if idx := strings.LastIndex(rest, ":timeout="); idx >= 0 {
		timeoutStr = rest[idx+len(":timeout="):]
		rest = rest[:idx]
	}
	needle, ok := strings.CutPrefix(rest, "contains=")
	if !ok || needle == "" {
		return fmt.Errorf("expect=%s requires a non-empty contains=", expType)
	}
	timeout, err := parseEngineTimeout(timeoutStr)
	if err != nil {
		return fmt.Errorf("expect=%s: %w", expType, err)
	}
	kind := EngineStepExpectOutput
	if expType == engineActionStream {
		kind = EngineStepExpectStream
	}
	r.EngineSteps = append(r.EngineSteps, EngineStep{Kind: kind, Text: needle, Timeout: timeout})
	return nil
}

// EngineEventBuffer accumulates delivered events with a scrollback so an
// expectation registered after its event arrived still matches.
type EngineEventBuffer struct {
	mu     sync.Mutex
	events []string
	waiter chan struct{}
}

// NewEngineEventBuffer returns an empty buffer.
func NewEngineEventBuffer() *EngineEventBuffer {
	return &EngineEventBuffer{}
}

// OnEvent records one delivered event; wire it to sdk.Plugin.OnEvent.
func (b *EngineEventBuffer) OnEvent(event string) error {
	b.mu.Lock()
	b.events = append(b.events, event)
	if b.waiter != nil {
		close(b.waiter)
		b.waiter = nil
	}
	b.mu.Unlock()
	return nil
}

// Len returns the number of recorded events; use it as a position marker for
// WaitFrom to exclude deliveries that predate a step.
func (b *EngineEventBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

// Wait blocks until pred matches any recorded event (scrollback included) or
// the deadline passes. Returns the matching event or "".
func (b *EngineEventBuffer) Wait(ctx context.Context, deadline time.Time, pred func(string) bool) string {
	return b.WaitFrom(ctx, 0, deadline, pred)
}

// WaitFrom is Wait restricted to events recorded at position >= from.
func (b *EngineEventBuffer) WaitFrom(ctx context.Context, from int, deadline time.Time, pred func(string) bool) string {
	scanned := from
	for {
		b.mu.Lock()
		for ; scanned < len(b.events); scanned++ {
			if pred(b.events[scanned]) {
				ev := b.events[scanned]
				b.mu.Unlock()
				return ev
			}
		}
		if b.waiter == nil {
			b.waiter = make(chan struct{})
		}
		w := b.waiter
		b.mu.Unlock()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return ""
		}
		timer := time.NewTimer(remaining)
		select {
		case <-w:
			timer.Stop()
		case <-timer.C:
			return ""
		case <-ctx.Done():
			timer.Stop()
			return ""
		}
	}
}

// EngineDispatch abstracts sdk.Plugin.DispatchCommand for the executor core
// (and its unit tests): returns (status, data, error).
type EngineDispatch func(ctx context.Context, command string) (string, string, error)

// RunEngineSteps executes the steps in file order against a live engine.
// Called by the spawned `ze-test engine-steps` executor from within
// OnAllPluginsReady; events must be wired to buf before the daemon starts
// delivering them.
func RunEngineSteps(ctx context.Context, dispatch EngineDispatch, buf *EngineEventBuffer, steps []EngineStep) error {
	var tb textbuf.Buffer

	lastCommand := ""
	lastOutput := ""
	for i, step := range steps {
		switch step.Kind {
		case EngineStepCommand, EngineStepStream:
			status, data, err := dispatch(ctx, step.Text)
			if err != nil {
				return fmt.Errorf("engine step %d (%s): %w", i+1, step.Text, err)
			}
			lastCommand = step.Text
			lastOutput = tb.Reset().Str(status).Byte(' ').Str(data).String()

		case EngineStepExpectOutput:
			deadline := time.Now().Add(step.Timeout)
			for !strings.Contains(lastOutput, step.Text) {
				if lastCommand == "" {
					return fmt.Errorf("engine step %d: expect=output before any command=", i+1)
				}
				if time.Now().After(deadline) {
					return fmt.Errorf("engine step %d: output missing %q within %s (last %q output %.200q)",
						i+1, step.Text, step.Timeout, lastCommand, lastOutput)
				}
				select {
				case <-ctx.Done():
					return fmt.Errorf("engine step %d interrupted: %w", i+1, ctx.Err())
				case <-time.After(200 * time.Millisecond):
				}
				status, data, err := dispatch(ctx, lastCommand)
				if err != nil {
					return fmt.Errorf("engine step %d re-dispatch %q: %w", i+1, lastCommand, err)
				}
				lastOutput = tb.Reset().Str(status).Byte(' ').Str(data).String()
			}

		case EngineStepExpectEvent:
			// Delivered events carry the BARE payload JSON -- no namespace or
			// name envelope exists on the wire (plugin/server/dispatch.go
			// payloadToJSON) -- so the step scopes by EXCLUSIVE subscription:
			// subscribe to exactly this pair, count only deliveries recorded
			// after the subscription, unsubscribe again. Any delivery inside
			// the window is, by construction, this event.
			ns, name := step.Namespace, step.Name
			pos := buf.Len()
			subCmd := tb.Reset().Str("request subscribe ").Str(ns).Str(" event ").Str(name).String()
			status, _, err := dispatch(ctx, subCmd)
			if err != nil {
				return fmt.Errorf("engine step %d subscribe %s/%s: %w", i+1, ns, name, err)
			}
			if status != "done" {
				return fmt.Errorf("engine step %d subscribe %s/%s: status=%q", i+1, ns, name, status)
			}
			ev := buf.WaitFrom(ctx, pos, time.Now().Add(step.Timeout), func(string) bool { return true })
			unsubCmd := tb.Reset().Str("request unsubscribe ").Str(ns).Str(" event ").Str(name).String()
			if _, _, unsubErr := dispatch(ctx, unsubCmd); unsubErr != nil {
				return fmt.Errorf("engine step %d unsubscribe %s/%s: %w", i+1, ns, name, unsubErr)
			}
			if ev == "" {
				return fmt.Errorf("engine step %d: event %s/%s not delivered within %s",
					i+1, ns, name, step.Timeout)
			}

		case EngineStepExpectStream:
			needle := step.Text
			if ev := buf.Wait(ctx, time.Now().Add(step.Timeout), func(e string) bool {
				return strings.Contains(e, needle)
			}); ev == "" {
				return fmt.Errorf("engine step %d: stream output missing %q within %s",
					i+1, needle, step.Timeout)
			}
		}
	}
	return nil
}
