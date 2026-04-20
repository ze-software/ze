package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockSink is a test-only replySink that captures frames in memory.
// IsSSE starts false and flips true after UpgradeToSSE so Elicit's
// upgrade path exercises the normal sequence.
type mockSink struct {
	mu       sync.Mutex
	frames   [][]byte
	sse      bool
	writeErr error
}

func (m *mockSink) WriteFrame(frame []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	// Copy so later mutations to the caller's buffer don't racily alter
	// recorded frames — paranoia, tests currently pass fresh marshals.
	cp := make([]byte, len(frame))
	copy(cp, frame)
	m.frames = append(m.frames, cp)
	return nil
}

func (m *mockSink) IsSSE() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sse
}

func (m *mockSink) UpgradeToSSE() (replySink, error) {
	m.mu.Lock()
	m.sse = true
	m.mu.Unlock()
	return m, nil
}

func (m *mockSink) Frames() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]byte, len(m.frames))
	copy(out, m.frames)
	return out
}

// newElicitTestSession returns a session wired up for Elicit tests:
// clientElicit=true, a mockSink already bound as the active POST sink.
// Caller MUST call the returned cleanup to clear the sink before the
// session is released back to the registry.
func newElicitTestSession(t *testing.T, r *sessionRegistry) (*session, *mockSink, func()) {
	t.Helper()
	sess, err := r.CreateWithCapabilities(ProtocolVersion, Identity{}, true)
	if err != nil {
		t.Fatalf("CreateWithCapabilities: %v", err)
	}
	sink := &mockSink{}
	release, err := sess.SetActivePostSink(sink)
	if err != nil {
		t.Fatalf("SetActivePostSink: %v", err)
	}
	return sess, sink, release
}

// newElicitTestRegistry returns a sessionRegistry suitable for short-lived
// elicit tests. Caller MUST call Close.
func newElicitTestRegistry(t *testing.T) *sessionRegistry {
	t.Helper()
	r := newSessionRegistry(time.Minute, 0, 8)
	t.Cleanup(r.Close)
	return r
}

// validFlatSchema returns a minimal schema the validator accepts.
func validFlatSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"answer": map[string]any{"type": "string"},
		},
	}
}

// VALIDATES: AC-15 — a requestedSchema that is not an object at the root is rejected.
// PREVENTS: servers passing a bare primitive as the schema and the client seeing a malformed JSON-RPC request.
func TestElicit_SchemaRejectsNonObjectRoot(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]any
	}{
		{"missing type", map[string]any{"properties": map[string]any{}}},
		{"type=string at root", map[string]any{"type": "string"}},
		{"type=array at root", map[string]any{"type": "array"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateElicitSchema(tt.schema); !errors.Is(err, ErrElicitSchemaInvalid) {
				t.Fatalf("want ErrElicitSchemaInvalid, got %v", err)
			}
		})
	}
}

// VALIDATES: AC-15 — a nested object property is rejected.
// PREVENTS: the flat-schema rule silently permitting one level of nesting.
func TestElicit_SchemaRejectsNestedObject(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"profile": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
	}
	err := validateElicitSchema(schema)
	if !errors.Is(err, ErrElicitSchemaInvalid) {
		t.Fatalf("want ErrElicitSchemaInvalid, got %v", err)
	}
	if !strings.Contains(err.Error(), "profile") {
		t.Errorf("error should name the offending path; got %v", err)
	}
}

// VALIDATES: AC-15 — an array-of-object property is rejected.
// PREVENTS: accepting an arrays-of-primitives gotcha that the spec also forbids.
func TestElicit_SchemaRejectsArray(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tags": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
	if err := validateElicitSchema(schema); !errors.Is(err, ErrElicitSchemaInvalid) {
		t.Fatalf("want ErrElicitSchemaInvalid, got %v", err)
	}
}

// VALIDATES: AC-15 — oneOf/allOf/anyOf at the property level are rejected.
// PREVENTS: JSON-Schema features the spec intentionally excludes.
func TestElicit_SchemaRejectsComposition(t *testing.T) {
	for _, kw := range []string{"oneOf", "allOf", "anyOf", "$ref", "not"} {
		t.Run(kw, func(t *testing.T) {
			schema := map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x": map[string]any{kw: []any{map[string]any{"type": "string"}}},
				},
			}
			if err := validateElicitSchema(schema); !errors.Is(err, ErrElicitSchemaInvalid) {
				t.Fatalf("want ErrElicitSchemaInvalid for %s, got %v", kw, err)
			}
		})
	}
}

