// Design: docs/research/l2tpv2-ze-integration.md -- native unit release after remote cleanup

package kernel

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// BindNativeReleaseSeal retains one unborrowed reference to the exact PPP unit.
// The factory calls it before transferring the handle or issuing a remote
// operation. Duplicating the unit retains its open file description, so closing
// the borrowed streams does not unregister the interface before the seal agrees.
func (s *session) BindNativeReleaseSeal(seal ppp.NativeReleaseSeal) error {
	if nilSessionDependency(seal) {
		return errors.New("ppp: native release requires an admitted policy seal")
	}
	tag, err := nativeBirthTag(seal)
	if err != nil {
		return err
	}
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if s.releaseSeal != nil {
		return errors.New("ppp: a native release seal is already bound to this session")
	}
	if s.ops.duplicateFD == nil {
		return errors.New("ppp: the kernel owner cannot retain its native unit")
	}
	if s.ops.stampBirth == nil {
		return errors.New("ppp: the kernel owner cannot confirm its native birth tag")
	}
	if err := s.ops.stampBirth(s.identity, tag); err != nil {
		return fmt.Errorf("ppp: confirm the native unit birth: %w", err)
	}
	var fd int
	var retained *nativePPPUnitHold
	if lease, original := s.unitLease.(*claimedCreatedUnit); original {
		retained, err = lease.unit.retainNativeUnit(s.channelFD, s.ops.duplicateFD, s.ops.closeFD)
		if err == nil {
			fd = retained.descriptor
		}
	} else {
		// Only private logical session fixtures can supply a non-native lease.
		// Public constructors consume CreatedUnit and always take the branch above.
		fd, err = s.ops.duplicateFD(s.unitFD)
	}
	if err != nil {
		return fmt.Errorf("ppp: retain the native unit: %w", err)
	}
	if fd < 0 || fd == s.unitFD || fd == s.channelFD {
		// An alias is not a new owned descriptor and MUST NOT be closed here.
		return errors.New("ppp: native unit retention returned an invalid duplicate")
	}
	s.releaseSeal = seal
	s.birthTag = tag
	s.retainedUnit = fd
	s.retainedCreatedUnit = retained
	s.unitRetained = true
	return nil
}

// Linux include/uapi/linux/if.h sets IFALIASZ to 256; netif_set_alias
// rejects lengths at or above that limit. Netlink strings also reserve NUL.
const maxNativeBirthTagLength = 255

func nativeBirthTag(seal ppp.NativeReleaseSeal) (tag string, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("ppp: native birth policy panicked: %w", sessionPanicFailure(failure))
		}
	}()
	tag = seal.NativeBirthTag()
	if tag == "" || len(tag) > maxNativeBirthTagLength || strings.IndexByte(tag, 0) >= 0 {
		return "", errors.New("ppp: native birth tag must contain 1..255 bytes without a NUL byte")
	}
	return tag, nil
}

// NativeReleaseBound permits reservation only after the factory confirmed birth
// and retained the exact native unit, while this handle remains open.
func (s *session) NativeReleaseBound() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed && s.releaseSeal != nil && s.unitRetained
}

// NativeReleasePending distinguishes stopped borrowed I/O from completed native
// release. The driver MUST retain this exact owner while the result is true.
func (s *session) NativeReleasePending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed && s.unitRetained
}

// NativeReleaseBirthTag keeps the successful binding receipt independently of
// later policy changes and descriptor release. It never calls external code.
func (s *session) NativeReleaseBirthTag() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.birthTag
}

func (s *session) closeWithNativeReleaseSeal() error {
	// Serialize Close and binding, but never hold the session's operation lock
	// while an external seal waits for its bounded outstanding RPC.
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	s.mu.Lock()
	first := !s.closed
	s.closed = true
	retained := s.unitRetained
	seal, fd := s.releaseSeal, s.retainedUnit
	originalHold := s.retainedCreatedUnit
	s.mu.Unlock()

	if first {
		// Both readers must stop even when a remote result is uncertain. The
		// private duplicate keeps the same native unit alive through both closes.
		err := errors.Join(
			closeSessionStream(s.channel, s.channelFD, s.ops.closeFD),
			closeSessionStream(s.unit, s.unitFD, s.ops.closeFD),
		)
		s.mu.Lock()
		s.closeErr = err
		s.mu.Unlock()
	}
	if retained {
		if err := sealNativeRelease(seal); err != nil {
			s.mu.Lock()
			result := errors.Join(s.closeErr, err)
			s.mu.Unlock()
			return result
		}
		var err error
		if originalHold != nil {
			err = originalHold.close(s.ops.closeFD)
		} else {
			err = closeSessionStream(nil, fd, s.ops.closeFD)
		}
		s.mu.Lock()
		s.unitRetained = false
		s.retainedUnit = -1
		s.closeErr = errors.Join(s.closeErr, err)
		s.mu.Unlock()
	}
	s.mu.Lock()
	result := s.closeErr
	s.mu.Unlock()
	return result
}

func sealNativeRelease(seal ppp.NativeReleaseSeal) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("ppp: native release seal panicked: %w", sessionPanicFailure(failure))
		}
	}()
	return seal.SealNativeRelease()
}
