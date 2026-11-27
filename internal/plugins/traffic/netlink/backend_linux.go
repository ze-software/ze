// Design: docs/architecture/core-design.md -- kernel traffic control backend Linux implementation
// Related: ops_linux.go -- tcOps seam for kernel traffic control operations
// Related: snapshot_linux.go -- snapshot persistence and identity validation

//go:build linux

package trafficnetlink

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"sync"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/internal/core/statestore"
)

var errNoRootQdiscFound = errors.New("no root qdisc found")

// errSnapshotPersistUnavailable gates the destructive qdisc replace: when no state
// store is registered, saveTCSnapshots silently no-ops, so replacing the operator's
// root qdisc would strand the original unrestorable across a crash/restart. The
// backend refuses to snapshot-and-replace in that state, exactly as an unresolved
// config directory did before persistence moved into the shared zefs store.
var errSnapshotPersistUnavailable = errors.New("traffic control snapshot persistence unavailable: no state store registered")

// resolveOSName translates a logical interface name to its kernel device via the
// shared iface resolver, so traffic control operations honor the os-name / mac-match
// selectors. Best-effort: if no backend is loaded or the device is absent, the
// name passes through unchanged (identical to pre-resolver behavior). Resolving
// at the backend's logic methods -- not inside the tcOps kernel adapter -- keeps
// the snapshot map and the kernel link keyed consistently on the os device name
// (ensureSnapshot keys by link.Attrs().Name), so RestoreOriginal finds the
// snapshot Apply stored even when the logical name differs from the os device.
func resolveOSName(name string) string {
	if b, err := iface.Resolve(name); err == nil && b.OsName != "" {
		return b.OsName
	}
	return name
}

// classMeta holds the Ze-configured class names and default class name for one
// OS interface so live netlink readback can label kernel handles with operator
// names.
type classMeta struct {
	names        map[uint32]string
	defaultClass string
}

// readOnlyReader implements traffic.QdiscReader directly against netlink
// without mutating state or touching snapshot persistence. Safe for concurrent
// use.
type readOnlyReader struct {
	ops tcOps
}

// backend implements traffic.Backend and traffic.QdiscReader using
// vishvananda/netlink traffic control API. Safe for concurrent use.
type backend struct {
	mu               sync.Mutex
	ops              tcOps
	snapshotReadyErr error
	bootID           string
	snapshots        map[string]tcInterfaceSnapshot
	classNames       map[string]map[uint32]string // osName -> handle -> Ze class name
	defaultClasses   map[string]string            // osName -> Ze default class name
}

var (
	_ traffic.QdiscReader = (*backend)(nil)
	_ traffic.QdiscReader = readOnlyReader{}
)

func registerReadOnlyReader() {
	if err := traffic.RegisterReader(backendName, readOnlyReader{ops: netlinkOps{}}); err != nil {
		panic("BUG: trafficnetlink: reader registration failed: " + err.Error())
	}
}

func newBackend() (traffic.Backend, error) {
	// Only a failed boot-id read gates snapshot readiness at construction, since
	// without it snapshot identity cannot be validated. Persistence availability is
	// not captured here: it is re-checked at snapshot time in ensureSnapshot against
	// the process-wide statestore, so a store registered after construction is
	// honored and a missing store blocks the destructive qdisc replace.
	bootID, bootErr := currentBootID()
	snapshots, loadErr := loadTCSnapshots()
	if loadErr != nil {
		return nil, loadErr
	}
	return newBackendWithOps(netlinkOps{}, bootErr, bootID, snapshots), nil
}

func newBackendWithOps(ops tcOps, snapshotReadyErr error, bootID string, snapshots map[string]tcInterfaceSnapshot) *backend {
	if snapshots == nil {
		snapshots = map[string]tcInterfaceSnapshot{}
	}
	return &backend{
		ops:              ops,
		snapshotReadyErr: snapshotReadyErr,
		bootID:           bootID,
		snapshots:        snapshots,
		classNames:       map[string]map[uint32]string{},
		defaultClasses:   map[string]string{},
	}
}

