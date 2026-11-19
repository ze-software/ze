// Design: docs/architecture/api/commands.md -- family arguments preserve summary metadata

package peer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/family"
)

// TestShowBGPFamilyArgumentsKeepSummaryMetadata derives the full family forms
// from their registry and adds the three shorthands the overview handler accepts.
// These are arguments of the parent summary, so its aliases and answer metadata
// must remain available on every spelling.
func TestShowBGPFamilyArgumentsKeepSummaryMetadata(t *testing.T) {
	paths := family.RegisteredFamilyNames()
	require.NotEmpty(t, paths, "the family population must be linked")
	paths = append(paths, "ipv4", "ipv6", "l2vpn", "IPV4")
	parentShape, parentDeclared := command.ShapeForCommand("show bgp")
	require.True(t, parentDeclared)
	parentColumns := command.ColumnsForCommand("show bgp")
	require.NotEmpty(t, parentColumns)
	parentFields := command.AddressFieldsForCommand("show bgp")
	require.NotEmpty(t, parentFields)
	parentAliases := command.AliasesForCommand("show bgp")
	require.NotEmpty(t, parentAliases)
	for _, name := range paths {
		t.Run(name, func(t *testing.T) {
			require.True(t, isFamilyArg(name), "population must be accepted by handleBgpOverview")
			path := "show bgp " + name
			shape, declared := command.ShapeForCommand(path)
			require.True(t, declared)
			require.Equal(t, parentShape, shape)
			require.Equal(t, parentColumns, command.ColumnsForCommand(path))
			require.Equal(t, parentFields, command.AddressFieldsForCommand(path))
			require.Equal(t, parentAliases, command.AliasesForCommand(path))
			require.Equal(t, command.DeclaredForCommand("show bgp"), command.DeclaredForCommand(strings.ToLower(path)))
		})
	}
}
