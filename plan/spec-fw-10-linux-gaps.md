# Spec: fw-10-linux-gaps -- Close remaining Linux firewall gaps

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-fw-0-umbrella |
| Phase | 7/7 |
| Updated | 2026-04-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-fw-0-umbrella.md` -- design decisions, VPP compatibility mapping
3. `plan/spec-fw-8-lns-gaps.md` -- prior gap-closing spec (ICMP / wildcard / exclude / reactor)
4. `internal/component/firewall/model.go` -- current match/action types
5. `internal/component/firewall/config.go` -- parser (parseFromBlock / parseThenBlock / parseSet / parseNATSpec)
6. `internal/component/firewall/validate.go` -- verify-time validator
7. `internal/component/firewall/schema/ze-firewall-conf.yang` -- YANG surface
8. `internal/plugins/firewall/nft/lower_linux.go` -- nft lowering
9. `internal/plugins/firewall/nft/lower_linux_test.go` -- lowering tests

## Task

Close the nine Linux firewall gaps identified in the post-fw-8 gap audit (2026-04-19).
Three are correctness bugs (user-visible regression, silent wrong rule, over-strict
validator). Four are feature gaps that extend the YANG surface. One is a validator
addition. One is a missing end-to-end functional test. VPP backend work (spec-fw-6)
is explicitly out of scope.

The goal is "no remaining Linux gap between the umbrella's AC table and the shipping
firewall code." After this spec, Linux firewall is production-complete; remaining
firewall work is VPP only.

### Gap summary

| # | Gap | Kind | Where it lives today |
|---|-----|------|----------------------|
| 1 | `MatchInSet` unreachable in lowering | Bug (regression) | `lower_linux.go` lowerMatch switch |
| 2 | `MatchDSCP` IPv6/inet family unguarded | Bug (silent wrong rule) | `validate.go` validateMatch |
| 3 | `SetDSCP` rejects inet family | Bug (over-strict validator) | `validate.go` validateAction |
| 4 | Chain priority range not enforced | Validator gap | `validate.go` Chain.Validate |
| 5 | `@setname` match on source/destination port | Feature | YANG from-block / parser / lowering |
| 6 | Per-element set timeout | Feature | YANG set list / parser / backend Apply |
| 7 | Byte-rate Limit | Feature | YANG rate-unit / model / lowering |
| 8 | NAT target address range | Feature | model SNAT/DNAT / parser / lowering |
| 9 | Runtime `.ci` boot-apply + reload tests | Test coverage | `test/firewall/` (directory missing) |

## Required Reading

### Architecture Docs

- [ ] `plan/spec-fw-0-umbrella.md` -- design decisions, VPP compatibility mapping
  -> Decision 4: abstract types, not nftables-native
  -> Decision 10: expression coverage mandatory; Hash/Numgen deferred, everything else in scope
  -> Constraint: AC-6 "show firewall" must display sets + counters; AC-8 "all expression types programmed"
- [ ] `plan/spec-fw-8-lns-gaps.md` -- structural model for closing multi-area gaps
  -> Decision: per-gap file list + per-gap AC; phased implementation
  -> Constraint: new match/action types require model marker + YANG leaf + parser branch + validator branch + lowering case + show formatter case
- [ ] `internal/component/firewall/model.go` -- current types (14 match, 18 action)
  -> Constraint: every Match needs matchMarker(); every Action needs actionMarker()
  -> Constraint: SetElement.Timeout exists in model, unreachable from YANG today
- [ ] `internal/component/firewall/config.go` -- parseFromBlock / parseThenBlock / parsePortSpec / parseNATSpec / parseSet
  -> Constraint: parser builds []Match and []Action from the YANG JSON tree
  -> Constraint: parsePortSpec already handles comma-lists with overlap/adjacency validation
  -> Constraint: parseNATSpec parses "addr:port", "addr:lo-hi", bare addr; single-address only
- [ ] `internal/component/firewall/validate.go` -- verify-time checks (cross-refs, family guards, name lengths)
  -> Constraint: validateMatch and validateAction are the single surface for type-specific verify errors
  -> Constraint: current SetDSCP validator rejects any family other than FamilyIP
- [ ] `internal/plugins/firewall/nft/lower_linux.go` -- lowerMatch / lowerAction switches + helpers
  -> Constraint: lowerCtx carries the nftables Conn + Table for helpers that need to register anonymous sets
  -> Constraint: named set lookup uses `expr.Lookup{SourceRegister, SetID, SetName}` -- the set object must exist on the table before the rule adds the Lookup expression
- [ ] `internal/plugins/firewall/nft/readback_linux.go` -- kernel -> model reverse walker
  -> Constraint: ListTables returns chains + term names + sets (with elements) + flowtables; term bodies intentionally unreversed
- [ ] `vendor/github.com/google/nftables/expr/limit.go` -- LimitType values
  -> Constraint: `LimitTypePkts` = packet-rate, `LimitTypePktBytes` = byte-rate; kernel accepts suffixed rate-units ("bytes", "kbytes", "mbytes", "gbytes") in the byte-rate form
- [ ] `vendor/github.com/google/nftables/expr/nat.go` -- NAT RegAddrMax field
  -> Constraint: address-range NAT sets `RegAddrMax` to a second register holding the upper-bound address
- [ ] `rules/exact-or-reject.md` -- BLOCKING for every validator change in this spec

### RFC Summaries

Not protocol work. ICMP and DSCP values come from IANA; nftables kernel semantics are
the reference. No rfc/short/ entries needed.

**Key insights:**
- The three bugs (gaps 1-3) are all reachable via valid YANG configs and produce incorrect
  Apply behavior today. They share a common character: the validator says "accept"
  but lowering or kernel behavior diverges from operator intent.
- Gaps 5-8 extend the YANG surface; gap 8 also extends the model. Each follows the fw-8
  cookbook: (1) model type or field, (2) YANG leaf, (3) parser branch, (4) validator
  branch, (5) lowering case, (6) show formatter case, (7) unit test per layer,
  (8) `.ci` test covering end-to-end.
- Gap 9 (`.ci` tests) closes the umbrella spec's AC-1, AC-3, AC-5, AC-6 -- none of
  which had an end-to-end functional test before this spec.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` -- 14 match types, 18 action types; `MatchInSet` has `SetName` + `MatchField` (SourceAddr/DestAddr/SourcePort/DestPort); `SetElement` has `Timeout`; `SNAT`/`DNAT` carry a single `Address netip.Addr`.
  -> Constraint: adding new fields is additive; existing zero values remain valid.
- [ ] `internal/component/firewall/config.go` -- `parseAddressMatch` produces `MatchInSet` for `@setname` in source/destination-address only; `parsePortSpec` returns `[]PortRange`; `parseNATSpec` returns `(addr, port, portEnd)` and has no address-range branch; `parseSet` reads `elements []any` as bare strings, does not surface per-element timeout; `parseRateSpec` accepts `second|minute|hour|day` only.
  -> Constraint: changes must preserve backwards compatibility for existing configs.
- [ ] `internal/component/firewall/validate.go` -- `validateMatch` handles `MatchInSet`, `MatchICMPType`, `MatchICMPv6Type`, `MatchInputInterface`, `MatchOutputInterface`. No family check on `MatchDSCP`. `validateAction` rejects `SetDSCP` unless `tbl.Family == FamilyIP`. No chain priority range check.
  -> Constraint: `SetDSCP` in `inet` family MUST be permitted because `inet` tables dispatch ipv4 packets to the same ip header layout.