// Apply programs traffic control configuration on each named interface: snapshot the original
// root qdisc, replace it, then rebuild classes and filters. Removing ownership
// is explicit via RestoreOriginal or Close so callers that manage one dynamic
// interface do not accidentally remove other traffic-control-owned interfaces.
//
// ctx is accepted for interface parity with other backends but is NOT honored:
// vishvananda/netlink's traffic control operations are synchronous CGO-free syscalls with no
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

	if err := verifyDesired(desired); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, name := range sortedDesiredNames(desired) {
		qos := desired[name]
		link, err := b.ops.linkByName(resolveOSName(name))
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
	if err := b.replaceRootQdisc(link, rootQdisc); err != nil {
		return fmt.Errorf("qdisc replace: %w", err)
	}

	// Replacing the root qdisc drops every prior class in the kernel; clear
	// cached class metadata now so a partial class-install failure never
	// leaves stale Ze names behind.
	osName := link.Attrs().Name
	delete(b.classNames, osName)
	delete(b.defaultClasses, osName)

	// Add classes under root qdisc (for classful qdiscs like HTB/HFSC).
	rootHandle := rootQdisc.Attrs().Handle
	names := make(map[uint32]string, len(qos.Qdisc.Classes))
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
		names[classHandle] = tc.Name
		for _, f := range tc.Filters {
			filters, err := translateFilter(f, linkIdx, rootHandle, classHandle)
			if err != nil {
				return fmt.Errorf("class %q filter: %w", tc.Name, err)
			}
			for _, filter := range filters {
				if err := b.ops.filterAdd(filter); err != nil {
					return fmt.Errorf("class %q filter add: %w", tc.Name, err)
				}
			}
		}
	}

	if len(names) > 0 {
		b.classNames[osName] = names
		if qos.Qdisc.DefaultClass != "" {
			b.defaultClasses[osName] = qos.Qdisc.DefaultClass
		}
	}

	// The root qdisc above shapes egress, which on a subscriber interface is
	// the download direction. The upload direction arrives on the ingress hook,
	// where a policer is the enforcement available (policer_linux.go).
	return b.applyIngressPolicer(link, qos.Ingress)
}

// replaceRootQdisc rebuilds a root even when its handle is already installed.
// Linux qdisc_change (net/sched/sch_api.c) treats a same-handle replacement as
// an in-place change. HTB has no change callback and returns EINVAL; other
// schedulers can retain stale classes and filters. Delete that root first.
// Both callers retain the durable original snapshot until restoration succeeds.
func (b *backend) replaceRootQdisc(link netlink.Link, desired netlink.Qdisc) error {
	qdiscs, err := b.ops.qdiscList(link)
	if err != nil {
		return fmt.Errorf("list qdiscs before replacement: %w", err)
	}
	for _, current := range qdiscs {
		attrs := current.Attrs()
		if desired.Attrs().Handle != 0 && attrs.Parent == netlink.HANDLE_ROOT && attrs.Handle == desired.Attrs().Handle {
			if err := b.ops.qdiscDel(current); err != nil {
				return fmt.Errorf("delete same-handle root qdisc: %w", err)
			}
			break
		}
	}
	return b.ops.qdiscReplace(desired)
}

func (b *backend) ensureSnapshot(link netlink.Link) error {
	name := link.Attrs().Name
	if snap, ok := b.snapshots[name]; ok {
		return snap.validateLink(link, b.bootID)
	}
	if b.snapshotReadyErr != nil {
		return b.snapshotReadyErr
	}
	// Persistence interlock: a NEW snapshot is only durable if a state store is
	// registered. With none, saveSnapshots below no-ops, so replacing the root
	// qdisc would leave the original unrestorable after a restart. Bail BEFORE the
	// caller's qdiscReplace rather than destroy state we cannot persist.
	if statestore.Store() == nil {
		return errSnapshotPersistUnavailable
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
		return fmt.Errorf("qdisc %q has %d class(es); backend kernel cannot snapshot class state exactly", root.Type(), len(classes))
	}
	filters, err := b.ops.filterList(link, root.Attrs().Handle)
	if err != nil {
		return fmt.Errorf("list filters for qdisc %q: %w", root.Type(), err)
	}
	if len(filters) > 0 {
		return fmt.Errorf("qdisc %q has %d filter(s); backend kernel cannot snapshot filter state exactly", root.Type(), len(filters))
	}
	return nil
}

