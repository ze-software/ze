// Design: docs/architecture/config/syntax.md — write-through set of OS-portable interface address
package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"

	// Register the iface YANG so the editor schema knows "interface ethernet".
	_ "github.com/ze-software/ze/internal/component/iface/yang"
)

// TestWriteThroughInterfaceAddressRoundTrip pins the fix for the corrupt change-file
// warning in test/web/scenario-interface-setup.wb. The correct unit-address path is
// `unit 0 ipv4 address` / `unit 0 ipv6 address` (address lives under the per-family
// container), and both containers are now present on every OS, so the write-through
// set must succeed and the change file must re-parse cleanly on the host OS —
// including darwin, where ipv4/ipv6 used to be pruned and `set ... unit 0 address ...`
// produced an unparseable change file.
//
// The ipv6 case also guards the colon-heavy value `fd00::1/64` (a value with several
// ':' plus '/') through the metadata-annotated change-file round-trip.
//
// VALIDATES: editor write-through of mac/address and unit ipv4/ipv6 address.
// PREVENTS: "discarding corrupt change file" / OS-divergent interface config syntax.
func TestWriteThroughInterfaceAddressRoundTrip(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	session := NewEditSession("insecure", "local")
	ed.SetSession(session)

	// mac/address (value contains ':'), unit ipv4 address (contains '/'), and unit
	// ipv6 address (contains '::' and '/').
	require.NoError(t, ed.SetValue([]string{"interface", "ethernet", "iface-test", "mac"}, "address", "00:aa:bb:cc:dd:01"))
	require.NoError(t, ed.SetValue([]string{"interface", "ethernet", "iface-test", "unit", "0", "ipv4"}, "address", "192.0.2.2/30"))
	require.NoError(t, ed.SetValue([]string{"interface", "ethernet", "iface-test", "unit", "0", "ipv6"}, "address", "fd00::1/64"))

	changePath := ChangePath(configPath, session.User)
	data, rerr := os.ReadFile(changePath) //nolint:gosec // test path
	require.NoError(t, rerr)
	assert.Contains(t, string(data), "mac address 00:aa:bb:cc:dd:01")
	// address is a bracket-syntax leaf-list, serialized as `address [ <cidr> ]`.
	assert.Contains(t, string(data), "unit 0 ipv4 address [ 192.0.2.2/30 ]")
	assert.Contains(t, string(data), "unit 0 ipv6 address [ fd00::1/64 ]")

	// The whole change file round-trips: no "unknown field" / "corrupt change file".
	_, _, _, perr := config.ParseChangeFile(string(data), config.NewSetParser(ed.schema))
	require.NoError(t, perr, "change file must re-parse cleanly")
}
