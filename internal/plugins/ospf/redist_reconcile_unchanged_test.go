// VALIDATES: RFC 2328 Section 12.4 -- a reconciliation pass that replays retained
// redistribution intents with unchanged contents originates no new instance of the
// Type-5 or Type-7 LSA and installs nothing, in both address families.
// PREVENTS: VyOS T9448 (every commit made ospfd flush and re-originate its Type-5 LSAs).
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// reconcileEngine builds an ASBR attached to the backbone (interface "a") and to NSSA
// 0.0.0.7 (interface "b"), so one redistributed prefix yields both a Type-5 and a Type-7.
// The LSDB clock is the returned pointer, so a test can step past MinLSInterval.
func reconcileEngine(t *testing.T, f nssaTestFamily) (*engine, *time.Time) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.7.1","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0","area-type":"normal"},"0.0.0.7":{"area-id":"0.0.0.7","area-type":"nssa"}}},"redistribute":{"static":{"source":"static"}}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	var codec Codec = v4Codec{}
	if f.v3 {
		codec = v6Codec{}
	}
	eng := newEngineWithCodecAF(transport.New(&fakeBackend{}), codec, f.af)
	t.Cleanup(eng.shutdown)
	now := time.Unix(1000, 0)
	eng.lsdb = ospflsdb.New(func() time.Time { return now })
	eng.setConfig(cfg)
	eng.running["a"] = interfaceConfig{Name: "a", AreaID: types.BackboneArea}
	eng.running["b"] = interfaceConfig{Name: "b", AreaID: types.AreaID{0, 0, 0, 7}}
	eng.forwardingAddress = func(string) (netip.Addr, bool) {
		if f.af.isIPv4() {
			return netip.MustParseAddr("192.0.2.2"), true
		}
		return netip.MustParseAddr("2001:db8:7::2"), true
	}
	return eng, &now
}

// reconcileType5Header returns the header of this router's AS-wide external LSA for prefix.
func reconcileType5Header(t *testing.T, eng *engine, f nssaTestFamily, prefix netip.Prefix) packet.LSAHeader {
	t.Helper()
	var key types.LSAKey
	if f.v3 {
		key = v6ExternalKey(eng.cfg.RouterID, eng.redistV6[prefix])
	} else {
		key = types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(prefix.Addr().As4()), AdvertisingRouter: eng.cfg.RouterID}
	}
	lsa, exists := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !exists || lsa.Header.Age.IsMaxAge() {
		t.Fatalf("Type-5 for %s missing or purged: exists=%v header=%+v", prefix, exists, lsa.Header)
	}
	return lsa.Header
}

// TestReconcileUnchangedImportOriginatesNothing injects one prefix and runs the
// reconciliation pass a configuration commit runs, twice, each time past MinLSInterval so
// a re-origination would not be rate-limited. The first pass may originate LSAs that did
// not exist yet (the NSSA border router's Type-7 default); the second pass is a commit
// that changes nothing. RFC 2328 Section 12.4: "These events should cause new
// originations if and only if the contents of the new LSA would be different". The
// Type-5 and Type-7 keep their sequence numbers through both passes and the second pass
// installs nothing, so nothing floods. The final step changes the route tag and proves
// the same pass does re-originate a changed LSA, so the unchanged result is not
// rate-limiting.
// MUTATION: drop the existingSelfBodyUnchanged short-circuit in OriginateExternal,
// OriginateNSSA or OriginateSelf.
func TestReconcileUnchangedImportOriginatesNothing(t *testing.T) {
	for _, f := range nssaTestFamilies {
		t.Run(f.name, func(t *testing.T) {
			eng, now := reconcileEngine(t, f)
			prefix := nssaImportPrefix(f)
			if err := eng.InjectExternal(prefix, "static", staticExternal(77)); err != nil {
				t.Fatal(err)
			}
			type5 := reconcileType5Header(t, eng, f, prefix)
			type7, exists := nssaImportedLSA(t, eng, f, prefix)
			if !exists || type7.header.Age.IsMaxAge() {
				t.Fatalf("Type-7 missing or purged: exists=%v header=%+v", exists, type7.header)
			}

			installs := 0
			eng.lsdb.SetOnChange(func(types.AreaID) { installs++ })
			for pass := 1; pass <= 2; pass++ {
				installs = 0
				*now = now.Add(6 * time.Second)
				eng.reconcileExternalImports()
				if after := reconcileType5Header(t, eng, f, prefix); after.Sequence != type5.Sequence {
					t.Fatalf("pass %d re-originated the unchanged Type-5: sequence %v -> %v", pass, type5.Sequence, after.Sequence)
				}
				after7, exists := nssaImportedLSA(t, eng, f, prefix)
				if !exists || after7.header.Age.IsMaxAge() || after7.header.Sequence != type7.header.Sequence {
					t.Fatalf("pass %d touched the unchanged Type-7: before=%+v after=%+v exists=%v", pass, type7.header, after7.header, exists)
				}
			}
			if installs != 0 {
				t.Fatalf("a commit that changes nothing installed %d LSA(s), want none flooded", installs)
			}

			// Positive control: a changed route tag through the same pass re-originates.
			eng.nssaMu.Lock()
			imported := eng.externalImports[prefix]
			imported.ext.Tag = 91
			eng.externalImports[prefix] = imported
			eng.nssaMu.Unlock()
			*now = now.Add(6 * time.Second)
			eng.reconcileExternalImports()
			if after := reconcileType5Header(t, eng, f, prefix); after.Sequence != type5.Sequence.Next() {
				t.Fatalf("changed tag did not re-originate the Type-5: sequence %v -> %v", type5.Sequence, after.Sequence)
			}
		})
	}
}