func rootQdisc(qdiscs []netlink.Qdisc) (netlink.Qdisc, error) {
	for _, qdisc := range qdiscs {
		if qdisc.Attrs().Parent == netlink.HANDLE_ROOT {
			return qdisc, nil
		}
	}
	return nil, errNoRootQdiscFound
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
	// Resolve to the os device so the snapshot lookup (Apply stores snapshots
	// keyed by os via ensureSnapshot's link.Attrs().Name) and the kernel link
	// agree even when the caller passes a logical name. Idempotent when
	// ifaceName is already an os device. If the device is gone, resolution
	// fails and falls back to the logical name; the os-keyed snapshot is then
	// not found and restore is a correct no-op -- a removed device has no traffic control
	// state to restore, and ensureSnapshot re-validates (and replaces on a
	// bootID change) any stale snapshot when the device returns.
	ifaceName = resolveOSName(ifaceName)
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
	// Clear the ingress policer before the root qdisc goes back. A policer left
	// on the hook rate-limits whoever gets this interface next, and a pppN
	// number is reused as soon as the subscriber count drops.
	if err := b.removeIngressPolicer(link); err != nil {
		return fmt.Errorf("trafficnetlink: interface %q: %w", ifaceName, err)
	}
	if snap.Qdisc.restoredByDelete() {
		// The original root was noqueue: the interface had no queueing discipline.
		// Re-enter that state by deleting the root ze installed, which is what
		// `tc qdisc del dev X root` does; the kernel then reports noqueue again. There
		// is no qdisc object to replace with, and adding one named noqueue is not
		// the inverse operation.
		root := &netlink.GenericQdisc{
			QdiscAttrs: snap.Qdisc.Attrs.toNetlink(link.Attrs().Index),
			QdiscType:  snap.Qdisc.Type,
		}
		if err := b.ops.qdiscDel(root); err != nil {
			return fmt.Errorf("trafficnetlink: interface %q: restore original %q by deleting the root qdisc: %w", ifaceName, snap.Qdisc.Type, err)
		}
	} else {
		qdisc, err := snap.Qdisc.toNetlink(link.Attrs().Index)
		if err != nil {
			return fmt.Errorf("trafficnetlink: interface %q: %w", ifaceName, err)
		}
		if err := b.replaceRootQdisc(link, qdisc); err != nil {
			return fmt.Errorf("trafficnetlink: interface %q: restore qdisc %q: %w", ifaceName, qdisc.Type(), err)
		}
	}
	delete(b.snapshots, ifaceName)
	if err := b.saveSnapshots(); err != nil {
		b.snapshots[ifaceName] = snap
		return err
	}
	delete(b.classNames, ifaceName)
	delete(b.defaultClasses, ifaceName)
	return nil
}

func (b *backend) saveSnapshots() error {
	return saveTCSnapshots(b.snapshots)
}

func (b *backend) snapshotMetaFor(osName string) classMeta {
	b.mu.Lock()
	defer b.mu.Unlock()
	return classMeta{
		names:        maps.Clone(b.classNames[osName]),
		defaultClass: b.defaultClasses[osName],
	}
}

func (b *backend) snapshotAllMeta() map[string]classMeta {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.classNames) == 0 && len(b.defaultClasses) == 0 {
		return nil
	}
	out := make(map[string]classMeta, max(len(b.classNames), len(b.defaultClasses)))
	for osName, names := range b.classNames {
		out[osName] = classMeta{
			names:        maps.Clone(names),
			defaultClass: b.defaultClasses[osName],
		}
	}
	for osName, def := range b.defaultClasses {
		if _, ok := out[osName]; !ok {
			out[osName] = classMeta{defaultClass: def}
		}
	}
	return out
}

// ListQdiscs returns current traffic control state and live statistics for an
// interface.
func (b *backend) ListQdiscs(ifaceName string) (traffic.InterfaceQoS, error) {
	osName := resolveOSName(ifaceName)
	return readQdiscForInterface(b.ops, ifaceName, osName, b.snapshotMetaFor(osName))
}

// ListAllQdiscs returns current traffic control state and live statistics for
// all kernel links using one bulk qdisc dump.
func (b *backend) ListAllQdiscs() ([]traffic.InterfaceQoS, error) {
	return readAllQdiscs(b.ops, b.snapshotAllMeta())
}

