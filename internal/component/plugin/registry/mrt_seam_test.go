package registry

import "testing"

// VALIDATES: the MRT message/peer bridges reach the BGP reactor factory through
// the registry seam (Set/Get MRTMessageCallback + MRTPeerCallback), so the
// always-on hub no longer imports internal/plugins/mrt and //go:build ze_mrt can
// drop MRT from the binary.
// PREVENTS: a regression that re-introduces a direct cmd/ze/hub -> plugins/mrt
// import (which pins MRT into every binary), or a Set that clears a live bridge.

type fakeMRTMsgCB struct{ id int }

func (*fakeMRTMsgCB) OnBGPMessage(_ any, _ uint8, _ bool, _ []byte) {}

type fakeMRTPeerCB struct{ id int }

func (*fakeMRTPeerCB) OnPeerEstablished(_ any)      {}
func (*fakeMRTPeerCB) OnPeerClosed(_ any, _ string) {}

func TestMRTCallbackSeam(t *testing.T) {
	// A registered bridge round-trips through the seam.
	msg := &fakeMRTMsgCB{id: 1}
	SetMRTMessageCallback(msg)
	if got := GetMRTMessageCallback(); got != msg {
		t.Fatalf("GetMRTMessageCallback = %v, want the registered bridge", got)
	}

	peer := &fakeMRTPeerCB{id: 1}
	SetMRTPeerCallback(peer)
	if got := GetMRTPeerCallback(); got != peer {
		t.Fatalf("GetMRTPeerCallback = %v, want the registered bridge", got)
	}

	// nil is ignored: it must not clear a live bridge (mirrors SetRIBDumpCallback).
	SetMRTMessageCallback(nil)
	if got := GetMRTMessageCallback(); got != msg {
		t.Fatal("SetMRTMessageCallback(nil) cleared a live message bridge")
	}
	SetMRTPeerCallback(nil)
	if got := GetMRTPeerCallback(); got != peer {
		t.Fatal("SetMRTPeerCallback(nil) cleared a live peer bridge")
	}
}
