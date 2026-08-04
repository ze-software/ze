// Related: nlritype.go -- SnapshotForTest and ResetForTest, the two teardowns

package nlritype

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// VALIDATES: SnapshotForTest empties the registry for the duration and puts back exactly
// what was registered, including a family registered before it ran.
// PREVENTS: the failure ResetForTest has. Registration happens in plugin init(), so a test
// binary that links an NLRI plugin starts with real rulings; a teardown that CLEARS instead
// of restoring leaves the next test finding no recognizer for a family the daemon rules
// for. Its Section 5.4 filter then does nothing and it passes proving nothing, which is a
// green bar over an unenforced MUST.
func TestSnapshotForTestRestoresRegistrations(t *testing.T) {
	t.Cleanup(ResetForTest)
	ResetForTest()

	// Stand in for what a plugin's init() would have left here.
	preexisting := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	require.NoError(t, Register(preexisting, func([]byte, bool) bool { return true }))
	require.NotNil(t, Get(preexisting))

	restore := SnapshotForTest()
	assert.Nil(t, Get(preexisting),
		"the snapshot must hand the test an empty registry to work in")

	other := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMVPN}
	require.NoError(t, Register(other, func([]byte, bool) bool { return false }))

	restore()

	assert.NotNil(t, Get(preexisting),
		"restore must put back what was registered before the snapshot")
	assert.Nil(t, Get(other),
		"restore must remove what the test registered")
}
