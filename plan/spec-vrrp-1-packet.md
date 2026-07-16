# Spec: vrrp-1 -- VRRP Packet Codec (v2/v3 Encode, Decode, Receive Validation)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 9/9 (implementation complete; awaiting orchestrator Review Gate + closure) |
| Updated | 2026-07-14 |

Child 1 of `plan/spec-vrrp-0-umbrella.md`. Siblings: `spec-vrrp-2-fsm.md`,
`spec-vrrp-3-macvlan.md`, `spec-vrrp-4-transport.md`, `spec-vrrp-5-plugin.md`,
`spec-vrrp-6-interop.md`, `spec-vrrp-7-vpp.md`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/spec-vrrp-0-umbrella.md` -- user decisions, R-1/R-2, child scope table
3. `.claude/rules/planning.md` -- workflow rules
4. `rfc/short/rfc9568.md` (Wire Formats, Validation, Errata), `rfc/short/rfc3768.md` (same)
5. `internal/component/bfd/packet/control.go` -- in-repo codec style model
6. `ai/rules/buffer-first.md` -- WriteTo(buf, off) int contract

## Task

Implement the VRRP packet codec as a pure-Go package `internal/plugins/vrrp/packet`:
encode and decode RFC 9568 (VRRPv3, IPv4+IPv6) and RFC 3768 (VRRPv2, IPv4-only)
ADVERTISEMENT messages, with the full ordered receive-validation ladder, a typed
error taxonomy that maps 1:1 to `ze_vrrp_packet_errors_total{reason}` labels, an
IHL-aware IPv4 header-strip helper, golden byte vectors, negative tests derived
from verified reference-implementation bugs (holo-vrrp, uvrrpd), boundary tests,
a fuzz target wired into `make ze-fuzz-test`, and a zero-allocation happy-path
decode.

No sockets, no build tags, no netlink, no goroutines: the package is consumed by
the FSM (spec-vrrp-2), the engine (spec-vrrp-5, the Decode caller per cross-spec
decision D-B) and the transport (spec-vrrp-4, strip helper + tx buffer fill).
Internal time unit is
MILLISECONDS everywhere (umbrella R-2); conversion to wire units (v3 centiseconds,
v2 whole seconds) happens ONLY inside encode/decode.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/buffer-first.md` - encoding contract
  → Constraint: encode is `WriteTo(buf, off) int` into a caller/pool buffer; no `append`, no `make([]byte)` in encode helpers; checksum backfill at `off+6` uses the skip-and-backfill pattern; `make` stays legal in pool New, tests, and result copies
  → Decision: decode-side zero-allocation is a goal of this spec (buffer-first.md governs encode; the decode target is adopted here explicitly, see Key Design Decisions)
- [ ] `ai/rules/module-tiers.md` - package placement
  → Decision: `internal/plugins/vrrp/packet` is a leaf subpackage of the vrrp edge plugin; imports stdlib (+ `net/netip`) only; only vrrp sibling packages may import it
- [ ] `ai/rules/plugin-self-containment.md` - no central spelling
  → Constraint: protocol number 112, multicast groups, virtual-MAC prefixes are defined locally in this package; nothing is added to `internal/component/firewall/protocol.go` or any central registry; metrics counters are NOT registered here (children 4/5 register them inside the plugin) -- this package only exports the error→reason mapping
- [ ] `internal/component/bfd/packet/control.go` - in-repo style model for a control-plane protocol codec
  → Decision: copy the BFD codec shape: value-type message struct, `WriteTo(buf, off) int` (control.go:94), allocation-free parse returning (value, consumed, error) (control.go:144), package-level typed `Err*` sentinels (control.go:68-77), ordered validation ladder after field extraction (control.go:171-195)
