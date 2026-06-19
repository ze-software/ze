// Design: plan/spec-mpls-3-rsvp-te.md -- RSVP-TE signaling engine
// RFC: rfc/short/rfc3209.md
// RFC: rfc/short/rfc2205.md
// Related: transport.go -- Transport seam the engine sends/receives over
// Related: build.go -- PATH/RESV/PathErr message construction
// Related: fsm.go -- LSP/LSPTable state this engine drives
// Related: admission.go -- bandwidth admission applied on reservation
//
// The engine ties the transport, codec, LSP state and admission control
// together. It implements the control plane for point-to-point LSPs:
//
//   - Ingress: sends PATH toward the tunnel endpoint, moves to ResvReceived/Up
//     when the matching RESV returns with a LABEL.
//   - Egress (SESSION.TunnelEndpoint == local router-id): on PATH, runs
//     admission control, allocates a label and returns a RESV; on admission
//     failure returns a PathErr (RFC 2205 Section A.5).
//
// Transit forwarding (multi-hop PATH/RESV relay) and dataplane label
// programming are tracked as remaining work in the spec; the engine logs and
// drops PATH messages destined past this node so behavior is explicit rather
// than silently wrong.
package rsvpte

import (
	"context"
	"log/slog"
	"net/netip"
	"time"
)

// fibProgrammer programs MPLS forwarding entries for an LSP. The production
// implementation emits to sysrib; tests use a fake. It is a seam so the engine
// is testable without the kernel and so the (separate) dataplane work can land
// behind a stable interface.
type fibProgrammer interface {
	// ProgramPush installs an ingress push entry: traffic to fec is forwarded
	// to nextHop with label pushed.
	ProgramPush(fec netip.Prefix, label uint32, nextHop netip.Addr) error
	// ProgramSwap installs a transit swap entry: packets arriving with inLabel
	// are forwarded to nextHop with inLabel swapped for outLabel.
	ProgramSwap(inLabel, outLabel uint32, nextHop netip.Addr) error
	// ProgramPop installs an egress pop entry: packets arriving with inLabel are
	// stripped of the label and forwarded toward nextHop (disposition).
	ProgramPop(inLabel uint32, nextHop netip.Addr) error
	// Remove withdraws a previously programmed push entry for fec.
	Remove(fec netip.Prefix) error
	// RemoveSwap withdraws a previously programmed swap or pop entry for inLabel.
	RemoveSwap(inLabel uint32) error
}

// engine runs the RSVP-TE control plane over a Transport. Per-LSP state is
// guarded by each LSP's own mutex (see LSP.mu), so the engine itself is
// stateless beyond its immutable dependencies.
type engine struct {
	transport Transport
	table     *lspTable
	admission *admissionController
	fib       fibProgrammer
	cfg       rsvpteConfig
	log       *slog.Logger
}

func newEngine(t Transport, table *lspTable, adm *admissionController, fib fibProgrammer, cfg rsvpteConfig, log *slog.Logger) *engine {
	return &engine{transport: t, table: table, admission: adm, fib: fib, cfg: cfg, log: log}
}

// run consumes received packets until the context is canceled or the transport
// closes its channel.
func (e *engine) run(ctx context.Context) {
	recv := e.transport.Recv()
	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-recv:
			if !ok {
				return
			}
			e.handlePacket(pkt)
		}
	}
}

func (e *engine) handlePacket(pkt Packet) {
	msg, err := DecodeMessage(pkt.Payload)
	if err != nil {
		e.log.Warn("rsvp-te: decode failed", "src", pkt.Src, "error", err)
		return
	}
	switch msg.Header.MsgType {
	case MsgTypePath:
		e.handlePath(pkt.Src, msg)
	case MsgTypeResv:
		e.handleResv(pkt.Src, msg)
	case MsgTypePathErr:
		e.handlePathErr(msg)
	case MsgTypePathTear:
		e.handlePathTear(msg)
	default:
		e.log.Debug("rsvp-te: unhandled message type", "type", msg.Header.MsgType)
	}
}

