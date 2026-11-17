// Design: docs/architecture/l2tp/followup-l2tp-call.md -- realm relay handoff (LAC role)
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 4.4.5 (proxy LCP and authentication)
// Related: frame_channel.go -- borrowed reader interruption for a relay
// Related: proxy_capture.go -- the LCP exchange the relay forwards

package ppp

import (
	"errors"
)

// errChannelNotQuiescable is returned by QuiesceSession for a session whose
// channel reader cannot be woken: it was not started with
// StartSession.CaptureProxyLCP, or its transport offers no way to interrupt
// a blocked read.
var errChannelNotQuiescable = errors.New("ppp: session channel cannot be quiesced; only a session started with CaptureProxyLCP can hand its subscriber to a realm relay")

// QuiesceSession stops the session's reader on its channel descriptor and
// returns only once that reader has returned, so a kernel channel bridge set
// after it is the only consumer of the subscriber's frames. The session keeps
// its LCP state and may still write: RefuseAuth answers through it, and
// ReleaseSession hands it over. A frame the subscriber sends from here on is
// not read; PPP recovers it by retransmission.
//
// Only a session started with StartSession.CaptureProxyLCP can be quiesced.
// MUST be called while the session waits for its authentication decision,
// that is after its EventAuthRequest and before any AuthResponse. Safe for
// concurrent use; a second call returns at once.
func (d *Driver) QuiesceSession(request EventAuthRequest) error {
	s, err := d.lockAuthRequest(request)
	if err != nil {
		return err
	}
	defer request.pending.mu.Unlock()
	if s.readerQuiesce == nil {
		return errChannelNotQuiescable
	}
	s.readerQuiesceOnce.Do(func() { close(s.readerQuiesce) })
	if err := s.frames.InterruptRead(); err != nil {
		return err
	}
	select {
	case <-s.readerGone:
	case <-s.done:
		return ErrSessionNotFound
	case <-d.stopCh:
		return ErrDriverStopped
	}
	if _, err := d.liveSession(request.Control); err != nil {
		return err
	}
	return nil
}

// ReleaseSession forgets a session another layer now carries, such as a
// realm relay whose kernel bridge took the subscriber's channel. It sends no
// authentication reply, no LCP Terminate-Request and no EventSessionDown,
// closes the session's channel and unit descriptors (the unit was never
// connected: NCP never ran), and returns once the session goroutine exited.
// The transport keeps its own PPPoX socket.
//
// MUST follow a successful QuiesceSession of this same control. Safe for concurrent use.
func (d *Driver) ReleaseSession(request EventAuthRequest) error {
	s, err := d.lockAuthRequest(request)
	if err != nil {
		return err
	}
	d.mu.Lock()
	current, ok := d.sessionForControlLocked(request.Control)
	if !ok {
		d.mu.Unlock()
		request.pending.mu.Unlock()
		return ErrSessionNotFound
	}
	if current != s {
		d.mu.Unlock()
		request.pending.mu.Unlock()
		return ErrSessionNotFound
	}
	delete(d.sessions, s.subscriberID)
	d.mu.Unlock()
	request.pending.waiting = false
	// Publish the winning release while retirement is still excluded.
	s.released.Store(true)
	request.pending.mu.Unlock()

	request.Control.Cancel()
	s.closeFiles()
	<-s.done
	return nil
}

// RefuseAuth rejects the pending authentication like AuthResponse with
// accept=false, and then closes the link: the session writes CHAP Failure
// (or PAP Authenticate-Nak) carrying message, then one LCP Terminate-Request,
// and ends with EventSessionDown. message is display text: on an MS-CHAPv2
// session it becomes the M= text of a structured RFC 2759 Section 6 Failure
// (mschapv2RefusalMessage), so the caller never formats one. A realm relay that could not carry the
// subscriber refuses it this way, whether or not the session was quiesced.
//
// Safe for concurrent use. Non-blocking, with AuthResponse's
// ErrAuthResponsePending on a duplicate.
func (d *Driver) RefuseAuth(request EventAuthRequest, message string) error {
	_, err := d.liveSession(request.Control)
	if err != nil {
		return err
	}
	return request.answer(authResponseMsg{message: message, terminate: true, displayText: true})
}

// liveSession accepts only this driver's exact admitted worker capability.
func (d *Driver) liveSession(control SessionControl) (*pppSession, error) {
	d.mu.Lock()
	s, ok := d.sessionForControlLocked(control)
	d.mu.Unlock()
	if !ok {
		return nil, ErrSessionNotFound
	}
	select {
	case <-s.sessStop:
		return nil, ErrSessionNotFound
	case <-d.stopCh:
		return nil, ErrDriverStopped
	default:
		return s, nil
	}
}

// readerQuiesced reports whether QuiesceSession asked the channel reader to
// stop. A nil channel (a session that cannot be quiesced) never is.
func (s *pppSession) readerQuiesced() bool {
	// refuse-by-default: guards readFrames -- a session that cannot be quiesced reports every read error
	if s.readerQuiesce == nil {
		return false
	}
	select {
	case <-s.readerQuiesce:
		return true
	default:
		return false
	}
}

// closeLinkAfterRefusal writes the LCP Terminate-Request a RefuseAuth owes
// after the authentication reply. With the reader possibly quiesced nobody
// waits for the Terminate-Ack, so the session ends at once and the transport
// sends PADT.
//
// RFC 1661 Section 3.5: "If authentication fails, the authenticator SHOULD
// proceed instead to the Link Termination phase."
func (s *pppSession) closeLinkAfterRefusal(decision authResponseMsg) {
	if !decision.terminate {
		return
	}
	if !s.sendTerminateRequest() {
		s.logger.Warn("ppp: relay refusal could not write its LCP Terminate-Request; the transport still ends the session")
	}
}
