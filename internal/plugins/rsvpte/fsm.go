// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE per-LSP state machine
// RFC: rfc/short/rfc2205.md
// RFC: rfc/short/rfc3209.md
// RFC: rfc/short/rfc4090.md
// Related: wire.go -- decoded messages drive state transitions
// Related: frr.go -- protectionRequest carried on the PSB (RFC 4090)
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

// lspState represents the current state of an LSP.
type lspState uint8

const (
	LSPStateDown lspState = iota
	LSPStatePathSent
	LSPStatePathReceived
	LSPStateResvSent
	LSPStateResvReceived
	LSPStateUp
)

func (s lspState) String() string {
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

// lspRole identifies this node's role for a given LSP.
type lspRole uint8

const (
	RoleIngress lspRole = iota
	RoleTransit
	RoleEgress
)

func (r lspRole) String() string {
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

// lspKey uniquely identifies an LSP.
type lspKey struct {
	TunnelEndpoint netip.Addr
	TunnelID       uint16
	ExtTunnelID    uint32
	SenderAddr     netip.Addr
	LSPID          uint16
}

func (k lspKey) String() string {
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

// pathStateBlock (PSB) stores PATH state for an LSP (RFC 2205 Section 2.1).
type pathStateBlock struct {
	Session        sessionIPv4
	SenderTemplate senderTemplateIPv4
	Hop            rsvpHop
	ERO            []eroHop
	SenderTSpec    FlowSpec
	LabelRequest   labelRequest
	RefreshPeriod  time.Duration
	LastRefresh    time.Time
	// Protection, when set, requests RFC 4090 local protection for this LSP: PATH
	// then carries SESSION_ATTRIBUTE (protection-desired flags) and FAST_REROUTE.
	// A transit node fills it from the received PATH (protectionFromPath).
	Protection *protectionRequest
}

// resvStateBlock (RSB) stores RESV state for an LSP (RFC 2205 Section 2.1).
type resvStateBlock struct {
	Session  sessionIPv4
	FlowSpec FlowSpec
	Label    labelObject
	Style    uint32
	Hop      rsvpHop
	RRO      []rroEntry

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

	Key   lspKey
	State lspState
	Role  lspRole

	PSB *pathStateBlock
	RSB *resvStateBlock

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
	Replaces *lspKey

	// Reserved records whether admission control has already committed this
	// LSP's bandwidth, so PATH refreshes (RFC 2205 soft-state) do not reserve
	// it again on every refresh interval.
	Reserved bool

	// AdmissionIface is the interface this LSP's bandwidth was reserved against,
	// resolved once at reserve time so release charges the same link (empty when
	// admission was skipped because no interface could be resolved).
	AdmissionIface string

	// RFC 4090 Fast Reroute. On a transit PLR, Bypass names the bypass LSP armed
	// to protect this LSP (nil = no backup); ProtectionInUse is set once a local
	// repair has redirected traffic onto that bypass. IsBypass marks an LSP that
	// is itself a configured facility-backup bypass (PLR-sourced), so it is not
	// treated as a protected tunnel. BackupLabel is the inner label the PLR pushes
	// under the bypass label on local repair: the merge point's label for the
	// protected LSP (the next hop's swapped label for link protection, or the
	// next-next hop's recorded label for node protection, RFC 4090 Section 6.4.2).
	Bypass          *lspKey
	ProtectionInUse bool
	IsBypass        bool
	BackupLabel     uint32

	CreatedAt   time.Time
	LastChanged time.Time
}

// firstDynamicLabel is the lowest label handed out; 0-15 are reserved
// (RFC 3032) and 16-999 are left for static/other allocators.
const firstDynamicLabel = 1000

// lspTable manages all LSPs at this node. Thread-safe.
type lspTable struct {
	mu   sync.RWMutex
	lsps map[lspKey]*LSP

	nextLabel uint32
	freed     []uint32 // labels returned by ReleaseLabel, reused before nextLabel
}

// newLSPTable creates an empty LSP table.
func newLSPTable() *lspTable {
	return &lspTable{
		lsps:      make(map[lspKey]*LSP),
		nextLabel: firstDynamicLabel,
	}
}

// AllocateLabel returns a local label, preferring labels returned to the free
// list so wraparound cannot collide with a label that is still in use.
func (t *lspTable) AllocateLabel() uint32 {
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

// releaseLabel returns a label to the free list for reuse. A zero label (never
// allocated) is ignored.
func (t *lspTable) releaseLabel(label uint32) {
	if label == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.freed = append(t.freed, label)
}

// GetOrCreate returns the LSP for the given key, creating it if absent.
func (t *lspTable) GetOrCreate(key lspKey) (*LSP, bool) {
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
func (t *lspTable) Get(key lspKey) (*LSP, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	lsp, ok := t.lsps[key]
	return lsp, ok
}

// Remove deletes an LSP.
func (t *lspTable) Remove(key lspKey) *LSP {
	t.mu.Lock()
	defer t.mu.Unlock()
	lsp, ok := t.lsps[key]
	if ok {
		delete(t.lsps, key)
	}
	return lsp
}

// All returns a snapshot of all LSPs.
func (t *lspTable) All() []*LSP {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]*LSP, 0, len(t.lsps))
	for _, lsp := range t.lsps {
		out = append(out, lsp)
	}
	return out
}

// Len returns the number of LSPs.
func (t *lspTable) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.lsps)
}

// expiredPSBs returns LSP keys whose PATH state has expired.
func (t *lspTable) expiredPSBs(now time.Time, factor int) []lspKey {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var expired []lspKey
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

// setState updates the LSP state with a timestamp. The caller must hold lsp.mu.
func (lsp *LSP) setState(s lspState) {
	lsp.State = s
	lsp.LastChanged = time.Now()
}

// keyFromMessage extracts an LSPKey from a parsed RSVP message.
func keyFromMessage(msg *ParsedMessage) lspKey {
	return lspKey{
		TunnelEndpoint: msg.Session.TunnelEndpoint,
		TunnelID:       msg.Session.TunnelID,
		ExtTunnelID:    msg.Session.ExtTunnelID,
		SenderAddr:     msg.SenderTemplate.SenderAddr,
		LSPID:          msg.SenderTemplate.LSPID,
	}
}