// sendPath transmits a PATH for an ingress LSP toward its tunnel endpoint. It
// builds the message under the LSP lock (PSB is shared with the engine and
// refresh goroutines) and sends it outside the lock.
func (e *engine) sendPath(lsp *LSP) error {
	lsp.mu.Lock()
	if lsp.PSB == nil {
		lsp.mu.Unlock()
		return nil
	}
	raw := buildPath(lsp.PSB, e.cfg.RouterID, defaultIPTTL)
	dst := lsp.Key.TunnelEndpoint
	lsp.mu.Unlock()

	if err := e.transport.Send(dst, raw); err != nil {
		return err
	}
	if m := rsvpteMetricsPtr.Load(); m != nil {
		m.pathMsgSent.Inc()
	}
	return nil
}

// sendResv re-sends this node's RESV upstream to keep the reservation soft-state
// alive (RFC 2205 Section 3.7). Egress and transit nodes hold an RSB and a valid
// PrevHop; an ingress LSP originates PATH instead, so it is skipped here.
func (e *engine) sendResv(lsp *LSP) error {
	lsp.mu.Lock()
	if lsp.RSB == nil || lsp.PSB == nil || !lsp.PrevHop.IsValid() {
		lsp.mu.Unlock()
		return nil
	}
	raw := buildResv(lsp.RSB, lsp.PSB.SenderTemplate, e.cfg.RefreshPeriod, e.cfg.RouterID)
	dst := lsp.PrevHop
	lsp.RSB.LastRefresh = time.Now()
	lsp.mu.Unlock()

	if err := e.transport.Send(dst, raw); err != nil {
		return err
	}
	if m := rsvpteMetricsPtr.Load(); m != nil {
		m.resvMsgSent.Inc()
	}
	return nil
}

const defaultIPTTL = 64

// handlePath processes a received PATH. This node is the egress (tail-end) when
// the SESSION tunnel endpoint is our router-id; otherwise it is a transit node
// and relays the PATH downstream along the ERO.
func (e *engine) handlePath(src netip.Addr, msg *ParsedMessage) {
	if m := rsvpteMetricsPtr.Load(); m != nil {
		m.pathMsgRecv.Inc()
	}
	if !msg.HasSession || !msg.HasSenderTemplate {
		e.log.Warn("rsvp-te: PATH missing SESSION/SENDER_TEMPLATE", "src", src)
		return
	}
	if msg.Session.TunnelEndpoint == e.cfg.RouterID {
		e.handlePathEgress(src, msg)
		return
	}
	e.handlePathTransit(src, msg)
}

// reserve runs admission control for the PATH's requested bandwidth on the
// interface facing the neighbor (src). It returns the resolved interface (empty
// when accounting was skipped) and ok=false (after sending a PathErr and bumping
// the denial metric) when the reservation is rejected. The caller stores the
// returned interface on the LSP so release later charges the same link.
func (e *engine) reserve(src netip.Addr, msg *ParsedMessage) (string, bool) {
	iface, ok := e.admissionInterface(src)
	if !ok {
		return "", true // no resolvable interface: accounting skipped (see admissionInterface)
	}
	if err := e.admission.reserveSession(iface, sessionFromIPv4(msg.Session), float64(msg.SenderTSpec.TokenRate)); err != nil {
		e.log.Info("rsvp-te: admission denied", "iface", iface, "bandwidth", msg.SenderTSpec.TokenRate)
		e.sendPathErr(src, msg, ErrCodeAdmissionControlFailure, ErrValueRequestedBandwidth)
		if m := rsvpteMetricsPtr.Load(); m != nil {
			m.admissionDenied.Inc()
		}
		return "", false
	}
	return iface, true
}

