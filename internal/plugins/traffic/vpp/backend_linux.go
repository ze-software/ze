// Design: docs/architecture/traffic/fw-7-traffic-vpp.md -- VPP traffic backend
// Related: ops.go -- vppOps interface consumed by applyWithOps / applyAll / applyInterface / reconcileRemovals

//go:build linux

package trafficvpp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.fd.io/govpp/binapi/interface_types"

	"github.com/ze-software/ze/internal/component/traffic"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

var errVppComponentNotInitialized = errors.New("vpp component not initialized")

// waitConnectedTimeout bounds how long Apply blocks waiting for VPP to be
// reachable. Value is the 5s agreed in spec Decision 1.
const waitConnectedTimeout = 5 * time.Second

const waitConnectorPoll = 50 * time.Millisecond

// backend implements traffic.Backend on top of VPP's binary API.
//
// Current scope: HTB and TBF qdiscs with EXACTLY ONE class, translated
// to a single VPP policer bound to interface egress via PolicerOutput.
// The verifier (see verify.go) rejects multi-class configs, every
// filter type, and every other qdisc type, so Apply only sees
// single-class HTB/TBF configs.
//
// Apply acquires a fresh api.Channel per call (spec Decision 3) and
// tears it down before returning; no channel is held across calls.
// Cross-Apply state: the (iface -> name -> policer index) set currently
// bound to interface output, and the (iface -> qdisc type) map so
// ListQdiscs reports what was actually configured. The next Apply
// diffs against the policer set to remove policers that the new
// desired no longer references.
type backend struct {
	mu sync.Mutex

	// connector is the accessor func used at Apply time. The connector
	// may be unavailable when LoadBackend runs (if VPP starts after
	// traffic), so we capture the accessor and resolve lazily.
	connector func() *vppcomp.Connector

	// interfaceOutputPolicers maps ifaceName -> (policer name -> policer
	// index). The index is required by PolicerDel; the name is used for
	// PolicerOutput binding and for human-readable reconciliation.
	interfaceOutputPolicers map[string]map[string]uint32

	// interfaceQdiscTypes records the qdisc type configured per
	// interface so ListQdiscs reports the correct type. Populated
	// alongside interfaceOutputPolicers at the end of a successful Apply.
	interfaceQdiscTypes map[string]traffic.QdiscType

	// interfaceClassifyBindings records, per interface, the classify tables
	// bound to its policer-classify feature for a class with protocol
	// filters. VPP classify tables are anonymous, so this in-memory tracker
	// is the only handle to unbind + delete them on a later Apply (see
	// classify_linux.go). Absent key == no classify pipeline on that iface.
	interfaceClassifyBindings map[string]classifyBinding
}

// newBackend is the factory registered with traffic.RegisterBackend("vpp").
// Captures the live accessor for the VPP component; the connector itself
// is resolved at each Apply so late VPP startup is tolerated.
func newBackend() (traffic.Backend, error) {
	return &backend{
		connector:                 vppcomp.GetActiveConnector,
		interfaceOutputPolicers:   make(map[string]map[string]uint32),
		interfaceQdiscTypes:       make(map[string]traffic.QdiscType),
		interfaceClassifyBindings: make(map[string]classifyBinding),
	}, nil
}

