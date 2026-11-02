package sdk

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAcceptProtocolVersion pins the Stage 1 answers this SDK accepts: every
// version from 1 to rpc.ProtocolVersion, so a plugin built on it still starts
// against an engine that speaks only version 1, and nothing outside that range.
//
// VALIDATES: the warning lines are additive: a version 1 engine is still
// spoken to, and only an unknown contract is refused.
// PREVENTS: a newer SDK refusing an engine whose bytes it reads unchanged.
func TestAcceptProtocolVersion(t *testing.T) {
	t.Parallel()

	for version := 1; version <= rpc.ProtocolVersion; version++ {
		answer := []byte(`{"protocol-version":` + strconv.Itoa(version) + `}`)
		assert.NoError(t, acceptProtocolVersion(answer), "version %d", version)
	}
	for _, answer := range []string{``, `{}`, `{"protocol-version":0}`, `{"protocol-version":` + strconv.Itoa(rpc.ProtocolVersion+1) + `}`} {
		assert.Error(t, acceptProtocolVersion([]byte(answer)), "answer %q", answer)
	}
}

// TestRunDeclaresWarningLinesVersion checks that a plugin built on this SDK
// declares, with no code of its own, the protocol version that carries the
// warning lines. The method: the fake engine reads the Stage 1 request the
// plugin sends from Run.
//
// VALIDATES: a new-SDK plugin negotiates rpc.ProtocolVersionWarningLines, so
// the engine sends it the warning text (plugin.WriteAnswer).
// PREVENTS: an SDK whose plugins never receive the text the engine gates on
// the declared version.
func TestRunDeclaresWarningLinesVersion(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, &Registration{})
	}()

	req := readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, rpc.MethodDeclareRegistration, req.Method)
	var declared rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(req.Params, &declared))
	assert.GreaterOrEqual(t, declared.ProtocolVersion, rpc.ProtocolVersionWarningLines)
	require.NoError(t, engine.mux.SendResult(ctx, req.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}))

	completeStartupFromStage2(t, ctx, engine)
	shutdownPlugin(t, ctx, engine, errCh)
}
