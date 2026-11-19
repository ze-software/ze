// Design: docs/architecture/appliance/kernel-profiles.md -- original PPP IPv6 control sockets
// Related: created_unit.go -- sole original creator and descriptor borrow

package kernel

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// IPv6ControlSocketKind selects the two carried original-unit control roles.
// Zero is unavailable. Arbitrary numeric conversions are refused at admission.
type IPv6ControlSocketKind uint32

const (
	IPv6ControlSocketUnspecified IPv6ControlSocketKind = iota
	IPv6ControlSocketICMP6Raw
	IPv6ControlSocketUDP6
)

// OpenIPv6ControlSocket creates a new owned file for this exact confirmed unit.
// Safe for concurrent use and unit closure. The caller MUST invoke it through
// the existing joined original-namespace helper: a foreign caller namespace
// is refused by the creator-only positive query, never silently entered here.
// Query51 must positively advertise complete control bit9 before ioctl43 runs;
// both native operations use the same short actual WithDescriptor borrow.
// No network/ACK wait or duplicate of the creator file occurs in that borrow.
//
// A nonnil file is always the newly returned owned control socket, including
// descriptor zero; the caller MUST Close it even when an error accompanies it.
// This method never closes that returned file on the caller's behalf. Labels,
// global backend/current-name lookup or an attached file supply no authority.
// A nil/zero, closed or unconfirmed CreatedUnit is unavailable and is refused.
func (unit *CreatedUnit) OpenIPv6ControlSocket(ctx context.Context, kind IPv6ControlSocketKind) (*os.File, error) {
	if ctx == nil {
		return nil, errors.New("ppp: original IPv6 control socket has no context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateIPv6ControlSocketKind(kind); err != nil {
		return nil, err
	}
	if unit == nil {
		return nil, errors.New("ppp: original IPv6 control socket has no created unit")
	}
	unit.mu.Lock()
	if unit.closed {
		unit.mu.Unlock()
		return nil, os.ErrClosed
	}
	if !unit.ready {
		unit.mu.Unlock()
		return nil, errors.New("ppp: original unit creation was not confirmed")
	}
	if unit.file == nil {
		unit.mu.Unlock()
		return nil, errors.New("ppp: confirmed original unit has no creator file")
	}
	// Only the actual creator writes these fields before publishing ready.
	// Neither label is accepted as a separate public construction parameter.
	index := unit.identity.NativeIndex
	number := unit.number
	unit.mu.Unlock()
	if index == 0 {
		return nil, errors.New("ppp: original IPv6 control socket has no captured native index")
	}
	if index > uint32(1<<31-1) {
		return nil, errors.New("ppp: original IPv6 control index exceeds signed native range")
	}
	if number < 0 {
		return nil, errors.New("ppp: original IPv6 control socket has no captured unit number")
	}
	if uint64(number) > uint64(1<<31-1) {
		return nil, errors.New("ppp: original IPv6 control unit exceeds signed native range")
	}
	return unit.openIPv6ControlSocket(ctx, kind, int32(index), int32(number))
}

func validateIPv6ControlSocketKind(kind IPv6ControlSocketKind) error {
	switch kind {
	case IPv6ControlSocketICMP6Raw, IPv6ControlSocketUDP6:
		return nil
	case IPv6ControlSocketUnspecified:
		return errors.New("ppp: original IPv6 control socket kind is unspecified")
	default:
		// The caller can convert a numeric value; this is open boundary input.
		return fmt.Errorf("ppp: original IPv6 control socket kind %d is unsupported", kind)
	}
}