// VALIDATES: string primitives with every supported format pass the validator.
// PREVENTS: the validator being too strict and rejecting legitimate schemas.
func TestElicit_SchemaAllowsString(t *testing.T) {
	for _, format := range []string{"", "email", "uri", "date", "date-time"} {
		t.Run("format="+format, func(t *testing.T) {
			prop := map[string]any{
				"type":        "string",
				"title":       "Display",
				"description": "desc",
				"minLength":   float64(1),
				"maxLength":   float64(50),
			}
			if format != "" {
				prop["format"] = format
			}
			schema := map[string]any{
				"type":       "object",
				"properties": map[string]any{"name": prop},
				"required":   []any{"name"},
			}
			if err := validateElicitSchema(schema); err != nil {
				t.Fatalf("unexpected rejection: %v", err)
			}
		})
	}
}

// VALIDATES: number / integer primitives with min/max pass the validator.
// PREVENTS: the validator confusing JSON number with JSON integer.
func TestElicit_SchemaAllowsNumber(t *testing.T) {
	for _, typ := range []string{"number", "integer"} {
		t.Run(typ, func(t *testing.T) {
			schema := map[string]any{
				"type": "object",
				"properties": map[string]any{
					"age": map[string]any{
						"type":    typ,
						"minimum": float64(0),
						"maximum": float64(120),
					},
				},
			}
			if err := validateElicitSchema(schema); err != nil {
				t.Fatalf("%s: %v", typ, err)
			}
		})
	}
}

// VALIDATES: boolean primitive with a default passes the validator.
// PREVENTS: rejecting default values, which the spec explicitly supports.
func TestElicit_SchemaAllowsBoolean(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"enabled": map[string]any{
				"type":    "boolean",
				"default": false,
			},
		},
	}
	if err := validateElicitSchema(schema); err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
}

// VALIDATES: enum with enumNames passes the validator.
// PREVENTS: rejecting the enum pattern the spec explicitly supports.
func TestElicit_SchemaAllowsEnum(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"choice": map[string]any{
				"type":      "string",
				"enum":      []any{"a", "b", "c"},
				"enumNames": []any{"Alpha", "Bravo", "Charlie"},
			},
		},
	}
	if err := validateElicitSchema(schema); err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
}

// VALIDATES: unknown primitive type (e.g. "null") is rejected.
// PREVENTS: schema types the spec does not list slipping through.
func TestElicit_SchemaRejectsUnknownType(t *testing.T) {
	for _, typ := range []string{"null", "", "object", "array"} {
		t.Run(typ, func(t *testing.T) {
			schema := map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x": map[string]any{"type": typ},
				},
			}
			if err := validateElicitSchema(schema); !errors.Is(err, ErrElicitSchemaInvalid) {
				t.Fatalf("want ErrElicitSchemaInvalid for type=%q, got %v", typ, err)
			}
		})
	}
}

// VALIDATES: nil / empty schema is rejected — elicitation/create requires a schema.
// PREVENTS: a caller passing a nil map and the server sending an empty requestedSchema.
func TestElicit_SchemaRejectsEmpty(t *testing.T) {
	if err := validateElicitSchema(nil); !errors.Is(err, ErrElicitSchemaInvalid) {
		t.Fatalf("want ErrElicitSchemaInvalid for nil, got %v", err)
	}
	if err := validateElicitSchema(map[string]any{"type": "object"}); !errors.Is(err, ErrElicitSchemaInvalid) {
		t.Fatalf("want ErrElicitSchemaInvalid for missing properties, got %v", err)
	}
}

// VALIDATES: enum on a non-string-typed property is rejected (MCP spec
// illustrates enum only under type=string).
// PREVENTS: a caller writing {"type":"number","enum":[1,2]} expecting
// it to mean "pick one of these numbers" -- the spec shape is the
// string+enumNames form and the validator must catch the mistake.
func TestElicit_SchemaRejectsEnumOnNonString(t *testing.T) {
	for _, typ := range []string{"number", "integer", "boolean"} {
		t.Run(typ, func(t *testing.T) {
			schema := map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x": map[string]any{
						"type": typ,
						"enum": []any{1, 2, 3},
					},
				},
			}
			if err := validateElicitSchema(schema); !errors.Is(err, ErrElicitSchemaInvalid) {
				t.Fatalf("want ErrElicitSchemaInvalid for enum on type=%q, got %v", typ, err)
			}
		})
	}
}

