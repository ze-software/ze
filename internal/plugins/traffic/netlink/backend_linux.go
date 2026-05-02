// Design: docs/architecture/core-design.md -- tc backend Linux implementation
// Related: ops_linux.go -- tcOps seam for kernel tc operations
// Related: snapshot_linux.go -- snapshot persistence and identity validation

//go:build linux

package trafficnetlink

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

// backend implements traffic.Backend using vishvananda/netlink tc API.
type backend struct {
	mu               sync.Mutex
	ops              tcOps
	snapshotPath     string
	snapshotReadyErr error
	bootID           string
	snapshots        map[string]tcInterfaceSnapshot
}

func newBackend() (traffic.Backend, error) {
	path, pathErr := defaultSnapshotPath()
	bootID, bootErr := currentBootID()
	snapshots, loadErr := loadTCSnapshots(path)
	if loadErr != nil {
		return nil, loadErr
	}
	return newBackendWithOps(netlinkOps{}, path, errors.Join(pathErr, bootErr), bootID, snapshots), nil
}

func newBackendWithOps(ops tcOps, snapshotPath string, snapshotReadyErr error, bootID string, snapshots map[string]tcInterfaceSnapshot) *backend {
	if snapshots == nil {
		snapshots = map[string]tcInterfaceSnapshot{}
	}
	return &backend{ops: ops, snapshotPath: snapshotPath, snapshotReadyErr: snapshotReadyErr, bootID: bootID, snapshots: snapshots}
}

// Apply programs tc configuration on each named interface: snapshot the original
// root qdisc, replace it, then rebuild classes and filters. Removing ownership
// is explicit via RestoreOriginal or Close so callers that manage one dynamic
// interface do not accidentally remove other tc-owned interfaces.
//
// ctx is accepted for interface parity with other backends but is NOT honored:
// vishvananda/netlink's tc operations are synchronous CGO-free syscalls with no
// ctx-aware variants. A SIGTERM mid-Apply will wait for the in-flight syscall
// to return (all syscalls here are fast in practice). Do not drop the parameter
// -- the interface would regress and tests rely on the signature.
func (b *backend) Apply(ctx context.Context, desired map[string]traffic.InterfaceQoS) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, name := range sortedDesiredNames(desired) {
		qos := desired[name]
		link, err := b.ops.linkByName(name)
		if err != nil {
			return fmt.Errorf("trafficnetlink: interface %q: %w", name, err)
		}
		if err := b.ensureSnapshot(link); err != nil {
			return fmt.Errorf("trafficnetlink: interface %q: snapshot original qdisc: %w", name, err)
		}
		if err := b.applyInterface(link, &qos); err != nil {
			return fmt.Errorf("trafficnetlink: interface %q: %w", name, err)
		}
	}
	return nil
}

func (b *backend) applyInterface(link netlink.Link, qos *traffic.InterfaceQoS) error {
	linkIdx := link.Attrs().Index

	// Replace root qdisc.
	rootQdisc, err := translateQdisc(qos.Qdisc, linkIdx)
	if err != nil {
		return fmt.Errorf("translate qdisc: %w", err)
	}
	if err := b.ops.qdiscReplace(rootQdisc); err != nil {
		return fmt.Errorf("qdisc replace: %w", err)
	}

	// Add classes under root qdisc (for classful qdiscs like HTB/HFSC).
	rootHandle := rootQdisc.Attrs().Handle
	for i, tc := range qos.Qdisc.Classes {
		class, err := translateClass(qos.Qdisc.Type, tc, linkIdx, rootHandle, uint32(i+1))
		if err != nil {
			return fmt.Errorf("class %q: translate: %w", tc.Name, err)
		}
		if err := b.ops.classAdd(class); err != nil {
			return fmt.Errorf("class %q: add: %w", tc.Name, err)
		}

		// Add filters for this class.
		classHandle := class.Attrs().Handle
		for _, f := range tc.Filters {
			filter, err := translateFilter(f, linkIdx, rootHandle, classHandle)
			if err != nil {
				return fmt.Errorf("class %q filter: %w", tc.Name, err)
			}
			if err := b.ops.filterAdd(filter); err != nil {
				return fmt.Errorf("class %q filter add: %w", tc.Name, err)
			}
		}
	}

	return nil
}

func (b *backend) ensureSnapshot(link netlink.Link) error {
	name := link.Attrs().Name
	if snap, ok := b.snapshots[name]; ok {
		return snap.validateLink(link, b.bootID)
	}
	if b.snapshotReadyErr != nil {
		return b.snapshotReadyErr
	}
	qdiscs, err := b.ops.qdiscList(link)
	if err != nil {
		return fmt.Errorf("list qdiscs: %w", err)
	}
	root, err := rootQdisc(qdiscs)
	if err != nil {
		return err
	}
	if err := b.requireRestorableRoot(link, root); err != nil {
		return err
	}
	snap, err := newInterfaceSnapshot(link, b.bootID, root)
	if err != nil {
		return err
	}
	b.snapshots[name] = snap
	if err := b.saveSnapshots(); err != nil {
		delete(b.snapshots, name)
		return err
	}
	return nil
}

