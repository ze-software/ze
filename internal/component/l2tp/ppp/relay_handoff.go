// Design: docs/architecture/l2tp/followup-l2tp-call.md -- realm relay handoff (LAC role)
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 4.4.5 (proxy LCP and authentication)
// Related: relay_reader_linux.go -- the channel reader a quiesce can wake
// Related: proxy_capture.go -- the LCP exchange the relay forwards

package ppp

import (
	"errors"
	"io"
	"time"
)

// errChannelNotQuiescable is returned by QuiesceSession for a session whose
// channel reader cannot be woken: it was not started with
// StartSession.CaptureProxyLCP, or its transport offers no way to interrupt
// a blocked read.
var errChannelNotQuiescable = errors.New("ppp: session channel cannot be quiesced; only a session started with CaptureProxyLCP can hand its subscriber to a realm relay")

// errReadInterrupted is what an interruptible channel file returns from Read
// once InterruptRead ran. It is sticky: every later Read returns it too.
var errReadInterrupted = errors.New("ppp: channel read interrupted for a relay handoff")

// readInterrupter is a channel file whose blocked Read can be woken.
// relay_reader_linux.go implements it for /dev/ppp.
type readInterrupter interface {
	InterruptRead() error
}

// readDeadliner is a channel file with read deadlines, as net.Conn has; a
// deadline in the past wakes a blocked Read.
type readDeadliner interface {
	SetReadDeadline(t time.Time) error
}

// interruptRead wakes a Read blocked on f. A file that offers neither form
// cannot be quiesced, and the caller refuses the handoff.
func interruptRead(f io.Reader) error {
	if v, ok := f.(readInterrupter); ok {
		return v.InterruptRead()
	}
	if v, ok := f.(readDeadliner); ok {
		return v.SetReadDeadline(time.Now())
	}
	return errChannelNotQuiescable
}

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
func (d *Driver) QuiesceSession(tunnelID, sessionID uint16) error {
	s, err := d.liveSession(tunnelID, sessionID)
	if err != nil {
		return err
	}
	if s.readerQuiesce == nil {
		return errChannelNotQuiescable
	}
	s.readerQuiesceOnce.Do(func() { close(s.readerQuiesce) })
	if err := interruptRead(s.chanFile); err != nil {
		return err
	}
	select {
	case <-s.readerGone:
	case <-s.done:
		return ErrSessionNotFound
	case <-d.stopCh:
		return ErrDriverStopped
	}
	if _, err := d.liveSession(tunnelID, sessionID); err != nil {
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
// MUST follow a successful QuiesceSession. Safe for concurrent use.
func (d *Driver) ReleaseSession(tunnelID, sessionID uint16) error {
	k := sessionKey{tunnelID, sessionID}
	d.mu.Lock()
	s, ok := d.sessions[k]
	if !ok {
		d.mu.Unlock()
		return ErrSessionNotFound
	}
	delete(d.sessions, k)
	d.mu.Unlock()

	// released is read by run's teardown, after sessStop wakes the
	// session, so it is stored first.
	s.released.Store(true)
	s.sessStopOnce.Do(func() { close(s.sessStop) })
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
func (d *Driver) RefuseAuth(tunnelID, sessionID uint16, message string) error {
	s, err := d.liveSession(tunnelID, sessionID)
	if err != nil {
		return err
	}
	select {
	case s.authRespCh <- authResponseMsg{message: message, terminate: true, displayText: true}:
		return nil
	default: // buffered(1) already full: a decision is already pending
		return ErrAuthResponsePending
	}
}

// liveSession returns the session for the key, or ErrSessionNotFound.
func (d *Driver) liveSession(tunnelID, sessionID uint16) (*pppSession, error) {
	d.mu.Lock()
	s, ok := d.sessions[sessionKey{tunnelID, sessionID}]
	d.mu.Unlock()
	if !ok {
		return nil, ErrSessionNotFound
	}
	return s, nil
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
