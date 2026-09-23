// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- accepted reservation state.
// RFC: rfc/short/rfc2205.md, rfc/short/rfc3209.md
package rsvpte

import (
	"bytes"
	"errors"
	"net/netip"
	"time"
)

func ownedObject(old, received []byte) []byte {
	if bytes.Equal(old, received) {
		return old
	}
	return bytes.Clone(received)
}

func receivedPathState(msg *ParsedMessage, old *pathStateBlock) *pathStateBlock {
	psb := &pathStateBlock{Session: msg.Session, SenderTemplate: msg.SenderTemplate,
		Hop: msg.Hop, SenderTSpec: msg.SenderTSpec, LabelRequest: msg.LabelRequest,
		RecordRoute: msg.HasRRO, RecordLabels: msg.SessionAttr.Flags&SessAttrLabelRecording != 0,
		RefreshPeriod: receivedRefreshPeriod(msg), LastRefresh: time.Now()}
	if old != nil {
		psb.ReceivedAdspec = old.ReceivedAdspec
		psb.SenderTSpecRaw = old.SenderTSpecRaw
		psb.SessionAttr = old.SessionAttr
	}
	psb.ReceivedAdspec = ownedObject(psb.ReceivedAdspec, msg.AdspecRaw)
	psb.SenderTSpecRaw = ownedObject(psb.SenderTSpecRaw, msg.SenderTSpecRaw)
	psb.SessionAttr = ownedObject(psb.SessionAttr, msg.SessionAttrRaw)
	psb.ForwardObjects = ownObjects(msg.ForwardObjects)
	return psb
}

// reservationPathMTU requires an actual usable outgoing advertisement. The
// received M is bounded by both that advertisement and the native egress link.
// Caller MUST hold lsp.mu; SENDER_TSPEC.M is never an input to this decision.
func reservationPathMTU(lsp *LSP, flow FlowSpec) uint32 {
	advertised := adspecPathMTU(lsp.PSB.Adspec, lsp.PSB.SenderTSpec.Service)
	if lsp.ProtectionInUse {
		advertised = lsp.RepairPathMTU
	}
	mtu := receivedPathMTU(flow, advertised != 0)
	if mtu == 0 {
		return 0
	}
	mtu = min(mtu, advertised)
	if !lsp.ProtectionInUse && lsp.PSB.Route.MTU != 0 {
		mtu = min(mtu, lsp.PSB.Route.MTU)
	}
	return mtu
}

func (e *engine) sessionStyle(session sessionIPv4) uint32 {
	for _, lsp := range e.table.All() {
		if sessionFromKey(lsp.Key) != sessionFromIPv4(session) {
			continue
		}
		lsp.mu.Lock()
		var style uint32
		if lsp.RSB != nil {
			style = lsp.RSB.Style
		}
		lsp.mu.Unlock()
		if style != 0 {
			return style
		}
	}
	return 0
}

func (e *engine) missingSenderCode(session sessionIPv4) uint8 {
	for _, lsp := range e.table.All() {
		if sessionFromKey(lsp.Key) == sessionFromIPv4(session) {
			return ErrCodeNoSender
		}
	}
	return ErrCodeNoPath
}

func (e *engine) restoreReservation(iface string, key lspKey, old *resvStateBlock, bandwidth float64) {
	if old == nil {
		e.admission.release(iface, key)
		return
	}
	// The receive loop and config replacement are serialized. No other
	// reservation can consume the released delta before this rollback.
	if err := e.admission.reserve(iface, key, old.Style, bandwidth); err != nil {
		e.log.Error("rsvp-te: reservation rollback failed", "error", err)
	}
}