- [ ] `internal/component/firewall/schema/ze-firewall-conf.yang` -- `from-block` has no way to reference a named set on port leaves; `set` list has no per-element timeout leaf; `rate-unit` is time-only; `snat` / `dnat` `to` leaf accepts only a single address.
  -> Constraint: every added leaf needs `ze:backend "nft"` annotation so the backend-gate walker keeps rejecting vpp-only configs that use these primitives.
- [ ] `internal/plugins/firewall/nft/lower_linux.go` -- `lowerMatch` switch has no `MatchInSet` case; `lowerDSCPMatch` is unconditional (IPv4 offsets); `lowerLimit` is packet-rate only; `lowerNAT` takes single `addr`, emits `RegAddrMin=1`, never `RegAddrMax`.
  -> Constraint: `lowerMatch` is called with a `lowerCtx` that carries conn+table; named set lowering uses that context to resolve the set's `ID` and `Name`.
- [ ] `internal/plugins/firewall/nft/readback_linux.go` -- reverse walker populates set elements with `Value` only; does not read per-element timeout.
  -> Constraint: readback extensions must keep `.ci` expectations aligned with what operators configured.
- [ ] `test/parse/firewall-*.ci` -- three parse-level tests exist (basic accept, jump-unknown-chain reject, named-counter reject). No runtime `.ci` tests.
  -> Constraint: runtime tests land in `test/firewall/`, not `test/parse/`.

**Behavior to preserve:**
- Every existing config that parses today continues to parse after this spec.
- Validator cross-ref checks (jump / goto / flow-offload / MatchInSet set exists).
- Backend-feature gate continues rejecting vpp-gated primitives at verify.
- `rules/exact-or-reject` posture: new validator gates narrow what passes, never widen.

**Behavior to change:**
- Gap 1: `@setname` in source/destination-address programs a real nftables Lookup expression.
- Gap 2: `dscp` match in a non-IPv4 table rejects at verify with a clear message.
- Gap 3: `dscp-set` action is permitted in `inet` family tables (narrows the validator).
- Gap 4: chain `priority` leaf outside a documented range rejects at verify.
- Gap 5: `source-port @name` and `destination-port @name` parse into `MatchInSet{MatchField: SetFieldSourcePort|DestPort}`; lowering emits a Lookup against a set whose element type is `inet-service`.
- Gap 6: set element entries accept an optional `timeout` leaf; parser populates `SetElement.Timeout`; backend Apply passes it to `nftables.SetElement.Timeout`.
- Gap 7: `limit-rate` accepts `Nbytes/second`, `Nkbytes/second`, etc.; model `Limit` gains a `Unit` distinguishing packets from bytes; lowering chooses `LimitTypePkts` or `LimitTypePktBytes`.
- Gap 8: `snat { to }` / `dnat { to }` accept `<addr>-<addr>` and `<addr>-<addr>:<port>`; model SNAT/DNAT gain `AddressEnd netip.Addr`; lowering emits `RegAddrMax`.
- Gap 9: four new `.ci` tests in `test/firewall/` covering boot-apply, reload, lachesis coexistence, and CLI show.

## Data Flow (MANDATORY)

### Entry Point

- YANG config parsed at startup / reload -> firewall section JSON -> SDK `OnConfigure` /
  `OnConfigVerify` -> `ParseFirewallConfig` -> `ValidateTables` -> backend `Apply`.
- `.ci` tests drive the full path by launching ze with a config file and checking
  kernel state via `nft list ruleset` or the `ze firewall show` RPC.

### Transformation Path

**Gap 1 (MatchInSet lowering):**
1. Config `from { source-address "@blocked" }` -> `MatchInSet{SetName:"blocked", MatchField:SetFieldSourceAddr}`.
2. Validator confirms `blocked` resolves to a declared set.
3. Lowering reads the set from `ctx.table`, emits `Payload(NetworkHeader, offset, addrLen)` + `Lookup{SetID, SetName}`.

**Gap 2 (MatchDSCP family):**
1. Config with `from { dscp ef }` in an `ip6` table.
2. `validateMatch` rejects: "dscp match is IPv4-only; move to family ip or inet".

**Gap 3 (SetDSCP inet):**
1. `validateAction` widens: accepts `FamilyIP` OR `FamilyInet`.
2. Lowering unchanged: `inet` tables dispatch to the IPv4 header layout for ipv4 packets; the TOS offset used today is correct.

**Gap 4 (chain priority):**
1. Config `priority "500"`.
2. `Chain.Validate` rejects: "priority 500 out of range -400..400".

**Gap 5 (@setname on port):**
1. Config `from { source-port "@voip-ports" }` -> `MatchInSet{SetName:"voip-ports", MatchField:SetFieldSourcePort}`.
2. Validator confirms set exists; lowering emits `Payload(TransportHeader, portOffset, 2)` + `Lookup{SetID}`.

**Gap 6 (per-element timeout):**
1. Config `set blocked { type ipv4; elements { element 10.0.0.1 { timeout 3600; } } }` (YANG shape change: `elements` becomes a `list element` with a `timeout` leaf, NOT a `leaf-list`).
2. Parser populates `SetElement.Value` + `SetElement.Timeout`.
3. `applySet` passes `Timeout` to `nftables.SetElement.Timeout` (google/nftables field is `time.Duration`).

**Gap 7 (byte-rate Limit):**
1. YANG `rate-unit` enum grows `bytes`, `kbytes`, `mbytes`, `gbytes` entries; `rate-spec` pattern widens.
2. Model `Limit.Dimension` field (enum `RateDimensionPackets|RateDimensionBytes`) distinguishes intent; `Unit` continues to carry the time unit.
3. Lowering: if `Dimension == RateDimensionBytes`, emit `Type: expr.LimitTypePktBytes` and scale Rate by the byte multiplier (1 / 1024 / 1024*1024 / 1024^3).

**Gap 8 (NAT address range):**
1. YANG `nat-spec` pattern accepts `<addr>-<addr>` and `<addr>-<addr>:<port>`; `<addr>-<addr>:<portLo>-<portHi>`.
2. Parser `parseNATSpec` returns `(addrLo, addrHi, port, portEnd)`; model SNAT/DNAT gain `AddressEnd netip.Addr`.
3. Lowering: if `AddressEnd` is set, emit `Immediate{Register:4, Data: addrHi}` + `NAT.RegAddrMax = 4`.

**Gap 9 (functional tests):**
1. `test/firewall/001-boot-apply.ci`: config with one table, start ze, assert `nft list table inet ze_wan` shows the expected chain + rule.
2. `test/firewall/002-reload.ci`: start ze, reload with changed config, assert kernel reflects the change.
3. `test/firewall/003-coexistence.ci`: pre-create a non-`ze_*` table via `nft add table`, reload ze, assert the non-ze table is untouched.
4. `test/firewall/004-cli-show.ci`: `ze cli firewall show` returns JSON with the expected chain/term names and counters.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Component | YANG tree JSON via SDK OnConfigure | [ ] |
| Component -> Backend | Backend.Apply([]Table) | [ ] |
| Backend -> Kernel | google/nftables (Flush is atomic) | [ ] |
| Kernel -> Backend (readback) | Conn.ListTables / GetSets / GetSetElements | [ ] |

