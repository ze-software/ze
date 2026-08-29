// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE signaling engine
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
	"sync/atomic"
	"time"
)

// fibProgrammer programs MPLS forwarding entries for an LSP. The production
// implementation emits to sysrib; tests use a fake. It is a seam so the engine
// is testable without the kernel and so the (separate) dataplane work can land
// behind a stable interface.
type fibProgrammer interface {
	// programPush installs an ingress push entry: traffic to fec is forwarded
	// to nextHop with label pushed.
	programPush(fec netip.Prefix, label uint32, nextHop netip.Addr) error
	// programSwap installs a transit swap entry: packets arriving with inLabel
	// are forwarded to nextHop with inLabel swapped for outLabel.
	programSwap(inLabel, outLabel uint32, nextHop netip.Addr) error
	// programBackup installs a facility-backup local-repair entry (RFC 4090
	// Section 3.2): packets arriving with inLabel are forwarded to nextHop with
	// the outLabels stack imposed (the bypass label over the swapped protected
	// label). The kernel AF_MPLS swap already accepts a multi-label stack.
	programBackup(inLabel uint32, outLabels []uint32, nextHop netip.Addr) error
	// programPop installs an egress pop entry: packets arriving with inLabel are
	// stripped of the label and forwarded toward nextHop (disposition).
	programPop(inLabel uint32, nextHop netip.Addr) error
	// removePush withdraws a previously programmed push entry for fec.
	removePush(fec netip.Prefix) error
	// removeSwap withdraws a previously programmed swap or pop entry for inLabel.
	removeSwap(inLabel uint32) error
}

// engine runs the RSVP-TE control plane over a Transport. Per-LSP state is
// guarded by each LSP's own mutex (see LSP.mu), so the engine itself is
// stateless beyond its immutable dependencies.
type engine struct {
	transport Transport
	table     *lspTable
	admission *admissionController
	fib       fibProgrammer
	// cfgPtr holds the active config behind an atomic pointer so OnConfigApply can
	// swap in a reloaded config (new interfaces, bypasses, refresh period) while the
	// run loop reads it. Read it via e.cfg(); never store the struct directly.
	cfgPtr atomic.Pointer[rsvpteConfig]
	log    *slog.Logger
}

// cfg returns the current engine config (atomically loaded; reload-safe). It
// copies the small struct (slice headers, not backing arrays), so snapshot it once
// per handler when reading several fields.
func (e *engine) cfg() rsvpteConfig { return *e.cfgPtr.Load() }

// setConfig swaps in a reloaded config (called from OnConfigApply) so the engine's
// runtime reads (selectBypass, admissionInterface, message builders) see it.
// RouterID is the LSR identity, fixed at engine creation (the engine is only built
// once a valid router-id exists); a changed/removed router-id is NOT adopted at
// runtime, since the engine's As4-based key and message-encode reads would then
// panic on the zero Addr. Changing the router-id is a restart-class operation.
func (e *engine) setConfig(cfg rsvpteConfig) {
	cfg.RouterID = e.cfg().RouterID
	e.cfgPtr.Store(&cfg)
}

