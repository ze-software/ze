// Design: docs/features/interfaces.md -- creation-owned VLAN lifetimes
// Related: manage_linux.go -- the same qosMapMu orders create, native writes and delete
//go:build linux

package ifacenetlink

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

const nativeVLANAliasPrefix = iface.NativeVLANAliasPrefix
const nativeVLANResourceLimit = 4096
const nativeVLANLeaseLimit = 65536
const nativeAccessIndexMax = 1<<31 - 1
const nativeVLANOperationLimit = 2 * time.Second

type nativeVLANState interface{ nativeVLANState() }
type nativeVLANUnmarked struct{ cause error }
type nativeVLANPending struct{ cause error }
type nativeVLANReady struct{}
type nativeVLANRetiring struct{ cause error }
type nativeVLANRetired struct{}

func (nativeVLANUnmarked) nativeVLANState() {}
func (nativeVLANPending) nativeVLANState()  {}
func (nativeVLANReady) nativeVLANState()    {}
func (nativeVLANRetiring) nativeVLANState() {}
func (nativeVLANRetired) nativeVLANState()  {}

// nativeVLANResource is the native creator's resource, not a selector registry.
// Its birth/spec are fixed before the first create; index is fixed before Ready.
// qosMapMu owns all fields except the bounded atomic lease count.
type nativeVLANResource struct {
	owner        *nativeVLANResources
	identity     iface.NativeVLANIdentity
	alias        string
	retiredAlias string
	parent       int
	parentName   string
	vlan         int
	state        nativeVLANState
	leases       atomic.Uint32
	writers      atomic.Uint32
	creation     *nativeVLANTransaction // Original CREATE request; never reconstructed by a selector.
}
type nativeVLANOps struct {
	byName         func(string) (netlink.Link, error)
	byIndex        func(int) (netlink.Link, error)
	add            func(netlink.Link) (netlink.Link, error)
	list           func() ([]netlink.Link, error)
	bind           func(context.Context) error
	bindCreate     func(*nativeVLANResource) error
	resumeOriginal func(context.Context) error
	transport      *nativeVLANTransport
	up             func(netlink.Link) error
	modify         func(netlink.Link) error
	remove         func(netlink.Link) error
	alias          func(netlink.Link, string) error
	timeout        func(time.Duration) error
	close          func() error
}
type nativeVLANBackendPhase uint8

const (
	nativeVLANBackendUnspecified nativeVLANBackendPhase = iota
	nativeVLANBackendOpen
	nativeVLANBackendClosing
	nativeVLANBackendClosed
)

// This collection contains only this backend's creation receipts, including
// uncertain creates. Native leases retain the original pointer and never look
// it up again. Both count limits are checked before native mutation/publication.
type nativeVLANResources struct {
	opsMu                       sync.Mutex                     // Sole publication lock; published ops never changes.
	openOpsForTest              func() (*nativeVLANOps, error) // Set only before first use; nil uses the real factory.
	observeOpsContentionForTest func()                         // Set before first use; observe only, no blocking or lock acquisition.
	workerMu                    sync.Mutex
	worker                      *nativeVLANWorker
	records                     map[iface.NativeVLANBirth]*nativeVLANResource
	ops                         *nativeVLANOps
	phase                       nativeVLANBackendPhase
	retireNext                  int
}