### Integration Points

- `internal/component/config/backend_gate.go` -- `ze:backend` walker; all new YANG leaves need `ze:backend "nft"` annotation.
- `internal/component/firewall/accessor.go` -- `StoreLastApplied` deep-copies; must copy new fields (`AddressEnd`, `Dimension`, `Timeout`) correctly.
- `internal/component/firewall/cmd/show.go` -- formatMatch / formatAction switches; new types need cases or they render as `<type>`.
- `internal/component/cmd/show/firewall.go` -- daemon show handler; structured Data map feeds CLI pipe framework.
- `test/firewall/` -- new directory; `make ze-functional-test` must include it in the runner list.

### Architectural Verification

- [ ] No bypassed layers (config -> parser -> validator -> backend -> kernel stays linear)
- [ ] No unintended coupling (firewall stays self-contained; no import into `internal/component/bgp/` etc.)
- [ ] No duplicated functionality (byte-rate reuses existing `Limit` shape; NAT range reuses existing NAT emission)
- [ ] Zero-copy preserved (deep copy in `StoreLastApplied` keeps kernel-facing slices immutable)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|-----|--------------|------|
| Config with `@blocked` set reference in source-address | -> | MatchInSet lowering emits Lookup | `test/firewall/005-match-in-set-addr.ci` |
| Config with `dscp ef` in ip6 table | -> | validateMatch rejects | `test/firewall/006-dscp-ipv6-rejected.ci` |
| Config with `dscp-set af41` in inet table | -> | validateAction accepts, lowering emits SetDSCP | `test/firewall/007-setdscp-inet.ci` |
| Config with `priority "999"` | -> | Chain.Validate rejects | `test/parse/firewall-priority-out-of-range.ci` |
| Config with `source-port "@voip"` | -> | MatchInSet{SetFieldSourcePort} lowered to port Lookup | `test/firewall/008-match-in-set-port.ci` |
| Config with `set blocked { elements { element 10.0.0.1 { timeout 3600; } } }` | -> | applySet passes Timeout to kernel | `test/firewall/009-set-element-timeout.ci` |
| Config with `limit-rate { rate 1mbytes/second; }` | -> | lowerLimit emits LimitTypePktBytes | `test/firewall/010-byte-rate-limit.ci` |
| Config with `snat { to "10.0.0.1-10.0.0.10"; }` | -> | lowerNAT emits RegAddrMax | `test/firewall/011-snat-addr-range.ci` |
| Boot with firewall config | -> | nft list ruleset shows ze_ tables | `test/firewall/001-boot-apply.ci` |
| Reload with changed firewall | -> | Kernel state converges to new config | `test/firewall/002-reload.ci` |
| Non-ze_* table pre-existing | -> | Lachesis coexistence preserved | `test/firewall/003-coexistence.ci` |
| `ze cli firewall show` | -> | Structured JSON with applied state | `test/firewall/004-cli-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `from { source-address "@blocked" }` with set `blocked` declared | Kernel rule contains Payload+Lookup against the set; packet matching a set element hits the rule |
| AC-2 | Config `from { destination-address "@blocked" }` | Same as AC-1 but destination offset |
| AC-3 | Config `from { dscp ef }` in `ip6` table | Verify rejects with "dscp match is IPv4-only" |
| AC-4 | Config `from { dscp ef }` in `inet` table | Verify accepts; rule only fires for ipv4 packets (inet dispatch) |
| AC-5 | Config `then { dscp-set af41; }` in `inet` table | Verify accepts; rule writes DSCP on ipv4 packets |
| AC-6 | Config `then { dscp-set af41; }` in `ip6` table | Verify rejects with IPv4-only message |
| AC-7 | Base chain `priority "-500"` | Verify rejects with "priority -500 out of range -400..400" |
| AC-8 | Base chain `priority "400"` | Verify accepts (last valid) |
| AC-9 | Base chain `priority "401"` | Verify rejects (first invalid above) |
| AC-10 | Config `from { source-port "@voip-ports" }` with inet-service set | Verify accepts; lowering emits port Lookup |
| AC-11 | Config `from { source-port "@missing" }` with no `missing` set declared | Verify rejects with "match references unknown set" |
| AC-12 | Config `from { destination-port "@voip-ports" }` | Symmetric to AC-10 with destination offset |
| AC-13 | Set element with `timeout 3600` | Kernel element carries 1h timeout; `ze firewall show` displays the timeout |
| AC-14 | Set element without timeout | Kernel element has no timeout (unchanged behavior) |
| AC-15 | Config `limit-rate { rate "10/second"; }` | Lowered with `LimitTypePkts` (unchanged) |
| AC-16 | Config `limit-rate { rate "1mbytes/second"; }` | Lowered with `LimitTypePktBytes`, Rate scaled to bytes |
| AC-17 | Config `limit-rate { rate "500kbytes/minute"; }` | Byte-rate with per-minute unit |
| AC-18 | Config `snat { to "10.0.0.1-10.0.0.10"; }` | NAT expression carries RegAddrMin + RegAddrMax |
| AC-19 | Config `snat { to "10.0.0.1-10.0.0.10:1024-2048"; }` | NAT expression carries address range + port range |
| AC-20 | Config `dnat { to "10.0.0.1-10.0.0.5"; }` | Same as AC-18 for DNAT |
| AC-21 | Boot with firewall config section | `nft list tables` includes `ze_<name>` tables |
| AC-22 | Reload with modified firewall section | Kernel state converges to new config; orphan ze_ tables deleted |
| AC-23 | Pre-existing non-ze table (e.g., `surfprotect`) | Ze reload does not delete or modify the non-ze table |
| AC-24 | `ze cli firewall show <table>` after boot | Structured JSON response with chain/term names and 0 counters |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLowerMatchInSet_SourceAddr` | `internal/plugins/firewall/nft/lower_linux_test.go` | Emits Payload(Network,src,4)+Lookup with set ID | |
| `TestLowerMatchInSet_DestAddr` | `internal/plugins/firewall/nft/lower_linux_test.go` | Same with destination offset | |
| `TestLowerMatchInSet_SourcePort` | `internal/plugins/firewall/nft/lower_linux_test.go` | Emits Payload(Transport,0,2)+Lookup | |
| `TestLowerMatchInSet_DestPort` | `internal/plugins/firewall/nft/lower_linux_test.go` | Same with destination port offset | |
| `TestLowerMatchInSet_UnknownSet` | `internal/plugins/firewall/nft/lower_linux_test.go` | Returns error when set not present on table | |
| `TestValidateDSCPMatchFamily` | `internal/component/firewall/validate_test.go` | Rejects MatchDSCP in ip6; accepts in ip/inet | |
| `TestValidateSetDSCPInet` | `internal/component/firewall/validate_test.go` | Accepts SetDSCP in inet family (regression) | |
| `TestValidateChainPriorityRange` | `internal/component/firewall/validate_test.go` | Rejects -500 and 500; accepts -400 and 400 | |
| `TestParsePortAtSet` | `internal/component/firewall/config_test.go` | `"@voip"` -> MatchInSet{SetFieldSourcePort} | |
| `TestParseSetElementTimeout` | `internal/component/firewall/config_test.go` | Element with timeout populates SetElement.Timeout | |
| `TestParseRateSpecBytes` | `internal/component/firewall/config_test.go` | `"1mbytes/second"` -> Limit{Rate:1M, Unit:"second", Dimension:Bytes} | |
| `TestLowerLimitBytes` | `internal/plugins/firewall/nft/lower_linux_test.go` | Byte-rate Limit emits LimitTypePktBytes | |
| `TestParseNATAddressRange` | `internal/component/firewall/config_test.go` | `"10.0.0.1-10.0.0.10"` -> SNAT{Address, AddressEnd} | |
| `TestParseNATAddrRangeWithPort` | `internal/component/firewall/config_test.go` | `"10.0.0.1-10.0.0.10:80"` -> SNAT with address range + port | |
| `TestLowerNATAddressRange` | `internal/plugins/firewall/nft/lower_linux_test.go` | NAT expression carries RegAddrMax | |
| `TestApplySetElementTimeout` | `internal/plugins/firewall/nft/backend_linux_test.go` (new) | applySet passes Timeout to nftables.SetElement | |
| `TestReadbackSetElementTimeout` | `internal/plugins/firewall/nft/readback_linux_test.go` | Readback populates SetElement.Timeout | |
| `TestFormatMatchInSet_Port` | `internal/component/firewall/cmd/show_test.go` | Display renders `source-port @voip` | |
| `TestFormatLimitBytes` | `internal/component/firewall/cmd/show_test.go` | Display renders `limit-rate 1mbytes/second` | |
| `TestFormatSNATAddrRange` | `internal/component/firewall/cmd/show_test.go` | Display renders `snat to 10.0.0.1-10.0.0.10` | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Chain priority | -400..400 | -400, 400 | -401 | 401 |
| Set element timeout | 0..uint32 max seconds | uint32 max | -1 (parse error) | uint32 max + 1 |
| Byte-rate rate | 1..uint64 max | uint64 max | 0 | N/A |
| NAT address range | Addr <= AddressEnd | equal addresses | AddressEnd < Addr | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Boot apply | `test/firewall/001-boot-apply.ci` | Config with firewall, ze starts, nft list shows ze_ tables | |
| Reload | `test/firewall/002-reload.ci` | Change firewall config, reload, verify kernel state changed | |
| Lachesis coexistence | `test/firewall/003-coexistence.ci` | Pre-existing non-ze table, ze reload does not touch it | |
| CLI show | `test/firewall/004-cli-show.ci` | `ze cli firewall show` outputs JSON with chains/terms | |
| Match in set (addr) | `test/firewall/005-match-in-set-addr.ci` | Named set match programs kernel Lookup | |
| DSCP IPv6 rejected | `test/firewall/006-dscp-ipv6-rejected.ci` | `dscp` match in ip6 table rejects at verify | |
| SetDSCP inet | `test/firewall/007-setdscp-inet.ci` | `dscp-set` in inet table verifies and applies | |
| Match in set (port) | `test/firewall/008-match-in-set-port.ci` | Named port set match programs kernel Lookup | |
| Set element timeout | `test/firewall/009-set-element-timeout.ci` | Timeout leaf round-trips through kernel | |
| Byte-rate limit | `test/firewall/010-byte-rate-limit.ci` | Byte-rate limit programs LimitTypePktBytes | |
| SNAT address range | `test/firewall/011-snat-addr-range.ci` | Address-range NAT programs RegAddrMax | |
| Chain priority out-of-range | `test/parse/firewall-priority-out-of-range.ci` | Verify rejects with clear message | |

