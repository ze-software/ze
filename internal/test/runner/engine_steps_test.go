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

func TestParseEngineExpectContainsColonNeedle(t *testing.T) {
	// A contains= needle may itself hold ':' -- e.g. a compact-JSON fragment
	// like "rekey-count":1 that a rekey test polls for. The ':timeout=' suffix
	// must still be split off, but colons inside the needle preserved. Forms
	// without a colon or without a timeout must keep working unchanged.
	r := parseCIRecord(t, `command=show vpn ipsec sa | json
expect=output:contains="rekey-count":1:timeout=7
expect=output:contains=aes-cbc
expect=stream:contains=a:b:c:timeout=3
`)

	want := []EngineStep{
		{Kind: EngineStepCommand, Text: "show vpn ipsec sa | json"},
		{Kind: EngineStepExpectOutput, Text: `"rekey-count":1`, Timeout: 7 * time.Second},
		{Kind: EngineStepExpectOutput, Text: "aes-cbc", Timeout: engineStepDefaultTimeout},
		{Kind: EngineStepExpectStream, Text: "a:b:c", Timeout: 3 * time.Second},
	}
	if len(r.EngineSteps) != len(want) {
		t.Fatalf("EngineSteps len = %d, want %d (%+v)", len(r.EngineSteps), len(want), r.EngineSteps)
	}
	for i, w := range want {
		g := r.EngineSteps[i]
		if g.Kind != w.Kind || g.Text != w.Text || g.Timeout != w.Timeout {
			t.Errorf("step %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestParseEngineExpectMatches(t *testing.T) {
	// matches=<regex> parses to Match="matches" with the regex source in Text;
	// the ':timeout=' suffix is still split off. A regex may contain ':'.
	r := parseCIRecord(t, `command=show rib
expect=output:matches=engine-(running|ready):timeout=4
expect=stream:matches=child-(up|rekeyed)
`)
	want := []EngineStep{
		{Kind: EngineStepCommand, Text: "show rib"},
		{Kind: EngineStepExpectOutput, Match: "matches", Text: "engine-(running|ready)", Timeout: 4 * time.Second},
		{Kind: EngineStepExpectStream, Match: "matches", Text: "child-(up|rekeyed)", Timeout: engineStepDefaultTimeout},
	}
	if len(r.EngineSteps) != len(want) {
		t.Fatalf("EngineSteps len = %d, want %d (%+v)", len(r.EngineSteps), len(want), r.EngineSteps)
	}
	for i, w := range want {
		g := r.EngineSteps[i]
		if g.Kind != w.Kind || g.Match != w.Match || g.Text != w.Text || g.Timeout != w.Timeout {
			t.Errorf("step %d = %+v, want %+v", i, g, w)
		}
	}

	// A malformed regex must fail at PARSE time (R-3), not hang to timeout.
	et := &EncodingTests{}
	if err := et.parseLine(newRecord("bad-regex"), "test/engine/fake.ci", "expect=output:matches=engine-[running"); err == nil {
		t.Error("expect=output:matches= with an invalid regex must fail parse")
	}
}

func TestParseEngineExpectAbsent(t *testing.T) {
	// absent=<needle> parses to Match="absent"; colons inside the needle are
	// preserved (only the trailing ':timeout=' is split off).
	r := parseCIRecord(t, `command=show rib
expect=output:absent=172.16.0.0/16:timeout=3
expect=output:absent=a:b:c
`)
	want := []EngineStep{
		{Kind: EngineStepCommand, Text: "show rib"},
		{Kind: EngineStepExpectOutput, Match: "absent", Text: "172.16.0.0/16", Timeout: 3 * time.Second},
		{Kind: EngineStepExpectOutput, Match: "absent", Text: "a:b:c", Timeout: engineStepDefaultTimeout},
	}
	if len(r.EngineSteps) != len(want) {
		t.Fatalf("EngineSteps len = %d, want %d (%+v)", len(r.EngineSteps), len(want), r.EngineSteps)
	}
	for i, w := range want {
		g := r.EngineSteps[i]
		if g.Kind != w.Kind || g.Match != w.Match || g.Text != w.Text || g.Timeout != w.Timeout {
			t.Errorf("step %d = %+v, want %+v", i, g, w)
		}
	}

	// absent= is output-only: expect=stream:absent= must fail parse (AC-8).
	et := &EncodingTests{}
	if err := et.parseLine(newRecord("stream-absent"), "test/engine/fake.ci", "expect=stream:absent=x"); err == nil {
		t.Error("expect=stream:absent= must fail parse (absent is output-only)")
	}
}

func TestParseEngineExpectJSON(t *testing.T) {
	// json=<path>=<value> parses to Match="json", Path + Text split at the FIRST
	// '='; an IPv6-colon value is preserved because ':timeout=' is stripped first.
	r := parseCIRecord(t, `command=show rib
expect=output:json=0.prefix=172.16.0.0/16:timeout=6
expect=output:json=0.nexthop=2001:db8::1
`)
	want := []EngineStep{
		{Kind: EngineStepCommand, Text: "show rib"},
		{Kind: EngineStepExpectOutput, Match: "json", Path: "0.prefix", Text: "172.16.0.0/16", Timeout: 6 * time.Second},
		{Kind: EngineStepExpectOutput, Match: "json", Path: "0.nexthop", Text: "2001:db8::1", Timeout: engineStepDefaultTimeout},
	}
	if len(r.EngineSteps) != len(want) {
		t.Fatalf("EngineSteps len = %d, want %d (%+v)", len(r.EngineSteps), len(want), r.EngineSteps)
	}
	for i, w := range want {
		g := r.EngineSteps[i]
		if g.Kind != w.Kind || g.Match != w.Match || g.Path != w.Path || g.Text != w.Text || g.Timeout != w.Timeout {
			t.Errorf("step %d = %+v, want %+v", i, g, w)
		}
	}

	// A json= operand missing the path=value '=' must fail parse.
	et := &EncodingTests{}
	if err := et.parseLine(newRecord("json-novalue"), "test/engine/fake.ci", "expect=output:json=justapath"); err == nil {
		t.Error("expect=output:json= without a path=value '=' must fail parse")
	}
	// json= is output-only: expect=stream:json= must fail parse (AC-8).
	if err := et.parseLine(newRecord("stream-json"), "test/engine/fake.ci", "expect=stream:json=a=b"); err == nil {
		t.Error("expect=stream:json= must fail parse (json is output-only)")
	}
}

func TestParseEngineExpectContainsStillDefaults(t *testing.T) {
	// contains= keeps Match=="" (default) so existing serialized steps are
	// byte-identical (A-3): no "match"/"path" field is emitted.
	r := parseCIRecord(t, "command=show x\nexpect=output:contains=engine-running\n")
	if len(r.EngineSteps) != 2 {
		t.Fatalf("EngineSteps = %+v", r.EngineSteps)
	}
	g := r.EngineSteps[1]
	if g.Match != "" || g.Path != "" || g.Text != "engine-running" {
		t.Errorf("contains step = %+v, want Match=\"\" Path=\"\" Text=engine-running", g)
	}
}

func TestEngineStepsForRunWidensTimeoutsUnderParallel(t *testing.T) {
	// The executor's internal per-step polls (e.g. an establishment wait) must
	// get the same parallel headroom as the outer daemon budget, or they flake
	// under contention while the outer budget is fine. Serial runs are unchanged.
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "show x"},
		{Kind: EngineStepExpectOutput, Text: "y", Timeout: 10 * time.Second},
		{Kind: EngineStepExpectEvent, Namespace: "vpn-ipsec", Name: "sa-up", Timeout: 5 * time.Second},
	}

	serial := (&Runner{concurrency: 1}).engineStepsForRun(steps)
	if serial[1].Timeout != 10*time.Second || serial[2].Timeout != 5*time.Second {
		t.Errorf("serial run must not widen timeouts: %+v", serial)
	}

	par := (&Runner{concurrency: 4}).engineStepsForRun(steps)
	if par[1].Timeout != 10*time.Second*ParallelTimeoutHeadroom ||
		par[2].Timeout != 5*time.Second*ParallelTimeoutHeadroom {
		t.Errorf("parallel run must widen timeouts by %dx: %+v", ParallelTimeoutHeadroom, par)
	}
	// A command step carries no timeout (0) and stays 0; the original slice is
	// not mutated.
	if par[0].Timeout != 0 {
		t.Errorf("command step timeout must stay 0: %+v", par[0])
	}
	if steps[1].Timeout != 10*time.Second {
		t.Errorf("original steps must not be mutated: %+v", steps)
	}
}

func TestEngineStepsFileRoundTrip(t *testing.T) {
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "show vpn ipsec status"},
		{Kind: EngineStepExpectOutput, Text: "engine-running", Timeout: 5 * time.Second},
		{Kind: EngineStepExpectEvent, Namespace: "vpn-ipsec", Name: "sa-up", Timeout: 10 * time.Second},
	}
	data, err := marshalEngineSteps(steps)
	if err != nil {
		t.Fatalf("marshalEngineSteps: %v", err)
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
	// waitFrom(Len()) must ignore that old event and time out.
	pos := b.Len()
	got = b.waitFrom(t.Context(), pos, time.Now().Add(50*time.Millisecond), func(string) bool {
		return true
	})
	if got != "" {
		t.Fatalf("waitFrom(Len) = %q, want timeout (old events excluded)", got)
	}
	// A post-position delivery satisfies waitFrom.
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = b.OnEvent(`{"peer-name":"peer-2"}`)
	}()
	got = b.waitFrom(t.Context(), pos, time.Now().Add(time.Second), func(string) bool {
		return true
	})
	if got == "" {
		t.Fatal("waitFrom missed the post-position event")
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

func TestRunEngineStepsMatchesRegex(t *testing.T) {
	// The executor re-dispatches lastCommand until the regexp matches lastOutput.
	f := &fakeDispatch{responses: map[string][]string{
		"show rib": {
			`done {"engine-starting":true}`,
			`done {"engine-ready":true}`,
		},
	}}
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "show rib"},
		{Kind: EngineStepExpectOutput, Match: "matches", Text: "engine-(running|ready)", Timeout: 3 * time.Second},
	}
	if err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps); err != nil {
		t.Fatalf("RunEngineSteps: %v", err)
	}
	if len(f.calls) < 2 {
		t.Fatalf("calls = %v, want re-dispatch polling until the regexp matched", f.calls)
	}
}