// acceptReservation commits resources only after policy, admission and native
// installation all succeed. A failed replacement never overwrites accepted RSB.
func (e *engine) acceptReservation(src netip.Addr, msg *ParsedMessage, descriptor *flowDescriptor, filter *reservationFilter) {
	failed := *descriptor
	failed.Filters = []reservationFilter{*filter}
	key := keyFromFilter(msg.Session, filter.Filter)
	lsp, found := e.table.Get(key)
	if !found {
		lsp, found = e.repairedLSP(key, src)
	}
	if !found {
		e.rejectReservation(src, msg, &failed, e.missingSenderCode(msg.Session), 0, false)
		return
	}
	key = lsp.Key
	if !e.cfg().Policy.allows(key.SenderAddr, key.TunnelEndpoint) {
		e.rejectReservation(src, msg, &failed, ErrCodePolicyControlFailure, 0, false)
		return
	}
	if style := e.sessionStyle(msg.Session); style != 0 && style != msg.Style {
		e.rejectReservation(src, msg, &failed, ErrCodeConflictingStyle, uint16(style), false)
		return
	}
	lsp.mu.Lock()
	if lsp.PSB == nil || lsp.Role == RoleEgress {
		lsp.mu.Unlock()
		e.rejectReservation(src, msg, &failed, ErrCodeNoSender, 0, false)
		return
	}
	// Our PATH advertises LIH zero. A returning reservation must name that
	// exact outgoing handle, not its own unrelated incoming interface index.
	if msg.Hop.LIH != 0 || (msg.HasHop && !e.samePeer(msg.Hop.NextHop, src)) {
		lsp.mu.Unlock()
		return
	}
	repaired := lsp.ProtectionInUse
	if repaired {
		if filter.Filter.SenderAddr != lsp.RepairSender {
			lsp.mu.Unlock()
			return
		}
	} else if lsp.NextHop.IsValid() && !e.samePeer(lsp.NextHop, src) {
		lsp.mu.Unlock()
		return
	}
	old, oldBandwidth, oldIface := lsp.RSB, lsp.Bandwidth, lsp.AdmissionIface
	inLabel, role, bypassKey := lsp.InLabel, lsp.Role, lsp.Bypass
	pathMTU := reservationPathMTU(lsp, descriptor.FlowSpec)
	confirmHere := msg.HasResvConfirm && (role == RoleIngress || old != nil && reservationCovers(old.FlowSpec, descriptor.FlowSpec))
	lsp.mu.Unlock()
	allocated := role == RoleTransit && inLabel == 0
	if allocated {
		inLabel = e.table.AllocateLabel()
	}
	bypassUp := e.bypassEstablished(bypassKey)
	lsp.mu.Lock()
	rsb := e.receivedReservation(lsp, msg, descriptor, filter, inLabel, pathMTU, bypassUp)
	if confirmHere {
		rsb.ResvConfirm = netip.Addr{}
	}
	var raw []byte
	if role == RoleTransit {
		outgoing := *rsb
		outgoing.Hop = lsp.PSB.Hop
		raw = buildResv(&outgoing, lsp.PSB.SenderTemplate, e.cfg().RefreshPeriod, e.cfg().RouterID)
	}
	lsp.mu.Unlock()
	if role == RoleTransit && len(raw) == 0 {
		if allocated {
			e.table.releaseLabel(inLabel)
		}
		e.rejectReservation(src, msg, &failed, ErrCodeTrafficControlSystem, 0, old != nil)
		return
	}
	if confirmHere && len(buildResvConf(msg, &failed, e.cfg().RouterID)) == 0 {
		if allocated {
			e.table.releaseLabel(inLabel)
		}
		e.rejectReservation(src, msg, &failed, ErrCodeTrafficControlSystem, 0, old != nil)
		return
	}
	iface, _ := e.admissionInterface(src)
	bandwidth := float64(descriptor.FlowSpec.TokenRate) * 8
	if err := e.admission.reserve(iface, key, msg.Style, bandwidth); err != nil {
		if allocated {
			e.table.releaseLabel(inLabel)
		}
		e.rejectReservation(src, msg, &failed, ErrCodeAdmissionControlFailure, ErrValueRequestedBandwidth, old != nil)
		if m := rsvpteMetricsPtr.Load(); m != nil {
			m.admissionDenied.Inc()
		}
		return
	}
	installErr := e.installReservation(lsp, role, inLabel, filter.Label.Label, src, pathMTU, repaired)
	if installErr != nil {
		e.restoreReservation(iface, key, old, oldBandwidth)
		if allocated {
			e.table.releaseLabel(inLabel)
		}
		e.rejectReservation(src, msg, &failed, ErrCodeTrafficControlSystem, 0, old != nil)
		return
	}
	lsp.mu.Lock()
	lsp.RSB, lsp.Reserved, lsp.AdmissionIface, lsp.Bandwidth = rsb, true, iface, bandwidth
	if role == RoleTransit {
		lsp.InLabel = inLabel
	}
	if repaired {
		lsp.BackupLabel = filter.Label.Label
	} else {
		lsp.OutLabel, lsp.NextHop = filter.Label.Label, src
		e.recordBackupLabel(lsp, filter)
	}
	lsp.mu.Unlock()
	if oldIface != "" && oldIface != iface {
		e.admission.release(oldIface, key)
	}
	// RFC 2205 Section 2.6: "If the reservation request from Rj is equal
	// to or smaller than the reservation in place on a node, its Resv is
	// not forwarded further, and if the Resv included a confirmation-
	// request object, a ResvConf message is sent back to Rj."
	if role == RoleTransit && !confirmHere {
		if err := e.sendResv(lsp); err != nil {
			e.log.Warn("rsvp-te: reservation relay failed", "error", err)
			return
		}
	}
	lsp.mu.Lock()
	lsp.setState(LSPStateUp)
	replaces := lsp.Replaces
	lsp.Replaces = nil
	lsp.mu.Unlock()
	emitLSPUp(e.log, lsp, e.table.Len())
	if confirmHere {
		failed.FlowSpec = rsb.FlowSpec
		if err := e.sendResvConf(lsp, msg, &failed); err != nil {
			e.log.Warn("rsvp-te: confirmation send failed", "error", err)
		}
	}
	if replaces != nil {
		e.teardownLSP(*replaces)
	}
}

