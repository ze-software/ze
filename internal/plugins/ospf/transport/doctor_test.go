// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- OSPF raw-socket doctor tests

package transport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestCheckOSPFRawSocketUnavailable(t *testing.T) {
	old := rawSocketProbe
	rawSocketProbe = func() bool { return false }
	t.Cleanup(func() { rawSocketProbe = old })
	tree := config.NewTree()
	tree.GetOrCreateContainer("ospf")
	diags := checkOSPFRawSocket(diagnostic.DoctorCheckContext{Tree: tree})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-ospf-raw-socket", diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
}

func TestCheckOSPFRawSocketAvailable(t *testing.T) {
	old := rawSocketProbe
	rawSocketProbe = func() bool { return true }
	t.Cleanup(func() { rawSocketProbe = old })
	tree := config.NewTree()
	tree.GetOrCreateContainer("ospf")
	assert.Empty(t, checkOSPFRawSocket(diagnostic.DoctorCheckContext{Tree: tree}))
}

func TestCheckOSPFRawSocketAbsentConfig(t *testing.T) {
	old := rawSocketProbe
	rawSocketProbe = func() bool { return false }
	t.Cleanup(func() { rawSocketProbe = old })
	assert.Empty(t, checkOSPFRawSocket(diagnostic.DoctorCheckContext{Tree: config.NewTree()}))
}

func TestOSPFDoctorRawSocketCodeRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	meta := diagnostic.Lookup("doctor-ospf-raw-socket")
	require.NotNil(t, meta)
	assert.Equal(t, "OSPF raw IP socket unavailable", meta.Title)
}

func TestOSPFDoctorRawSocketCheckRegistered(t *testing.T) {
	assert.Contains(t, diagnostic.DoctorCheckNames(), "ospf-raw-socket")
}
