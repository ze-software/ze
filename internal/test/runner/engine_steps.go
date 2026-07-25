// Design: docs/architecture/testing/ci-format.md -- engine-step directives
//
// Engine steps drive a live test daemon through CLI dispatch, first-class in
// .ci instead of an embedded Python observer:
//
//	command=<cli command text>
//	stream=<monitor command text>
//	expect=output:contains=<text>[:timeout=<dur>]
//	expect=output:matches=<regexp>[:timeout=<dur>]
//	expect=output:absent=<text>[:timeout=<dur>]
//	expect=output:json=<dotted.path>=<value>[:timeout=<dur>]
//	expect=event:namespace=<ns>:name=<name>[:timeout=<dur>]
//	expect=stream:contains=<text>[:timeout=<dur>]
//	expect=stream:matches=<regexp>[:timeout=<dur>]
//
// The expect=output predicate re-dispatches the most recent command until the
// predicate holds or the timeout expires: contains= (substring, the default),
// matches= (regexp), absent= (substring must NOT be present, for withdrawals),
// or json= (a dotted path into the JSON data field stringifies to the value).
// expect=stream supports contains= / matches= over delivered stream events;
// absent= / json= are expect=output only.
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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
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
	Text      string         `json:"text,omitempty"` // command/stream text, or the predicate operand (contains=/matches= needle, absent= needle, or json= value)
	Namespace string         `json:"namespace,omitempty"`
	Name      string         `json:"name,omitempty"`
	Timeout   time.Duration  `json:"timeout,omitempty"`
	// Match is the expect=output/expect=stream predicate kind: "" or "contains"
	// (substring, the default and back-compatible form), "matches" (regexp over
	// the output), "absent" (substring must NOT be present; output-only), or
	// "json" (dotted Path into the JSON data equals Text; output-only). omitempty
	// keeps pre-existing contains= steps byte-identical in engine-steps.json.
	Match string `json:"match,omitempty"`
	// Path is the dotted JSON path for Match=="json" (e.g. "0.prefix"); segments
	// index maps by key or arrays by integer index.
	Path string `json:"path,omitempty"`
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

// expect=output / expect=stream predicate kinds stored in EngineStep.Match. The
// empty string means contains= (substring, the default and back-compatible
// form), so it is deliberately not named here.
const (
	engineMatchRegexp = "matches" // regexp over the output
	engineMatchAbsent = "absent"  // substring must be absent (output-only)
	engineMatchJSON   = "json"    // dotted Path into JSON data equals Text (output-only)
)