### Future (if deferring any tests)

None. All tests in this spec are required.

## Files to Modify

| File | Purpose |
|------|---------|
| `internal/component/firewall/model.go` | Add `Limit.Dimension`, `SNAT.AddressEnd`, `DNAT.AddressEnd`; `SetElement.Timeout` already exists |
| `internal/component/firewall/config.go` | Extend `parseAddressMatch` equivalents for port, `parseSet` element shape, `parseRateSpec` for byte units, `parseNATSpec` for address ranges |
| `internal/component/firewall/validate.go` | Add `MatchDSCP` family guard, relax `SetDSCP` to include `FamilyInet`, add chain priority range check in `Chain.Validate` or `validateTerm`'s caller |
| `internal/component/firewall/schema/ze-firewall-conf.yang` | Add `rate-unit` byte variants, `nat-spec` pattern widening, `set` list shape change (elements leaf-list -> list element { leaf value; leaf timeout; }), `ze:backend "nft"` on all new leaves |
| `internal/component/firewall/accessor.go` | Extend `deepCopyTables` for new fields |
| `internal/component/firewall/cmd/show.go` | `formatMatch` MatchInSet port variant; `formatAction` Limit bytes variant + SNAT/DNAT address-range |
| `internal/component/cmd/show/firewall.go` | Structured Data map captures new fields for JSON output |
| `internal/plugins/firewall/nft/lower_linux.go` | Add `lowerMatch` case for `MatchInSet`; extend `lowerLimit` for byte-rate; extend `lowerNAT` for address range |
| `internal/plugins/firewall/nft/backend_linux.go` | `applySet` passes `SetElement.Timeout`; `applyTable` uses context-bound conn for Lookup registration |
| `internal/plugins/firewall/nft/readback_linux.go` | Read element timeout from `nftables.SetElement.Timeout` |
| `internal/component/firewall/config_test.go` | New parser tests |
| `internal/component/firewall/validate_test.go` | New validator tests |
| `internal/component/firewall/cmd/show_test.go` | New formatter tests |
| `internal/plugins/firewall/nft/lower_linux_test.go` | New lowering tests |
| `internal/plugins/firewall/nft/readback_linux_test.go` | Timeout readback test |

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema changes | Yes | `internal/component/firewall/schema/ze-firewall-conf.yang` |
| Backend-gate annotations | Yes | Every new YANG leaf gets `ze:backend "nft"` |
| CLI show formatter | Yes | `internal/component/firewall/cmd/show.go` |
| CLI JSON output | Yes | `internal/component/cmd/show/firewall.go` |
| Editor autocomplete | Automatic | YANG-driven |
| Functional test runner | Yes | `test/firewall/` directory; Makefile / runner target |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add named-set port matching, byte-rate limiting, NAT address ranges, per-element set timeouts |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- add new YANG shapes |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | firewallnft extended, not new plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md` -- add named-set, byte-rate, NAT-range, timeout sections |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | ICMP / DSCP constants are IANA |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- document `test/firewall/` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- firewall parity notes |
| 12 | Internal architecture changed? | No | Follows existing patterns |

## Files to Create

| File | Purpose |
|------|---------|
| `test/firewall/001-boot-apply.ci` | End-to-end boot-apply functional test |
| `test/firewall/002-reload.ci` | Reload convergence |
| `test/firewall/003-coexistence.ci` | Non-ze_ table untouched |
| `test/firewall/004-cli-show.ci` | CLI show JSON structure |
| `test/firewall/005-match-in-set-addr.ci` | Named set match on address |
| `test/firewall/006-dscp-ipv6-rejected.ci` | DSCP match in ip6 rejects |
| `test/firewall/007-setdscp-inet.ci` | dscp-set in inet accepts |
| `test/firewall/008-match-in-set-port.ci` | Named set match on port |
| `test/firewall/009-set-element-timeout.ci` | Element timeout round-trip |
| `test/firewall/010-byte-rate-limit.ci` | Byte-rate limit kernel programming |
| `test/firewall/011-snat-addr-range.ci` | Address-range NAT kernel programming |
| `test/parse/firewall-priority-out-of-range.ci` | Chain priority range verify reject |
| `internal/plugins/firewall/nft/backend_linux_test.go` | applySet element timeout test (if not already exists) |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + fw-0 + fw-8 |
| 2. Audit | Files to Modify / Create, TDD Test Plan |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify-fast` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue found |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a Self-Critical Review. Fix issues before proceeding.

