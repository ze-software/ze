//go:build linux

package firewallnft

import (
	"testing"

	"github.com/google/nftables"
	"golang.org/x/sys/unix"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

// VALIDATES: P0 -- every valid (hook, family) pair that the forward
// path lowers produces a readable reverse mapping.
// PREVENTS: `ze firewall show` rendering "unknown hook" for configs we
// just applied.
func TestRaiseHookRoundTrip(t *testing.T) {
	type tc struct {
		hook   firewall.ChainHook
		family firewall.TableFamily
	}
	tests := []tc{
		{firewall.HookInput, firewall.FamilyInet},
		{firewall.HookOutput, firewall.FamilyInet},
		{firewall.HookForward, firewall.FamilyInet},
		{firewall.HookPrerouting, firewall.FamilyIP},
		{firewall.HookPostrouting, firewall.FamilyIP6},
		{firewall.HookIngress, firewall.FamilyNetdev},
		{firewall.HookEgress, firewall.FamilyNetdev},
	}
	for _, tt := range tests {
		t.Run(tt.hook.String(), func(t *testing.T) {
			forwardHook, err := lowerHook(tt.hook)
			if err != nil {
				t.Fatalf("lowerHook(%v): %v", tt.hook, err)
			}
			forwardFam, err := lowerFamily(tt.family)
			if err != nil {
				t.Fatalf("lowerFamily(%v): %v", tt.family, err)
			}
			got, ok := raiseHook(forwardHook, forwardFam)
			if !ok {
				t.Fatalf("raiseHook(%v, %v) returned !ok", forwardHook, forwardFam)
			}
			if got != tt.hook {
				t.Errorf("round trip: got %v, want %v", got, tt.hook)
			}
		})
	}
}

// VALIDATES: nil / out-of-range inputs do not panic.
func TestRaiseHookNil(t *testing.T) {
	if _, ok := raiseHook(nil, nftables.TableFamilyINet); ok {
		t.Error("raiseHook(nil) must return !ok")
	}
	// 99 is not a known netfilter hook.
	bogus := nftables.ChainHook(99)
	if _, ok := raiseHook(&bogus, nftables.TableFamilyINet); ok {
		t.Error("raiseHook(bogus) must return !ok")
	}
}

// VALIDATES: netdev family disambiguates ingress/egress from the
// overlapping INET hook numbers (NF_INET_LOCAL_IN and NF_NETDEV_INGRESS
// are both 0; NF_INET_LOCAL_OUT and NF_NETDEV_EGRESS are both 1).
// PREVENTS: a netdev-family ingress chain being reported as `input`.
func TestRaiseHookNetdevDisambiguation(t *testing.T) {
	ingress := nftables.ChainHook(unix.NF_NETDEV_INGRESS)
	egress := nftables.ChainHook(unix.NF_NETDEV_EGRESS)

	if got, ok := raiseHook(&ingress, nftables.TableFamilyNetdev); !ok || got != firewall.HookIngress {
		t.Errorf("netdev 0 = %v (ok=%v), want HookIngress", got, ok)
	}
	if got, ok := raiseHook(&ingress, nftables.TableFamilyINet); !ok || got != firewall.HookInput {
		t.Errorf("inet 0 = %v (ok=%v), want HookInput", got, ok)
	}
	if got, ok := raiseHook(&egress, nftables.TableFamilyNetdev); !ok || got != firewall.HookEgress {
		t.Errorf("netdev 1 = %v (ok=%v), want HookEgress", got, ok)
	}
	if got, ok := raiseHook(&egress, nftables.TableFamilyINet); !ok || got != firewall.HookOutput {
		t.Errorf("inet 1 = %v (ok=%v), want HookOutput", got, ok)
	}
}

// VALIDATES: every chain policy, chain type, and set type round-trips.
func TestRaiseEnumsRoundTrip(t *testing.T) {
	policies := []firewall.Policy{firewall.PolicyAccept, firewall.PolicyDrop}
	for _, p := range policies {
		raw, err := lowerPolicy(p)
		if err != nil {
			t.Fatalf("lowerPolicy(%v): %v", p, err)
		}
		got, ok := raisePolicy(&raw)
		if !ok || got != p {
			t.Errorf("policy round trip: %v -> %v (ok=%v)", p, got, ok)
		}
	}
	types := []firewall.ChainType{firewall.ChainFilter, firewall.ChainNAT, firewall.ChainRoute}
	for _, ct := range types {
		raw, err := lowerChainType(ct)
		if err != nil {
			t.Fatalf("lowerChainType(%v): %v", ct, err)
		}
		got, ok := raiseChainType(raw)
		if !ok || got != ct {
			t.Errorf("chain type round trip: %v -> %v (ok=%v)", ct, got, ok)
		}
	}
	setTypes := []firewall.SetType{
		firewall.SetTypeIPv4, firewall.SetTypeIPv6, firewall.SetTypeEther,
		firewall.SetTypeInetService, firewall.SetTypeMark, firewall.SetTypeIfname,
	}
	for _, st := range setTypes {
		raw, err := lowerSetType(st)
		if err != nil {
			t.Fatalf("lowerSetType(%v): %v", st, err)
		}
		got, ok := raiseSetType(raw)
		if !ok || got != st {
			t.Errorf("set type round trip: %v -> %v (ok=%v)", st, got, ok)
		}
	}
}

// VALIDATES: P0 -- set element keys decoded from the kernel match the
// string form the operator wrote into config, closing the round-trip.
// PREVENTS: `ze firewall show group blocked` emitting hex blobs or
// malformed values.
func TestEncodeDecodeSetElementRoundTrip(t *testing.T) {
	tests := []struct {
		typ firewall.SetType
		in  string
	}{
		{firewall.SetTypeIPv4, "10.0.0.5"},
		{firewall.SetTypeIPv6, "2001:db8::1"},
		{firewall.SetTypeInetService, "443"},
		{firewall.SetTypeMark, "0x10"},
		{firewall.SetTypeEther, "aa:bb:cc:dd:ee:ff"},
		{firewall.SetTypeIfname, "eth0"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			key, err := encodeSetElementKey(tt.typ, tt.in)
			if err != nil {
				t.Fatalf("encodeSetElementKey(%v, %q): %v", tt.typ, tt.in, err)
			}
			got := decodeSetElementKey(tt.typ, key)
			if got != tt.in {
				t.Errorf("round trip: %q -> %q", tt.in, got)
			}
		})
	}
}

// VALIDATES: decoded keys with unexpected lengths fall back to hex
// rather than panicking. A future kernel extension that packs extra
// bytes into a key must not blow up CLI readback.
func TestDecodeSetElementKeyMalformed(t *testing.T) {
	got := decodeSetElementKey(firewall.SetTypeIPv4, []byte{0x01})
	if got == "" {
		t.Error("decoder returned empty string on malformed key")
	}
	if got[0] != '0' {
		t.Errorf("got %q, expected hex-prefixed fallback", got)
	}
}
