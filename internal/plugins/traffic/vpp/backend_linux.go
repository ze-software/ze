// Design: plan/spec-fw-7-traffic-vpp.md -- VPP traffic backend
// Related: ops.go -- vppOps interface consumed by applyWithOps / applyAll / applyInterface / reconcileRemovals

//go:build linux

package trafficvpp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/policer"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
	vppcomp "codeberg.org/thomas-mangin/ze/internal/component/vpp"
)

// waitConnectedTimeout bounds how long Apply blocks waiting for VPP to be
// reachable. Value is the 5s agreed in spec Decision 1.
const waitConnectedTimeout = 5 * time.Second

const waitConnectorPoll = 50 * time.Millisecond

const policerNamePrefix = "ze/"

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
}

// newBackend is the factory registered with traffic.RegisterBackend("vpp").
// Captures the live accessor for the VPP component; the connector itself
// is resolved at each Apply so late VPP startup is tolerated.
func newBackend() (traffic.Backend, error) {
	return &backend{
		connector:               vppcomp.GetActiveConnector,
		interfaceOutputPolicers: make(map[string]map[string]uint32),
		interfaceQdiscTypes:     make(map[string]traffic.QdiscType),
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

	return b.applyWithOps(&govppOps{ch: ch}, desired)
}

func (b *backend) waitConnector(ctx context.Context) (*vppcomp.Connector, error) {
	if b.connector == nil {
		return nil, fmt.Errorf("vpp component not initialized")
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
	var undo []func()
	applyErr := b.applyAll(ops, nameIndex, desired, newOutputPolicers, newQdiscTypes, &undo)
	if applyErr != nil {
		// Undo what this Apply programmed so VPP returns to its pre-Apply
		// state before the component's journal rollback re-applies the
		// previous desired.
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
		return fmt.Errorf("traffic-vpp: %w", applyErr)
	}

	// Reconcile removals: policer bindings present in previous state but
	// absent from the new desired state get torn down. Tolerant of
	// VPP-side absence (e.g. after a VPP restart the cached indexes are
	// stale); those deletions log a warning and continue instead of
	// failing the whole Apply.
	b.reconcileRemovals(ops, nameIndex, newOutputPolicers)

	b.interfaceOutputPolicers = newOutputPolicers
	b.interfaceQdiscTypes = newQdiscTypes
	return nil
}

// cleanupStartupOrphans runs only when this Ze process has no in-memory VPP
// policer tracker yet. It removes old Ze-named policers that are present in
// VPP but absent from the desired config, so daemon restart does not leave
// stale traffic policing behind. Desired Ze policers are unbound, kept, and
// then rebound by applyAll; foreign policers are ignored.
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
	undo *[]func(),
) error {
	for ifaceName, qos := range desired {
		swIfIndex, ok := nameIndex[ifaceName]
		if !ok {
			return fmt.Errorf("interface %q not present in vpp", ifaceName)
		}
		if err := b.applyInterface(ops, ifaceName, swIfIndex, qos, newOutputPolicers, newQdiscTypes, undo); err != nil {
			return fmt.Errorf("interface %q: %w", ifaceName, err)
		}
	}
	return nil
}

// applyInterface programs one interface's policers. Distinguishes CREATE
// (name not in prior state) from UPDATE (name already tracked by a
// previous Apply) to avoid two failure modes found in review:
//
//  1. Undo closures on UPDATE would tear down previously-working state
//     if a later class/interface fails. The component's journal rollback
//     would re-apply eventually, but in the window between undo and
//     rollback the operator's traffic goes unshaped. Undo is queued
//     only for CREATE operations.
//
//  2. `PolicerOutput(apply=true)` on an already-bound (policer, iface)
//     pair has unverified VPP idempotency. Skipping the call for UPDATE
//     avoids triggering whatever VPP does for "already bound" on every
//     same-config reload.
//
// Called with b.mu held.
func (b *backend) applyInterface(
	ops vppOps,
	ifaceName string,
	swIfIndex interface_types.InterfaceIndex,
	desired traffic.InterfaceQoS,
	newOutputPolicers map[string]map[string]uint32,
	newQdiscTypes map[string]traffic.QdiscType,
	undo *[]func(),
) error {
	qdisc := desired.Qdisc
	if qdisc.Type != traffic.QdiscHTB && qdisc.Type != traffic.QdiscTBF {
		// Verifier rejects every other qdisc type. Fail loudly here so a
		// verifier bypass (test harness, programmatic injection, future
		// refactor) does not silently leave the interface unconfigured.
		return fmt.Errorf("qdisc %s: not supported by backend vpp (verifier bypass?)", qdisc.Type)
	}
	// Defense in depth: the verifier guarantees exactly one class,
	// but if a future code path or test harness bypasses the verifier,
	// multi-policer stacking on VPP's output feature arc would resurface
	// (effective rate = min(rates) instead of per-class shaping). Refuse
	// here so the bypass fails loudly instead of shipping silently wrong.
	if len(qdisc.Classes) != 1 {
		return fmt.Errorf("qdisc %s: expected exactly 1 class, got %d (verifier bypass?)", qdisc.Type, len(qdisc.Classes))
	}

	prevSet := b.interfaceOutputPolicers[ifaceName] // nil if first Apply for this iface
	thisIfacePolicers := make(map[string]uint32, len(qdisc.Classes))

	for _, cls := range qdisc.Classes {
		name := policerName(ifaceName, cls.Name)
		_, isUpdate := prevSet[name]

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

		// Bind only on CREATE. UPDATE case: previous Apply already bound
		// the policer by name; VPP's feature-arc registration persists
		// across PolicerAddDel upserts because the binding references
		// the name, not the index.
		if !isUpdate {
			if err := ops.policerOutput(name, swIfIndex, true); err != nil {
				return fmt.Errorf("class %q: %w", cls.Name, err)
			}
			boundName, boundIdx := name, swIfIndex
			*undo = append(*undo, func() {
				_ = ops.policerOutput(boundName, boundIdx, false)
			})
		}
		thisIfacePolicers[name] = policerIdx
	}
	newOutputPolicers[ifaceName] = thisIfacePolicers
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
) {
	lg := logger()
	for ifaceName, prevSet := range b.interfaceOutputPolicers {
		newSet := newOutputPolicers[ifaceName]
		swIfIndex, ifacePresent := nameIndex[ifaceName]
		for name, policerIdx := range prevSet {
			if _, keep := newSet[name]; keep {
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

// policerName builds the VPP policer name from interface and class. VPP
// limits names to 64 bytes; the verifier rejects any class whose produced
// name exceeds the limit, so this function does not truncate.
func policerName(ifaceName, className string) string {
	return fmt.Sprintf("%s%s/%s", policerNamePrefix, ifaceName, className)
}

// govppOps is the production adapter that implements vppOps on top of a
// live GoVPP api.Channel. Stateless by design -- the channel is the only
// field, and each method is a direct wrap around a single VPP RPC with
// retval decoding. Tests substitute a fakeOps that records calls.
//
// See `ops.go` for the interface definition.
type govppOps struct {
	ch api.Channel
}

// dumpInterfaces walks SwInterfaceDump and returns a name->sw_if_index
// map for every interface VPP currently knows about. Called once per
// Apply; sw_if_index values change on interface recreate so the lookup
// must not be cached across Applys.
func (g *govppOps) dumpInterfaces() (map[string]interface_types.InterfaceIndex, error) {
	req := &interfaces.SwInterfaceDump{SwIfIndex: ^interface_types.InterfaceIndex(0)}
	rctx := g.ch.SendMultiRequest(req)
	out := make(map[string]interface_types.InterfaceIndex)
	for {
		d := &interfaces.SwInterfaceDetails{}
		last, err := rctx.ReceiveReply(d)
		if err != nil {
			return nil, fmt.Errorf("SwInterfaceDump: %w", err)
		}
		if last {
			break
		}
		out[d.InterfaceName] = d.SwIfIndex
	}
	return out, nil
}

// dumpPolicers returns every policer name currently present in VPP. Used only
// by startup orphan cleanup; normal reconciliation uses the in-memory tracker
// because VPP does not expose enough binding state to reconstruct Ze's full
// desired model.
func (g *govppOps) dumpPolicers() ([]string, error) {
	req := &policer.PolicerDump{}
	rctx := g.ch.SendMultiRequest(req)
	var names []string
	for {
		d := &policer.PolicerDetails{}
		last, err := rctx.ReceiveReply(d)
		if err != nil {
			return nil, fmt.Errorf("PolicerDump: %w", err)
		}
		if last {
			break
		}
		names = append(names, d.Name)
	}
	return names, nil
}

// policerAddDel wraps PolicerAddDel with retval checking. Retval != 0 is
// decoded via api.RetvalToVPPApiError so the caller sees VPP's named error
// (e.g. ENOMEM, INVALID_VALUE) instead of a raw integer. Returns the index
// VPP assigned to the policer (required by policerDel).
func (g *govppOps) policerAddDel(req *policer.PolicerAddDel) (uint32, error) {
	reply := &policer.PolicerAddDelReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, fmt.Errorf("PolicerAddDel: %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return 0, fmt.Errorf("PolicerAddDel: %w", apiErr)
	}
	return reply.PolicerIndex, nil
}

// policerDel removes a policer by its VPP-assigned index. Used during
// reconciliation to clean up policers no longer referenced.
func (g *govppOps) policerDel(index uint32) error {
	req := &policer.PolicerDel{PolicerIndex: index}
	reply := &policer.PolicerDelReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("PolicerDel: %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return fmt.Errorf("PolicerDel: %w", apiErr)
	}
	return nil
}

// policerDeleteByName removes a policer by name via the older add/del API.
// Startup orphan cleanup only has names from PolicerDump, not VPP indexes.
func (g *govppOps) policerDeleteByName(name string) error {
	req := &policer.PolicerAddDel{IsAdd: false, Name: name}
	reply := &policer.PolicerAddDelReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("PolicerAddDel(delete): %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return fmt.Errorf("PolicerAddDel(delete): %w", apiErr)
	}
	return nil
}

// policerOutput binds (apply=true) or unbinds (apply=false) a policer by
// name to an interface's output. This is the mechanism by which a
// configured class rate actually limits egress traffic.
func (g *govppOps) policerOutput(name string, swIfIndex interface_types.InterfaceIndex, apply bool) error {
	req := &policer.PolicerOutput{Name: name, SwIfIndex: swIfIndex, Apply: apply}
	reply := &policer.PolicerOutputReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("PolicerOutput: %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return fmt.Errorf("PolicerOutput: %w", apiErr)
	}
	return nil
}
