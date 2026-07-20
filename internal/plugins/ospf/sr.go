// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing IPv4 origination
// and reception coordinator. SR contributes TLV builders to the RFC 7770 RI LSA
// (SR-Algorithm/SRGB/SRLB/SRMS) and the RFC 7684 Extended Prefix/Link Opaque LSAs
// (Prefix-SID / Adj-SID / LAN-Adj-SID); the registration lives in register_sr.go.
// Because the builders receive only a router-ID / link context, this node's SR
// config is held in srWire and consulted per router; an unconfigured router
// contributes no TLVs (plugin self-containment). Remote capability reception reads
// RI LSA bodies, which carry no per-TLV callback context.
// RFC: rfc/short/rfc8665.md (§3 RI TLVs, §5 Prefix-SID, §6 Adj-SID)

package ospf

import (
	"strconv"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/sr"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// srWire holds this node's configured SR state, consulted by the RI and Extended
// Prefix/Link TLV builders (which are handed only a router ID / link context).
var srWire = newSRWireStore()

// srWireStore is the per-router SR origination state. caps holds each local
// router's EFFECTIVE (shared) SR config -- the RFC 8666 §4 shared RI capabilities
// (SRGB/SRLB/algorithms), preferring the IPv4 block, else the IPv6 block; v6caps
// holds the IPv6 family's own config (its IPv6 node Prefix-SIDs) when BOTH address
// families configure SR, so the IPv6 origination reads its own prefixes rather than
// the IPv4 block's (each AF advertises only its own family's prefixes). adj holds the
// Adj-SIDs allocated per adjacency (keyed by the RFC 2328 link data), populated by the
// adjacency lifecycle.
type srWireStore struct {
	mu     sync.RWMutex
	caps   map[types.RouterID]sr.SRConfig
	v6caps map[types.RouterID]sr.SRConfig
	adj    map[types.RouterID]map[[4]byte]sr.AdjSID
}

func newSRWireStore() *srWireStore {
	return &srWireStore{
		caps:   make(map[types.RouterID]sr.SRConfig),
		v6caps: make(map[types.RouterID]sr.SRConfig),
		adj:    make(map[types.RouterID]map[[4]byte]sr.AdjSID),
	}
}

// set stores the effective/shared config for a router (used by config apply and by
// tests). Clearing SR (a disabled config) removes both the shared and the v6-specific
// entry so a reload cannot leave a stale IPv6 override behind.
func (s *srWireStore) set(router types.RouterID, cfg sr.SRConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cfg.Enabled {
		s.caps[router] = cfg
	} else {
		delete(s.caps, router)
		delete(s.v6caps, router)
	}
}

// setV6 stores the IPv6 family's own SR config as an override consulted by getAF for
// the IPv6 family. A disabled config removes the override (the IPv6 family then falls
// back to the shared config).
func (s *srWireStore) setV6(router types.RouterID, cfg sr.SRConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cfg.Enabled {
		s.v6caps[router] = cfg
	} else {
		delete(s.v6caps, router)
	}
}

func (s *srWireStore) get(router types.RouterID) (sr.SRConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.caps[router]
	return c, ok
}

// getAF returns the SR config the given address family must originate from: the IPv6
// family reads its own config when one was stored (both families configured SR), else
// the shared config (only one family configured SR, RFC 8666 §4 shared capabilities).
// The IPv4 family always reads the shared config (IPv4-preferred).
func (s *srWireStore) getAF(router types.RouterID, isV6 bool) (sr.SRConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if isV6 {
		if c, ok := s.v6caps[router]; ok {
			return c, true
		}
	}
	c, ok := s.caps[router]
	return c, ok
}

func (s *srWireStore) setAdj(router types.RouterID, linkData [4]byte, a sr.AdjSID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.adj[router]
	if m == nil {
		m = make(map[[4]byte]sr.AdjSID)
		s.adj[router] = m
	}
	m[linkData] = a
}

func (s *srWireStore) clearAdj(router types.RouterID, linkData [4]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.adj[router]; m != nil {
		delete(m, linkData)
	}
}

func (s *srWireStore) adjFor(router types.RouterID, linkData [4]byte) (sr.AdjSID, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m := s.adj[router]; m != nil {
		a, ok := m[linkData]
		return a, ok
	}
	return sr.AdjSID{}, false
}

// adjList returns every Adj-SID allocated for the router (snapshot / show).
func (s *srWireStore) adjList(router types.RouterID) []sr.AdjSID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.adj[router]
	if len(m) == 0 {
		return nil
	}
	out := make([]sr.AdjSID, 0, len(m))
	for _, a := range m {
		out = append(out, a)
	}
	return out
}

