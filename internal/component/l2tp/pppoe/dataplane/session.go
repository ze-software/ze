// Design: docs/architecture/l2tp/bng-5-pppoe.md -- admitted session ownership
// Related: registry.go -- provider selection and partial factory cleanup

package dataplane

import (
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"

	"github.com/ze-software/ze/internal/core/callsink"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// OpenedSession separates PPP's data-plane ownership from the access
// concentrator's transport ownership. A nil interface is invalid. The sealed
// interface can be created only by NewOpenedSession, never as a public zero value.
// Frames and Dataplane borrow the admitted values and are safe for concurrent
// access. Their own interface contracts govern the operations they expose.
type OpenedSession interface {
	Frames() ppp.FrameChannel
	Dataplane() ppp.SessionHandle
	// RelayDescriptor borrows the transport descriptor until CloseTransport.
	// The relay owner MUST serialize its descriptor use against transport close.
	// A transport without this capability returns an explicit error.
	RelayDescriptor() (int, error)
	// WithDescriptor lends the relay descriptor only during use. The caller
	// MUST perform the complete descriptor operation inside use and MUST NOT
	// retain, close or reenter it. CloseTransport waits until use returns.
	WithDescriptor(use func(int) error) error
	// WithOriginalFile lends the SAME original transport for bounded local
	// capture/ioctls. The caller MUST NOT retain, close, reenter, or wait on
	// backend/network operations. Unsupported transports explicitly refuse.
	WithOriginalFile(use func(*os.File) error) error
	// RetainOriginalFile keeps this exact transport open until its bridge's
	// original channels close. CloseTransport refuses pending retained roles.
	RetainOriginalFile() (callsink.RelayFileLease, error)
	// CloseTransport is safe for concurrent use. The transport owner MUST call
	// it when the access session ends; every call returns the first close error.
	// A provider panic becomes a retained error, never a successful later close.
	// It leaves a separate data-plane handle for the PPP driver to close.
	CloseTransport() error
	openedSession()
}

// RelayTransport is the optional capability of a transport owner that can
// hand its kernel transport to a relay. The descriptor is borrowed, never
// transferred: the caller MUST NOT close it or retain it past transport close.
// A failure MUST return an error, never an invented or stale descriptor.
type RelayTransport interface {
	RelayDescriptor() (int, error)
}

// OriginalRelayTransport is an optional native transport capability. The
// callback MUST receive its retained original file, never a new wrapper or
// duplicate. Its owner MUST exclude Close throughout use.
type OriginalRelayTransport interface {
	WithOriginalFile(func(*os.File) error) error
}

// NewOpenedSession consumes handle and transport on every outcome. On failure
// it closes every supplied, nonnil owner and includes any close errors in its
// refusal. On success the caller MUST arrange both ownership lifetimes to end.
// The handle MUST implement its idempotent Close contract, including when it
// also owns transport. Neither input may contain a typed nil. Frames' Control
// stream is required; Opened may be nil but MUST NOT contain a typed nil. The
// handle's dynamic value MUST be comparable for exact ownership comparisons.
func NewOpenedSession(frames ppp.FrameChannel, handle ppp.SessionHandle,
	transport io.Closer) (opened OpenedSession, err error) {
	// Until publication, even a provider accessor panic leaves both ownership
	// roles here. Release each independently before the admission guard sees it.
	defer func() {
		failure := recover()
		if opened != nil {
			return
		}
		cleanupErr := errors.Join(
			closeSessionOwner(handle, "data-plane"),
			closeSessionOwner(transport, "transport"),
		)
		if failure != nil {
			if cleanupErr == nil {
				panic(failure)
			}
			cause, ok := failure.(error)
			if !ok {
				cause = fmt.Errorf("PPPoE session construction panicked: %v", failure)
			}
			panic(errors.Join(cause, cleanupErr))
		}
		err = errors.Join(err, cleanupErr)
	}()
	borrowed, err := validateSessionFrames(frames)
	if err == nil {
		if nilResource(handle) {
			err = errors.New("PPPoE session has no data-plane handle")
		}
	}
	if err == nil {
		if !reflect.ValueOf(handle).Comparable() {
			err = errors.New("PPPoE session data-plane handle must be comparable for exact ownership checks")
		}
	}
	if err == nil {
		if nilResource(transport) {
			err = errors.New("PPPoE session has no transport owner")
		}
	}
	if err != nil {
		return nil, err
	}
	return &openedSession{frames: borrowed, handle: handle, transport: transport}, nil
}

// Each owner gets its own panic boundary so a failed release cannot prevent
// another role's release or become cached success. Error payloads keep identity.
func closeSessionOwner(owner io.Closer, role string) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			if cause, ok := failure.(error); ok {
				err = fmt.Errorf("PPPoE session %s cleanup panicked: %w", role, cause)
				return
			}
			err = fmt.Errorf("PPPoE session %s cleanup panicked: %v", role, failure)
		}
	}()
	if nilResource(owner) {
		return nil
	}
	return owner.Close()
}

