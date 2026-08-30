// Design: docs/features/interfaces.md -- Traffic mirroring via tc mirred
// Overview: ifacenetlink.go -- package hub

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

var errIfaceMirrorAtLeastOneOf = errors.New("iface: mirror: at least one of ingress or egress must be true")

// The qdisc at handle ffff: is a SHARED attachment point. Ze installs the
// mirror's mirred filters on it at priority 1, and flow-export sampling
// installs its own filter at priority 100 (SampleFilterPriority in
// internal/plugins/flowexport/sampling). Both hooks, ingress and egress, hang
// off that one qdisc object, so deleting the qdisc deletes every filter of
// every subsystem. Mirror teardown therefore removes its own filters and leaves
// the qdisc: a teardown cannot know who created it, and the set of attached
// filters can change between reading it and acting on it. Only a rollback
// deletes, because it created the qdisc moments earlier and knows so.
const (
	// mirrorFilterPriority is the tc filter priority the mirror owns.
	mirrorFilterPriority uint16 = 1
	// mirrorQdiscHandleMajor is the major number of the ingress-side qdisc
	// handle, ffff:0.
	mirrorQdiscHandleMajor uint16 = 0xffff
)

// isNotFound reports whether an error means "there was nothing to delete", so
// the desired state is already reached. The kernel answers a FilterDel that
// matches nothing with one of exactly two errnos, measured against Linux 6.12
// in the QEMU VM: EINVAL when the link carries no qdisc at handle ffff: at all,
// and ENOENT when the qdisc is there but the hook holds no filter at that
// priority. Both are tolerated and nothing else is: this is the only error gate
// on the teardown path, so a wider one lets a real failure report success.
func isNotFound(err error) bool {
	return err != nil && (errors.Is(err, unix.ENOENT) || errors.Is(err, unix.EINVAL))
}

func (b *netlinkBackend) SetupMirror(srcIface, dstIface string, ingress, egress bool) error {
	if err := iface.ValidateIfaceName(srcIface); err != nil {
		return fmt.Errorf("iface: mirror: src: %w", err)
	}
	if err := iface.ValidateIfaceName(dstIface); err != nil {
		return fmt.Errorf("iface: mirror: dst: %w", err)
	}
	if !ingress && !egress {
		return errIfaceMirrorAtLeastOneOf
	}

	src, err := netlink.LinkByName(srcIface)
	if err != nil {
		return fmt.Errorf("iface: mirror: src %q not found: %w", srcIface, err)
	}
	dst, err := netlink.LinkByName(dstIface)
	if err != nil {
		return fmt.Errorf("iface: mirror: dst %q not found: %w", dstIface, err)
	}

	return setupClsactMirror(src, dst.Attrs().Index, ingress, egress)
}

// setupClsactMirror installs a mirred filter on the clsact hooks the caller
// asks for. clsact carries both hooks, so one qdisc kind serves ingress-only,
// egress-only, and two destinations at once, which the plain ingress qdisc
// cannot: it has no egress hook.
func setupClsactMirror(src netlink.Link, dstIndex int, ingress, egress bool) error {
	srcIndex := src.Attrs().Index
	created, err := ensureClsactQdisc(srcIndex)
	if err != nil {
		return err
	}

	if ingress {
		if err := addMirrorFilter(srcIndex, netlink.HANDLE_MIN_INGRESS, dstIndex); err != nil {
			undoMirrorSetup(src, created)
			return fmt.Errorf("iface: mirror: add ingress filter: %w", err)
		}
	}

	if egress {
		if err := addMirrorFilter(srcIndex, netlink.HANDLE_MIN_EGRESS, dstIndex); err != nil {
			undoMirrorSetup(src, created)
			return fmt.Errorf("iface: mirror: add egress filter: %w", err)
		}
	}

	return nil
}

// ensureClsactQdisc adds the clsact qdisc and reports whether this call
// created it. EEXIST means another subsystem got there first, and the hooks
// the mirror needs are already present, so it is not an error.
func ensureClsactQdisc(linkIndex int) (bool, error) {
	if err := netlink.QdiscAdd(clsactQdisc(linkIndex)); err != nil {
		if !errors.Is(err, unix.EEXIST) {
			return false, fmt.Errorf("iface: mirror: add clsact qdisc: %w", err)
		}
		return false, nil
	}
	return true, nil
}

// addMirrorFilter installs the mirred filter on one clsact hook. An existing
// filter at the mirror's priority is replaced, so applying an unchanged config
// twice leaves one filter per hook instead of failing.
func addMirrorFilter(linkIndex int, parent uint32, dstIndex int) error {
	filter := mirrorFilter(linkIndex, parent, dstIndex)
	err := netlink.FilterAdd(filter)
	if errors.Is(err, unix.EEXIST) {
		_ = netlink.FilterDel(filter)
		err = netlink.FilterAdd(filter)
	}
	return err
}

