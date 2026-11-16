// Design: docs/architecture/l2tp/subscriber-session-model.md -- PPPoE auth/pool drain goroutines
// Related: server.go -- InterfaceServer session lifecycle

package pppoe

import (
	"log/slog"
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/pkg/ze"
)

// authDrainDriver is the authentication decision boundary of the PPP driver.
type authDrainDriver interface {
	AuthEventsOut() <-chan ppp.AuthEvent
	AuthResponse(tunnelID, sessionID uint16, accept bool, message string, blob []byte) error
}

// startPPPoEAuthDrain answers the PPP driver's authentication requests. A
// request whose realm a relay-realm binding names goes to relay first and
// never reaches the auth handler (realm_relay.go); relay may be nil.
func startPPPoEAuthDrain(logger *slog.Logger, d authDrainDriver, resolve func() subscriber.AuthHandler, bus ze.EventBus, pending *sync.Map, relay *realmRelay) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range d.AuthEventsOut() {
			req, ok := ev.(ppp.EventAuthRequest)
			if !ok {
				continue
			}
			if err := subscriber.RequireID(logger, req.SubscriberID); err != nil {
				sendPPPoEAuthDrainResponse(logger, d, req, subscriber.AuthResult{Accept: false, Message: err.Error()})
				continue
			}
			if relay != nil && relay.decide(&req) {
				continue
			}
			// Plugins can replace the provider after this drain has started.
			handler := resolve()
			respond := func(accept bool, msg string, blob []byte) error {
				return d.AuthResponse(req.TunnelID, req.SessionID, accept, msg, blob)
			}
			r := callPPPoEAuthHandler(handler, req, subscriber.AuthRespondFunc(respond))
			subscriber.RecordAuthResult(subscriber.AccessPPPoE, r.Accept)
			if r.Accept && req.Username != "" {
				pending.Store(
					pendingAuthKey{ifindex: int(req.TunnelID), sessionID: req.SessionID},
					pendingAuthInfo{username: req.Username, authMethod: req.Method.String()},
				)
			}
			if bus != nil {
				if _, err := subevents.SessionAuthResult.Emit(bus, &subevents.SessionAuthResultPayload{
					SessionID:  req.SubscriberID,
					AccessType: subscriber.AccessPPPoE,
					Username:   req.Username,
					Accept:     r.Accept,
					Reason:     r.Message,
				}); err != nil {
					logger.Warn("pppoe: auth-result emit failed", "error", err)
				}
			}
			if r.Handled {
				continue
			}
			sendPPPoEAuthDrainResponse(logger, d, req, r)
		}
	}()
	return done
}

// sendPPPoEAuthDrainResponse serves ordinary decisions and missing-ID refusals.
// Callers MUST pass only a synchronous decision; an async provider owns its reply.
func sendPPPoEAuthDrainResponse(logger *slog.Logger, d authDrainDriver, req ppp.EventAuthRequest, result subscriber.AuthResult) {
	if cause := d.AuthResponse(req.TunnelID, req.SessionID, result.Accept, result.Message, result.AuthResponseBlob); cause != nil {
		const problem = "Ze could not deliver a subscriber authentication decision"
		const why = "The PPP session may have ended before its decision arrived"
		const fix = "ze show errors"
		err := opererr.Wrap(cause, problem, why, fix)
		logger.Warn(err.Error(), "code", report.CodeInternalDefect,
			"subscriber", req.SubscriberID, "ifindex", req.TunnelID, "session", req.SessionID)
		report.RaiseWarningFix("subscriber", report.CodeInternalDefect, "authentication answer", problem+": "+why, fix)
	}
}

func callPPPoEAuthHandler(handler subscriber.AuthHandler, req ppp.EventAuthRequest, respond subscriber.AuthRespondFunc) (result subscriber.AuthResult) {
	// Owner ruling 2026-10-18: "PPPOE expectation of those session should be
	// explicitly configured." An empty auth slot refuses every request, the
	// same as the L2TP drain, so no credential is admitted unchecked. A
	// configured no-auth session is admitted by the registered provider
	// (l2tp-auth-local, l2tp-auth-radius) after PPP's allow-no-auth guard.
	if handler == nil {
		return subscriber.AuthResult{Accept: false, Message: "no auth handler configured"}
	}
	// The handler task's boundary (panic_guard.go): a panicking handler refuses
	// this session's authentication, and the drain answers the next request.
	defer pppoePanics().auth.Contain(describeHandlerCall(req.TunnelID, req.SessionID), func(*taskguard.Panic) {
		result = subscriber.AuthResult{Accept: false, Message: "internal error"}
	})
	return handler(req, respond)
}

// poolDrainDriver is the address-decision boundary of a PPP driver, allowing
// a test to order session teardown before the pool's answer is delivered.
type poolDrainDriver interface {
	IPEventsOut() <-chan ppp.IPEvent
	IPResponse(tunnelID, sessionID uint16, answer ppp.IPResponseArgs) error
}

// startPPPoEPoolDrain serves requests until Driver.Stop closes them. The caller
// MUST stop the driver and wait for done before releasing the subscriber bus.
func startPPPoEPoolDrain(logger *slog.Logger, d poolDrainDriver, handler subscriber.PoolHandler, bus ze.EventBus) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range d.IPEventsOut() {
			req, ok := ev.(ppp.EventIPRequest)
			if !ok {
				continue
			}
			var r ppp.IPResponseArgs
			if err := subscriber.RequireID(logger, req.SubscriberID); err != nil {
				r = ppp.IPResponseArgs{Accept: false, Family: req.Family, Reason: err.Error()}
			} else {
				r = callPPPoEPoolHandler(handler, req)
			}
			if err := d.IPResponse(req.TunnelID, req.SessionID, r); err != nil {
				subevents.ReleaseUndeliveredPoolAnswer(logger, bus, req, r)
				logger.Warn("pppoe: pool drain response failed",
					"ifindex", req.TunnelID, "session", req.SessionID, "error", err)
			}
		}
	}()
	return done
}

func callPPPoEPoolHandler(handler subscriber.PoolHandler, req ppp.EventIPRequest) (result ppp.IPResponseArgs) {
	if handler == nil {
		return ppp.IPResponseArgs{Accept: false, Reason: "no pool handler"}
	}
	// The handler task's boundary (panic_guard.go): a panicking handler refuses
	// this session's address request, and the drain answers the next request.
	defer pppoePanics().pool.Contain(describeHandlerCall(req.TunnelID, req.SessionID), func(*taskguard.Panic) {
		result = ppp.IPResponseArgs{Accept: false, Reason: "internal error"}
	})
	return handler(req)
}