// Apply reconciles VPP's policer state to match the desired InterfaceQoS
// for each named interface. On error, any VPP state this call programmed
// is undone via the undo list, leaving VPP in its pre-Apply state so the
// component's journal rollback can re-apply the previous desired cleanly.
//
// ctx is propagated from the traffic component's plugin lifecycle. A canceled
// ctx short-circuits WaitConnected so a daemon shutdown is not blocked for the
// full waitConnectedTimeout when VPP is unreachable. WaitConnected applies its
// own timeout on top of the caller's ctx.
func (b *backend) Apply(ctx context.Context, desired map[string]traffic.InterfaceQoS) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	connCtx, connCancel := context.WithTimeout(ctx, waitConnectedTimeout)
	conn, err := b.waitConnector(connCtx)
	connCancel()
	if err != nil {
		return fmt.Errorf("traffic-vpp: %w", err)
	}
	if err := conn.WaitConnected(ctx, waitConnectedTimeout); err != nil {
		return fmt.Errorf("traffic-vpp: %w", err)
	}

	ch, err := conn.NewChannel()
	if err != nil {
		return fmt.Errorf("traffic-vpp: new channel: %w", err)
	}
	// ch.Close on GoVPP is a void method (no error return), so nothing
	// to log here. Kept documented so a future GoVPP version bump that
	// gains an error return is a compile-time signal to decide how to
	// handle it (propagate vs warn-only via logger()).
	defer ch.Close()

	// newGovppOps binds the reply deadline to the channel before the first
	// request goes out. govpp leaves a new channel on core.DefaultReplyTimeout,
	// which is 0 and means "wait forever", and a pooled one carries whatever
	// its previous owner set (timeout_linux.go).
	return b.applyWithOps(newGovppOps(ch), desired)
}

func (b *backend) waitConnector(ctx context.Context) (*vppcomp.Connector, error) {
	if b.connector == nil {
		return nil, errVppComponentNotInitialized
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if conn := b.connector(); conn != nil {
		return conn, nil
	}
	tick := time.NewTicker(waitConnectorPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tick.C:
			if conn := b.connector(); conn != nil {
				return conn, nil
			}
		}
	}
}

// applyWithOps runs the Apply pipeline against a vppOps seam. Split from Apply
// so unit tests can inject a scripted fakeOps without touching the ctx /
// connector / channel lifecycle logic.
//
// Called with b.mu held.
func (b *backend) applyWithOps(ops vppOps, desired map[string]traffic.InterfaceQoS) error {
	nameIndex, err := ops.dumpInterfaces()
	if err != nil {
		return fmt.Errorf("traffic-vpp: %w", err)
	}
	if err := b.cleanupStartupOrphans(ops, nameIndex, desired); err != nil {
		return fmt.Errorf("traffic-vpp: %w", err)
	}

	newOutputPolicers := make(map[string]map[string]uint32)
	newQdiscTypes := make(map[string]traffic.QdiscType, len(desired))
	newClassifyBindings := make(map[string]classifyBinding)
	var undo []func()
	applyErr := b.applyAll(ops, nameIndex, desired, newOutputPolicers, newQdiscTypes, newClassifyBindings, &undo)
	if applyErr != nil {
		// Undo what this Apply programmed so VPP returns to its pre-Apply
		// state before the component's journal rollback re-applies the
		// previous desired.
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
		return fmt.Errorf("traffic-vpp: %w", applyErr)
	}

	// Reconcile removals: policer bindings and classify tables present in
	// previous state but absent from the new desired state get torn down.
	// Tolerant of VPP-side absence (e.g. after a VPP restart the cached
	// indexes are stale); those deletions log a warning and continue instead
	// of failing the whole Apply.
	b.reconcileRemovals(ops, nameIndex, newOutputPolicers, newClassifyBindings)
	b.reconcileClassifyRemovals(ops, nameIndex, newClassifyBindings, newOutputPolicers)

	b.interfaceOutputPolicers = newOutputPolicers
	b.interfaceQdiscTypes = newQdiscTypes
	b.interfaceClassifyBindings = newClassifyBindings
	return nil
}

