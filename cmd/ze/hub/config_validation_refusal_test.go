// Design: docs/architecture/cli/error-surface.md -- The operator error
// Related: api.go -- configValidationHook, the commit check of the REST and gRPC sessions

package hub

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestConfigValidationHookEnumRefusal commits, through the hook every REST and
// gRPC config session commit runs, a candidate holding a value outside an
// enumeration.
//
// VALIDATES: AC-8 at the hook -- the refusal names the leaf and the refused
// value, lists the accepted values, and ends with the set-form fix from the
// root (`fix: set interface ethernet eth0 unit 0 ipv4 rpf-check strict`),
// with no `line N:` and no file-form fix; errors.As reaches the typed
// ValidationError, so the session surfaces answer the shared refusal.
// PREVENTS: the session commit answering the file form of the fix, or a
// refusal that lost its type on the way to REST (400) and gRPC
// (InvalidArgument).
// The fixture uses an always-on root (interface), not bgp, for the reason
// TestConfigValidationHookRunsFullValidation gives.
func TestConfigValidationHookEnumRefusal(t *testing.T) {
	hook := configValidationHook("test.conf")
	const good = `interface { ethernet eth0 { unit 0 { ipv4 { rpf-check strict; } } } }`
	const bad = `interface { ethernet eth0 { unit 0 { ipv4 { rpf-check stict; } } } }`
	require.NoError(t, hook(good, good), "the control candidate is accepted")

	err := hook(good, bad)
	require.Error(t, err)
	text := err.Error()
	assert.True(t, strings.HasPrefix(text, "config validation failed:\n"), "text %q", text)
	assert.Contains(t, text, `interface ethernet eth0 unit 0 ipv4 rpf-check: "stict" is not an accepted value`)
	assert.Contains(t, text, "rpf-check accepts only these values:")
	assert.True(t, strings.HasSuffix(text, "\n  fix: set interface ethernet eth0 unit 0 ipv4 rpf-check strict"), "text %q", text)
	assert.NotContains(t, text, "line ")
	assert.NotContains(t, text, "rpf-check strict;")

	var refusal *configyang.ValidationError
	require.True(t, errors.As(err, &refusal), "errors.As finds the ValidationError")
	assert.Equal(t, configyang.ErrTypeEnum, refusal.Type)
	assert.Equal(t, "stict", refusal.Got)
}
