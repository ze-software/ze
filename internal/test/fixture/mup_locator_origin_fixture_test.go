package fixture

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/locatororigin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestMUPFixtureHelpers verifies the pure parsing, snapshot, and assertion
// helpers used by the MUP Segment Discovery functional test fixtures.
//
// VALIDATES: argument parsing, locator-origin snapshot construction, replay
// envelope detection, Adj-RIB-In keep/absent checks, report warning/error
// checks, and show doctor code checks.
// PREVENTS: silent false-green functional test assertions from misparsed flags
// or mismatched JSON keys.
func TestMUPFixtureHelpers(t *testing.T) {
	t.Run("parseMUPObserverArgs", func(t *testing.T) {
		opts, err := parseMUPObserverArgs([]string{
			"--fence=10.0.0.0/24",
			"--wait-file=locator.ready",
			"--ready-file=peers.ready",
			"--keep=010004110000FDE90000000140C000020100000001",
			"--absent=0100020C0000FDE900000001C0000201",
			"--expect-warning=doctor-bgp-mup-locator-unresolved",
			"--expect-doctor=doctor-bgp-mup-locator-unresolved",
			"--expect-error=doctor-bgp-mup-locator-mismatch",
		})
		require.NoError(t, err)
		assert.Equal(t, "10.0.0.0/24", opts.fence)
		assert.Equal(t, "locator.ready", opts.waitFile)
		assert.Equal(t, "peers.ready", opts.readyFile)
		assert.Equal(t, []string{"010004110000fde90000000140c000020100000001"}, opts.keepHex)
		assert.Equal(t, []string{"0100020c0000fde900000001c0000201"}, opts.absentHex)
		assert.Equal(t, "doctor-bgp-mup-locator-unresolved", opts.expectWarning)
		assert.Equal(t, "doctor-bgp-mup-locator-unresolved", opts.expectDoctor)
		assert.Equal(t, "doctor-bgp-mup-locator-mismatch", opts.expectError)

		_, err = parseMUPObserverArgs([]string{"--ready-file="})
		assert.Error(t, err)
		_, err = parseMUPObserverArgs([]string{"--wait-file="})
		assert.Error(t, err)
		_, err = parseMUPObserverArgs([]string{"--keep="})
		assert.Error(t, err)
		_, err = parseMUPObserverArgs([]string{"--absent="})
		assert.Error(t, err)
		_, err = parseMUPObserverArgs([]string{"--unknown=1"})
		assert.Error(t, err)

		require.NoError(t, checkMUPRSPeersReady(map[string]any{
			"peers": []any{
				map[string]any{"address": "127.0.0.1", "up": true, "families": []any{"ipv4/unicast", "ipv4/mup"}},
				map[string]any{"address": "127.0.0.2", "up": true, "families": []any{"ipv4/unicast", "ipv4/mup"}},
			},
		}, 2))
		assert.Error(t, checkMUPRSPeersReady(map[string]any{
			"peers": []any{
				map[string]any{"address": "127.0.0.1", "up": true, "families": []any{"ipv4/unicast", "ipv4/mup"}},
				map[string]any{"address": "127.0.0.2", "up": true, "families": []any{}},
			},
		}, 2))
		require.NoError(t, checkMUPRSPeersReady(map[string]any{
			"peers": []any{map[string]any{"address": "127.0.0.1", "up": true, "families": []any{"ipv4/unicast", "ipv4/mup"}}},
		}, 1))
	})

	t.Run("buildMUPLocatorFixtureSnapshot and replay detection", func(t *testing.T) {
		snap := buildMUPLocatorFixtureSnapshot("locator-publisher", 1, true)
		assert.Equal(t, "locator-publisher", snap.Source)
		assert.Equal(t, uint64(1), snap.Generation)
		assert.True(t, snap.Active)
		require.Len(t, snap.Entries, 1)
		assert.Equal(t, netip.MustParsePrefix("2001:db8::/48"), snap.Entries[0].Prefix)
		assert.True(t, snap.Entries[0].Known)
		assert.Equal(t, "0100.0000.0001", snap.Entries[0].Originator)
		assert.Contains(t, snap.Entries[0].Addresses, netip.MustParseAddr("2001:db8:140::1"))

		inactive := buildMUPLocatorFixtureSnapshot("locator-publisher", 2, false)
		assert.False(t, inactive.Active)
		assert.Empty(t, inactive.Entries)

		reqEnv, err := json.Marshal(rpc.EventEnvelope{
			Namespace: locatororigin.Namespace,
			Event:     locatororigin.EventSnapshotRequest,
			Payload:   json.RawMessage("null"),
		})
		require.NoError(t, err)
		assert.True(t, isLocatorOriginReplayRequest(string(reqEnv)))
		assert.False(t, isLocatorOriginReplayRequest(`{"namespace":"bgp","event":"state"}`))
		assert.False(t, isLocatorOriginReplayRequest("not-json"))
	})

	t.Run("checkMUPAdjRIBInData", func(t *testing.T) {
		doc := map[string]any{
			"adj-rib-in": map[string]any{
				"127.0.0.1": []any{
					map[string]any{"nlri-hex": "010004110000FDE90000000140C000020100000001"},
				},
			},
		}
		require.NoError(t, checkMUPAdjRIBInData(doc,
			[]string{"010004110000fde90000000140c000020100000001"},
			[]string{"0100020c0000fde900000001c0000201"},
		))
		assert.Error(t, checkMUPAdjRIBInData(doc, []string{"0100020c0000fde900000001c0000201"}, nil))
		assert.Error(t, checkMUPAdjRIBInData(doc, nil, []string{"010004110000fde90000000140c000020100000001"}))
	})

	t.Run("checkMUPReportIssues and checkMUPDoctorData", func(t *testing.T) {
		warnings := map[string]any{
			"warnings": []any{
				map[string]any{
					"code":   "doctor-bgp-mup-locator-unresolved",
					"source": "bgp-nlri-mup",
					"fix":    "ze show isis database",
				},
			},
		}
		require.NoError(t, checkMUPReportIssues(warnings, "warnings", "doctor-bgp-mup-locator-unresolved", "ze show isis database"))
		assert.Error(t, checkMUPReportIssues(warnings, "warnings", "doctor-bgp-mup-locator-unresolved", "wrong fix"))
		assert.Error(t, checkMUPReportIssues(warnings, "warnings", "missing-code", ""))

		doctorDoc := map[string]any{
			"diagnostics": []any{
				map[string]any{"code": "doctor-bgp-mup-locator-unresolved"},
			},
		}
		require.NoError(t, checkMUPDoctorData(doctorDoc, "doctor-bgp-mup-locator-unresolved"))
		assert.Error(t, checkMUPDoctorData(doctorDoc, "doctor-bgp-mup-locator-mismatch"))
	})
}
