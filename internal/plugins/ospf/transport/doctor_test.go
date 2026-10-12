// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- OSPF raw-socket doctor tests

package transport

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestCheckOSPFRawSocketUnavailable(t *testing.T) {
	old := rawSocketProbe
	rawSocketProbe = func() error { return errProbeRefused }
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
	rawSocketProbe = func() error { return nil }
	t.Cleanup(func() { rawSocketProbe = old })
	tree := config.NewTree()
	tree.GetOrCreateContainer("ospf")
	assert.Empty(t, checkOSPFRawSocket(diagnostic.DoctorCheckContext{Tree: tree}))
}

func TestCheckOSPFRawSocketAbsentConfig(t *testing.T) {
	old := rawSocketProbe
	rawSocketProbe = func() error { return errProbeRefused }
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

// errProbeRefused stands in for any reason the platform probe gives.
var errProbeRefused = errors.New("probe refused")

// TestCheckOSPFRawSocketCarriesProbeReason proves the warning carries the reason the probe
// gives, so a host that is not Linux reads that the transport is Linux-only
// instead of a CAP_NET_RAW hint that cannot apply there.
//
// VALIDATES: the diagnostic message is the probe's own reason, word for word.
// PREVENTS: a fixed capability message hiding why the transport cannot run.
//
// Method: swap the rawSocketProbe seam for one refusing with a known reason.
func TestCheckOSPFRawSocketCarriesProbeReason(t *testing.T) {
	const reason = "the OSPF raw transport is Linux-only, and this host runs plan9"
	old := rawSocketProbe
	rawSocketProbe = func() error { return errors.New(reason) }
	t.Cleanup(func() { rawSocketProbe = old })
	tree := config.NewTree()
	tree.GetOrCreateContainer("ospf")

	diags := checkOSPFRawSocket(diagnostic.DoctorCheckContext{Tree: tree})
	require.Len(t, diags, 1)
	assert.Equal(t, reason, diags[0].Message)
}