// cleanupStartupOrphans runs only when this Ze process has no in-memory VPP
// policer tracker yet. It removes old Ze-named policers that are present in
// VPP but absent from the desired config, so daemon restart does not leave
// stale traffic policing behind. Desired Ze policers are unbound, kept, and
// then rebound by applyAll; foreign policers are ignored.
//
// Classify tables (protocol filters) are NOT reclaimed here: VPP classify
// tables are anonymous (no Ze-owned name to match on), so a table left by a
// previous process cannot be identified at startup. An unbound classify table
// polices nothing (the policer-classify feature is re-pointed at the fresh
// tables applyAll creates), so a leak is inert memory, reclaimed only by a VPP
// restart. In-process reconcile (reconcileClassifyRemovals) does delete tables
// by their tracked indices. This gap is documented in the spec's Known
// Limitations.
//
// Called with b.mu held.
func (b *backend) cleanupStartupOrphans(
	ops vppOps,
	nameIndex map[string]interface_types.InterfaceIndex,
	desired map[string]traffic.InterfaceQoS,
) error {
	if len(b.interfaceOutputPolicers) != 0 {
		return nil
	}
	existing, err := ops.dumpPolicers()
	if err != nil {
		return fmt.Errorf("dump policers: %w", err)
	}
	desiredNames := desiredPolicerNames(desired)
	for _, name := range existing {
		if !strings.HasPrefix(name, policerNamePrefix) {
			continue
		}
		if ifaceName, ok := ifaceNameFromPolicerName(name); ok {
			if swIfIndex, present := nameIndex[ifaceName]; present {
				if err := ops.policerOutput(name, swIfIndex, false); err != nil {
					logger().Warn("traffic-vpp: unbind startup ze policer failed",
						"policer", name, "iface", ifaceName, "err", err)
				}
			}
		}
		if desiredNames[name] {
			continue
		}
		if err := ops.policerDeleteByName(name); err != nil {
			return fmt.Errorf("delete startup orphan policer %q: %w", name, err)
		}
	}
	return nil
}

func desiredPolicerNames(desired map[string]traffic.InterfaceQoS) map[string]bool {
	names := make(map[string]bool)
	for ifaceName, qos := range desired {
		for _, cls := range qos.Qdisc.Classes {
			names[policerName(ifaceName, cls.Name)] = true
		}
	}
	return names
}

func ifaceNameFromPolicerName(name string) (string, bool) {
	rest, ok := strings.CutPrefix(name, policerNamePrefix)
	if !ok {
		return "", false
	}
	ifaceName, _, ok := strings.Cut(rest, "/")
	if !ok || ifaceName == "" {
		return "", false
	}
	return ifaceName, true
}

// applyAll walks every interface in desired and programs its state.
// Returns the first interface-level error it encounters -- unlike the
// verifier which aggregates errors via errors.Join, this path has side
// effects in VPP so short-circuiting keeps the undo list manageable.
// The caller is responsible for running the undo list on error.
//
// Called with b.mu held.
func (b *backend) applyAll(
	ops vppOps,
	nameIndex map[string]interface_types.InterfaceIndex,
	desired map[string]traffic.InterfaceQoS,
	newOutputPolicers map[string]map[string]uint32,
	newQdiscTypes map[string]traffic.QdiscType,
	newClassifyBindings map[string]classifyBinding,
	undo *[]func(),
) error {
	for ifaceName, qos := range desired {
		swIfIndex, ok := nameIndex[ifaceName]
		if !ok {
			return fmt.Errorf("interface %q not present in vpp", ifaceName)
		}
		if err := b.applyInterface(ops, ifaceName, swIfIndex, qos, newOutputPolicers, newQdiscTypes, newClassifyBindings, undo); err != nil {
			return fmt.Errorf("interface %q: %w", ifaceName, err)
		}
	}
	return nil
}

