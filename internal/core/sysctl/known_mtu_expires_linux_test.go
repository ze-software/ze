// Design: docs/guide/ipsec.md -- Path MTU of a tunnel
// Related: known_linux.go -- the path MTU aging keys
//
// VALIDATES: the two path MTU aging periods are known keys, so a setting the
// operator writes under `sysctl { setting ... }` is checked against the range
// before Ze writes it: a period of at least one second is accepted and 0 or a
// negative value, which would expire a learned PMTU the moment it is learned, is
// refused.
// PREVENTS: an IPsec SA whose outer path PMTU never stays learned long enough for
// a packet to reach the peer and an ICMP error to come back (RFC 4301 Section
// 8.2.2), set by a typo with no refusal.

//go:build linux

package sysctl

import "testing"

func TestPathMTUAgingPeriodIsAKnownBoundedKey(t *testing.T) {
	for _, key := range []string{"net.ipv4.route.mtu_expires", "net.ipv6.route.mtu_expires"} {
		def, ok := Lookup(key)
		if !ok {
			t.Fatalf("%s is not a known key, so a configured period is never range-checked", key)
		}
		if def.Type != TypeIntRange {
			t.Fatalf("%s has type %v, want TypeIntRange", key, def.Type)
		}
		for _, good := range []string{"1", "600", "2147483"} {
			if err := Validate(key, good); err != nil {
				t.Errorf("%s=%s refused: %v", key, good, err)
			}
		}
		for _, bad := range []string{"0", "-1", "2147484", "ten"} {
			if err := Validate(key, bad); err == nil {
				t.Errorf("%s=%s accepted; the period must be between 1 and 2147483 seconds", key, bad)
			}
		}
	}
}
