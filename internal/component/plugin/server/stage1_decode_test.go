package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// refusedDeclaration is one Stage 1 declaration the decoder refuses, and the
// parts of the operator error it must answer.
type refusedDeclaration struct {
	name   string
	params string
	want   []string
}

// refusedDeclarations are the declarations both decoder entries refuse. Each
// declares protocol version 1, so the refusal is the decoder's and never the
// version check's.
var refusedDeclarations = []refusedDeclaration{
	{
		name:   "the retired schema key",
		params: `{"protocol-version":1,"schema":{"module":"acme-conf","namespace":"urn:acme","yang-text":"module acme-conf {}"}}`,
		want: []string{
			`sent its YANG module under "schema", a key ze no longer reads`,
			`fix: send "schemas":[{"module":...,"namespace":...,"yang-text":...}] in place of "schema":{...}`,
			rpc.DeclarationSection,
		},
	},
	{
		name:   "an unknown top-level key",
		params: `{"protocol-version":1,"shade":"blue"}`,
		want: []string{
			"sent a Stage 1 declaration that carries a key ze does not know",
			"fix: remove the key, or spell it as docs/plugin-development/protocol.md lists the keys",
			rpc.DeclarationSection,
		},
	},
	{
		name:   "an unknown key inside a module entry",
		params: `{"protocol-version":1,"schemas":[{"module":"acme-conf","namespace":"urn:acme","yang-text":"module acme-conf {}","revision":"2026-01-01"}]}`,
		want: []string{
			"sent a Stage 1 declaration that carries a key ze does not know",
			rpc.DeclarationSection,
		},
	},
}

// TestStage1HandshakeRefusesUndecodableDeclaration proves the startup
// handshake refuses a declaration that carries the retired "schema" key or
// any other key ze does not know: the handshake fails with the operator
// error naming the plugin and the fix, the plugin reads that same text as its
// Stage 1 answer, and no stage hook runs, so the sink publishes no module.
//
// Method: a real PluginConn over a pipe, the plugin end sending the raw JSON
// a plugin in another language would write.
func TestStage1HandshakeRefusesUndecodableDeclaration(t *testing.T) {
	t.Parallel()

	for _, tc := range refusedDeclarations {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			engineConn, pluginMux := newDriverPipe(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			callErr := make(chan error, 1)
			go func() {
				_, err := pluginMux.CallRPC(ctx, rpc.MethodDeclareRegistration, json.RawMessage(tc.params))
				callErr <- err
			}()

			sink := &recordingSink{pc: engineConn}
			err := runStartupHandshake(ctx, sink)
			require.Error(t, err)
			_, refused := errors.AsType[*opererr.Error](err)
			assert.True(t, refused, "the refusal is an operator error, got %T: %v", err, err)
			assert.Contains(t, err.Error(), `plugin "widget"`)
			for _, want := range tc.want {
				assert.Contains(t, err.Error(), want)
			}
			assert.NotContains(t, err.Error(), "declared no protocol version", "the decoder refuses it, not the version check")

			select {
			case answerErr := <-callErr:
				require.Error(t, answerErr, "the plugin's Stage 1 call is answered with the refusal")
				for _, want := range tc.want {
					assert.Contains(t, answerErr.Error(), want, "the plugin reads the operator's text")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("the plugin did not receive the refusal")
			}
			assert.Equal(t, []string{"Conn"}, sink.order, "the refusal is the only answer: OnRegistration never runs, so no module is published")
		})
	}
}

// TestEarlyStage1RefusesUndecodableDeclaration proves the early read at
// `ze start` gives the same refusal as the handshake: the plugin's outcome is
// the decoder's operator error, no module is answered, and the plugin stays
// held, so its tier refuses the request it kept on the wire.
//
// Method: HoldDeclaredPlugins over a real PluginConn, the plugin end sending
// the raw JSON.
func TestEarlyStage1RefusesUndecodableDeclaration(t *testing.T) {
	for _, tc := range refusedDeclarations {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			cfg := plugin.PluginConfig{Name: "acme", Run: "acme-plugin", StageTimeout: 2 * time.Second}
			manager := process.NewProcessManager(nil)
			proc, pluginSide := heldPlugin(t, manager, cfg)
			spawner := &fakeDeclaredSpawner{manager: manager}
			go func() {
				pluginConn := rpc.NewConn(pluginSide, pluginSide)
				//nolint:errcheck // the test ends the call by canceling ctx
				pluginConn.CallRPC(ctx, rpc.MethodDeclareRegistration, json.RawMessage(tc.params))
			}()

			declarations := HoldDeclaredPlugins(ctx, spawner, []plugin.PluginConfig{cfg}, nil)

			require.Len(t, declarations, 1)
			require.Error(t, declarations[0].Err)
			_, refused := errors.AsType[*opererr.Error](declarations[0].Err)
			assert.True(t, refused, "the outcome is the decoder's operator error, got %T", declarations[0].Err)
			assert.Contains(t, declarations[0].Err.Error(), `plugin "acme"`)
			for _, want := range tc.want {
				assert.Contains(t, declarations[0].Err.Error(), want)
			}
			assert.NotContains(t, declarations[0].Err.Error(), "sent no declaration", "the declaration was read; its key was refused")
			assert.Empty(t, declarations[0].Schemas, "a refused declaration publishes no module")
			assert.Empty(t, spawner.failed, "the plugin stays held: its tier refuses the request on the wire")

			request, err := proc.Conn().ReadRequest(ctx)
			require.NoError(t, err)
			assert.Equal(t, rpc.MethodDeclareRegistration, request.Method, "the tier reads the request the early step kept")
		})
	}
}