// clear resets the store (test helper + config reload).
func (s *srWireStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.caps = make(map[types.RouterID]sr.SRConfig)
	s.v6caps = make(map[types.RouterID]sr.SRConfig)
	s.adj = make(map[types.RouterID]map[[4]byte]sr.AdjSID)
}

// ---- RI capability TLV builders (RFC 8665 §3; RFC 8666 §4 reuses them for v6) ----

func srBuildAlgorithm(router types.RouterID) []packet.RITLV {
	if _, ok := srWire.get(router); !ok {
		return nil
	}
	// RFC 8665 §3.1: when advertised, Algorithm 0 (SPF) MUST be included.
	return []packet.RITLV{{Type: sr.V4TypeSRAlgorithm, Value: sr.EncodeAlgorithmValue([]uint8{0})}}
}

func srBuildSRGB(router types.RouterID) []packet.RITLV {
	cfg, ok := srWire.get(router)
	if !ok {
		return nil
	}
	// RFC 8665 §3.2: each range is a separate TLV, emitted in configured order.
	out := make([]packet.RITLV, 0, len(cfg.SRGB))
	for _, r := range cfg.SRGB {
		out = append(out, packet.RITLV{Type: sr.V4TypeSRGB, Value: sr.EncodeRangeValue(r)})
	}
	return out
}

func srBuildSRLB(router types.RouterID) []packet.RITLV {
	cfg, ok := srWire.get(router)
	if !ok || len(cfg.SRLB) == 0 {
		return nil
	}
	out := make([]packet.RITLV, 0, len(cfg.SRLB))
	for _, r := range cfg.SRLB {
		out = append(out, packet.RITLV{Type: sr.V4TypeSRLB, Value: sr.EncodeRangeValue(r)})
	}
	return out
}

func srBuildSRMS(router types.RouterID) []packet.RITLV {
	cfg, ok := srWire.get(router)
	if !ok || !cfg.HasSRMS {
		return nil
	}
	return []packet.RITLV{{Type: sr.V4TypeSRMS, Value: sr.EncodeSRMSValue(cfg.SRMSPreference)}}
}

// ---- Extended Prefix / Link sub-TLV builders ----

func srBuildPrefixSID(ctx extSubTLVContext) []packet.ExtSubTLV {
	cfg, ok := srWire.get(ctx.Router)
	if !ok {
		return nil
	}
	for _, p := range cfg.Prefixes {
		if p.Prefix == ctx.Prefix {
			ps := sr.PrefixSID{
				Flags:     sr.SIDFlags{NP: p.NoPHP, E: p.ExplicitNull},
				Algorithm: 0,
				Index:     p.Index,
			}
			return []packet.ExtSubTLV{{Type: sr.V4TypePrefixSID, Value: sr.EncodePrefixSIDValue(ps)}}
		}
	}
	return nil
}

func srBuildAdjSID(ctx extSubTLVContext) []packet.ExtSubTLV {
	a, ok := srWire.adjFor(ctx.Router, ctx.LinkData)
	if !ok || a.IsLAN {
		return nil
	}
	return []packet.ExtSubTLV{{Type: sr.V4TypeAdjSID, Value: sr.EncodeAdjSIDValue(a)}}
}

func srBuildLANAdjSID(ctx extSubTLVContext) []packet.ExtSubTLV {
	a, ok := srWire.adjFor(ctx.Router, ctx.LinkData)
	if !ok || !a.IsLAN {
		return nil
	}
	return []packet.ExtSubTLV{{Type: sr.V4TypeLANAdjSID, Value: sr.EncodeLANAdjSIDValue(a)}}
}

// ---- Reception validation (the ext-4 Receive callback carries no LSA context;
// install-relevant reception reads the LSDB, see srDecodeRemoteCapabilities) ----

func srReceivePrefixSID(value []byte) {
	// A malformed sub-TLV is ignored and counted; decoding validates the V/L
	// combination and length (RFC 8665 §5). The ext-4 dispatch recover-wraps the call.
	if _, err := sr.DecodePrefixSIDValue(value); err != nil {
		srMetrics.Load().observeMalformed("ipv4", "prefix-sid")
	}
}

