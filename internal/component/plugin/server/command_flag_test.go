package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// TestValidateCommandArgsFlag drives the dispatcher's argument validation over
// a command holding a YANG `type empty` leaf, the shape of `request data backup
// path <abs> [spare <n>] [force]`. The flag is its own keyword in any position
// and takes no value, so it never fills the open uint leaf beside it.
// VALIDATES: storage-2 AC-4 (`force` reaches the handler).
// PREVENTS: a flag keyword refused as "invalid value "force", expected unsigned integer".
func TestValidateCommandArgsFlag(t *testing.T) {
	defs := []command.ArgDef{
		{Name: "path", Kind: command.ArgString, Mandatory: true},
		{Name: "spare", Kind: command.ArgUint, UintBits: 8},
		{Name: "force", Kind: command.ArgFlag},
	}
	for _, args := range [][]string{
		{"path", "/b.zefs", "force"},
		{"force", "path", "/b.zefs"},
		{"path", "/b.zefs", "spare", "5", "force"},
		{"path", "/b.zefs"},
	} {
		_, err := command.ValidateArgs(args, defs, nil)
		require.NoError(t, err, args)
	}
	_, err := command.ValidateArgs([]string{"path", "/b.zefs", "force", "force"}, defs, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `duplicate keyword "force"`)
	_, err = command.ValidateArgs([]string{"path", "/b.zefs", "force", "yes"}, defs, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"yes"`)
	assert.Equal(t, "force takes no value", command.ValidateArgString("x", &defs[2]).Error())
}
