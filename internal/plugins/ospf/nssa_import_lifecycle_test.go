// VALIDATES: RFC 3101 forwarding-address replacement and per-source P-bit
// configuration through engine injection, link-down handling and withdrawal.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
)

func nssaImportEngine(t *testing.T, f nssaTestFamily) (*engine, *time.Time) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.7.1","areas":{"area":{"0.0.0.7":{"area-id":"0.0.0.7","area-type":"nssa"}}},"redistribute":{"static":{"source":"static","nssa-propagate":true,"metric":"33","tag":"77"},"connected":{"source":"connected","nssa-propagate":false},"bgp":{"source":"bgp"}}}}`), nil)
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
	for _, name := range []string{"a", "b", "unused"} {
		ic := interfaceConfig{Name: name, AreaID: types.AreaID{0, 0, 0, 7}, NetworkType: types.NetworkLoopback}
		eng.running[name] = ic
		runtime := ospfiface.New(ospfiface.Config{Name: name, AreaID: ic.AreaID, RouterID: cfg.RouterID, NetworkType: types.NetworkLoopback}, nil, ospfiface.NopMetrics())
		runtime.Start()
		eng.interfaces[name] = runtime
		t.Cleanup(runtime.Stop)
	}
	eng.forwardingAddress = func(name string) (netip.Addr, bool) {
		if name == "unused" {
			return netip.Addr{}, false
		}
		if f.af.isIPv4() {
			if name == "a" {
				return netip.MustParseAddr("192.0.2.1"), true
			}
			return netip.MustParseAddr("192.0.2.2"), true
		}
		if name == "a" {
			return netip.MustParseAddr("2001:db8:7::1"), true
		}
		return netip.MustParseAddr("2001:db8:7::2"), true
	}
	return eng, &now
}

func nssaImportedLSA(t *testing.T, eng *engine, f nssaTestFamily, prefix netip.Prefix) (packet.LSAHeader, netip.Addr, bool, uint32, uint32, bool) {
	t.Helper()
	key := types.LSAKey{Type: types.LSTypeNSSA, AdvertisingRouter: eng.cfg.RouterID}
	if f.v3 {
		lsid := eng.redistV6[prefix]
		if prefix.Bits() == 0 {
			lsid = v6NSSADefaultLSID
		}
		key = v6NSSAKey(eng.cfg.RouterID, lsid)
	} else {
		key.LinkStateID = types.LinkStateID(prefix.Addr().As4())
	}
	lsa, exists := eng.lsdb.LookupLSA(types.AreaID{0, 0, 0, 7}, key)
	if !exists {
		return packet.LSAHeader{}, netip.Addr{}, false, 0, 0, false
	}
	if f.v3 {
		decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
		if err != nil {
			t.Fatal(err)
		}
		body, err := decoded.DecodeExternal()
		if err != nil {
			t.Fatal(err)
		}
		return lsa.Header, v6ForwardingAddr(body.ForwardingAddr, f.af), ospfv3packet.NSSAPropagate(body), body.Metric, body.ExternalRouteTag, true
	}
	body, err := lsa.DecodeExternal()
	if err != nil {
		t.Fatal(err)
	}
	return lsa.Header, netip.AddrFrom4(body.ForwardingAddr), lsa.Header.Options.Has(types.OptionNP), body.Metric, body.ExternalRouteTag, true
}

func nssaImportPrefix(f nssaTestFamily) netip.Prefix {
	if f.af.isIPv4() {
		return netip.MustParsePrefix("203.0.113.0/24")
	}
	return netip.MustParsePrefix("2001:db8:100::/64")
}

// RFC requirement: RFC3101-2.3-3 positive -- the engine's interface-down callback replaces the forwarding address and re-originates each imported Type-7 without another redistribution event.
// MUTATION: remove reconcileExternalImports from onInterfaceDown, or retain Down interfaces in externalInterfaces.
func TestNSSAImportReoriginatesAfterForwardingInterfaceDown(t *testing.T) {
	for _, f := range nssaTestFamilies {
		t.Run(f.name, func(t *testing.T) {
			eng, now := nssaImportEngine(t, f)
			prefix := nssaImportPrefix(f)
			if err := eng.InjectExternal(prefix, "static", 91); err != nil {
				t.Fatal(err)
			}
			before, fa, propagate, _, _, exists := nssaImportedLSA(t, eng, f, prefix)
			first, _ := eng.forwardingAddress("a")
			if !exists || fa != first || !propagate {
				t.Fatalf("initial Type-7 exists=%v fa=%v P=%v", exists, fa, propagate)
			}
			*now = now.Add(6 * time.Second)
			eng.onInterfaceDown(1, "a")
			after, fa, propagate, metric, tag, exists := nssaImportedLSA(t, eng, f, prefix)
			second, _ := eng.forwardingAddress("b")
			if !exists || after.Age.IsMaxAge() || after.Sequence != before.Sequence.Next() || fa != second || !propagate || metric != 33 || tag != 91 {
				t.Fatalf("replacement header=%+v fa=%v P=%v metric=%d tag=%d exists=%v", after, fa, propagate, metric, tag, exists)
			}
			*now = now.Add(6 * time.Second)
			eng.onInterfaceDown(2, "b")
			withdrawn, _, _, _, _, exists := nssaImportedLSA(t, eng, f, prefix)
			if !exists || !withdrawn.Age.IsMaxAge() {
				t.Fatalf("last forwarding address lost: header=%+v exists=%v, want MaxAge", withdrawn, exists)
			}
		})
	}
}