func TestRunEngineStepsMatchesTimeoutNamesRegex(t *testing.T) {
	f := &fakeDispatch{responses: map[string][]string{
		"show rib": {`done {"engine-starting":true}`},
	}}
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "show rib"},
		{Kind: EngineStepExpectOutput, Match: "matches", Text: "engine-(running|ready)", Timeout: 400 * time.Millisecond},
	}
	err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps)
	if err == nil || !strings.Contains(err.Error(), "engine-(running|ready)") {
		t.Fatalf("RunEngineSteps = %v, want timeout error naming the regexp", err)
	}
}

func TestRunEngineStepsAbsent(t *testing.T) {
	// absent= passes only after a present->absent transition (R-1): the command
	// step sees the needle present, the executor re-dispatches until it is gone.
	f := &fakeDispatch{responses: map[string][]string{
		"show rib": {
			`done {"routes":["172.16.0.0/16"]}`,
			`done {"routes":[]}`,
		},
	}}
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "show rib"},
		{Kind: EngineStepExpectOutput, Match: "absent", Text: "172.16.0.0/16", Timeout: 3 * time.Second},
	}
	if err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps); err != nil {
		t.Fatalf("RunEngineSteps: %v", err)
	}
	if len(f.calls) < 2 {
		t.Fatalf("calls = %v, want a re-dispatch proving the present->absent transition", f.calls)
	}
}

