package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// adjOutTestKey keys raw, an announcement, the way the session writer does.
func adjOutTestKey(t *testing.T, fam family.Family, received uint32, raw []byte, addPath bool) adjOutKey {
	t.Helper()
	var key adjOutKey
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	require.NoError(t, fwdPathKeyFor(&key.path, fam, received, raw, false, scratch[:]))
	key.addPath = addPath
	return key
}

// TestAdjOutWithdrawalNLRILabeled proves a forwarded labeled route, whose
// key lost its label stack and which keeps no exact NLRI, is withdrawn with
// the RFC 8277 Section 2.4 Compatibility field 0x800000 and a length octet
// that counts it, its ADD-PATH identifier first.
func TestAdjOutWithdrawalNLRILabeled(t *testing.T) {
	labeled := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFI(4)}
	// Two labels (100, then 200 with S set) and 10.0.0.0/24: 24+24+24 bits.
	raw := []byte{72, 0x00, 0x06, 0x40, 0x00, 0x0c, 0x81, 10, 0, 0}
	key := adjOutTestKey(t, labeled, 9, raw, true)

	wire, err := adjOutWithdrawalNLRI(key, &adjRIBOutRoute{})
	require.NoError(t, err)
	require.Equal(t, []byte{0, 0, 0, 9, 48, 0x80, 0x00, 0x00, 10, 0, 0}, wire)
}

// TestAdjOutWithdrawalNLRIUnicast proves a plain prefix is written back from
// its key unchanged.
func TestAdjOutWithdrawalNLRIUnicast(t *testing.T) {
	raw := []byte{24, 192, 0, 2}
	key := adjOutTestKey(t, family.IPv4Unicast, 0, raw, false)

	wire, err := adjOutWithdrawalNLRI(key, &adjRIBOutRoute{})
	require.NoError(t, err)
	require.Equal(t, raw, wire)
}

// TestAdjOutWithdrawalNLRIOverflow proves an NLRI longer than the inline
// 33-octet fwdPathKey.nlri array (such as a 49-octet BGP-LS VPN NLRI stored in
// overflow) rebuilds its withdrawal and stages its announcement owe without
// slicing past the inline array bound.
func TestAdjOutWithdrawalNLRIOverflow(t *testing.T) {
	bgplsVPN := family.Family{AFI: family.AFI(16388), SAFI: family.SAFI(72)}
	// 49-octet BGP-LS VPN Node NLRI (exceeds fwdPathNLRIMax = 33).
	raw := []byte{
		0x00, 0x01, 0x00, 0x2d,
		0x00, 0x00, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x07,
		0x03,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00,
		0x01, 0x00, 0x00, 0x18,
		0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9,
		0x02, 0x01, 0x00, 0x04, 0x12, 0x34, 0x56, 0x78,
		0x02, 0x03, 0x00, 0x04, 0xc0, 0x00, 0x02, 0x09,
	}
	key := adjOutTestKey(t, bgplsVPN, 0, raw, false)
	require.NotEmpty(t, key.path.overflow)
	require.Greater(t, int(key.path.length), len(key.path.nlri))

	wire, err := adjOutWithdrawalNLRI(key, &adjRIBOutRoute{})
	require.NoError(t, err)
	require.Equal(t, raw, wire)

	var w adjOutWrite
	owe := w.stageAnnouncementOwe(bgplsVPN, key, raw)
	require.Equal(t, bgplsVPN, owe.family)
	require.Equal(t, raw, owe.nlri)
}

// TestAdjOutWithdrawalBatches proves the snapshot is per family, honors the
// family filter, and holds nothing for a session the table is not bound to.
func TestAdjOutWithdrawalBatches(t *testing.T) {
	session := &Session{}
	table := &adjRIBOut{session: session, routes: map[adjOutKey]*adjRIBOutRoute{
		adjOutTestKey(t, family.IPv4Unicast, 0, []byte{24, 192, 0, 2}, false): {},
		adjOutTestKey(t, family.IPv4Unicast, 0, []byte{24, 192, 0, 3}, false): {},
	}}

	batches, err := table.withdrawalBatches(session, family.Family{})
	require.NoError(t, err)
	require.Len(t, batches, 1)
	require.Len(t, batches[0].NLRIs, 2)

	batches, err = table.withdrawalBatches(session, family.IPv6Unicast)
	require.NoError(t, err)
	require.Empty(t, batches)

	batches, err = table.withdrawalBatches(&Session{}, family.Family{})
	require.NoError(t, err)
	require.Empty(t, batches)
}