func newEngine(t Transport, table *lspTable, adm *admissionController, fib fibProgrammer, cfg rsvpteConfig, log *slog.Logger) *engine {
	e := &engine{transport: t, table: table, admission: adm, fib: fib, log: log}
	e.cfgPtr.Store(&cfg)
	return e
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
	if msg.HasUnknownObject {
		e.rejectUnknownObject(pkt.Src, msg)
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
	raw := buildPath(lsp.PSB, e.cfg().RouterID, defaultIPTTL)
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
	raw := buildResv(lsp.RSB, lsp.PSB.SenderTemplate, e.cfg().RefreshPeriod, e.cfg().RouterID)
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
	if msg.Session.TunnelEndpoint == e.cfg().RouterID {
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

// receivedRefreshPeriod returns the refresh period that sets the lifetime of the
// state this PATH creates. RFC 2205 Section 3.7: "Each Path or Resv message carries
// a TIME_VALUES object containing the refresh time R used to generate refreshes. The
// recipient node uses this R to determine the lifetime L of the stored state created
// or refreshed by the message." So the lifetime follows the SENDER's period, never
// this node's configured one: the sender refreshes on its own schedule, and a local
// refresh-period commit that shortened the lifetime of state a neighbor keeps alive
// would delete a live reservation on the next cleanup tick.
//
// TIME_VALUES is mandatory in a PATH (RFC 2205 Section 3.1.3), so a message without
// one falls back to the suggested default of 30 seconds rather than to the local
// period. The value is clamped to maxRefreshPeriod so a neighbor cannot advertise a
// period that keeps ze's state alive for years.
func receivedRefreshPeriod(msg *ParsedMessage) time.Duration {
	if !msg.HasTimeValues || msg.TimeValues.RefreshPeriod == 0 {
		return DefaultRefreshPeriod
	}
	period := time.Duration(msg.TimeValues.RefreshPeriod) * time.Millisecond
	if period > maxRefreshPeriod {
		return maxRefreshPeriod
	}
	return period
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
	// RFC 4090: carry the protection request so the egress records its label too
	// (label recording, needed by an upstream PLR for node-protection backups).
	egressProtection := protectionFromPath(msg)
	var egressRecordLabel uint32
	if egressProtection != nil {
		egressRecordLabel = label
	}
	lsp.PSB = &pathStateBlock{
		Session:        msg.Session,
		SenderTemplate: msg.SenderTemplate,
		SenderTSpec:    msg.SenderTSpec,
		LabelRequest:   msg.LabelRequest,
		RefreshPeriod:  receivedRefreshPeriod(msg),
		LastRefresh:    time.Now(),
		Protection:     egressProtection,
	}
	lsp.RSB = &resvStateBlock{
		Session:  msg.Session,
		FlowSpec: msg.SenderTSpec,
		Label:    labelObject{Label: label},
		Style:    StyleSharedExplicit,
		// RFC 3209 Section 4.4: the egress records its own address as the first
		// RRO entry; upstream nodes prepend themselves as the RESV travels back.
		RRO:         e.recordRoute(nil, msg.Session.TunnelID, 0, egressRecordLabel),
		LastRefresh: time.Now(),
	}
	lsp.setState(LSPStateUp)
	lsp.mu.Unlock()

	raw := buildResv(lsp.RSB, msg.SenderTemplate, e.cfg().RefreshPeriod, e.cfg().RouterID)
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
			if err := e.fib.programPop(label, netip.Addr{}); err != nil {
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

	// RFC 4090: if the head-end requested local protection, this transit node is a
	// candidate PLR. Reconstruct the request and arm a configured bypass whose
	// merge point is the protected NHOP (link) or NNHOP (node protection). Arming
	// just records the association; the data-plane switch happens on link-down.
	pr := protectionFromPath(msg)
	var bypass *lspKey
	if pr != nil {
		if bk, sel := e.selectBypass(rem, pr); sel {
			bypass = &bk
		}
	}

	lsp.mu.Lock()
	lsp.Role = RoleTransit
	lsp.PrevHop = src
	lsp.NextHop = nextHop
	lsp.Reserved = true
	lsp.AdmissionIface = admIface
	lsp.Bandwidth = msg.SenderTSpec.TokenRate
	lsp.Bypass = bypass
	lsp.PSB = &pathStateBlock{
		Session:        msg.Session,
		SenderTemplate: msg.SenderTemplate,
		Hop:            rsvpHop{NextHop: src}, // PHOP: where RESV is sent back to
		ERO:            rem,
		SenderTSpec:    msg.SenderTSpec,
		LabelRequest:   msg.LabelRequest,
		RefreshPeriod:  receivedRefreshPeriod(msg),
		LastRefresh:    time.Now(),
		Protection:     pr,
	}
	lsp.setState(LSPStatePathReceived)
	lsp.mu.Unlock()

	if bypass != nil && !alreadyReserved {
		e.log.Info("rsvp-te: PLR armed bypass for protected LSP", "lsp", key.String(), "bypass", bypass.String())
	}

	// Relay the PATH downstream with a decremented TTL. The ERO is forwarded
	// unchanged (rem still begins with the next hop) so the next node strips
	// itself in turn. The protection request is relayed so downstream transits
	// also become candidate PLRs.
	// TIME_VALUES states the period at which the state this PATH creates downstream
	// will actually be refreshed (RFC 2205 Section 3.7), and a transit node relays a
	// PATH only when one arrives: the downstream cadence is the sender's period, not
	// this node's. Writing the local period here would let a refresh-period commit on
	// a transit node shrink the downstream node's cleanup timeout below the rate the
	// relays actually arrive at, and the downstream would delete a live reservation.
	// This node's own period governs the RESVs it generates upstream, which it does
	// refresh on that cadence (buildResv reads e.cfg().RefreshPeriod).
	fwd := &pathStateBlock{
		Session:        msg.Session,
		SenderTemplate: msg.SenderTemplate,
		ERO:            rem,
		SenderTSpec:    msg.SenderTSpec,
		LabelRequest:   msg.LabelRequest,
		RefreshPeriod:  receivedRefreshPeriod(msg),
		Protection:     pr,
	}
	raw := buildPath(fwd, e.cfg().RouterID, msg.Header.TTL-1)
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
	for len(rem) > 0 && rem[0].Address.Addr() == e.cfg().RouterID {
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
// selfFlags sets the RFC 4090 protection flags on this node's RRO subobject (0 at
// the head-end and egress; a PLR reports available/in-use/node protection).
// recordLabel, when non-zero, records this node's label right after its address
// (RFC 3209 Section 4.4.3 label recording), so an upstream PLR can learn the merge
// point's label for node-protection backup forwarding.
func (e *engine) recordRoute(downstream []rroEntry, tunnelID uint16, selfFlags uint8, recordLabel uint32) []rroEntry {
	rro, truncated := prependRRO(e.cfg().RouterID, downstream)
	if truncated {
		e.log.Warn("rsvp-te: recorded route truncated at hop limit; possible routing loop",
			"limit", maxRecordRouteHops, "tunnel", tunnelID)
	}
	if len(rro) == 0 || rro[0].Type != RROSubIPv4 {
		return rro
	}
	if selfFlags != 0 {
		rro[0].Flags = selfFlags
	}
	if recordLabel != 0 {
		withLabel := make([]rroEntry, 0, len(rro)+1)
		withLabel = append(withLabel, rro[0], rroEntry{Type: RROSubLabel, Label: recordLabel})
		withLabel = append(withLabel, rro[1:]...)
		rro = withLabel
	}
	return rro
}

// labelForAddr finds the label an address recorded in an RRO (the RROSubLabel
// subobject immediately following its address subobject, RFC 3209 Section 4.4.3).
// Returns 0 / false when the address is absent or recorded no label.
func labelForAddr(rro []rroEntry, addr netip.Addr) (uint32, bool) {
	for i := range rro {
		if rro[i].Type == RROSubIPv4 && rro[i].Address == addr {
			if i+1 < len(rro) && rro[i+1].Type == RROSubLabel {
				return rro[i+1].Label, true
			}
			return 0, false
		}
	}
	return 0, false
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
		RRO:         e.recordRoute(msg.RRO, msg.Session.TunnelID, 0, 0),
		LastRefresh: time.Now(),
	}
	lsp.setState(LSPStateUp)
	replaces := lsp.Replaces
	lsp.Replaces = nil
	lsp.mu.Unlock()

	if e.fib != nil {
		fec := netip.PrefixFrom(key.TunnelEndpoint, key.TunnelEndpoint.BitLen())
		if err := e.fib.programPush(fec, msg.Label.Label, src); err != nil {
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
	lsp.mu.Lock()
	if lsp.PSB == nil {
		lsp.mu.Unlock()
		e.log.Warn("rsvp-te: transit RESV without path state", "lsp", key.String())
		return
	}
	inLabel := lsp.InLabel
	lsp.mu.Unlock()
	// Allocate the local in-label OUTSIDE the LSP lock: AllocateLabel takes the
	// table mutex, so holding lsp.mu across it would invert the table->lsp lock
	// order used by the cleanup walk (expiredPSBs). handleResvTransit runs on the
	// single receive goroutine, so no other writer of InLabel races here.
	allocated := inLabel == 0
	if allocated {
		inLabel = e.table.AllocateLabel()
	}

	lsp.mu.Lock()
	if lsp.PSB == nil {
		// Torn down between the unlock and the re-lock (defensive; the receive
		// goroutine is serial). Release the label we just allocated.
		lsp.mu.Unlock()
		if allocated {
			e.table.releaseLabel(inLabel)
		}
		return
	}
	lsp.InLabel = inLabel
	lsp.OutLabel = msg.Label.Label
	lsp.NextHop = src
	// RFC 4090 Section 6.4.2: compute the inner label the PLR pushes under the
	// bypass label on a local repair, and record this node's label so an upstream
	// PLR can do the same. For link protection the merge point is the NHOP, which
	// expects the label it advertised (msg.Label). For node protection the merge
	// point is the NNHOP, whose label is recorded in the received RESV RRO.
	recordLabel := uint32(0)
	if lsp.PSB.Protection != nil {
		recordLabel = inLabel
		if lsp.PSB.Protection.NodeProtection {
			// RFC 4090 Section 6.4.2: node protection needs the NNHOP's own label
			// (the merge point expects it, not the NHOP's). Leave BackupLabel 0
			// ("unresolved") unless the NNHOP recorded its label in the RESV RRO, so
			// a re-arm on a later PATH refresh cannot reintroduce a wrong-label
			// repair (tryLocalRepair refuses when BackupLabel is 0).
			lsp.BackupLabel = 0
			var nnhopLabel uint32
			var ok bool
			if len(lsp.PSB.ERO) >= 2 {
				nnhopLabel, ok = labelForAddr(msg.RRO, lsp.PSB.ERO[1].Address.Addr())
			}
			if ok {
				lsp.BackupLabel = nnhopLabel
			} else if lsp.Bypass != nil {
				// The NNHOP did not record its label (a peer ignoring
				// label-recording-desired). Disarm rather than blackhole.
				e.log.Warn("rsvp-te: node protection requested but the NNHOP label is unavailable; disarming bypass",
					"lsp", key.String())
				lsp.Bypass = nil
			}
		} else {
			lsp.BackupLabel = msg.Label.Label // link protection: the NHOP's advertised label
		}
	}
	// RFC 4090 Section 4.4: a PLR reports its protection state (available / in use /
	// node) in its own RRO subobject so the head-end can see the LSP is protected.
	protFlags := rroProtectionFlags(lsp)
	lsp.RSB = &resvStateBlock{
		Session:  msg.Session,
		FlowSpec: msg.FlowSpec,
		Label:    labelObject{Label: inLabel},
		Style:    msg.Style,
		// RFC 3209 Section 4.4: record this node ahead of the downstream route
		// before relaying the RESV upstream.
		RRO:         e.recordRoute(msg.RRO, msg.Session.TunnelID, protFlags, recordLabel),
		LastRefresh: time.Now(),
	}
	prevHop := lsp.PSB.Hop.NextHop
	filter := lsp.PSB.SenderTemplate
	// Build the RESV under the lock (like sendResv/sendPath): reading lsp.RSB after
	// the unlock would race the refresh goroutine's LastRefresh write into it.
	raw := buildResv(lsp.RSB, filter, e.cfg().RefreshPeriod, e.cfg().RouterID)
	lsp.setState(LSPStateUp)
	lsp.mu.Unlock()

	if e.fib != nil {
		if err := e.fib.programSwap(inLabel, msg.Label.Label, src); err != nil {
			e.log.Warn("rsvp-te: program swap failed", "lsp", key.String(), "error", err)
		}
	}

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
		evt := &pathErrEvent{
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
	// RFC 4090 Section 6.5: a Notify ("Tunnel locally repaired") from a PLR means
	// this LSP is now riding its bypass. The head-end re-optimizes onto a fresh
	// path (make-before-break) and lets the old, locally-repaired LSP be torn down
	// once the replacement is up.
	if msg.HasSenderTemplate && msg.ErrorSpec.ErrorCode == ErrCodeNotify &&
		msg.ErrorSpec.ErrorValue == ErrValueTunnelLocallyRepaired {
		e.reoptimizeOnNotify(keyFromMessage(msg))
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
	isBypass := lsp.IsBypass
	lsp.mu.Unlock()

	if admIface != "" {
		e.admission.ReleaseSession(admIface, sessionFromKey(key), float64(bandwidth))
	}
	if e.fib != nil {
		switch role {
		case RoleTransit:
			if inLabel != 0 {
				if err := e.fib.removeSwap(inLabel); err != nil {
					e.log.Warn("rsvp-te: fib remove swap failed", "lsp", key.String(), "error", err)
				}
			}
		case RoleIngress:
			fec := netip.PrefixFrom(key.TunnelEndpoint, key.TunnelEndpoint.BitLen())
			if err := e.fib.removePush(fec); err != nil {
				e.log.Warn("rsvp-te: fib remove failed", "lsp", key.String(), "error", err)
			}
		case RoleEgress:
			// Egress programmed a pop (in-label keyed); withdraw it.
			if inLabel != 0 {
				if err := e.fib.removeSwap(inLabel); err != nil {
					e.log.Warn("rsvp-te: fib remove pop failed", "lsp", key.String(), "error", err)
				}
			}
		}
	}
	// Relay the teardown downstream so the rest of the LSP is cleared.
	if role == RoleTransit && psb != nil && nextHop.IsValid() {
		raw := buildPathTear(psb, e.cfg().RouterID)
		if err := e.transport.Send(nextHop, raw); err != nil {
			e.log.Warn("rsvp-te: transit PathTear relay failed", "next-hop", nextHop, "error", err)
		}
	}
	e.table.releaseLabel(inLabel)
	// Defensive: a bypass is an ingress LSP this PLR head-ends and should not
	// receive a PathTear for its own session, but if one ever removes a bypass,
	// clear the stale association on protected LSPs that armed it.
	if isBypass {
		e.clearBypassReferences(key)
	}
	emitLSPDown(e.log, lsp, e.table.Len())
	e.log.Info("rsvp-te: PathTear processed", "lsp", key.String())
}

// rejectUnknownObject answers a message that carried an object class ze does not
// implement and whose Class-Num high-order bit is zero. RFC 2205 Section 3.10
// rejects the whole message, and Appendix B returns Error Code 13 with the
// object's (Class-Num, C-Type) as the Error Value. Nothing in the message is
// acted on: handlePacket calls this instead of dispatching on the message type.
//
// RFC 4090 Section 4.2 is the case that matters today. A Path carrying a DETOUR
// object (Class-Num 63) reaches an LSR with no one-to-one backup support, and the
// PathErr is what tells the PLR its detour LSP is not established. Without it the
// PLR believes it has a working detour that nothing on this node will ever use.
//
// Only a Path is answered. ze builds no ResvErr, so a Resv, a Tear, or a PathErr
// carrying such an object is dropped with a log line and no error message.
func (e *engine) rejectUnknownObject(src netip.Addr, msg *ParsedMessage) {
	obj := msg.UnknownObject
	e.log.Warn("rsvp-te: message rejected, unknown object class",
		"src", src, "msg-type", msg.Header.MsgType, "class-num", obj.ClassNum, "c-type", obj.CType)
	if msg.Header.MsgType != MsgTypePath {
		return
	}
	if !msg.HasSession || !msg.HasSenderTemplate {
		e.log.Warn("rsvp-te: cannot report unknown object class, PATH missing SESSION/SENDER_TEMPLATE", "src", src)
		return
	}
	value := uint16(obj.ClassNum)<<8 | uint16(obj.CType)
	e.sendPathErr(src, msg, ErrCodeUnknownObjectClass, value)
}

func (e *engine) sendPathErr(dst netip.Addr, msg *ParsedMessage, code uint8, value uint16) {
	es := errorSpec{ErrorNode: e.cfg().RouterID, ErrorCode: code, ErrorValue: value}
	raw := buildPathErr(msg.Session, msg.SenderTemplate, msg.SenderTSpec, es, e.cfg().RouterID)
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
	es := errorSpec{ErrorNode: e.cfg().RouterID, ErrorCode: ErrCodeRoutingProblem, ErrorValue: ErrValueNoRouteAvailable}
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

		// RFC 4090 Section 6.5: a protected transit LSP with a ready bypass is
		// locally repaired -- traffic is redirected onto the bypass and the
		// head-end is notified (PathErr Notify, code 25/3) -- instead of being torn
		// down. The protected LSP keeps forwarding via the bypass until the
		// head-end re-optimizes.
		if role == RoleTransit && e.tryLocalRepair(lsp, key) {
			if psb != nil && prevHop.IsValid() {
				nes := errorSpec{ErrorNode: e.cfg().RouterID, ErrorCode: ErrCodeNotify, ErrorValue: ErrValueTunnelLocallyRepaired}
				raw := buildPathErr(psb.Session, psb.SenderTemplate, psb.SenderTSpec, nes, e.cfg().RouterID)
				if err := e.transport.Send(prevHop, raw); err != nil {
					e.log.Warn("rsvp-te: local-repair Notify send failed", "lsp", key.String(), "iface", ifaceName, "error", err)
				}
			}
			e.log.Info("rsvp-te: LSP locally repaired on link failure", "lsp", key.String(), "iface", ifaceName)
			continue
		}

		switch role {
		case RoleTransit, RoleEgress:
			if psb != nil && prevHop.IsValid() {
				raw := buildPathErr(psb.Session, psb.SenderTemplate, psb.SenderTSpec, es, e.cfg().RouterID)
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
	evt := &pathErrEvent{
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
	isBypass := lsp.IsBypass
	lsp.mu.Unlock()

	if admIface != "" {
		e.admission.ReleaseSession(admIface, sessionFromKey(key), float64(bandwidth))
	}
	if e.fib != nil {
		switch role {
		case RoleTransit, RoleEgress:
			if inLabel != 0 {
				if err := e.fib.removeSwap(inLabel); err != nil {
					e.log.Warn("rsvp-te: fib remove failed on link-down", "lsp", key.String(), "error", err)
				}
			}
		case RoleIngress:
			fec := netip.PrefixFrom(key.TunnelEndpoint, key.TunnelEndpoint.BitLen())
			if err := e.fib.removePush(fec); err != nil {
				e.log.Warn("rsvp-te: fib remove failed on link-down", "lsp", key.String(), "error", err)
			}
		}
	}
	e.table.releaseLabel(inLabel)
	// RFC 4090: if a bypass LSP is gone, protected LSPs that armed it no longer have
	// a backup -- clear their stale association so they stop reporting protection.
	if isBypass {
		e.clearBypassReferences(key)
	}
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
	if len(e.cfg().Interfaces) == 1 {
		return e.cfg().Interfaces[0].Name, true
	}
	if !neighbor.IsValid() {
		return "", false
	}
	for _, ic := range e.cfg().Interfaces {
		if ic.Prefix.IsValid() && ic.Prefix.Contains(neighbor) {
			return ic.Name, true
		}
	}
	return "", false
}