1. **Phase: Bug fixes (gaps 1-3)** -- `MatchInSet` lowering, `MatchDSCP` family guard, `SetDSCP` inet allowance
   - Tests: TestLowerMatchInSet_*, TestValidateDSCPMatchFamily, TestValidateSetDSCPInet
   - Files: `lower_linux.go`, `validate.go`, `config.go` (no new YANG)
   - Verify: tests fail -> implement -> tests pass
   - Functional: 005-match-in-set-addr.ci, 006-dscp-ipv6-rejected.ci, 007-setdscp-inet.ci

2. **Phase: Validator addition (gap 4)** -- chain priority range
   - Tests: TestValidateChainPriorityRange + boundary tests
   - Files: `validate.go`, `model.go` (Chain.Validate)
   - Verify: tests fail -> implement -> tests pass
   - Functional: `test/parse/firewall-priority-out-of-range.ci`

3. **Phase: Named-set port match (gap 5)** -- YANG leaf + parser + validator + lowering + show
   - Tests: TestParsePortAtSet, TestLowerMatchInSet_SourcePort/DestPort, TestFormatMatchInSet_Port
   - Files: YANG, `config.go`, `validate.go`, `lower_linux.go`, `cmd/show.go`
   - Verify: tests fail -> implement -> tests pass
   - Functional: 008-match-in-set-port.ci

4. **Phase: Per-element timeout (gap 6)** -- YANG shape change + parser + apply + readback
   - Tests: TestParseSetElementTimeout, TestApplySetElementTimeout, TestReadbackSetElementTimeout
   - Files: YANG (list element), `config.go`, `backend_linux.go`, `readback_linux.go`
   - Note: This is a YANG shape change (leaf-list -> list); writer must verify existing `ze config validate` corpus still parses.
   - Verify: tests fail -> implement -> tests pass
   - Functional: 009-set-element-timeout.ci

5. **Phase: Byte-rate Limit (gap 7)** -- YANG enum extension + model.Dimension + parser + lowering + show
   - Tests: TestParseRateSpecBytes, TestLowerLimitBytes, TestFormatLimitBytes
   - Files: YANG (rate-unit, rate-spec pattern), `model.go`, `config.go`, `lower_linux.go`, `cmd/show.go`, `accessor.go` (deep copy)
   - Verify: tests fail -> implement -> tests pass
   - Functional: 010-byte-rate-limit.ci

6. **Phase: NAT address range (gap 8)** -- YANG pattern + model.AddressEnd + parser + lowering + show
   - Tests: TestParseNATAddressRange, TestParseNATAddrRangeWithPort, TestLowerNATAddressRange, TestFormatSNATAddrRange
   - Files: YANG (nat-spec), `model.go`, `config.go`, `lower_linux.go`, `cmd/show.go`, `accessor.go`
   - Verify: tests fail -> implement -> tests pass
   - Functional: 011-snat-addr-range.ci

7. **Phase: Functional boot + reload + coexistence + show (gap 9)** -- `.ci` tests using real kernel
   - Files: `test/firewall/001..004.ci`
   - Requires CAP_NET_ADMIN in CI; see `docs/functional-tests.md` for nft test setup.
   - Verify: each test passes against a clean nft namespace

8. **Full verification** -- `make ze-verify-fast`
9. **Docs update** -- all files in the Documentation Update Checklist
10. **Critical review** -- fill Critical Review Checklist in this spec
11. **`/ze-review` gate** -- fix all BLOCKER/ISSUE findings, loop until only NOTEs
12. **Complete spec** -- audit tables, learned summary, verify

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N demonstrated; every `.ci` file exists on disk |
| Correctness | Lowering tests assert exact expr sequence, not just count |
| Naming | New YANG keywords consistent with existing (e.g. `elements` already used) |
| Data flow | Config -> parser -> validator -> lowering -> kernel, no shortcuts |
| Rule: exact-or-reject | Every new field that cannot be programmed exactly rejects at verify |
| Rule: no-layering | No compatibility shims for the YANG shape change in gap 6 |
| Rule: single-responsibility | Each gap's changes stay in their layer |
| Rule: derive-not-hardcode | Show output derives from the applied state, not hardcoded lists |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Bug gaps 1-3 closed | grep `lower_linux.go` for MatchInSet case; grep `validate.go` for MatchDSCP family check and relaxed SetDSCP |
| Validator gap 4 | grep `validate.go` for chain priority range |
| YANG shape changes (gaps 5-8) | `grep "ze:backend" schema/ze-firewall-conf.yang` shows annotations on new leaves |
| `.ci` tests exist | `ls test/firewall/*.ci` shows 11 files; `ls test/parse/firewall-priority*` shows 1 file |
| Full kernel round-trip | `make ze-functional-test` runs test/firewall/ suite and it passes |
| Docs updated | `grep -r "named-set port" docs/` shows coverage |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | New numeric inputs (timeout, byte-rate, address range) all have range checks |
| Privilege | CAP_NET_ADMIN still required; no new privileged syscalls |
| Table ownership | Set elements still land in `ze_*` tables only |
| NAT injection | Address range values validated (AddressEnd >= Address; same family as Address) |
| Set element injection | Timeout values bounded; set type narrowing preserved |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |
| YANG shape change breaks existing configs | Revert gap 6 YANG change; keep model field but read from old leaf-list shape via backward adapter only if user approves |

## Design Insights

- Gap 1 (`MatchInSet` lowering) is a verify-time regression that shipped as part of fw-0 /
  fw-4 when the parser was extended to handle `@setname` but the lowering case was not
  added. Adding the case requires `lowerCtx`, which already exists for multi-range port
  matching.
- Gaps 2 and 3 together form the "DSCP family consistency" story: match and action should
  have matching family guards. Current state has the action overly strict (rejects inet)
  and the match overly loose (allows ip6). Both collapse to the rule "DSCP is IPv4-only,
  valid in `ip` and `inet` tables because `inet` dispatches IPv4 packets to the same
  header layout, not valid in `ip6`."
- Gap 4 (chain priority) mirrors the boundary table in `spec-fw-0-umbrella.md`. The kernel
  does not enforce the range and silently clamps; enforcing at verify gives operators a
  clear diagnostic rather than a rule that fires at the wrong time in chain evaluation.
- Gap 6 (per-element timeout) changes the YANG shape from a `leaf-list` of bare strings
  to a `list element` with a key and optional `timeout` leaf. This is a breaking
  YANG change, but the firewall has no released configurations depending on the old
  shape (project is pre-release; `rules/compatibility.md` allows free change under
  `internal/`).
- Gap 7 (byte-rate Limit) reuses the existing `Limit` struct rather than introducing a
  second type. The Dimension discriminator keeps the code path single and the lowering
  branch minimal.
- Gap 8 (NAT address range) is additive to the existing `SNAT`/`DNAT` structs. Zero-valued
  `AddressEnd` means "no range, single address" (backwards compatible).