// ListQdiscs returns current traffic control state and live statistics for an
// interface without Ze class-name metadata.
func (r readOnlyReader) ListQdiscs(ifaceName string) (traffic.InterfaceQoS, error) {
	osName := resolveOSName(ifaceName)
	return readQdiscForInterface(r.ops, ifaceName, osName, classMeta{})
}

// ListAllQdiscs returns current traffic control state and live statistics for
// all kernel links without Ze class-name metadata.
func (r readOnlyReader) ListAllQdiscs() ([]traffic.InterfaceQoS, error) {
	return readAllQdiscs(r.ops, nil)
}

func readQdiscForInterface(ops tcOps, ifaceName, osName string, meta classMeta) (traffic.InterfaceQoS, error) {
	link, err := ops.linkByName(osName)
	if err != nil {
		return traffic.InterfaceQoS{}, fmt.Errorf("trafficnetlink: interface %q: %w", ifaceName, err)
	}

	qdiscs, err := ops.qdiscList(link)
	if err != nil {
		return traffic.InterfaceQoS{}, fmt.Errorf("trafficnetlink: list qdiscs: %w", err)
	}

	var root netlink.Qdisc
	for _, q := range qdiscs {
		if q.Attrs().Parent == netlink.HANDLE_ROOT {
			root = q
			break
		}
	}

	qos, err := buildInterfaceQoS(ops, link, ifaceName, root, meta.names, meta.defaultClass)
	if err != nil {
		return traffic.InterfaceQoS{}, fmt.Errorf("trafficnetlink: interface %q: %w", ifaceName, err)
	}
	return qos, nil
}

func readAllQdiscs(ops tcOps, allMeta map[string]classMeta) ([]traffic.InterfaceQoS, error) {
	links, err := ops.linkList()
	if err != nil {
		return nil, fmt.Errorf("trafficnetlink: list links: %w", err)
	}
	qdiscs, err := ops.qdiscList(nil)
	if err != nil {
		return nil, fmt.Errorf("trafficnetlink: list qdiscs: %w", err)
	}

	rootsByIndex := make(map[int]netlink.Qdisc, len(links))
	for _, q := range qdiscs {
		attrs := q.Attrs()
		if attrs.Parent == netlink.HANDLE_ROOT {
			rootsByIndex[attrs.LinkIndex] = q
		}
	}

	slices.SortFunc(links, func(a, b netlink.Link) int {
		return cmp.Compare(a.Attrs().Name, b.Attrs().Name)
	})

	result := make([]traffic.InterfaceQoS, 0, len(links))
	for _, link := range links {
		attrs := link.Attrs()
		meta := allMeta[attrs.Name]
		qos, err := buildInterfaceQoS(ops, link, attrs.Name, rootsByIndex[attrs.Index], meta.names, meta.defaultClass)
		if err != nil {
			if isLinkVanishedErr(err) {
				continue
			}
			return nil, fmt.Errorf("trafficnetlink: interface %q: %w", attrs.Name, err)
		}
		result = append(result, qos)
	}
	return result, nil
}

func isLinkVanishedErr(err error) bool {
	return errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENODEV)
}

// extractQdiscStats converts kernel tc netlink statistics to traffic.QdiscStats.
// Returns nil when netlink provided no statistics, keeping absent telemetry
// distinct from real zero counters.
func extractQdiscStats(s *netlink.ClassStatistics) *traffic.QdiscStats {
	if s == nil {
		return nil
	}
	if s.Basic == nil && s.Queue == nil {
		return nil
	}
	stats := &traffic.QdiscStats{}
	if s.Basic != nil {
		stats.Bytes = s.Basic.Bytes
		stats.Packets = uint64(s.Basic.Packets)
	}
	if s.Queue != nil {
		stats.Qlen = s.Queue.Qlen
		stats.Backlog = s.Queue.Backlog
		stats.Drops = s.Queue.Drops
		stats.Requeues = s.Queue.Requeues
		stats.Overlimits = s.Queue.Overlimits
	}
	return stats
}