// undoMirrorSetup returns the interface to the state it was in before a failed
// filter add. It clears the mirror's priority on both hooks, which covers every
// filter this call can have installed. The qdisc goes only if this call created
// it and nothing else has attached to it since.
//
// Clearing a priority CAN remove a mirror filter an earlier call installed
// rather than this one. Through the config path it cannot: removeStaleMirrors
// retires a changed mirror before the new one is installed.
func undoMirrorSetup(src netlink.Link, createdQdisc bool) {
	if err := removeMirrorFilters(src.Attrs().Index); err != nil {
		logger().Warn("iface: mirror: rollback could not remove mirror filters",
			"iface", src.Attrs().Name, "err", err)
	}
	if !createdQdisc {
		return
	}
	if err := removeUnusedIngressQdisc(src); err != nil {
		logger().Warn("iface: mirror: rollback could not remove the clsact qdisc",
			"iface", src.Attrs().Name, "err", err)
	}
}

func (b *netlinkBackend) RemoveMirror(srcIface string) error {
	if err := iface.ValidateIfaceName(srcIface); err != nil {
		return fmt.Errorf("iface: mirror: %w", err)
	}

	link, err := netlink.LinkByName(srcIface)
	if err != nil {
		return fmt.Errorf("iface: mirror: %q not found: %w", srcIface, err)
	}

	if err := removeMirrorFilters(link.Attrs().Index); err != nil {
		return fmt.Errorf("iface: mirror: remove mirror filter on %q: %w", srcIface, err)
	}

	// The qdisc at ffff: is deliberately LEFT, whether or not a mirror filter
	// was found. This is the implementer's reasoning rather than a ruling, so a
	// later reader should feel free to overturn it on better evidence.
	// RemoveMirror knows only that its own filters are gone; it does
	// not know who created the qdisc, and a teardown is not the moment to guess.
	//
	// Listing the remaining filters and deleting on an empty answer looks safe
	// and is not. RemoveSampling (internal/plugins/flowexport/sampling/tc_linux.go)
	// deletes its filter and deliberately leaves the qdisc, so an interface with
	// sampling configured presents both hooks empty for the length of a
	// reconfigure. Deleting there takes a qdisc SetupSampling created: its own
	// EEXIST-tolerant QdiscAdd then succeeds against nothing, its FilterAdd
	// fails, and flow export stops on that interface until it is reconfigured.
	// No ordering of the list and the delete closes that window, because the
	// answer can change between them.
	//
	// An empty clsact qdisc carries no filter and classifies no packet, and the
	// next SetupMirror or SetupSampling adopts it through the same EEXIST branch
	// that already exists. It is not free: the device keeps a miniq and the
	// ingress static key stays referenced for the life of the namespace. That is
	// the price of not deleting another subsystem's attachment point.
	// undoMirrorSetup still removes one, because a rollback DOES know it created
	// the qdisc moments earlier.
	return nil
}

// ListMirrors reports every mirror ze's own filters install right now. It reads
// the clsact hooks of every link rather than the memory of an earlier apply. A
// restart keeps no memory and a skipped teardown leaves none, so the kernel is
// the only place the answer survives either.
//
// The cost is two filter dumps per link. A link with no ingress-side qdisc
// answers ENOENT or EINVAL and costs one round trip.
//
// A filter that is not ze's own is not reported. The mirror owns priority 1
// with a matchall classifier and a mirred action. Every other filter on those
// hooks belongs to another subsystem (flow-export sampling sits at priority
// 100) or to the operator.
func (b *netlinkBackend) ListMirrors() ([]iface.MirrorState, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("iface: mirror: list links: %w", err)
	}

	names := make(map[int]string, len(links))
	for _, link := range links {
		names[link.Attrs().Index] = link.Attrs().Name
	}

	var states []iface.MirrorState
	for _, link := range links {
		ingress, err := mirrorDestinationName(link, netlink.HANDLE_MIN_INGRESS, names)
		if err != nil {
			return nil, err
		}
		egress, err := mirrorDestinationName(link, netlink.HANDLE_MIN_EGRESS, names)
		if err != nil {
			return nil, err
		}
		if ingress == "" && egress == "" {
			continue
		}
		states = append(states, iface.MirrorState{
			Interface: link.Attrs().Name,
			Ingress:   ingress,
			Egress:    egress,
		})
	}
	return states, nil
}

