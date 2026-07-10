// VALIDATES: engine-step directive parsing (command=/stream=/expect=output|
// event|stream), the steps-file round-trip handed to the spawned
// `ze-test engine-steps` executor, and the executor core's step semantics
// (spec-test-coverage-gaps AC-2).
// PREVENTS: directive drift breaking the ipsec suite silently -- the parse
// layer and step semantics are the contract the .ci files are written against.
package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func parseCIRecord(t *testing.T, body string) *Record {
	t.Helper()
	et := &EncodingTests{}
	r := newRecord("engine-steps-test")
	for _, line := range splitLines(body) {
		if line == "" || line[0] == '#' {
			continue
		}
		if err := et.parseLine(r, "test/engine/fake.ci", line); err != nil {
			t.Fatalf("parseLine(%q): %v", line, err)
		}
	}
	return r
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func TestParseEngineSteps(t *testing.T) {
	r := parseCIRecord(t, `command=show vpn ipsec status
expect=output:contains=engine-running:timeout=5
expect=event:namespace=vpn-ipsec:name=sa-up:timeout=10
stream=monitor vpn ipsec
expect=stream:contains=child-up:timeout=5
`)

	want := []EngineStep{
		{Kind: EngineStepCommand, Text: "show vpn ipsec status"},
		{Kind: EngineStepExpectOutput, Text: "engine-running", Timeout: 5 * time.Second},
		{Kind: EngineStepExpectEvent, Namespace: "vpn-ipsec", Name: "sa-up", Timeout: 10 * time.Second},
		{Kind: EngineStepStream, Text: "monitor vpn ipsec"},
		{Kind: EngineStepExpectStream, Text: "child-up", Timeout: 5 * time.Second},
	}
	if len(r.EngineSteps) != len(want) {
		t.Fatalf("EngineSteps len = %d, want %d (%+v)", len(r.EngineSteps), len(want), r.EngineSteps)
	}
	for i, w := range want {
		g := r.EngineSteps[i]
		if g.Kind != w.Kind || g.Text != w.Text || g.Namespace != w.Namespace ||
			g.Name != w.Name || g.Timeout != w.Timeout {
			t.Errorf("step %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestEngineStepsFileRoundTrip(t *testing.T) {
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "show vpn ipsec status"},
		{Kind: EngineStepExpectOutput, Text: "engine-running", Timeout: 5 * time.Second},
		{Kind: EngineStepExpectEvent, Namespace: "vpn-ipsec", Name: "sa-up", Timeout: 10 * time.Second},
	}
	data, err := MarshalEngineSteps(steps)
	if err != nil {
		t.Fatalf("MarshalEngineSteps: %v", err)
	}
	got, err := UnmarshalEngineSteps(data)
	if err != nil {
		t.Fatalf("UnmarshalEngineSteps: %v", err)
	}
	if len(got) != len(steps) {
		t.Fatalf("round-trip len = %d, want %d", len(got), len(steps))
	}
	for i := range steps {
		if got[i] != steps[i] {
			t.Errorf("step %d = %+v, want %+v", i, got[i], steps[i])
		}
	}
}

func TestParseEngineCommandKeepsColons(t *testing.T) {
	r := parseCIRecord(t, "command=request l2tp outgoing-call remote lns1 called 555:1234\n")
	if len(r.EngineSteps) != 1 {
		t.Fatalf("EngineSteps = %+v", r.EngineSteps)
	}
	if got := r.EngineSteps[0].Text; got != "request l2tp outgoing-call remote lns1 called 555:1234" {
		t.Fatalf("command text = %q (colons must be preserved)", got)
	}
}

func TestParseEngineTimeoutForms(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"":      engineStepDefaultTimeout,
		"7":     7 * time.Second,
		"250ms": 250 * time.Millisecond,
	} {
		got, err := parseEngineTimeout(in)
		if err != nil {
			t.Fatalf("parseEngineTimeout(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("parseEngineTimeout(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := parseEngineTimeout("soon"); err == nil {
		t.Error("parseEngineTimeout(soon) should fail")
	}
}

func TestParseEngineRejects(t *testing.T) {
	et := &EncodingTests{}
	for _, line := range []string{
		"command=",
		"stream=",
		"expect=output:timeout=5",
		"expect=event:namespace=vpn-ipsec",
		"expect=event:name=sa-up",
		"expect=stream:timeout=5",
	} {
		r := newRecord("engine-steps-reject")
		if err := et.parseLine(r, "test/engine/fake.ci", line); err == nil {
			t.Errorf("parseLine(%q) should fail", line)
		}
	}
}

// // test-relax: an earlier revision matched delivered events by a
// namespace/name envelope, but the plugin deliver-event wire carries the BARE
// payload JSON only (internal/component/plugin/server/dispatch.go
// payloadToJSON) -- there is nothing to match an envelope against. The
// executor now scopes expect=event by EXCLUSIVE per-step subscription
// (subscribe -> any delivery counts -> unsubscribe), tested below.
func TestEngineEventBufferScrollbackAndPositions(t *testing.T) {
	b := NewEngineEventBuffer()
	if err := b.OnEvent(`{"peer-name":"peer-1"}`); err != nil {
		t.Fatalf("OnEvent: %v", err)
	}
	// An event that arrived BEFORE the wait: scrollback satisfies it.
	got := b.Wait(t.Context(), time.Now().Add(time.Second), func(e string) bool {
		return strings.Contains(e, "peer-1")
	})
	if got == "" {
		t.Fatal("Wait missed the scrollback event")
	}
	// WaitFrom(Len()) must ignore that old event and time out.
	pos := b.Len()
	got = b.WaitFrom(t.Context(), pos, time.Now().Add(50*time.Millisecond), func(string) bool {
		return true
	})
	if got != "" {
		t.Fatalf("WaitFrom(Len) = %q, want timeout (old events excluded)", got)
	}
	// A post-position delivery satisfies WaitFrom.
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = b.OnEvent(`{"peer-name":"peer-2"}`)
	}()
	got = b.WaitFrom(t.Context(), pos, time.Now().Add(time.Second), func(string) bool {
		return true
	})
	if got == "" {
		t.Fatal("WaitFrom missed the post-position event")
	}
}

// fakeDispatch scripts DispatchCommand responses for RunEngineSteps.
type fakeDispatch struct {
	calls     []string
	responses map[string][]string // command -> successive "status data" outputs
}

func (f *fakeDispatch) dispatch(_ context.Context, command string) (string, string, error) {
	f.calls = append(f.calls, command)
	outs := f.responses[command]
	if len(outs) == 0 {
		return "done", "", nil
	}
	out := outs[0]
	if len(outs) > 1 {
		f.responses[command] = outs[1:]
	}
	status, data, _ := strings.Cut(out, " ")
	if status == "ERR" {
		return "", "", errors.New(data)
	}
	return status, data, nil
}

func TestRunEngineStepsOutputPolling(t *testing.T) {
	f := &fakeDispatch{responses: map[string][]string{
		"show vpn ipsec status": {
			`done {"engine-starting":true}`,
			`done {"engine-running":true,"configured-peers":1}`,
		},
	}}
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "show vpn ipsec status"},
		{Kind: EngineStepExpectOutput, Text: "engine-running", Timeout: 3 * time.Second},
	}
	err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps)
	if err != nil {
		t.Fatalf("RunEngineSteps: %v", err)
	}
	// First result lacked the needle; the executor must have re-dispatched.
	if len(f.calls) < 2 {
		t.Fatalf("calls = %v, want re-dispatch polling", f.calls)
	}
}

func TestRunEngineStepsOutputTimeout(t *testing.T) {
	f := &fakeDispatch{responses: map[string][]string{
		"show vpn ipsec status": {`done {"engine-starting":true}`},
	}}
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "show vpn ipsec status"},
		{Kind: EngineStepExpectOutput, Text: "engine-running", Timeout: 400 * time.Millisecond},
	}
	err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps)
	if err == nil || !strings.Contains(err.Error(), "engine-running") {
		t.Fatalf("RunEngineSteps = %v, want output-timeout error naming the needle", err)
	}
}

