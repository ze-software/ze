// VALIDATES: spec-ospfv3-3-ipv6-transport AC-7, AC-15 -- the doctor check fires
// doctor-ospfv3-raw-socket when OSPFv3 is configured and the raw IPv6 socket
// cannot be opened (CAP_NET_RAW), is silent when it can or when OSPFv3 is not
// configured, and the code + check are registered. PREVENTS a runtime failure
// surfacing only as a silent loop after the daemon starts.

package transport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestOSPFv3CheckRawSocketUnavailable(t *testing.T) {
	old := rawSocketProbe
	rawSocketProbe = func() bool { return false }
	t.Cleanup(func() { rawSocketProbe = old })
	tree := config.NewTree()
	tree.GetOrCreateContainer("ospfv3")
	diags := checkOSPFv3RawSocket(diagnostic.DoctorCheckContext{Tree: tree})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-ospfv3-raw-socket", diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
}

func TestOSPFv3CheckRawSocketAvailable(t *testing.T) {
	old := rawSocketProbe
	rawSocketProbe = func() bool { return true }
	t.Cleanup(func() { rawSocketProbe = old })
	tree := config.NewTree()
	tree.GetOrCreateContainer("ospfv3")
	assert.Empty(t, checkOSPFv3RawSocket(diagnostic.DoctorCheckContext{Tree: tree}))
}

func TestOSPFv3CheckRawSocketAbsentConfig(t *testing.T) {
	old := rawSocketProbe
	rawSocketProbe = func() bool { return false }
	t.Cleanup(func() { rawSocketProbe = old })
	assert.Empty(t, checkOSPFv3RawSocket(diagnostic.DoctorCheckContext{Tree: config.NewTree()}))
}

func TestOSPFv3DoctorCodeRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	meta := diagnostic.Lookup("doctor-ospfv3-raw-socket")
	require.NotNil(t, meta)
	assert.Equal(t, "OSPFv3 raw IPv6 socket unavailable", meta.Title)
}

func TestOSPFv3DoctorRawSocketCheckRegistered(t *testing.T) {
	assert.Contains(t, diagnostic.DoctorCheckNames(), "ospfv3-raw-socket")
}
