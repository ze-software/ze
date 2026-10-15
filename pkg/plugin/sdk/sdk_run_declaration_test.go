// Design: docs/architecture/api/process-protocol.md -- plugin SDK
// Related: sdk.go -- Run takes the declaration by pointer
// Related: sdk_query.go -- RunOrDeclare takes the declaration by pointer
package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: Run refuses a nil declaration with a named error.
// PREVENTS: a nil pointer panicking inside stage 1 instead of an error the
// plugin author can read.
//
// The goal is that a plugin passing no declaration gets a refusal, not a
// crash. The method calls Run with nil on a connected plugin and asserts the
// error is the nil-declaration refusal.
func TestRunRefusesNilRegistration(t *testing.T) {
	t.Parallel()

	p, _ := newTestPair(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := p.Run(ctx, nil)
	require.ErrorIs(t, err, errNilRegistration)
}

// VALIDATES: Run copies the declaration at entry, so the fields it derives
// from registered callbacks reach the engine without being written back.
// PREVENTS: Run writing into the caller's declaration, which a caller that
// shares one declaration between plugins would see change under it.
//
// The goal is that the caller's value is only read, once. The method
// registers a drain handler, which makes Run declare DrainHandler, then reads
// the stage 1 request on the engine side and asserts the engine saw the
// derived field while the caller's declaration still holds false.
func TestRunCopiesRegistration(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)
	p.OnDrain(func(context.Context, rpc.DrainInput) error { return nil })

	reg := &Registration{WantsConfig: []string{"bgp"}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, reg) }()

	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, rpc.MethodDeclareRegistration, req.Method)

	var sent Registration
	require.NoError(t, json.Unmarshal(req.Params, &sent))
	assert.True(t, sent.DrainHandler, "the engine never saw the derived drain declaration")
	assert.False(t, reg.DrainHandler, "Run wrote the derived field into the caller's declaration")

	cancel()
	<-errCh
}

// VALIDATES: RunOrDeclare refuses a nil declaration before activation.
// PREVENTS: a plugin starting live, or answering a query with "null", when it
// passed no declaration.
//
// The goal is one refusal in both modes. The method calls the entry point with
// nil, with and without the query mode, and asserts exit code 1, an empty
// stdout and an activation function that never ran.
func TestRunOrDeclareRefusesNilRegistration(t *testing.T) {
	for _, mode := range []string{"", ModeDeclare} {
		require.NoError(t, env.Set(EnvPluginMode, mode))

		activated := false
		var stdout bytes.Buffer

		code := runOrDeclare(&stdout, nil, func() int {
			activated = true
			return 0
		})

		assert.Equal(t, 1, code, "mode %q", mode)
		assert.False(t, activated, "mode %q: the activation function ran with no declaration", mode)
		assert.Empty(t, stdout.String(), "mode %q", mode)
	}
	require.NoError(t, env.Set(EnvPluginMode, ""))
}