func (e *engine) installReservation(lsp *LSP, role lspRole, inLabel, outLabel uint32, src netip.Addr, pathMTU uint32, repaired bool) error {
	if e.fib == nil {
		return errForwardingUnavailable
	}
	if repaired {
		return e.refreshBackupForwarding(lsp, outLabel, pathMTU)
	}
	if role == RoleTransit {
		return e.fib.programSwap(inLabel, outLabel, src, pathMTU)
	}
	tableID := uint32(0)
	if lsp.IsBypass {
		tableID = bypassTableID(lsp.Key)
	}
	fec := netip.PrefixFrom(lsp.Key.TunnelEndpoint, 32)
	return e.fib.programPush(fec, []uint32{outLabel}, src, tableID, pathMTU)
}

// receivedReservation retains borrowed objects and prepares the upstream RRO.
// Caller MUST hold lsp.mu.
func (e *engine) receivedReservation(lsp *LSP, msg *ParsedMessage, descriptor *flowDescriptor, filter *reservationFilter, inLabel, pathMTU uint32, bypassUp bool) *resvStateBlock {
	rsb := &resvStateBlock{Session: msg.Session, FlowSpec: descriptor.FlowSpec,
		FlowSpecRaw: bytes.Clone(descriptor.FlowSpecRaw), ForwardObjects: ownObjects(msg.ForwardObjects),
		Label: filter.Label, Style: msg.Style, Hop: msg.Hop, PathMTU: pathMTU,
		ResvConfirm: msg.ResvConfirm, LastRefresh: time.Now(), RefreshPeriod: receivedRefreshPeriod(msg)}
	if pathMTU != 0 {
		rsb.FlowSpec.MaxPacketSize = pathMTU
	}
	if lsp.RSB != nil {
		rsb.FlowSpecRaw = ownedObject(lsp.RSB.FlowSpecRaw, descriptor.FlowSpecRaw)
		rsb.RRODropped = lsp.RSB.RRODropped
	}
	var label uint32
	var flags uint8
	if lsp.Role == RoleTransit {
		rsb.Label.Label = inLabel
		if lsp.PSB.RecordLabels {
			label = inLabel
		}
		if bypassUp {
			flags = rroProtectionFlags(lsp)
		}
	}
	if lsp.PSB.RecordRoute && !rsb.RRODropped {
		rsb.RRO = e.recordRoute(filter.RRO, msg.Session.TunnelID, flags, label)
		rsb.RRODropped = rsb.RRO == nil
	}
	return rsb
}

func (e *engine) recordBackupLabel(lsp *LSP, filter *reservationFilter) {
	if lsp.PSB.Protection == nil {
		return
	}
	lsp.BackupLabel = filter.Label.Label
	if !lsp.PSB.Protection.NodeProtection {
		return
	}
	lsp.BackupLabel = 0
	remaining := lsp.PSB.ERO
	if lsp.Role == RoleIngress {
		remaining = e.remainingERO(remaining)
	}
	if len(remaining) >= 2 {
		lsp.BackupLabel, _ = labelForAddr(filter.RRO, remaining[1].Address.Addr())
	}
	if lsp.BackupLabel == 0 {
		lsp.Bypass = nil
	}
}

