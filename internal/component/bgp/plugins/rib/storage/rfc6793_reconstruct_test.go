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

	// AS_PATH from an OLD speaker: AS_SEQUENCE [64500, AS_TRANS]. Two AS numbers.
	wireASPathTwoHops = []byte{
		0x40, 0x02, 0x06,
		0x02, 0x02, 0xFB, 0xF4, 0x5B, 0xA0,
	}

	// AS4_PATH: AS_SEQUENCE [65001, 199524, 199525]. Three AS numbers, one more
	// than the AS_PATH beside it carries.
	wireAS4PathThreeHops = []byte{
		0xC0, 0x11, 0x0E,
		0x02, 0x03, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x03, 0x0B, 0x64, 0x00, 0x03, 0x0B, 0x65,
	}

	// AS_PATH from an OLD speaker inside a confederation: AS_CONFED_SEQUENCE
	// [65001, 65002] leading, then AS_SEQUENCE [64500, AS_TRANS]. RFC 5065
	// leaves the confederation segment out of the AS number count, so this path
	// counts two AS numbers.
	wireASPathConfedLeading = []byte{
		0x40, 0x02, 0x0C,
		0x03, 0x02, 0xFD, 0xE9, 0xFD, 0xEA,
		0x02, 0x02, 0xFB, 0xF4, 0x5B, 0xA0,
	}

	// AS_PATH whose confederation segment TRAILS the sequence: AS_SEQUENCE
	// [64500, AS_TRANS] then AS_CONFED_SEQUENCE [65001]. Two AS numbers.
	wireASPathConfedTrailing = []byte{
		0x40, 0x02, 0x0A,
		0x02, 0x02, 0xFB, 0xF4, 0x5B, 0xA0,
		0x03, 0x01, 0xFD, 0xE9,
	}

	// AS4_PATH: AS_SEQUENCE [199524, 199525]. Two AS numbers.
	wireAS4PathTwoHops = []byte{
		0xC0, 0x11, 0x0A,
		0x02, 0x02, 0x00, 0x03, 0x0B, 0x64, 0x00, 0x03, 0x0B, 0x65,
	}

	// AGGREGATOR from an OLD speaker carrying a real two-octet AS (64500), not
	// AS_TRANS, with aggregator address 10.0.0.1.
	wireAggregatorRealAS = []byte{
		0xC0, 0x07, 0x06,
		0xFB, 0xF4, 0x0A, 0x00, 0x00, 0x01,
	}
)

// entryASPath returns the AS path bytes ParseAttributes interned for entry.
func entryASPath(t *testing.T, entry RouteEntry) []byte {
	t.Helper()
	require.True(t, entry.HasASPath(), "the route must carry an AS path")
	got, err := pool.ASPath.Get(entry.ASPath)
	require.NoError(t, err)
	return got
}







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

// AS_PATH from an OLD speaker carrying an AS_SET in its leading part:
// AS_SEQUENCE [64500], AS_SET [65001, 65002], AS_SEQUENCE [AS_TRANS].
// RFC 4271 Section 9.1.2.2 counts the set as one, so this path counts three.
var wireASPathWithLeadingSet = []byte{
	0x40, 0x02, 0x0E,
	0x02, 0x01, 0xFB, 0xF4,
	0x01, 0x02, 0xFD, 0xE9, 0xFD, 0xEA,
	0x02, 0x01, 0x5B, 0xA0,
}

// AS4_PATH whose segment claims three AS numbers and carries one. The attribute
// length is honest, so the iterator hands the value over and the AS4_PATH parse
// is what refuses it.
var wireAS4PathMalformed = []byte{
	0xC0, 0x11, 0x06,
	0x02, 0x03, 0x00, 0x03, 0x0B, 0x64,
}

// AGGREGATOR in the four-octet form, which is the wrong width for a session
// that did not negotiate the four-octet AS capability. RFC 7606 Section 7.7
// rejects every length but the negotiated one.
var wireAggregatorWrongWidth = []byte{
	0xC0, 0x07, 0x08,
	0x00, 0x00, 0xFB, 0xF4, 0x0A, 0x00, 0x00, 0x01,
}





