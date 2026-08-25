// Design: docs/features/interfaces.md -- Traffic mirroring on the VPP dataplane
// Overview: ifacevpp.go -- vppBackendImpl, mirrors tracking map
//
// VPP SPAN (Switched Port ANalyzer) is programmed via
// sw_interface_span_enable_disable. The netlink backend (mirror_linux.go)
// mirrors via tc mirred on the source device; the VPP half maps the same
// (src, dst, ingress, egress) Backend.SetupMirror signature onto SPAN's
// (from, to, state, is_l2) fields.
//
// A-6 mapping: ingress -> RX, egress -> TX, both -> RX_TX; is_l2 is false
// (device SPAN) to match netlink's device-level port mirror. VPP additionally
// supports L2 bridge-domain SPAN (is_l2=true); that variant is intentionally
// not exposed here because the netlink parity target is a device port mirror.

package ifacevpp

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/span"

	"github.com/ze-software/ze/internal/component/iface"
)

// SetupMirror programs a VPP SPAN entry copying traffic from srcIface to
// dstIface. At least one of ingress/egress must be set (matching the netlink
// backend's errIfaceMirrorAtLeastOneOf). The installed (dst, is_l2) entry is
// recorded so RemoveMirror can disable it.
func (b *vppBackendImpl) SetupMirror(srcIface, dstIface string, ingress, egress bool) error {
	if !ingress && !egress {
		return fmt.Errorf("ifacevpp: mirror %q: at least one of ingress or egress must be set", srcIface)
	}
	fromIdx, err := b.resolveIndex(srcIface)
	if err != nil {
		return fmt.Errorf("ifacevpp: mirror src %q: %w", srcIface, err)
	}
	toIdx, err := b.resolveIndex(dstIface)
	if err != nil {
		return fmt.Errorf("ifacevpp: mirror dst %q: %w", dstIface, err)
	}
	const isL2 = false // device SPAN: netlink parity (A-6)
	if err := b.spanEnableDisable(fromIdx, toIdx, spanState(ingress, egress), isL2); err != nil {
		return fmt.Errorf("ifacevpp: SetupMirror %q->%q: %w", srcIface, dstIface, err)
	}
	b.recordMirror(srcIface, dstIface, isL2)
	return nil
}

// RemoveMirror disables every SPAN entry recorded for srcIface. It is
// idempotent: with no recorded entry it is a no-op, matching the netlink
// backend's tolerance of an already-absent qdisc.
//
// Every recorded destination is attempted, and the failures are joined. One
// source carries two destinations when the ingress and egress mirrors name
// different interfaces (setupMirrorSpec, internal/component/iface/
// config_mirror.go). takeMirrors drops the whole record before the first
// disable, so a return on the first failure leaves the second destination
// copying traffic with no record left to replay. The operator removed that
// mirror and it keeps running.
func (b *vppBackendImpl) RemoveMirror(srcIface string) error {
	dests := b.takeMirrors(srcIface)
	if len(dests) == 0 {
		return nil
	}
	fromIdx, err := b.resolveIndex(srcIface)
	if err != nil {
		return fmt.Errorf("ifacevpp: mirror src %q: %w", srcIface, err)
	}
	var errs []error
	// Sorted, so one teardown that fails reports the same way on every run,
	// the way removeStaleMirrors walks the previous config in its own order
	// rather than map-random (internal/component/iface/config_mirror.go).
	for _, dst := range slices.Sorted(maps.Keys(dests)) {
		isL2 := dests[dst]
		// Resolve here, not at setup time: the destination may have been
		// deleted and recreated since, which gives it a new SwIfIndex under
		// the same name. Disabling SPAN on the index it held then would leave
		// the live copy running.
		toIdx, err := b.resolveIndex(dst)
		if err != nil {
			errs = append(errs, fmt.Errorf("ifacevpp: mirror dst %q: %w", dst, err))
			continue
		}
		if err := b.spanEnableDisable(fromIdx, toIdx, span.SPAN_STATE_API_DISABLED, isL2); err != nil {
			errs = append(errs, fmt.Errorf("ifacevpp: RemoveMirror %q->%q: %w", srcIface, dst, err))
		}
	}
	return errors.Join(errs...)
}

