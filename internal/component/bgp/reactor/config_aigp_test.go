// Design: docs/architecture/route-selection.md -- explicit AIGP configuration vs RFC 7311 iBGP default
// RFC: rfc/short/rfc7311.md, Section 3.3
// Related: config_aigp.go -- applyAIGPSettings and PeerSettings.AIGPEnabled
package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAIGPDefaultPathStillSelectsByAIGP verifies that an iBGP peer without an
// explicit `session { aigp { ... } }` block keeps RFC 7311 Section 3.3 default
// AIGPEnabled() == true while AIGPConfigured stays false, whereas explicit
// `enabled false` clears both and explicit `originate true` sets both.
//
// VALIDATES: AIGPConfigured distinguishes explicit operator configuration from
// the RFC 7311 Section 3.3 iBGP default without altering AIGPEnabled() (S-8,
// AC-15, S26).
// PREVENTS: an iBGP peer with no written `aigp` block either losing RFC 7311
// default AIGP selection or falsely triggering explicit-AIGP commit checks.
func TestAIGPDefaultPathStillSelectsByAIGP(t *testing.T) {
	baseTree := func(aigp map[string]any) map[string]any {
		session := map[string]any{
			"asn": map[string]any{
				"local":  "65000",
				"remote": "65000",
			},
		}
		if aigp != nil {
			session["aigp"] = aigp
		}
		return map[string]any{
			"connection": map[string]any{
				"remote": map[string]any{"ip": "192.0.2.10"},
				"local":  map[string]any{"ip": "192.0.2.1"},
			},
			"session": session,
		}
	}

	t.Run("ibgp_default_unwritten", func(t *testing.T) {
		ps, err := parsePeerFromTree("ibgp-default", baseTree(nil), 65000, 0x01020304)
		require.NoError(t, err)
		assert.True(t, ps.IsIBGP())
		assert.False(t, ps.AIGPConfigured, "unwritten aigp block must leave AIGPConfigured false")
		assert.True(t, ps.AIGPEnabled(), "RFC 7311 Section 3.3 iBGP default must stay enabled")
	})

	t.Run("ibgp_explicit_disabled", func(t *testing.T) {
		ps, err := parsePeerFromTree("ibgp-off", baseTree(map[string]any{"enabled": "false"}), 65000, 0x01020304)
		require.NoError(t, err)
		assert.False(t, ps.AIGPConfigured, "explicit enabled false must keep AIGPConfigured false")
		assert.False(t, ps.AIGPEnabled(), "explicit enabled false must disable AIGPEnabled")
	})

	t.Run("ibgp_explicit_originate", func(t *testing.T) {
		ps, err := parsePeerFromTree("ibgp-originate", baseTree(map[string]any{"originate": "true"}), 65000, 0x01020304)
		require.NoError(t, err)
		assert.True(t, ps.AIGPConfigured, "explicit originate block must set AIGPConfigured true")
		assert.True(t, ps.AIGPEnabled(), "explicit originate on iBGP must keep AIGPEnabled true")
	})
}