// mirrorDestinationName returns the device the mirror's own filter on one
// clsact hook copies to, and "" when the hook carries no mirror of ze's.
//
// A listing that fails for any reason other than "there is nothing there" is
// returned as an error, never as "". The caller reconciles against this answer
// and an empty one reads as "no mirror to retire", which is the permissive
// no-op a read must not invent (ai/rules/evidence.md).
func mirrorDestinationName(link netlink.Link, parent uint32, names map[int]string) (string, error) {
	filters, err := netlink.FilterList(link, parent)
	if err != nil {
		if isNotFound(err) {
			// No qdisc at ffff:, or the qdisc is there and this hook holds no
			// filter. Both mean this direction copies nothing.
			return "", nil
		}
		return "", fmt.Errorf("iface: mirror: list filters on %q: %w", link.Attrs().Name, err)
	}
	for _, filter := range filters {
		attrs := filter.Attrs()
		if attrs == nil || attrs.Priority != mirrorFilterPriority {
			continue
		}
		matchAll, ok := filter.(*netlink.MatchAll)
		if !ok {
			continue
		}
		for _, action := range matchAll.Actions {
			mirred, ok := action.(*netlink.MirredAction)
			if !ok || mirred.MirredAction != netlink.TCA_EGRESS_MIRROR {
				continue
			}
			if name, known := names[mirred.Ifindex]; known {
				return name, nil
			}
			return iface.MirrorDestinationUnresolved, nil
		}
	}
	return "", nil
}

// removeMirrorFilters deletes the mirror's own filters from both clsact hooks.
// A filter that is already gone is not an error: the desired state is reached
// either way.
func removeMirrorFilters(linkIndex int) error {
	for _, parent := range []uint32{netlink.HANDLE_MIN_INGRESS, netlink.HANDLE_MIN_EGRESS} {
		err := netlink.FilterDel(mirrorFilter(linkIndex, parent, 0))
		if err != nil && !isNotFound(err) {
			return err
		}
	}
	return nil
}

// removeUnusedIngressQdisc deletes the qdisc at handle ffff: when no filter is
// left on either hook. It fails closed: when the remaining filters cannot be
// listed, the qdisc stays. Leaving an empty qdisc costs a miniq on the device;
// deleting a shared one costs another subsystem every filter it installed, so
// the asymmetry decides every uncertain case in favor of leaving it.
//
// Only undoMirrorSetup calls this, and only when it created the qdisc itself.
// The filter check is what stops a rollback taking a filter that arrived in
// between; the "no filter left" answer alone is NOT a safe last-user test,
// which is why RemoveMirror does not use one.
func removeUnusedIngressQdisc(link netlink.Link) error {
	for _, parent := range []uint32{netlink.HANDLE_MIN_INGRESS, netlink.HANDLE_MIN_EGRESS} {
		filters, err := netlink.FilterList(link, parent)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			logger().Warn("iface: mirror: cannot list remaining filters, leaving the qdisc in place",
				"iface", link.Attrs().Name, "err", err)
			return nil
		}
		if len(filters) > 0 {
			return nil
		}
	}

	qdisc := ingressSideQdisc(link)
	if qdisc == nil {
		return nil
	}
	if err := netlink.QdiscDel(qdisc); err != nil && !isNotFound(err) {
		return err
	}
	return nil
}

// ingressSideQdisc returns the qdisc installed at handle ffff:, clsact or the
// older ingress kind, or nil when the link has none. netlink spells the parent
// of both as HANDLE_CLSACT, which is HANDLE_INGRESS.
func ingressSideQdisc(link netlink.Link) netlink.Qdisc {
	qdiscs, err := netlink.QdiscList(link)
	if err != nil {
		logger().Warn("iface: mirror: cannot list qdiscs, leaving the qdisc in place",
			"iface", link.Attrs().Name, "err", err)
		return nil
	}
	for _, q := range qdiscs {
		attrs := q.Attrs()
		if attrs == nil {
			continue
		}
		if attrs.Handle == netlink.MakeHandle(mirrorQdiscHandleMajor, 0) && attrs.Parent == netlink.HANDLE_CLSACT {
			return q
		}
	}
	return nil
}

// clsactQdisc describes the shared ingress-side qdisc of a link.
func clsactQdisc(linkIndex int) *netlink.Clsact {
	return &netlink.Clsact{
		LinkIndex: linkIndex,
		Handle:    netlink.MakeHandle(mirrorQdiscHandleMajor, 0),
		Parent:    netlink.HANDLE_CLSACT,
	}
}

// mirrorFilter describes the mirror's filter on one clsact hook. A dstIndex of
// 0 carries no action and identifies the filter for deletion, which the kernel
// matches on link, parent, priority and protocol.
func mirrorFilter(linkIndex int, parent uint32, dstIndex int) *netlink.MatchAll {
	filter := &netlink.MatchAll{
		LinkIndex: linkIndex,
		Parent:    parent,
		Priority:  mirrorFilterPriority,
		Protocol:  unix.ETH_P_ALL,
	}
	if dstIndex != 0 {
		filter.Actions = []netlink.Action{
			&netlink.MirredAction{
				Action:       netlink.TC_ACT_PIPE,
				MirredAction: netlink.TCA_EGRESS_MIRROR,
				Ifindex:      dstIndex,
			},
		}
	}
	return filter
}
