// VALIDATES: ze-fib-dummy-conf.yang augments /fib:fib with a container `dummy`
// that declares zero child leaves or containers (AC-12).
// PREVENTS: unread leaves being added under `fib dummy`, repeating the recorded
// defect of `fib p4`.
package fibdummy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

func TestDummySchemaHasNoLeaf(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	require.NoError(t, err, "load default YANG schema")

	fibMod := loader.GetEntry("ze-fib-conf")
	require.NotNil(t, fibMod, "ze-fib-conf module must be loaded")

	fibContainer := fibMod.Dir["fib"]
	require.NotNil(t, fibContainer, "ze-fib-conf must define /fib container")

	dummyContainer := fibContainer.Dir["dummy"]
	require.NotNil(t, dummyContainer, "ze-fib-dummy-conf must augment /fib:fib with container dummy")

	assert.Empty(t, dummyContainer.Dir, "fib/dummy container must hold no leaves or child containers")
}
