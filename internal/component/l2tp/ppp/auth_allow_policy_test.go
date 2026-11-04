package ppp

import (
	"testing"
	"time"
)

// TestAuthAllowEmptyRefusesPAPNak proves that an empty permitted fallback list
// cannot downgrade required CHAP, by observing the real driver's LCP response.
func TestAuthAllowEmptyRefusesPAPNak(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 16001)
	defer closeConn(pair.peerEnd)
	ops, _, _ := newFakeOps()
	d := newDriver(driverConfig{Logger: discardLogger(), Backend: &fakeBackend{}, Ops: ops})
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()
	d.SessionsIn() <- StartSession{
		TunnelID: 16, SessionID: 1, ChanFD: 16001, UnitFD: 16002,
		AuthMethod: AuthMethodCHAPMD5, AuthRequired: true,
		DisableIPCP: true, DisableIPv6CP: true,
	}
	proto, payload := readPeerFrame(t, pair.peerEnd)
	if proto != ProtoLCP {
		t.Fatalf("first protocol = %04x, want LCP", proto)
	}
	request, err := ParseLCPPacket(payload)
	if err != nil {
		t.Fatal(err)
	}
	if request.Code != LCPConfigureRequest {
		t.Fatalf("first LCP code = %d, want Configure-Request", request.Code)
	}
	options := buildOptionStream([]LCPOption{authProtoOpt(authProtoPAP)})
	var frame [MaxFrameLen]byte
	n := WriteFrame(frame[:], 0, ProtoLCP, nil)
	n += WriteLCPPacket(frame[:], n, LCPConfigureNak, request.Identifier, options)
	if err := pair.peerEnd.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := pair.peerEnd.Write(frame[:n]); err != nil {
		t.Fatal(err)
	}
	proto, payload = readPeerFrame(t, pair.peerEnd)
	if proto != ProtoLCP {
		t.Fatalf("response protocol = %04x, want LCP", proto)
	}
	response, err := ParseLCPPacket(payload)
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != LCPTerminateRequest {
		t.Fatalf("PAP Nak response code = %d, want Terminate-Request; empty policy permitted downgrade", response.Code)
	}
	select {
	case event := <-d.EventsOut():
		if _, opened := event.(EventLCPUp); opened {
			t.Fatalf("refused PAP published LCPUp: %+v", event)
		}
	default:
	}
	select {
	case event := <-d.AuthEventsOut():
		if _, requested := event.(EventAuthRequest); requested {
			t.Fatalf("refused PAP requested authentication: %+v", event)
		}
	default:
	}
}
