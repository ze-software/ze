// Design: docs/architecture/traffic/cos-dynamic.md -- original cleanup target
// Related: shared_access.go -- retained baseline and original warning lifetime

//go:build ze_l2tp

package cos

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	coreCos "github.com/ze-software/ze/internal/core/cos"
)

const originalCoSOperationLimit = 2 * time.Second

// A target is retained before its first Snapshot. Implementations retain the
// actual recipient, or explicitly refuse the newly introduced delayed paths.
// The interface is private; a name or a callback is never a native capability.
type originalCoSTarget interface {
	readLink(string) (*iface.InterfaceInfo, error)
	write(string, map[uint32]uint32, map[uint32]uint32) error
	allowRecovery() error
	stopOwner()
	finish() error
}

type originalCoSTargetFactory func(string) (originalCoSTarget, error)

// creatorCoSTarget owns one borrowed map lease, including the original native
// transport. Neither reads nor writes resolve a current backend or device.
type creatorCoSTarget struct {
	name     string
	identity iface.NativeVLANIdentity
	lease    iface.NativeVLANLease
	join     iface.NativeVLANOperationJoiner
}

func captureCreatorCoSTarget(name string) (originalCoSTarget, error) {
	ctx, cancel := context.WithTimeout(context.Background(), originalCoSOperationLimit)
	defer cancel()
	prepared, err := iface.ResolvePreparedAccess(ctx, iface.ActiveBackendName(), name)
	if err != nil {
		return nil, err
	}
	lease, err := prepared.BorrowNativeVLAN(ctx)
	closeErr := prepared.Close()
	if err != nil {
		return nil, errors.Join(err, closeErr)
	}
	if closeErr != nil {
		return nil, errors.Join(closeErr, lease.Close())
	}
	identity, err := lease.Identity()
	if err != nil {
		return nil, errors.Join(err, lease.Close())
	}
	join, valid := lease.(iface.NativeVLANOperationJoiner)
	if !valid {
		return nil, errors.Join(errors.New("the original VLAN lease cannot join its issued native operations"), lease.Close())
	}
	return &creatorCoSTarget{name: name, identity: identity, lease: lease, join: join}, nil
}

func (target *creatorCoSTarget) readLink(name string) (*iface.InterfaceInfo, error) {
	if name != target.name {
		return nil, errors.New("the retained class-of-service target names another access")
	}
	ctx, cancel := context.WithTimeout(context.Background(), originalCoSOperationLimit)
	defer cancel()
	if err := target.join.JoinNativeVLAN(ctx); err != nil {
		return nil, err
	}
	actual, err := target.lease.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return &iface.InterfaceInfo{Name: target.identity.Device, Index: target.identity.Index, Type: "vlan",
		IngressQoSMap: maps.Clone(actual.Ingress), EgressQoSMap: maps.Clone(actual.Egress)}, nil
}

func (target *creatorCoSTarget) write(name string, ingress, egress map[uint32]uint32) error {
	if name != target.name {
		return errors.New("the retained class-of-service target names another access")
	}
	desired, err := newSharedCoSProfile(coreCos.Profile{IngressMap: ingress, EgressMap: egress})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), originalCoSOperationLimit)
	defer cancel()
	if err := target.join.JoinNativeVLAN(ctx); err != nil {
		return err
	}
	if err := target.lease.Replace(ctx, iface.NativeVLANMaps{Ingress: maps.Clone(ingress), Egress: maps.Clone(egress)}); err != nil {
		return err
	}
	actual, err := target.lease.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("confirm original class-of-service maps: %w", err)
	}
	confirmed, err := newSharedCoSProfile(coreCos.Profile{IngressMap: actual.Ingress, EgressMap: actual.Egress})
	if err != nil {
		return err
	}
	if confirmed != desired {
		return errors.New("original class-of-service map readback does not match the requested effective maps")
	}
	return nil
}

func (*creatorCoSTarget) allowRecovery() error { return nil }
func (*creatorCoSTarget) stopOwner()           {}

func (target *creatorCoSTarget) finish() error {
	ctx, cancel := context.WithTimeout(context.Background(), originalCoSOperationLimit)
	defer cancel()
	if err := target.join.JoinNativeVLAN(ctx); err != nil {
		return err
	}
	return target.lease.Close()
}