func buildInterfaceQoS(
	ops tcOps,
	link netlink.Link,
	ifaceName string,
	root netlink.Qdisc,
	classNames map[uint32]string,
	defaultClass string,
) (traffic.InterfaceQoS, error) {
	qos := traffic.InterfaceQoS{Interface: ifaceName}
	if root == nil {
		return qos, nil
	}

	rootAttrs := root.Attrs()
	qos.Qdisc.Kind = root.Type()
	qos.Qdisc.Type = raiseQdiscType(root)
	qos.Qdisc.Handle = formatTCHandle(rootAttrs.Handle)
	qos.Qdisc.Stats = extractQdiscStats((*netlink.ClassStatistics)(rootAttrs.Statistics))

	if tbf, ok := root.(*netlink.Tbf); ok {
		if tbf.Rate > math.MaxUint64/8 {
			return traffic.InterfaceQoS{}, fmt.Errorf("tbf readback rate %d bytes/s overflows bits/s", tbf.Rate)
		}
		qos.Qdisc.RateBps = tbf.Rate * 8
	}

	if !isClasslessQdisc(root) {
		classes, err := readClassesForRoot(ops, link, rootAttrs.Handle, classNames)
		if err != nil {
			return traffic.InterfaceQoS{}, err
		}
		qos.Qdisc.Classes = classes
	}

	qos.Qdisc.DefaultClass = resolveDefaultClass(root, classNames, defaultClass)
	return qos, nil
}

func readClassesForRoot(
	ops tcOps,
	link netlink.Link,
	rootHandle uint32,
	classNames map[uint32]string,
) ([]traffic.TrafficClass, error) {
	kernelClasses, err := ops.classList(link, rootHandle)
	if err != nil {
		return nil, fmt.Errorf("list classes: %w", err)
	}
	if len(kernelClasses) == 0 {
		return []traffic.TrafficClass{}, nil
	}
	slices.SortFunc(kernelClasses, func(a, b netlink.Class) int {
		return cmp.Compare(a.Attrs().Handle, b.Attrs().Handle)
	})

	classes := make([]traffic.TrafficClass, 0, len(kernelClasses))
	for _, cls := range kernelClasses {
		attrs := cls.Attrs()
		handle := formatTCHandle(attrs.Handle)
		name := classNames[attrs.Handle]
		if name == "" {
			name = handle
		}
		tc := traffic.TrafficClass{
			Name:   name,
			Handle: handle,
			Stats:  extractQdiscStats(attrs.Statistics),
		}
		if htb, ok := cls.(*netlink.HtbClass); ok {
			if htb.Rate > math.MaxUint64/8 {
				return nil, fmt.Errorf("htb class %s readback rate %d bytes/s overflows bits/s", handle, htb.Rate)
			}
			if htb.Ceil > math.MaxUint64/8 {
				return nil, fmt.Errorf("htb class %s readback ceil %d bytes/s overflows bits/s", handle, htb.Ceil)
			}
			tc.Rate = htb.Rate * 8
			tc.Ceil = htb.Ceil * 8
			tc.Priority = uint8(min(htb.Prio, math.MaxUint8))
		}
		classes = append(classes, tc)
	}
	return classes, nil
}

func resolveDefaultClass(root netlink.Qdisc, classNames map[uint32]string, defaultClass string) string {
	if defaultClass != "" {
		return defaultClass
	}
	var defMinor uint32
	switch q := root.(type) {
	case *netlink.Htb:
		defMinor = q.Defcls
	case *netlink.Hfsc:
		defMinor = uint32(q.Defcls)
	default:
		return ""
	}
	if defMinor == 0 {
		return ""
	}
	defHandle := tcHandle(defMinor)
	if major := root.Attrs().Handle & 0xFFFF0000; major != 0 {
		defHandle = major | (defMinor & 0xFFFF)
	}
	if name := classNames[defHandle]; name != "" {
		return name
	}
	return formatTCHandle(defHandle)
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
	clear(b.classNames)
	clear(b.defaultClasses)
	return errors.Join(errs...)
}

func sortedDesiredNames(desired map[string]traffic.InterfaceQoS) []string {
	names := make([]string, 0, len(desired))
	for name := range desired {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func sortedSnapshotNames(snapshots map[string]tcInterfaceSnapshot) []string {
	names := make([]string, 0, len(snapshots))
	for name := range snapshots {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