// handlePathEgress terminates the LSP: admission control, label allocation and
// a RESV back upstream toward the sender. It is idempotent across PATH
// refreshes (RFC 2205 soft-state): bandwidth is reserved and a label allocated
// only on the first PATH; refreshes reuse both.
func (e *engine) handlePathEgress(src netip.Addr, msg *ParsedMessage) {
	key := keyFromMessage(msg)
	lsp, existed := e.table.GetOrCreate(key)

	lsp.mu.Lock()
	alreadyReserved := lsp.Reserved
	label := lsp.InLabel
	admIface := lsp.AdmissionIface
	lsp.mu.Unlock()

	if !alreadyReserved {
		iface, ok := e.reserve(src, msg)
		if !ok {
			if !existed {
				e.table.Remove(key)
			}
			return
		}
		admIface = iface
	}
	if label == 0 {
		label = e.table.AllocateLabel()
	}

	lsp.mu.Lock()
	lsp.Role = RoleEgress
	lsp.PrevHop = src
	lsp.InLabel = label
	lsp.Reserved = true
	lsp.AdmissionIface = admIface
	lsp.Bandwidth = msg.SenderTSpec.TokenRate
	lsp.PSB = &pathStateBlock{
		Session:        msg.Session,
		SenderTemplate: msg.SenderTemplate,
		SenderTSpec:    msg.SenderTSpec,
		LabelRequest:   msg.LabelRequest,
		RefreshPeriod:  e.cfg.RefreshPeriod,
		LastRefresh:    time.Now(),
	}
	lsp.RSB = &resvStateBlock{
		Session:  msg.Session,
		FlowSpec: msg.SenderTSpec,
		Label:    labelObject{Label: label},
		Style:    StyleSharedExplicit,
		// RFC 3209 Section 4.4: the egress records its own address as the first
		// RRO entry; upstream nodes prepend themselves as the RESV travels back.
		RRO:         e.recordRoute(nil, msg.Session.TunnelID),
		LastRefresh: time.Now(),
	}
	lsp.setState(LSPStateUp)
	lsp.mu.Unlock()

	raw := buildResv(lsp.RSB, msg.SenderTemplate, e.cfg.RefreshPeriod, e.cfg.RouterID)
	if err := e.transport.Send(src, raw); err != nil {
		e.log.Warn("rsvp-te: send RESV failed", "dest", src, "error", err)
		return
	}
	if m := rsvpteMetricsPtr.Load(); m != nil {
		m.resvMsgSent.Inc()
	}
	emitLSPUp(e.log, lsp, e.table.Len())
	if !alreadyReserved {
		// Program the pop (disposition) entry once: packets arriving with our
		// in-label are decapsulated and IP-forwarded.
		if e.fib != nil {
			if err := e.fib.ProgramPop(label, netip.Addr{}); err != nil {
				e.log.Warn("rsvp-te: program pop failed", "lsp", key.String(), "error", err)
			}
		}
		e.log.Info("rsvp-te: egress LSP up", "lsp", key.String(), "in-label", label)
	}
}

// handlePathTransit installs path state and relays the PATH to the next ERO hop.
// The reservation is committed on RESV (handleResvTransit); admission here
// reserves eagerly so an oversubscribed transit link rejects with a PathErr
// before the PATH propagates further.
func (e *engine) handlePathTransit(src netip.Addr, msg *ParsedMessage) {
	// RFC 2205 Section 3.8: bound relay so a cyclic/malformed ERO cannot loop a
	// PATH forever. Drop (do not install state or reserve) when the hop budget
	// is exhausted.
	if msg.Header.TTL <= 1 {
		e.log.Warn("rsvp-te: transit PATH TTL exhausted, dropping", "src", src, "dest", msg.Session.TunnelEndpoint)
		return
	}
	nextHop, rem, ok := e.nextHopFromERO(msg.ERO)
	if !ok {
		e.log.Warn("rsvp-te: transit PATH has no usable ERO next hop", "src", src, "dest", msg.Session.TunnelEndpoint)
		e.sendPathErr(src, msg, ErrCodeRoutingProblem, ErrValueBadEROObject)
		return
	}

	key := keyFromMessage(msg)
	lsp, existed := e.table.GetOrCreate(key)
	lsp.mu.Lock()
	alreadyReserved := lsp.Reserved
	admIface := lsp.AdmissionIface
	lsp.mu.Unlock()
	if !alreadyReserved {
		iface, ok := e.reserve(src, msg)
		if !ok {
			if !existed {
				e.table.Remove(key)
			}
			return
		}
		admIface = iface
	}

	lsp.mu.Lock()
	lsp.Role = RoleTransit
	lsp.PrevHop = src
	lsp.NextHop = nextHop
	lsp.Reserved = true
	lsp.AdmissionIface = admIface
	lsp.Bandwidth = msg.SenderTSpec.TokenRate
	lsp.PSB = &pathStateBlock{
		Session:        msg.Session,
		SenderTemplate: msg.SenderTemplate,
		Hop:            rsvpHop{NextHop: src}, // PHOP: where RESV is sent back to
		ERO:            rem,
		SenderTSpec:    msg.SenderTSpec,
		LabelRequest:   msg.LabelRequest,
		RefreshPeriod:  e.cfg.RefreshPeriod,
		LastRefresh:    time.Now(),
	}
	lsp.setState(LSPStatePathReceived)
	lsp.mu.Unlock()

	// Relay the PATH downstream with a decremented TTL. The ERO is forwarded
	// unchanged (rem still begins with the next hop) so the next node strips
	// itself in turn.
	fwd := &pathStateBlock{
		Session:        msg.Session,
		SenderTemplate: msg.SenderTemplate,
		ERO:            rem,
		SenderTSpec:    msg.SenderTSpec,
		LabelRequest:   msg.LabelRequest,
		RefreshPeriod:  e.cfg.RefreshPeriod,
	}
	raw := buildPath(fwd, e.cfg.RouterID, msg.Header.TTL-1)
	if err := e.transport.Send(nextHop, raw); err != nil {
		e.log.Warn("rsvp-te: transit PATH relay failed", "next-hop", nextHop, "error", err)
		return
	}
	if m := rsvpteMetricsPtr.Load(); m != nil {
		m.pathMsgSent.Inc()
	}
	if !alreadyReserved {
		e.log.Info("rsvp-te: transit PATH relayed", "lsp", key.String(), "next-hop", nextHop)
	}
}