// marshalEngineSteps serializes steps for engine-steps.json. Runner-internal:
// only the runner writes the file (runner_exec.go); the spawned executor reads
// it cross-package via the exported UnmarshalEngineSteps.
func marshalEngineSteps(steps []EngineStep) ([]byte, error) {
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

// parseEngineExpectContains handles expect=output / expect=stream directives.
// The predicate operand may itself hold ':' -- e.g. a compact-JSON fragment
// like "rekey-count":1 that a rekey test polls for, or an IPv6 json= value --
// so parseLine routes these here before the generic ':' splitter (which would
// truncate the operand at its first colon), mirroring how command=/stream= keep
// their raw remainder (parseEngineCmd). rest is everything after
// "expect=output:" / "expect=stream:". The optional trailing ":timeout=<dur>"
// is split off the END first; the remainder is one predicate:
//
//	contains=<needle>            substring (default kind, Match=="")
//	matches=<regexp>             regexp, compiled here so a bad regexp fails at parse (R-3)
//	absent=<needle>              substring must be absent (expect=output only)
//	json=<dotted.path>=<value>   JSON path stringifies to value (expect=output only)
func parseEngineExpectContains(r *Record, expType, rest string) error {
	timeoutStr := ""
	if idx := strings.LastIndex(rest, ":timeout="); idx >= 0 {
		timeoutStr = rest[idx+len(":timeout="):]
		rest = rest[:idx]
	}
	timeout, err := parseEngineTimeout(timeoutStr)
	if err != nil {
		return fmt.Errorf("expect=%s: %w", expType, err)
	}
	kind := EngineStepExpectOutput
	if expType == engineActionStream {
		kind = EngineStepExpectStream
	}
	step := EngineStep{Kind: kind, Timeout: timeout}

	switch {
	case strings.HasPrefix(rest, "contains="):
		step.Text = rest[len("contains="):]
		if step.Text == "" {
			return fmt.Errorf("expect=%s requires a non-empty contains=", expType)
		}
	case strings.HasPrefix(rest, "matches="):
		step.Match = engineMatchRegexp
		step.Text = rest[len("matches="):]
		if step.Text == "" {
			return fmt.Errorf("expect=%s requires a non-empty matches=", expType)
		}
		if _, reErr := regexp.Compile(step.Text); reErr != nil {
			return fmt.Errorf("expect=%s matches= invalid regexp %q: %w", expType, step.Text, reErr)
		}
	case strings.HasPrefix(rest, "absent="):
		if expType == engineActionStream {
			return fmt.Errorf("expect=stream does not support absent= (output-only predicate)")
		}
		step.Match = engineMatchAbsent
		step.Text = rest[len("absent="):]
		if step.Text == "" {
			return fmt.Errorf("expect=%s requires a non-empty absent=", expType)
		}
	case strings.HasPrefix(rest, "json="):
		if expType == engineActionStream {
			return fmt.Errorf("expect=stream does not support json= (output-only predicate)")
		}
		operand := rest[len("json="):]
		path, value, ok := strings.Cut(operand, "=")
		if !ok || path == "" || value == "" {
			return fmt.Errorf("expect=%s json= requires <dotted.path>=<value>, got %q", expType, operand)
		}
		step.Match = engineMatchJSON
		step.Path = path
		step.Text = value
	default:
		return fmt.Errorf("expect=%s requires one of contains=/matches=/absent=/json=", expType)
	}

	r.EngineSteps = append(r.EngineSteps, step)
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
// waitFrom to exclude deliveries that predate a step.
func (b *EngineEventBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

// Wait blocks until pred matches any recorded event (scrollback included) or
// the deadline passes. Returns the matching event or "".
func (b *EngineEventBuffer) Wait(ctx context.Context, deadline time.Time, pred func(string) bool) string {
	return b.waitFrom(ctx, 0, deadline, pred)
}

// waitFrom is Wait restricted to events recorded at position >= from.
func (b *EngineEventBuffer) waitFrom(ctx context.Context, from int, deadline time.Time, pred func(string) bool) string {
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
	lastData := "" // raw data field alone, for json= path walks (A-3 constraint)
	for i, step := range steps {
		switch step.Kind {
		case EngineStepCommand, EngineStepStream:
			status, data, err := dispatch(ctx, step.Text)
			if err != nil {
				return fmt.Errorf("engine step %d (%s): %w", i+1, step.Text, err)
			}
			lastCommand = step.Text
			lastData = data
			lastOutput = tb.Reset().Str(status).Byte(' ').Str(data).String()

		case EngineStepExpectOutput:
			// matches= regexps are compiled once here (parse already validated
			// them; R-3) so the poll loop never recompiles.
			var re *regexp.Regexp
			if step.Match == engineMatchRegexp {
				compiled, reErr := regexp.Compile(step.Text)
				if reErr != nil {
					return fmt.Errorf("engine step %d: matches= invalid regexp %q: %w", i+1, step.Text, reErr)
				}
				re = compiled
			}
			deadline := time.Now().Add(step.Timeout)
			for !engineOutputSatisfied(step, re, lastOutput, lastData) {
				if lastCommand == "" {
					return fmt.Errorf("engine step %d: expect=output before any command=", i+1)
				}
				if time.Now().After(deadline) {
					return engineOutputTimeoutErr(i+1, step, lastCommand, lastOutput, lastData)
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
				lastData = data
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
			ev := buf.waitFrom(ctx, pos, time.Now().Add(step.Timeout), func(string) bool { return true })
			unsubCmd := tb.Reset().Str("request unsubscribe ").Str(ns).Str(" event ").Str(name).String()
			if _, _, unsubErr := dispatch(ctx, unsubCmd); unsubErr != nil {
				return fmt.Errorf("engine step %d unsubscribe %s/%s: %w", i+1, ns, name, unsubErr)
			}
			if ev == "" {
				return fmt.Errorf("engine step %d: event %s/%s not delivered within %s",
					i+1, ns, name, step.Timeout)
			}

		case EngineStepExpectStream:
			// expect=stream supports contains= (default) and matches= only;
			// absent=/json= are rejected at parse (output-only, AC-8).
			var pred func(string) bool
			if step.Match == engineMatchRegexp {
				re, reErr := regexp.Compile(step.Text)
				if reErr != nil {
					return fmt.Errorf("engine step %d: matches= invalid regexp %q: %w", i+1, step.Text, reErr)
				}
				pred = re.MatchString
			} else {
				needle := step.Text
				pred = func(e string) bool { return strings.Contains(e, needle) }
			}
			if ev := buf.Wait(ctx, time.Now().Add(step.Timeout), pred); ev == "" {
				if step.Match == engineMatchRegexp {
					return fmt.Errorf("engine step %d: stream never matched regexp %q within %s",
						i+1, step.Text, step.Timeout)
				}
				return fmt.Errorf("engine step %d: stream output missing %q within %s",
					i+1, step.Text, step.Timeout)
			}
		}
	}
	return nil
}

// engineOutputSatisfied reports whether an expect=output predicate holds for the
// most recent dispatch. output is "status data"; data is the raw data field
// alone (used by json=). re is the pre-compiled matches= regexp (nil otherwise).
// An empty/"contains" Match is the default substring check.
func engineOutputSatisfied(step EngineStep, re *regexp.Regexp, output, data string) bool {
	switch step.Match {
	case engineMatchRegexp:
		return re != nil && re.MatchString(output)
	case engineMatchAbsent:
		return !strings.Contains(output, step.Text)
	case engineMatchJSON:
		v, ok := engineJSONPathValue(data, step.Path)
		return ok && v == step.Text
	default: // "" or "contains"
		return strings.Contains(output, step.Text)
	}
}

// engineOutputTimeoutErr builds the timeout error for an expect=output step,
// naming the predicate operand and the last observed output/data (truncated,
// mirroring the %.200q style of the original contains= error) so a failing json=
// path or unmatched regexp is diagnosable without re-running (R-2).
func engineOutputTimeoutErr(stepNum int, step EngineStep, lastCommand, lastOutput, lastData string) error {
	switch step.Match {
	case engineMatchRegexp:
		return fmt.Errorf("engine step %d: output never matched regexp %q within %s (last %q output %.200q)",
			stepNum, step.Text, step.Timeout, lastCommand, lastOutput)
	case engineMatchAbsent:
		return fmt.Errorf("engine step %d: substring %q still present within %s (last %q output %.200q)",
			stepNum, step.Text, step.Timeout, lastCommand, lastOutput)
	case engineMatchJSON:
		return fmt.Errorf("engine step %d: json path %q never equaled %q within %s (last %q data %.200q)",
			stepNum, step.Path, step.Text, step.Timeout, lastCommand, lastData)
	default:
		return fmt.Errorf("engine step %d: output missing %q within %s (last %q output %.200q)",
			stepNum, step.Text, step.Timeout, lastCommand, lastOutput)
	}
}

// engineJSONPathValue walks a dotted path into JSON-decoded data and returns the
// leaf stringified for comparison. Segments index a JSON object by key or a JSON
// array by integer index (0..len-1; negative or out-of-range yields not-found,
// never a panic). Non-JSON data, a missing key, or an out-of-range index all
// return ("", false) -- treated as "not satisfied yet" while polling.
func engineJSONPathValue(data, path string) (string, bool) {
	var v any
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		return "", false
	}
	for seg := range strings.SplitSeq(path, ".") {
		switch node := v.(type) {
		case map[string]any:
			next, ok := node[seg]
			if !ok {
				return "", false
			}
			v = next
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return "", false
			}
			v = node[idx]
		default:
			return "", false
		}
	}
	return engineStringifyJSON(v), true
}

// engineStringifyJSON renders a JSON-decoded leaf for string comparison: a
// string is returned verbatim; everything else (number, bool, null, or a nested
// object/array) is marshaled back to compact JSON.
func engineStringifyJSON(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
