// Design: plan/spec-mpls-3-rsvp-te.md -- RSVP-TE per-LSP state machine
// Related: wire.go -- decoded messages drive state transitions
//
// RFC 3209 Section 2: RSVP-TE uses soft-state with PATH/RESV refresh.
// PATH flows downstream (ingress to egress), RESV flows upstream.
// Each LSP is identified by (SESSION, SENDER_TEMPLATE).
package rsvpte

import (
	"net/netip"
	"strconv"
	"sync"
	"time"
)

// LSPState represents the current state of an LSP.
type LSPState uint8

const (
	LSPStateDown LSPState = iota
	LSPStatePathSent
	LSPStatePathReceived
	LSPStateResvSent
	LSPStateResvReceived
	LSPStateUp
)

func (s LSPState) String() string {
	switch s {
	case LSPStateDown:
		return "down"
	case LSPStatePathSent:
		return "path-sent"
	case LSPStatePathReceived:
		return "path-received"
	case LSPStateResvSent:
		return "resv-sent"
	case LSPStateResvReceived:
		return "resv-received"
	case LSPStateUp:
		return "up"
	default:
		return "unknown"
	}
}

// LSPRole identifies this node's role for a given LSP.
type LSPRole uint8

const (
	RoleIngress LSPRole = iota
	RoleTransit
	RoleEgress
)

func (r LSPRole) String() string {
	switch r {
	case RoleIngress:
		return "ingress"
	case RoleTransit:
		return "transit"
	case RoleEgress:
		return "egress"
	default:
		return "unknown"
	}
}

// LSPKey uniquely identifies an LSP.
type LSPKey struct {
	TunnelEndpoint netip.Addr
	TunnelID       uint16
	ExtTunnelID    uint32
	SenderAddr     netip.Addr
	LSPID          uint16
}

func (k LSPKey) String() string {
	var buf []byte
	buf = append(buf, k.TunnelEndpoint.String()...)
	buf = append(buf, '/')
	buf = strconv.AppendUint(buf, uint64(k.TunnelID), 10)
	buf = append(buf, '/')
	buf = strconv.AppendUint(buf, uint64(k.ExtTunnelID), 10)
	buf = append(buf, '/')
	buf = append(buf, k.SenderAddr.String()...)
	buf = append(buf, '/')
	buf = strconv.AppendUint(buf, uint64(k.LSPID), 10)
	return string(buf)
}

// PathStateBlock (PSB) stores PATH state for an LSP (RFC 2205 Section 2.1).
type PathStateBlock struct {
	Session        SessionIPv4
	SenderTemplate SenderTemplateIPv4
	Hop            RSVPHop
	ERO            []EROHop
	SenderTSpec    FlowSpec
	LabelRequest   LabelRequest
	RefreshPeriod  time.Duration
	LastRefresh    time.Time
}

// ResvStateBlock (RSB) stores RESV state for an LSP (RFC 2205 Section 2.1).
type ResvStateBlock struct {
	Session  SessionIPv4
	FlowSpec FlowSpec
	Label    LabelObject
	Style    uint32
	Hop      RSVPHop
	RRO      []RROEntry

	LastRefresh time.Time
}

// LSP tracks the full state of one LSP at this node.
//
// mu guards every mutable field below. The signaling engine, the refresh and
// cleanup loops, and the show commands all run on different goroutines and
// touch the same LSP, so each holds mu around its reads/writes. Hold mu only
// for field access; never across a transport send or event emit.
type LSP struct {
	mu sync.Mutex

	Key   LSPKey
	State LSPState
	Role  LSPRole

	PSB *PathStateBlock
	RSB *ResvStateBlock

	InLabel  uint32
	OutLabel uint32
	NextHop  netip.Addr
	PrevHop  netip.Addr

	Bandwidth float32

	SetupPriority uint8
	HoldPriority  uint8

	// Replaces, when set on a make-before-break LSP, names the older LSP this
	// one supersedes. RFC 3209 Section 6.1: the old LSP is torn down only once
	// the replacement is up, so traffic is never dropped during a reroute.
	Replaces *LSPKey

	// Reserved records whether admission control has already committed this
	// LSP's bandwidth, so PATH refreshes (RFC 2205 soft-state) do not reserve
	// it again on every refresh interval.
	Reserved bool

	// AdmissionIface is the interface this LSP's bandwidth was reserved against,
	// resolved once at reserve time so release charges the same link (empty when
	// admission was skipped because no interface could be resolved).
	AdmissionIface string

	CreatedAt   time.Time
	LastChanged time.Time
}

