// Design: docs/research/l2tpv2-ze-integration.md -- auth/pool drain goroutines
// Related: handler.go -- AuthHandler, PoolHandler, AuthResult types
// Related: subsystem.go -- spawns drain goroutines per pppDriver

package l2tp

import (
	"log/slog"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/pkg/ze"
)

// authDrainResponse carries the handler result plus session IDs so the
// test harness can inspect decisions without a real Driver.
type authDrainResponse struct {
	tunnelID  uint16
	sessionID uint16
	result    AuthResult
}

// poolDrainResponse carries the handler result plus session IDs so the
// test harness can inspect decisions without a real Driver.
type poolDrainResponse struct {
	tunnelID  uint16
	sessionID uint16
	result    ppp.IPResponseArgs
}

// drainAuth reads EventAuthRequests from the channel and calls the
// handler, sending results on out. Used by tests. Exits when ch closes.
func drainAuth(ch <-chan ppp.AuthEvent, handler AuthHandler, out chan<- authDrainResponse) {
	for ev := range ch {
		req, ok := ev.(ppp.EventAuthRequest)
		if !ok {
			continue
		}

		respond := func(accept bool, msg string, blob []byte) error {
			out <- authDrainResponse{
				tunnelID:  req.TunnelID,
				sessionID: req.SessionID,
				result:    AuthResult{Accept: accept, Message: msg, AuthResponseBlob: blob, Handled: true},
			}
			return nil
		}

		resp := callAuthHandler(handler, req, respond)
		if resp.Handled {
			continue
		}
		out <- authDrainResponse{
			tunnelID:  req.TunnelID,
			sessionID: req.SessionID,
			result:    resp,
		}
	}
}

// authDrainDriver is the authentication decision boundary of the PPP driver.
type authDrainDriver interface {
	AuthEventsOut() <-chan ppp.AuthEvent
	AuthResponse(request ppp.EventAuthRequest, accept bool, message string, blob []byte) error
}

// startAuthDrain spawns the production auth drain goroutine that reads
// from d.AuthEventsOut(), calls the registered handler, and forwards
// the decision to d.AuthResponse(). Exits when the channel closes
// (Driver.Stop). The done channel is closed on exit.
//
// resolve is called PER REQUEST, not once here. The auth slot is
// late-binding on purpose: l2tp-auth-local claims it in init() and
// l2tp-auth-radius re-claims it at configure time, which is what makes the
// precedence "RADIUS when configured, local otherwise"
// (internal/component/l2tp/plugins/authradius/register.go). This drain used to
// capture the handler once at subsystem start and hold it forever, so that
// precedence could never take effect: the listener binds before the plugins
// configure, so the captured handler was always local's. In
// test/l2tp/radius-acct-wire.ci the daemon bound at .365, RADIUS configured at
// .366, and local answered the request at .390 -- one millisecond too late to
// matter. A registry written to be re-claimed needs a consumer that re-reads
// it.
func startAuthDrain(logger *slog.Logger, d authDrainDriver, resolve func() AuthHandler, bus ze.EventBus) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range d.AuthEventsOut() {
			req, ok := ev.(ppp.EventAuthRequest)
			if !ok {
				continue
			}
			if err := subscriber.RequireID(logger, req.SubscriberID); err != nil {
				sendAuthDrainResponse(logger, d, req, AuthResult{Accept: false, Message: err.Error()})
				continue
			}
			handler := resolve()
			respond := func(accept bool, msg string, blob []byte) error {
				return d.AuthResponse(req, accept, msg, blob)
			}
			r := callAuthHandler(handler, req, respond)
			subscriber.RecordAuthResult(subscriber.AccessL2TP, r.Accept)
			if bus != nil {
				if _, err := subevents.SessionAuthResult.Emit(bus, &subevents.SessionAuthResultPayload{
					SessionID:  req.SubscriberID,
					AccessType: subscriber.AccessL2TP,
					Username:   req.Username,
					Accept:     r.Accept,
					Reason:     r.Message,
				}); err != nil {
					logger.Warn("l2tp: auth-result emit failed", "error", err)
				}
			}
			if r.Handled {
				continue
			}
			sendAuthDrainResponse(logger, d, req, r)
		}
	}()
	return done
}