// reservationObjects computes the opaque union for a shared reservation. State
// retains each request's own objects so a later tear removes its contribution.
func (e *engine) reservationObjects(lsp *LSP) [][]byte {
	var result [][]byte
	for _, candidate := range e.table.All() {
		if sessionFromKey(candidate.Key) != sessionFromKey(lsp.Key) {
			continue
		}
		candidate.mu.Lock()
		if candidate.RSB != nil && (candidate == lsp || candidate.RSB.Style == StyleSharedExplicit) {
			result = appendObjectUnion(result, candidate.RSB.ForwardObjects)
		}
		candidate.mu.Unlock()
	}
	return result
}

// appendObjectUnion deduplicates owned opaque objects without copying payloads.
func appendObjectUnion(result, objects [][]byte) [][]byte {
	for _, object := range objects {
		duplicate := false
		for _, retained := range result {
			if bytes.Equal(retained, object) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, object)
		}
	}
	return result
}

func (e *engine) handleResvErr(src netip.Addr, msg *ParsedMessage) {
	for i := range msg.FlowDescriptors {
		descriptor := &msg.FlowDescriptors[i]
		for _, filter := range descriptor.Filters {
			key := keyFromFilter(msg.Session, filter.Filter)
			lsp, found := e.table.Get(key)
			if !found {
				lsp, found = e.mergedLSP(key)
			}
			if !found {
				continue
			}
			lsp.mu.Lock()
			if lsp.RSB == nil || lsp.RSB.Style != msg.Style || !e.samePeer(lsp.PrevHop, src) {
				lsp.mu.Unlock()
				continue
			}
			rsb := lsp.RSB
			forward := true
			if msg.ErrorSpec.ErrorCode == ErrCodeAdmissionControlFailure {
				rsb.Blockade = descriptor.FlowSpec
				rsb.BlockadeUntil = time.Now().Add(3 * e.cfg().RefreshPeriod)
				if msg.ErrorSpec.Flags&ErrFlagInPlace != 0 && reservationStrictlyGreater(descriptor.FlowSpec, rsb.FlowSpec) {
					forward = false
				}
			}
			dst, role := lsp.NextHop, lsp.Role
			outgoing := *descriptor
			outgoing.Filters = []reservationFilter{{Filter: lsp.PSB.SenderTemplate}}
			hop := rsb.Hop
			lsp.mu.Unlock()
			if !forward {
				if err := e.sendResv(lsp); err != nil {
					e.log.Warn("rsvp-te: unblocked refresh failed", "error", err)
				}
				continue
			}
			if role == RoleEgress {
				e.log.Warn("rsvp-te: receiver reservation rejected", "code", msg.ErrorSpec.ErrorCode, "flags", msg.ErrorSpec.Flags)
				continue
			}
			if dst.IsValid() {
				hop.NextHop = e.cfg().RouterID
				raw := buildReservationControl(MsgTypeResvErr, msg.Session, hop, msg.Style, &outgoing, msg.ErrorSpec, msg.ForwardObjects)
				if len(raw) > 0 {
					if err := e.transport.Send(dst, raw); err != nil {
						e.log.Warn("rsvp-te: reservation error relay failed", "error", err)
					}
				}
			}
		}
	}
}

func reservationStrictlyGreater(have, want FlowSpec) bool {
	return reservationCovers(have, want) && have != want
}

func (e *engine) handleResvTear(src netip.Addr, msg *ParsedMessage) {
	for _, descriptor := range msg.FlowDescriptors {
		for _, filter := range descriptor.Filters {
			key := keyFromFilter(msg.Session, filter.Filter)
			lsp, found := e.table.Get(key)
			if !found {
				lsp, found = e.repairedLSP(key, src)
			}
			if !found {
				continue
			}
			lsp.mu.Lock()
			// RFC 2205 Section 3.1.6: "Matching reservation state must
			// match the SESSION, STYLE, and FILTER_SPEC objects as well as
			// the LIH in the RSVP_HOP object."
			if lsp.RSB == nil || lsp.RSB.Style != msg.Style || lsp.RSB.Hop != msg.Hop || !e.samePeer(lsp.RSB.Hop.NextHop, src) {
				lsp.mu.Unlock()
				continue
			}
			lsp.mu.Unlock()
			e.removeReservation(lsp, msg.ForwardObjects)
		}
	}
}

