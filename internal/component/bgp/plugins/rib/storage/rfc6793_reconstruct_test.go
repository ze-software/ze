// RFC: rfc/short/rfc6793.md — the receive-side AS path reconstruction of Section 4.2.3
//
// Drives ParseAttributes (attrparse.go), the ingest path that turns a received
// UPDATE's attribute bytes into the interned RIB entry. Every case here enters
// through that entry point rather than through canonicalizeASPath, because the
// AGGREGATOR gate and the AS path merge are one procedure in the RFC and only
// the entry point runs both.

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pool "github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
)

var (
	// AS_PATH from an OLD speaker: AS_SEQUENCE [64500, 65001, AS_TRANS] in two
	// octets. Three AS numbers, one more than the AS4_PATH beside it carries.
	wireASPathThreeHops = []byte{
		0x40, 0x02, 0x08,
		0x02, 0x03, 0xFB, 0xF4, 0xFD, 0xE9, 0x5B, 0xA0,
	}

	// AS4_PATH: AS_SEQUENCE [199524] in four octets. One AS number.
	wireAS4PathOneHop = []byte{
		0xC0, 0x11, 0x06,
		0x02, 0x01, 0x00, 0x03, 0x0B, 0x64,
	}
)

// TestParseAttributesCostsNothingExtraWithoutAS4Path pins the same cost at the
// ingest entry point rather than at the AS path producer alone. An UPDATE from
// a session that negotiated four-octet AS support reaches neither the AGGREGATOR
// choice's read of the AS number nor the AS path reconstruction, so the whole
// parse is what it was before RFC 6793 Section 4.2.3 ran here.
//
// The interning pools deduplicate, so the second and later parses of one
// attribute set allocate only what the parse itself needs.
func TestParseAttributesCostsNothingExtraWithoutAS4Path(t *testing.T) {
	raw := concat(wireOriginIGP,
		[]byte{0x40, 0x02, 0x0A, 0x02, 0x02, 0x00, 0x00, 0xFB, 0xF4, 0x00, 0x00, 0xFD, 0xE9},
		wireNextHop)

	// Warm the pools, so the run measures the parse rather than the first
	// intern of each value.
	warm, err := ParseAttributes(raw)
	require.NoError(t, err)
	warm.Release()

	allocs := testing.AllocsPerRun(100, func() {
		entry, err := ParseAttributes(raw)
		if err != nil {
			t.Fatal(err)
		}
		entry.Release()
	})
	t.Logf("the common ingest path costs %.0f allocations per UPDATE", allocs)
	assert.Zero(t, allocs, "an UPDATE with no AS4_PATH must allocate nothing on ingest")
}

// TestParseAttributesReconciliesNothing pins what this function stopped doing.
//
// Until 2026-09-09 ParseAttributes performed the RFC 6793 Section 4.2.3
// reconstruction itself, and a test here asserted what that cost. The
// reconciliation now happens once, at ingest, so the bytes reaching this
// function already carry four-octet truth (attribute.ReconcileASPathFamily,
// reached from Session.processMessage).
//
// The assertion is therefore the opposite of the old one: handed an
// OLD-speaker shape, this function MUST NOT merge it. It interns what it was
// given. A ParseAttributes that reconstructed again would corrupt a path that
// was already correct, by taking a leading part twice.
func TestParseAttributesReconciliesNothing(t *testing.T) {
	raw := concat(wireOriginIGP, wireASPathThreeHops, wireAS4PathOneHop)

	entry, err := ParseAttributes(raw)
	require.NoError(t, err)
	defer entry.Release()

	stored, err := pool.ASPath.Get(entry.ASPath)
	require.NoError(t, err)
	assert.Equal(t, wireASPathThreeHops[3:], stored,
		"the AS_PATH is interned as it arrived, with no leading part taken from it")
}
