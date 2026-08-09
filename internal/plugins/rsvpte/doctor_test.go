// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- raw-socket readiness doctor check tests

package rsvpte

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestCheckRSVPTERawSocketUnavailable(t *testing.T) {
	// VALIDATES: rsvp-te configured + no raw socket -> doctor-rsvpte-rawsock-unavailable.
	// PREVENTS: RSVP-TE silently failing to signal because CAP_NET_RAW is missing.
	old := rsvpRawSocketProbe
	rsvpRawSocketProbe = func() bool { return false }
	t.Cleanup(func() { rsvpRawSocketProbe = old })

	tree := config.NewTree()
	tree.GetOrCreateContainer("rsvp-te")

	diags := checkRSVPTERawSocket(registry.DoctorCheckContext{Tree: tree})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-rsvpte-rawsock-unavailable", diags[0].Code)
	assert.Equal(t, "warning", diags[0].Severity)
}

func TestCheckRSVPTERawSocketAvailable(t *testing.T) {
	// VALIDATES: when the raw socket opens, no warning is emitted.
	old := rsvpRawSocketProbe
	rsvpRawSocketProbe = func() bool { return true }
	t.Cleanup(func() { rsvpRawSocketProbe = old })

	tree := config.NewTree()
	tree.GetOrCreateContainer("rsvp-te")
	assert.Empty(t, checkRSVPTERawSocket(registry.DoctorCheckContext{Tree: tree}))
}

func TestCheckRSVPTERawSocketAbsentConfig(t *testing.T) {
	// VALIDATES: the check fires only when rsvp-te is configured.
	// PREVENTS: doctor warning about RSVP-TE on boxes that do not use it.
	old := rsvpRawSocketProbe
	rsvpRawSocketProbe = func() bool { return false }
	t.Cleanup(func() { rsvpRawSocketProbe = old })

	assert.Empty(t, checkRSVPTERawSocket(registry.DoctorCheckContext{Tree: config.NewTree()}))
	assert.Empty(t, checkRSVPTERawSocket(registry.DoctorCheckContext{Tree: nil}))
}

func TestRSVPTEDoctorCheckRegistered(t *testing.T) {
	// VALIDATES: rsvp-te registers the rsvp-te-rawsock doctor check via the plugin
	// registry, so `ze doctor` runs it and removing rsvp-te removes the check.
	checks := registry.PluginDoctorChecks()
	found := false
	for _, c := range checks {
		if c.Name == "rsvp-te-rawsock" {
			found = true
			break
		}
	}
	assert.True(t, found,
		"doctor check rsvp-te-rawsock not registered via Registration.DoctorChecks")
}
