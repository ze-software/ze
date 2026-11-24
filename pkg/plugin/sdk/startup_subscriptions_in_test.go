// Design: docs/architecture/api/process-protocol.md -- Stage 5 ready delivery options.
// VALIDATES: SetEventFormat, SetEncoding, and SetEnvelope populate the per-process
// wire delivery fields and serialize them onto the Stage 5 ReadyInput RPC without
// disturbing one another or altering defaults when unset.
// PREVENTS: Wire delivery preferences (format, encoding, envelope) being dropped
// or clobbered during Stage 5 ready after startup subscriptions moved to Stage 1
// Receives declarations.
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

// TestReadyDeliveryOptionSetters verifies that SetEventFormat, SetEncoding, and
// SetEnvelope independently update readyFormat, readyEncoding, and readyEnvelope.
//
// VALIDATES: Each delivery setter updates only its own field, preserving the
// other two fields and zero-value defaults when unset.
// PREVENTS: Calling one delivery setter resetting or failing to record another.
func TestReadyDeliveryOptionSetters(t *testing.T) {
	t.Parallel()

	p := &Plugin{}
	assert.Empty(t, p.readyFormat)
	assert.Empty(t, p.readyEncoding)
	assert.False(t, p.readyEnvelope)

	p.SetEventFormat("full")
	assert.Equal(t, "full", p.readyFormat)
	assert.Empty(t, p.readyEncoding)
	assert.False(t, p.readyEnvelope)

	p.SetEncoding("text")
	assert.Equal(t, "full", p.readyFormat)
	assert.Equal(t, "text", p.readyEncoding)
	assert.False(t, p.readyEnvelope)

	p.SetEnvelope(true)
	assert.Equal(t, "full", p.readyFormat)
	assert.Equal(t, "text", p.readyEncoding)
	assert.True(t, p.readyEnvelope)

	p.SetEnvelope(false)
	assert.Equal(t, "full", p.readyFormat)
	assert.Equal(t, "text", p.readyEncoding)
	assert.False(t, p.readyEnvelope)
}

// TestReadyInputWireDeliveryOptions runs a real 5-stage handshake over net.Pipe
// and verifies the Stage 5 ReadyInput payload carries Format, Encoding, and Envelope.
//
// VALIDATES: Plugin.Run populates rpc.ReadyInput{Format, Encoding, Envelope}
// from SetEventFormat, SetEncoding, and SetEnvelope on the Stage 5 ready RPC.
// PREVENTS: Stage 5 ready dropping wire delivery preferences before the engine
// activates Stage 1 Receives subscriptions.
func TestReadyInputWireDeliveryOptions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		format       string
		encoding     string
		envelope     bool
		wantFormat   string
		wantEncoding string
		wantEnvelope bool
	}{
		{
			name: "defaults",
		},
		{
			name:         "all-options-set",
			format:       "full",
			encoding:     "text",
			envelope:     true,
			wantFormat:   "full",
			wantEncoding: "text",
			wantEnvelope: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p, engine := newTestPair(t)
			if tc.format != "" {
				p.SetEventFormat(tc.format)
			}
			if tc.encoding != "" {
				p.SetEncoding(tc.encoding)
			}
			if tc.envelope {
				p.SetEnvelope(true)
			}

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			errCh := make(chan error, 1)
			go func() {
				errCh <- p.Run(ctx, &Registration{})
			}()

			// Stage 1: declare-registration.
			req := readEngineRequest(t, ctx, engine.mux)
			require.Equal(t, rpc.MethodDeclareRegistration, req.Method)
			require.NoError(t, engine.mux.SendResult(ctx, req.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}))

			// Stage 2: configure.
			require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:configure", struct{}{}))

			// Stage 3: declare-capabilities.
			req = readEngineRequest(t, ctx, engine.mux)
			require.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
			require.NoError(t, engine.mux.SendOK(ctx, req.ID))

			// Stage 4: share-registry.
			require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:share-registry", struct{}{}))

			// Stage 5: ready.
			req = readEngineRequest(t, ctx, engine.mux)
			require.Equal(t, "ze-plugin-engine:ready", req.Method)
			var ready rpc.ReadyInput
			require.NoError(t, json.Unmarshal(req.Params, &ready))
			assert.Equal(t, tc.wantFormat, ready.Format)
			assert.Equal(t, tc.wantEncoding, ready.Encoding)
			assert.Equal(t, tc.wantEnvelope, ready.Envelope)
			assert.Empty(t, ready.Transport)
			require.NoError(t, engine.mux.SendOK(ctx, req.ID))

			require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", struct {
				Reason string `json:"reason"`
			}{Reason: "test-complete"}))

			select {
			case err := <-errCh:
				require.NoError(t, err)
			case <-time.After(2 * time.Second):
				t.Fatal("plugin did not exit after bye")
			}
		})
	}
}
