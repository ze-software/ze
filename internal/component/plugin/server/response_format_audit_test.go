// Design: ai/rules/cli.md -- "A command's response payload MUST be structured
// data. It MUST NOT be text a renderer already formatted."
//
// The rule's other half, "every command that produces output MUST support all
// pipe operators", was already satisfiable by a handler that answered with
// finished text: `| json`, `| yaml` and `| table` each returned that same text.
// The type system closes most of the route, because ResponseData
// (internal/component/plugin/types.go) refuses a bare string in Response.Data.
// It does not close all of it: RawJSON satisfies the interface, and a payload
// that marshals to a JSON STRING is valid JSON that every renderer prints back
// unchanged.
//
// So the invariant this holds is narrower than "is it JSON" and is the one that
// actually bites: the payload's top-level JSON value is an OBJECT or an ARRAY.
// A scalar has no fields for `| table` to column and no mapping for `| yaml` to
// indent, so applyYAML (internal/component/command/pipe.go) and
// applyTableStyled (pipe_table.go) both fall through to printing the scalar
// back.
//
// The walk is scoped to the `ze-show:` namespace, and the scope is load-bearing
// rather than a convenience. Judging a payload means CALLING the handler, and a
// handler is free to act: handleDaemonQuit (system.go) is a registered RPC that
// quits the daemon, and invoking the whole registry runs it. `show` is the read
// verb by this repo's own grammar rule ("Choosing the verb", ai/rules/cli.md),
// so the namespace is the population that can be called for its answer alone.
// It is 190 of about 380 registered wire methods.
package server_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	// Trigger every builtin RPC init() registration, matching the composition
	// root the running daemon assembles (mirrors all_import_test.go).
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// readVerbNamespace is the wire-method prefix of the read verb. See the package
// comment: the walk calls handlers, so it may only call the ones whose verb
// says they answer a question.
const readVerbNamespace = "ze-show:"

// formatOperators are the three renderings of one payload that ai/rules/cli.md
// names. Each takes the JSON string ResponseJSON produced.
var formatOperators = []string{"json", "yaml", "table"}

// invokeOffline runs one handler against a zero CommandContext and reports
// whether it survived. Handlers that dereference a context field directly
// (ctx.Server.ProcessManager(), ctx.Process.SetEncoding) panic on a zero one,
// so a panic is a classification here rather than a failure: it says the
// command needs a daemon and is out of this tier's reach.
func invokeOffline(h pluginserver.Handler) (resp *plugin.Response, survived bool) {
	defer func() {
		if recover() != nil {
			resp, survived = nil, false
		}
	}()
	r, err := h(&pluginserver.CommandContext{}, nil)
	if err != nil {
		return nil, true
	}
	return r, true
}

// decodeStructured decodes a payload and holds the invariant: its top-level
// JSON value is an object or an array. A scalar is the shape a pre-rendered
// text answer takes once RawJSON has carried it through Response.Data, and it
// is valid JSON, so json.Valid cannot see it.
func decodeStructured(payload string) (any, error) {
	var top any
	if err := json.Unmarshal([]byte(payload), &top); err != nil {
		return nil, fmt.Errorf("payload is not JSON: %w", err)
	}
	switch top.(type) {
	case map[string]any, []any:
		return top, nil
	default:
		return nil, fmt.Errorf("payload's top-level JSON value is %T, so `| table` has "+
			"no fields to column and `| yaml` has no mapping to indent", top)
	}
}

// hasContent reports whether a decoded payload carries anything to render. An
// empty object or array is a legitimate answer (no peers, no routes), and every
// renderer correctly produces nothing for it, so it earns no output assertion.
func hasContent(top any) bool {
	switch v := top.(type) {
	case map[string]any:
		return len(v) > 0
	case []any:
		return len(v) > 0
	default:
		return false
	}
}

// census counts what happened to each command, so a shrinking reachable set is
// visible instead of reading as full coverage.
type census struct {
	needsDaemon int // panicked on a zero context
	noPayload   int // error, StatusError, or nil Data
	judged      int // produced a payload this test could judge
}