// VALIDATES: CreateWithCapabilities sets the clientElicit flag on the
// returned session. Regression guard -- earlier drafts mutated the field
// after insertion into the registry, opening a race window where a GET
// on the session-id could observe clientElicit=false briefly.
// PREVENTS: a future refactor moving the capability behind a setter (or
// into a separate capability struct) silently dropping the flag.
func TestRegistry_CreateWithCapabilities(t *testing.T) {
	r := newElicitTestRegistry(t)
	sess, err := r.CreateWithCapabilities(ProtocolVersion, Identity{}, true)
	if err != nil {
		t.Fatalf("CreateWithCapabilities: %v", err)
	}
	if !sess.ClientSupportsElicit() {
		t.Errorf("ClientSupportsElicit() = false, want true")
	}
	// Same session id is reachable via registry.Get and carries the flag.
	got, ok := r.Get(sess.ID())
	if !ok {
		t.Fatalf("Get(sess.ID()) missing")
	}
	if !got.ClientSupportsElicit() {
		t.Errorf("registry.Get(id).ClientSupportsElicit() = false, want true")
	}
	// Zero (false) case for symmetry.
	sess2, err := r.CreateWithCapabilities(ProtocolVersion, Identity{}, false)
	if err != nil {
		t.Fatalf("CreateWithCapabilities(false): %v", err)
	}
	if sess2.ClientSupportsElicit() {
		t.Errorf("ClientSupportsElicit() = true, want false on clientElicit=false")
	}
}

// VALIDATES: AC-15a — handler calls Elicit without the client having
// declared the elicitation capability.
// PREVENTS: servers silently sending elicitation/create in violation of
// the spec's client-consent MUST.
func TestElicit_NoCapabilityReturnsUnsupported(t *testing.T) {
	r := newElicitTestRegistry(t)
	sess, err := r.Create(ProtocolVersion, Identity{}) // no capability
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sink := &mockSink{}
	release, err := sess.SetActivePostSink(sink)
	if err != nil {
		t.Fatalf("SetActivePostSink: %v", err)
	}
	defer release()

	_, err = sess.Elicit(context.Background(), "hello", validFlatSchema())
	if !errors.Is(err, ErrElicitUnsupported) {
		t.Fatalf("want ErrElicitUnsupported, got %v", err)
	}
	if n := len(sink.Frames()); n != 0 {
		t.Fatalf("no frame should be sent when capability missing; got %d frames", n)
	}
	if sess.PendingElicitCount() != 0 {
		t.Fatalf("no correlation should be registered; count=%d", sess.PendingElicitCount())
	}
}

// VALIDATES: AC-13 — client accept returns the content map.
// PREVENTS: the content field being dropped, mis-parsed, or the channel
// write racing the map delete.
func TestElicit_AcceptReturnsContent(t *testing.T) {
	r := newElicitTestRegistry(t)
	sess, sink, release := newElicitTestSession(t, r)
	defer release()

	type result struct {
		content map[string]any
		err     error
	}
	done := make(chan result, 1)
	go func() {
		c, e := sess.Elicit(context.Background(), "Which command?", validFlatSchema())
		done <- result{content: c, err: e}
	}()

	// Wait for the frame to be written — Elicit is in its select by then.
	id := waitForElicitID(t, sink)
	if !sess.ResolveElicit(id, elicitResponse{
		Action:  elicitActionAccept,
		Content: map[string]any{"answer": "show bgp summary"},
	}) {
		t.Fatalf("ResolveElicit returned false for known id %q", id)
	}

	got := <-done
	if got.err != nil {
		t.Fatalf("Elicit: %v", got.err)
	}
	if got.content["answer"] != "show bgp summary" {
		t.Fatalf("content[answer] = %v, want 'show bgp summary'", got.content["answer"])
	}
	if sess.PendingElicitCount() != 0 {
		t.Fatalf("correlation not cleaned up; count=%d", sess.PendingElicitCount())
	}
}

// VALIDATES: AC-14 — client decline returns ErrElicitDeclined, content nil.
// PREVENTS: a decline path accidentally propagating a zero-value content
// map that a handler might treat as "accepted empty".
func TestElicit_DeclineSentinel(t *testing.T) {
	r := newElicitTestRegistry(t)
	sess, sink, release := newElicitTestSession(t, r)
	defer release()

	done := make(chan error, 1)
	go func() {
		_, e := sess.Elicit(context.Background(), "?", validFlatSchema())
		done <- e
	}()
	id := waitForElicitID(t, sink)
	sess.ResolveElicit(id, elicitResponse{Action: elicitActionDecline})
	if err := <-done; !errors.Is(err, ErrElicitDeclined) {
		t.Fatalf("want ErrElicitDeclined, got %v", err)
	}
}

// VALIDATES: AC-14 — client cancel returns ErrElicitCanceled.
// PREVENTS: decline and cancel being conflated; handler fallback logic
// depends on distinguishing explicit-reject from dismiss.
func TestElicit_CancelSentinel(t *testing.T) {
	r := newElicitTestRegistry(t)
	sess, sink, release := newElicitTestSession(t, r)
	defer release()

	done := make(chan error, 1)
	go func() {
		_, e := sess.Elicit(context.Background(), "?", validFlatSchema())
		done <- e
	}()
	id := waitForElicitID(t, sink)
	sess.ResolveElicit(id, elicitResponse{Action: elicitActionCancel})
	if err := <-done; !errors.Is(err, ErrElicitCanceled) {
		t.Fatalf("want ErrElicitCanceled, got %v", err)
	}
}

