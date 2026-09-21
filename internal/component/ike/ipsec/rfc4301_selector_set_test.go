// VALIDATES: RFC 4301 Section 4.4.1.2 asks users not to put several selector sets in
// one SPD entry until IKE can convey them. Ze's entry holds ONE local prefix, ONE
// remote prefix, one protocol and one port pair, so an operator cannot write a
// second set: the structure meets the obligation, and this pins that structure.

package ipsec

import "testing"

// RFC requirement: RFC4301-4.4.1.2-1 positive -- an operator SPD entry is read as exactly
// one selector set: one local prefix, one remote prefix, one protocol and one port pair.
func TestRFC4301SPDEntryHoldsOneSelectorSet(t *testing.T) {
	cfg, err := ParseIPsecConfig(spdTree("one", map[string]string{"action": "bypass", "protocol": "17"},
		map[string]string{"prefix": "10.0.0.0/24", "port": "53"},
		map[string]string{"prefix": "10.1.0.0/24", "port": "any"}))
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	p, ok := cfg.Policies["one"]
	if !ok {
		t.Fatalf("entry one was not read; got %d entries", len(cfg.Policies))
	}
	if p.LocalPrefix == nil || p.LocalPrefix.String() != "10.0.0.0/24" {
		t.Errorf("local prefix %v, want the one written, 10.0.0.0/24", p.LocalPrefix)
	}
	if p.RemotePrefix == nil || p.RemotePrefix.String() != "10.1.0.0/24" {
		t.Errorf("remote prefix %v, want the one written, 10.1.0.0/24", p.RemotePrefix)
	}
	if p.Protocol != 17 || p.LocalPort.Port != 53 || !p.RemotePort.IsAny() {
		t.Errorf("protocol %d local port %d remote any=%v, want 17, 53, true", p.Protocol, p.LocalPort.Port, p.RemotePort.IsAny())
	}
}
