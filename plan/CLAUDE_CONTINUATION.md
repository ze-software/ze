# Claude Continuation State

**Last Updated:** 2025-12-21

---

## CURRENT PRIORITY

**Implement ALIGN items from `plan/align-implementation.md`**

31 items across 8 phases (26 original + 5 from RFC violations)

Start with Phase 1 (Critical): 5 items

---

## ACTIVE WORK

### ExaBGP Alignment Implementation

Progress: Phase 6 complete (26/31 items)

**Completed Phases:**
- ✅ Phase 1: Critical (5/5)
- ✅ Phase 2: Capabilities (3/3)
- ✅ Phase 3: Timers (1/1)
- ✅ Phase 4: Attributes (6/6)
- ✅ Phase 5: MP-NLRI (4/4)
- ✅ Phase 6: NLRI Types (7/7)

**Next:** Phase 8 (Errors - 1 item) and Phase 9 (Config - 4 items)

### Recent Implementation:
- **6.1 EVPN Type 1 (Ethernet Auto-Discovery) - RFC 7432 §7.1:**
  - Added `EVPNType1` struct with RD, ESI, EthernetTag, Labels
- **6.2 EVPN Type 4 (Ethernet Segment) - RFC 7432 §7.4:**
  - Added `EVPNType4` struct with RD, ESI, OriginatorIP
- **6.3-6.5 FlowSpec VPN, VPLS, RTC:** Already implemented
- **6.6 EVPN Type 5 Prefix Encoding - RFC 9136 §3.1:**
  - Fixed `parseEVPNType5()` to require length 34 (IPv4) or 58 (IPv6)
- **6.7 BGP-LS Descriptor Encoding - RFC 7752 §3.2:**
  - Fixed `BGPLSLink.Bytes()` and `BGPLSPrefix.Bytes()` to not wrap TLVs
  - Link/prefix descriptor TLVs now appear directly in NLRI per RFC

---

## RECENT COMMITS

- `4503f2b` Add ExaBGP comparison report and improve session protocols
- `73c8a8d` Add ASN4-aware AS_PATH encoding based on capability negotiation

---

## TEST STATUS

✅ **All 1048 tests pass** (`make test`)

### Lint Issues (40 pre-existing)

- `exhaustive` - missing switch cases
- `goconst` - repeated string literals
- `gocritic` - ifElseChain patterns
- `godot` - comment formatting
- `gosec` - integer overflow warnings
- `prealloc` - slice pre-allocation
- `unused` - unused functions

These are pre-existing and not blocking.

---

## KEY FILES

| Purpose | File |
|---------|------|
| Alignment plan | `plan/exabgp-alignment.md` |
| Comparison report | `.claude/zebgp/EXABGP_COMPARISON_REPORT.md` |
| This file | `plan/CLAUDE_CONTINUATION.md` |
| Protocols | `.claude/ESSENTIAL_PROTOCOLS.md` |
| TDD rules | `.claude/TDD_ENFORCEMENT.md` |

---

## NOTES

- All code changes require TDD (test first, show failure, implement, show pass)
- Plans go in `plan/`, protocols go in `.claude/`
- Check ExaBGP reference before implementing BGP features
- **ALWAYS run `make test` at the end of work BEFORE requesting a commit**