// nextHopFromERO strips leading ERO subobjects that name this node's own
// address and returns the address of the next remaining hop plus the remaining
// ERO (RFC 3209 Section 4.3.4: a node removes the subobject identifying itself,
// then the new first subobject identifies the next hop).
func (e *engine) nextHopFromERO(ero []eroHop) (netip.Addr, []eroHop, bool) {
	rem := ero
	for len(rem) > 0 && rem[0].Address.Addr() == e.cfg.RouterID {
		rem = rem[1:]
	}
	if len(rem) == 0 {
		return netip.Addr{}, nil, false
	}
	return rem[0].Address.Addr(), rem, true
}

// handleResv processes a received RESV and dispatches by this node's role for
// the LSP: ingress programs a push and comes up; transit allocates a local
// label, programs a swap and relays the RESV upstream.
func (e *engine) handleResv(src netip.Addr, msg *ParsedMessage) {
	if m := rsvpteMetricsPtr.Load(); m != nil {
		m.resvMsgRecv.Inc()
	}
	if !msg.HasSession || !msg.HasLabel {
		e.log.Warn("rsvp-te: RESV missing SESSION/LABEL", "src", src)
		return
	}
	key := keyFromMessage(msg)
	lsp, ok := e.table.Get(key)
	if !ok {
		e.log.Debug("rsvp-te: RESV for unknown LSP", "lsp", key.String())
		return
	}
	switch lsp.Role {
	case RoleIngress:
		e.handleResvIngress(src, msg, lsp, key)
	case RoleTransit:
		e.handleResvTransit(src, msg, lsp, key)
	default:
		e.log.Debug("rsvp-te: RESV for egress LSP ignored", "lsp", key.String())
	}
}

// recordRoute prepends this node to the downstream RRO and warns if the recorded
// route hit maxRecordRouteHops. A truncation means a pathological path or routing
// loop, so it must not pass silently (the head-end's view would be incomplete).
func (e *engine) recordRoute(downstream []rroEntry, tunnelID uint16) []rroEntry {
	rro, truncated := prependRRO(e.cfg.RouterID, downstream)
	if truncated {
		e.log.Warn("rsvp-te: recorded route truncated at hop limit; possible routing loop",
			"limit", maxRecordRouteHops, "tunnel", tunnelID)
	}
	return rro
}

