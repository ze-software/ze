// Design: docs/research/l2tpv2-ze-integration.md -- LCP negotiation lifetime bounds

package ppp

import (
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// TestLCPAuthenticationRestartReceivesFreshConvergenceBound spends most of the
// initial LCP deadline, opens into CHAP, then sends an RCR during authentication.
// The nested restart must receive its own full three-second Nak-loop bound.
// Observing only the outer loop's Opened boolean misses both nested edges.
func TestLCPAuthenticationRestartReceivesFreshConvergenceBound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, events := configuredLCPSession(t, LCPStateInitial, 500*time.Millisecond, 2, 3, 5)
		s.configuredAuthMethod = AuthMethodCHAPMD5
		s.authTimeout = 10 * time.Second
		peer := runLifecycleSession(t, s)
		initial := time.Now()
		request := readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest)
		for range 10 {
			time.Sleep(250 * time.Millisecond)
			writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureNak, request.Identifier, optStream(mruOption(1400)))
			request = readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest)
		}
		writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureAck, request.Identifier, request.Data)
		writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest, 0x31, nil)
		readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureAck)
		readLifecyclePacket(t, peer, ProtoCHAP, CHAPCodeChallenge)
		request = restartLifecycleLCP(t, peer)
		restarted := time.Now()
		if restarted.Sub(initial) != 2500*time.Millisecond {
			t.Fatalf("restart at %s, want 2.5s", restarted.Sub(initial))
		}
		for range 11 {
			time.Sleep(250 * time.Millisecond)
			writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureNak, request.Identifier, optStream(mruOption(1400)))
			request = readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest)
		}
		time.Sleep(250 * time.Millisecond)
		synctest.Wait()
		down := awaitLifecycleDown(t, events)
		if elapsed := time.Since(restarted); elapsed != 3*time.Second {
			t.Fatalf("restarted lifetime ended after %s, want 3s", elapsed)
		}
		if !strings.Contains(down.Reason, "LCP convergence timeout after 3s") {
			t.Fatalf("reason = %q", down.Reason)
		}
	})
}