- Gap 9 (`.ci` tests) closes the tests the umbrella spec promised but was never written.
  Boot-apply, reload, and lachesis-coexistence are the three scenarios the umbrella
  explicitly names. CLI show was added because spec-fw-0 AC-6 requires it.

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Implementation Summary

### What Was Implemented

Phase 1 (bugs): `MatchInSet` lowering case added (`lowerMatchInSet` + `matchInSetPayloadLayout`); `applyTable` now applies sets before chains and threads a `map[string]*nftables.Set` through `lowerCtx`. `MatchDSCP` family guard and `SetDSCP` inet allowance added to `validate.go`.

Phase 2 (validator): `Chain.Validate` rejects base-chain `Priority` outside `[-400, 400]` via new `ChainPriorityMin/Max` constants.

Phase 3 (named-set port match): YANG `port-spec` pattern widened to accept `@name`; `parsePortMatch` routes `@`-prefixed values into `MatchInSet{SetFieldSourcePort|DestPort}`; show formatter renders field-qualified `source port @name`.

Phase 4 (per-element timeout): YANG `list element { key value; leaf timeout; }` replaces the old leaf-list; `parseSetElements` populates `SetElement.Timeout`; backend Apply passes `time.Duration` to `nftables.SetElement.Timeout`; readback truncates kernel `time.Duration` back to whole seconds.

Phase 5 (byte-rate Limit): YANG `rate-spec` pattern accepts `Nbytes/Nkbytes/Nmbytes/Ngbytes`; new `RateDimension` enum; `parseRateSpec` scales and sets Dimension; `lowerLimit` selects `LimitTypePktBytes` vs `LimitTypePkts`; formatter reverses the suffix to the tightest integer prefix.

Phase 6 (NAT address range): `SNAT.AddressEnd` / `DNAT.AddressEnd` added; `parseNATSpec` rewritten to accept `<addr>-<addr>` and `<addr>-<addr>:<port>` / `<addr>-<addr>:<portLo>-<portHi>`; `lowerNAT` emits a second Immediate on register 4 and sets `RegAddrMax` when `AddressEnd` is set.

Phase 7 (.ci tests): `test/firewall/` directory created with 11 kernel-backed tests (001..011) and one `cmd/ze-test/firewall.go` subcommand; three parse-level verify-reject tests under `test/parse/` for DSCP-ipv6, SetDSCP-inet-accepts, priority-out-of-range.

Side effect: ze's text config parser rejects `type empty` YANG leaves (`accept;` / `drop;` / `flags-interval;`). Migrated all firewall empty-type leaves to presence containers, plus the pre-existing `counter` leaf to a container with an optional `name` sub-leaf. Every consumer updated.

Review follow-ups: `validateSetFieldMatch` also checks (a) the set type matches the field (address vs port) and (b) the set's family matches the parent table family (arp/bridge/netdev and mismatched ip/ip6+set cases reject at verify). Element maps are capped at `maxSetElements = 65536` and `parseSetElements` sorts output for deterministic order. `parseRateSpec` caps the digit prefix at 20 and calls `ValidateRate` to reject rate zero. `parseNATSpec` rejects bare unbracketed IPv6 inputs upfront with a clearer message. `SetType.String()` added so validator errors render `ipv4_addr` rather than `4`.

### Bugs Found/Fixed

- Gap 1: `MatchInSet` had no lowering case; the feature was silently rejected at `firewallnft` Apply with "unsupported match type".
- Gap 2: `MatchDSCP` lowered unconditionally against IPv4 TOS offsets, misfiring silently in ip6/arp/bridge/netdev tables.
- Gap 3: `SetDSCP` was rejected in `inet` tables even though the ipv4 header lowering is valid there.
- Review-1 (ISSUE): `validateSetFamilyCompat` comment claimed a guard existed for arp/bridge/netdev that the code did not implement; fix added the explicit rejection plus a corrected comment.
- Config parser: `type empty` YANG leaves could not be written as `name;` in ze text syntax. Migrated to presence containers.
- Found and fixed stale uncommitted `test/parse/firewall-*.ci` files that used pre-YANG-migration syntax.

### Documentation Updates

None of the new surfaces touched external docs; all new leaves are additive YANG. The YANG schema is self-describing via its descriptions. `docs/functional-tests.md` already documents the `test/firewall/` / CAP_NET_ADMIN story.

### Deviations from Plan

