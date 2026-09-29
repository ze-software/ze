// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- NSSA Type-7 to Type-5 translation.
// Related: nssa_test.go -- the translator election and eligibility tests.
//
// VALIDATES: RFC 3101 section 3.2: the translated Type-5 copies network, mask, path type,
// metric, forwarding address and route tag from its CURRENT source Type-7, and carries the
// translator's Router ID as advertising router.
// PREVENTS: a translator that keeps the fields of an earlier instance of the Type-7, or that
// substitutes its own mask, path type, metric or tag.
package ospf

import (
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC3101-3.2-1 negative -- after the source Type-7 is re-originated with a
// different mask (255.255.255.0), path type (E2), metric (50), forwarding address (10.5.0.3) and
// route tag (12), the translated Type-5 advertised by the translator no longer carries any of
// the earlier values (255.255.0.0, E1, 33, 10.5.0.2, 7): each field follows the Type-7.
func TestRFC3101TranslatedType5FollowsSourceFields(t *testing.T) {
	// Goal: every field of the Type-5 comes from the Type-7, not from a stale copy or a
	// default. Method: translate, change every field of the Type-7, translate again.
	eng, nssa := nssaTransEngine(t, "10.0.6.9")
	// The LSDB clock is the test's, so the re-origination can pass MinLSInterval (RFC 2328
	// section 12.4) instead of being deferred.
	now := transTime
	eng.lsdb = ospflsdb.New(func() time.Time { return now })
	self := ridOf("10.0.6.9")
	asbr := ridOf("10.0.6.2")
	key := types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(ip4Of("10.20.0.0")), AdvertisingRouter: self}

	eng.lsdb.OriginateNSSA(nssa, asbr, ip4Of("10.20.0.0"), ip4Of("255.255.0.0"), false, 33, ip4Of("10.5.0.2"), 7, true)
	eng.translateNSSA(transTime)
	if _, ok := eng.lsdb.LookupLSA(types.BackboneArea, key); !ok {
		t.Fatal("no translated Type-5 for the first Type-7 instance")
	}

	now = now.Add(10 * time.Second) // twice the default 5-second MinLSInterval
	eng.lsdb.OriginateNSSA(nssa, asbr, ip4Of("10.20.0.0"), ip4Of("255.255.255.0"), true, 50, ip4Of("10.5.0.3"), 12, true)
	eng.translateNSSA(now)

	t5, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !ok {
		t.Fatal("translated Type-5 advertised by the translator's Router ID is missing")
	}
	body, err := t5.DecodeExternal()
	if err != nil {
		t.Fatalf("DecodeExternal: %v", err)
	}
	if body.NetworkMask != ip4Of("255.255.255.0") {
		t.Errorf("mask = %v, want the Type-7's 255.255.255.0", body.NetworkMask)
	}
	if !body.ExternalType2 {
		t.Error("path type = E1, want the Type-7's E2")
	}
	if body.Metric != 50 {
		t.Errorf("metric = %d, want the Type-7's 50", body.Metric)
	}
	if body.ForwardingAddr != ip4Of("10.5.0.3") {
		t.Errorf("forwarding address = %v, want the Type-7's 10.5.0.3", body.ForwardingAddr)
	}
	if body.ExternalRouteTag != 12 {
		t.Errorf("route tag = %d, want the Type-7's 12", body.ExternalRouteTag)
	}
}