func (b *backend) requireRestorableRoot(link netlink.Link, root netlink.Qdisc) error {
	classes, err := b.ops.classList(link, root.Attrs().Handle)
	if err != nil {
		return fmt.Errorf("list classes for qdisc %q: %w", root.Type(), err)
	}
	if len(classes) > 0 {
		return fmt.Errorf("qdisc %q has %d class(es); backend tc cannot snapshot class state exactly", root.Type(), len(classes))
	}
	filters, err := b.ops.filterList(link, root.Attrs().Handle)
	if err != nil {
		return fmt.Errorf("list filters for qdisc %q: %w", root.Type(), err)
	}
	if len(filters) > 0 {
		return fmt.Errorf("qdisc %q has %d filter(s); backend tc cannot snapshot filter state exactly", root.Type(), len(filters))
	}
	return nil
}

func rootQdisc(qdiscs []netlink.Qdisc) (netlink.Qdisc, error) {
	for _, qdisc := range qdiscs {
		if qdisc.Attrs().Parent == netlink.HANDLE_ROOT {
			return qdisc, nil
		}
	}
	return nil, fmt.Errorf("no root qdisc found")
}

// RestoreOriginal restores and drops the pre-Ze qdisc snapshot for ifaceName.
func (b *backend) RestoreOriginal(ctx context.Context, ifaceName string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.restoreOriginalLocked(ifaceName)
}

func (b *backend) restoreOriginalLocked(ifaceName string) error {
	snap, ok := b.snapshots[ifaceName]
	if !ok {
		return nil
	}
	if b.snapshotReadyErr != nil {
		return b.snapshotReadyErr
	}
	link, err := b.ops.linkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("trafficnetlink: interface %q: %w", ifaceName, err)
	}
	if err := snap.validateLink(link, b.bootID); err != nil {
		return fmt.Errorf("trafficnetlink: interface %q: %w", ifaceName, err)
	}
	qdisc, err := snap.Qdisc.toNetlink(link.Attrs().Index)
	if err != nil {
		return fmt.Errorf("trafficnetlink: interface %q: %w", ifaceName, err)
	}
	if err := b.ops.qdiscReplace(qdisc); err != nil {
		return fmt.Errorf("trafficnetlink: interface %q: restore qdisc %q: %w", ifaceName, qdisc.Type(), err)
	}
	delete(b.snapshots, ifaceName)
	if err := b.saveSnapshots(); err != nil {
		b.snapshots[ifaceName] = snap
		return err
	}
	return nil
}

func (b *backend) saveSnapshots() error {
	return saveTCSnapshots(b.snapshotPath, b.snapshots)
}

// ListQdiscs returns current tc state for an interface.
func (b *backend) ListQdiscs(ifaceName string) (traffic.InterfaceQoS, error) {
	link, err := b.ops.linkByName(ifaceName)
	if err != nil {
		return traffic.InterfaceQoS{}, fmt.Errorf("trafficnetlink: interface %q: %w", ifaceName, err)
	}

	qdiscs, err := b.ops.qdiscList(link)
	if err != nil {
		return traffic.InterfaceQoS{}, fmt.Errorf("trafficnetlink: list qdiscs: %w", err)
	}

	qos := traffic.InterfaceQoS{Interface: ifaceName}
	if root, err := rootQdisc(qdiscs); err == nil {
		qos.Qdisc.Type = raiseQdiscType(root)
	}

	return qos, nil
}

// Close restores qdiscs still owned by this backend instance. Snapshots that
// fail to restore (interface gone at shutdown) are dropped from disk so stale
// entries do not block the next startup.
func (b *backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	var errs []error
	for _, name := range sortedSnapshotNames(b.snapshots) {
		if err := b.restoreOriginalLocked(name); err != nil {
			errs = append(errs, err)
		}
	}
	if len(b.snapshots) > 0 {
		b.snapshots = map[string]tcInterfaceSnapshot{}
		if saveErr := b.saveSnapshots(); saveErr != nil {
			errs = append(errs, saveErr)
		}
	}
	return errors.Join(errs...)
}

func sortedDesiredNames(desired map[string]traffic.InterfaceQoS) []string {
	names := make([]string, 0, len(desired))
	for name := range desired {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedSnapshotNames(snapshots map[string]tcInterfaceSnapshot) []string {
	names := make([]string, 0, len(snapshots))
	for name := range snapshots {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