func (e *engine) handleResvIngress(src netip.Addr, msg *ParsedMessage, lsp *LSP, key lspKey) {
	lsp.mu.Lock()
	lsp.OutLabel = msg.Label.Label
	lsp.NextHop = src
	// RFC 3209 Section 4.4: the head-end records the full path the LSP took from
	// the RESV's RRO (prepending itself) so `show rsvp-te session` can display it.
	lsp.RSB = &resvStateBlock{
		Session:     msg.Session,
		FlowSpec:    msg.FlowSpec,
		Label:       msg.Label,
		Style:       msg.Style,
		RRO:         e.recordRoute(msg.RRO, msg.Session.TunnelID),
		LastRefresh: time.Now(),
	}
	lsp.setState(LSPStateUp)
	replaces := lsp.Replaces
	lsp.Replaces = nil
	lsp.mu.Unlock()

	if e.fib != nil {
		fec := netip.PrefixFrom(key.TunnelEndpoint, key.TunnelEndpoint.BitLen())
		if err := e.fib.ProgramPush(fec, msg.Label.Label, src); err != nil {
			e.log.Warn("rsvp-te: program push failed", "lsp", key.String(), "error", err)
		}
	}
	emitLSPUp(e.log, lsp, e.table.Len())
	e.log.Info("rsvp-te: ingress LSP up", "lsp", key.String(), "out-label", msg.Label.Label)

	// RFC 3209 Section 6.1: now that the replacement is up, tear the old LSP.
	if replaces != nil {
		e.teardownLSP(*replaces)
	}
}

// handleResvTransit allocates this node's incoming label, programs the swap from
// that label to the downstream label, and relays a RESV upstream carrying the
// local label so the upstream node swaps to it.
func (e *engine) handleResvTransit(src netip.Addr, msg *ParsedMessage, lsp *LSP, key lspKey) {
	if lsp.PSB == nil {
		e.log.Warn("rsvp-te: transit RESV without path state", "lsp", key.String())
		return
	}
	inLabel := lsp.InLabel
	if inLabel == 0 {
		inLabel = e.table.AllocateLabel()
	}

	lsp.mu.Lock()
	lsp.InLabel = inLabel
	lsp.OutLabel = msg.Label.Label
	lsp.NextHop = src
	lsp.RSB = &resvStateBlock{
		Session:  msg.Session,
		FlowSpec: msg.FlowSpec,
		Label:    labelObject{Label: inLabel},
		Style:    msg.Style,
		// RFC 3209 Section 4.4: record this node ahead of the downstream route
		// before relaying the RESV upstream.
		RRO:         e.recordRoute(msg.RRO, msg.Session.TunnelID),
		LastRefresh: time.Now(),
	}
	prevHop := lsp.PSB.Hop.NextHop
	filter := lsp.PSB.SenderTemplate
	lsp.setState(LSPStateUp)
	lsp.mu.Unlock()

	if e.fib != nil {
		if err := e.fib.ProgramSwap(inLabel, msg.Label.Label, src); err != nil {
			e.log.Warn("rsvp-te: program swap failed", "lsp", key.String(), "error", err)
		}
	}

	raw := buildResv(lsp.RSB, filter, e.cfg.RefreshPeriod, e.cfg.RouterID)
	if err := e.transport.Send(prevHop, raw); err != nil {
		e.log.Warn("rsvp-te: transit RESV relay failed", "prev-hop", prevHop, "error", err)
		return
	}
	if m := rsvpteMetricsPtr.Load(); m != nil {
		m.resvMsgSent.Inc()
	}
	emitLSPUp(e.log, lsp, e.table.Len())
	e.log.Info("rsvp-te: transit LSP up", "lsp", key.String(), "in-label", inLabel, "out-label", msg.Label.Label)
}

func (e *engine) handlePathErr(msg *ParsedMessage) {
	if m := rsvpteMetricsPtr.Load(); m != nil {
		m.pathErrRecv.Inc()
	}
	if !msg.HasSession {
		return
	}
	eb := getEventBus()
	if eb != nil {
		evt := &PathErrEvent{
			TunnelEndpoint: msg.Session.TunnelEndpoint.String(),
			TunnelID:       msg.Session.TunnelID,
			ErrorCode:      msg.ErrorSpec.ErrorCode,
			ErrorValue:     msg.ErrorSpec.ErrorValue,
			ErrorNode:      msg.ErrorSpec.ErrorNode.String(),
		}
		if _, err := PathErr.Emit(eb, evt); err != nil {
			e.log.Warn("rsvp-te: path-err emit failed", "error", err)
		}
	}
	e.log.Info("rsvp-te: PathErr received", "tunnel", msg.Session.TunnelEndpoint,
		"code", msg.ErrorSpec.ErrorCode, "value", msg.ErrorSpec.ErrorValue)
}

