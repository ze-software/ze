// Design: docs/architecture/iface/logical-name-resolution.md -- original native VLAN custody
// Related: resolve_backend.go -- one exact backend generation; backend.go -- failed close retains its owner
package iface

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// ErrNativeAccessUnsupported means resolution supplied no original native VLAN
// owner. Ordinary discovery can use the binding. A requested native CoS change
// MUST refuse this result; a name, index or profile is not a replacement owner.
var ErrNativeAccessUnsupported = errors.New("interface has no original native VLAN owner")
var ErrNativeAccessPending = errors.New("original native VLAN is still in use")

const NativeVLANAliasPrefix = "ze:access-vlan:"
const NativeVLANRetiredAliasPrefix = "ze:access-vlan-retired:"

// NativeVLANBirth is a comparable packed identity, not a native capability.
// Its zero value is unavailable. Only constructors/parsers initialize its bytes.
type NativeVLANBirth struct{ value [16]byte }

func NewNativeVLANBirth() (NativeVLANBirth, error) {
	var birth NativeVLANBirth
	if _, err := rand.Read(birth.value[:]); err != nil {
		return NativeVLANBirth{}, err
	}
	if birth == (NativeVLANBirth{}) {
		return NativeVLANBirth{}, errors.New("native VLAN entropy returned an empty birth")
	}
	return birth, nil
}
func ParseNativeVLANBirth(alias string) (NativeVLANBirth, error) {
	return parseNativeVLANBirth(alias, NativeVLANAliasPrefix)
}
func ParseRetiredNativeVLANBirth(alias string) (NativeVLANBirth, error) {
	return parseNativeVLANBirth(alias, NativeVLANRetiredAliasPrefix)
}
func parseNativeVLANBirth(alias, prefix string) (NativeVLANBirth, error) {
	if !strings.HasPrefix(alias, prefix) {
		return NativeVLANBirth{}, errors.New("native VLAN alias has another owner kind")
	}
	wire := strings.TrimPrefix(alias, prefix)
	if len(wire) != 32 {
		return NativeVLANBirth{}, errors.New("native VLAN birth must contain 32 hexadecimal digits")
	}
	var birth NativeVLANBirth
	if _, err := hex.Decode(birth.value[:], []byte(wire)); err != nil {
		return NativeVLANBirth{}, err
	}
	if birth == (NativeVLANBirth{}) {
		return NativeVLANBirth{}, errors.New("native VLAN birth is empty")
	}
	if hex.EncodeToString(birth.value[:]) != wire {
		return NativeVLANBirth{}, errors.New("native VLAN birth is not canonical lowercase hexadecimal")
	}
	return birth, nil
}
func (b NativeVLANBirth) Alias() (string, error)        { return b.alias(NativeVLANAliasPrefix) }
func (b NativeVLANBirth) RetiredAlias() (string, error) { return b.alias(NativeVLANRetiredAliasPrefix) }
func (b NativeVLANBirth) alias(prefix string) (string, error) {
	if b == (NativeVLANBirth{}) {
		return "", errors.New("native VLAN birth is unavailable")
	}
	return prefix + hex.EncodeToString(b.value[:]), nil
}
func (b NativeVLANBirth) Compare(other NativeVLANBirth) int {
	return bytes.Compare(b.value[:], other.value[:])
}
func (b NativeVLANBirth) MarshalText() ([]byte, error) {
	alias, err := b.Alias()
	if err != nil {
		return nil, err
	}
	return []byte(alias), nil
}
func (b *NativeVLANBirth) UnmarshalText(text []byte) error {
	if b == nil {
		return errors.New("native VLAN birth destination is absent")
	}
	parsed, err := ParseNativeVLANBirth(string(text))
	if err != nil {
		return err
	}
	*b = parsed
	return nil
}

// NativeVLANIdentity is a self-contained boundary value, never a capability.
// The original native creator produces Birth. Consumers MUST NOT construct a
// capability from these labels or resolve another owner when an operation fails.
type NativeVLANIdentity struct {
	Device string          `json:"device"`
	Index  int             `json:"native-index"`
	Birth  NativeVLANBirth `json:"native-birth"`
}

// NativeVLANMaps is an owned snapshot; its maps never borrow a netlink reply.
type NativeVLANMaps struct {
	Ingress map[uint32]uint32
	Egress  map[uint32]uint32
}

// NativeVLANPreparation retains the created resource before any map operation.
// Only Borrow constructs a map-operation capability; ordinary static maps are
// still permitted while only preparation references exist.
type NativeVLANPreparation interface {
	Identity() (NativeVLANIdentity, error)
	Borrow(context.Context) (NativeVLANLease, error)
	Close() error
}

// NativeVLANLease retains the same created resource. Borrow MUST retain that
// resource pointer, never perform name/index lookup. Snapshot and Replace MUST
// use the creator's original native transport and verify its birth in one reply.
// The caller MUST join/fence all issued native operations before Close and keep
// the lease until its original participant's retirement ACK. Close ends one
// lease, not the interface; it is not an ACK for an independently issued command.
type NativeVLANLease interface {
	Identity() (NativeVLANIdentity, error)
	Borrow(context.Context) (NativeVLANLease, error)
	Snapshot(context.Context) (NativeVLANMaps, error)
	Replace(context.Context, NativeVLANMaps) error
	Close() error
}

// NativeVLANProvider is optional. CaptureNativeVLAN admits only a resource this
// exact backend created, matching every field of the original binding. It MUST
// increment native custody before publishing a lease and return Unsupported for
// unowned/preexisting links. Other failures mean unknown and MUST remain errors.
type NativeVLANProvider interface {
	CaptureNativeVLAN(context.Context, Binding) (NativeVLANPreparation, error)
}