func TestRunEngineStepsAbsentTimeout(t *testing.T) {
	// The needle never disappears -> timeout error naming the substring.
	f := &fakeDispatch{responses: map[string][]string{
		"show rib": {`done {"routes":["172.16.0.0/16"]}`},
	}}
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "show rib"},
		{Kind: EngineStepExpectOutput, Match: "absent", Text: "172.16.0.0/16", Timeout: 400 * time.Millisecond},
	}
	err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps)
	if err == nil || !strings.Contains(err.Error(), "172.16.0.0/16") {
		t.Fatalf("RunEngineSteps = %v, want timeout error naming the substring", err)
	}
}

func TestRunEngineStepsJSONPath(t *testing.T) {
	// json= walks the raw JSON data (not status+data) and compares the stringified
	// leaf. The array is empty first, then populated: the executor re-dispatches.
	f := &fakeDispatch{responses: map[string][]string{
		"show rib": {
			`done []`,
			`done [{"prefix":"172.16.0.0/16","nexthop":"10.0.0.2"}]`,
		},
	}}
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "show rib"},
		{Kind: EngineStepExpectOutput, Match: "json", Path: "0.prefix", Text: "172.16.0.0/16", Timeout: 3 * time.Second},
	}
	if err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps); err != nil {
		t.Fatalf("RunEngineSteps: %v", err)
	}
	if len(f.calls) < 2 {
		t.Fatalf("calls = %v, want re-dispatch polling until the path appeared", f.calls)
	}
}