func (e *engine) handlePathTear(msg *ParsedMessage) {
	if !msg.HasSession || !msg.HasSenderTemplate {
		return
	}
	key := keyFromMessage(msg)
	lsp := e.table.Remove(key)
	if lsp == nil {
		return
	}
	// Snapshot fields under the LSP lock; a refresh/show goroutine may still
	// hold this pointer until it next consults the table.
	lsp.mu.Lock()
	role := lsp.Role
	inLabel := lsp.InLabel
	psb := lsp.PSB
	nextHop := lsp.NextHop
	bandwidth := lsp.Bandwidth
	admIface := lsp.AdmissionIface
	lsp.mu.Unlock()

	if admIface != "" {
		e.admission.ReleaseSession(admIface, sessionFromKey(key), float64(bandwidth))
	}
	if e.fib != nil {
		switch role {
		case RoleTransit:
			if inLabel != 0 {
				if err := e.fib.RemoveSwap(inLabel); err != nil {
					e.log.Warn("rsvp-te: fib remove swap failed", "lsp", key.String(), "error", err)
				}
			}
		case RoleIngress:
			fec := netip.PrefixFrom(key.TunnelEndpoint, key.TunnelEndpoint.BitLen())
			if err := e.fib.Remove(fec); err != nil {
				e.log.Warn("rsvp-te: fib remove failed", "lsp", key.String(), "error", err)
			}
		case RoleEgress:
			// Egress programmed a pop (in-label keyed); withdraw it.
			if inLabel != 0 {
				if err := e.fib.RemoveSwap(inLabel); err != nil {
					e.log.Warn("rsvp-te: fib remove pop failed", "lsp", key.String(), "error", err)
				}
			}
		}
	}
	// Relay the teardown downstream so the rest of the LSP is cleared.
	if role == RoleTransit && psb != nil && nextHop.IsValid() {
		raw := buildPathTear(psb, e.cfg.RouterID)
		if err := e.transport.Send(nextHop, raw); err != nil {
			e.log.Warn("rsvp-te: transit PathTear relay failed", "next-hop", nextHop, "error", err)
		}
	}
	e.table.releaseLabel(inLabel)
	emitLSPDown(e.log, lsp, e.table.Len())
	e.log.Info("rsvp-te: PathTear processed", "lsp", key.String())
}

func (e *engine) sendPathErr(dst netip.Addr, msg *ParsedMessage, code uint8, value uint16) {
	es := errorSpec{ErrorNode: e.cfg.RouterID, ErrorCode: code, ErrorValue: value}
	raw := buildPathErr(msg.Session, msg.SenderTemplate, msg.SenderTSpec, es, e.cfg.RouterID, defaultIPTTL)
	if err := e.transport.Send(dst, raw); err != nil {
		e.log.Warn("rsvp-te: send PathErr failed", "dest", dst, "error", err)
	}
}

// handleLinkDown reacts to interface ifaceName going down. Every LSP whose
// bandwidth was reserved against that interface has lost its downstream path, so
// RSVP-TE reports the failure toward the head-end -- a transit/egress node sends
// a PathErr upstream, an ingress head-end emits a local path-err event -- and
// tears the local state. RFC 2205 Section 3.1.3 (PathErr toward the sender);
// RFC 3209 Section 4.3.5 error code 24 "Routing Problem" value 5 "No route
// available toward destination".
//
// It runs from the interface-event subscription goroutine; the LSP table,
// per-LSP locks and admission controller are all concurrency-safe (the cleanup
// loop already mutates them off the engine goroutine).
func (e *engine) handleLinkDown(ifaceName string) {
	if ifaceName == "" {
		return
	}
	es := errorSpec{ErrorNode: e.cfg.RouterID, ErrorCode: ErrCodeRoutingProblem, ErrorValue: ErrValueNoRouteAvailable}
	for _, lsp := range e.table.All() {
		lsp.mu.Lock()
		nextHop := lsp.NextHop
		state := lsp.State
		key := lsp.Key
		role := lsp.Role
		prevHop := lsp.PrevHop
		psb := lsp.PSB
		lsp.mu.Unlock()
		// An LSP is affected when the interface toward its DOWNSTREAM next hop is
		// the one that failed. Match on that link, not on AdmissionIface: the
		// reservation is charged against the upstream (PHOP) interface, and an
		// ingress LSP never sets AdmissionIface at all, so matching AdmissionIface
		// targets the wrong link (or no link) for the failure (F7).
		//
		// Egress LSPs have no NextHop (the tail disposes to IP, not to an LSP hop),
		// so they only match here via the single-interface admissionInterface
		// shortcut. Detecting an egress-side link failure precisely would need the
		// inner FEC's IP-FIB egress interface, which RSVP-TE does not track; the
		// upstream learns of an egress that stops refreshing via soft-state expiry.
		iface, ok := e.admissionInterface(nextHop)
		if state == LSPStateDown || !ok || iface != ifaceName {
			continue
		}

		switch role {
		case RoleTransit, RoleEgress:
			if psb != nil && prevHop.IsValid() {
				raw := buildPathErr(psb.Session, psb.SenderTemplate, psb.SenderTSpec, es, e.cfg.RouterID, defaultIPTTL)
				if err := e.transport.Send(prevHop, raw); err != nil {
					e.log.Warn("rsvp-te: link-down PathErr send failed", "lsp", key.String(), "iface", ifaceName, "error", err)
				}
			}
		case RoleIngress:
			e.emitLocalPathErr(key, es)
		}
		e.tearLSPLocal(key)
		e.log.Info("rsvp-te: LSP torn down on link failure", "lsp", key.String(), "iface", ifaceName)
	}
}

