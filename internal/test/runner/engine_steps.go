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
//	expect=command-error:contains=<text>
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
// match delivered events. The executor declares ONE startup subscription
// derived from the expect=event steps themselves (EngineStepSubscriptionFor)
// and asks for enveloped delivery, so an event the daemon fired before the
// executor reached its step is already in the buffer and still matches.

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
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// EngineStepKind identifies one engine-step directive.
type EngineStepKind uint8

// Engine step kinds, in .ci-author terms.
const (
	EngineStepCommand            EngineStepKind = iota + 1 // command=<text>
	EngineStepStream                                       // stream=<text>
	EngineStepExpectOutput                                 // expect=output:contains=
	EngineStepExpectEvent                                  // expect=event:namespace=:name=
	EngineStepExpectStream                                 // expect=stream:contains=
	EngineStepExpectCommandError                           // expect=command-error:contains=
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

// EngineStepSubscription is the startup event subscription a parsed step list
// needs: the ONE namespace its expect=event steps name, and the distinct event
// names within it, in first-appearance order. The zero value (no namespace, no
// events) says the steps expect no event at all, and the executor then declares
// no subscription.
type EngineStepSubscription struct {
	Namespace string
	Events    []string
}

// EngineStepSubscriptionFor derives the startup event subscription from the
// expect=event steps themselves, so a .ci declares what it waits for exactly
// once, in the step it already wrote (ai/rules/principles.md).
//
// The subscription MUST be in place before the daemon starts its peers, which
// is why the executor declares it before Run rather than at the step: the IKE
// engine emits its first sa-up from startup reconciliation, and a step-time
// subscribe could only ever observe the SECOND one.
//
// rpc.SubscribeEventsInput carries ONE namespace, so steps naming two of them
// have no correct startup subscription and this returns an error naming both.
// Picking one and dropping the rest would leave the dropped steps waiting on an
// event nobody delivers, and report that as a product failure.
func EngineStepSubscriptionFor(steps []EngineStep) (EngineStepSubscription, error) {
	var sub EngineStepSubscription
	var namespaces []string
	seenNamespace := make(map[string]bool)
	seenEvent := make(map[string]bool)

	for _, step := range steps {
		if step.Kind != EngineStepExpectEvent {
			continue
		}
		if !seenNamespace[step.Namespace] {
			seenNamespace[step.Namespace] = true
			namespaces = append(namespaces, step.Namespace)
		}
		if !seenEvent[step.Name] {
			seenEvent[step.Name] = true
			sub.Events = append(sub.Events, step.Name)
		}
	}

	if len(namespaces) == 0 {
		return EngineStepSubscription{}, nil
	}
	if len(namespaces) > 1 {
		return EngineStepSubscription{}, fmt.Errorf(
			"expect=event steps name %d event namespaces (%s), and one startup subscription carries exactly one: split the test",
			len(namespaces), strings.Join(namespaces, ", "))
	}
	sub.Namespace = namespaces[0]
	return sub, nil
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
//
// parseEngineExpectCommandError handles expect=command-error:contains=<text>.
//
// It asserts that the PRECEDING command= failed, and that its message contains
// the needle. Without it no .ci could assert an operational error at all: the
// SDK turns a StatusError response into a Go error, so the command step aborts
// the run before any expect= is reached.
//
// That gap made a whole class of behavior untestable end to end, and it is the
// class where being wrong is most expensive: a command that must REFUSE (an
// unreadable dataplane, a reserved SPI, a missing capability) is exactly the
// command whose failure mode is answering confidently instead.
//
// There is no timeout. An error is the result of one dispatch, so re-dispatching
// until it appears would be waiting for a state change that no expect= can cause.
func parseEngineExpectCommandError(r *Record, rest string) error {
	needle, ok := strings.CutPrefix(rest, "contains=")
	if !ok {
		return fmt.Errorf("expect=command-error requires contains=")
	}
	if needle == "" {
		return fmt.Errorf("expect=command-error requires a non-empty contains=")
	}
	r.EngineSteps = append(r.EngineSteps, EngineStep{
		Kind: EngineStepExpectCommandError,
		Text: needle,
	})
	return nil
}

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

// Wait blocks until pred matches any recorded event (scrollback included) or
// the deadline passes. Returns the matching event or "".
//
// The scan starts at the first event this buffer ever recorded, never at the
// length it had when the step began. A daemon emits its first event when the
// thing happens, not when a test is ready to look: the IKE engine emits sa-up
// from its startup reconciliation, before the executor runs step one. pred is
// what scopes the match, so it MUST identify the event rather than accept any
// delivery.
func (b *EngineEventBuffer) Wait(ctx context.Context, deadline time.Time, pred func(string) bool) string {
	scanned := 0
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

	// pendingErr holds a command= failure until the very next step decides what
	// it means: an expect=command-error consumes it, anything else raises it.
	var pendingErr error
	pendingErrStep := 0
	pendingErrCmd := ""

	for i, step := range steps {
		if pendingErr != nil && step.Kind != EngineStepExpectCommandError {
			return fmt.Errorf("engine step %d (%s): %w", pendingErrStep, pendingErrCmd, pendingErr)
		}
		switch step.Kind {
		case EngineStepCommand, EngineStepStream:
			status, data, err := dispatch(ctx, step.Text)
			if err != nil {
				// The error is HELD, not raised, so the next step can be an
				// expect=command-error that consumes it. It is not swallowed:
				// pendingErr below fails the run at the next step of any other
				// kind, and at the end of the file. A command that fails and is
				// never checked still fails the test, which is the behavior
				// every existing .ci relies on.
				pendingErr = err
				pendingErrStep = i + 1
				pendingErrCmd = step.Text
				lastCommand = ""
				lastOutput = ""
				lastData = ""
				continue
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
			// Every delivered event carries its own (namespace, event) identity,
			// because the executor opted the whole process into enveloped
			// delivery (plugin/server/dispatch.go buildEventEnvelope). So the
			// step matches on that identity over the WHOLE buffer, scrollback
			// included, and needs no subscription of its own: the one the
			// executor declared before Run covers every expect=event in the
			// file (EngineStepSubscriptionFor).
			//
			// A bare (un-enveloped) payload decodes with an empty Namespace and
			// Event, so it can never satisfy a step: a delivery this executor
			// did not ask to be enveloped is not silently accepted.
			ns, name := step.Namespace, step.Name
			pred := func(e string) bool {
				envelope, envErr := rpc.ParseEventEnvelope(e)
				if envErr != nil {
					return false
				}
				return envelope.Namespace == ns && envelope.Event == name
			}
			if ev := buf.Wait(ctx, time.Now().Add(step.Timeout), pred); ev == "" {
				return fmt.Errorf("engine step %d: event %s/%s not delivered within %s",
					i+1, ns, name, step.Timeout)
			}

		case EngineStepExpectCommandError:
			if pendingErr == nil {
				return fmt.Errorf("engine step %d: expect=command-error, but the preceding command succeeded", i+1)
			}
			msg := pendingErr.Error()
			if !strings.Contains(msg, step.Text) {
				return fmt.Errorf("engine step %d: expect=command-error contains=%q, got %.400q", i+1, step.Text, msg)
			}
			pendingErr = nil
			pendingErrStep = 0
			pendingErrCmd = ""

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

	// A command that failed as the LAST step has no following step to raise it,
	// so the run would end green on a command that errored. Raise it here.
	if pendingErr != nil {
		return fmt.Errorf("engine step %d (%s): %w", pendingErrStep, pendingErrCmd, pendingErr)
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
	// The error is bound before the test rather than inline: gocritic's
	// uncheckedInlineErr reads the inline form here as unchecked.
	err := json.Unmarshal([]byte(data), &v)
	if err != nil {
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
