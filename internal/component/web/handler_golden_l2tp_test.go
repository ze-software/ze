//go:build ze_l2tp

package web

import (
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
)

// The two cases below capture the L2TP session detail route with a subsystem
// behind it. The case in handler_golden_test.go requests the same route with
// none, so it captures the 503 and reaches neither branch of the handler.
//
// VALIDATES: handleL2TPDetail (handler_l2tp.go) answers a page and a JSON body
// that describe one session, both projected from one l2tpDetailBody.
// PREVENTS: the page and the API body drifting apart with every test green.
func init() {
	webHandlerGoldenCases = append(webHandlerGoldenCases,
		webHandlerCase{
			Name: "get-l2tp-session-live", Method: http.MethodGet, Target: "/l2tp/42",
			Setup: publishGoldenL2TPSession,
		},
		webHandlerCase{
			Name: "get-l2tp-session-json", Method: http.MethodGet, Target: "/l2tp/42?format=json",
			Setup: publishGoldenL2TPSession,
		},
	)
}

// publishGoldenL2TPSession publishes one fixed session and two fixed events.
// Every value is written here, the timestamps included, because the capture
// serves the request twice and compares the two answers. The times are UTC, so
// the rendered timeline is the same on any machine.
func publishGoldenL2TPSession(t *testing.T) {
	t.Helper()

	up := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

	publishFakeL2TP(t, &fakeL2TPService{
		sessionOK: true,
		session: l2tp.SessionSnapshot{
			LocalSID:       42,
			RemoteSID:      43,
			TunnelLocalTID: 7,
			State:          "established",
			StateNum:       4,
			CreatedAt:      up,
			Username:       "operator@example.net",
			AssignedAddr:   netip.MustParseAddr("192.0.2.10"),
			Family:         "ipv4",
			TxConnectSpeed: 100000000,
			RxConnectSpeed: 100000000,
			PppInterface:   "ppp0",
			LNSMode:        true,
		},
		events: []l2tp.ObserverEvent{
			{Timestamp: up, Type: l2tp.ObserverEventSessionUp, TunnelID: 7, SessionID: 42, Actor: "lns"},
			{Timestamp: up.Add(30 * time.Second), Type: l2tp.ObserverEventEchoRTT, TunnelID: 7, SessionID: 42,
				RTT: 12 * time.Millisecond},
		},
	})
}
