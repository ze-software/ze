// Design: plan/spec-mpls-3-rsvp-te.md -- RSVP-TE signaling engine
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
	table     *LSPTable
	admission *AdmissionController
	fib       fibProgrammer
	cfg       rsvpteConfig
	log       *slog.Logger
}

func newEngine(t Transport, table *LSPTable, adm *AdmissionController, fib fibProgrammer, cfg rsvpteConfig, log *slog.Logger) *engine {
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
	raw := BuildPath(lsp.PSB, e.cfg.RouterID, defaultIPTTL)
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
	if err := e.admission.ReserveSession(iface, sessionFromIPv4(msg.Session), float64(msg.SenderTSpec.TokenRate)); err != nil {
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
	key := KeyFromMessage(msg)
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
	lsp.PSB = &PathStateBlock{
		Session:        msg.Session,
		SenderTemplate: msg.SenderTemplate,
		SenderTSpec:    msg.SenderTSpec,
		LabelRequest:   msg.LabelRequest,
		RefreshPeriod:  e.cfg.RefreshPeriod,
		LastRefresh:    time.Now(),
	}
	lsp.RSB = &ResvStateBlock{
		Session:     msg.Session,
		FlowSpec:    msg.SenderTSpec,
		Label:       LabelObject{Label: label},
		Style:       StyleSharedExplicit,
		LastRefresh: time.Now(),
	}
	lsp.SetState(LSPStateUp)
	lsp.mu.Unlock()

	raw := BuildResv(lsp.RSB, msg.SenderTemplate, e.cfg.RefreshPeriod, e.cfg.RouterID, defaultIPTTL)
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

	key := KeyFromMessage(msg)
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
	lsp.PSB = &PathStateBlock{
		Session:        msg.Session,
		SenderTemplate: msg.SenderTemplate,
		Hop:            RSVPHop{NextHop: src}, // PHOP: where RESV is sent back to
		ERO:            rem,
		SenderTSpec:    msg.SenderTSpec,
		LabelRequest:   msg.LabelRequest,
		RefreshPeriod:  e.cfg.RefreshPeriod,
		LastRefresh:    time.Now(),
	}
	lsp.SetState(LSPStatePathReceived)
	lsp.mu.Unlock()

	// Relay the PATH downstream with a decremented TTL. The ERO is forwarded
	// unchanged (rem still begins with the next hop) so the next node strips
	// itself in turn.
	fwd := &PathStateBlock{
		Session:        msg.Session,
		SenderTemplate: msg.SenderTemplate,
		ERO:            rem,
		SenderTSpec:    msg.SenderTSpec,
		LabelRequest:   msg.LabelRequest,
		RefreshPeriod:  e.cfg.RefreshPeriod,
	}
	raw := BuildPath(fwd, e.cfg.RouterID, msg.Header.TTL-1)
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
func (e *engine) nextHopFromERO(ero []EROHop) (netip.Addr, []EROHop, bool) {
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
	key := KeyFromMessage(msg)
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

func (e *engine) handleResvIngress(src netip.Addr, msg *ParsedMessage, lsp *LSP, key LSPKey) {
	lsp.mu.Lock()
	lsp.OutLabel = msg.Label.Label
	lsp.NextHop = src
	lsp.SetState(LSPStateUp)
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
		e.tearReplaced(*replaces)
	}
}

// handleResvTransit allocates this node's incoming label, programs the swap from
// that label to the downstream label, and relays a RESV upstream carrying the
// local label so the upstream node swaps to it.
func (e *engine) handleResvTransit(src netip.Addr, msg *ParsedMessage, lsp *LSP, key LSPKey) {
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
	lsp.RSB = &ResvStateBlock{
		Session:     msg.Session,
		FlowSpec:    msg.FlowSpec,
		Label:       LabelObject{Label: inLabel},
		Style:       msg.Style,
		LastRefresh: time.Now(),
	}
	prevHop := lsp.PSB.Hop.NextHop
	filter := lsp.PSB.SenderTemplate
	lsp.SetState(LSPStateUp)
	lsp.mu.Unlock()

	if e.fib != nil {
		if err := e.fib.ProgramSwap(inLabel, msg.Label.Label, src); err != nil {
			e.log.Warn("rsvp-te: program swap failed", "lsp", key.String(), "error", err)
		}
	}

	raw := BuildResv(lsp.RSB, filter, e.cfg.RefreshPeriod, e.cfg.RouterID, defaultIPTTL)
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
	key := KeyFromMessage(msg)
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
		raw := BuildPathTear(psb, e.cfg.RouterID, defaultIPTTL)
		if err := e.transport.Send(nextHop, raw); err != nil {
			e.log.Warn("rsvp-te: transit PathTear relay failed", "next-hop", nextHop, "error", err)
		}
	}
	e.table.ReleaseLabel(inLabel)
	emitLSPDown(e.log, lsp, e.table.Len())
	e.log.Info("rsvp-te: PathTear processed", "lsp", key.String())
}

func (e *engine) sendPathErr(dst netip.Addr, msg *ParsedMessage, code uint8, value uint16) {
	es := ErrorSpec{ErrorNode: e.cfg.RouterID, ErrorCode: code, ErrorValue: value}
	raw := BuildPathErr(msg.Session, msg.SenderTemplate, msg.SenderTSpec, es, e.cfg.RouterID, defaultIPTTL)
	if err := e.transport.Send(dst, raw); err != nil {
		e.log.Warn("rsvp-te: send PathErr failed", "dest", dst, "error", err)
	}
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