// emitLocalPathErr publishes a path-err event for an LSP whose failure was
// detected at the head-end itself, where there is no upstream node to receive a
// wire PathErr.
func (e *engine) emitLocalPathErr(key lspKey, es errorSpec) {
	eb := getEventBus()
	if eb == nil {
		return
	}
	evt := &PathErrEvent{
		TunnelEndpoint: key.TunnelEndpoint.String(),
		TunnelID:       key.TunnelID,
		ErrorCode:      es.ErrorCode,
		ErrorValue:     es.ErrorValue,
		ErrorNode:      es.ErrorNode.String(),
	}
	if _, err := PathErr.Emit(eb, evt); err != nil {
		e.log.Warn("rsvp-te: local path-err emit failed", "error", err)
	}
}

// tearLSPLocal removes an LSP and its forwarding/admission state without relaying
// a teardown (the local link to it failed, so there is nothing to relay over). It
// mirrors the cleanup half of handlePathTear.
func (e *engine) tearLSPLocal(key lspKey) {
	lsp := e.table.Remove(key)
	if lsp == nil {
		return
	}
	lsp.mu.Lock()
	role := lsp.Role
	inLabel := lsp.InLabel
	bandwidth := lsp.Bandwidth
	admIface := lsp.AdmissionIface
	lsp.mu.Unlock()

	if admIface != "" {
		e.admission.ReleaseSession(admIface, sessionFromKey(key), float64(bandwidth))
	}
	if e.fib != nil {
		switch role {
		case RoleTransit, RoleEgress:
			if inLabel != 0 {
				if err := e.fib.RemoveSwap(inLabel); err != nil {
					e.log.Warn("rsvp-te: fib remove failed on link-down", "lsp", key.String(), "error", err)
				}
			}
		case RoleIngress:
			fec := netip.PrefixFrom(key.TunnelEndpoint, key.TunnelEndpoint.BitLen())
			if err := e.fib.Remove(fec); err != nil {
				e.log.Warn("rsvp-te: fib remove failed on link-down", "lsp", key.String(), "error", err)
			}
		}
	}
	e.table.releaseLabel(inLabel)
	emitLSPDown(e.log, lsp, e.table.Len())
}

// admissionInterface resolves the interface to account a reservation against,
// given the LSP's neighbor on this node's link. With a single configured
// interface it is unambiguous. With several, the neighbor is matched against
// each interface's configured local prefix (the `address` leaf): the interface
// whose prefix contains the neighbor owns the link the LSP traverses here. If no
// interface matches (no prefixes configured, or neighbor outside them) admission
// accounting is skipped for this LSP rather than charged to the wrong link.
func (e *engine) admissionInterface(neighbor netip.Addr) (string, bool) {
	if len(e.cfg.Interfaces) == 1 {
		return e.cfg.Interfaces[0].Name, true
	}
	if !neighbor.IsValid() {
		return "", false
	}
	for _, ic := range e.cfg.Interfaces {
		if ic.Prefix.IsValid() && ic.Prefix.Contains(neighbor) {
			return ic.Name, true
		}
	}
	return "", false
}