func (resources *nativeVLANResources) beginLocked(ctx context.Context) error {
	if ctx == nil {
		return errors.New("native VLAN operation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if resources.phase == nativeVLANBackendClosed {
		return os.ErrClosed
	}
	if err := resources.initializeOps(); err != nil {
		return err
	}
	if resources.phase == nativeVLANBackendUnspecified {
		resources.phase = nativeVLANBackendOpen
	}
	budget := nativeVLANOperationLimit
	if deadline, ok := ctx.Deadline(); ok {
		budget = min(budget, time.Until(deadline))
	}
	if budget < time.Microsecond {
		return context.DeadlineExceeded
	}
	if resources.ops.bind != nil {
		if err := resources.ops.bind(ctx); err != nil {
			return err
		}
	}
	return resources.ops.timeout(budget)
}

// createDirect MUST run only on the retained original worker under qosMapMu.
// CREATE identity comes from its exact native echo. Unmarked uncertainty never stamps later.
func (resources *nativeVLANResources) createDirect(ctx context.Context, spec iface.VLANSpec) error {
	if err := iface.ValidateIfaceName(spec.Parent); err != nil {
		return err
	}
	if err := validateVLANID(spec.VLANID); err != nil {
		return err
	}
	if err := validateQoSMap(spec.IngressQoSMap); err != nil {
		return err
	}
	if err := validateQoSMap(spec.EgressQoSMap); err != nil {
		return err
	}
	name := fmt.Sprintf("%s.%d", spec.Parent, spec.VLANID)
	if err := iface.ValidateIfaceName(name); err != nil {
		return err
	}
	if resources.phase == nativeVLANBackendClosing {
		return iface.ErrNativeAccessPending
	}
	if err := resources.beginLocked(ctx); err != nil {
		return err
	}
	if existing := resources.byNameLocked(name); existing != nil {
		if _, ready := existing.state.(nativeVLANReady); ready {
			if _, err := existing.readLocked(ctx); err != nil {
				return err
			}
			return iface.ErrInterfaceExists
		}
		// An unmarked CREATE that released this fence can never authorize a later
		// stamp, even if its original delayed echo supplies the same native index.
		if unmarked, ok := existing.state.(nativeVLANUnmarked); ok {
			return errors.Join(iface.ErrNativeAccessPending, unmarked.cause)
		}
		if _, pending := existing.state.(nativeVLANPending); !pending {
			return iface.ErrNativeAccessPending
		}
		link, err := existing.readLocked(ctx)
		if err != nil {
			return existing.retainedCause(err)
		}
		if err := resources.beginLocked(ctx); err != nil {
			return err
		}
		if err := resources.ops.up(link); err != nil {
			existing.state = nativeVLANPending{err}
			return err
		}
		existing.state = nativeVLANReady{}
		return nil
	}
	if len(resources.records) >= nativeVLANResourceLimit {
		return errors.New("native VLAN creation receipt capacity exhausted")
	}
	parent, err := resources.ops.byName(spec.Parent)
	if err != nil {
		return err
	}
	if parent == nil {
		return errors.New("native VLAN parent is absent")
	}
	if parent.Attrs() == nil {
		return errors.New("native VLAN parent attributes are absent")
	}
	if parent.Attrs().Index <= 0 {
		return errors.New("native VLAN parent has no original index")
	}
	birth, err := iface.NewNativeVLANBirth()
	if err != nil {
		return err
	}
	if resources.records[birth] != nil {
		return errors.New("native VLAN birth already belongs to a creation receipt")
	}
	alias, err := birth.Alias()
	if err != nil {
		return err
	}
	retiredAlias, err := birth.RetiredAlias()
	if err != nil {
		return err
	}
	record := &nativeVLANResource{owner: resources, identity: iface.NativeVLANIdentity{Device: name, Birth: birth}, alias: alias, retiredAlias: retiredAlias, parent: parent.Attrs().Index, parentName: spec.Parent, vlan: spec.VLANID, state: nativeVLANUnmarked{errors.New("original native CREATE is unconfirmed")}}
	if resources.records == nil {
		resources.records = make(map[iface.NativeVLANBirth]*nativeVLANResource)
	}
	resources.records[birth] = record
	link := &netlink.Vlan{Name: name, ParentIndex: record.parent, VlanId: record.vlan, IngressQosMap: maps.Clone(spec.IngressQoSMap), EgressQosMap: maps.Clone(spec.EgressQoSMap)}
	if err := resources.beginLocked(ctx); err != nil {
		delete(resources.records, birth)
		return err
	}
	// The result comes from this exact CREATE socket/sequence's native echo,
	// never from LinkByName/LinkByIndex or a subsequent current-owner lookup.
	if resources.ops.bindCreate != nil {
		if err := resources.ops.bindCreate(record); err != nil {
			delete(resources.records, birth)
			return err
		}
	}
	actual, createErr := resources.ops.add(link)
	if actual != nil {
		if err := record.createdEcho(actual); err != nil {
			record.state = nativeVLANUnmarked{errors.Join(createErr, err)}
			return record.retainedCause(err)
		}
	}
	if createErr != nil {
		if original := record.creation; original != nil {
			if original.sendPhase == nativeVLANSendPrepared {
				delete(resources.records, birth)
				return createErr
			}
			if original.nativeSuccess() {
				if ctx.Err() != nil {
					return record.rollbackUnpublishedOriginal(actual, createErr)
				}
			}
		}
		if errors.Is(createErr, unix.EEXIST) {
			delete(resources.records, birth)
		} else {
			record.state = nativeVLANUnmarked{createErr}
		}
		return createErr
	}
	if actual == nil {
		err := errors.New("successful original CREATE supplied no native echo")
		record.state = nativeVLANUnmarked{err}
		return err
	}
	if err := resources.beginLocked(ctx); err != nil {
		record.state = nativeVLANUnmarked{err}
		return err
	}
	// Linux ignores Alias at create (LL-RTNL-NEWLINK-NO-ALIAS). Stamp only
	// while the SAME uninterrupted creator fence still protects the native echo.
	record.state = nativeVLANPending{errors.New("original native ownership stamp is unconfirmed")}
	if err := resources.ops.alias(actual, record.alias); err != nil {
		record.state = nativeVLANPending{err}
		return err
	}
	installed, err := record.readLocked(ctx)
	if err != nil {
		record.state = nativeVLANPending{err}
		return err
	}
	if err := resources.beginLocked(ctx); err != nil {
		record.state = nativeVLANPending{err}
		return err
	}
	if err := resources.ops.up(installed); err != nil {
		record.state = nativeVLANPending{err}
		return err
	}
	record.state = nativeVLANReady{}
	return nil
}

// Names are raw native/config selectors only here. No lease ever calls this
// bounded boundary lookup again after it retained its original resource pointer.
func (resources *nativeVLANResources) byNameLocked(name string) *nativeVLANResource {
	for _, record := range resources.records {
		if record.identity.Device == name {
			return record
		}
	}
	return nil
}

func (record *nativeVLANResource) readLocked(ctx context.Context) (*netlink.Vlan, error) {
	resources := record.owner
	if err := resources.beginLocked(ctx); err != nil {
		return nil, err
	}
	var link netlink.Link
	var err error
	if record.identity.Index == 0 {
		return nil, errors.New("original CREATE supplied no confirmed native echo index")
	}
	link, err = resources.ops.byIndex(record.identity.Index)
	if err != nil {
		return nil, err
	}
	if _, unmarked := record.state.(nativeVLANUnmarked); unmarked {
		return nil, errors.New("unmarked original index permits only absence reconciliation")
	}
	vlan, ok := link.(*netlink.Vlan)
	if !ok {
		return nil, errors.New("original native access is no longer a VLAN")
	}
	if vlan == nil {
		return nil, errors.New("original native access reply is empty")
	}
	if vlan.Attrs().Name != record.identity.Device {
		return nil, errors.New("native access reply names another device")
	}
	actualBirth, birthErr := iface.ParseNativeVLANBirth(vlan.Attrs().Alias)
	if birthErr != nil {
		if _, retiring := record.state.(nativeVLANRetiring); !retiring {
			return nil, errors.New("native access reply names another birth")
		}
		actualBirth, birthErr = iface.ParseRetiredNativeVLANBirth(vlan.Attrs().Alias)
		if birthErr != nil {
			return nil, birthErr
		}
	}
	if actualBirth != record.identity.Birth {
		return nil, errors.New("native access reply names another birth")
	}
	if vlan.Attrs().Index <= 0 {
		return nil, errors.New("native access reply has no positive kernel index")
	}
	if vlan.Attrs().Index > nativeAccessIndexMax {
		return nil, errors.New("native access reply exceeds the kernel index range")
	}
	if record.identity.Index != 0 {
		if vlan.Attrs().Index != record.identity.Index {
			return nil, errors.New("native access reply names another index")
		}
	}
	if vlan.ParentIndex != record.parent {
		return nil, errors.New("native access reply names another parent")
	}
	if vlan.VlanId != record.vlan {
		return nil, errors.New("native access reply names another VLAN")
	}
	return vlan, nil
}
func (record *nativeVLANResource) retainedCause(err error) error {
	switch state := record.state.(type) {
	case nativeVLANUnmarked:
		return errors.Join(state.cause, err)
	case nativeVLANPending:
		return errors.Join(state.cause, err)
	case nativeVLANRetiring:
		return errors.Join(state.cause, err)
	case nativeVLANReady:
		return err
	case nativeVLANRetired:
		return err
	}
	panic("BUG: native VLAN creator stored an unknown lifecycle variant")
}

func (b *netlinkBackend) captureNativeVLANDirect(ctx context.Context, binding iface.Binding) (iface.NativeVLANPreparation, error) {
	if ctx == nil {
		return nil, errors.New("native access capture requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b.nativeVLANs.phase == nativeVLANBackendClosed {
		return nil, os.ErrClosed
	}
	if b.nativeVLANs.phase == nativeVLANBackendClosing {
		return nil, iface.ErrNativeAccessPending
	}
	if !strings.HasPrefix(binding.Alias, nativeVLANAliasPrefix) {
		return nil, fmt.Errorf("%w: access has no native creation marker", iface.ErrNativeAccessUnsupported)
	}
	birth, err := iface.ParseNativeVLANBirth(binding.Alias)
	if err != nil {
		return nil, err
	}
	record := b.nativeVLANs.records[birth]
	if record == nil {
		return nil, fmt.Errorf("%w: access was not created by this native owner", iface.ErrNativeAccessUnsupported)
	}
	if _, ready := record.state.(nativeVLANReady); !ready {
		if pending, ok := record.state.(nativeVLANPending); ok {
			return nil, errors.Join(iface.ErrNativeAccessPending, pending.cause)
		}
		return nil, errors.New("native access creation is not confirmed")
	}
	if binding.Ifindex != record.identity.Index {
		return nil, errors.New("native access binding names another index")
	}
	if binding.OsName != record.identity.Device {
		return nil, errors.New("native access binding names another device")
	}
	if _, err := record.readLocked(ctx); err != nil {
		return nil, err
	}
	return record.referenceLocked()
}
func (record *nativeVLANResource) referenceLocked() (*nativeVLANReference, error) {
	if record.owner.phase == nativeVLANBackendClosed {
		return nil, os.ErrClosed
	}
	if record.owner.phase == nativeVLANBackendClosing {
		return nil, iface.ErrNativeAccessPending
	}
	if _, ready := record.state.(nativeVLANReady); !ready {
		return nil, errors.New("native VLAN owner is not active")
	}
	if record.leases.Load() >= nativeVLANLeaseLimit {
		return nil, errors.New("native VLAN lease capacity exhausted")
	}
	record.leases.Add(1) // Increments hold qosMapMu; concurrent closes can only reduce it.
	return &nativeVLANReference{resource: record, identity: record.identity, release: func() { record.leases.Add(^uint32(0)) }}, nil
}
func (record *nativeVLANResource) borrowLocked() (iface.NativeVLANLease, error) {
	reference, err := record.referenceLocked()
	if err != nil {
		return nil, err
	}
	record.writers.Add(1)
	reference.release = func() { record.writers.Add(^uint32(0)); record.leases.Add(^uint32(0)) }
	return &nativeVLANLease{nativeVLANReference: reference}, nil
}

type nativeVLANReference struct {
	resource *nativeVLANResource
	identity iface.NativeVLANIdentity
	closed   atomic.Bool
	release  func()
}
type nativeVLANLease struct{ *nativeVLANReference }

func (lease *nativeVLANReference) Identity() (iface.NativeVLANIdentity, error) {
	if lease == nil {
		return iface.NativeVLANIdentity{}, errors.New("native VLAN lease is absent")
	}
	if lease.resource == nil {
		return iface.NativeVLANIdentity{}, errors.New("native VLAN lease is uninitialized")
	}
	if lease.closed.Load() {
		return iface.NativeVLANIdentity{}, os.ErrClosed
	}
	return lease.identity, nil
}
func (lease *nativeVLANReference) Borrow(ctx context.Context) (iface.NativeVLANLease, error) {
	if ctx == nil {
		return nil, errors.New("native VLAN borrow requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if lease == nil {
		return nil, errors.New("native VLAN lease is absent")
	}
	if lease.resource == nil {
		return nil, errors.New("native VLAN lease is uninitialized")
	}
	if !qosMapMu.TryLock() {
		return nil, iface.ErrNativeAccessPending
	}
	defer qosMapMu.Unlock()
	if lease.closed.Load() {
		return nil, os.ErrClosed
	}
	return lease.resource.borrowLocked()
}
func (lease *nativeVLANReference) Close() error {
	if lease == nil {
		return errors.New("native VLAN lease is absent")
	}
	if lease.resource == nil {
		return errors.New("native VLAN lease is uninitialized")
	}
	if lease.closed.CompareAndSwap(false, true) {
		lease.release()
	}
	return nil
}
func (lease *nativeVLANLease) operationLocked(ctx context.Context) error {
	if ctx == nil {
		return errors.New("native VLAN operation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if lease == nil {
		return errors.New("native VLAN lease is absent")
	}
	if lease.nativeVLANReference == nil {
		return errors.New("native VLAN map lease is uninitialized")
	}
	if lease.resource == nil {
		return errors.New("native VLAN resource is absent")
	}
	if lease.closed.Load() {
		return os.ErrClosed
	}
	if lease.resource.identity != lease.identity {
		return errors.New("native VLAN receipt identity changed")
	}
	if _, ready := lease.resource.state.(nativeVLANReady); !ready {
		return errors.New("native VLAN receipt is retired")
	}
	return nil
}
func (lease *nativeVLANLease) snapshotDirect(ctx context.Context) (iface.NativeVLANMaps, error) {
	if err := lease.operationLocked(ctx); err != nil {
		return iface.NativeVLANMaps{}, err
	}
	actual, err := lease.resource.readLocked(ctx)
	if err != nil {
		return iface.NativeVLANMaps{}, err
	}
	if lease.closed.Load() {
		return iface.NativeVLANMaps{}, os.ErrClosed
	}
	return iface.NativeVLANMaps{Ingress: maps.Clone(actual.IngressQosMap), Egress: maps.Clone(actual.EgressQosMap)}, nil
}
func (lease *nativeVLANLease) replaceDirect(ctx context.Context, wanted iface.NativeVLANMaps) error {
	if err := validateQoSMap(wanted.Ingress); err != nil {
		return err
	}
	if err := validateQoSMap(wanted.Egress); err != nil {
		return err
	}
	if err := lease.operationLocked(ctx); err != nil {
		return err
	}
	actual, err := lease.resource.readLocked(ctx)
	if err != nil {
		return err
	}
	in, out := qosReplaceMaps(actual.EgressQosMap, wanted.Ingress, wanted.Egress)
	modify := &netlink.Vlan{Name: lease.identity.Device, Index: lease.identity.Index, TxQLen: -1, VlanId: actual.VlanId, IngressQosMap: in, EgressQosMap: out}
	if err := lease.resource.owner.beginLocked(ctx); err != nil {
		return err
	}
	if err := lease.resource.owner.ops.modify(modify); err != nil {
		return err
	}
	if lease.closed.Load() {
		return os.ErrClosed
	}
	return nil
}

// A legacy caller cannot bypass an active original participant or write through
// an unrelated backend. No-owner legacy devices retain their existing behavior.
func (resources *nativeVLANResources) legacyTargetLocked(link netlink.Link) error {
	if resources.phase == nativeVLANBackendClosed {
		return os.ErrClosed
	}
	if resources.phase == nativeVLANBackendClosing {
		return iface.ErrNativeAccessPending
	}
	for _, child := range resources.records {
		if child.parent == link.Attrs().Index {
			if child.leases.Load() != 0 {
				return iface.ErrNativeAccessPending
			}
		}
	}
	record := resources.byNameLocked(link.Attrs().Name)
	if record == nil {
		if strings.HasPrefix(link.Attrs().Alias, nativeVLANAliasPrefix) {
			return errors.New("native VLAN belongs to another original backend")
		}
		return nil
	}
	if record.writers.Load() != 0 {
		return iface.ErrNativeAccessPending
	}
	if _, ready := record.state.(nativeVLANReady); !ready {
		return errors.New("native VLAN creation is not confirmed")
	}
	if link.Attrs().Index != record.identity.Index {
		return errors.New("legacy VLAN target names another index")
	}
	birth, err := iface.ParseNativeVLANBirth(link.Attrs().Alias)
	if err != nil {
		return err
	}
	if birth != record.identity.Birth {
		return errors.New("legacy VLAN target names another birth")
	}
	return nil
}
func (resources *nativeVLANResources) deleteLocked(name string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nativeVLANOperationLimit)
	defer cancel()
	return resources.deleteDirect(ctx, name)
}

// nativeVLANLinkAbsent accepts the native transport's original kernel cause
// and the library's initialized missing-link wrapper. Other lookup failures
// cannot authorize retirement or successful deletion.
func nativeVLANLinkAbsent(err error) bool {
	if errors.Is(err, unix.ENODEV) {
		return true
	}
	_, absent := errors.AsType[netlink.LinkNotFoundError](err)
	return absent
}

// closeDirect MUST end a turn when nativeVLANRetirementExpired classifies its
// current operation, even before ctx.Err observes the native deadline. The
// next-resource cursor MUST survive for a later Close with a fresh budget.
func (resources *nativeVLANResources) closeDirect(ctx context.Context) error {
	if resources.ops != nil {
		if resources.ops.resumeOriginal != nil {
			if err := resources.ops.resumeOriginal(ctx); err != nil {
				return err
			}
		}
	}
	if resources.phase == nativeVLANBackendClosed {
		return nil
	}
	for _, record := range resources.records {
		if record.leases.Load() != 0 {
			return iface.ErrNativeAccessPending
		}
	}
	resources.phase = nativeVLANBackendClosing
	// Only our own unleased creation markers are retired. This permits a later
	// owner to manage an ordinary preexisting VLAN without claiming native CoS.
	births := make([]iface.NativeVLANBirth, 0, len(resources.records))
	for birth := range resources.records {
		births = append(births, birth)
	}
	slices.SortFunc(births, func(a, b iface.NativeVLANBirth) int { return a.Compare(b) })
	var failure error
	start := resources.retireNext
	for offset := range births {
		at := (start + offset) % len(births)
		record := resources.records[births[at]]
		if _, retired := record.state.(nativeVLANRetired); retired {
			continue
		}
		if err := ctx.Err(); err != nil {
			failure = errors.Join(failure, err)
			break
		}
		resources.retireNext = (at + 1) % len(births) // A timed-out resource cannot starve its siblings on retry.
		link, err := record.readLocked(ctx)
		if err != nil {
			if nativeVLANRetirementExpired(err) {
				return errors.Join(failure, record.retainedCause(err))
			}
			if nativeVLANLinkAbsent(err) {
				record.state = nativeVLANRetired{}
				continue
			}
			failure = errors.Join(failure, record.retainedCause(err))
			continue
		}
		if link.Attrs().Alias == record.retiredAlias {
			record.state = nativeVLANRetired{}
			continue
		}
		if err := resources.beginLocked(ctx); err != nil {
			failure = errors.Join(failure, err)
			if nativeVLANRetirementExpired(err) {
				return failure
			}
			continue
		}
		record.state = nativeVLANRetiring{errors.New("native VLAN ownership retirement is unconfirmed")}
		if err := resources.ops.alias(link, record.retiredAlias); err != nil {
			record.state = nativeVLANRetiring{err}
			failure = errors.Join(failure, err)
			if nativeVLANRetirementExpired(err) {
				return failure
			}
			continue
		}
		record.state = nativeVLANRetired{}
	}
	if failure != nil {
		return failure
	}
	for _, record := range resources.records {
		if _, retired := record.state.(nativeVLANRetired); !retired {
			return iface.ErrNativeAccessPending
		}
	}
	if resources.ops != nil {
		if err := resources.ops.close(); err != nil {
			return err
		}
	}
	resources.phase = nativeVLANBackendClosed
	return nil
}

// nativeVLANRetirementExpired classifies only the current operation's cause,
// never joined historical failures. closeDirect MUST stop its turn when true,
// preserving the next-resource cursor and every original cause for its retry.
func nativeVLANRetirementExpired(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return errors.Is(err, context.Canceled)
}

func (record *nativeVLANResource) createdEcho(link netlink.Link) error {
	vlan, ok := link.(*netlink.Vlan)
	if !ok {
		return errors.New("original CREATE echo is not a VLAN")
	}
	if vlan == nil {
		return errors.New("original CREATE echo is absent")
	}
	if vlan.Name != record.identity.Device {
		return errors.New("original CREATE echo names another device")
	}
	if vlan.Index <= 0 {
		return errors.New("original CREATE echo has no positive index")
	}
	if vlan.Index > nativeAccessIndexMax {
		return errors.New("original CREATE echo exceeds the native index range")
	}
	if vlan.ParentIndex != record.parent {
		return errors.New("original CREATE echo names another parent")
	}
	if vlan.VlanId != record.vlan {
		return errors.New("original CREATE echo names another VLAN")
	}
	if vlan.Alias != "" {
		return errors.New("original CREATE echo unexpectedly carries an ownership alias")
	}
	if record.identity.Index != 0 {
		if record.identity.Index != vlan.Index {
			return errors.New("original CREATE echo conflicts with its retained index")
		}
	}
	record.identity.Index = vlan.Index
	return nil
}

func (resources *nativeVLANResources) create(spec iface.VLANSpec) error {
	spec.IngressQoSMap = maps.Clone(spec.IngressQoSMap)
	spec.EgressQoSMap = maps.Clone(spec.EgressQoSMap)
	_, err := resources.run(context.Background(), false, func(ctx context.Context) (any, error) { return nil, resources.createDirect(ctx, spec) }, nil)
	return err
}

func (b *netlinkBackend) CaptureNativeVLAN(ctx context.Context, binding iface.Binding) (iface.NativeVLANPreparation, error) {
	value, err := b.nativeVLANs.run(ctx, false, func(ctx context.Context) (any, error) { return b.captureNativeVLANDirect(ctx, binding) }, func(value any) error {
		lease, ok := value.(iface.NativeVLANPreparation)
		if !ok {
			return errors.New("native capture discard returned no preparation")
		}
		return lease.Close()
	})
	if err != nil {
		return nil, err
	}
	lease, ok := value.(iface.NativeVLANPreparation)
	if !ok {
		return nil, errors.New("native capture returned no preparation")
	}
	return lease, nil
}

func (lease *nativeVLANLease) Snapshot(ctx context.Context) (iface.NativeVLANMaps, error) {
	if lease == nil {
		return iface.NativeVLANMaps{}, errors.New("native VLAN map lease is absent")
	}
	if lease.nativeVLANReference == nil {
		return iface.NativeVLANMaps{}, errors.New("native VLAN map lease is uninitialized")
	}
	if lease.resource == nil {
		return iface.NativeVLANMaps{}, errors.New("native VLAN resource is absent")
	}
	value, err := lease.resource.owner.run(ctx, false, func(ctx context.Context) (any, error) { return lease.snapshotDirect(ctx) }, nil)
	if err != nil {
		return iface.NativeVLANMaps{}, err
	}
	snapshot, ok := value.(iface.NativeVLANMaps)
	if !ok {
		return iface.NativeVLANMaps{}, errors.New("native snapshot returned another value kind")
	}
	return snapshot, nil
}

func (lease *nativeVLANLease) Replace(ctx context.Context, wanted iface.NativeVLANMaps) error {
	if lease == nil {
		return errors.New("native VLAN map lease is absent")
	}
	if lease.nativeVLANReference == nil {
		return errors.New("native VLAN map lease is uninitialized")
	}
	if lease.resource == nil {
		return errors.New("native VLAN resource is absent")
	}
	wanted.Ingress = maps.Clone(wanted.Ingress)
	wanted.Egress = maps.Clone(wanted.Egress)
	_, err := lease.resource.owner.run(ctx, false, func(ctx context.Context) (any, error) { return nil, lease.replaceDirect(ctx, wanted) }, nil)
	return err
}

func (resources *nativeVLANResources) close() error {
	return resources.closeWithContext(context.Background())
}

func (resources *nativeVLANResources) deleteDirect(ctx context.Context, name string) (bool, error) {
	if resources.phase == nativeVLANBackendClosed {
		return true, os.ErrClosed
	}
	if err := resources.beginLocked(ctx); err != nil {
		return true, err
	}
	record := resources.byNameLocked(name)
	var link netlink.Link
	var err error
	if record != nil {
		link, err = record.readLocked(ctx)
	} else {
		link, err = resources.ops.byName(name)
	}
	if err != nil {
		if nativeVLANLinkAbsent(err) {
			if record != nil {
				record.state = nativeVLANRetired{}
				delete(resources.records, record.identity.Birth)
			}
			return true, nil
		}
		return true, err
	}
	return true, resources.deleteLinkLocked(ctx, link)
}

// rollbackUnpublishedOriginal MUST run in the SAME uninterrupted creator fence
// after the exact CREATE echo and success ACK. Caller expiry permits cleanup,
// never a delayed ownership stamp or admission. No name/index lookup supplies
// this authority. A failed cleanup retains the original resource and socket.
func (record *nativeVLANResource) rollbackUnpublishedOriginal(link netlink.Link, cause error) error {
	original := record.creation
	if original == nil {
		return record.retainedCause(errors.Join(cause, errors.New("unpublished CREATE lost its original transaction")))
	}
	if original.creator != record {
		return record.retainedCause(errors.Join(cause, errors.New("unpublished CREATE belongs to another original resource")))
	}
	if !original.nativeSuccess() {
		return record.retainedCause(errors.Join(cause, errors.New("unpublished CREATE has no original success ACK")))
	}
	if link == nil {
		return record.retainedCause(errors.Join(cause, errors.New("unpublished CREATE has no original echo")))
	}
	if len(original.links) != 1 {
		return record.retainedCause(errors.Join(cause, errors.New("unpublished CREATE has no unique original echo")))
	}
	if original.links[0] != link {
		return record.retainedCause(errors.Join(cause, errors.New("unpublished rollback substituted its original echo")))
	}
	if err := record.createdEcho(link); err != nil {
		return record.retainedCause(errors.Join(cause, err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), nativeVLANOperationLimit)
	defer cancel()
	if err := record.owner.beginLocked(ctx); err != nil {
		record.state = nativeVLANUnmarked{errors.Join(cause, err)}
		return errors.Join(cause, err)
	}
	if err := record.owner.deleteLinkOriginalLocked(ctx, link, record); err != nil {
		record.state = nativeVLANUnmarked{errors.Join(cause, err)}
		return errors.Join(cause, err)
	}
	record.state = nativeVLANRetired{}
	delete(record.owner.records, record.identity.Birth)
	return cause
}

// initializeOps MUST publish on the caller before a worker starts. Later worker
// calls reuse that transport. Synchronous rollback shares this publication point.
// opsMu is acquired after workerMu or qosMapMu and never acquires either of them.
// A failed factory retains no transport; its own partial-socket cleanup remains
// authoritative and the next caller may retry. A published transport never changes.
func (resources *nativeVLANResources) initializeOps() error {
	if !resources.opsMu.TryLock() {
		if resources.observeOpsContentionForTest != nil {
			resources.observeOpsContentionForTest()
		}
		resources.opsMu.Lock()
	}
	defer resources.opsMu.Unlock()
	if resources.ops != nil {
		return nil
	}
	open := newNativeVLANOps
	if resources.openOpsForTest != nil {
		open = resources.openOpsForTest
	}
	ops, err := open()
	if err != nil {
		return err
	}
	resources.ops = ops
	return nil
}

// closeWithContext retains the original worker until its actual done channel
// closes. stopped ends admission, but request.finish can still be returning an
// abandoned result; neither state alone proves the worker has been joined.
func (resources *nativeVLANResources) closeWithContext(ctx context.Context) error {
	bounded, cancel, err := nativeVLANContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	resources.workerMu.Lock()
	worker := resources.worker
	if worker != nil {
		if worker.stopped {
			resources.workerMu.Unlock()
			return resources.join(bounded)
		}
	}
	if worker == nil {
		if !qosMapMu.TryLock() {
			resources.workerMu.Unlock()
			return iface.ErrNativeAccessPending
		}
		if resources.ops == nil {
			if len(resources.records) == 0 {
				resources.phase = nativeVLANBackendClosed
				worker = &nativeVLANWorker{owner: resources, done: make(chan struct{}), stopped: true}
				close(worker.done)
				resources.worker = worker
				qosMapMu.Unlock()
				resources.workerMu.Unlock()
				return nil
			}
		}
		qosMapMu.Unlock()
	}
	resources.workerMu.Unlock()
	_, err = resources.run(bounded, true, func(turn context.Context) (any, error) {
		return nil, resources.closeDirect(turn)
	}, nil)
	if !errors.Is(err, os.ErrClosed) {
		return err
	}
	// A successful stop may race the first observation above. Only an actually
	// stopped original worker may turn run's admission refusal into a join;
	// an os.ErrClosed from a failed native close remains its original error.
	resources.workerMu.Lock()
	worker = resources.worker
	if worker == nil {
		resources.workerMu.Unlock()
		return err
	}
	stopped := worker.stopped
	resources.workerMu.Unlock()
	if !stopped {
		return err
	}
	return resources.join(bounded)
}