// Unsupported legacy access keeps the historical immediate callback path. It
// grants no new operator retry or stopped-runner mutation authority. Once the
// original runner stops, every callback on this retained target refuses too.
type unresolvedCoSTarget struct {
	name    string
	read    func(string) (*iface.InterfaceInfo, error)
	update  updateQoSFunc
	stopped bool
}

type originalCoSTargetUnavailable struct{ name string }

func (failure *originalCoSTargetUnavailable) Error() string {
	return fmt.Sprintf("the original native recipient for access %q was not captured; a replacement or matching device label cannot authorize delayed cleanup", failure.name)
}

const originalCoSTargetFix = "Preserve this daemon and warning. Restore the recorded baseline with the original device owner; replacement VLANs cannot confirm it. Create future VLANs through Ze's interface backend before admitting sessions."

func (target *unresolvedCoSTarget) allowRecovery() error {
	return &originalCoSTargetUnavailable{name: target.name}
}

func (target *unresolvedCoSTarget) stopOwner() { target.stopped = true }
func (*unresolvedCoSTarget) finish() error     { return nil }

func (target *unresolvedCoSTarget) readLink(name string) (*iface.InterfaceInfo, error) {
	if target.stopped {
		return nil, target.allowRecovery()
	}
	if name != target.name {
		return nil, errors.New("the legacy class-of-service target names another access")
	}
	return target.read(name)
}

func (target *unresolvedCoSTarget) write(name string, ingress, egress map[uint32]uint32) error {
	if target.stopped {
		return target.allowRecovery()
	}
	if name != target.name {
		return errors.New("the legacy class-of-service target names another access")
	}
	return target.update(name, ingress, egress)
}

func (h *cosHandler) captureOriginalCoSTarget(name string) (originalCoSTarget, error) {
	if h.captureTarget != nil {
		return h.captureTarget(name)
	}
	if iface.GetBackend() != nil {
		target, err := captureCreatorCoSTarget(name)
		if err == nil {
			return target, nil
		}
		if !errors.Is(err, iface.ErrNativeAccessUnsupported) {
			return nil, err
		}
	}
	return &unresolvedCoSTarget{name: name, read: h.readLink, update: h.updateQoS}, nil
}

// First-Snapshot failure retains the already captured lease, although there is
// no usable baseline and no user was admitted. Cleanup can only join and close.
type sharedCoSCapturePending struct {
	access *sharedCoSAccess
	cause  error
}

func (pending *sharedCoSCapturePending) sharedOwner() *sharedCoSAccess { return pending.access }
func (*sharedCoSCapturePending) sharedAccessState()                    {}

// Native restoration already succeeded. Only the original issued-operation
// join and reference release remain; retry never repeats a native map write.
type sharedCoSReleasePending struct {
	access *sharedCoSAccess
	cause  error
}

func (pending *sharedCoSReleasePending) sharedOwner() *sharedCoSAccess { return pending.access }
func (*sharedCoSReleasePending) sharedAccessState()                    {}

func (h *cosHandler) releaseUnusedSharedAccess(access *sharedCoSAccess) error {
	if len(access.users) != 0 {
		return nil
	}
	if h.accesses[access.name] != access {
		return nil
	}
	if h.pending != nil {
		if h.pending.prepared().access == access {
			return nil
		}
	}
	if err := access.target.finish(); err != nil {
		h.accesses[access.name] = &sharedCoSReleasePending{access: access, cause: err}
		return err
	}
	delete(h.accesses, access.name)
	if _, unraised := access.warning.(sharedCoSWarningUnraised); unraised {
		return h.closeUnusedCleanupReservation(access)
	}
	// Native release is complete; clearance is a separate obligation. Keeping
	// this variant makes even an interrupted caller's next retry report-only.
	h.accesses[access.name] = &sharedCoSReportPending{access: access}
	return nil
}

func (h *cosHandler) finishRetainedSharedAccess(state sharedCoSAccessState) error {
	access := state.sharedOwner()
	switch pending := state.(type) {
	case *sharedCoSCapturePending:
		if err := access.target.finish(); err != nil {
			return errors.Join(pending.cause, err)
		}
	case *sharedCoSReleasePending:
		if err := access.target.finish(); err != nil {
			return errors.Join(pending.cause, err)
		}
	default:
		return errors.New("the retained class-of-service state is not awaiting original resource release")
	}
	delete(h.accesses, access.name)
	return nil
}
