// Design: docs/plugin-development/README.md -- telling a startup failure from a later one
// Related: startup_error.go -- StartupError under test

package sdk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRunMarksStartupFailure proves every error Run returns before stage 5
// completes carries StartupError with the stage it failed at, so a caller can
// log a startup failure and a later failure under different codes without
// reading the message.
//
// The method plays the engine, refuses one stage per case, and checks the
// mark with errors.As; the nil declaration case fails before stage 1 is sent.
func TestRunMarksStartupFailure(t *testing.T) {
	t.Parallel()

	t.Run("nil declaration", func(t *testing.T) {
		t.Parallel()
		p, _ := newTestPair(t)
		err := p.Run(t.Context(), nil)
		startup, ok := errors.AsType[*StartupError](err)
		require.True(t, ok, "Run error %v carries no StartupError", err)
		assert.Equal(t, 1, startup.Stage)
		assert.ErrorIs(t, err, errNilRegistration, "the mark keeps the cause")
	})

	t.Run("stage 1 refused", func(t *testing.T) {
		t.Parallel()
		p, engine := newTestPair(t)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		errCh := make(chan error, 1)
		go func() { errCh <- p.Run(ctx, &Registration{}) }()

		req := readEngineRequest(t, ctx, engine.mux)
		require.Equal(t, rpc.MethodDeclareRegistration, req.Method)
		require.NoError(t, engine.mux.SendError(ctx, req.ID, "refused at stage 1"))

		err := <-errCh
		startup, ok := errors.AsType[*StartupError](err)
		require.True(t, ok, "Run error %v carries no StartupError", err)
		assert.Equal(t, 1, startup.Stage)
		assert.Contains(t, err.Error(), "stage 1 (declare-registration)", "the message is unchanged")
	})

	t.Run("stage 3 refused", func(t *testing.T) {
		t.Parallel()
		p, engine := newTestPair(t)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		errCh := make(chan error, 1)
		go func() { errCh <- p.Run(ctx, &Registration{}) }()

		completeStagesOneAndTwo(t, ctx, engine)
		req := readEngineRequest(t, ctx, engine.mux)
		require.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
		require.NoError(t, engine.mux.SendError(ctx, req.ID, "refused at stage 3"))

		err := <-errCh
		startup, ok := errors.AsType[*StartupError](err)
		require.True(t, ok, "Run error %v carries no StartupError", err)
		assert.Equal(t, 3, startup.Stage)
		assert.Contains(t, err.Error(), "stage 3 (declare-capabilities)")
	})

	t.Run("stage 5 refused", func(t *testing.T) {
		t.Parallel()
		p, engine := newTestPair(t)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		errCh := make(chan error, 1)
		go func() { errCh <- p.Run(ctx, &Registration{}) }()

		completeStagesOneToFour(t, ctx, engine)
		req := readEngineRequest(t, ctx, engine.mux)
		require.Equal(t, "ze-plugin-engine:ready", req.Method)
		require.NoError(t, engine.mux.SendError(ctx, req.ID, "refused at stage 5"))

		err := <-errCh
		startup, ok := errors.AsType[*StartupError](err)
		require.True(t, ok, "Run error %v carries no StartupError", err)
		assert.Equal(t, 5, startup.Stage)
	})
}

// TestRunPostReadyFailureUnmarked proves an error Run returns after stage 5
// completed carries no StartupError: the plugin started, so its failure is a
// stop, not a startup failure.
//
// The method completes all five stages, then lets OnStarted fail.
func TestRunPostReadyFailureUnmarked(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p.OnStarted(func(context.Context) error { return errors.New("started and then failed") })
	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, &Registration{}) }()

	completeStagesOneToFour(t, ctx, engine)
	req := readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	err := <-errCh
	require.Error(t, err)
	assert.Contains(t, err.Error(), "started and then failed")
	_, marked := errors.AsType[*StartupError](err)
	assert.False(t, marked, "a failure after ready carries the startup mark: %v", err)
}

// completeStagesOneAndTwo answers declare-registration and sends configure.
func completeStagesOneAndTwo(t *testing.T, ctx context.Context, engine *engineSide) {
	t.Helper()
	req := readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, rpc.MethodDeclareRegistration, req.Method)
	require.NoError(t, engine.mux.SendResult(ctx, req.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}))
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:configure", configInput))
}

// completeStagesOneToFour runs stages 1 to 4 and leaves the ready request
// for the caller to read.
func completeStagesOneToFour(t *testing.T, ctx context.Context, engine *engineSide) {
	t.Helper()
	completeStagesOneAndTwo(t, ctx, engine)
	req := readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))
	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:share-registry", registryInput))
}