// firstDynamicLabel is the lowest label handed out; 0-15 are reserved
// (RFC 3032) and 16-999 are left for static/other allocators.
const firstDynamicLabel = 1000

// LSPTable manages all LSPs at this node. Thread-safe.
type LSPTable struct {
	mu   sync.RWMutex
	lsps map[LSPKey]*LSP

	nextLabel uint32
	freed     []uint32 // labels returned by ReleaseLabel, reused before nextLabel
}

// NewLSPTable creates an empty LSP table.
func NewLSPTable() *LSPTable {
	return &LSPTable{
		lsps:      make(map[LSPKey]*LSP),
		nextLabel: firstDynamicLabel,
	}
}

// AllocateLabel returns a local label, preferring labels returned to the free
// list so wraparound cannot collide with a label that is still in use.
func (t *LSPTable) AllocateLabel() uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if n := len(t.freed); n > 0 {
		label := t.freed[n-1]
		t.freed = t.freed[:n-1]
		return label
	}
	label := t.nextLabel
	t.nextLabel++
	if t.nextLabel > MaxLabel {
		t.nextLabel = firstDynamicLabel
	}
	return label
}

// ReleaseLabel returns a label to the free list for reuse. A zero label (never
// allocated) is ignored.
func (t *LSPTable) ReleaseLabel(label uint32) {
	if label == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.freed = append(t.freed, label)
}

// GetOrCreate returns the LSP for the given key, creating it if absent.
func (t *LSPTable) GetOrCreate(key LSPKey) (*LSP, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	lsp, exists := t.lsps[key]
	if !exists {
		now := time.Now()
		lsp = &LSP{
			Key:         key,
			State:       LSPStateDown,
			CreatedAt:   now,
			LastChanged: now,
		}
		t.lsps[key] = lsp
	}
	return lsp, exists
}

// Get returns the LSP for the given key.
func (t *LSPTable) Get(key LSPKey) (*LSP, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	lsp, ok := t.lsps[key]
	return lsp, ok
}

// Remove deletes an LSP.
func (t *LSPTable) Remove(key LSPKey) *LSP {
	t.mu.Lock()
	defer t.mu.Unlock()
	lsp, ok := t.lsps[key]
	if ok {
		delete(t.lsps, key)
	}
	return lsp
}

// All returns a snapshot of all LSPs.
func (t *LSPTable) All() []*LSP {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]*LSP, 0, len(t.lsps))
	for _, lsp := range t.lsps {
		out = append(out, lsp)
	}
	return out
}

// Len returns the number of LSPs.
func (t *LSPTable) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.lsps)
}

// ExpiredPSBs returns LSP keys whose PATH state has expired.
func (t *LSPTable) ExpiredPSBs(now time.Time, factor int) []LSPKey {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var expired []LSPKey
	for key, lsp := range t.lsps {
		lsp.mu.Lock()
		if lsp.PSB == nil {
			lsp.mu.Unlock()
			continue
		}
		deadline := lsp.PSB.LastRefresh.Add(lsp.PSB.RefreshPeriod * time.Duration(factor))
		lsp.mu.Unlock()
		if now.After(deadline) {
			expired = append(expired, key)
		}
	}
	return expired
}

// SetState updates the LSP state with a timestamp. The caller must hold lsp.mu.
func (lsp *LSP) SetState(s LSPState) {
	lsp.State = s
	lsp.LastChanged = time.Now()
}

// KeyFromMessage extracts an LSPKey from a parsed RSVP message.
func KeyFromMessage(msg *ParsedMessage) LSPKey {
	return LSPKey{
		TunnelEndpoint: msg.Session.TunnelEndpoint,
		TunnelID:       msg.Session.TunnelID,
		ExtTunnelID:    msg.Session.ExtTunnelID,
		SenderAddr:     msg.SenderTemplate.SenderAddr,
		LSPID:          msg.SenderTemplate.LSPID,
	}
}