// removeReservation withdraws only the reservation. Sender PATH state and all
// other senders' shared charges survive a ResvTear and can receive a later Resv.
func (e *engine) removeReservation(lsp *LSP, objects [][]byte) {
	lsp.mu.Lock()
	if lsp.RSB == nil || lsp.PSB == nil {
		lsp.mu.Unlock()
		return
	}
	role, label, iface, style := lsp.Role, lsp.InLabel, lsp.AdmissionIface, lsp.RSB.Style
	var messages []sentReservationTear
	if lsp.MergedPaths == nil {
		messages = append(messages, sentReservationTear{lsp.PrevHop, lsp.PSB.Hop, lsp.PSB.SenderTemplate})
	} else {
		for filter, branch := range lsp.MergedPaths {
			messages = append(messages, sentReservationTear{branch.PrevHop, branch.Hop, filter})
		}
	}
	lsp.mu.Unlock()
	if e.fib == nil {
		return
	}
	var err error
	if role == RoleIngress {
		tableID := uint32(0)
		if lsp.IsBypass {
			tableID = bypassTableID(lsp.Key)
		}
		err = e.fib.removePush(netip.PrefixFrom(lsp.Key.TunnelEndpoint, 32), tableID)
	} else if label != 0 {
		err = e.fib.removeSwap(label)
	}
	if err != nil {
		e.log.Warn("rsvp-te: reservation withdrawal rejected", "error", err)
		return
	}
	lsp.mu.Lock()
	lsp.RSB, lsp.Reserved, lsp.InLabel, lsp.OutLabel = nil, false, 0, 0
	lsp.Bandwidth, lsp.AdmissionIface = 0, ""
	if role == RoleIngress {
		lsp.setState(LSPStatePathSent)
	} else {
		lsp.setState(LSPStatePathReceived)
	}
	lsp.mu.Unlock()
	e.admission.release(iface, lsp.Key)
	e.table.releaseLabel(label)
	for _, message := range messages {
		if !message.dst.IsValid() {
			continue
		}
		message.hop.NextHop = e.cfg().RouterID
		descriptor := flowDescriptor{Filters: []reservationFilter{{Filter: message.filter}}}
		raw := buildReservationControl(MsgTypeResvTear, sessionIPv4{TunnelEndpoint: lsp.Key.TunnelEndpoint,
			TunnelID: lsp.Key.TunnelID, ExtTunnelID: lsp.Key.ExtTunnelID}, message.hop, style, &descriptor, errorSpec{}, objects)
		if len(raw) > 0 {
			if err := e.transport.Send(message.dst, raw); err != nil {
				e.log.Warn("rsvp-te: reservation teardown relay failed", "error", err)
			}
		}
	}
	emitLSPDown(e.log, lsp, e.table.Len())
}

type sentReservationTear struct {
	dst    netip.Addr
	hop    rsvpHop
	filter senderTemplateIPv4
}

func (e *engine) withdrawPolicy(lsp *LSP) {
	lsp.mu.Lock()
	if lsp.PSB == nil {
		lsp.mu.Unlock()
		return
	}
	msg := ParsedMessage{Session: lsp.PSB.Session, SenderTemplate: lsp.PSB.SenderTemplate,
		SenderTSpec: lsp.PSB.SenderTSpec, ForwardObjects: lsp.PSB.ForwardObjects}
	prev, next := lsp.PrevHop, lsp.NextHop
	var descriptor *flowDescriptor
	if lsp.RSB != nil {
		msg.Style, msg.Hop = lsp.RSB.Style, lsp.RSB.Hop
		descriptor = &flowDescriptor{FlowSpec: lsp.RSB.FlowSpec, FlowSpecRaw: lsp.RSB.FlowSpecRaw,
			Filters: []reservationFilter{{Filter: lsp.PSB.SenderTemplate}}}
	}
	lsp.mu.Unlock()
	if next.IsValid() && descriptor != nil {
		e.rejectReservation(next, &msg, descriptor, ErrCodePolicyControlFailure, 0, false)
	}
	if prev.IsValid() {
		e.sendPathErr(prev, &msg, ErrCodePolicyControlFailure, 0)
	}
	e.teardownLSP(lsp.Key)
}

var errReservationMessage = errors.New("rsvp-te: reservation message exceeds carrier limit")
