// Design: docs/guide/graceful-restart.md -- What Ze Advertises
// RFC: rfc/short/rfc4724.md -- Section 4, the families a speaker may list
// Overview: gr_capability.go -- extractGRCapabilities, parseGRCapValue
// Related: rfc4724_gr_capability_test.go -- grConfig, grPayloadForConfig

package gr

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ribLessFamily is the name registerRIBLessFamily gives the family it declares.
const ribLessFamily = "ipv4/rib-less"

// registerRIBLessFamily declares, for the life of one test, an address family
// the way an external plugin declares one at runtime (RegisterFamilyBatch, the
// same call the plugin server makes), with no NLRI splitter behind it. The RIB
// plugin cannot store such a family's routes, so after a restart nothing in Ze
// holds them to send again (OWNER RULING 7 and 8(a)).
func registerRIBLessFamily(t *testing.T) {
	t.Helper()
	declared := family.FamilyRegistration{AFI: family.AFIIPv4, SAFI: family.SAFI(241), AFIName: "ipv4", SAFIName: "rib-less"}
	added, err := family.RegisterFamilyBatch([]family.FamilyRegistration{declared})
	require.NoError(t, err, "the runtime family registers")
	t.Cleanup(func() { family.UnregisterFamilyBatch(added) })

	fam, known := family.LookupFamily(ribLessFamily)
	require.True(t, known, "precondition: the family index knows %s", ribLessFamily)
	require.False(t, nlrisplit.Supported(fam), "precondition: no splitter frames %s", ribLessFamily)
}

// captureGRWarnings routes the package logger to a buffer at warn level for
// the life of one test.
func captureGRWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	withGRLoggerRestored(t)
	var buf bytes.Buffer
	SetLogger(slogutil.LoggerWithOutput(grSubsystem, "warn", &buf))
	return &buf
}

// TestRFC4724GRCapabilityOmitsAFamilyNoRIBStores reads the code-64 payload for
// a session that carries a family the RIB plugin cannot store, from the
// configuration text through extractGRCapabilities.
//
// VALIDATES: RFC 4724 Section 4: "A BGP speaker MAY advertise the Graceful
// Restart Capability for an address family to its peer if it has the ability to
// preserve its forwarding state for the address family when BGP restarts."
// PREVENTS: a tuple promising preservation for a family whose routes Ze cannot
// keep, so cannot send again after the restart.
//
// RFC requirement: RFC4724-4-4 negative -- a session carrying ipv4/unicast and
// a runtime-registered family with no NLRI splitter receives the payload
// "007800010100": the Restart Time 120 and the ipv4/unicast tuple alone. A
// graceful-restart family list naming only that family is narrowed to nothing,
// and the capability is still sent, as "0078". Each drop logs a warning naming
// the peer and the family.
func TestRFC4724GRCapabilityOmitsAFamilyNoRIBStores(t *testing.T) {
	registerRIBLessFamily(t)

	t.Run("session-families", func(t *testing.T) {
		warnings := captureGRWarnings(t)
		payload := grPayloadForConfig(t, grConfig([]string{"ipv4/unicast", ribLessFamily}, ""))
		// 0078       Restart Flags 0, Restart Time 120 seconds (the YANG default)
		// 0001 01 00 AFI 1 (IPv4), SAFI 1 (unicast), F bit clear
		assert.Equal(t, "007800010100", payload, "only the family the RIB stores is listed")
		assert.Contains(t, warnings.String(), "peer=peer1", "the warning names the peer")
		assert.Contains(t, warnings.String(), "family="+ribLessFamily, "the warning names the dropped family")
	})

	t.Run("configured-family-no-rib-stores", func(t *testing.T) {
		warnings := captureGRWarnings(t)
		payload := grPayloadForConfig(t, grConfig([]string{"ipv4/unicast", ribLessFamily},
			"\t\t\t\t\tfamily {\n\t\t\t\t\t\tname [ "+ribLessFamily+" ];\n\t\t\t\t\t}\n"))
		assert.Equal(t, "0078", payload, "the capability is sent with no tuple")
		assert.Contains(t, warnings.String(), "peer=peer1", "the warning names the peer")
		assert.Contains(t, warnings.String(), "family="+ribLessFamily, "the warning names the dropped family")
	})
}

// TestRFC4724GRCapabilityKeepsEveryFamilyTheRIBStores is the contrast: a
// compiled-in family other than unicast, which the RIB plugin stores through
// its own splitter, stays in the capability and draws no warning.
//
// RFC requirement: RFC4724-4-4 positive -- a session carrying ipv4/flow and
// ipv4/unicast, both families with an NLRI splitter, receives the payload
// "0078" + "00018500" + "00010100": one tuple for each, F bit clear, and the
// warning buffer stays empty.
func TestRFC4724GRCapabilityKeepsEveryFamilyTheRIBStores(t *testing.T) {
	warnings := captureGRWarnings(t)
	fam, known := family.LookupFamily("ipv4/flow")
	require.True(t, known, "precondition: the family index knows ipv4/flow")
	require.True(t, nlrisplit.Supported(fam), "precondition: a splitter frames ipv4/flow")

	payload := grPayloadForConfig(t, grConfig([]string{"ipv4/unicast", "ipv4/flow"}, ""))
	// 0078       Restart Flags 0, Restart Time 120 seconds
	// 0001 85 00 AFI 1 (IPv4), SAFI 133 (flow), F bit clear
	// 0001 01 00 AFI 1 (IPv4), SAFI 1 (unicast), F bit clear
	assert.Equal(t, "00780001850000010100", payload, "both families the RIB stores are listed")
	assert.Empty(t, warnings.String(), "no family was dropped, so nothing is logged")
}