// PreparedAccess retains a constructed native owner or an explicit unsupported
// variant beside the read-only discovery facts. Its zero value is unavailable.
// Safe concurrently. A caller MUST close it after discovery and sessions stop.
type PreparedAccess struct {
	binding Binding
	native  preparedNativeAccess
}
type preparedNativeAccess interface {
	borrow(context.Context) (NativeVLANLease, error)
	close() error
}
type ownedPreparedNativeAccess struct{ lease NativeVLANPreparation }
type unavailablePreparedNativeAccess struct{ cause error }

func (p ownedPreparedNativeAccess) borrow(ctx context.Context) (NativeVLANLease, error) {
	return p.lease.Borrow(ctx)
}
func (p ownedPreparedNativeAccess) close() error { return p.lease.Close() }
func (p unavailablePreparedNativeAccess) borrow(context.Context) (NativeVLANLease, error) {
	return nil, p.cause
}
func (unavailablePreparedNativeAccess) close() error { return nil } // Explicit unsupported variant acquired no native lease.
func (p *PreparedAccess) Binding() (Binding, error) {
	if p == nil {
		return Binding{}, errors.New("prepared native access is absent")
	}
	if p.native == nil {
		return Binding{}, errors.New("prepared native access is uninitialized")
	}
	return p.binding, nil
}
func (p *PreparedAccess) BorrowNativeVLAN(ctx context.Context) (NativeVLANLease, error) {
	if ctx == nil {
		return nil, errors.New("native access requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("prepared native access is absent")
	}
	if p.native == nil {
		return nil, errors.New("prepared native access is uninitialized")
	}
	return p.native.borrow(ctx)
}
func (p *PreparedAccess) Close() error {
	if p == nil {
		return errors.New("prepared native access is absent")
	}
	if p.native == nil {
		return errors.New("prepared native access is uninitialized")
	}
	return p.native.close()
}

// ResolvePreparedAccess uses the same exact-backend resolver and keeps its
// original creation owner before checking generation again. No replacement is
// loaded. Unsupported native CoS leaves ordinary discovery available explicitly.
func ResolvePreparedAccess(ctx context.Context, required, name string) (*PreparedAccess, error) {
	if ctx == nil {
		return nil, errors.New("prepared interface access requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	backendsMu.Lock()
	generation := activeBackendGeneration
	backendsMu.Unlock()
	backend, binding, err := ResolveBackend(required, name)
	if err != nil {
		return nil, err
	}
	backendsMu.Lock()
	if activeBackendGeneration != generation {
		backendsMu.Unlock()
		return nil, errors.New("interface backend generation changed before native access capture")
	}
	if activeBackendName != required {
		backendsMu.Unlock()
		return nil, errors.New("interface backend selection changed before native access capture")
	}
	backendsMu.Unlock()
	var native preparedNativeAccess
	if provider, ok := backend.(NativeVLANProvider); ok {
		lease, captureErr := provider.CaptureNativeVLAN(ctx, binding)
		if captureErr != nil {
			if !absentNativeCapability(lease) {
				return nil, errors.Join(captureErr, lease.Close())
			}
			if !errors.Is(captureErr, ErrNativeAccessUnsupported) {
				return nil, captureErr
			}
			native = unavailablePreparedNativeAccess{cause: captureErr}
		} else {
			if absentNativeCapability(lease) {
				return nil, errors.New("native VLAN provider returned no retained owner")
			}
			identity, identityErr := lease.Identity()
			if identityErr != nil {
				return nil, errors.Join(identityErr, lease.Close())
			}
			if identity.Device != binding.OsName {
				return nil, errors.Join(errors.New("native VLAN provider returned another device"), lease.Close())
			}
			if identity.Index != binding.Ifindex {
				return nil, errors.Join(errors.New("native VLAN provider returned another native index"), lease.Close())
			}
			originalBirth, birthErr := ParseNativeVLANBirth(binding.Alias)
			if birthErr != nil {
				return nil, errors.Join(birthErr, lease.Close())
			}
			if identity.Birth != originalBirth {
				return nil, errors.Join(errors.New("native VLAN provider returned another native birth"), lease.Close())
			}
			native = ownedPreparedNativeAccess{lease: lease}
		}
	} else {
		native = unavailablePreparedNativeAccess{cause: fmt.Errorf("%w: backend %q provides no native VLAN custody", ErrNativeAccessUnsupported, required)}
	}
	backendsMu.Lock()
	generationChanged := activeBackendGeneration != generation
	selectionChanged := activeBackendName != required
	backendsMu.Unlock()
	if generationChanged {
		return nil, errors.Join(errors.New("interface backend generation changed while capturing original access"), native.close())
	}
	if selectionChanged {
		return nil, errors.Join(errors.New("interface backend selection changed while capturing original access"), native.close())
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, native.close())
	}
	return &PreparedAccess{binding: binding, native: native}, nil
}
func absentNativeCapability(capability any) bool {
	if capability == nil {
		return true
	}
	value := reflect.ValueOf(capability)
	//exhaustive:ignore -- only nil-capable implementation kinds can hold typed nil.
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// NativeVLANOperationJoiner reports physical return of original native work.
// The caller MUST fence its participant before joining; nil confirms no issued
// original native operation is still running, not that an unknown write worked.
// A deadline/error MUST retain the same lease and original command receipt.
type NativeVLANOperationJoiner interface{ JoinNativeVLAN(context.Context) error }
