// Design: ai/rules/cli.md -- "Every JSON key MUST be lowercase kebab-case
// matching its YANG leaf".
//
// The plugin API module declares each RPC's output leaves, and a client that
// reads the model builds its decoder from those declarations. A handler that
// writes a key the model does not declare, or a value of another type than the
// leaf declares, hands that client a payload its decoder refuses. Until
// 2026-09-15 session-ping declared `pong` as a boolean and wrote the daemon pid
// into it (plan/journal/declared-format-contradicts-payload.md).
//
// The check runs the real path: the handler registered in session.go, reached
// through the Dispatcher the daemon loads its builtins into, and the schema the
// same loader reads. A test over the handler alone cannot see the leaf, and a
// test over the schema alone cannot see the payload.
package server_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	// Trigger every module and RPC registration, so the dispatcher under test
	// holds the handler the daemon holds.
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// rpcOutputLeaves answers the declared output leaves of one RPC, by name.
func rpcOutputLeaves(t *testing.T, loader *yang.Loader, module, rpc string) map[string]yang.LeafMeta {
	t.Helper()
	for _, meta := range yang.ExtractRPCs(loader, module) {
		if meta.Name != rpc {
			continue
		}
		leaves := make(map[string]yang.LeafMeta, len(meta.Output))
		for _, leaf := range meta.Output {
			leaves[leaf.Name] = leaf
		}
		return leaves
	}
	t.Fatalf("%s declares no rpc %q", module, rpc)
	return nil
}

// yangTypeAccepts reports whether a decoded JSON value is one the YANG type
// names. The decoder ran with UseNumber, so a JSON number arrives as
// json.Number and its integer range can be checked. A type this table does
// not know is an error, never a pass: a silent accept is the defect this test
// exists to catch.
func yangTypeAccepts(yangType string, value any) error {
	switch yangType {
	case "boolean":
		if _, ok := value.(bool); ok {
			return nil
		}
	case "string":
		if _, ok := value.(string); ok {
			return nil
		}
	case "uint8", "uint16", "uint32", "uint64":
		number, ok := value.(json.Number)
		if !ok {
			break
		}
		bits, _ := strconv.Atoi(strings.TrimPrefix(yangType, "uint"))
		if _, err := strconv.ParseUint(number.String(), 10, bits); err == nil {
			return nil
		}
	case "int8", "int16", "int32", "int64":
		number, ok := value.(json.Number)
		if !ok {
			break
		}
		bits, _ := strconv.Atoi(strings.TrimPrefix(yangType, "int"))
		if _, err := strconv.ParseInt(number.String(), 10, bits); err == nil {
			return nil
		}
	default:
		return fmt.Errorf("YANG type %q is not one this test knows how to judge", yangType)
	}
	return fmt.Errorf("value %v (%T) is not a YANG %s", value, value, yangType)
}

// TestSessionPingAnswersTheDeclaredOutputLeaves proves the session-ping payload
// is the one ze-plugin-api.yang declares.
//
// VALIDATES: every key the handler writes is a declared output leaf, every
// declared output leaf is written, and each value is of the leaf's type.
// PREVENTS: a boolean leaf carrying an integer on the wire, which a client
// generated from the model cannot decode.
func TestSessionPingAnswersTheDeclaredOutputLeaves(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load the embedded YANG modules")
	leaves := rpcOutputLeaves(t, loader, "ze-plugin-api", "session-ping")
	require.NotEmpty(t, leaves, "session-ping declares no output leaf")

	dispatcher := pluginserver.NewDispatcher()
	pluginserver.LoadBuiltins(dispatcher, yang.WireMethodToPath(loader),
		yang.PathToDescription(loader), yang.PathToHelp(loader), yang.PathToArgDefs(loader))

	resp, err := dispatcher.Dispatch(&pluginserver.CommandContext{}, "plugin session ping")
	require.NoError(t, err)
	payload, err := plugin.ResponseJSON(resp, nil)
	require.NoError(t, err)

	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()
	var answer map[string]any
	require.NoError(t, decoder.Decode(&answer), "payload %s", payload)

	for name, value := range answer {
		leaf, declared := leaves[name]
		if !declared {
			t.Errorf("key %q is answered and ze-plugin-api.yang declares no such output leaf on session-ping", name)
			continue
		}
		if typeErr := yangTypeAccepts(leaf.Type, value); typeErr != nil {
			t.Errorf("leaf %q: %v", name, typeErr)
		}
	}
	for name := range leaves {
		if _, answered := answer[name]; !answered {
			t.Errorf("leaf %q is declared and the handler did not answer it", name)
		}
	}
}
