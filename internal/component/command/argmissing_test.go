// Design: docs/architecture/api/commands.md -- every route validates.
// Related: argfaults.go -- the shared whole-command correction.
package command

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestLoneMissingArgumentUsesStructuredRefusal judges the validator's real
// missing-leaf result, preserving its positional binding and empty success
// value while requiring the same structured refusal as a lone rejected value.
func TestLoneMissingArgumentUsesStructuredRefusal(t *testing.T) {
	defs := []ArgDef{
		mustArgDef(NewEnumArg("selector", []string{"peer-a"}, ArgOptions{Mandatory: true})),
		mustArgDef(NewStringArg("label", nil, nil, ArgOptions{Mandatory: true})),
	}
	validated, err := ValidateArgs([]string{"peer-a"}, defs, nil)
	require.Error(t, err)
	assert.Empty(t, validated.Tokens(), "a refused call must answer no validated tokens")
	missing, found := errors.AsType[*MissingArgumentError](err)
	require.True(t, found, "the refusal must retain the missing leaf's typed cause")
	assert.Equal(t, "label", missing.Leaf)
	bound, found := missing.Positional("selector")
	assert.True(t, found)
	assert.Equal(t, "peer-a", bound)
	assert.GreaterOrEqual(t, strings.Count(missing.Error(), "\n"), 2,
		"the missing-leaf cause itself must explain the requirement and its correction")
	assert.Contains(t, missing.Error(), "\nfix: ")
	assert.Contains(t, missing.Error(), "label <label>")
	faults, found := errors.AsType[*ArgumentFaults](err)
	require.True(t, found, "a lone missing leaf must enter the common correction route")
	require.Len(t, faults.Faults(), 1)
	assert.Same(t, missing, faults.Faults()[0])
	assert.Zero(t, faults.Omitted())
	refusal, found := errors.AsType[*opererr.Error](err)
	require.True(t, found, "the validator must supply a problem, explanation and fix")
	assert.Contains(t, refusal.Problem(), "label")
	assert.Contains(t, refusal.Why(), "label")
	assert.Contains(t, refusal.Fix(), "label <label>")
	assert.NotContains(t, refusal.Error(), "this command has 1 faults")
}

// TestLoneMissingArgumentModelRoutes refuses the same actual model input on
// the shared validator, local-data and ze-verb entry points. Each route must
// give the operator's entire corrected line and never call a refused handler;
// a complete line still reaches each local handler exactly once.
func TestLoneMissingArgumentModelRoutes(t *testing.T) {
	withRouteFaultsSource(t)
	args := []string{"asn", "65001", "router-id", "192.0.2.1"}
	const fix = "show test route faults asn 65001 router-id 192.0.2.1 hold-time <hold-time>"
	t.Run("model", func(t *testing.T) {
		validated, err := ValidateModelArgs(routeFaultsPath, args, nil)
		requireLoneMissingArgument(t, err, "hold-time", fix)
		assert.Empty(t, validated.Tokens())
		_, err = ValidateModelArgs(routeFaultsPath, append(append([]string{}, args...), "hold-time", "30"), nil)
		require.NoError(t, err)
	})
	t.Run("local-data", func(t *testing.T) {
		called := 0
		t.Cleanup(AddLocalDataForTest(routeFaultsPath, func(ValidatedArgs) LocalAnswer {
			called++
			return LocalAnswer{}
		}))
		var code int
		var served bool
		stderr := captureStderr(t, func() {
			_, code, served = ServeLocal(routeFaultsPath+" "+strings.Join(args, " "), "")
		})
		assert.True(t, served)
		assert.Equal(t, 1, code)
		assert.Zero(t, called)
		assert.Contains(t, stderr, "hold-time")
		assert.Contains(t, stderr, "\nfix: "+fix+"\n")
		stderr = captureStderr(t, func() {
			_, code, served = ServeLocal(routeFaultsPath+" "+strings.Join(args, " ")+" hold-time 30", "")
		})
		assert.True(t, served)
		assert.Zero(t, code)
		assert.Empty(t, stderr)
		assert.Equal(t, 1, called)
	})
	t.Run("ze-verb", func(t *testing.T) {
		called := 0
		handler := func(ValidatedArgs) int {
			called++
			return 0
		}
		var code int
		stderr := captureStderr(t, func() { code = InvokeLocal(routeFaultsPath, handler, args) })
		assert.Equal(t, 1, code)
		assert.Zero(t, called)
		assert.Contains(t, stderr, "hold-time")
		assert.Contains(t, stderr, "\nfix: ze "+fix+"\n")
		stderr = captureStderr(t, func() {
			code = InvokeLocal(routeFaultsPath, handler, append(append([]string{}, args...), "hold-time", "30"))
		})
		assert.Zero(t, code)
		assert.Empty(t, stderr)
		assert.Equal(t, 1, called)
	})
}

// TestLoneMissingArgumentCorrectionKeepsRestAndSecretBoundaries requires the
// new one-fault route to reuse the existing correction writer: a missing
// keyword precedes rest text, and a valid credential never enters the fix.
func TestLoneMissingArgumentCorrectionKeepsRestAndSecretBoundaries(t *testing.T) {
	const path = "show test missing correction"
	defs := []ArgDef{
		mustArgDef(NewUintArg("hold-time", 16, nil, ArgOptions{Mandatory: true})),
		mustArgDef(NewStringArg("content", nil, nil, ArgOptions{})),
		mustArgDef(NewStringArg("text", nil, nil, ArgOptions{Rest: true})),
	}
	withArgDefSource(t, func(p string) ([]ArgDef, error) {
		if p == path {
			return defs, nil
		}
		return nil, nil
	})
	_, err := ValidateModelArgs(path, []string{"content", "private-credential", "text", "hello", "world"}, nil)
	requireLoneMissingArgument(t, err, "hold-time",
		path+" content <redacted> hold-time <hold-time> text hello world")
	assert.NotContains(t, err.Error(), "private-credential")
}

// requireLoneMissingArgument checks the complete correction at a route that
// knows its command path, and the typed identities behind that presentation.
func requireLoneMissingArgument(t *testing.T, err error, leaf, fix string) {
	t.Helper()
	require.Error(t, err)
	missing, found := errors.AsType[*MissingArgumentError](err)
	require.True(t, found)
	assert.Equal(t, leaf, missing.Leaf)
	faults, found := errors.AsType[*ArgumentFaults](err)
	require.True(t, found)
	assert.Len(t, faults.Faults(), 1)
	refusal, found := errors.AsType[*opererr.Error](err)
	require.True(t, found)
	assert.Contains(t, refusal.Problem(), leaf)
	assert.Contains(t, refusal.Why(), leaf)
	assert.Equal(t, fix, refusal.Fix())
	assert.Contains(t, err.Error(), "\nfix: "+fix)
}
