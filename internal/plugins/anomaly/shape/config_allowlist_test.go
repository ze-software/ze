// VALIDATES: the allowlist an operator writes reaches the responder's arming
// guard, whether they name one protected prefix or several.
// PREVENTS: a self-lockout on a protected source. Tree.ToMap collapses a
// one-member leaf-list to a bare string, and the parser asserted []any on it,
// so an operator who allowlisted exactly one prefix -- their own management
// network, the ordinary case -- got an empty allowlist and the responder armed
// a shaping action against that network. The existing coverage missed it twice:
// the config tests fed a JSON array of one, which no producer emits at one
// member, and the responder test set Config.Allowlist directly, which skips the
// parse the operator's config actually goes through.

package shape

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseConfigAllowlistSingleEntry feeds ParseConfig the shape the config
// pipeline emits for a leaf-list at each member count, single member first.
func TestParseConfigAllowlistSingleEntry(t *testing.T) {
	for _, tc := range []struct {
		name string
		json string
		want []netip.Prefix
	}{
		{
			name: "one prefix, the bare string ToMap emits at count one",
			json: `{"anomaly":{"shape":{"mode":"armed","allowlist":"10.0.0.0/8"}}}`,
			want: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		},
		{
			name: "two prefixes, the array ToMap emits at count two",
			json: `{"anomaly":{"shape":{"mode":"armed","allowlist":["10.0.0.0/8","192.168.0.0/16"]}}}`,
			want: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("192.168.0.0/16")},
		},
		{
			name: "no allowlist at all",
			json: `{"anomaly":{"shape":{"mode":"armed"}}}`,
			want: nil,
		},
		{
			name: "an unparseable prefix is dropped, the rest survive",
			json: `{"anomaly":{"shape":{"mode":"armed","allowlist":["not-a-prefix","10.0.0.0/8"]}}}`,
			want: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig(tc.json)
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.Allowlist)
		})
	}
}

// TestOnDetectedSkipsSingleAllowlistedPrefix drives the whole chain the
// operator's config takes, from the delivered JSON through ParseConfig into the
// responder, and asserts the arming guard is reached. Setting Config.Allowlist
// by hand would prove the guard works while leaving it unreachable, which is
// how this defect stayed live.
func TestOnDetectedSkipsSingleAllowlistedPrefix(t *testing.T) {
	cfg, err := ParseConfig(`{"anomaly":{"shape":{"mode":"armed","allowlist":"198.51.100.0/24"}}}`)
	require.NoError(t, err)
	require.NoError(t, cfg.Validate())

	tr := newTestResponder(t, cfg)
	tr.r.onDetected(det("198.51.100.5/32"))

	assert.Zero(t, tr.termCount(), "an allowlisted source must install no firewall term")
	assert.Zero(t, tr.r.armedCount, "an allowlisted source must not be armed")
}

// TestOnDetectedArmsASourceOutsideTheSingleAllowlistedPrefix is the negative
// half: the same one-prefix config must still arm everything the operator did
// NOT protect. Without it, a fix that returned an allowlist matching everything
// would pass the test above.
func TestOnDetectedArmsASourceOutsideTheSingleAllowlistedPrefix(t *testing.T) {
	cfg, err := ParseConfig(`{"anomaly":{"shape":{"mode":"armed","allowlist":"198.51.100.0/24"}}}`)
	require.NoError(t, err)
	require.NoError(t, cfg.Validate())

	tr := newTestResponder(t, cfg)
	tr.r.onDetected(det("203.0.113.5/32"))

	assert.Positive(t, tr.termCount(), "a source outside the allowlist must still be shaped")
	assert.Equal(t, 1, tr.r.armedCount)
}