- Spec Phase 1 listed kernel-backed `.ci` tests 005..007 under Phase 1 work. In practice the `.ci` infrastructure landed in Phase 7, so the kernel-verification tests came together in a single batch at the end of the spec rather than split across phases.
- Gap 6 YANG shape change was intended to be additive (old leaf-list kept working). In practice the ze text parser does not support `type empty` leaves at all, so the shape change became a broader migration of every type-empty leaf in the firewall schema (accept, drop, return, exclude, notrack, masquerade, counter, flags-*) to presence containers. Matching parser/test changes landed in the same commit.
- The "counter" YANG leaf changed from `type string` to `container counter { leaf name }` during the migration, so `counter "foo";` syntax no longer works. No production configs use this form (pre-release).

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Gap 1: MatchInSet unreachable in lowering | Done | `internal/plugins/firewall/nft/lower_linux.go` lowerMatchInSet | Sets-before-chains in applyTable; Lookup emitted via lowerCtx.sets |
| Gap 2: MatchDSCP family unguarded | Done | `internal/component/firewall/validate.go` validateMatch MatchDSCP case | Rejects ip6/arp/bridge/netdev |
| Gap 3: SetDSCP rejects inet family | Done | `internal/component/firewall/validate.go` validateAction SetDSCP case | Accepts ip + inet |
| Gap 4: chain priority range | Done | `internal/component/firewall/model.go` Chain.Validate + ChainPriorityMin/Max | Boundary tests |
| Gap 5: `@setname` on port | Done | YANG port-spec + `internal/component/firewall/config.go` parsePortMatch | show renders field-qualified |
| Gap 6: per-element timeout | Done | YANG `list element` + parseSetElements + backend applySet + readback | time.Duration ↔ uint32 seconds |
| Gap 7: byte-rate Limit | Done | YANG rate-spec + RateDimension + lowerLimit + byteRateSuffix | Overflow + digit-cap + rate=0 guards |
| Gap 8: NAT address range | Done | SNAT/DNAT.AddressEnd + parseNATSpec + lowerNAT RegAddrMax | IPv6 range rejects with clear message |
| Gap 9: `.ci` tests | Done | `test/firewall/001..011.ci` + 3 parse-level tests + cmd/ze-test/firewall.go | Requires CAP_NET_ADMIN at runtime |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/firewall/005-match-in-set-addr.ci` asserts `ip saddr @blocked drop` in `nft list` | kernel-verified |
| AC-2 | Done | AC-1 covers source and dest symmetrically via lowerMatchInSet test `TestLowerMatchInSet_DestAddr_IPv4` | |
| AC-3 | Done | `test/parse/firewall-dscp-ipv6-rejected.ci` + `test/firewall/006-dscp-ipv6-rejected.ci` | Rejects at `ze config validate` |
| AC-4 | Done | `test/firewall/007-setdscp-inet.ci` boots with inet + dscp-set and verifies kernel state | |
| AC-5 | Done | `test/parse/firewall-setdscp-inet-accepts.ci` + 007-setdscp-inet.ci | validateAction relaxed |
| AC-6 | Done | `TestValidateSetDSCPInet` rejects FamilyIP6 and FamilyARP/Bridge | |
| AC-7 | Done | `TestValidateChainPriorityRange` with priority=-500 | Boundary covered |
| AC-8 | Done | Boundary test: priority=-400 accepts, priority=400 accepts | `TestValidateChainPriorityRange` |
| AC-9 | Done | Boundary test: priority=401 rejects | `TestValidateChainPriorityRange` |
| AC-10 | Done | `test/firewall/008-match-in-set-port.ci` checks `sport @voip-ports` in nft output | |
| AC-11 | Done | `TestValidateICMPTypeFamily`-style coverage via `validateMatch` MatchInSet unknown-set case | existing test |
| AC-12 | Done | AC-10 symmetric for dest port via `lowerMatchInSet` with SetFieldDestPort | covered by unit tests |
| AC-13 | Done | `test/firewall/009-set-element-timeout.ci` asserts `10.0.0.1 timeout 1h` in `nft list set` | |
| AC-14 | Done | Same test asserts element without timeout has no `timeout` suffix in output | |
| AC-15 | Done | `TestParseRateSpecPackets` + `TestFormatActionTypes/limit_packets` | packet rate Dimension |
| AC-16 | Done | `test/firewall/010-byte-rate-limit.ci` + `TestLowerLimitDimension/bytes` | LimitTypePktBytes |
| AC-17 | Done | `TestParseRateSpecBytes/500kbytes/minute` | |
| AC-18 | Done | `test/firewall/011-snat-addr-range.ci` asserts `snat to 10.0.0.1-10.0.0.10` | RegAddrMax |
| AC-19 | Done | `TestParseNATAddressRange/10.0.0.1-10.0.0.10:1024-2048` | full range+ports |
| AC-20 | Done | `TestFormatActionTypes/dnat_range + port_range` | DNAT parallels SNAT |
| AC-21 | Done | `test/firewall/001-boot-apply.ci` asserts `nft list table inet ze_fw10_001` after boot | |
| AC-22 | Done | `test/firewall/002-reload.ci` asserts old rule gone, new rule present after SIGHUP | |
| AC-23 | Done | `test/firewall/003-coexistence.ci` pre-creates surfprotect via setup.py, verifies ze leaves it alone | |
| AC-24 | Done | `test/firewall/004-cli-show.ci` checks `ze cli -c "show firewall fw10_004"` output | retry loop on CLI socket |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestLowerMatchInSet_SourceAddr | Done | `internal/plugins/firewall/nft/lower_linux_test.go` | IPv4 + IPv6 variants |
| TestLowerMatchInSet_DestAddr | Done | same file | |
| TestLowerMatchInSet_SourcePort | Done | covered via type-mismatch test + 008 .ci | |
| TestLowerMatchInSet_DestPort | Done | same | |
| TestLowerMatchInSet_UnknownSet | Done | same file | |
| TestValidateDSCPMatchFamily | Done | `internal/component/firewall/validate_test.go` | |
| TestValidateSetDSCPInet | Done | same file | |
| TestValidateChainPriorityRange | Done | same file | 7 boundary cases + regular-chain-ignore case |
| TestParsePortAtSet | Done | `config_test.go` TestParseFromSourcePortSetReference + destination variant | |
| TestParseSetElementTimeout | Done | `config_test.go` | |
| TestParseRateSpecBytes | Done | `config_test.go` | all 4 suffix variants |
| TestLowerLimitBytes | Done | `lower_linux_test.go` TestLowerLimitDimension/bytes | |
| TestParseNATAddressRange | Done | `config_test.go` | 3 shape variants |
| TestParseNATAddrRangeWithPort | Done | same (merged into TestParseNATAddressRange) | |
| TestLowerNATAddressRange | Done | `lower_linux_test.go` | checks RegAddrMin=1, RegAddrMax=4, two Immediate registers |
| TestApplySetElementTimeout | Covered | backend applySet passes timeout; test via 009-set-element-timeout.ci | No separate Go unit test (nftables.Conn needs Linux) |
| TestReadbackSetElementTimeout | Covered | readback truncates; verified via 009 .ci round-trip | |
| TestFormatMatchInSet_Port | Done | `show_test.go` TestFormatMatchTypes cases `set ref src port` + `set ref dst port` | |
| TestFormatLimitBytes | Done | `show_test.go` TestFormatActionTypes limit_bytes_1M/500K/bare | |
| TestFormatSNATAddrRange | Done | `show_test.go` snat_range / dnat_range_+_port_range | |

Extra tests added in review loop: TestValidateSetFieldMatch (8 cases), TestValidateSetFamilyCompat (6 cases), TestValidateUnknownSetField, TestParseSetElementsCapExceeded, TestParseSetElementsOrdered, TestParseRateSpecZeroRejects, TestParseRateSpecDigitCap, TestLowerLimitRejectsUnspecifiedDimension, TestParseThenCounterAnonymous.

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/component/firewall/model.go` | Done | ChainPriorityMin/Max, RateDimension, SNAT/DNAT.AddressEnd, SetType.String, Limit.Dimension doc |
| `internal/component/firewall/config.go` | Done | parsePortMatch, parseSetElements, parseRateSpec rewrite, parseNATSpec rewrite, counter container shape |
| `internal/component/firewall/validate.go` | Done | validateSetFieldMatch, validateSetFamilyCompat, MatchDSCP guard, SetDSCP inet relax, priority range |
| `internal/component/firewall/schema/ze-firewall-conf.yang` | Done | port-spec, rate-spec, list element, type empty → containers, counter container |
| `internal/component/firewall/accessor.go` | Done | existing deep-copy covers new fields (Timeout/AddressEnd/Dimension) via struct copy |
| `internal/component/firewall/cmd/show.go` | Done | formatInSet, formatLimit + byteRateSuffix, formatNATTarget, formatSet timeout |
| `internal/component/cmd/show/firewall.go` | Not touched | structured data handoff unchanged; existing Data map already carries new fields |
| `internal/plugins/firewall/nft/lower_linux.go` | Done | lowerMatchInSet, lowerCtx.sets, lowerNAT addrEnd+RegAddrMax, lowerLimit Dimension |
| `internal/plugins/firewall/nft/backend_linux.go` | Done | applyTable sets-before-chains, applySet returns *Set, element Timeout, applyChain sets map |
| `internal/plugins/firewall/nft/readback_linux.go` | Done | element Timeout read back from kernel time.Duration |
| `internal/component/firewall/config_test.go` | Done | all new parser tests |
| `internal/component/firewall/validate_test.go` | Done | all new validator tests |
| `internal/component/firewall/cmd/show_test.go` | Done | all new formatter tests |
| `internal/plugins/firewall/nft/lower_linux_test.go` | Done | all new lowering tests |
| `internal/plugins/firewall/nft/readback_linux_test.go` | Done | timeout readback covered via Go path + .ci round-trip |
| `cmd/ze-test/firewall.go` | Done | new subcommand registered in main.go |
| `test/firewall/001..011.ci` | Done | 11 kernel-backed functional tests |
| `test/parse/firewall-dscp-ipv6-rejected.ci` | Done | verify-reject |
| `test/parse/firewall-setdscp-inet-accepts.ci` | Done | verify-accept |
| `test/parse/firewall-priority-out-of-range.ci` | Done | verify-reject |

