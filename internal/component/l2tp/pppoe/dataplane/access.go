// Design: docs/architecture/l2tp/bng-5-pppoe.md -- prepared access ownership
// Related: registry.go -- exact provider selection before discovery starts

package dataplane

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode"
)

// AccessInfo is a provider's raw resolution snapshot. Device identifies the
// session data-plane device; ControlDevice and DiscoveryIfIndex identify the
// Linux Ethernet device in Namespace. HWAddr is that Linux device's source
// MAC, which can differ from the data-plane MAC. Empty Namespace means the
// caller's network namespace. NewAccess validates and copies the snapshot.
type AccessInfo struct {
	Device           string
	ControlDevice    string
	Namespace        string
	DiscoveryIfIndex int
	HWAddr           [6]byte
	MTU              int
}

// Access is an immutable resolution with an owned preparation lease. Nil is
// invalid; NewAccess and NewRecoverableAccess construct one. The caller MUST
// Close it after every session using the access has stopped and discovery no
// longer dispatches to it. A failed start/reload MUST close every unpublished
// access. Accessors and Close are safe for concurrent use; Close invokes the
// owner once and returns the same result to every caller. Closing does not
// mutate the snapshot.
type Access interface {
	Device() string
	ControlDevice() string
	Namespace() string
	DiscoveryIfIndex() int
	HWAddr() [6]byte
	MTU() int
	Close() error
	preparedAccess()
}

// NewAccess consumes owner on every outcome. A provider with no resource to
// release MUST supply an explicit no-resource owner; a missing or typed-nil
// owner is refused. A failed validation closes the owner before returning and
// retains cleanup errors. On success the caller owns the returned lease.
// On failure it MUST return a nil Access, not an interface holding a nil pointer.
func NewAccess(info AccessInfo, owner io.Closer) (Access, error) {
	access, err := newAccess(info, owner)
	if err != nil {
		return nil, err
	}
	return access, nil
}

// newAccess MUST own validation and owner consumption for both public constructors.
// Its concrete success result lets recovery bind before interface publication.
// A nil result always accompanies an error; no owner is published on failure.
func newAccess(info AccessInfo, owner io.Closer) (*preparedAccess, error) {
	if err := validateAccess(info); err != nil {
		return nil, errors.Join(err, closeAccessOwner(owner))
	}
	if nilResource(owner) {
		return nil, errors.New("PPPoE access has no preparation owner")
	}
	return &preparedAccess{info: info, owner: owner, closed: make(chan struct{})}, nil
}

func validateAccess(info AccessInfo) error {
	if info.Device == "" {
		return errors.New("PPPoE access has no session device")
	}
	if strings.ContainsRune(info.Device, 0) {
		return errors.New("PPPoE access session device contains a NUL byte")
	}
	if info.ControlDevice == "" {
		return errors.New("PPPoE access has no Linux control device")
	}
	const linuxDeviceMax = 15
	if len(info.ControlDevice) > linuxDeviceMax {
		return fmt.Errorf("PPPoE access control device %q exceeds %d bytes", info.ControlDevice, linuxDeviceMax)
	}
	if strings.ContainsRune(info.ControlDevice, 0) {
		return errors.New("PPPoE access control device contains a NUL byte")
	}
	if len(info.Namespace) > 255 {
		return errors.New("PPPoE access namespace exceeds 255 bytes")
	}
	if strings.ContainsAny(info.Namespace, "/\\\x00") {
		return errors.New("PPPoE access namespace contains a path separator or NUL byte")
	}
	for _, r := range info.Namespace {
		if !unicode.IsPrint(r) {
			return errors.New("PPPoE access namespace contains a non-printable character")
		}
		if unicode.IsSpace(r) {
			return errors.New("PPPoE access namespace contains whitespace")
		}
	}
	if info.DiscoveryIfIndex <= 0 {
		return errors.New("PPPoE access has no positive Linux discovery interface index")
	}
	if info.HWAddr == [6]byte{} {
		return errors.New("PPPoE access has no Linux source MAC address")
	}
	if info.HWAddr[0]&1 != 0 {
		return errors.New("PPPoE access Linux source MAC address is not unicast")
	}
	if info.MTU <= 0 {
		return errors.New("PPPoE access has no positive MTU")
	}
	return nil
}

type preparedAccess struct {
	info      AccessInfo
	owner     io.Closer
	closeOnce sync.Once
	closeErr  error
	closed    chan struct{}  // completed first Close; publishes its immutable outcome
	recovery  AccessRecovery // immutable original owner; nil for NewAccess
}

func (a *preparedAccess) Device() string        { return a.info.Device }
func (a *preparedAccess) ControlDevice() string { return a.info.ControlDevice }
func (a *preparedAccess) Namespace() string     { return a.info.Namespace }
func (a *preparedAccess) DiscoveryIfIndex() int { return a.info.DiscoveryIfIndex }
func (a *preparedAccess) HWAddr() [6]byte       { return a.info.HWAddr }
func (a *preparedAccess) MTU() int              { return a.info.MTU }
func (*preparedAccess) preparedAccess()         {}

func (a *preparedAccess) Close() error {
	a.closeOnce.Do(func() { a.closeErr = closeAccessOwner(a.owner); close(a.closed) })
	return a.closeErr
}

// A broken cleanup cannot bypass a failed preparation's release boundary.
// Error panic payloads preserve their identity in the returned close result.
func closeAccessOwner(owner io.Closer) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			if cause, ok := failure.(error); ok {
				err = fmt.Errorf("PPPoE access cleanup panicked: %w", cause)
				return
			}
			err = fmt.Errorf("PPPoE access cleanup panicked: %v", failure)
		}
	}()
	if nilResource(owner) {
		return nil
	}
	return owner.Close()
}