// sendAuthDrainResponse serves both ordinary decisions and missing-ID refusals.
// Callers MUST pass only a synchronous decision; an async provider owns its reply.
func sendAuthDrainResponse(logger *slog.Logger, d authDrainDriver, req ppp.EventAuthRequest, result AuthResult) {
	if cause := d.AuthResponse(req, result.Accept, result.Message, result.AuthResponseBlob); cause != nil {
		const problem = "Ze could not deliver a subscriber authentication decision"
		const why = "The PPP session may have ended before its decision arrived"
		const fix = "ze show errors"
		err := opererr.Wrap(cause, problem, why, fix)
		logger.Warn(err.Error(), "code", report.CodeInternalDefect,
			"subscriber", req.SubscriberID, "tunnel", req.TunnelID, "session", req.SessionID)
		report.RaiseWarningFix("subscriber", report.CodeInternalDefect, "authentication answer", problem+": "+why, fix)
	}
}

func callAuthHandler(handler AuthHandler, req ppp.EventAuthRequest, respond AuthRespondFunc) (result AuthResult) {
	if handler == nil {
		return AuthResult{Accept: false, Message: "no auth handler configured"}
	}

	// The handler task's boundary (panic_guard.go): a panicking handler refuses
	// this session's authentication, and the drain answers the next request.
	defer l2tpPanics().auth.Contain(describeHandlerCall(req.TunnelID, req.SessionID), func(*taskguard.Panic) {
		result = AuthResult{Accept: false, Message: "internal error"}
	})

	return handler(req, respond)
}

// drainPool reads EventIPRequests from the channel and calls the
// handler, sending results on out. Used by tests. Exits when ch closes.
func drainPool(ch <-chan ppp.IPEvent, handler PoolHandler, out chan<- poolDrainResponse) {
	for ev := range ch {
		req, ok := ev.(ppp.EventIPRequest)
		if !ok {
			continue
		}

		resp := callPoolHandler(handler, req)
		out <- poolDrainResponse{
			tunnelID:  req.TunnelID,
			sessionID: req.SessionID,
			result:    resp,
		}
	}
}

// poolDrainDriver is the address-decision boundary of a PPP driver. Keeping
// this seam narrow lets a test order session teardown before answer delivery.
type poolDrainDriver interface {
	IPEventsOut() <-chan ppp.IPEvent
	IPResponse(request ppp.EventIPRequest, answer ppp.IPResponseArgs) error
}

// startPoolDrain reads the driver's requests until Driver.Stop closes them.
// The caller MUST stop the driver and wait for done before releasing its bus.
// An undelivered answer returns its allocation through that bus.
func startPoolDrain(logger *slog.Logger, d poolDrainDriver, resolve func() PoolHandler, bus ze.EventBus) <-chan struct{} {
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
				// Resolve per valid request: l2tp-pool claims its slot at
				// configure time, after the listener has bound.
				handler := resolve()
				r = callPoolHandler(handler, req)
			}
			if err := d.IPResponse(req, r); err != nil {
				subevents.ReleaseUndeliveredPoolAnswer(logger, bus, req, r)
				logger.Warn("l2tp: pool drain response failed",
					"tunnel", req.TunnelID, "session", req.SessionID, "error", err)
			}
		}
	}()
	return done
}

func callPoolHandler(handler PoolHandler, req ppp.EventIPRequest) (result ppp.IPResponseArgs) {
	if handler == nil {
		return ppp.IPResponseArgs{Accept: false, Reason: "no pool handler"}
	}

	// The handler task's boundary (panic_guard.go): a panicking handler refuses
	// this session's address request, and the drain answers the next request.
	defer l2tpPanics().pool.Contain(describeHandlerCall(req.TunnelID, req.SessionID), func(*taskguard.Panic) {
		result = ppp.IPResponseArgs{Accept: false, Reason: "internal error"}
	})

	return handler(req)
}
