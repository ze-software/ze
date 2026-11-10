package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// vlanIDMessage is the error-message of the zt:vlan-id range
// (internal/component/config/yang/modules/ze-types.yang).
const vlanIDMessage = "IEEE 802.1Q reserves VLAN ID 0 and 4095, so a VLAN ID is 1 to 4094. Give a VLAN ID from 1 to 4094."

// TestUnitVIDRefusalSpeaksTheVLANRange walks the vid argument of every unit
// command to its bounds and past them, through the definitions the dispatcher
// validates with.
//
// VALIDATES: AC-12 of spec-operator-errors-name-the-fix: `create interface
// <name> unit 9999` is refused by the argument validator, which runs before
// handleUnitAdd, with the range and the reason 802.1Q gives for it. The range
// is declared once, on zt:vlan-id, and every unit command reaches it.
// PREVENTS: the bare `value 9999 out of range 1..4094`, which names neither
// why the range holds nor what to type, and a unit command whose vid leaf
// keeps its own range with no message.
func TestUnitVIDRefusalSpeaksTheVLANRange(t *testing.T) {
	schema, err := configyang.DefaultLoader()
	require.NoError(t, err)
	argDefs := configyang.PathToArgDefs(schema)

	paths := []string{
		"create interface dummy name unit",
		"create interface bridge name unit",
		"create interface unit",
		"delete interface name unit",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			var def *command.ArgDef
			for i := range argDefs[path] {
				if argDefs[path][i].Name() == "vid" {
					def = &argDefs[path][i]
				}
			}
			require.NotNil(t, def, "the model declares no vid leaf on %q", path)

			for _, accepted := range []string{"1", "4094"} {
				assert.NoError(t, command.ValidateArgString(accepted, def), "VLAN ID %s", accepted)
			}
			for _, refused := range []string{"0", "4095", "9999"} {
				err := command.ValidateArgString(refused, def)
				if assert.Error(t, err, "VLAN ID %s", refused) {
					assert.Equal(t, "value "+refused+" out of range 1..4094: "+vlanIDMessage, err.Error())
				}
			}
		})
	}
}
