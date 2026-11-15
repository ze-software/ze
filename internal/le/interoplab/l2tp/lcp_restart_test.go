// Design: docs/labs/l2tp-interop.md -- a discriminating configured-Restart oracle.
// Related: lcp_restart.go -- wire observation reader and final verdict.
package l2tp

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// restartTrace constructs independent wire bytes, including the CDN on the
// same tunnel/session as the three requests. The healthy data plane is a real
// lab precondition and is not represented by this pure oracle fixture.
func restartTrace(t *testing.T) []restartWire {
	t.Helper()
	request := pppDatagram(protocolLCP, []byte{1, 8, 0, 4})
	injected := pppDatagram(protocolLCP, []byte{1, 0x61, 0, 4})
	reply := pppDatagram(protocolLCP, []byte{2, 8, 0, 4})
	control := make([]byte, 20)
	binary.BigEndian.PutUint16(control, 0xc802)
	binary.BigEndian.PutUint16(control[2:4], uint16(len(control)))
	// pppDatagram's bare header carries Tunnel7, Session9.
	binary.BigEndian.PutUint16(control[4:6], 7)
	binary.BigEndian.PutUint16(control[6:8], 9)
	copy(control[12:18], []byte{0x80, 8, 0, 0, 0, 0})
	binary.BigEndian.PutUint16(control[18:20], 14)
	event := func(direction string, packet []byte, elapsed time.Duration, drop bool) restartWire {
		encoded := hex.EncodeToString(packet)
		forwarded := encoded
		if drop {
			forwarded = ""
		}
		return restartWire{direction, encoded, forwarded, int64(elapsed), true}
	}
	return []restartWire{
		event(relayInjected, injected, 0, false),
		event(relayFromZe, request, time.Millisecond, false),
		event(relayFromLAC, reply, 2*time.Millisecond, true),
		event(relayFromZe, request, 501*time.Millisecond, false),
		event(relayFromZe, request, 1001*time.Millisecond, false),
		event(relayFromZe, control, 1501*time.Millisecond, false),
	}
}

// TestLCPRestartWireOracle checks both a conforming wire trace and the broken
// implementations the scenario exists to reject, without starting Docker.
func TestLCPRestartWireOracle(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func([]restartWire) []restartWire
	}{
		{name: "configured"},
		{name: "fixed3s", change: func(e []restartWire) []restartWire {
			e[3].ElapsedNanos = int64(3 * time.Second)
			e[4].ElapsedNanos = int64(6 * time.Second)
			e[5].ElapsedNanos = int64(9 * time.Second)
			return e
		}},
		{name: "fixed1s", change: func(e []restartWire) []restartWire {
			e[3].ElapsedNanos = int64(1001 * time.Millisecond)
			e[4].ElapsedNanos = int64(2001 * time.Millisecond)
			e[5].ElapsedNanos = int64(3001 * time.Millisecond)
			return e
		}},
		{name: "extra request", change: func(e []restartWire) []restartWire { return append(e[:5], append([]restartWire{e[4]}, e[5:]...)...) }},
		{name: "early expiry", change: func(e []restartWire) []restartWire { e[5].ElapsedNanos = int64(1100 * time.Millisecond); return e }},
		{name: "no real peer reply", change: func(e []restartWire) []restartWire { return append(e[:2], e[3:]...) }},
		{name: "reply was forwarded", change: func(e []restartWire) []restartWire { e[2].Forwarded = e[2].Original; return e }},
		{name: "no CDN", change: func(e []restartWire) []restartWire { return e[:5] }},
		{name: "another session CDN", change: func(e []restartWire) []restartWire {
			packet, err := hex.DecodeString(e[5].Original)
			if err != nil {
				t.Fatal(err)
			}
			packet[7]++
			e[5].Original = hex.EncodeToString(packet)
			e[5].Forwarded = e[5].Original
			return e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := restartTrace(t)
			if tc.change != nil {
				events = tc.change(events)
			}
			var text strings.Builder
			for _, event := range events {
				encoded, err := json.Marshal(event)
				if err != nil {
					t.Fatal(err)
				}
				text.Write(encoded)
				text.WriteByte('\n')
			}
			evidence, err := readRestartEvidence(text.String())
			if err == nil {
				err = requireRestartEvidence(evidence)
			}
			if tc.change == nil {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("broken wire trace passed the configured-Restart oracle")
			}
		})
	}
}

// TestLCPRestartScenarioPreparation proves the named operator scenario reaches
// the real Ze, transparent relay and independent xl2tpd peers in that order.
func TestLCPRestartScenarioPreparation(t *testing.T) {
	root := l2tpCheckout(t)
	plans, needsKernel, err := plansAt(root, scenarioLCPRestart, testEnvironment())
	if err != nil {
		t.Fatal(err)
	}
	if !needsKernel {
		t.Fatal("Restart scenario omitted kernel preflight")
	}
	if len(plans) != 1 {
		t.Fatalf("selected %d plans, want1", len(plans))
	}
	prepared, err := plans[0].Prepare(t.Context(), interoplab.PrepareContext{
		Source: plans[0].Source, Network: interoplab.Network{Name: "restart", IPv4: netip.MustParsePrefix("172.29.0.0/24")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Peers) != 3 {
		t.Fatalf("peers = %d, want3", len(prepared.Peers))
	}
	for index, name := range []string{peerZe, peerAVPRelay, peerLAC} {
		if prepared.Peers[index].Name != name {
			t.Fatalf("peer%d = %s, want%s", index, prepared.Peers[index].Name, name)
		}
	}
	relayBuilt := false
	for _, image := range imageBuilds(root, plans, "unused") {
		if image.Name == imageAVPRelay {
			relayBuilt = true
		}
	}
	if !relayBuilt {
		t.Fatal("Restart scenario does not build its relay image")
	}
}
