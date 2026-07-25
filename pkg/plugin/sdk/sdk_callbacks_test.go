package sdk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestOnExecuteCommandAnyMap(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", map[string]any{"running": true, "peers": 5}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Command: "test"})
	require.NoError(t, err)

	var out rpc.ExecuteCommandOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "done", out.Status)

	var data map[string]any
	require.NoError(t, json.Unmarshal(out.Data, &data))
	assert.Equal(t, true, data["running"])
	assert.Equal(t, float64(5), data["peers"])

	cancel()
	<-errCh
}

func TestOnExecuteCommandAnyStruct(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	type entry struct {
		Prefix  string `json:"prefix"`
		NextHop string `json:"next-hop"`
	}

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", []entry{{Prefix: "10.0.0.0/24", NextHop: "10.0.0.1"}}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Command: "show"})
	require.NoError(t, err)

	var out rpc.ExecuteCommandOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "done", out.Status)

	var entries []entry
	require.NoError(t, json.Unmarshal(out.Data, &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "10.0.0.0/24", entries[0].Prefix)

	cancel()
	<-errCh
}

func TestOnExecuteCommandAnyNil(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", nil, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Command: "noop"})
	require.NoError(t, err)

	var out rpc.ExecuteCommandOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "done", out.Status)
	assert.Empty(t, out.Data)

	cancel()
	<-errCh
}

func TestOnExecuteCommandAnySlice(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", []string{"cache", "route", "peer"}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Command: "events"})
	require.NoError(t, err)

	var out rpc.ExecuteCommandOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "done", out.Status)

	var events []string
	require.NoError(t, json.Unmarshal(out.Data, &events))
	assert.Equal(t, []string{"cache", "route", "peer"}, events)

	cancel()
	<-errCh
}

// TestOnExecuteCommandRawMessagePassthrough locks in the contract that a handler
// returning json.RawMessage is embedded verbatim (single marshal), not re-quoted.
// Pipeline terminals and the RPKI handler rely on this to ship hand-built JSON.
func TestOnExecuteCommandRawMessagePassthrough(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", json.RawMessage(`{"running":true,"peers":5}`), nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Command: "raw"})
	require.NoError(t, err)

	var out rpc.ExecuteCommandOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "done", out.Status)

	// Data is an object, NOT a JSON string -- the RawMessage passed through unquoted.
	var data map[string]any
	require.NoError(t, json.Unmarshal(out.Data, &data))
	assert.Equal(t, true, data["running"])

	cancel()
	<-errCh
}

// TestOnDoctorCheckCallback verifies the doctor-check callback dispatches correctly.
func TestOnDoctorCheckCallback(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnDoctorCheck(func(name string) ([]rpc.DoctorCheckDiagnostic, error) {
		return []rpc.DoctorCheckDiagnostic{
			{Code: "doctor-rpki-cache-unreachable", Severity: "warning", Message: "cache at 10.0.0.1:8282 not responding"},
		}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:doctor-check",
		rpc.DoctorCheckInput{Name: "rpki-cache-reachable"})
	require.NoError(t, err)

	var out rpc.DoctorCheckOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Len(t, out.Diagnostics, 1)
	assert.Equal(t, "doctor-rpki-cache-unreachable", out.Diagnostics[0].Code)
	assert.Equal(t, "warning", out.Diagnostics[0].Severity)

	cancel()
	<-errCh
}

// TestOnDoctorCheckDefaultNoOp verifies empty diagnostics when no handler registered.
func TestOnDoctorCheckDefaultNoOp(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:doctor-check",
		rpc.DoctorCheckInput{Name: "any-check"})
	require.NoError(t, err)

	var out rpc.DoctorCheckOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Empty(t, out.Diagnostics)

	cancel()
	<-errCh
}

// TestOnEnrichShowCallback verifies the enrich-show callback dispatches correctly.
//
// VALIDATES: AC-2 (external plugin receives base map, returns enrichment)
// PREVENTS: SDK callback wiring regression for enrich-show RPC.
func TestOnEnrichShowCallback(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnEnrichShow(func(command, key, mode string, base map[string]any) (map[string]any, error) {
		return map[string]any{
			"cos-profile": "residential",
			"speed":       float64(1000),
			"mode":        mode,
		}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:enrich-show",
		rpc.EnrichShowInput{
			Command: "show subscriber detail",
			Key:     "cos",
			Mode:    "detail",
			Base:    map[string]any{"id": "s1", "state": "active"},
		})
	require.NoError(t, err)

	var out rpc.EnrichShowOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "residential", out.Data["cos-profile"])
	assert.Equal(t, float64(1000), out.Data["speed"])
	assert.Equal(t, "detail", out.Data["mode"])

	cancel()
	<-errCh
}

// TestOnEnrichShowDefaultNoOp verifies empty data when no handler registered.
func TestOnEnrichShowDefaultNoOp(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:enrich-show",
		rpc.EnrichShowInput{
			Command: "show subscriber detail",
			Key:     "cos",
			Base:    map[string]any{"id": "s1"},
		})
	require.NoError(t, err)

	var out rpc.EnrichShowOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Empty(t, out.Data)

	cancel()
	<-errCh
}

// TestOnExecuteCommandStringIsDoubleEncoded documents the hazard that motivated
// the single-marshal sweep: a handler that returns a pre-marshaled JSON *string*
// (instead of a Go value) is re-quoted by the SDK, so the wire Data is a JSON
// string, not the intended object. Handlers must return structs/maps/RawMessage.
func TestOnExecuteCommandStringIsDoubleEncoded(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", `{"running":true}`, nil // BUG shape: pre-marshaled string
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Command: "str"})
	require.NoError(t, err)

	var out rpc.ExecuteCommandOutput
	require.NoError(t, json.Unmarshal(raw, &out))

	// Data decodes to a STRING, not an object -- proving the double-encode.
	// A handler must NOT return a JSON string; this asserts the failure mode.
	var asString string
	require.NoError(t, json.Unmarshal(out.Data, &asString))
	assert.Equal(t, `{"running":true}`, asString)

	var asObject map[string]any
	assert.Error(t, json.Unmarshal(out.Data, &asObject),
		"a pre-marshaled string must not decode as an object (it is double-encoded)")

	cancel()
	<-errCh
}