func TestRunEngineStepsJSONPathTimeoutNamesPath(t *testing.T) {
	// A path that never resolves times out with an error naming the path + data.
	f := &fakeDispatch{responses: map[string][]string{
		"show rib": {`done [{"prefix":"10.0.0.0/24"}]`},
	}}
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "show rib"},
		{Kind: EngineStepExpectOutput, Match: "json", Path: "0.missing", Text: "x", Timeout: 400 * time.Millisecond},
	}
	err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps)
	if err == nil || !strings.Contains(err.Error(), "0.missing") {
		t.Fatalf("RunEngineSteps = %v, want timeout error naming the json path", err)
	}
}

func TestRunEngineStepsContainsUnchanged(t *testing.T) {
	// Match=="" and Match=="contains" both behave exactly as the legacy contains=.
	for _, match := range []string{"", "contains"} {
		f := &fakeDispatch{responses: map[string][]string{
			"show vpn ipsec status": {
				`done {"engine-starting":true}`,
				`done {"engine-running":true}`,
			},
		}}
		steps := []EngineStep{
			{Kind: EngineStepCommand, Text: "show vpn ipsec status"},
			{Kind: EngineStepExpectOutput, Match: match, Text: "engine-running", Timeout: 3 * time.Second},
		}
		if err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps); err != nil {
			t.Fatalf("RunEngineSteps(Match=%q): %v", match, err)
		}
	}
}

func TestRunEngineStepsStreamMatchesRegex(t *testing.T) {
	// expect=stream supports matches= over delivered stream events.
	f := &fakeDispatch{responses: map[string][]string{}}
	buf := NewEngineEventBuffer()
	steps := []EngineStep{
		{Kind: EngineStepStream, Text: "monitor vpn ipsec"},
		{Kind: EngineStepExpectStream, Match: "matches", Text: "child-(up|rekeyed)", Timeout: 2 * time.Second},
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = buf.OnEvent(`{"detail":"child-rekeyed esp"}`)
	}()
	if err := RunEngineSteps(t.Context(), f.dispatch, buf, steps); err != nil {
		t.Fatalf("RunEngineSteps: %v", err)
	}
}

func TestEngineJSONPathValue(t *testing.T) {
	// Direct coverage of the path walker, including the array-index boundary:
	// last valid index (len-1), invalid above (index == len), invalid below
	// (negative index) -- none may panic.
	cases := []struct {
		data, path, want string
		ok               bool
	}{
		{`[{"prefix":"p"}]`, "0.prefix", "p", true},
		{`{"count":3}`, "count", "3", true},          // number stringifies
		{`{"a":{"b":"c"}}`, "a.b", "c", true},        // nested object
		{`[1,2,3]`, "0", "1", true},                  // first valid index
		{`[1,2,3]`, "2", "3", true},                  // last valid index (len-1)
		{`[1,2,3]`, "3", "", false},                  // invalid above: index == len
		{`[1,2,3]`, "-1", "", false},                 // invalid below: negative index
		{`[{"prefix":"p"}]`, "0.missing", "", false}, // missing key
		{`not json`, "0", "", false},                 // non-JSON data
		{`{"ok":true}`, "ok", "true", true},          // bool stringifies
	}
	for _, c := range cases {
		got, ok := engineJSONPathValue(c.data, c.path)
		if got != c.want || ok != c.ok {
			t.Errorf("engineJSONPathValue(%q, %q) = (%q, %v), want (%q, %v)", c.data, c.path, got, ok, c.want, c.ok)
		}
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

// --- expect=command-error -------------------------------------------------
//
// The directive exists because the plugin SDK turns a StatusError response into
// a Go error, so a command that must REFUSE aborted the run before any expect=
// could see it. That made the whole refusal class untestable end to end, which is
// the class where being wrong is most expensive.

func TestParseEngineExpectCommandError(t *testing.T) {
	r := parseCIRecord(t, "command=show vpn ipsec dataplane sa\nexpect=command-error:contains=cannot enumerate\n")
	if len(r.EngineSteps) != 2 {
		t.Fatalf("EngineSteps = %d, want 2", len(r.EngineSteps))
	}
	step := r.EngineSteps[1]
	if step.Kind != EngineStepExpectCommandError {
		t.Errorf("Kind = %d, want EngineStepExpectCommandError", step.Kind)
	}
	if step.Text != "cannot enumerate" {
		t.Errorf("Text = %q, want the needle", step.Text)
	}
}

// The needle may itself hold ':' -- an error message routinely does.
func TestParseEngineExpectCommandErrorKeepsColons(t *testing.T) {
	r := parseCIRecord(t, "command=x\nexpect=command-error:contains=xfrm: state list: permission denied\n")
	if got := r.EngineSteps[1].Text; got != "xfrm: state list: permission denied" {
		t.Errorf("Text = %q, want the colons preserved", got)
	}
}

func TestParseEngineExpectCommandErrorRejectsEmpty(t *testing.T) {
	for _, line := range []string{
		"expect=command-error:contains=",
		"expect=command-error:matches=nope",
	} {
		et := &EncodingTests{}
		r := newRecord("engine-steps-test")
		if err := et.parseLine(r, "t.ci", line); err == nil {
			t.Errorf("parseLine(%q) = nil, want an error", line)
		}
	}
}

func TestRunEngineStepsCommandErrorConsumed(t *testing.T) {
	f := &fakeDispatch{responses: map[string][]string{
		"show vpn ipsec dataplane sa": {"ERR the active dataplane backend cannot enumerate the SAD"},
	}}
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "show vpn ipsec dataplane sa"},
		{Kind: EngineStepExpectCommandError, Text: "cannot enumerate the SAD"},
	}
	if err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps); err != nil {
		t.Fatalf("RunEngineSteps = %v, want the expected error to be consumed", err)
	}
}

