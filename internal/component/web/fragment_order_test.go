// Design: docs/architecture/web-components.md -- form fields from provider.Children()

package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// webOrderModule declares the leaves of container web-order-fixture remote,
// local, bind: against name order (bind, local, remote), so a form that
// follows name order lists them in an order this text does not hold. Its
// container timers holds no field.
const webOrderModule = `module web-order-fixture-conf {
    namespace "urn:ze:test:web-order-fixture";
    prefix wof;

    container web-order-fixture {
        leaf remote { type string; }
        leaf local { type string; }
        container timers {
            leaf hold { type uint16; }
        }
        leaf bind { type string; }
    }
}
`

// TestWebFormFieldsDefinitionOrder: the web editor's form for a container
// lists its leaves in YANG definition order. Method: join a module declaring
// remote, local, bind to the compiled-in schema through YANGSchemaWithPlugins,
// the road an external plugin's module takes, and build the fragment of that
// container.
//
// VALIDATES: AC-13 of spec-config-yang-definition-order: the form follows
// provider.Children(), which the converter builds in definition order.
// PREVENTS: the form listing the fields in another order than show prints
// the same block.
func TestWebFormFieldsDefinitionOrder(t *testing.T) {
	schema, err := config.YANGSchemaWithPlugins(map[string]string{"web-order-fixture-conf.yang": webOrderModule})
	require.NoError(t, err)

	data := buildFragmentData(schema, config.NewTree(), []string{"web-order-fixture"})
	var leaves []string
	for _, field := range data.Fields {
		leaves = append(leaves, field.Leaf)
	}
	assert.Equal(t, []string{"remote", "local", "bind"}, leaves)
}