- [ ] `internal/component/bfd/packet/fuzz_test.go` + `control_test.go` + `pool.go` - test and pool patterns
  → Constraint: fuzz target asserts accepted-packet invariants and round-trips (fuzz_test.go:15); round-trip table tests (control_test.go:12); pool sized to protocol max (pool.go:26) -- vrrp exports MaxLen sizing constants from this package; the transport (child 4) uses a fixed per-instance tx buffer sized from them (no pool, per spec-vrrp-4's design)
- [ ] `mk/test-fuzz.mk` - fuzz target wiring
  → Constraint: each fuzz target is enumerated individually (`-fuzz=FuzzName` per line, mk/test-fuzz.mk:12); the bfd packet entries at mk/test-fuzz.mk:59-60 are the pattern to follow for `FuzzDecode`

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9568.md` - VRRPv3 (Wire Formats, Validation, Encoding/Decoding Rules, Constants, Errata)
  → Constraint: v3 header 8 bytes; Max Advertise Interval = 12-bit centiseconds 1..4095 (erratum 8301, zero MUST be discarded); Reserve nibble zero on tx / ignored on rx; count 0 MUST be ignored (erratum 8299); first IPv6 VIP MUST be link-local (erratum 8300); TTL/hop-limit MUST be 255 on rx; interval mismatch is NOT a discard (backup adopts, §6.4.2)
  → Decision: RFC 9568 §5.2.8 computes the v3/IPv4 checksum WITHOUT a pseudo-header (rfc/full/rfc9568.txt:880-885, "only includes the VRRP message"; change note :194-196 records this as an explicit RFC 9568 clarification). ~~the umbrella constraint mandates pseudo-header for v4 AND v6 (interop reality: keepalived, and holo's missing-pseudo-header is listed as a bug). This spec follows the umbrella; see A-1 and Key Design Decisions~~ (superseded 2026-07-14: primary sources settled A-1 -- the umbrella annotation was wrong and has been corrected by the orchestrator; the interim design was tx message-only; reverted 2026-07-15 after a keepalived interop capture -- FINAL: tx is the RFC 5798 pseudo-header form (keepalived interop), rx dual-accepts (pseudo-header canonical/unflagged, RFC 9568 message-only accepted and flagged checksum-rfc9568-message-only); see Key Design Decisions)
- [ ] `rfc/short/rfc3768.md` - VRRPv2 (Wire Formats, Validation, Constants)
  → Constraint: v2 header carries Auth Type (byte 4, only 0 accepted) + Adver Int (byte 5, whole seconds 1..255) and a MANDATORY 8-byte zero Authentication Data trailer counted in the completeness check (rfc3768 §7.1); checksum plain RFC 1071 over the entire VRRP message, NO pseudo-header; interval mismatch MUST discard (unlike v3)

**Key insights:**
- The two reference implementations get v3 details wrong in opposite directions; every verified bug in the digests becomes a negative test here (umbrella R-1).
- The v3/IPv4 checksum computation is isolated in one function; that isolation is load-bearing for both the tx form and the rx dual-accept (RFC 5798 pseudo-header primary/canonical -- ze's tx form; RFC 9568 message-only fallback accepted + flagged checksum-rfc9568-message-only).
- BFD's `packet` package is a proven in-repo template: value structs, typed errors, ordered ladder, per-target fuzz enumeration.

## Current Behavior (MANDATORY)

**Source files read:** (producers read directly this session)
- [ ] `internal/component/bfd/packet/control.go` - WriteTo(buf,off) int :94; ParseControl value-decode, zero alloc :144; typed errors :68; RFC-ordered checks :171-195
- [ ] `internal/component/bfd/packet/pool.go` - pool buffer sized to RFC max packet :26; Acquire/Release contract :53-75
- [ ] `internal/component/bfd/packet/fuzz_test.go` - fuzz invariants + seed corpus from mutated golden packet :15-139
- [ ] `internal/component/bfd/packet/control_test.go` - round-trip table tests :12; offset-honored test :109
- [ ] `internal/component/firewall/protocol.go` - existing `"vrrp": 112` mapping :14 (constant precedent; NOT imported by this package)
- [ ] `mk/test-fuzz.mk` - per-target enumeration :12; bfd packet fuzz entries :59-60

**Behavior to preserve:** (unless user explicitly said to change)
- No vrrp code exists anywhere in the tree today (only the firewall protocol-name map and the VPP translate map mention vrrp); this child creates a new leaf package and touches nothing else
- `internal/component/bfd/packet/` untouched -- it is a style model, not a dependency
- `mk/test-fuzz.mk` existing targets unchanged; `FuzzDecode` line is append-only
- Firewall `match protocol vrrp` (proto 112) behavior unchanged

**Behavior to change:**
- None. New package only, plus one appended fuzz-target line in `mk/test-fuzz.mk`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Rx: transport readLoop (spec-vrrp-4) produces `(meta RxMeta, payload []byte)`: for AF_INET raw sockets the kernel delivers the full IPv4 datagram, so the transport first calls this package's `StripIPv4Header` (IHL-aware); for IPv6 the kernel delivers the payload, hop-limit/src/dst via cmsg. ~~The transport hands the pair to Decode~~ (superseded 2026-07-14 cross-review, decision D-B) The ENGINE (spec-vrrp-5) is the Decode caller -- it owns the per-interface/family group table backing `Lookup`; the transport only delivers raw pairs and exposes a `RecordRxError(reason)` counting hook.
- Tx: the FSM (spec-vrrp-2) fills an `Advertisement` value from group state and calls `WriteTo` into the transport's fixed per-instance tx buffer, then `FillChecksum` with the pseudo-header inputs.
- Until children 2/4 land, the executable entry points are the golden-vector unit tests and `FuzzDecode`.

### Transformation Path
1. Raw IPv4 datagram → `StripIPv4Header` (validates IHL≥5, header within datagram, extracts TTL/src/dst) → VRRP payload + `RxMeta`
2. Transport delivers `(RxMeta, payload)` to the engine (spec-vrrp-5); the engine calls `Decode(payload, meta, lookup)` with its group-table-backed `Lookup` (D-B) → ordered validation ladder (13 rows, table below)
3. `Decode` success → `Advertisement` value (interval already converted to milliseconds; VIPs exposed lazily over the rx buffer) → engine runs the v2-only address-list comparison (engine-side check, see taxonomy) → FSM event (spec-vrrp-2)
4. `Decode` failure (or engine-side check failure) → typed error / label → engine maps it via `Reason(err)` and counts through the transport's `RecordRxError(reason)` hook into `ze_vrrp_packet_errors_total{reason}`; packet dropped
5. Encode: group state → `Advertisement` → `WriteTo(buf, off)` writes the version-specific layout with the Checksum field ZERO → `FillChecksum(buf, off, n, src, dst)` backfills at `off+6` (skipped by transport when offloading v6 checksum to the kernel via IPV6_CHECKSUM)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| packet ↔ transport (spec-vrrp-4) | Same-process Go calls: `StripIPv4Header` (rx meta production), `WriteTo`+`FillChecksum` (tx); RxMeta is a plain value struct | [ ] |
| packet ↔ engine (spec-vrrp-5) | `Decode(payload, meta, lookup)` -- the engine owns the group table and supplies `Lookup` (D-B); errors counted via the transport's `RecordRxError(reason)` hook | [ ] |
| packet ↔ FSM (spec-vrrp-2) | `Advertisement` value + distinct decode outcomes (v3 interval carried as data; v2 mismatch as typed error) | [ ] |
| packet ↔ metrics | Exported `Reason(err) string` mapping + the engine-raised `address-list` label; counters registered/incremented in the plugin (children 4/5) via `RecordRxError` | [ ] |
| packet ↔ kernel | NONE. No sockets, no build tags in this package | [ ] |

### Integration Points
- `internal/component/bfd/packet/control.go:94,144` - codec shape being replicated (WriteTo/parse contracts)
- `mk/test-fuzz.mk:59-60` - fuzz enumeration pattern extended with `FuzzDecode`
- spec-vrrp-4 transport readLoop - production caller of `StripIPv4Header`; delivers (RxMeta, payload) and exposes the `RecordRxError` hook (wiring row below)
- spec-vrrp-5 engine - production caller of `Decode` with its group-table `Lookup` (D-B)
- spec-vrrp-2 FSM - the production caller of `WriteTo` and consumer of decode outcomes

### Architectural Verification
- [ ] No bypassed layers (codec never touches sockets/kernel; transport never parses wire bytes itself)
- [ ] No unintended coupling (package imports stdlib + net/netip only; no ze imports; nothing outside `internal/plugins/vrrp/` imports it)
- [ ] No duplicated functionality (RFC 1071 helper is local and trivial; no shared checksum util exists in ze to reuse -- verified by grep for `rfc1071`/`onesComplement` returning nothing)
- [ ] Zero-copy preserved where applicable (decode keeps a view into the rx buffer for VIPs; encode writes into the caller's pool buffer at offset)
- [ ] Registration over hardcoding -- no vrrp spelling added to any central/shared package; protocol constants defined locally in the vrrp-owned package; fuzz target added through the established per-target enumeration in `mk/test-fuzz.mk`; metrics counters registered later by plugin-owned code, this package only exports the reason mapping (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ~~v3/IPv4 checksum MUST include the IPv4 pseudo-header for keepalived interop~~ Re-settled 2026-07-15: the original assumption held on the wire -- tx uses the RFC 5798 IPv4 pseudo-header (keepalived 2.3.1 requires it; the interim message-only design of 2026-07-14 broke interop), rx dual-accepts (pseudo-header canonical, RFC 9568 message-only accepted and flagged checksum-rfc9568-message-only) | rfc/full/rfc9568.txt:880-885 ("only includes the VRRP message") + change note :194-196 (explicit 9568 clarification). Web-verified ecosystem: keepalived historically used the pseudo-header (RFC 5798 reading, v3_checksum_as_v2 knob) but is message-only since issue #2324; FRR removed the pseudo-header in 2022; Cisco always message-only; Arista 9568-compliant with an exclude knob. Pseudo-header = LEGACY behavior (uvrrpd); holo's message-only is CORRECT (digest bug-5 entry was wrong). Umbrella annotation was wrong; corrected by the orchestrator | Legacy peers would be silently rejected without the rx fallback -- dual-accept + counter covers them | spec-vrrp-6 capture: keepalived 2.3.1 sends the pseudo-header sum and decodes on ze's canonical path, the checksum-rfc9568-message-only counter stays 0; the G2 message-only unit vector covers a strict-RFC-9568 peer | resolved 2026-07-15 (the 2026-07-14 flip to message-only was itself reverted after the keepalived capture; two Mistake Log rows) |
| A-2 | `netip.AddrFrom4` / `netip.AddrFrom16` / `As4` / `As16` are allocation-free value conversions, so lazy VIP access allocates nothing | Go stdlib netip design (value types, no pointers for bare addrs) | Fall back to documenting one small fixed allocation per packet as acceptable at 1 pkt/s (Key Design Decisions) | `TestDecodeZeroAlloc` via `testing.AllocsPerRun` == 0 | unvalidated |
| A-3 | The rx buffer is reused per socket read (umbrella Architectural Verification), so an `Advertisement`'s lazy VIP view is only valid until the next read | Umbrella: "rx buffers reused per socket; encode into caller buffer" | ~~FSM must copy VIPs before persisting (constraint exported to spec-vrrp-2)~~ (corrected 2026-07-14 cross-review: spec-2's AdvertReceived carries only VIPCount, nothing in the FSM persists VIPs) The lifetime contract binds the ENGINE (spec-vrrp-5): it must AppendVIPs-copy anything it persists before the next socket read; if child 4 chooses per-packet buffers instead, the constraint relaxes harmlessly | Lifetime documented on the type; engine-side copy rule lands in spec-vrrp-5; race detector in child-4 QEMU tests | unvalidated |
| A-4 | Config (spec-vrrp-5) guarantees v3 intervals are multiples of 10 ms in 10..40950 and v2 intervals multiples of 1000 ms in 1000..255000, so encode conversion ms→wire is exact | spec-vrrp-5's verifier enforces v3 interval %10==0 at verify time (its cross-leaf validation table, decision D-C) + umbrella boundary table + config-option pattern | Encode asserts and returns a typed `ErrIntervalRange` -- kept as defense-in-depth against engine bugs (D-C); boundary tests cover the assert independently of the verifier | `TestBoundaryInterval` here + child-5 verifier test | unvalidated |
| A-5 | One fuzz target per line in `mk/test-fuzz.mk` is the required registration (no auto-discovery) | `mk/test-fuzz.mk:10-12` comment: "-fuzz=. fails with matches more than one" | Adjust to whatever pattern the makefile has moved to | `make ze-fuzz-test` run includes FuzzDecode | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | (umbrella R-1) Reference implementations normalized wrong v3 behavior; copying their shape may smuggle semantics | A negative test derived from rfc/short contradicts code copied from a digest | Codec written from `rfc/short/` tables only; every verified holo/uvrrpd bug is an explicit negative test (table below) |
| R-2 | (umbrella R-2) s/cs/ms unit confusion, the single most common VRRP defect | Golden decode test expects 1000 ms and gets 100 or 100000 | One internal unit (ms); conversion exists ONLY in encode/decode; boundary tests pin both wire ranges; grep gate in Critical Review for stray unit arithmetic |
| R-3 | Hand-computed golden checksums in this spec are wrong | Round-trip passes but golden byte assert fails, or spec-vrrp-6 capture disagrees | Checksums recomputed by an independent straight-line RFC 1071 reference inside the tests; final authority = live keepalived captures (cross-check rows below) |
| R-4 | Lazy VIP view over a reused rx buffer causes use-after-reuse in a careless caller | Corrupt VIP values in engine (child-5) tests; race detector hits in child-4 QEMU runs | Lifetime contract documented on the type + `AppendVIPs` copy helper provided; ~~spec-vrrp-2 inherits the copy-before-persist constraint~~ (corrected 2026-07-14: the constraint binds the ENGINE, spec-vrrp-5 -- spec-2's AdvertReceived carries only VIPCount and persists no VIPs) |
| R-5 | ~~A-1 resolves against the umbrella (keepalived actually follows published RFC 9568 for v3/IPv4)~~ Resolved 2026-07-15: keepalived does NOT follow published RFC 9568 for v3/IPv4 -- it requires the RFC 5798 pseudo-header; design is tx pseudo-header + rx dual-accept (RFC 9568 message-only became the accepted+flagged fallback) | Residual signal: checksum-rfc9568-message-only counter nonzero = a strict-RFC-9568 (message-only) peer on the LAN | Nothing to fix -- the counter is the operator-visible answer; spec-vrrp-6 asserts it stays 0 with keepalived (which sends the pseudo-header form) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| transport readLoop (spec-vrrp-4) delivers (RxMeta, payload); ~~transport~~ the engine (spec-vrrp-5) calls Decode with its group-table Lookup (D-B) | → | packet.Decode ordered validation ladder | `TestDecodeGoldenV2`, `TestDecodeGoldenV3IPv4`, `TestDecodeGoldenV3IPv6` (this child) + end-to-end `test/vrrp/vrrp-instance-up.ci` (spec-vrrp-5) |
| FSM advert timer (spec-vrrp-2) sends an advertisement | → | Advertisement.WriteTo + FillChecksum | `TestEncodeGoldenV3IPv4` (this child) + `effective-vrrp-keepalived.py` scenario 1 (spec-vrrp-6) |
| AF_INET raw socket datagram (spec-vrrp-4) | → | packet.StripIPv4Header (IHL-aware) | `TestStripIPv4HeaderIHL` (this child) + `test/vrrp/vrrp-backup-hold.ci` (spec-vrrp-6 QEMU) |
| make ze-fuzz-test | → | FuzzDecode (adversarial input never panics) | `FuzzDecode` enumerated in `mk/test-fuzz.mk` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Encode v2 advert (vrid 10, prio 200, 2 VIPs, 1000 ms) | Exact golden bytes: auth type 0, Adver Int byte = 1 (seconds), 8-byte zero auth trailer, checksum 0x92ED |
| AC-2 | Encode v3 IPv4 and v3 IPv6 adverts (vectors below) | Exact golden bytes: reserved nibble 0, 12-bit interval = 100 cs, checksums 0xDEFB v3/IPv4 (RFC 5798 pseudo-header -- ze's tx form for keepalived interop; ~~0x828A message-only~~ reverted 2026-07-15 after the keepalived capture; `TestEncodeGoldenV3IPv4` asserts 0xDEFB) / 0x3F5D v6 pseudo-header |
| AC-3 | Decode each golden vector with matching RxMeta | Field-equal Advertisement; `AdverIntervalMS` == 1000 for all three (wire units converted at the boundary only) |
| AC-4 | Each row of the ordered validation table violated in isolation | The row's typed error is returned, `Reason(err)` yields the row's reason label, earlier rows win when multiple violations coexist |
| AC-5 | v3 advert whose interval differs from local config | NO error; Advertisement carries the received interval for FSM adoption. v2 advert with mismatched interval → `ErrV2IntervalMismatch` (discard) |
| AC-6 | IPv4 datagrams with IHL 5, 6, 15; IHL < 5; header longer than datagram | Strip helper honors IHL for valid values, returns typed errors for malformed (holo fixed-20-byte-strip bug prevented) |
| AC-7 | `make ze-fuzz-test` | FuzzDecode runs 10s, no panic, no accepted packet violating the ladder invariants |
| AC-8 | Happy-path Decode of each golden vector | 0 heap allocations (`testing.AllocsPerRun`) |
| AC-9 | Boundary table rows (vrid, priority, count, both interval ranges) | Every Last Valid accepted, every Invalid Below/Above rejected exactly as tabled |
| AC-10 | G2c (v3/IPv4 pseudo-header checksum 0xDEFB), G2 (v3/IPv4 message-only checksum 0x828A), and a payload failing BOTH sums | G2c (pseudo-header) ACCEPTED as canonical, MsgOnlyChecksum unset; G2 (message-only) ACCEPTED but flagged MsgOnlyChecksum (reason checksum-rfc9568-message-only); both-fail payload → ErrChecksum |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs keepalived (default v2) next to a ze backup; ze holds Backup while keepalived advertises | wire → transport rx → StripIPv4Header → engine Decode (spec-vrrp-5, D-B) → FSM backup hold (spec-vrrp-2) | `TestDecodeGoldenV2` (here) + `test/vrrp/vrrp-backup-hold.ci` (spec-vrrp-6) |
| 2 | ze is Active; keepalived peer accepts its v3 adverts | FSM timer → WriteTo + FillChecksum → tx socket (spec-vrrp-4) → keepalived stays Backup | `TestEncodeGoldenV3IPv4` (here) + `effective-vrrp-keepalived.py` scenario 1 (spec-vrrp-6) |
| 3 | Off-link attacker replays a forwarded advert (TTL < 255) | rx → Decode → ErrTTL → drop + `ze_vrrp_packet_errors_total{reason="ttl"}` | `TestNegativeReferenceBugs/ttl-not-255` (here) + `test/vrrp/vrrp-metrics.ci` (spec-vrrp-5) |

## Design: Advertisement Type and API

### Advertisement fields (one struct serves encode and decode)

| Field | Type | Unit / range | Encode source | Decode source |
|-------|------|--------------|---------------|---------------|
| Version | uint8 | 2 or 3 | group config | wire byte 0 high nibble |
| Family | uint8 (packet.V4 / packet.V6) | - | group config | RxMeta.Family |
| VRID | uint8 | 1..255 | group config | wire byte 1 |
| Priority | uint8 | encode: 1..254 config, 255 owner, 0 resign; decode: raw 0..255 | FSM state | wire byte 2 |
| Count | uint8 | encode 1..16 (config cap); decode: whatever length-exactness admits | len(VIPs) | wire byte 3 |
| AdverIntervalMS | uint32 | MILLISECONDS ALWAYS (umbrella R-2); v3 wire 10..40950, v2 wire 1000..255000 | group config / learned | converted from wire (v3 cs×10, v2 s×1000) |
| VIPs | []netip.Addr | max 16 on encode | config-owned slice, allocated once per group | nil on decode (lazy view used instead) |
| wireVIPs (unexported) | []byte | view into rx buffer | unused | sub-slice of payload; valid until next socket read (A-3) |
| MsgOnlyChecksum | bool | decode-only outcome flag | n/a -- encode always emits the RFC 5798 pseudo-header sum | true when rx matched ONLY the RFC 9568 message-only sum, not the pseudo-header form ze sends (counted as checksum-rfc9568-message-only; marks a strict-RFC-9568 peer) |

Accessors (decode side, allocation-free): `VIPCount() int`, `VIPAt(i int) netip.Addr`
(bounds-checked), `AppendVIPs(dst []netip.Addr) []netip.Addr` (explicit copy for
callers that persist beyond the buffer lifetime).

### RxMeta (produced by transport, spec-vrrp-4)

| Field | Type | Source |
|-------|------|--------|
| TTL | uint8 | IPv4 header (via StripIPv4Header) or IPv6 hop-limit cmsg |
| Src | netip.Addr | IP header / cmsg -- checksum pseudo-header input + FSM tie-break input |
| Dst | netip.Addr | IP header / cmsg -- checksum pseudo-header input |
| Family | uint8 | socket family |
| IfIndex | int | transport socket binding; informational (engine/logging, D-A) -- Decode ignores it |

### Lookup (caller-supplied group table probe)

`Lookup` is a function value `(vrid uint8) → (Local, bool)` supplied per
receiving interface+family by the ENGINE (spec-vrrp-5), which owns the group
table (D-B). `Local` carries `Version uint8` and
`AdverIntervalMS uint32` (needed for the v2 mismatch discard). Unknown vrid is a
typed error (`ErrUnknownVRID`) so the caller counts vrid-mismatch separately from
malformed traffic -- on a shared LAN, other routers' groups are normal, not attacks.

### Wire layouts

v3 (RFC 9568 §5.1-5.2; offsets from start of VRRP message):

```
byte 0        : Version(4b)=3 | Type(4b)=1
byte 1        : Virtual Rtr ID
byte 2        : Priority
byte 3        : IPvX Addr Count
bytes 4-5     : Reserve(4b)=0 | Max Advertise Interval(12b, centiseconds 1..4095)
bytes 6-7     : Checksum
bytes 8..     : Count x 4 (IPv4) or Count x 16 (IPv6) addresses
total         : 8 + Count*4 (v4) | 8 + Count*16 (v6)   -- EXACT, no trailer
```

v2 (RFC 3768 §5.1-5.3; IPv4 only):

```
byte 0        : Version(4b)=2 | Type(4b)=1
byte 1        : Virtual Rtr ID
byte 2        : Priority
byte 3        : Count IP Addrs
byte 4        : Auth Type (only 0 accepted; 1,2,unknown -> discard)
byte 5        : Adver Int (whole seconds 1..255)
bytes 6-7     : Checksum
bytes 8..     : Count x 4 addresses
trailer       : 8 bytes Authentication Data (zero on tx, ignored on rx,
                counted in the completeness check)
total         : 8 + Count*4 + 8   -- EXACT
```

### Encode contract (buffer-first)

| Element | Contract |
|---------|----------|
| `WriteTo(buf, off) int` | Writes the version-specific layout with the Checksum field ZERO; returns bytes written; panics on short buffer like the rest of ze's buffer-first code (caller sizes from MaxLen constants); converts AdverIntervalMS→wire unit, returning/asserting range per A-4 |
| `FillChecksum(buf, off, n int, src, dst netip.Addr)` | Computes the version/family-correct checksum over `buf[off:off+n]` and backfills bytes off+6..7 (skip-and-backfill, `ai/rules/buffer-first.md`) |
| v6 offload note | Transport MAY set IPV6_CHECKSUM (offset 6) and skip FillChecksum on tx, letting the kernel fill/verify; the software path MUST exist regardless -- it is the only path for v4/v2 and for unit tests and for rx verification when the kernel option is not used |
| `MaxLenV2` = 80, `MaxLenV3v4` = 72, `MaxLenV3v6` = 264 | 16-VIP maxima; exported so the transport (child 4) sizes its fixed per-instance tx buffer ~~tx pool like bfd pool.go:26~~ (aligned 2026-07-14 with spec-vrrp-4's design: fixed buffer, no pool) |
| Constants | ProtoNumber=112, MulticastV4=224.0.0.18, MulticastV6=ff02::12, MAC prefixes 00:00:5e:00:01/02 -- defined here (vrrp-owned), consumed by children 4/5 (the pure FSM never touches MACs) |

### Checksum definitions (checksum.go)

| Case | Coverage |
|------|----------|
| v2 (IPv4 only) | RFC 1071 one's complement over the ENTIRE VRRP message including the 8-byte auth trailer; checksum field zeroed; NO pseudo-header (RFC 3768 §5.3.7) |
| v3 IPv4 (tx + primary rx) | RFC 1071 over the VRRP message PLUS the RFC 5798 IPv4 pseudo-header: src (4B), dst (4B), zero (1B), protocol 112 (1B), VRRP length (2B). This is ze's tx form and the primary rx form (checksum.go `pseudoSumV4Legacy`). ~~message-only, no pseudo-header (RFC 9568 §5.2.8)~~ reverted 2026-07-15 -- a keepalived 2.3.1 wire capture proved keepalived computes and REQUIRES the pseudo-header and rejects message-only as "Invalid VRRPv3 checksum"; interoperating with the installed base outranks the RFC 9568 clarification (checksum.go:97-110) |
| v3 IPv4 (rx fallback, dual-accept) | If the pseudo-header sum fails, recompute the RFC 9568 message-only sum (VRRP message only, no pseudo-header). Match → ACCEPT, set MsgOnlyChecksum, count reason checksum-rfc9568-message-only (a strict-RFC-9568 peer: keepalived post-#2324, FRR post-2022, Cisco). Both sums fail → ErrChecksum |
| v3 IPv6 | RFC 1071 over the VRRP message PLUS the RFC 8200 §8.1 pseudo-header: src (16B), dst (16B), upper-layer length (4B), zero (3B), next header 112 (1B) |

Rx verification recomputes with the ACTUAL packet src/dst from RxMeta (uvrrpd
digest). The v3/IPv4 sum selection lives in ONE function; that isolation
implements the dual-accept: primary RFC 5798 pseudo-header (ze's tx form and the
deployed base's), fallback RFC 9568 message-only (accept + flag as
checksum-rfc9568-message-only), reject only when both sums fail. The
primary/fallback ordering was reversed here until the 2026-07-15 keepalived
capture proved the pseudo-header form is the one on the wire in practice
(checksum.go `verifyReceived`).

### Ordered receive-validation ladder (validate.go)

Order is fixed and test-enforced (`TestValidationOrder`). §7.1 of both RFCs
requires every check to pass but does not mandate evaluation order; this order
puts the vrid lookup before the checksum so ze never checksums other routers'
multicast traffic, and computes the checksum over the FULL received payload so a
v3 packet carrying a spurious v2 auth trailer fails the later length-exactness
check with an actionable reason instead of a generic checksum error.

| # | Check | Pass condition | Typed error | reason label | RFC cite |
|---|-------|----------------|-------------|--------------|----------|
| 1 | Minimum length | len(payload) >= 8 | ErrTruncated | truncated | 9568 §7.1 / 3768 §7.1 |
| 2 | Version nibble | byte0 high nibble ∈ {2,3} | ErrVersion | version | 9568 §7.1 / 3768 §7.1 |
| 3 | Type nibble | byte0 low nibble == 1 | ErrType | type | 9568 §5.2.2 / 3768 §5.3.2 |
| 4 | VRID known | Lookup(vrid) ok | ErrUnknownVRID | vrid | 9568 §7.1 (erratum 8298 keeps owner-rx as log-only -- owner handling is FSM, child 2; applied to v2 as well, deviation noted in Known Limitations) |
| 5 | Version matches group | wire version == Local.Version | ErrVersion | version | 9568 §7.1 ("verify version"), per-group v2 opt-in (umbrella) |
| 6 | Checksum | v2 + v3/IPv6: verifies per table above, over full payload, src/dst from RxMeta. v3/IPv4: primary RFC 5798 pseudo-header sum (ze's tx form); on mismatch retry the RFC 9568 message-only sum -- match ACCEPTS with MsgOnlyChecksum set; both fail → error | ErrChecksum | checksum (reject); checksum-rfc9568-message-only (accepted, counted) | 9568 §5.2.8 / 3768 §7.1 |
| 7 | TTL / hop-limit | RxMeta.TTL == 255 (GTSM) | ErrTTL | ttl | 9568 §5.1.1.3, §5.1.2.3 / 3768 §5.2.3 |
| 8 | Count non-zero | byte3 >= 1 | ErrCountZero | count-zero | 9568 erratum 8299 / §5.2.5 |
| 9 | Length exactness | v3: len == 8+count*addrsize; v2: len == 8+count*4+8 | ErrLength | length | 9568 §7.1 / 3768 §7.1 (catches v3+auth-trailer, trailing garbage, count/length lies) |
| 10 | v2 auth type | byte4 == 0 | ErrAuthType | auth-type | 3768 §5.3.6, §7.1 (types 1, 2, unknown all discarded; no auth implemented, umbrella decision) |
| 11 | Interval extraction | v3: 12-bit cs != 0 (→ms×10); v2: seconds byte != 0 (→ms×1000) | ErrIntervalZero | interval-zero | 9568 erratum 8301; v2 zero is stricter-than-RFC but can never match a legal local config (1..255 s), so behavior equals the mandated mismatch discard with a more precise reason |
| 12 | Interval policy | v2: wire ms == Local.AdverIntervalMS else discard. v3: NEVER an error -- value returned in Advertisement for FSM adoption (§6.4.2), SHOULD-log only | ErrV2IntervalMismatch (v2 only) | interval-mismatch | 3768 §7.1 MUST discard / 9568 §7.1 no drop |
| 13 | v3 IPv6 first VIP link-local | VIPAt(0) in fe80::/10 | ErrFirstNotLinkLocal | linklocal | 9568 erratum 8300 / §5.2.9 |

Family-vs-header-family mismatch (9568 §5.2.9) cannot occur inside Decode: the
address size is derived from RxMeta.Family, and exactness (row 9) rejects
anything inconsistent; documented as a comment, not a separate row.

### Typed error taxonomy → metrics reasons

One row per validation failure; `Reason(err) string` is the exported mapping the
engine uses to label `ze_vrrp_packet_errors_total{reason}` through the
transport's `RecordRxError(reason)` hook (registered in children 4/5 -- no dead
counters, holo bug 9). Reason labels are the third
column of the ladder table plus, from the strip helper: ErrIPv4HeaderShort →
`ip-header`, ErrIPv4BadIHL → `ip-header`. `TestErrorReasonMapping` asserts the
mapping is total (every exported Err* has a reason) and injective where the
table says so. `checksum-rfc9568-message-only` is NOT an error: it labels an
ACCEPTED packet (MsgOnlyChecksum true); the transport counts it under the same
metric so strict-RFC-9568 peers (which send the message-only sum ze does not)
are operator-visible. `TestErrorReasonMapping` covers this label too. `address-list` is an additional ENGINE-raised label (v2-only
post-Decode address-list comparison, owned by spec-vrrp-5 -- see Known
Limitations), not a Decode ladder row; `TestErrorReasonMapping` includes it in
the label inventory.

## 🧪 TDD Test Plan

### Golden byte vectors (hex)

Derived field-by-field from the RFC wire diagrams; checksums hand-computed via
RFC 1071 (folded 32-bit sum, complement). The tests recompute each checksum with
an independent straight-line reference implementation before asserting the
literal bytes (R-3).

Vector G1 -- v2 IPv4: vrid 10, prio 200, VIPs 192.0.2.1 + 192.0.2.2, interval
1000 ms (wire: 1 s). Checksum over entire message incl. trailer = 0x92ED.

```
21 0A C8 02 00 01 92 ED  C0 00 02 01 C0 00 02 02
00 00 00 00 00 00 00 00                            (24 bytes)
```

Vector G2 -- v3 IPv4 message-only (rx-accept only, flagged): vrid 10, prio 200,
VIPs 192.0.2.1 + 192.0.2.2, interval 1000 ms (wire: 0x064 cs). Checksum
message-only per RFC 9568 §5.2.8 = 0x828A (folded message sum 0x27D73 → 0x7D75 →
complement 0x828A). Decode ACCEPTS it but sets MsgOnlyChecksum (reason
checksum-rfc9568-message-only) -- it is the strict-RFC-9568 form a peer such as
post-#2324 keepalived sends, NOT the form ze transmits. Roles reversed
2026-07-15: this vector was the canonical form under the interim design until the
keepalived capture; the pseudo-header vector G2c below is now canonical.

```
31 0A C8 02 00 64 82 8A  C0 00 02 01 C0 00 02 02   (16 bytes)
```

Vector G2c -- v3 IPv4 canonical (tx + primary rx): identical fields with the RFC
5798 pseudo-header checksum (src 192.0.2.251, dst 224.0.0.18, proto 112, len 16;
pseudo sum 0x1A38D + message sum 0x27D73 = 0x42100 → fold 0x2104 →
complement 0xDEFB). This is the form ze TRANSMITS for v3/IPv4 (keepalived
interop) and the primary rx form; Decode accepts it with MsgOnlyChecksum UNSET.
`TestEncodeGoldenV3IPv4` asserts these exact bytes.

```
31 0A C8 02 00 64 DE FB  C0 00 02 01 C0 00 02 02   (16 bytes)
```

Vector G3 -- v3 IPv6: vrid 10, prio 100, VIPs fe80::1 + 2001:db8::1, interval
1000 ms. Pseudo-header src fe80::c8, dst ff02::12, len 40, NH 112 → checksum
0x3F5D.

```
31 0A 64 02 00 64 3F 5D  FE 80 00 00 00 00 00 00
00 00 00 00 00 00 00 01  20 01 0D B8 00 00 00 00
00 00 00 00 00 00 00 01                            (40 bytes)
```

Cross-check row: vectors G1/G2c/G3 are validated against live keepalived
captures in spec-vrrp-6 (`vrrp-v2-keepalived`, `vrrp-v3-keepalived` scenarios).
The 2026-07-15 keepalived 2.3.1 capture showed its v3/IPv4 advert carries the
RFC 5798 pseudo-header checksum (its own advert checksum 0xa102 matches the
pseudo-header sum, not the message-only sum 0x448e), so it decodes on ze's
canonical (pseudo-header) rx path and the checksum-rfc9568-message-only counter
stays 0. G2 (message-only) is a unit-test-only strict-RFC-9568-peer fixture. Any
divergence reopens this spec's Mistake Log.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEncodeGoldenV2` | `internal/plugins/vrrp/packet/packet_test.go` | WriteTo+FillChecksum == G1 exactly (trailer zeroed, seconds byte) | |
| `TestEncodeGoldenV3IPv4` | `packet_test.go` | == G2c exactly (reserved nibble 0, 12-bit cs, RFC 5798 pseudo-header checksum 0xDEFB -- ze's tx form) | |
| `TestEncodeGoldenV3IPv6` | `packet_test.go` | == G3 exactly (v6 pseudo-header) | |
| `TestDecodeGoldenV2` / `TestDecodeGoldenV3IPv4` / `TestDecodeGoldenV3IPv6` | `packet_test.go` | Field-equal Advertisement; AdverIntervalMS == 1000 in all three | |
| `TestRoundTrip` | `packet_test.go` | Encode→decode matrix: {v2, v3v4, v3v6} × counts {1, 3, 16} × priorities {0, 1, 100, 254, 255} | |
| `TestWriteToOffset` | `packet_test.go` | Non-zero offset honored, earlier bytes untouched (bfd control_test.go:109 model) | |
| `TestChecksumRFC1071` | `packet_test.go` | Sum/fold/complement against independent reference + odd-length input | |
| `TestValidationOrder` | `packet_test.go` | Packet violating rows N and N+k returns row N's error, for every adjacent pair | |
| `TestErrorReasonMapping` | `packet_test.go` | Reason() total over all Err* plus the accepted-outcome label checksum-rfc9568-message-only; labels match the ladder table | |
| `TestDecodeV3IntervalMismatchNotError` | `packet_test.go` | v3 mismatched interval decodes cleanly, value surfaced (holo/uvrrpd adopt-bug prevented) | |
| `TestDecodeV2IntervalMismatchDiscard` | `packet_test.go` | v2 mismatch → ErrV2IntervalMismatch | |
| `TestDecodeV3IPv4MsgOnlyChecksumCompat` | `packet_test.go` | G2c (pseudo-header) accepted as canonical with MsgOnlyChecksum unset; G2 (message-only) accepted with MsgOnlyChecksum set; both-sums-fail → ErrChecksum | |
| `TestStripIPv4HeaderIHL` | `packet_test.go` | IHL 5/6/15 strip correct payload+RxMeta; IHL<5, header>datagram → typed errors | |
| `TestNegativeReferenceBugs` | `packet_test.go` | Table below, one subtest per row | |
| `TestBoundaryVRID` / `TestBoundaryPriority` / `TestBoundaryCount` / `TestBoundaryInterval` | `packet_test.go` | Boundary table rows | |
| `TestDecodeZeroAlloc` | `packet_test.go` | AllocsPerRun == 0 on all three golden decodes incl. VIPAt access | |
| `BenchmarkDecode` / `BenchmarkEncode` | `packet_test.go` | Perf baseline; allocation regression guard | |
| `FuzzDecode` | `internal/plugins/vrrp/packet/fuzz_test.go` | Never panics; accepted packets satisfy every ladder invariant; round-trips; seed corpus = golden vectors + one mutation per ladder row (bfd fuzz_test.go:104 model) | |

### Negative tests from verified reference-implementation bugs
| # | Input | Must | Source bug |
|---|-------|------|-----------|
| N1 | G2 bytes: RFC 9568 message-only sum 0x828A (no pseudo-header) | ACCEPT with MsgOnlyChecksum + reason checksum-rfc9568-message-only (a strict-RFC-9568 peer) | keepalived post-#2324 / FRR post-2022 / Cisco emit this; ze transmits the RFC 5798 pseudo-header form instead (2026-07-15 keepalived capture) |
| N1b | G2 bytes with a checksum failing BOTH sums (e.g. 0x0000) | ErrChecksum | - |
| N2 | v3 IPv4 packet with 8-byte v2 auth trailer appended | ErrLength (exactness), NOT checksum | uvrrpd v3/IPv4 spurious trailer |
| N3 | IPv4 datagram with IHL=6 (options) | Strip yields correct payload; decode succeeds | holo bug 11 (fixed 20-byte strip) |
| N4 | RxMeta.TTL = 64 | ErrTTL | holo bug 3 (missing GTSM check) |
| N5 | Count = 0, length exact for 0 | ErrCountZero | RFC 9568 erratum 8299 |
| N6 | v3 interval bits = 0 | ErrIntervalZero | RFC 9568 erratum 8301 |
| N7 | v6 count = 255 with exact length (4088-byte payload) | Decode succeeds lazily, zero allocation; VIPAt bounds hold | uvrrpd MAXSIZE-sized-for-v4 bug; allocation blowup |
| N8 | v2 auth type = 1 (keepalived simple-password) | ErrAuthType | 3768 §5.3.6; keepalived nonstandard auth |
| N9 | v2 packet at a v3-configured group (and inverse) | ErrVersion at row 5 | interop mode out of scope (umbrella) |
| N10 | v3 skew/adopt inputs: interval 4095 cs decodes to 40950 ms exactly | No rounding drift | holo 100x cs/s bug class (R-2) |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| vrid (decode) | 1-255 | 255 | 0 → ErrUnknownVRID (nothing configurable at 0) | N/A (uint8) |
| vrid (encode) | 1-255 | 255 | 0 rejected | N/A (uint8) |
| priority (decode, raw) | 0-255 | 255 | N/A (0 = resign, valid) | N/A (uint8) |
| priority (encode) | 0-255 (0 resign / 1-254 config / 255 owner) | 254 config | N/A | N/A (config range is child-5 YANG) |
| count (encode) | 1-16 | 16 | 0 rejected | 17 rejected (config cap) |
| count (decode) | 1-(what exact length admits) | 255 (v6, N7) | 0 → ErrCountZero | count*size > payload → ErrLength |
| AdverIntervalMS v3 | 10-40950 (wire 1-4095 cs) | 40950 | 0 → ErrIntervalZero (decode) / encode assert | 40960 → encode assert (12-bit wire cap) |
| AdverIntervalMS v2 | 1000-255000 (wire 1-255 s) | 255000 | 999 / 0 → encode assert / ErrIntervalZero | 256000 → encode assert (8-bit wire cap) |
| IHL (strip helper) | 5-15 | 15 (60-byte header) | 4 → ErrIPv4BadIHL | N/A (4-bit field) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Decode path end-to-end | `test/vrrp/vrrp-instance-up.ci` (owned by spec-vrrp-5) | Operator commits a vrrp group; instance receives/sends adverts through this codec | |
| Error counters visible | `test/vrrp/vrrp-metrics.ci` (owned by spec-vrrp-5) | Malformed packet increments ze_vrrp_packet_errors_total{reason} | |

This child is a pure-Go codec with no user-facing surface of its own; its .ci
coverage is delivered by the spec-vrrp-5 functional suite named above (this
package is on the only packet path, so those tests cannot pass without it).
Within this child the Go entry points are the golden/negative/boundary unit
tests and `FuzzDecode`.

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| v3 golden-vector + checksum cross-check | `test/interop/scenarios/vrrp-v3-keepalived/` (owned by spec-vrrp-6) | keepalived | Live captures decode cleanly on the canonical pseudo-header path (keepalived 2.3.1 sends the RFC 5798 pseudo-header sum, capture 2026-07-15); checksum-rfc9568-message-only counter stays 0 with keepalived | |
| v2 wire format | `test/interop/scenarios/vrrp-v2-keepalived/` (owned by spec-vrrp-6) | keepalived (default v2) | RFC 3768 layout incl. auth trailer interops | |

### Future (if deferring any tests)
- None deferred. Interop execution is owned by spec-vrrp-6 by umbrella design (this child has no sockets); the golden vectors here are its fixtures.

## Files to Modify
- `mk/test-fuzz.mk` - append one `FuzzDecode` line targeting `./internal/plugins/vrrp/packet/...` (per-target enumeration pattern, mk/test-fuzz.mk:59-60)

All codec feature code is NEW (this package does not exist); see Files to Create.
The umbrella carries the cross-child Files to Modify list.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A | Config surface is spec-vrrp-5; this package receives already-validated values |
| YANG validation constraints | N/A | spec-vrrp-5 (interval multiples/ranges, max-elements 16) -- encode asserts independently (A-4) |
| YANG custom validators | N/A | spec-vrrp-5 |
| CLI commands/flags | N/A | No CLI surface |
| CLI grammar | N/A | No CLI surface |
| Editor autocomplete | N/A | No config surface |
| Functional test for new RPC/API | N/A here | Codec has no RPC; end-to-end via `test/vrrp/*.ci` (spec-vrrp-5) |
| Pipe completeness | N/A | No command output |
| Env var registration | N/A | No env vars (umbrella: all config is YANG) |
| Doctor check for runtime dependencies | N/A | Pure Go, no runtime dependencies; raw-socket doctor check is spec-vrrp-4 |
| Prometheus counters/metrics | Partial | Reason-label taxonomy + `Reason(err)` mapping defined HERE; `ze_vrrp_packet_errors_total{reason}` registration/increment in spec-vrrp-4/5 (plugin-owned, self-containment) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Codec is internal; umbrella row covers the feature docs at close |
| 2 | Config syntax changed? | No | No config surface in this child |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | Plugin registration is spec-vrrp-5 |
| 6 | Has a user guide page? | No | `docs/guide/vrrp.md` lands with spec-vrrp-5 |
| 7 | Wire format changed? | No | VRRP is not a ze-internal wire format (umbrella row 7) |
| 8 | Plugin SDK/protocol changed? | No | No SDK changes |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes (at umbrella close) | `docs/features/rfc-status.md` rows for 9568/3768 cite this package's validate.go/checksum.go as source anchors; written when the umbrella closes (spec-vrrp-5/6 own the rows) |
| 10 | Test infrastructure changed? | Yes | `mk/test-fuzz.mk` gains FuzzDecode -- self-documenting enumeration; `docs/functional-tests.md` untouched (no .ci here) |
| 11 | Affects daemon comparison? | No | Umbrella row at close |
| 12 | Internal architecture changed? | No | Leaf package, no contract changes elsewhere |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No (here) | Counter docs land with registration (spec-vrrp-4/5) |
| 15 | Registered inventory changed? | No | No registrations in this child |
| 16 | Changed source files referenced by doc anchors? | No | grep docs/ for mk/test-fuzz.mk anchors at implementation time; expected none |
| 17 | Existing docs show examples for this area? | No | None exist yet |

## Files to Create
- `internal/plugins/vrrp/packet/packet.go` - Advertisement type, Family/constants (proto 112, multicast groups, MAC prefixes, MaxLen*), WriteTo, unit conversions (names indicative)
- `internal/plugins/vrrp/packet/checksum.go` - RFC 1071 sum/fold, v4/v6 pseudo-header builders, FillChecksum, rx verify (A-1 selection isolated here)
- `internal/plugins/vrrp/packet/validate.go` - RxMeta, Lookup/Local, Decode with the 13-row ordered ladder, typed Err* sentinels, Reason(err) mapping, StripIPv4Header
- `internal/plugins/vrrp/packet/packet_test.go` - golden, round-trip, order, negative, boundary, zero-alloc, benchmarks
- `internal/plugins/vrrp/packet/fuzz_test.go` - FuzzDecode + seed corpus

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Create/Modify, TDD Test Plan -- check what exists (expected: nothing) |
| 3. Wiring phase | Wiring Test table -- package skeleton + failing golden tests + FuzzDecode line in mk/test-fuzz.mk |
| 4. Implement (TDD) | Phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-fuzz-test` (no .ci in this child) |
| 6. Critical review | Critical Review Checklist below |
| 7-9. Fix/re-verify loop | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a Self-Critical Review. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- package skeleton with stub Decode/WriteTo; write `TestDecodeGoldenV3IPv4` + `TestEncodeGoldenV3IPv4` (failing); append FuzzDecode line to `mk/test-fuzz.mk` (fails: target missing)
   - Tests: `TestDecodeGoldenV3IPv4`, `TestEncodeGoldenV3IPv4`
   - Files: packet.go, validate.go (stubs), mk/test-fuzz.mk
   - Verify: package compiles, golden tests fail on stubs
2. **Phase: Checksum** -- RFC 1071 core + both pseudo-header builders + FillChecksum
   - Tests: `TestChecksumRFC1071`
   - Files: checksum.go
   - Verify: reference-vs-implementation agreement incl. odd length
3. **Phase: Encode** -- WriteTo v2/v3 layouts, ms→wire conversion asserts
   - Tests: `TestEncodeGoldenV2/V3IPv4/V3IPv6`, `TestWriteToOffset`, `TestBoundaryInterval` (encode side)
   - Files: packet.go
   - Verify: all three golden encodes byte-exact
4. **Phase: Decode + ladder** -- RxMeta, Lookup, ordered ladder, typed errors, Reason mapping, lazy VIP view
   - Tests: `TestDecodeGolden*`, `TestValidationOrder`, `TestErrorReasonMapping`, `TestDecodeV3IntervalMismatchNotError`, `TestDecodeV2IntervalMismatchDiscard`, `TestRoundTrip`
   - Files: validate.go, packet.go
   - Verify: ladder order enforced; v3/v2 interval semantics split correct
5. **Phase: IPv4 strip helper + negatives + boundaries** -- StripIPv4Header, negative table N1-N10, remaining boundary tests
   - Tests: `TestStripIPv4HeaderIHL`, `TestNegativeReferenceBugs`, `TestBoundary*`
   - Files: validate.go
   - Verify: every reference bug provably not replicated
6. **Phase: Zero-alloc + fuzz** -- AllocsPerRun gate, seed corpus, benchmarks; run `make ze-fuzz-test`
   - Tests: `TestDecodeZeroAlloc`, `FuzzDecode`, `BenchmarkDecode/Encode`
   - Files: fuzz_test.go, packet_test.go
   - Verify: 0 allocs; 10s fuzz clean
7. **RFC refs** -- add `// RFC 9568 Section X.Y` / `// RFC 3768 Section X.Y` comments per RFC Documentation section
8. **Full verification** -- `make ze-verify` (fuzz excluded there; already run in phase 6)
9. **Complete spec** -- audit tables, learned summary `plan/learned/NNN-vrrp-1-packet.md`, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 has implementation + test with file:line |
| Feature completeness | Every decode outcome child 2 needs is expressible: v3 adopted interval as data, v2 mismatch as error, vrid-mismatch separately countable, priority 0/255 pass through raw |
| Correctness | Golden checksums re-derived by the independent in-test reference; ms conversions exact at both wire extremes (N10); ladder order matches the table row-for-row |
| Naming | Errors `Err*`; reason labels exactly the ladder's kebab-case strings; constants named per RFC terms |
| Data flow | Package imports stdlib + net/netip only (grep import block); no build tags; no sockets |
| CLI grammar | N/A -- no CLI surface |
| Registration over hardcoding | No vrrp spelling added to any central/shared package; constants local to the vrrp-owned package; FuzzDecode added via the established mk/test-fuzz.mk enumeration; metrics counters NOT registered here (plugin-owned in children 4/5); `ai/rules/plugin-self-containment.md` removal test holds |
| Doctor checks | N/A -- no runtime dependencies |
| YANG validation | N/A here -- encode-side range asserts stand alone (A-4) |
| Prometheus counters | Reason taxonomy total and injective per table; no dead reasons (holo bug 9 class) |
| Rule: buffer-first | WriteTo(buf,off) int; FillChecksum backfills (skip-and-backfill); grep encode path for append/make -- none |
| Rule: umbrella R-2 | grep package for `time.Second`, `*100`, `/100` outside the two conversion points in encode/decode -- none |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Five package files exist | `ls internal/plugins/vrrp/packet/` |
| All unit tests pass | `go test ./internal/plugins/vrrp/packet/` |
| FuzzDecode wired | `grep FuzzDecode mk/test-fuzz.mk` + `make ze-fuzz-one FUZZ=FuzzDecode PKG=./internal/plugins/vrrp/packet/... TIME=30s` clean |
| Zero-alloc gate present | `grep AllocsPerRun internal/plugins/vrrp/packet/packet_test.go` |
| Golden bytes match spec | `go test -run TestEncodeGolden -v` output; bytes literal in test == hex blocks above |
| Negative table complete | `go test -run TestNegativeReferenceBugs -v` lists N1-N10 subtests |
| No central-package edits | `git diff --stat` touches only the new package + mk/test-fuzz.mk + plan/ |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Every read bounds-checked before access; ladder runs before any address materialization; IHL-aware strip rejects malformed headers; FuzzDecode proves no panic on arbitrary input |
| Spoofing resistance | TTL/hop-limit 255 (GTSM) enforced as ladder row 7; checksum verified with actual src/dst; failures dropped + typed, never processed |
| Resource exhaustion | Zero heap allocations on the rx path (lazy VIP view, N7); count bounded by length exactness; no per-packet goroutines or buffers |
| Error leakage | Typed errors carry reason identity only, never packet bytes; rate-limited logging is the caller's duty (documented) |
| Downgrade/auth | v2 auth types 1/2/unknown discarded (no false-security auth, umbrella decision); version pinned per group (row 5) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read the cited rfc/short section; if the spec table is wrong, fix table + Mistake Log |
| Golden checksum disagreement | Recompute with the in-test reference; if spec hex wrong, correct spec (typo-fix exception) + Mistake Log |
| checksum-rfc9568-message-only counter unexpectedly nonzero in spec-vrrp-6 | A strict-RFC-9568 (message-only) peer is on the segment; verify the keepalived version and capture the bytes into Mistake Log -- the dual-accept absorbs it, no code flip needed |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| v3/IPv4 checksum needs the RFC 5798 pseudo-header (umbrella rfc9568 annotation + holo digest bug 5) | RFC 9568's published text clarifies message-only for IPv4 (rfc/full/rfc9568.txt:880-885, change note :194-196); on paper the pseudo-header looked like legacy RFC 5798 behavior | Coordinator primary-source verification during spec design (before approval/implementation) | Spec revised same day (interim): tx message-only, rx dual-accept + a compat counter; golden vector roles swapped; umbrella annotation corrected by orchestrator. INTERIM ONLY -- the tx-message-only decision was itself reverted 2026-07-15 (see next row); the original pseudo-header assumption turned out correct on the wire |
| tx message-only (RFC 9568 §5.2.8) interoperates with keepalived | keepalived 2.3.1 computes and REQUIRES the RFC 5798 IPv4 pseudo-header and rejects message-only as "Invalid VRRPv3 checksum" | keepalived interop lab wire capture 2026-07-15 (keepalived's own advert checksum 0xa102 = pseudo-header sum, not message-only 0x448e); scripts/evidence/effective-vrrp-keepalived.py | v3/IPv4 tx changed to the RFC 5798 pseudo-header form (checksum.go pseudoSumV4Legacy / FillChecksum:86); rx keeps dual-accept but pseudo-header is now canonical/unflagged and message-only is accepted+flagged (checksum-rfc9568-message-only, validate.go:71); golden roles re-swapped (G2c pseudo-header 0xDEFB = canonical tx, G2 message-only 0x828A = rx-only flagged); code, golden tests, engine and transport all consistent |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- The umbrella's rfc9568 annotation ("pseudo-header v4 AND v6") contradicts both `rfc/short/rfc9568.md:103` and the published RFC 9568 §5.2.8 text (verified 2026-07-14: IPv4 = message only; IPv6 = pseudo-header). Originally captured as A-1/R-5 with the flip pre-costed.
- Resolution (same day, primary sources): rfc/full/rfc9568.txt:880-885 + change note :194-196 confirm message-only for IPv4 as a deliberate RFC 9568 clarification, and the ecosystem followed (keepalived issue #2324, FRR 2022, Cisco always, Arista compliant). On paper the pseudo-header looked like LEGACY RFC 5798 behavior. Interim design: tx message-only, rx dual-accept + a compat counter. Umbrella annotation corrected by the orchestrator. (This interim was reverted 2026-07-15 -- see the next bullet.)
- Second flip (2026-07-15, keepalived interop capture): tx reverted to the RFC 5798 pseudo-header form. keepalived 2.3.1 computes and REQUIRES the IPv4 pseudo-header and rejects message-only as "Invalid VRRPv3 checksum"; its own advert on the wire carried checksum 0xa102 (pseudo-header), not 0x448e (message-only). For a first-hop-redundancy feature, interoperating with the installed base outranks the RFC 9568 clarification. The one-function isolation absorbed this second flip too: tx (`FillChecksum`) emits the pseudo-header form, rx (`verifyReceived`) takes the pseudo-header sum as primary/canonical and flags the message-only sum as checksum-rfc9568-message-only (checksum.go:97-110). Proven by scripts/evidence/effective-vrrp-keepalived.py.
- Computing the rx checksum over the FULL received payload (not 8+count*size) makes the "v3 packet with v2 auth trailer" failure surface as an actionable length error instead of a mystery checksum error. Confirmed in code: N2 (zero auth trailer appended) passes checksum because zeros do not change the one's-complement sum, then fails row-9 length — exactly the intended actionable path.
- The buffer-free rx verify trick (sum message+checksum, expect fold to 0xFFFF) only holds when the checksum field sits on a 16-bit word boundary. Real VRRP messages are always even-length with the checksum at byte 6, so this is always true on the wire; a unit test that appended the checksum after odd-length synthetic input exposed the alignment assumption (test padded to fix; implementation unaffected).
- All four golden checksums (G1 0x92ED v2, G2 0x828A v3/IPv4 message-only rx-only, G2c 0xDEFB v3/IPv4 RFC 5798 pseudo-header = ze's tx form, G3 0x3F5D v6 pseudo-header) reproduce exactly from the single isolated RFC 1071 core; both the rx dual-accept and the tx-form choice live in `verifyReceived`/`FillChecksum` as predicted by the Core Insight.

## Core Insight

Every studied implementation disagrees somewhere on v3/IPv4 checksum scope; the
defense was isolating the computation in one function and pre-computing both
golden sums. That paid off twice: primary sources first pointed to RFC 9568
message-only (rfc/full/rfc9568.txt:880-885), then the 2026-07-15 keepalived
capture proved the installed base still requires the RFC 5798 pseudo-header, so
ze transmits the pseudo-header form and dual-accepts both on rx. Both flips were
contained in the one isolated function (checksum.go `verifyReceived` /
`FillChecksum`) instead of a redesign.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Internal unit = milliseconds; conversion only inside encode/decode | Native wire units per version (cs/s) | Umbrella R-2: kills the s/cs/ms defect class (holo 100x bug); one conversion site per direction |
| Lazy VIP view over the rx buffer + AppendVIPs copy helper; zero-alloc decode | Materialize []netip.Addr per packet | At 1 pkt/s an allocation would be tolerable (buffer-first.md governs encode, not decode), but laziness is free here, matches the bfd value-decode precedent, and bounds hostile count=255 packets (N7); the cost is the A-3 lifetime contract, paid once in documentation |
| VRID lookup (row 4) before checksum (row 6) | RFC §7.1 listing order (TTL/version/length/checksum first) | §7.1 mandates that all checks pass, not their order; multicast delivers every group's traffic to everyone -- skipping checksum for unknown vrids is O(1) rejection of the common non-error case, and vrid-mismatch stays a separately-counted outcome |
| Checksum over full payload; length exactness after | Checksum over computed 8+count*size span | Buggy-peer trailers (uvrrpd v3) then fail with reason `length` (actionable) rather than `checksum` (mystery) |
| WriteTo writes checksum-zero + separate FillChecksum backfill | WriteTo computes inline | Skip-and-backfill per buffer-first.md; lets transport skip software checksum when offloading v6 tx via IPV6_CHECKSUM while the software path remains for v4/tests/rx |
| ~~v3/IPv4 tx message-only (RFC 9568 §5.2.8), interim design~~ | ~~strict-9568-only; verify-both-accept-either~~ | Superseded 2026-07-15 -- the message-only tx broke keepalived 2.3.1 interop; replaced by the pseudo-header tx row below (the original A-1 pseudo-header assumption was correct on the wire) |
| v3/IPv4: tx RFC 5798 pseudo-header sum (keepalived interop); rx dual-accept -- primary pseudo-header sum (canonical, unflagged), fallback RFC 9568 message-only sum ACCEPTED and flagged checksum-rfc9568-message-only | tx message-only per RFC 9568 §5.2.8 (interim design 2026-07-14; broke keepalived 2.3.1, which requires the pseudo-header -- reverted 2026-07-15); strict-9568-only (silently breaks the installed base); YANG knob selecting the sum (needless config surface when dual-accept + counter gives correctness AND visibility) | keepalived 2.3.1 wire capture 2026-07-15 (its advert checksum 0xa102 = pseudo-header, rejects message-only as "Invalid VRRPv3 checksum"; checksum.go:97-110); the deployed base (older FRR, hardware VRRP, uvrrpd) does the same; the one-function isolation makes both the tx form and the rx dual-accept a contained change |
| Typed Err* sentinels + exported Reason(err); counters registered elsewhere | Codec-owned prometheus counters | Plugin self-containment: pure codec stays import-free; children 4/5 own registration; taxonomy fixed here so no dead/ad-hoc reasons appear later |
| Lookup as caller-supplied function returning Local{Version, AdverIntervalMS} | Decode without local knowledge + second validate pass | The ladder needs group version (row 5) and v2 local interval (row 12) to run in the specified order in one pass; a function value keeps the codec free of group-table types |
| v2 zero interval rejected structurally (row 11) | Let row 12 mismatch-discard catch it | Same observable behavior (a legal local config is >= 1000 ms), more precise metrics reason; documented as stricter-than-RFC |

## Known Limitations
- v2 authentication not implemented (auth type 0 only; types 1/2/unknown discarded) -- umbrella decision, RFC 9568 removed auth by design.
- v2 optional address-list comparison (RFC 3768 §7.1 MAY-check with MUST-drop on non-owner mismatch) is NOT in the ladder. ~~it needs the local VIP list, which the FSM owns; the lazy VIP accessors exist so spec-vrrp-2 can implement it; recorded as an explicit input to spec-vrrp-2~~ (corrected 2026-07-14 cross-review) It is performed by the ENGINE post-Decode: spec-vrrp-5 holds both the configured VIPs and the packet's VIPAt accessors. v2-only; mismatch = drop + reason `address-list` (engine-raised label in the taxonomy). spec-vrrp-5 owns that check.
- Owner-receives-advert handling (RFC 9568 erratum 8298: log, do not discard) is FSM policy (spec-vrrp-2); the codec neither knows nor checks local priority.
- v2 owner-receive deviation (looser-than-RFC): RFC 3768 §7.1 makes "the local router is not the IP address owner" a MUST-discard receive check; ze applies the v3 erratum-8298 log-only treatment to BOTH versions, deferring owner semantics to the FSM -- an owner (priority 255) never yields the Active/Master state, so the observable outcome matches the RFC's intent. Same documentation pattern as the stricter-than-RFC ladder row-11 note.
- No rate-limited logging in the codec; the engine/transport (children 4/5) own log throttling per reason.
- RFC 9568 §8.4.2 v2/v3 interop (dual-send/dual-listen) mode out of scope for the umbrella; row 5 pins one version per group.

## RFC Documentation

Add `// RFC 9568 Section X.Y: "<quoted requirement>"` (or RFC 3768) above enforcing code:
- validate.go ladder: one citation per row exactly as tabled (incl. errata 8299/8300/8301 ids; row 12 quotes both the v2 MUST-discard and the v3 no-drop sentences)
- checksum.go: RFC 9568 §5.2.8 quoted ("only includes the VRRP message"), plus a comment documenting that tx emits the RFC 5798 pseudo-header form for keepalived interop and rx dual-accepts (pseudo-header primary, RFC 9568 message-only accepted+flagged), with the 2026-07-15 keepalived capture rationale; RFC 3768 §5.3.7; RFC 8200 §8.1 for the v6 pseudo-header
- packet.go: §5.2.6 reserve-zero, §5.2.5 count, §5.2.7/§5.3.7 interval fields, RFC 3768 §5.3.10 auth-data-zero, §7.3 virtual MAC constants
- StripIPv4Header: GTSM rationale RFC 9568 §9 / RFC 5082 reference on the TTL extraction

## Implementation Summary

### What Was Implemented
- New leaf package `internal/plugins/vrrp/packet/` (stdlib + `net/netip` only): `packet.go`
  (294L), `checksum.go` (129L), `validate.go` (275L) + tests `packet_test.go` (430L),
  `checksum_test.go` (88L), `validate_test.go` (581L), `fuzz_test.go` (119L).
- `Advertisement` value struct + value-receiver `WriteTo(buf,off) int`, `FillChecksum`,
  `Decode`, `StripIPv4Header`, lazy VIP accessors (`VIPCount`/`VIPAt`/`AppendVIPs`),
  `Validate`, `Reason`, `VirtualMAC`, constants (`ProtoNumber`, `MulticastV4/V6`, MAC
  prefixes, `MaxLenV2=80`/`MaxLenV3v4=72`/`MaxLenV3v6=264`, `V4`/`V6`).
- RFC 1071 checksum with the v3/IPv4 dual-accept isolated in `verifyReceived`: RFC 5798
  pseudo-header primary (ze's tx form, keepalived interop), RFC 9568 message-only fallback
  (accept + `MsgOnlyChecksum` + `ReasonMsgOnlyChecksum` = "checksum-rfc9568-message-only");
  v3/IPv6 RFC 8200 pseudo-header; v2 message-only. tx (`FillChecksum`) emits the pseudo-header form.
- 13-row ordered ladder with per-row RFC citations; ms is the only internal unit, wire
  conversion isolated in four helpers (`msToV3Centiseconds`/`v3CentisecondsToMS`/
  `msToV2Seconds`/`v2SecondsToMS`).
- `mk/test-fuzz.mk`: appended `FuzzDecode` line for `./internal/plugins/vrrp/packet/...`.
- **TDD evidence.** RED (tests-first, build fails, no implementation):
  `# ... undefined: Advertisement / RxMeta / Lookup / checksum16 ... [build failed]`
  (tmp/vrrp-red.log). GREEN: `go test -race -count=1 ./internal/plugins/vrrp/packet/...`
  → `ok ... 1.349s`; verbose: 23 tests / 113 subtests PASS, 0 FAIL. All four golden
  vectors byte-exact (G1 0x92ED v2, G2c 0xDEFB v3/IPv4 pseudo-header = tx form, G2 0x828A
  message-only accept-with-MsgOnlyChecksum, G3 0x3F5D v6). Zero-alloc: `TestDecodeZeroAlloc`
  v2/v3v4/v3v6 = 0 allocs/run; N7 hostile
  count=255 also 0 allocs. Fuzz: `FuzzDecode` 10s = 4.6M execs, 0 crashes (tmp/vrrp-fuzz.log).
  golangci-lint scoped to the package = 0 issues.

### Bugs Found/Fixed
- No production bugs found in this child (new package). One TEST bug found and fixed while
  going green: the checksum round-trip invariant (sum of message+checksum folds to 0xFFFF)
  only holds when the checksum field sits on a 16-bit boundary. Appending the 2-byte
  checksum after an ODD-length fixture misaligns the words and fails to fold. Real VRRP
  messages are always even-length with the checksum at byte 6, so the implementation is
  correct; the test now pads an odd input with the same implicit zero byte the checksum
  computation used before appending (`checksum_test.go`).

### Documentation Updates
- `mk/test-fuzz.mk` self-documents the new `FuzzDecode` target (Documentation Update
  Checklist row 10). No other docs in scope for this child (rfc-status/guide rows land
  with spec-vrrp-5/6 per the checklist).

### Deviations from Plan
1. **Per-file test split (forced by the test-first hook).** Files to Create listed a single
   `packet_test.go` (+ `fuzz_test.go`). The `c_require_test_first` pre-write hook maps every
   `X.go` to `X_test.go`, so `checksum.go` and `validate.go` required `checksum_test.go` and
   `validate_test.go`. Every spec-listed test exists; only file placement is distributed
   (checksum tests in `checksum_test.go`; decode/ladder/negative/strip/reason tests in
   `validate_test.go`; encode/round-trip/boundary/zero-alloc/bench in `packet_test.go`).
   No AC or test dropped. Justification: hard hook constraint; tests-next-to-source is also
   cleaner Go.
2. **A-4 encode range check realized as `Validate() error`, not an inline WriteTo return.**
   The Encode contract fixes `WriteTo(buf,off) int` (the transport/FSM sibling contract),
   which cannot return an error, while A-4 asks encode to "assert and return a typed
   `ErrIntervalRange`". Resolution: `WriteTo` stays a pure buffer-first writer with a
   documented "validated Advertisement" precondition (only the natural short-buffer bounds
   panic, like bfd); the typed encode errors (`ErrIntervalRange`, plus `ErrCountRange`,
   `ErrVRIDRange`) are returned by a value-receiver `Validate()` (defense-in-depth, the
   engine calls it, boundary tests assert it). `Reason()` deliberately does NOT map these
   encode-side errors — they can never label a received packet — so it stays total and
   injective over the RX taxonomy; `TestErrorReasonMapping` asserts the RX inventory maps
   exactly and that encode errors + nil map to "". No behavior lost; this is the only
   realization consistent with the fixed `int` signature and the no-dead-reason rule.
3. **All `Advertisement` methods are value receivers** (WriteTo was already value): required
   so `Validate()` works on non-addressable values (e.g. `mk(16).Validate()`) and keeps the
   receiver set consistent. Still zero-alloc (stack copy of scalars + two slice headers;
   `TestDecodeZeroAlloc` proves 0 allocs).
4. **v3/IPv4 tx checksum: RFC 5798 pseudo-header, not RFC 9568 §5.2.8 message-only.** The plan
   (interim, 2026-07-14) had ze transmit the RFC 9568 message-only sum. A keepalived 2.3.1 wire
   capture on 2026-07-15 (`scripts/evidence/effective-vrrp-keepalived.py`) proved keepalived
   computes and REQUIRES the RFC 5798 IPv4 pseudo-header and rejects message-only as "Invalid
   VRRPv3 checksum" (its own advert checksum 0xa102 = pseudo-header sum, not message-only 0x448e).
   ze therefore TRANSMITS the pseudo-header form (`checksum.go` FillChecksum / `pseudoSumV4Legacy`)
   and rx dual-accepts (pseudo-header canonical; RFC 9568 message-only accepted and flagged
   `MsgOnlyChecksum` / checksum-rfc9568-message-only). This is a deliberate divergence from strict
   RFC 9568 §5.2.8 for first-hop-redundancy interop with the installed base; the message-only sum
   stays accepted on rx so a strict-RFC-9568 peer still interoperates. No AC dropped: AC-2 golden
   bytes and AC-10 dual-accept updated to match; golden vector roles re-swapped (G2c pseudo-header
   = canonical tx, G2 message-only = rx-only flagged).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Encode/decode RFC 9568 v3 (IPv4+IPv6) + RFC 3768 v2 ADVERTISEMENT | Done | `packet.go` WriteTo:238, `validate.go` Decode:105 | Byte-exact golden encode+decode |
| Full ordered receive-validation ladder (13 rows) | Done | `validate.go:105-243` | `TestValidationOrder` all adjacent pairs |
| Typed error taxonomy 1:1 with `ze_vrrp_packet_errors_total{reason}` | Done | `validate.go` Err* + Reason:82 | `TestErrorReasonMapping` total+injective over RX taxonomy |
| IHL-aware IPv4 header-strip helper | Done | `validate.go` StripIPv4Header:248 | `TestStripIPv4HeaderIHL` IHL 5/6/15 + errors |
| Golden byte vectors | Done | `packet_test.go` goldenV2/V3v4/V3v4Compat/V3v6 | G1/G2/G2c/G3 hand-verified + independent RFC 1071 ref |
| Negative tests from holo/uvrrpd bugs | Done | `validate_test.go` TestNegativeReferenceBugs | N1-N10 subtests |
| Boundary tests | Done | `packet_test.go` + `validate_test.go` | vrid/priority/count/interval, both wire ranges |
| Fuzz target in `make ze-fuzz-test` | Done | `fuzz_test.go` FuzzDecode + `mk/test-fuzz.mk` | 10s = 4.6M execs clean |
| Zero-allocation happy-path decode | Done | `validate.go` Decode (lazy VIP view) | `TestDecodeZeroAlloc` = 0 allocs |
| No sockets/build-tags/netlink/goroutines; stdlib+netip only | Done | package imports | `errors`, `net/netip` only |
| ms internal unit; conversion only at wire boundary | Done | `packet.go` 4 conversion helpers | R-2 grep clean; N10 exact at 4095 cs |
| v3/IPv4 tx RFC 5798 pseudo-header + rx dual-accept (A-1) | Done | `checksum.go` FillChecksum:86 / verifyReceived:130 | `TestDecodeV3IPv4MsgOnlyChecksumCompat` |
| MaxLen constants exported for transport | Done | `packet.go:71-75` | 80/72/264, `TestConstants` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestEncodeGoldenV2` | G1 == 0x92ED, auth 0, seconds byte, zero trailer |
| AC-2 | Done | `TestEncodeGoldenV3IPv4` / `V3IPv6` | G2c == 0xDEFB RFC 5798 pseudo-header (tx form), G3 == 0x3F5D v6 pseudo-header |
| AC-3 | Done | `TestDecodeGoldenV2/V3IPv4/V3IPv6` | field-equal; AdverIntervalMS == 1000 all three |
| AC-4 | Done | `TestValidationOrder` (13 subtests) | earliest violated row wins, every adjacent pair |
| AC-5 | Done | `TestDecodeV3IntervalMismatchNotError`, `TestDecodeV2IntervalMismatchDiscard` | v3 surfaces value, v2 → ErrV2IntervalMismatch |
| AC-6 | Done | `TestStripIPv4HeaderIHL` | IHL 5/6/15 strip, IHL<5 + header>datagram → typed errors |
| AC-7 | Done | `FuzzDecode` 10s | no panic, no accepted packet violating ladder invariants |
| AC-8 | Done | `TestDecodeZeroAlloc` + N7 | 0 allocs incl. VIPAt and hostile count=255 |
| AC-9 | Done | `TestBoundaryVRID/Count/Interval/PriorityAndAppendVIPs` | every Last-Valid accepted, Invalid rejected |
| AC-10 | Done | `TestDecodeV3IPv4MsgOnlyChecksumCompat`, N1/N1b | G2c (pseudo-header) accepted unflagged; G2 (message-only) accept+MsgOnlyChecksum; both-fail → ErrChecksum |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestEncodeGoldenV2/V3IPv4/V3IPv6 | Done | `packet_test.go` | byte-exact |
| TestDecodeGoldenV2/V3IPv4/V3IPv6 | Done | `validate_test.go` | field-equal |
| TestRoundTrip | Done | `packet_test.go` | {v2,v3v4,v3v6}×{1,3,16}×{0,1,100,254,255} = 45 combos |
| TestWriteToOffset | Done | `packet_test.go` | non-zero offset honored |
| TestChecksumRFC1071 | Done | `checksum_test.go` | independent reference + odd length + fold invariant |
| TestValidationOrder | Done | `validate_test.go` | 13 subtests |
| TestErrorReasonMapping | Done | `validate_test.go` | total+injective over RX taxonomy |
| TestDecodeV3IntervalMismatchNotError / V2...Discard | Done | `validate_test.go` | v3/v2 split |
| TestDecodeV3IPv4MsgOnlyChecksumCompat | Done | `validate_test.go` | N1/N1b |
| TestStripIPv4HeaderIHL | Done | `validate_test.go` | AC-6 |
| TestNegativeReferenceBugs | Done | `validate_test.go` | N1-N10 |
| TestBoundaryVRID/Priority/Count/Interval | Done | `packet_test.go` + `validate_test.go` | Priority folded into TestBoundaryPriorityAndAppendVIPs |
| TestDecodeZeroAlloc | Done | `packet_test.go` | AllocsPerRun == 0 |
| BenchmarkDecode/Encode | Done | `packet_test.go` | allocation regression guard |
| FuzzDecode | Done | `fuzz_test.go` | seed corpus = goldens + per-row mutations |
| TestFillChecksumFamilies, TestConstants | Done (extra) | `checksum_test.go`, `packet_test.go` | family checksum selection + constant drift guard |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/vrrp/packet/packet.go` | Done | 294L; Advertisement, constants, WriteTo, conversions, Validate |
| `internal/plugins/vrrp/packet/checksum.go` | Done | 129L; RFC 1071, pseudo-headers, FillChecksum, verifyReceived dual-accept |
| `internal/plugins/vrrp/packet/validate.go` | Done | 275L; RxMeta/Lookup/Local, Decode ladder, Err*, Reason, StripIPv4Header |
| `internal/plugins/vrrp/packet/packet_test.go` | Done | 430L |
| `internal/plugins/vrrp/packet/fuzz_test.go` | Done | 119L |
| `internal/plugins/vrrp/packet/checksum_test.go` | Done (added) | 88L; forced by test-first hook (Deviation 1) |
| `internal/plugins/vrrp/packet/validate_test.go` | Done (added) | 581L; forced by test-first hook (Deviation 1) |
| `mk/test-fuzz.mk` | Done | appended FuzzDecode line |

### Audit Summary
- **Total items:** 13 requirements + 10 ACs + 8 planned files + full TDD test set
- **Done:** all requirements, all AC-1..AC-10, all planned tests (+2 extra tests, +2 test files)
- **Partial:** none
- **Skipped:** none
- **Changed:** 3 deviations documented in Deviations from Plan (test-file split, Validate() encode-error path, value receivers) — none reduce scope; no AC dropped

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Byte-exact v2/v3 encode per RFC wire formats | unit golden tests | TestEncodeGoldenV2/V3IPv4/V3IPv6 pass; bytes == G1 0x92ED / G2c 0xDEFB (RFC 5798 pseudo-header tx) / G3 0x3F5D (packet_test.go) |
| Ordered receive validation with typed, countable failures | unit tests | TestValidationOrder (13 adjacent-pair subtests) + TestErrorReasonMapping (total+injective over the RX taxonomy incl. checksum-rfc9568-message-only) pass (validate_test.go) |
| Reference-implementation bugs provably not replicated | negative tests | TestNegativeReferenceBugs N1-N10 pass (validate_test.go) |
| Robust under adversarial input | fuzz run | FuzzDecode 10s = 4.6M execs, 0 crashes (make ze-fuzz-test; fuzz_test.go) |
| Zero-allocation happy-path decode | alloc test | TestDecodeZeroAlloc v2/v3v4/v3v6 = 0 allocs/run incl. hostile count=255 (packet_test.go) |
| Wire-true against a real peer | interop capture (spec-vrrp-6) | keepalived 2.3.1 accepts ze's v3/IPv4 pseudo-header adverts and ze decodes keepalived's on the canonical rx path (scripts/evidence/effective-vrrp-keepalived.py); checksum-rfc9568-message-only counter == 0 with keepalived |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | v3/IPv4 checksum spec text described the reverted message-only design while the code ships the RFC 5798 pseudo-header form; AC-2 / AC-10 unsatisfied-as-written | Key Design Decisions, AC-2, AC-10, checksum table | Reconcile spec text to the shipped pseudo-header design (code unchanged, interop-proven) |
| 2 | BLOCKER | Mistake Log missing the second flip (the 2026-07-15 revert of the interim message-only design back to the pseudo-header form) | Mistake Log | Add the second-flip Mistake Log row |
| 3 | ISSUE | Reason-label drift between the ladder table and the taxonomy prose | validate.go taxonomy section | Align spelling to checksum-rfc9568-message-only |
| 4 | ISSUE | RFC 9568 5.2.8 tx deviation not called out as a deliberate deviation | Deviations / checksum.go | Record tx pseudo-header as a deliberate RFC 9568 5.2.8 deviation for keepalived interop |

### Fixes applied
- Spec text (Key Design Decisions, A-1, AC-2, AC-10, checksum table) reconciled to the shipped RFC 5798 pseudo-header tx form; code left unchanged (already interop-proven against keepalived 2.3.1).
- Mistake Log gained the second-flip row (interim message-only reverted to the pseudo-header form after the 2026-07-15 keepalived capture).
- Reason-label spelling aligned; RFC 9568 5.2.8 tx deviation recorded as deliberate.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | 3 stale test-comment NOTEs still referencing the interim message-only canonical form | packet_test.go comments | Reworded (since fixed) |

### Final status
**Run 2 CLEAN: 0 BLOCKER, 0 ISSUE.** Run 1 (2 BLOCKER, 2 ISSUE) all fixed by reconciling the spec to the shipped, interop-proven design. NOTEs: 3 stale test-comment NOTEs from Run 2, since fixed.
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test (cross-child rows verified at umbrella close)
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/vrrp/packet/`, `mk/test-fuzz.mk`)
- [ ] Integration completeness proven (FuzzDecode runs in make ze-fuzz-test; child 2/4 seams documented)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: A-2, A-4, A-5 confirmed in this child; A-1 resolved at design time (2026-07-14, primary sources -- wire confirmation rides spec-vrrp-6); A-3 carries to spec-vrrp-2 with the carry recorded in the umbrella (none silently `unvalidated` at close)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (RFC Documentation section)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior (via spec-vrrp-5 .ci suite, referenced above)
- [ ] Interop tests for protocol features (fixtures here, execution spec-vrrp-6 -- umbrella-approved split)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-vrrp-1-packet.md`
- [ ] **Commit A:** code + tests + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-vrrp-1-packet.md` only
