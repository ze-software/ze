package l2tpauthradius

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	l2tpshaper "github.com/ze-software/ze/internal/component/l2tp/plugins/shaper"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// coaApplyBus delivers the real registered shaper's subscriptions. Assertions
// read the backend's installed state, rather than counting emitted events.
type coaApplyBus struct {
	mu       sync.Mutex
	handlers map[[2]string][]func(any)
}

func (b *coaApplyBus) Emit(namespace, event string, payload any) (int, error) {
	b.mu.Lock()
	handlers := append([]func(any){}, b.handlers[[2]string{namespace, event}]...)
	b.mu.Unlock()
	for _, handler := range handlers {
		handler(payload)
	}
	return 0, nil
}

func (b *coaApplyBus) Subscribe(namespace, event string, handler func(any)) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := [2]string{namespace, event}
	b.handlers[key] = append(b.handlers[key], handler)
	return func() {}
}

type coaApplyBackend struct {
	mu      sync.Mutex
	desired map[string]traffic.InterfaceQoS
	fail    bool
}

func (b *coaApplyBackend) Apply(_ context.Context, desired map[string]traffic.InterfaceQoS) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail {
		for _, qos := range desired {
			if qos.Qdisc.RateBps != 10_000_000 {
				// Model a backend failing after it changed one direction. The
				// original ten-megabit pair remains available for compensation.
				maps.Copy(b.desired, desired)
				return errors.New("test backend failed after changing the download rate")
			}
		}
	}
	maps.Copy(b.desired, desired)
	return nil
}

func (b *coaApplyBackend) ListQdiscs(iface string) (traffic.InterfaceQoS, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.desired[iface], nil
}

func (b *coaApplyBackend) Close() error { return nil }

var coaApplyBackendRegistration sync.Once
var coaApplyBackendCurrent *coaApplyBackend