// RFC requirement: RFC3101-2.3-3 negative -- taking down an unrelated interface leaves the Type-7 instance unchanged, and a withdrawn import is never replayed on a later interface event.
// MUTATION: purge every Type-7 on any link-down, or retain imports after WithdrawExternal.
func TestNSSAImportIgnoresUnrelatedDownAndWithdrawnIntent(t *testing.T) {
	for _, f := range nssaTestFamilies {
		t.Run(f.name, func(t *testing.T) {
			eng, now := nssaImportEngine(t, f)
			prefix := nssaImportPrefix(f)
			if err := eng.InjectExternal(prefix, "static", 0); err != nil {
				t.Fatal(err)
			}
			before, beforeFA, _, _, _, _ := nssaImportedLSA(t, eng, f, prefix)
			*now = now.Add(6 * time.Second)
			eng.onInterfaceDown(3, "unused")
			after, fa, _, _, _, exists := nssaImportedLSA(t, eng, f, prefix)
			if !exists || after.Sequence != before.Sequence || after.Age.IsMaxAge() || fa != beforeFA {
				t.Fatalf("unrelated interface changed Type-7: before=%+v after=%+v fa=%v", before, after, fa)
			}
			if removed, err := eng.WithdrawExternal(prefix); err != nil || !removed {
				t.Fatalf("withdraw removed=%v err=%v", removed, err)
			}
			*now = now.Add(6 * time.Second)
			eng.onInterfaceDown(1, "a")
			after, _, _, _, _, exists = nssaImportedLSA(t, eng, f, prefix)
			if exists && !after.Age.IsMaxAge() {
				t.Fatalf("withdrawn route replayed: %+v", after)
			}
		})
	}
}

// RFC requirement: RFC3101-x-3 positive -- the parsed per-source nssa-propagate leaf sets P on imported routes, including the default, through InjectExternal in every AF.
// MUTATION: externalPropagate always returns false or ignores the source's configured leaf.
func TestNSSAPerSourcePropagationEnabled(t *testing.T) {
	for _, f := range nssaTestFamilies {
		t.Run(f.name, func(t *testing.T) {
			eng, _ := nssaImportEngine(t, f)
			prefix, _ := eng.defaultRoute()
			for _, route := range []netip.Prefix{nssaImportPrefix(f), prefix} {
				if err := eng.InjectExternal(route, "static", 0); err != nil {
					t.Fatal(err)
				}
				h, fa, propagate, metric, tag, exists := nssaImportedLSA(t, eng, f, route)
				if !exists || h.Age.IsMaxAge() || !propagate || !fa.IsValid() || fa.IsUnspecified() || metric != 33 || tag != 77 {
					t.Fatalf("configured source %s: header=%+v fa=%v P=%v metric=%d tag=%d exists=%v", route, h, fa, propagate, metric, tag, exists)
				}
			}
		})
	}
}

// RFC requirement: RFC3101-x-3 negative -- explicit false and an unconfigured source keep P clear despite a usable forwarding address; one source's true setting cannot enable another.
// MUTATION: restore automatic P-bit selection whenever the NSSA has a forwarding address.
func TestNSSAPerSourcePropagationDisabled(t *testing.T) {
	for _, f := range nssaTestFamilies {
		t.Run(f.name, func(t *testing.T) {
			for _, source := range []string{"connected", "bgp"} {
				eng, _ := nssaImportEngine(t, f)
				prefix := nssaImportPrefix(f)
				if err := eng.InjectExternal(prefix, source, 0); err != nil {
					t.Fatal(err)
				}
				h, _, propagate, _, _, exists := nssaImportedLSA(t, eng, f, prefix)
				if !exists || h.Age.IsMaxAge() || propagate {
					t.Fatalf("source %s: exists=%v header=%+v P=%v, want live P-clear Type-7", source, exists, h, propagate)
				}
			}
		})
	}
}

// RFC requirement: RFC3101-x-3 negative -- an enabled source cannot request duplicate translation while the router also originates its Type-5 twin.
// MUTATION: externalPropagate ignores canType5.
func TestNSSAPerSourcePropagationCannotOverrideType5(t *testing.T) {
	for _, f := range nssaTestFamilies {
		t.Run(f.name, func(t *testing.T) {
			eng, _ := nssaImportEngine(t, f)
			cfg := eng.cfg
			cfg.Areas = append(cfg.Areas, areaConfig{AreaID: types.BackboneArea, AreaType: types.AreaTypeNormal})
			eng.setConfig(cfg)
			eng.running["backbone"] = interfaceConfig{Name: "backbone", AreaID: types.BackboneArea}
			prefix := nssaImportPrefix(f)
			if err := eng.InjectExternal(prefix, "static", 0); err != nil {
				t.Fatal(err)
			}
			header, _, propagate, _, _, exists := nssaImportedLSA(t, eng, f, prefix)
			if !exists || header.Age.IsMaxAge() || propagate {
				t.Fatalf("Type-7 twin: header=%+v P=%v exists=%v, want live P-clear", header, propagate, exists)
			}
			key := types.LSAKey{Type: types.LSTypeASExternal, AdvertisingRouter: cfg.RouterID}
			if f.v3 {
				key = v6ExternalKey(cfg.RouterID, eng.redistV6[prefix])
			} else {
				key.LinkStateID = types.LinkStateID(prefix.Addr().As4())
			}
			external, exists := eng.lsdb.LookupLSA(types.BackboneArea, key)
			if !exists || external.Header.Age.IsMaxAge() {
				t.Fatalf("Type-5 twin: header=%+v exists=%v, want live", external.Header, exists)
			}
		})
	}
}