func TestRunEngineStepsEventAndStream(t *testing.T) {
	f := &fakeDispatch{responses: map[string][]string{}}
	buf := NewEngineEventBuffer()
	steps := []EngineStep{
		{Kind: EngineStepExpectEvent, Namespace: "vpn-ipsec", Name: "sa-up", Timeout: 2 * time.Second},
		{Kind: EngineStepStream, Text: "monitor vpn ipsec"},
		{Kind: EngineStepExpectStream, Text: "child-up", Timeout: 2 * time.Second},
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = buf.OnEvent(`{"peer-name":"peer-1","local-address":"127.0.0.1"}`)
		time.Sleep(100 * time.Millisecond)
		_ = buf.OnEvent(`{"peer-name":"peer-1","detail":"child-up esp"}`)
	}()
	if err := RunEngineSteps(t.Context(), f.dispatch, buf, steps); err != nil {
		t.Fatalf("RunEngineSteps: %v", err)
	}
	// The event expectation scopes by exclusive subscription: subscribe at
	// the step, any delivery counts, then unsubscribe.
	joined := strings.Join(f.calls, ";")
	if !strings.Contains(joined, "request subscribe vpn-ipsec event sa-up") {
		t.Fatalf("calls = %v, want step-time subscription", f.calls)
	}
	if !strings.Contains(joined, "request unsubscribe vpn-ipsec event sa-up") {
		t.Fatalf("calls = %v, want unsubscribe after the event step", f.calls)
	}
}

func TestRunEngineStepsEventIgnoresPreSubscriptionDeliveries(t *testing.T) {
	f := &fakeDispatch{responses: map[string][]string{}}
	buf := NewEngineEventBuffer()
	// An event recorded BEFORE the expect=event step (e.g. noise from an
	// earlier stream step) must NOT satisfy the exclusive subscription window.
	if err := buf.OnEvent(`{"stale":"event"}`); err != nil {
		t.Fatalf("OnEvent: %v", err)
	}
	steps := []EngineStep{
		{Kind: EngineStepExpectEvent, Namespace: "vpn-ipsec", Name: "sa-up", Timeout: 300 * time.Millisecond},
	}
	if err := RunEngineSteps(t.Context(), f.dispatch, buf, steps); err == nil {
		t.Fatal("stale pre-subscription delivery must not satisfy expect=event")
	}
}

func TestRunEngineStepsExpectOutputBeforeCommand(t *testing.T) {
	f := &fakeDispatch{responses: map[string][]string{}}
	steps := []EngineStep{
		{Kind: EngineStepExpectOutput, Text: "anything", Timeout: time.Second},
	}
	if err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps); err == nil {
		t.Fatal("expect=output before any command= must fail")
	}
}