// applyInterface programs one interface's policers. Distinguishes CREATE
// (name not in prior state) from UPDATE (name already tracked by a
// previous Apply) to avoid undoing previously-working state:
//
//  1. Undo closures on UPDATE would tear down previously-working state
//     if a later class/interface fails. The component's journal rollback
//     would re-apply eventually, but in the window between undo and
//     rollback the operator's traffic goes unshaped. Undo is queued
//     only for CREATE operations.
//
// UPDATE still replays `PolicerOutput(apply=true)`: VPP-side events can
// remove the output binding while leaving Ze's in-memory tracker intact.
// Rebinding on every successful Apply makes same-process reapply converge
// back to the desired state.
//
// Called with b.mu held.
func (b *backend) applyInterface(
	ops vppOps,
	ifaceName string,
	swIfIndex interface_types.InterfaceIndex,
	desired traffic.InterfaceQoS,
	newOutputPolicers map[string]map[string]uint32,
	newQdiscTypes map[string]traffic.QdiscType,
	newClassifyBindings map[string]classifyBinding,
	undo *[]func(),
) error {
	qdisc := desired.Qdisc
	if qdisc.Type != traffic.QdiscHTB && qdisc.Type != traffic.QdiscTBF {
		// Verifier rejects every other qdisc type. Fail loudly here so a
		// verifier bypass (test harness, programmatic injection, future
		// refactor) does not silently leave the interface unconfigured.
		return fmt.Errorf("qdisc %s: not supported by backend vpp (verifier bypass?)", qdisc.Type)
	}
	// The verifier guarantees at least one class; a zero-class qdisc programs
	// nothing. Fail loudly on a verifier bypass rather than silently no-op.
	// Multi-class is supported: unfiltered classes bind to the egress
	// policer-output arc, filtered classes (protocol/dscp) steer through the
	// ingress policer-classify pipeline (built once per interface below).
	if len(qdisc.Classes) == 0 {
		return fmt.Errorf("qdisc %s: expected at least 1 class, got 0 (verifier bypass?)", qdisc.Type)
	}

	prevOutput := b.interfaceOutputPolicers[ifaceName]              // nil on first Apply
	prevClassify := b.interfaceClassifyBindings[ifaceName].policers // nil on first Apply
	thisOutputPolicers := make(map[string]uint32, len(qdisc.Classes))
	classifyPolicers := make(map[string]uint32)
	var steerings []classifySteer

	for _, cls := range qdisc.Classes {
		name := policerName(ifaceName, cls.Name)
		_, wasOutput := prevOutput[name]
		_, wasClassify := prevClassify[name]
		isUpdate := wasOutput || wasClassify

		p, err := policerFromClass(cls, qdisc.Type)
		if err != nil {
			return fmt.Errorf("class %q: %w", cls.Name, err)
		}
		p.Name = name
		policerIdx, err := ops.policerAddDel(&p)
		if err != nil {
			return fmt.Errorf("class %q: %w", cls.Name, err)
		}
		if !isUpdate {
			addedIdx := policerIdx
			*undo = append(*undo, func() {
				_ = ops.policerDel(addedIdx)
			})
		}

		if classSteers(cls) {
			// Filtered class: only matching traffic is policed, via the ingress
			// policer-classify pipeline. Its steerings are collected here and
			// programmed once per interface (after the class loop) so multiple
			// filtered classes share one table chain per family. The policer is
			// tracked in classifyPolicers (not thisOutputPolicers) so the output
			// reconcile never unbinds a feature that was never applied.
			classifyPolicers[name] = policerIdx
			steerings = append(steerings, collectClassSteerings(cls, policerIdx)...)
			continue
		}

		// Unfiltered class: bind on both CREATE and UPDATE. UPDATE rebinding
		// repairs the same-process case where VPP loses the output feature
		// binding between two Apply calls but Ze still tracks the policer.
		if err := ops.policerOutput(name, swIfIndex, true); err != nil {
			return fmt.Errorf("class %q: %w", cls.Name, err)
		}
		if !isUpdate {
			boundName, boundIdx := name, swIfIndex
			*undo = append(*undo, func() {
				_ = ops.policerOutput(boundName, boundIdx, false)
			})
		}
		thisOutputPolicers[name] = policerIdx
	}

	// Program the classify pipeline once for all filtered classes on this
	// interface: fresh tables (anonymous, no upsert) chained per family and
	// bound to the interface policer-classify feature. The previous binding is
	// torn down by reconcileClassifyRemovals.
	if len(steerings) > 0 {
		binding, err := applyInterfaceClassify(ops, swIfIndex, steerings, classifyPolicers, undo)
		if err != nil {
			return fmt.Errorf("interface %q classify: %w", ifaceName, err)
		}
		newClassifyBindings[ifaceName] = binding
	}

	newOutputPolicers[ifaceName] = thisOutputPolicers
	newQdiscTypes[ifaceName] = qdisc.Type
	return nil
}

