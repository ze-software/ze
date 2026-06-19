// Design: plan/spec-mpls-3-rsvp-te.md -- make-before-break reroute (AC-7)
// Related: engine.go -- sends the new PATH and tears the old LSP once up
// Related: fsm.go -- LSP.Replaces links the new LSP to the one it supersedes
//
// RFC 3209 Section 6.1 (make-before-break): to reroute an LSP without dropping
// traffic, the ingress signals a NEW LSP for the same SESSION using a fresh
// LSP_ID in the SENDER_TEMPLATE and the SHARED EXPLICIT (SE) reservation style.
// SE lets the new and old LSPs share bandwidth on common links so admission
// control does not double-count the reservation. Once the new LSP comes up
// (its RESV arrives) the old LSP is torn down.
package rsvpte

import (
	"net/netip"
	"time"
)

// reroute starts a make-before-break reroute of the ingress LSP identified by
// oldKey along newERO. It creates a replacement LSP with the next LSP_ID and the
// SE style, sends its PATH, and records the link so handleResv tears the old LSP
// down once the replacement is up. Returns the new LSP key.
func (e *engine) reroute(oldKey lspKey, newERO []eroHop) (lspKey, bool) {
	old, ok := e.table.Get(oldKey)
	if !ok {
		return lspKey{}, false
	}

	// Snapshot the old LSP's parameters under its lock.
	old.mu.Lock()
	if old.Role != RoleIngress {
		old.mu.Unlock()
		return lspKey{}, false
	}
	bandwidth := old.Bandwidth
	setupPrio := old.SetupPriority
	holdPrio := old.HoldPriority
	var tspec FlowSpec
	if old.PSB != nil {
		tspec = old.PSB.SenderTSpec
	}
	old.mu.Unlock()

	newKey := oldKey
	newKey.LSPID = oldKey.LSPID + 1
	newLSP, _ := e.table.GetOrCreate(newKey)

	newLSP.mu.Lock()
	newLSP.Role = RoleIngress
	newLSP.Bandwidth = bandwidth
	newLSP.SetupPriority = setupPrio
	newLSP.HoldPriority = holdPrio
	replaced := oldKey
	newLSP.Replaces = &replaced
	newLSP.PSB = &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: newKey.TunnelEndpoint, TunnelID: newKey.TunnelID, ExtTunnelID: newKey.ExtTunnelID},
		SenderTemplate: senderTemplateIPv4{SenderAddr: newKey.SenderAddr, LSPID: newKey.LSPID},
		ERO:            newERO,
		SenderTSpec:    tspec,
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  e.cfg.RefreshPeriod,
		LastRefresh:    time.Now(),
	}
	newLSP.setState(LSPStatePathSent)
	newLSP.mu.Unlock()

	if err := e.sendPath(newLSP); err != nil {
		e.log.Warn("rsvp-te: make-before-break PATH send failed", "lsp", newKey.String(), "error", err)
		return newKey, false
	}
	e.log.Info("rsvp-te: make-before-break started", "old", oldKey.String(), "new", newKey.String())
	return newKey, true
}

// tearReplaced tears down the LSP a make-before-break replacement supersedes,
// sending a PathTear toward its egress and releasing its admission bandwidth.
func (e *engine) tearReplaced(oldKey lspKey) {
	old := e.table.Remove(oldKey)
	if old == nil {
		return
	}
	// Snapshot the removed LSP's fields under its lock; a refresh or show
	// goroutine may still hold a reference until it next consults the table.
	old.mu.Lock()
	psb := old.PSB
	dst := old.NextHop
	bandwidth := old.Bandwidth
	inLabel := old.InLabel
	admIface := old.AdmissionIface
	old.mu.Unlock()

	if psb != nil {
		raw := buildPathTear(psb, e.cfg.RouterID)
		if !dst.IsValid() {
			dst = oldKey.TunnelEndpoint
		}
		if err := e.transport.Send(dst, raw); err != nil {
			e.log.Warn("rsvp-te: make-before-break old PathTear failed", "lsp", oldKey.String(), "error", err)
		}
	}
	if admIface != "" {
		e.admission.ReleaseSession(admIface, sessionFromKey(oldKey), float64(bandwidth))
	}
	if e.fib != nil {
		fec := netip.PrefixFrom(oldKey.TunnelEndpoint, oldKey.TunnelEndpoint.BitLen())
		if err := e.fib.Remove(fec); err != nil {
			e.log.Warn("rsvp-te: make-before-break old fib remove failed", "lsp", oldKey.String(), "error", err)
		}
	}
	e.table.releaseLabel(inLabel)
	emitLSPDown(e.log, old, e.table.Len())
	e.log.Info("rsvp-te: make-before-break old LSP torn down", "lsp", oldKey.String())
}
