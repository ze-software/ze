// Design: docs/architecture/l2tp/followup-l2tp-call.md -- exact relay departure barrier.
package dataplane

import (
	"errors"
	"os"

	"github.com/ze-software/ze/internal/core/callsink"
)

// RetainOriginalFile runs no backend call. Its exact token bars physical
// departure until the worker joins bridge.Close and releases that token.
// The caller MUST Close the returned lease after both bridge channels close.
func (s *openedSession) RetainOriginalFile() (callsink.RelayFileLease, error) {
	s.transportMu.Lock()
	defer s.transportMu.Unlock()
	if s.transportClosed {
		return nil, errors.New("PPPoE relay transport is closed")
	}
	if s.transportClosing {
		return nil, errors.New("PPPoE relay transport is departing")
	}
	if s.relayFile != nil {
		return nil, errors.New("PPPoE original transport already has a retained relay")
	}
	if _, supported := s.transport.(OriginalRelayTransport); !supported {
		return nil, errors.New("PPPoE transport cannot retain its original file for native relay")
	}
	lease := &relayFileLease{session: s}
	lease.self = lease
	s.relayFile = lease
	return lease, nil
}

// A lease is created only by its exact admitted openedSession. The session's
// mutex protects its current token and physical transport state.
type relayFileLease struct {
	self     *relayFileLease
	session  *openedSession
	closed   bool
	closeErr error
}

func (lease *relayFileLease) WithOriginalFile(use func(*os.File) error) error {
	if lease == nil {
		return errors.New("PPPoE relay file lease is missing")
	}
	if lease.session == nil {
		return errors.New("PPPoE relay file lease is unminted")
	}
	if lease.self != lease {
		return errors.New("PPPoE relay file lease was copied")
	}
	if use == nil {
		return errors.New("PPPoE relay original file has no operation")
	}
	s := lease.session
	s.transportMu.Lock()
	defer s.transportMu.Unlock()
	if s.relayFile != lease {
		return errors.New("PPPoE relay file lease is released or copied")
	}
	if s.transportClosing {
		return errors.New("PPPoE relay transport is departing")
	}
	// RetainOriginalFile validated the immutable owner; a miss is a Ze defect.
	owner, supported := s.transport.(OriginalRelayTransport)
	if !supported {
		return errors.New("PPPoE relay lease outlived its original-file transport capability")
	}
	return owner.WithOriginalFile(func(original *os.File) error {
		if original == nil {
			return errors.New("PPPoE relay transport returned no original file")
		}
		return use(original)
	})
}

func (lease *relayFileLease) Close() error {
	if lease == nil {
		return nil
	}
	if lease.session == nil {
		return errors.New("PPPoE relay file lease is unminted")
	}
	if lease.self != lease {
		return errors.New("PPPoE relay file lease was copied")
	}
	s := lease.session
	s.transportMu.Lock()
	defer s.transportMu.Unlock()
	if lease.closed {
		return lease.closeErr
	}
	if s.relayFile != lease {
		return errors.New("PPPoE relay file lease lost its original owner")
	}
	s.relayFile = nil
	lease.closed = true
	if s.transportClosing {
		// The departure producer already requested this exact close. Complete it
		// here after native bridge cleanup, even if no caller retries departure.
		// Mark consumed before physical Close; its error is retained, never retried.
		if !s.transportClosed {
			s.transportClosed = true
			s.closeErr = closeSessionOwner(s.transport, "transport")
		}
		lease.closeErr = s.closeErr
	}
	return lease.closeErr
}