// ListMirrors reports every device-level SPAN entry the dataplane carries, by
// dumping VPP rather than by reading the map recordMirror keeps. The map is
// in-memory and it empties on restart. It can never answer for a mirror the
// operator removed from the configuration while ze was down, which is the case
// this exists for.
//
// Only device SPAN (is_l2 false) is dumped, matching what SetupMirror installs.
// A bridge-domain SPAN entry is not ze's and is not reported.
//
// An entry whose source or destination index no longer resolves to a ze name is
// still reported. The entry is a live copy, and hiding it would leave the
// reconcile with nothing to retire. The destination then reads as
// iface.MirrorDestinationUnresolved, which no configuration can ask for, so the
// reconcile removes the entry.
func (b *vppBackendImpl) ListMirrors() ([]iface.MirrorState, error) {
	if err := b.ensureChannel(); err != nil {
		return nil, err
	}

	const isL2 = false // device SPAN: what SetupMirror installs
	ctx := b.ch.SendMultiRequest(&span.SwInterfaceSpanDump{IsL2: isL2})

	byInterface := make(map[string]*iface.MirrorState)
	var order []string
	for {
		details := &span.SwInterfaceSpanDetails{}
		last, err := ctx.ReceiveReply(details)
		if err != nil {
			return nil, fmt.Errorf("ifacevpp: SwInterfaceSpanDump: %w", err)
		}
		if last {
			break
		}
		if details.State == span.SPAN_STATE_API_DISABLED {
			continue
		}
		source, ok := b.names.lookupName(uint32(details.SwIfIndexFrom))
		if !ok {
			// The source is not a device ze names, so no configuration can ask
			// for it and no reconcile of ze's owns it.
			continue
		}
		destination, ok := b.names.lookupName(uint32(details.SwIfIndexTo))
		if !ok {
			destination = iface.MirrorDestinationUnresolved
		}
		state := byInterface[source]
		if state == nil {
			state = &iface.MirrorState{Interface: source}
			byInterface[source] = state
			order = append(order, source)
		}
		if details.State == span.SPAN_STATE_API_RX || details.State == span.SPAN_STATE_API_RX_TX {
			state.Ingress = destination
		}
		if details.State == span.SPAN_STATE_API_TX || details.State == span.SPAN_STATE_API_RX_TX {
			state.Egress = destination
		}
	}

	states := make([]iface.MirrorState, 0, len(order))
	for _, name := range order {
		states = append(states, *byInterface[name])
	}
	return states, nil
}

// spanState maps the (ingress, egress) direction flags to the VPP SPAN state.
func spanState(ingress, egress bool) span.SpanState {
	switch {
	case ingress && egress:
		return span.SPAN_STATE_API_RX_TX
	case ingress:
		return span.SPAN_STATE_API_RX
	default:
		return span.SPAN_STATE_API_TX
	}
}

// spanEnableDisable issues one sw_interface_span_enable_disable and checks the
// VPP return value.
func (b *vppBackendImpl) spanEnableDisable(from, to interface_types.InterfaceIndex, state span.SpanState, isL2 bool) error {
	req := &span.SwInterfaceSpanEnableDisable{
		SwIfIndexFrom: from,
		SwIfIndexTo:   to,
		State:         state,
		IsL2:          isL2,
	}
	reply := &span.SwInterfaceSpanEnableDisableReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("SwInterfaceSpanEnableDisable: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("SwInterfaceSpanEnableDisable retval=%d", reply.Retval)
	}
	return nil
}

// recordMirror stores a (dst, is_l2) SPAN entry under the source name so
// RemoveMirror can replay the delete. The destination is the ze NAME, so an
// interface recreated between the setup and the removal is disabled on the
// index it holds now rather than on the one it held then. Re-recording the
// same destination overwrites (SPAN enable is idempotent in VPP), so a
// re-apply does not accumulate duplicate entries. Lazily initializes the map.
func (b *vppBackendImpl) recordMirror(src, dst string, isL2 bool) {
	b.mirMu.Lock()
	defer b.mirMu.Unlock()
	if b.mirrors == nil {
		b.mirrors = make(map[string]map[string]bool)
	}
	dests := b.mirrors[src]
	if dests == nil {
		dests = make(map[string]bool)
		b.mirrors[src] = dests
	}
	dests[dst] = isL2
}

// takeMirrors removes and returns the recorded SPAN destination names for src.
func (b *vppBackendImpl) takeMirrors(src string) map[string]bool {
	b.mirMu.Lock()
	defer b.mirMu.Unlock()
	dests := b.mirrors[src]
	delete(b.mirrors, src)
	return dests
}
