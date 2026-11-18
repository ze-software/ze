// Design: docs/architecture/api/commands.md -- required arguments explain their purpose.
package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestLoneMissingArgumentPurpose requires the model's declared meaning in both
// the original missing cause and the operator explanation. A declaration with
// no help must honestly identify that gap rather than invent a purpose.
func TestLoneMissingArgumentPurpose(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opts    ArgOptions
		purpose string
	}{
		{name: "summary", opts: ArgOptions{Mandatory: true, ShortHelp: "Remote host whose service is checked"}, purpose: "Remote host whose service is checked"},
		{name: "description", opts: ArgOptions{Mandatory: true, ShortHelp: "Remote host", Description: "The hostname selects the service whose certificate the command inspects."}, purpose: "The hostname selects the service whose certificate the command inspects."},
		{name: "undeclared purpose", opts: ArgOptions{Mandatory: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defs := []ArgDef{mustArgDef(NewStringArg("target", nil, nil, tc.opts))}
			_, err := ValidateArgs(nil, defs, nil)
			require.Error(t, err)
			missing, found := errors.AsType[*MissingArgumentError](err)
			require.True(t, found)
			refusal, found := errors.AsType[*opererr.Error](err)
			require.True(t, found)
			if tc.purpose != "" {
				assert.Contains(t, missing.Error(), tc.purpose, "the typed cause must retain declared argument meaning")
				assert.Contains(t, refusal.Why(), tc.purpose, "the lone wrapper must explain the declared purpose")
				return
			}
			assert.Contains(t, refusal.Why(), "target")
			assert.Contains(t, refusal.Why(), "definition")
			assert.Contains(t, refusal.Why(), "mandatory")
			assert.Contains(t, refusal.Why(), "no further explanation")
			assert.Contains(t, missing.Error(), refusal.Why())
		})
	}
}

// TestLoneMissingArgumentPurposeReachesModelAndLocal follows a declared purpose
// through the actual model validator and both local command entries, while
// keeping refusal ahead of the registered handlers and retaining the full fix.
func TestLoneMissingArgumentPurposeReachesModelAndLocal(t *testing.T) {
	const path = "show test argument purpose"
	const purpose = "The target names the remote service whose certificate is inspected."
	defs := []ArgDef{mustArgDef(NewStringArg("target", nil, nil, ArgOptions{Mandatory: true, Description: purpose}))}
	withArgDefSource(t, func(p string) ([]ArgDef, error) {
		if p == path {
			return defs, nil
		}
		return nil, nil
	})
	_, err := ValidateModelArgs(path, nil, nil)
	require.Error(t, err)
	refusal, found := errors.AsType[*opererr.Error](err)
	require.True(t, found)
	assert.Contains(t, refusal.Why(), purpose)
	assert.Equal(t, path+" target <target>", refusal.Fix())
	called := 0
	t.Cleanup(AddLocalDataForTest(path, func(ValidatedArgs) LocalAnswer {
		called++
		return LocalAnswer{}
	}))
	var code int
	stderr := captureStderr(t, func() {
		_, code, _ = ServeLocal(path, "")
	})
	assert.Equal(t, 1, code)
	assert.Zero(t, called)
	assert.Contains(t, stderr, purpose)
	assert.Contains(t, stderr, "\nfix: "+path+" target <target>\n")
	stderr = captureStderr(t, func() {
		code = InvokeLocal(path, func(ValidatedArgs) int { called++; return 0 }, nil)
	})
	assert.Equal(t, 1, code)
	assert.Zero(t, called)
	assert.Contains(t, stderr, purpose)
	assert.Contains(t, stderr, "\nfix: ze "+path+" target <target>\n")
}

// TestLoneMissingArgumentUnplacedEvidenceIsOwned proves the guard's evidence
// excludes named options and survives changes to both the original arguments
// and a returned view. It judges the real validator's constrained-leaf error.
func TestLoneMissingArgumentUnplacedEvidenceIsOwned(t *testing.T) {
	defs := []ArgDef{
		mustArgDef(NewEnumArg("selector", []string{"peer-a"}, ArgOptions{Mandatory: true})),
		mustArgDef(NewEnumArg("family", []string{"ipv4/unicast"}, ArgOptions{})),
	}
	args := []string{"family", "ipv4/unicast", "unknown-a", "unknown-b"}
	_, err := ValidateArgs(args, defs, nil)
	require.Error(t, err)
	missing, found := errors.AsType[*MissingArgumentError](err)
	require.True(t, found)
	want := []string{"unknown-a", "unknown-b"}
	assert.Equal(t, want, missing.UnplacedTokens())
	args[2] = "changed-by-caller"
	view := missing.UnplacedTokens()
	require.Len(t, view, 2)
	view[0] = "changed-returned-view"
	assert.Equal(t, want, missing.UnplacedTokens())
}