func startCoARateOwner(t *testing.T) (*coaApplyBus, *coaApplyBackend, *rpc.DirectBridge) {
	t.Helper()
	backend := &coaApplyBackend{desired: make(map[string]traffic.InterfaceQoS)}
	coaApplyBackendCurrent = backend
	coaApplyBackendRegistration.Do(func() {
		if err := traffic.RegisterBackend("test-coa-apply", func() (traffic.Backend, error) {
			return coaApplyBackendCurrent, nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	if err := traffic.LoadBackend("test-coa-apply"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = traffic.CloseBackend() })
	bus := &coaApplyBus{handlers: make(map[[2]string][]func(any))}
	reg := registry.Lookup(l2tpshaper.Name)
	if reg == nil {
		t.Fatal("shaper plugin is not registered")
	}
	reg.ConfigureEventBus(bus)
	_, bridge := startAuthProvider(t, l2tpshaper.Name, `{"l2tp":{"shaper":{"qdisc-type":"tbf","default-rate":"10mbit","upload-rate":"5mbit"}}}`)
	return bus, backend, bridge
}

func coaAppliedFixture(t *testing.T) (string, []byte, *coaApplyBackend) {
	t.Helper()
	bus, backend, bridge := startCoARateOwner(t)
	if _, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
		TunnelID: 10, SessionID: 20, Interface: "ppp-coa-apply",
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := l2tpevents.SessionDown.Emit(bus, &l2tpevents.SessionDownPayload{TunnelID: 10, SessionID: 20}); err != nil {
			t.Error(err)
		}
	})
	qos, err := backend.ListQdiscs("ppp-coa-apply")
	if err != nil {
		t.Fatal(err)
	}
	if qos.Qdisc.RateBps != 10_000_000 {
		t.Fatalf("fixture did not install the initial rate: %+v", qos)
	}
	session := &subscriber.Session{
		ID: "coa-apply-regression", AccessType: subscriber.AccessL2TP,
		Username: "alice", TunnelID: 10, SessionID: 20,
		PppInterface: "ppp-coa-apply", AcctSessionID: "10-20-1",
		DownloadRate: 10_000_000, UploadRate: 5_000_000,
	}
	subscriber.DefaultRegistry.Add(session)
	t.Cleanup(func() { subscriber.DefaultRegistry.Remove(session.ID) })
	secret := []byte("test-coa-actual-apply")
	l2tp.PublishService(oneSessionService())
	t.Cleanup(func() { l2tp.PublishService(nil) })
	cl, err := newCoAListener(coaListenerConfig{
		AllowedSources: coaLoopbackSources(), DefaultSecret: secret, Bus: bus,
		Dispatch: func(ctx context.Context, command string, args []string, peer string) (string, json.RawMessage, error) {
			out, err := bridge.ExecuteCommand(ctx, "coa-regression", command, args, peer)
			if err != nil {
				return "error", nil, err
			}
			return out.Status, out.Data, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return cl.conn.LocalAddr().String(), secret, backend
}

func wantCoAInitialRate(t *testing.T, backend *coaApplyBackend) {
	t.Helper()
	qos, err := backend.ListQdiscs("ppp-coa-apply")
	if err != nil {
		t.Fatal(err)
	}
	if qos.Qdisc.RateBps != 10_000_000 {
		t.Errorf("refused request changed installed rate: got %d, want 10000000", qos.Qdisc.RateBps)
	}
}

func sendAppliedCoA(t *testing.T, addr string, code uint8, secret []byte, attrs []radius.Attr) *radius.Packet {
	t.Helper()
	wire := buildCoAPacket(t, code, secret, attrs, time.Now())
	response := newCoAClientSocket(t, addr).exchangeRaw(t, wire)
	var requestAuth [radius.AuthenticatorLen]byte
	copy(requestAuth[:], wire[4:4+radius.AuthenticatorLen])
	if !radius.VerifyResponseAuth(response, requestAuth, secret) {
		t.Fatal("response failed its RADIUS Response Authenticator")
	}
	pkt, err := radius.Decode(response)
	if err != nil {
		t.Fatal(err)
	}
	return pkt
}

// RFC requirement: RFC5176-3.3-1 negative -- an exact accounting-id registry hit
// cannot override a conflicting User-Name or NAS-Port: the signed CoA gets 503
// and leaves the actual download shaping rate unchanged.
func TestCoARegisteredSubscriberAllSelectorsMustMatch(t *testing.T) {
	for _, tc := range []struct {
		name     string
		selector radius.Attr
	}{
		{"username", radius.Attr{Type: radius.AttrUserName, Value: radius.AttrString("bob")}},
		{"nas-port", radius.Attr{Type: radius.AttrNASPort, Value: radius.AttrUint32(21)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr, secret, backend := coaAppliedFixture(t)
			resp := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
				{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
				{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit")}, tc.selector,
			})
			wantNAK(t, resp, radius.CodeCoANAK, radius.ErrorCauseSessionNotFound)
			wantCoAInitialRate(t, backend)
		})
	}
}

func TestCoAUnrepresentableRateDoesNotACK(t *testing.T) {
	addr, secret, backend := coaAppliedFixture(t)
	resp := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("1bit")},
	})
	wantNAK(t, resp, radius.CodeCoANAK, radius.ErrorCauseInvalidAttributeValue)
	wantCoAInitialRate(t, backend)
}

// RFC naming: RFC5176 Section 3.5 -- a registered PPPoE accounting id combined
// with a conflicting User-Name or NAS-Port gets signed Disconnect-NAK 503 and
// retains its registry entry. This regression discriminates Session Context Not
// Found (503) from Session Context Not Removable (504).
func TestDisconnectRegisteredSubscriberAllSelectorsMustMatch(t *testing.T) {
	addr, secret, backend := coaAppliedFixture(t)
	session, found := subscriber.DefaultRegistry.Get("coa-apply-regression")
	if !found {
		t.Fatal("fixture subscriber is absent")
	}
	session.AccessType = subscriber.AccessPPPoE
	session.PPPoESID = 20
	subscriber.DefaultRegistry.Add(&session)
	for _, selector := range []radius.Attr{
		{Type: radius.AttrUserName, Value: radius.AttrString("bob")},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(21)},
	} {
		resp := sendAppliedCoA(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
			{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}, selector,
		})
		wantNAK(t, resp, radius.CodeDisconnectNAK, radius.ErrorCauseSessionNotFound)
	}
	wantCoAInitialRate(t, backend)
	if _, found := subscriber.DefaultRegistry.Get(session.ID); !found {
		t.Error("mismatching Disconnect removed the subscriber")
	}
}

func TestCoABackendRefusalDoesNotACK(t *testing.T) {
	addr, secret, backend := coaAppliedFixture(t)
	backend.mu.Lock()
	backend.fail = true
	backend.mu.Unlock()
	resp := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit")},
	})
	wantNAK(t, resp, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
	wantCoAInitialRate(t, backend)
}

func TestCoAMissingBackendDoesNotACK(t *testing.T) {
	addr, secret, backend := coaAppliedFixture(t)
	if err := traffic.CloseBackend(); err != nil {
		t.Fatal(err)
	}
	resp := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit")},
	})
	wantNAK(t, resp, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
	wantCoAInitialRate(t, backend)
}

func TestCoAActualBackendApplyPrecedesACK(t *testing.T) {
	addr, secret, backend := coaAppliedFixture(t)
	resp := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
		{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(20)},
		{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit")},
	})
	if resp.Code != radius.CodeCoAACK {
		t.Fatalf("valid change response: got %d, want CoA-ACK", resp.Code)
	}
	qos, err := backend.ListQdiscs("ppp-coa-apply")
	if err != nil {
		t.Fatal(err)
	}
	if qos.Qdisc.RateBps != 20_000_000 {
		t.Errorf("ACK preceded installed change: got %d, want 20000000", qos.Qdisc.RateBps)
	}
}
