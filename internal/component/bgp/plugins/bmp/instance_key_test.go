package bmp

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rdPeerHeader answers an RD Instance Peer header for the IPv4 peer 10.0.0.1 in
// the instance the distinguisher names.
func rdPeerHeader(distinguisher uint64) PeerHeader {
	ph := PeerHeader{PeerType: PeerTypeL3VPN, Distinguisher: distinguisher, PeerAS: 65001, PeerBGPID: 0x01020304}
	copy(ph.Address[12:], []byte{10, 0, 0, 1})
	return ph
}

// TestBMPCompositeKeyIsUniquePerInstance proves the RIB key separates every
// pair of peers peerStateKey separates.
//
// VALIDATES: two RD Instance Peers (and two Local Instance Peers) with one
// address on one router get distinct keys; a Global Instance Peer keeps the
// "<router>:<peer-address>" form; a V-flagged IPv4-mapped peer does not share
// the key of an IPv4 peer of the same address.
// PREVENTS: RFC 7854 Section 4.2 instances merging in one RIB-In pool, where a
// Peer Down for one withdraws the routes of both.
// Method: build headers that differ only in the field under test.
func TestBMPCompositeKeyIsUniquePerInstance(t *testing.T) {
	const router = "192.0.2.8:12345"

	first := bmpCompositeKey(router, rdPeerHeader(1))
	second := bmpCompositeKey(router, rdPeerHeader(2))
	if first == second {
		t.Errorf("two RD instance peers share the key %q", first)
	}
	if first != "192.0.2.8:12345:instance:1:1:10.0.0.1" {
		t.Errorf("RD instance key = %q, want %q", first, "192.0.2.8:12345:instance:1:1:10.0.0.1")
	}

	local := rdPeerHeader(1)
	local.PeerType = PeerTypeLocal
	if key := bmpCompositeKey(router, local); key == first {
		t.Errorf("a Local Instance Peer shares the key %q of the RD Instance Peer with its distinguisher", key)
	}

	global := rdPeerHeader(0)
	global.PeerType = PeerTypeGlobal
	if key := bmpCompositeKey(router, global); key != "192.0.2.8:12345:10.0.0.1" {
		t.Errorf("global instance key = %q, want the unchanged %q", key, "192.0.2.8:12345:10.0.0.1")
	}

	mapped := global
	mapped.Flags = PeerFlagV
	copy(mapped.Address[10:12], []byte{0xff, 0xff})
	if key := bmpCompositeKey(router, mapped); key != "192.0.2.8:12345:::ffff:10.0.0.1" {
		t.Errorf("V-flagged IPv4-mapped key = %q, want %q", key, "192.0.2.8:12345:::ffff:10.0.0.1")
	}

	// peerStateKey and bmpCompositeKey must agree on what one peer is.
	padded := global
	copy(padded.Address[10:12], []byte{0xff, 0xff})
	if peerStateKey(router, padded) != peerStateKey(router, global) {
		t.Error("an IPv4 peer with non-zero padding is a second state row for the one RIB key")
	}
}

// TestShowBMPPeersListsEachRDInstanceUnderItsOwnKey proves `show bmp peers`
// reports two RD instance peers of one address as two rows with two rib-peers.
//
// VALIDATES: the rib-peer the looking glass names a BMP protocol by is distinct
// for each instance.
// PREVENTS: protocols/bmp listing one name for two peers.
func TestShowBMPPeersListsEachRDInstanceUnderItsOwnKey(t *testing.T) {
	bp := &BMPPlugin{state: newBMPState()}
	bp.state.peerUp("192.0.2.8:12345", rdPeerHeader(1), nil, nil)
	bp.state.peerUp("192.0.2.8:12345", rdPeerHeader(2), nil, nil)

	_, data, err := bp.handleCommand("show bmp peers")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Peers []struct {
			RIBPeer string `json:"rib-peer"`
		} `json:"peers"`
	}
	if err := json.Unmarshal(marshalAny(data), &result); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, p := range result.Peers {
		keys = append(keys, p.RIBPeer)
	}
	slices.Sort(keys)
	want := []string{"192.0.2.8:12345:instance:1:1:10.0.0.1", "192.0.2.8:12345:instance:1:2:10.0.0.1"}
	if !slices.Equal(keys, want) {
		t.Errorf("rib-peers = %q, want %q", keys, want)
	}
}

// TestRDInstancePeersReachTheRIBUnderSeparateKeys proves the routes of two RD
// instance peers of one address go to the RIB under two keys, and that a Peer
// Down of one withdraws only its own.
//
// VALIDATES: Route Monitoring injects under bmpCompositeKey for each instance,
// and Peer Down sends withdraw-protocol for that instance's key alone. The RIB
// stores and withdraws by that key (rib_inject.go withdrawAllForPeer,
// TestWithdrawAllForPeer).
// PREVENTS: a Peer Down for one VRF's peer withdrawing another VRF's routes.
// Method: run the plugin over a pipe and read what it asks the engine.
func TestRDInstancePeersReachTheRIBUnderSeparateKeys(t *testing.T) {
	bp := &BMPPlugin{stopCh: make(chan struct{}), state: newBMPState()}
	engine := startReloadEngine(t, bp, "")

	const router = "192.0.2.8:12345"
	update := make([]byte, bgpHeaderSize+4) // header plus empty withdrawn and attribute lengths
	done := make(chan struct{})
	go func() {
		defer close(done)
		bp.processRouteMonitoring(router, &RouteMonitoring{Peer: rdPeerHeader(1), BGPUpdate: update})
		bp.processRouteMonitoring(router, &RouteMonitoring{Peer: rdPeerHeader(2), BGPUpdate: update})
		bp.processPeerDown(router, &PeerDown{Peer: rdPeerHeader(1), Reason: 2})
	}()

	var injected, withdrawn []string
	for len(injected)+len(withdrawn) < 3 {
		select {
		case request := <-engine.mux.Requests():
			switch request.Method {
			case rpc.MethodInjectWireRoute:
				var in rpc.InjectWireRouteInput
				if err := json.Unmarshal(request.Params, &in); err != nil {
					t.Fatal(err)
				}
				injected = append(injected, in.PeerKey)
			case rpc.MethodDispatchCommandArgs:
				var in rpc.DispatchCommandArgsInput
				if err := json.Unmarshal(request.Params, &in); err != nil {
					t.Fatal(err)
				}
				withdrawn = append(withdrawn, in.Command+" "+in.Args[len(in.Args)-1])
			}
			if err := engine.mux.SendOK(engine.ctx, request.ID); err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("plugin stopped asking: injected %q, withdrawn %q", injected, withdrawn)
		}
	}
	<-done

	keyOne := bmpCompositeKey(router, rdPeerHeader(1))
	keyTwo := bmpCompositeKey(router, rdPeerHeader(2))
	if keyOne == keyTwo {
		t.Fatalf("both instances are stored under %q, so the Peer Down withdraws both", keyOne)
	}
	if !slices.Equal(injected, []string{keyOne, keyTwo}) {
		t.Errorf("routes injected under %q, want %q then %q", injected, keyOne, keyTwo)
	}
	if want := []string{"request bgp rib withdraw-protocol " + keyOne}; !slices.Equal(withdrawn, want) {
		t.Errorf("peer down asked %q, want %q", withdrawn, want)
	}
}