func srReceiveAdjSID(value []byte) {
	if _, err := sr.DecodeAdjSIDValue(value); err != nil {
		srMetrics.Load().observeMalformed("ipv4", "adj-sid")
	}
}

func srReceiveLANAdjSID(value []byte) {
	if _, err := sr.DecodeLANAdjSIDValue(value); err != nil {
		srMetrics.Load().observeMalformed("ipv4", "lan-adj-sid")
	}
}

// ---- Render (show ospf database opaque-area) ----

func srRenderPrefixSID(value []byte) string {
	p, err := sr.DecodePrefixSIDValue(value)
	if err != nil {
		return ""
	}
	b := []byte("Prefix-SID ")
	if p.IsLabel {
		b = append(b, "label="...)
		b = strconv.AppendUint(b, uint64(p.Label), 10)
	} else {
		b = append(b, "index="...)
		b = strconv.AppendUint(b, uint64(p.Index), 10)
	}
	b = append(b, " algo="...)
	b = strconv.AppendUint(b, uint64(p.Algorithm), 10)
	return string(b)
}

func srRenderAdjSID(value []byte) string {
	a, err := sr.DecodeAdjSIDValue(value)
	if err != nil {
		return ""
	}
	return srRenderAdj("Adj-SID", a)
}

func srRenderLANAdjSID(value []byte) string {
	a, err := sr.DecodeLANAdjSIDValue(value)
	if err != nil {
		return ""
	}
	return srRenderAdj("LAN-Adj-SID", a)
}

func srRenderAdj(kind string, a sr.AdjSID) string {
	b := []byte(kind)
	if a.IsLabel {
		b = append(b, " label="...)
		b = strconv.AppendUint(b, uint64(a.Label), 10)
	} else {
		b = append(b, " index="...)
		b = strconv.AppendUint(b, uint64(a.Index), 10)
	}
	return string(b)
}

// ---- Remote capability reception (RI LSA body decode) ----

// srRemoteCapabilities is a peer's SR capabilities decoded from its RI LSA: the
// advertised algorithms and the ordered SRGB/SRLB (RFC 8665 §3.1/§3.2).
type srRemoteCapabilities struct {
	Algorithms []uint8
	SRGB       sr.SRGB
	SRLB       sr.SRGB
	HasSRMS    bool
	SRMSPref   uint8
}

// srDecodeRemoteCapabilities extracts SR capabilities from an RI LSA body for the given
// address family. Ranges are concatenated in advertised order (RFC 8665 §3.2). A
// malformed range TLV (truncated, or a reserved/out-of-space label range rejected by the
// RFC 8665 §10 / RFC 8666 §11 receive hardening) is skipped and counted, not fatal, so
// one bad range does not discard the rest nor source a reserved label.
func srDecodeRemoteCapabilities(af string, body []byte) srRemoteCapabilities {
	var caps srRemoteCapabilities
	tlvs, err := packet.DecodeRITLVStream(body)
	if err != nil {
		return caps
	}
	var srgb, srlb []sr.LabelRange
	for _, tlv := range tlvs {
		switch tlv.Type {
		case sr.V4TypeSRAlgorithm:
			// RFC 8665 §3.1: when a router advertises more than one SR-Algorithm TLV, the
			// FIRST occurrence in the RI Opaque LSA is used and the rest are ignored.
			if caps.Algorithms != nil {
				continue
			}
			if algos, aerr := sr.DecodeAlgorithmValue(tlv.Value); aerr == nil {
				caps.Algorithms = algos
			}
		case sr.V4TypeSRGB:
			if r, rerr := sr.DecodeRangeValue(tlv.Value); rerr == nil {
				srgb = append(srgb, r)
			} else {
				srMetrics.Load().observeMalformed(af, "srgb")
			}
		case sr.V4TypeSRLB:
			if r, rerr := sr.DecodeRangeValue(tlv.Value); rerr == nil {
				srlb = append(srlb, r)
			} else {
				srMetrics.Load().observeMalformed(af, "srlb")
			}
		case sr.V4TypeSRMS:
			// RFC 8665 §3.4: the FIRST SRMS Preference TLV occurrence in the RI Opaque LSA
			// is used; subsequent instances are ignored.
			if caps.HasSRMS {
				continue
			}
			if pref, perr := sr.DecodeSRMSValue(tlv.Value); perr == nil {
				caps.HasSRMS = true
				caps.SRMSPref = pref
			}
		}
	}
	caps.SRGB = sr.NewSRGB(srgb)
	caps.SRLB = sr.NewSRGB(srlb)
	return caps
}