type openedSession struct {
	frames           *sessionFrames
	handle           ppp.SessionHandle
	transport        io.Closer
	transportMu      sync.Mutex
	transportClosed  bool
	transportClosing bool
	relayFile        *relayFileLease
	closeErr         error
}

func (s *openedSession) Frames() ppp.FrameChannel     { return s.frames }
func (s *openedSession) Dataplane() ppp.SessionHandle { return s.handle }

func (s *openedSession) CloseTransport() error {
	s.transportMu.Lock()
	defer s.transportMu.Unlock()
	s.transportClosing = true
	if s.relayFile != nil {
		return errors.New("PPPoE transport cleanup waits for its exact original relay channels")
	}
	if s.transportClosed {
		return s.closeErr
	}
	s.transportClosed = true
	s.closeErr = closeSessionOwner(s.transport, "transport")
	return s.closeErr
}

func (s *openedSession) RelayDescriptor() (int, error) {
	s.transportMu.Lock()
	defer s.transportMu.Unlock()
	return s.relayDescriptorLocked()
}

// WithDescriptor keeps the transport owned through the actual bridge ioctl,
// rather than only through the earlier descriptor lookup on the async caller.
func (s *openedSession) WithDescriptor(use func(int) error) error {
	if use == nil {
		return errors.New("PPPoE relay descriptor has no operation")
	}
	s.transportMu.Lock()
	defer s.transportMu.Unlock()
	descriptor, err := s.relayDescriptorLocked()
	if err != nil {
		return err
	}
	return use(descriptor)
}

func (s *openedSession) WithOriginalFile(use func(*os.File) error) error {
	if use == nil {
		return errors.New("PPPoE relay original file has no operation")
	}
	s.transportMu.Lock()
	defer s.transportMu.Unlock()
	if s.transportClosed {
		return errors.New("PPPoE session transport is closed; its original file is unavailable")
	}
	if s.transportClosing {
		return errors.New("PPPoE session transport is departing; its original file is unavailable")
	}
	owner, supported := s.transport.(OriginalRelayTransport)
	if !supported {
		return errors.New("PPPoE session transport does not support original-file kernel relay")
	}
	return owner.WithOriginalFile(func(original *os.File) error {
		if original == nil {
			return errors.New("PPPoE relay transport returned no original file")
		}
		return use(original)
	})
}

// relayDescriptorLocked MUST be called with transportMu held. Both borrowing
// entry points use it so they apply the same owner and descriptor validation.
func (s *openedSession) relayDescriptorLocked() (int, error) {
	if s.transportClosing {
		return -1, errors.New("PPPoE relay transport is departing")
	}
	if s.transportClosed {
		return -1, errors.New("PPPoE session transport is closed; its relay descriptor is unavailable")
	}
	owner, supported := s.transport.(RelayTransport)
	if !supported {
		return -1, errors.New("PPPoE session transport does not support kernel relay")
	}
	descriptor, err := owner.RelayDescriptor()
	if err != nil {
		return -1, err
	}
	if descriptor < 0 {
		return -1, errors.New("PPPoE session transport returned an invalid relay descriptor")
	}
	return descriptor, nil
}

func (*openedSession) openedSession() {}

// sessionFrames retains the validated endpoints rather than asking a mutable
// provider for them again after construction. The provider retains interruption.
type sessionFrames struct {
	channel ppp.FrameChannel
	control io.ReadWriter
	opened  io.Reader
}

func validateSessionFrames(frames ppp.FrameChannel) (*sessionFrames, error) {
	if nilResource(frames) {
		return nil, errors.New("PPPoE session has no frame channel")
	}
	control := frames.Control()
	if nilResource(control) {
		return nil, errors.New("PPPoE session has no PPP control stream")
	}
	opened := frames.Opened()
	if opened != nil {
		if nilResource(opened) {
			return nil, errors.New("PPPoE session has a typed-nil opened control stream")
		}
	}
	return &sessionFrames{channel: frames, control: control, opened: opened}, nil
}

func (f *sessionFrames) Control() io.ReadWriter { return f.control }
func (f *sessionFrames) Opened() io.Reader      { return f.opened }
func (f *sessionFrames) InterruptRead() error   { return f.channel.InterruptRead() }

// nilResource is used only at the provider boundary. Interface nil comparison
// alone cannot reject a typed nil supplied by a backend.
func nilResource(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	// Only nil-capable dynamic types need reflection's IsNil operation.
	//exhaustive:ignore
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
