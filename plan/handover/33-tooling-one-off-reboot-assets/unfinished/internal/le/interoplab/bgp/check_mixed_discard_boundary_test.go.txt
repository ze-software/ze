// Design: docs/architecture/testing/interop.md -- before/after withdrawal and independent wire proof.
package bgp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type mixedDiscardBoundaryLab struct {
	recordingLab
	seedMissing, dropWithdrawal, duplicateNextHop bool
	released                                      bool
	capture                                       string
}

func (lab *mixedDiscardBoundaryLab) Query(_ context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (string, error) {
	query := strings.Join(command, " ")
	if strings.Contains(query, "send bgp ") {
		lab.released = true
		return `{"status":"ok"}`, nil
	}
	if peer == peerSpeaker2 && strings.Contains(query, "jsonl") {
		return lab.capture, nil
	}
	if strings.Contains(query, "show bgp peer list") {
		return `{"peers":{"172.30.0.9":{"state":"established"},"172.30.0.11":{"state":"established"}}}`, nil
	}
	if strings.Contains(query, "show bgp neighbor") {
		return "BGP neighbor is 172.30.0.11\nBGP state = Established\nBGP neighbor is 172.30.0.2\n", nil
	}
	if strings.Contains(query, "show ip route bgp") {
		if !strings.Contains(query, "json") {
			return "B>* 203.0.113.0/24 [20/0] via 172.30.0.9, eth0\n", nil
		}
		entries := map[string]any{}
		for _, prefix := range lab.prefixes() {
			entries[prefix] = []any{map[string]any{"prefix": prefix, "protocol": "bgp", "selected": true, "destSelected": true, "installed": true, "nexthops": []any{map[string]any{"ip": "172.30.0.9", "active": true, "fib": true}}}}
		}
		value, _ := json.Marshal(entries)
		return string(value), nil
	}
	if strings.Contains(query, "show bgp ipv4 unicast") {
		// The old checker never announces the seed and asks its absence directly.
		if strings.Contains(query, "198.51.100.0/24") {
			return `{}`, nil
		}
		if strings.Contains(query, " json") && !strings.Contains(query, "/24") {
			routes := map[string]any{}
			for _, prefix := range lab.prefixes() {
				routes[prefix] = []any{map[string]any{"prefix": prefix, "aspath": map[string]any{"string": "65004"}, "nexthops": []any{map[string]any{"ip": "172.30.0.9"}}}}
			}
			value, _ := json.Marshal(map[string]any{"vrfId": 0, "vrfName": "default", "tableVersion": 0, "routerId": "172.30.0.3", "localAS": 65002, "routes": routes})
			return string(value), nil
		}
		return `{"prefix":"10.0.0.0/24","paths":[{"aspath":{"string":"65004"},"nexthops":[{"ip":"172.30.0.9"}]}]}`, nil
	}
	return "", fmt.Errorf("unexpected mixed witness query: %s %s", peer, query)
}

func (lab *mixedDiscardBoundaryLab) prefixes() []string {
	prefixes := []string{"10.0.0.0/24"}
	if !lab.seedMissing && (!lab.released || lab.dropWithdrawal) {
		prefixes = append(prefixes, "198.51.100.0/24")
	}
	if lab.released {
		prefixes = append(prefixes, "203.0.113.0/24", "198.51.101.0/24")
	}
	return prefixes
}

func TestMixedNativeCheckerRequiresSeedReleaseRemovalAndStrictWire(t *testing.T) {
	for _, test := range []struct {
		name                     string
		missing, drop, duplicate bool
	}{
		{"complete", false, false, false}, {"never announced", true, false, false}, {"withdraw dropped", false, true, false}, {"duplicate NEXT_HOP accepted by FRR", false, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			open, err := speakerOpen(65001, 90, net.ParseIP("172.30.0.2"), []bgpFamily{{1, 1}}, false, false)
			if err != nil {
				t.Fatal(err)
			}
			attrs, err := hex.DecodeString("4001010040020602010000fdec400304ac1e0009")
			if err != nil {
				t.Fatal(err)
			}
			var capture strings.Builder
			encoder := json.NewEncoder(&capture)
			write := func(frame []byte) {
				row := extendedRelayFrame{Original: hex.EncodeToString(frame)}
				if frame[18] == bgpOpen {
					row.Delivered = row.Original
				}
				if err := encoder.Encode(row); err != nil {
					t.Fatal(err)
				}
			}
			write(open)
			write(speakerKeepalive())
			write(speakerMessage(bgpUpdate, buildSpeakerUpdate(attrs, []byte{24, 10, 0, 0})))
			write(speakerMessage(bgpUpdate, buildSpeakerUpdate(attrs, []byte{24, 198, 51, 100})))
			if !test.drop {
				write(speakerMessage(bgpUpdate, []byte{0, 4, 24, 198, 51, 100, 0, 0}))
			}
			if test.duplicate {
				attrs = append(attrs, 0x40, 3, 4, 172, 30, 0, 9)
			}
			write(speakerMessage(bgpUpdate, buildSpeakerUpdate(attrs, []byte{24, 203, 0, 113})))
			write(speakerMessage(bgpUpdate, buildSpeakerUpdate(attrs, []byte{24, 198, 51, 101})))
			lab := &mixedDiscardBoundaryLab{seedMissing: test.missing, dropWithdrawal: test.drop, duplicateNextHop: test.duplicate, capture: capture.String()}
			ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
			defer cancel()
			err = checkRFC7606MixedUpdate(ctx, &interoplab.CheckContext{Lab: lab, Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")}})
			if test.name == "complete" {
				if err != nil {
					t.Fatal(err)
				}
				if !lab.released {
					t.Fatal("checker never released the mixed UPDATE after observing seed installation")
				}
			} else if err == nil {
				t.Fatal("native checker accepted missing before/after or inexact wire evidence")
			}
		})
	}
}