func TestRunEngineStepsCommandErrorWrongMessage(t *testing.T) {
	f := &fakeDispatch{responses: map[string][]string{
		"cmd": {"ERR something else entirely"},
	}}
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "cmd"},
		{Kind: EngineStepExpectCommandError, Text: "cannot enumerate"},
	}
	err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps)
	if err == nil || !strings.Contains(err.Error(), "cannot enumerate") {
		t.Fatalf("RunEngineSteps = %v, want a failure naming the expected needle", err)
	}
}

// A command that SUCCEEDS must not satisfy expect=command-error. Without this the
// directive would pass for the very regression it guards: a refusal quietly
// becoming a successful empty answer.
func TestRunEngineStepsCommandErrorRequiresAFailure(t *testing.T) {
	f := &fakeDispatch{responses: map[string][]string{
		"cmd": {`done {"sas":[]}`},
	}}
	steps := []EngineStep{
		{Kind: EngineStepCommand, Text: "cmd"},
		{Kind: EngineStepExpectCommandError, Text: "cannot enumerate"},
	}
	err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps)
	if err == nil || !strings.Contains(err.Error(), "preceding command succeeded") {
		t.Fatalf("RunEngineSteps = %v, want a failure saying the command succeeded", err)
	}
}

// An UNCONSUMED command failure still fails the run. Every .ci written before
// this directive existed relies on that, so the held error must not become a
// swallowed one.
func TestRunEngineStepsUnconsumedCommandErrorStillFails(t *testing.T) {
	f := &fakeDispatch{responses: map[string][]string{
		"cmd": {"ERR boom"},
	}}
	t.Run("followed by another step", func(t *testing.T) {
		steps := []EngineStep{
			{Kind: EngineStepCommand, Text: "cmd"},
			{Kind: EngineStepExpectOutput, Text: "anything", Timeout: time.Second},
		}
		err := RunEngineSteps(t.Context(), f.dispatch, NewEngineEventBuffer(), steps)
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("RunEngineSteps = %v, want the command error raised", err)
		}
	})
	t.Run("as the last step", func(t *testing.T) {
		f2 := &fakeDispatch{responses: map[string][]string{"cmd": {"ERR boom"}}}
		steps := []EngineStep{{Kind: EngineStepCommand, Text: "cmd"}}
		err := RunEngineSteps(t.Context(), f2.dispatch, NewEngineEventBuffer(), steps)
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("RunEngineSteps = %v, want the trailing command error raised", err)
		}
	})
}

func TestEngineStepsCommandErrorRoundTrip(t *testing.T) {
	steps := []EngineStep{{Kind: EngineStepExpectCommandError, Text: "cannot enumerate"}}
	data, err := marshalEngineSteps(steps)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	back, err := UnmarshalEngineSteps(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back) != 1 || back[0].Kind != EngineStepExpectCommandError || back[0].Text != "cannot enumerate" {
		t.Fatalf("round trip = %+v, want the step preserved", back)
	}
}
