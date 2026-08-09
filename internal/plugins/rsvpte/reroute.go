// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- make-before-break reroute (AC-7)
// RFC: rfc/short/rfc3209.md
// RFC: rfc/short/rfc2205.md
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
		RefreshPeriod:  e.cfg().RefreshPeriod,
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

// teardownLSP tears down an ingress LSP this node head-ends: it sends a PathTear
// toward the egress so transit and egress nodes clear their state hop-by-hop (RFC
// 2205 Section 3.1.5), and releases the LSP's push entry, admission bandwidth and
// label. It is the head-end mirror of handlePathTear's relay-and-clean, used both
// to retire the old LSP after a make-before-break reroute (RFC 3209 Section 6.1)
// and to remove a tunnel deleted from configuration.
func (e *engine) teardownLSP(key lspKey) {
	lsp := e.table.Remove(key)
	if lsp == nil {
		return
	}
	// Snapshot the removed LSP's fields under its lock; a refresh or show
	// goroutine may still hold a reference until it next consults the table.
	lsp.mu.Lock()
	psb := lsp.PSB
	dst := lsp.NextHop
	bandwidth := lsp.Bandwidth
	inLabel := lsp.InLabel
	admIface := lsp.AdmissionIface
	isBypass := lsp.IsBypass
	lsp.mu.Unlock()

	if psb != nil {
		raw := buildPathTear(psb, e.cfg().RouterID)
		if !dst.IsValid() {
			dst = key.TunnelEndpoint
		}
		if err := e.transport.Send(dst, raw); err != nil {
			e.log.Warn("rsvp-te: head-end PathTear send failed", "lsp", key.String(), "error", err)
		}
	}
	if admIface != "" {
		e.admission.ReleaseSession(admIface, sessionFromKey(key), float64(bandwidth))
	}
	if e.fib != nil {
		fec := netip.PrefixFrom(key.TunnelEndpoint, key.TunnelEndpoint.BitLen())
		if err := e.fib.removePush(fec); err != nil {
			e.log.Warn("rsvp-te: head-end fib remove failed", "lsp", key.String(), "error", err)
		}
	}
	e.table.releaseLabel(inLabel)
	if isBypass {
		e.clearBypassReferences(key)
	}
	emitLSPDown(e.log, lsp, e.table.Len())
	e.log.Info("rsvp-te: ingress LSP torn down", "lsp", key.String())
}