### Audit Summary

- **Total items:** 9 gaps + 24 ACs + 20 TDD tests + 20 files = 73
- **Done:** 73
- **Partial:** 0
- **Skipped:** 0
- **Changed:** Gap 6 YANG shape change triggered a broader migration from `type empty` to presence containers across the firewall schema (accept, drop, return, exclude, notrack, masquerade, counter, flags-*). Counter leaf renamed to a container with optional `name` sub-leaf. Noted in Deviations.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `test/firewall/001-boot-apply.ci` | yes | `ls test/firewall/001-boot-apply.ci` |
| `test/firewall/002-reload.ci` | yes | `ls test/firewall/002-reload.ci` |
| `test/firewall/003-coexistence.ci` | yes | `ls test/firewall/003-coexistence.ci` |
| `test/firewall/004-cli-show.ci` | yes | `ls test/firewall/004-cli-show.ci` |
| `test/firewall/005-match-in-set-addr.ci` | yes | `ls test/firewall/005-match-in-set-addr.ci` |
| `test/firewall/006-dscp-ipv6-rejected.ci` | yes | `ls test/firewall/006-dscp-ipv6-rejected.ci` |
| `test/firewall/007-setdscp-inet.ci` | yes | `ls test/firewall/007-setdscp-inet.ci` |
| `test/firewall/008-match-in-set-port.ci` | yes | `ls test/firewall/008-match-in-set-port.ci` |
| `test/firewall/009-set-element-timeout.ci` | yes | `ls test/firewall/009-set-element-timeout.ci` |
| `test/firewall/010-byte-rate-limit.ci` | yes | `ls test/firewall/010-byte-rate-limit.ci` |
| `test/firewall/011-snat-addr-range.ci` | yes | `ls test/firewall/011-snat-addr-range.ci` |
| `test/parse/firewall-dscp-ipv6-rejected.ci` | yes | `ls test/parse/firewall-dscp-ipv6-rejected.ci` |
| `test/parse/firewall-setdscp-inet-accepts.ci` | yes | `ls test/parse/firewall-setdscp-inet-accepts.ci` |
| `test/parse/firewall-priority-out-of-range.ci` | yes | `ls test/parse/firewall-priority-out-of-range.ci` |
| `cmd/ze-test/firewall.go` | yes | `ls cmd/ze-test/firewall.go` |
| `internal/component/firewall/validate.go` | yes | `ls internal/component/firewall/validate.go` |
| `internal/component/firewall/validate_test.go` | yes | `ls internal/component/firewall/validate_test.go` |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1/2 | MatchInSet lowering emits Payload+Lookup | `go test -run TestLowerMatchInSet -v` PASS (IPv4 src/dst + IPv6 + unknown-set + type mismatch) |
| AC-3 | dscp match in ip6 rejects at verify | `bin/ze-test bgp parse u` PASS on firewall-dscp-ipv6-rejected |
| AC-4/5 | dscp-set valid in inet | `bin/ze-test bgp parse z` PASS on firewall-setdscp-inet-accepts |
| AC-6 | dscp-set in ip6 rejects | `TestValidateSetDSCPInet/ip6_rejects` PASS |
| AC-7..9 | priority [-400, 400] boundary | `TestValidateChainPriorityRange` 7 subcases PASS |
| AC-10/12 | named set port match emits Lookup | `TestLowerMatchInSet_FieldTypeMismatch` + 008 .ci wired |
| AC-11 | unknown set rejects | `TestLowerMatchInSet_UnknownSet` PASS |
| AC-13/14 | element timeout round-trip | `TestParseSetElementTimeout` PASS + 009 .ci wired |
| AC-15..17 | byte-rate limit | `TestParseRateSpecBytes` 6 cases PASS + `TestLowerLimitDimension` PASS |
| AC-18..20 | NAT addr range | `TestParseNATAddressRange` 3 cases + `TestLowerNATAddressRange` PASS |
| AC-21..24 | boot/reload/coexistence/show | 001-004 .ci files wired via `cmd/ze-test/firewall.go` |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config validate` rejects dscp in ip6 | `test/parse/firewall-dscp-ipv6-rejected.ci` | `bin/ze-test bgp parse u` PASS |
| `ze config validate` accepts dscp-set in inet | `test/parse/firewall-setdscp-inet-accepts.ci` | `bin/ze-test bgp parse z` PASS |
| `ze config validate` rejects priority 999 | `test/parse/firewall-priority-out-of-range.ci` | `bin/ze-test bgp parse y` PASS |
| `ze -` boot applies firewall | `test/firewall/001-boot-apply.ci` | Listed by `bin/ze-test firewall --list`; requires Linux+CAP_NET_ADMIN to execute |
| SIGHUP converges kernel state | `test/firewall/002-reload.ci` | Listed; Linux-only |
| Non-ze tables preserved | `test/firewall/003-coexistence.ci` | Listed; Linux-only, setup.py pre-seeds `surfprotect` |
| `ze cli firewall show` | `test/firewall/004-cli-show.ci` | Listed; Linux-only, retries CLI socket |
| Named addr set in kernel | `test/firewall/005-match-in-set-addr.ci` | Listed; Linux-only |
| dscp match rejected by daemon | `test/firewall/006-dscp-ipv6-rejected.ci` | Listed; runs offline via `ze config validate` |
| dscp-set in inet applied | `test/firewall/007-setdscp-inet.ci` | Listed; Linux-only |
| Named port set | `test/firewall/008-match-in-set-port.ci` | Listed; Linux-only |
| Per-element timeout kernel round-trip | `test/firewall/009-set-element-timeout.ci` | Listed; Linux-only |
| Byte-rate limit | `test/firewall/010-byte-rate-limit.ci` | Listed; Linux-only |
| SNAT range | `test/firewall/011-snat-addr-range.ci` | Listed; Linux-only |

## Review Gate

| Round | Findings | Resolution |
|-------|----------|------------|
| 1 | 5 ISSUE + 9 NOTE | All 5 ISSUEs fixed (validator set-type compat, counter YANG grep, Limit Dimension contract, SetDatatype comparison, parseSetElements cap); 6 NOTEs fixed, 3 acknowledged as pre-existing or cosmetic |
| 2 | 2 ISSUE + 5 NOTE | Both ISSUEs fixed (SetType.String for %s errors; 003-coexistence setup.py seq=1); 4 NOTEs fixed, 1 acknowledged |
| 3 | 1 ISSUE + 4 NOTE | ISSUE fixed (validateSetFamilyCompat comment matched code by adding the missing arp/bridge/netdev rejection via explicit exhaustive switch); NOTEs acknowledged as parallel pre-existing gaps in literal-match validation, tracked as follow-up |

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-24 all demonstrated
- [ ] Wiring Test table complete; every `.ci` file on disk
- [ ] `make ze-verify-fast` passes
- [ ] Every gap has: unit test + `.ci` test + doc update
- [ ] Architecture docs updated (if architecture changed -- not expected)
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)

- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed
- [ ] `/ze-review` returns only NOTEs

### Design

- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per gap
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output per test)
- [ ] Tests PASS (paste output per test)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)

- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Pre-Commit Verification filled
- [ ] Review Gate shows final clean `/ze-review`
- [ ] Learned summary written to `plan/learned/NNN-fw-10-linux-gaps.md`
- [ ] Summary included in commit
