// Design: docs/research/l2tpv2-ze-integration.md -- auth/pool drain goroutines
// Related: handler.go -- AuthHandler, PoolHandler, AuthResult types
// Related: subsystem.go -- spawns drain goroutines per pppDriver

package l2tp

import (
	"fmt"
	"log/slog"

	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
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
func drainAuth(logger *slog.Logger, ch <-chan ppp.AuthEvent, handler AuthHandler, out chan<- authDrainResponse) {
	for ev := range ch {
		req, ok := ev.(ppp.EventAuthRequest)
		if !ok {
			continue
		}

		resp := callAuthHandler(logger, handler, req)
		out <- authDrainResponse{
			tunnelID:  req.TunnelID,
			sessionID: req.SessionID,
			result:    resp,
		}
	}
}

// startAuthDrain spawns the production auth drain goroutine that reads
// from d.AuthEventsOut(), calls the registered handler, and forwards
// the decision to d.AuthResponse(). Exits when the channel closes
// (Driver.Stop). The done channel is closed on exit.
func startAuthDrain(logger *slog.Logger, d *ppp.Driver, handler AuthHandler) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range d.AuthEventsOut() {
			req, ok := ev.(ppp.EventAuthRequest)
			if !ok {
				continue
			}
			r := callAuthHandler(logger, handler, req)
			if err := d.AuthResponse(req.TunnelID, req.SessionID, r.Accept, r.Message, r.AuthResponseBlob); err != nil {
				logger.Warn("l2tp: auth drain response failed",
					"tunnel", req.TunnelID, "session", req.SessionID, "error", err)
			}
		}
	}()
	return done
}

func callAuthHandler(logger *slog.Logger, handler AuthHandler, req ppp.EventAuthRequest) (result AuthResult) {
	if handler == nil {
		return AuthResult{Accept: true, Message: "no auth handler; accept all"}
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Error("auth handler panic", "tunnel", req.TunnelID, "session", req.SessionID, "panic", fmt.Sprint(r))
			result = AuthResult{Accept: false, Message: "internal error"}
		}
	}()

	return handler(req)
}

// drainPool reads EventIPRequests from the channel and calls the
// handler, sending results on out. Used by tests. Exits when ch closes.
func drainPool(logger *slog.Logger, ch <-chan ppp.IPEvent, handler PoolHandler, out chan<- poolDrainResponse) {
	for ev := range ch {
		req, ok := ev.(ppp.EventIPRequest)
		if !ok {
			continue
		}

		resp := callPoolHandler(logger, handler, req)
		out <- poolDrainResponse{
			tunnelID:  req.TunnelID,
			sessionID: req.SessionID,
			result:    resp,
		}
	}
}

// startPoolDrain spawns the production pool drain goroutine that reads
// from d.IPEventsOut(), calls the registered handler, and forwards the
// decision to d.IPResponse(). Exits when the channel closes
// (Driver.Stop). The done channel is closed on exit.
func startPoolDrain(logger *slog.Logger, d *ppp.Driver, handler PoolHandler) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range d.IPEventsOut() {
			req, ok := ev.(ppp.EventIPRequest)
			if !ok {
				continue
			}
			r := callPoolHandler(logger, handler, req)
			if err := d.IPResponse(req.TunnelID, req.SessionID, r); err != nil {
				logger.Warn("l2tp: pool drain response failed",
					"tunnel", req.TunnelID, "session", req.SessionID, "error", err)
			}
		}
	}()
	return done
}

func callPoolHandler(logger *slog.Logger, handler PoolHandler, req ppp.EventIPRequest) (result ppp.IPResponseArgs) {
	if handler == nil {
		return ppp.IPResponseArgs{Accept: false, Reason: "no pool handler"}
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Error("pool handler panic", "tunnel", req.TunnelID, "session", req.SessionID, "panic", fmt.Sprint(r))
			result = ppp.IPResponseArgs{Accept: false, Reason: "internal error"}
		}
	}()

	return handler(req)
}