// reconcileRemovals diffs the previous programmed state against the new
// state and unbinds + deletes policers no longer referenced. Deletion
// failures are logged as warnings rather than propagated because they
// happen naturally after a VPP restart (the cached policer index no
// longer exists), and failing the whole Apply for a stale cleanup would
// leave the new desired state partially programmed.
//
// When an interface has disappeared from VPP (nameIndex lookup fails),
// we still attempt PolicerDel for each of its policers: VPP policers
// are named entities independent of interface bindings, so a gone
// interface auto-unbinds but does NOT auto-delete the policer. Without
// the delete call those policers would leak in VPP forever (the
// backend's in-memory tracker gets cleared by the caller's
// `b.interfaceOutputPolicers = newOutputPolicers` assignment).
//
// Apply-order transient: Apply programs the NEW state before calling
// reconcileRemovals, so during the window between the new
// `PolicerOutput(apply=true)` and the old class's unbind here, both
// policers sit on VPP's output feature arc and run in series. For a
// rename c1->c2 with different rates, traffic in this window sees
// `min(old_rate, new_rate)`. The alternative order (reconcile first,
// then apply) would open a NO-shaping window instead, which is worse
// for burst control. Accepting the min-rate transient is deliberate.
//
// Called with b.mu held.
func (b *backend) reconcileRemovals(
	ops vppOps,
	nameIndex map[string]interface_types.InterfaceIndex,
	newOutputPolicers map[string]map[string]uint32,
	newClassifyBindings map[string]classifyBinding,
) {
	lg := logger()
	for ifaceName, prevSet := range b.interfaceOutputPolicers {
		newSet := newOutputPolicers[ifaceName]
		newClassify := newClassifyBindings[ifaceName].policers
		swIfIndex, ifacePresent := nameIndex[ifaceName]
		for name, policerIdx := range prevSet {
			if _, keep := newSet[name]; keep {
				continue
			}
			// Migrated to the classify path (class gained a steering filter):
			// the same-named policer is now tracked in the classify binding, so
			// do NOT delete it here -- reconcileClassifyRemovals owns it.
			if _, migrated := newClassify[name]; migrated {
				continue
			}
			if ifacePresent {
				if err := ops.policerOutput(name, swIfIndex, false); err != nil {
					lg.Warn("traffic-vpp: unbind stale policer failed (treating as already gone)",
						"policer", name, "iface", ifaceName, "err", err)
				}
			}
			// Always attempt PolicerDel: interface-absent means VPP
			// already auto-unbinded, but the named policer entity
			// persists until explicitly deleted.
			if err := ops.policerDel(policerIdx); err != nil {
				lg.Warn("traffic-vpp: delete stale policer failed (treating as already gone)",
					"policer", name, "idx", policerIdx, "iface-present", ifacePresent, "err", err)
			}
		}
	}
}

// ListQdiscs returns the currently-desired state for an interface. VPP
// does not have a symmetric read-back that recomposes ze's Qdisc shape;
// returning the last-applied desired is a pragmatic stub that keeps the
// CLI `ze cli traffic show` useful against a VPP backend. The qdisc
// type comes from interfaceQdiscTypes so the returned shape matches
// what the operator actually configured (HTB vs TBF).
func (b *backend) ListQdiscs(ifaceName string) (traffic.InterfaceQoS, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	policers := b.interfaceOutputPolicers[ifaceName]
	classes := make([]traffic.TrafficClass, 0, len(policers))
	for name := range policers {
		classes = append(classes, traffic.TrafficClass{Name: name})
	}
	qdiscType, ok := b.interfaceQdiscTypes[ifaceName]
	if !ok {
		// No apply recorded for this interface; surface as zero-value
		// qdisc (Type=qdiscUnknown) rather than lying with HTB.
		qdiscType = 0
	}
	return traffic.InterfaceQoS{
		Interface: ifaceName,
		Qdisc: traffic.Qdisc{
			Type:    qdiscType,
			Classes: classes,
		},
	}, nil
}

// Close releases backend resources. The VPP connection itself is owned by
// the vpp component; this backend holds no persistent channel.
func (b *backend) Close() error { return nil }