// PREVENTS: a `show` command shipping a response payload that `| json`,
//
//	`| yaml` and `| table` cannot each render, which is what a pre-rendered
//	text payload is. The type system refuses a bare string; this refuses the
//	scalar that reaches Response.Data through RawJSON.
func TestShowCommandPayloadsAreStructured(t *testing.T) {
	var c census

	for _, reg := range pluginserver.AllBuiltinRPCs() {
		if !strings.HasPrefix(reg.WireMethod, readVerbNamespace) {
			continue
		}

		resp, survived := invokeOffline(reg.Handler)
		if !survived {
			c.needsDaemon++
			continue
		}

		payload, err := plugin.ResponseJSON(resp, nil)
		if err != nil || payload == "" {
			c.noPayload++
			continue
		}
		c.judged++

		top, err := decodeStructured(payload)
		if err != nil {
			t.Errorf("%s: %v; answer with plugin.Map, plugin.Slice or a struct "+
				"embedding plugin.DataMarker (ai/rules/cli.md)", reg.WireMethod, err)
			continue
		}

		for _, op := range formatOperators {
			_, format, errMsg := command.ProcessPipesDefaultFormatChecked(reg.WireMethod+" | "+op, "")
			if errMsg != "" {
				t.Errorf("%s | %s: pipe chain rejected: %s", reg.WireMethod, op, errMsg)
				continue
			}
			if out := format(payload); out == "" && hasContent(top) {
				t.Errorf("%s | %s rendered nothing from a payload that carries content",
					reg.WireMethod, op)
			}
		}
	}

	t.Logf("payload census: %d judged, %d no payload, %d need a daemon",
		c.judged, c.noPayload, c.needsDaemon)

	// Non-vacuous: the walk must reach a real population. An empty
	// AllBuiltinRPCs(), a renamed namespace, or a change that makes every
	// handler panic on a zero context would otherwise pass having judged
	// nothing. The floor sits below both builds this runs in: 56 payloads are
	// judged under the feature tags `make` passes, and 38 under a bare
	// `go test`, which compiles fewer commands in.
	if c.judged < 30 {
		t.Fatalf("judged too few show-command payloads (%d); builtin registration is "+
			"broken, the %q namespace has moved, or the offline-invokable population "+
			"has collapsed", c.judged, readVerbNamespace)
	}
}

// PREVENTS: the audit above passing vacuously. It proves the check REFUSES the
//
//	payload shape it exists to catch, and that the three operators really do
//	give a reader nothing when they meet one.
func TestPreRenderedTextPayloadIsRefused(t *testing.T) {
	// A quoted string is what finished text looks like once it is JSON. It is
	// the shape RawJSON's own json.Valid guard cannot see, because the bytes
	// are a valid JSON value. Only the top-level shape says no.
	const text = `"bgp keepalive, message direction received"`

	payload, err := plugin.ResponseJSON(
		plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(text)), nil)
	require.NoError(t, err, "a JSON string passes RawJSON's validity guard")
	require.Equal(t, text, payload)

	_, err = decodeStructured(payload)
	require.Error(t, err, "decodeStructured must refuse a scalar payload")

	// And this is why it matters: every operator hands the reader the same text
	// back, so `| json`, `| yaml` and `| table` stop being three renderings.
	for _, op := range formatOperators {
		_, format, errMsg := command.ProcessPipesDefaultFormatChecked("x | "+op, "")
		require.Empty(t, errMsg)
		require.Contains(t, format(payload), "bgp keepalive",
			"`| %s` renders the text back rather than a structure", op)
	}

	// The control: the same three operators over a structured payload of the
	// same facts pass the check.
	structured, err := plugin.ResponseJSON(
		plugin.NewResponse(plugin.StatusDone,
			plugin.Map{"message": "keepalive", "direction": "received"}), nil)
	require.NoError(t, err)
	_, err = decodeStructured(structured)
	require.NoError(t, err, "a plugin.Map payload passes")
}