// VALIDATES: AC-15c — ctx cancel while suspended releases the correlation.
// PREVENTS: a goroutine leak when the client disconnects mid-elicit.
func TestElicit_ContextCancelDropsPending(t *testing.T) {
	r := newElicitTestRegistry(t)
	sess, sink, release := newElicitTestSession(t, r)
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, e := sess.Elicit(ctx, "?", validFlatSchema())
		done <- e
	}()
	waitForElicitID(t, sink)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Elicit did not return after ctx cancel")
	}
	if sess.PendingElicitCount() != 0 {
		t.Fatalf("correlation leaked after ctx cancel; count=%d", sess.PendingElicitCount())
	}
}

// VALIDATES: AC-15b — a response POST for an unknown id is silently dropped.
// PREVENTS: stale-reply DoS, reply-forgery panics, map corruption.
func TestElicit_UnknownIDIgnored(t *testing.T) {
	r := newElicitTestRegistry(t)
	sess, _, release := newElicitTestSession(t, r)
	defer release()

	// No pending elicit. ResolveElicit should return false and not panic.
	if sess.ResolveElicit("nonexistent-id", elicitResponse{Action: elicitActionAccept}) {
		t.Fatalf("ResolveElicit returned true for unknown id")
	}
	if sess.PendingElicitCount() != 0 {
		t.Fatalf("map perturbed by unknown-id resolve; count=%d", sess.PendingElicitCount())
	}
}

// VALIDATES: pending-elicit cap rejects a register beyond maxPendingElicits.
// PREVENTS: unbounded correlation-map growth from a malicious or stuck flow.
func TestElicit_PendingCapRejects(t *testing.T) {
	r := newElicitTestRegistry(t)
	sess, _, release := newElicitTestSession(t, r)
	defer release()

	for i := range maxPendingElicits {
		if _, _, err := sess.RegisterElicit(); err != nil {
			t.Fatalf("RegisterElicit %d: %v", i, err)
		}
	}
	if _, _, err := sess.RegisterElicit(); !errors.Is(err, ErrElicitTooMany) {
		t.Fatalf("want ErrElicitTooMany after cap, got %v", err)
	}
}

// VALIDATES: the emitted frame is a well-formed JSON-RPC request for
// elicitation/create with the given message and schema.
// PREVENTS: shape regressions (missing jsonrpc field, wrong method name,
// params key mis-cased) that would break every client.
func TestElicit_FrameShape(t *testing.T) {
	r := newElicitTestRegistry(t)
	sess, sink, release := newElicitTestSession(t, r)
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() {
		_, err := sess.Elicit(ctx, "Prompt", validFlatSchema())
		done <- err
	}()
	id := waitForElicitID(t, sink)
	// Cancel so the goroutine exits; the returned error is the expected
	// ctx-canceled path, not a test failure.
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ctx.Canceled after cancel; got %v", err)
	}

	frames := sink.Frames()
	if len(frames) != 1 {
		t.Fatalf("want 1 frame, got %d", len(frames))
	}
	var parsed map[string]any
	if err := json.Unmarshal(frames[0], &parsed); err != nil {
		t.Fatalf("frame not valid JSON: %v", err)
	}
	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", parsed["jsonrpc"])
	}
	if parsed["method"] != "elicitation/create" {
		t.Errorf("method = %v, want elicitation/create", parsed["method"])
	}
	if parsed["id"] != id {
		t.Errorf("id = %v, want %s", parsed["id"], id)
	}
	params, ok := parsed["params"].(map[string]any)
	if !ok {
		t.Fatalf("params not a map")
	}
	if params["message"] != "Prompt" {
		t.Errorf("message = %v, want Prompt", params["message"])
	}
	if _, hasSchema := params["requestedSchema"]; !hasSchema {
		t.Errorf("requestedSchema missing")
	}
}

// waitForElicitID polls the sink until exactly one frame has been written,
// parses the JSON-RPC id, and returns it. Fails the test after a bounded
// wait so a broken Elicit does not hang indefinitely.
func waitForElicitID(t *testing.T, sink *mockSink) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		frames := sink.Frames()
		if len(frames) >= 1 {
			var parsed map[string]any
			if err := json.Unmarshal(frames[0], &parsed); err != nil {
				t.Fatalf("frame parse: %v", err)
			}
			id, ok := parsed["id"].(string)
			if !ok {
				t.Fatalf("id not a string: %v", parsed["id"])
			}
			return id
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no frame arrived within 2s")
	return ""
}
