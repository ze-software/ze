# Spec: single-owner-reload-reconcile-5-isis-identity-purge

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | When an operator commits a new IS-IS `net` with a different System ID, Ze rebuilds its circuits and originates new LSPs under the new System ID, but never purges the LSPs it previously originated under the old System ID, leaving ghost routes in neighbors' LSDBs until MaxAge (1200 s). |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-single-owner-reload-reconcile.md` (Family S3, part 2 of 3).

## Task

**Root cause.** Half of journal row `unwired-feature` 2026-09-20 is already fixed at HEAD: `(*engine).reconcile` (`internal/plugins/isis/server.go:721-778`) detects `identityChanged` (`oldCfg.SystemID != newCfg.SystemID || oldCfg.Level != newCfg.Level || !slices.EqualFunc(oldCfg.NETs, newCfg.NETs, types.NET.Equal)`), updates SPF root and flooder System ID via `setConfig`, closes and reopens every desired circuit so outgoing IIHs/ISHs carry the new identity (`TestRFC3786NormalHelloIdentityReload`), and calls `e.originate()`.

However, `(*Originator).Originate` (`internal/plugins/isis/lsdb/origination.go:402-435`) builds `src := types.NewSourceID(node.SystemID, 0)` and only calls `purgeStaleFragmentsLocked(level, src, len(fragments))` for the NEW System ID. Every LSP fragment and DIS pseudo-node LSP this router originated under `oldCfg.SystemID` stays unpurged in the LSDB and in every neighbor's LSDB until MaxAge (default 1200 s) expires, leaving stale prefixes and transit adjacencies pointing at a vanished router.

**Goal.** When `(*engine).reconcile` detects `oldCfg.Present() && oldCfg.SystemID != newCfg.SystemID`, purge every own ordinary LSP fragment and DIS pseudo-node LSP originated under `oldCfg.SystemID` across every previously configured level (setting Remaining Lifetime to 0, re-signing if authentication is configured, arming SRM and transmitting the purge over active circuits before closing and reopening circuits under `newCfg.SystemID`, per ISO/IEC 10589 clause 7.3.16 / RFC 1142 Section 7.3.16), clear `lastOrigInput` / `pnLastInput`, and prove end-to-end neighbor purge on the wire against FRR in a named interop scenario.

**Why it matters for shipping.** Re-numbering a router's IS-IS System ID via commit immediately withdraws the old router's LSPs from the IGP domain instead of blackholing traffic toward the old System ID for up to 20 minutes.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, boundaries
- [ ] `ai/doors/routing-protocols.md` - IS-IS boundaries, RFC quote and interop scenario obligations
- [ ] `ai/doors/le-tooling.md` - interop lab conventions
- [ ] `docs/architecture/isis/isis-4-component-config.md` - reconcile and node identity
- [ ] `docs/architecture/isis/isis-5-adjacency.md` - circuit build and identity
- [ ] `docs/architecture/isis/isis-6-lsdb.md` - own-LSP origination and purge (`purge is not expiry`, `a purge must strip the body and be re-signed`)

### RFC / ISO References (Scope: protocol)
- [ ] `rfc/full/rfc1142.txt` Section 7.3.16 (IETF republication of ISO/IEC 10589 clause 7.3.16: LSP purging by setting Remaining Lifetime to 0, stripping TLVs except authentication if configured, and flooding over circuits)
- [ ] `rfc/short/rfc5304.md` / `rfc/short/rfc5310.md` (signing purged LSPs when authentication is enabled)

## Current Behavior (MANDATORY)

Verified at the producers on 2026-11-26.

**Source files read:**
- [ ] `internal/plugins/isis/server.go` - `reconcile` (lines 716-800) detects `identityChanged`, calls `setConfig(newCfg)` (updating `e.cfg`, `flooder.SetSystemID`, `spf.SetRoot`), closes and reopens every desired circuit, then calls `e.originate()`. It never purges LSPs originated under `oldCfg.SystemID`.
- [ ] `internal/plugins/isis/lsdb/origination.go` - `Originator.Originate` (lines 385-440) purges only excess fragments for `node.SystemID` via `purgeStaleFragmentsLocked(level, src, len(fragments))`; `Originator` has no method to purge every fragment and pseudo-node of an outgoing `SystemID`.
- [ ] `internal/plugins/isis/lsdb_wiring.go` - `originate` (lines 177-212) arms flood and emits `lsp-change` for `res.Purged`.
- [ ] `internal/plugins/isis/rfc3786_hello_identity_test.go` - `TestRFC3786NormalHelloIdentityReload` (lines 101-109) already proves circuits rebuild and IIH/ISH/new-LSP carry the new identity.

**Behavior to preserve:**
- Reloading an area-only NET change (same `SystemID`, different area ID) or level change continues rebuilding circuits and re-originating without purging the unchanged `SystemID`.
- Reloading a `SystemID` change still rebuilds circuits (`TestRFC3786NormalHelloIdentityReload`), re-roots SPF, and re-originates under `newCfg.SystemID`.
- Purged LSPs keep `docs/architecture/isis/isis-6-lsdb.md` invariants: strip non-authentication body TLVs, re-sign with the active level authentication key (if any), recompute Fletcher checksum, and retain in LSDB for `ZeroAgeLifetime` while flooding.

**Behavior to change:**
- Add `(*Originator).PurgeSystem(level Level, oldSys types.SystemID) []types.LSPID` in `internal/plugins/isis/lsdb/origination.go` to purge every unpurged own ordinary fragment (`pseudonode == 0`) and DIS pseudo-node LSP (`pseudonode != 0`) matching `oldSys` in `level`, returning the purged LSP IDs and clearing `lastSeq` / `answeredClaim` / `suspendUntil` for `oldSys`.
- In `(*engine).reconcile` (`internal/plugins/isis/server.go`), when `oldCfg.Present() && oldCfg.SystemID != newCfg.SystemID`, call `e.purgeIdentity(oldCfg)` BEFORE closing circuits and switching `e.cfg`: purge every `oldCfg.SystemID` LSP across `originationLevels(oldCfg.Level)`, arm SRM and flush an immediate flood send on open circuits so adjacent routers receive the zero-lifetime purges before the circuits close and reopen under `newCfg.SystemID`, and clear `lastOrigInput` / `pnLastInput` / `disPseudonode`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Node-identity circuit rebuild | `internal/plugins/isis/server.go` `engine.reconcile` (`identityChanged`); `rfc3786_hello_identity_test.go` `TestRFC3786NormalHelloIdentityReload` | Rebuilds every desired circuit, updates SPF root and flooder System ID, and originates under `newCfg.SystemID` | Extends: purges and floods all `oldCfg.SystemID` LSPs before tearing down circuits and installing `newCfg` |
| Own-LSP purge and signing | `internal/plugins/isis/lsdb/origination.go` `Originator.purgeStaleFragmentsLocked`, `encodeAndSign`; `lsdb/lsdb.go` `LSDB.MarkPurged` | Strips body, re-signs when authentication is configured, sets Remaining Lifetime = 0, retains for ZeroAgeLifetime, returns purged IDs | Extends: `Originator.PurgeSystem` reuses the exact same purge encoding, signing, and LSDB retention helper for all fragments and pseudo-nodes of `oldSys` |
| Flooder SRM arming and transmit | `internal/plugins/isis/lsdb_wiring.go` `armFlood`, `emitLSPChange`; `internal/plugins/isis/lsdb/flooding.go` `Flooder` | Arms SRM on eligible circuits and transmits queued LSPs/purges | Extends: `purgeIdentity` arms flood and triggers an immediate flush on active circuits before `closeCircuit` tears them down |

## Data Flow (MANDATORY)

### Entry Point
- Config commit / reload changing `isis { net ... }` (or `system-id`) -> `(*engine).reconcile(newCfg)`.

### Transformation Path
1. `reconcile` snapshots `oldCfg := e.cfg` and checks `systemIDChanged := oldCfg.Present() && oldCfg.SystemID != newCfg.SystemID`.
2. When `systemIDChanged` is true, BEFORE `e.setConfig(newCfg)` and BEFORE closing circuits:
   - `e.purgeIdentity(oldCfg)` acquires `electMu` -> `origMu` (matching `setConfig` lock order), calls `e.originator.PurgeSystem(level, oldCfg.SystemID)` for every level in `originationLevels(oldCfg.Level)`, arms SRM and emits `lsp-change` (`"purge"`) for every returned LSP ID, and flushes one synchronous flood pass over currently open circuits so neighbors receive the zero-lifetime purges while the old adjacencies are still up.
   - Clears `lastOrigInput`, `pnLastInput`, `pnLastOrigAt`, and `disPseudonode` entries for the retired identity.
3. `reconcile` then calls `e.setConfig(newCfg)`, closes and reopens every desired circuit (`identityChanged`), refreshes connected prefixes, calls `e.originate()` (originating fragment 0+ under `newCfg.SystemID`), and triggers SPF.
4. Rollback to `oldCfg` triggers `reconcile(oldCfg)`, which symmetrically purges `newCfg.SystemID` and restores `oldCfg.SystemID`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `isis` engine -> `isis/lsdb` | `Originator.PurgeSystem(level, oldSys) []types.LSPID` | Yes, existing internal package boundary |
| Ze IS-IS -> peer router (FRR) | Standard IS-IS zero-lifetime LSP purge PDUs (ISO/IEC 10589 clause 7.3.16 / RFC 1142 Section 7.3.16) on Ethernet LLC SAP `0xFE` | Yes, proven by new FRR interop scenario |

### Changed-Wire Census

No Ze-private wire protocol, RPC message, EventBus schema, YANG leaf, or persisted state format changes in this spec. On the IS-IS wire, Ze now emits standard ISO/IEC 10589 clause 7.3.16 zero-lifetime LSP purge PDUs (using the existing `packet.LSP` codec) when `SystemID` changes.

**Search evidence:** `internal/plugins/isis/lsdb/origination.go` already encodes and floods zero-lifetime purge LSPs via `purgeStaleFragmentsLocked` and `encodeAndSign`; `PurgeSystem` emits the identical ISO/IEC 10589 LSP PDU format with Remaining Lifetime = 0.

### Integration Points
- `internal/plugins/isis/lsdb/origination.go`: `PurgeSystem`.
- `internal/plugins/isis/server.go`, `lsdb_wiring.go`: `purgeIdentity` called from `reconcile` before `setConfig` / circuit teardown.
- `test/interop/scenarios/isis-system-id-change-frr/`: FRR interop scenario proving wire-level purge of old System ID LSP and convergence on new System ID.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Uses `Originator` -> `LSDB` -> `Flooder` -> `Transport` |
| No unintended coupling | Yes | Entire change stays inside `internal/plugins/isis/` and its interop scenario |
| No duplicated functionality | Yes | Reuses `Originator`'s existing purge encoding, signing, and LSDB mark-purged path |
| Zero-copy preserved where applicable | Yes | Control-plane config reload path only |
| Registration over hardcoding, outbound | Yes | Walks live LSDB entries matching `oldSys`, never hardcoding fragment or pseudo-node counts |
| Registration over hardcoding, inbound | Yes | No hardcoded interface or peer lists |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `engine.reconcile` with a new `net` carrying a different `SystemID` | -> | `purgeIdentity` -> `Originator.PurgeSystem` + flood | `TestSystemIDReloadPurgesOldLSPsBeforeCircuitRebuild` (`internal/plugins/isis/rfc3786_hello_identity_test.go`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An IS-IS node with established L1/L2 own LSPs (ordinary fragments and DIS pseudo-node LSPs) commits a `net` change with a new `SystemID` | Every own LSP and DIS pseudo-node LSP under the old `SystemID` is marked purged (Remaining Lifetime = 0) and flooded out active circuits before circuits close and reopen, and the new `SystemID` originates its fresh LSP set | Purged old-`SystemID` LSP headers remain in the local LSDB for `ZeroAgeLifetime` per ISO/IEC 10589 clause 7.3.16 so flooding completes, and neighbor-originated LSPs remain untouched |
| AC-2 | Authentication (RFC 5304 / RFC 5310) is configured when `SystemID` changes | Each purged old-`SystemID` LSP carries only the authentication TLV (TLV 10) with a valid signature computed by `encodeAndSign` and a recomputed Fletcher checksum, with all non-authentication body TLVs stripped | Only the authentication TLV (TLV 10) survives in a signed purge LSP per RFC 5304 section 2 / RFC 5310; all other body TLVs are stripped |
| AC-3 | A commit changes only the area address of `net` (same `SystemID`) | Circuits rebuild and re-originate own LSPs carrying the new area address and incremented sequence numbers under the unchanged `SystemID` (without purging the `SystemID`) | The node's existing `SystemID` LSPs remain active and advance by normal sequence increment because the router's `SystemID` identity did not change |
| AC-4 | A commit changing `SystemID` rolls back to the previous config | The transient new `SystemID`'s own ordinary and pseudo-node LSPs are marked purged (Remaining Lifetime = 0) and flooded out active circuits, and the original `SystemID` re-originates its fresh LSP set | Purged transient-`SystemID` LSP headers remain in the LSDB only for `ZeroAgeLifetime` while flooding completes; no active LSP of the transient `SystemID` survives |
| AC-5 | FRR interop scenario (`test/interop/scenarios/isis-system-id-change-frr/`) | After SIGHUP changes Ze's `net` to a new `SystemID`, FRR receives the zero-lifetime purge for the old `SystemID` immediately (rather than waiting for MaxAge 1200 s), establishes adjacency with the new `SystemID`, and installs routes via the new `SystemID` | FRR's own LSPs and routes to unrelated prefixes remain intact; no active route or unpurged LSP for Ze's old `SystemID` survives on FRR past purge flooding |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOriginatorPurgeSystemPurgesFragmentsAndPseudonodes` | `internal/plugins/isis/lsdb/rfc1195_origination_test.go` | AC-1, AC-2 (ordinary fragments + pseudo-node LSPs purged, signed when auth set, sequence state cleared) | |
| `TestSystemIDReloadPurgesOldLSPsBeforeCircuitRebuild` | `internal/plugins/isis/rfc3786_hello_identity_test.go` | AC-1, AC-3, AC-4 (captures wire frames proving zero-lifetime purge PDUs for old `SystemID` are transmitted before new `SystemID` IIHs/LSPs, area-only change does not purge, rollback purges transient ID) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `RemainingLifetime` of purged LSP | `0` | `0` (ISO/IEC 10589 clause 7.3.16 purge sentinel) | N/A (unsigned) | `> 0` is not a purge |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-system-id-reload-purge` (or covered by existing `test/isis/` reload suite + FRR interop below) | `test/isis/isis-system-id-reload-purge.ci` | Running daemon commits new IS-IS `net` with different `SystemID`; `show isis database` shows old `SystemID` LSP purged and new `SystemID` active | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `isis-system-id-change-frr` | `test/interop/scenarios/isis-system-id-change-frr/` | FRR | FRR receives zero-lifetime purge for Ze's old `SystemID` immediately on SIGHUP (without waiting 1200 s MaxAge) and converges routes on Ze's new `SystemID` (AC-5) | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add failing unit tests `TestOriginatorPurgeSystemPurgesFragmentsAndPseudonodes` and `TestSystemIDReloadPurgesOldLSPsBeforeCircuitRebuild`.
2. **Phase: LSDB `Originator.PurgeSystem`** -- implement `PurgeSystem(level Level, oldSys types.SystemID) []types.LSPID` in `internal/plugins/isis/lsdb/origination.go` quoting ISO/IEC 10589 clause 7.3.16 / RFC 1142 Section 7.3.16.
3. **Phase: Engine `purgeIdentity` on reload** -- call `e.purgeIdentity(oldCfg)` in `(*engine).reconcile` (`internal/plugins/isis/server.go`, `lsdb_wiring.go`) before `setConfig` and circuit teardown when `oldCfg.Present() && oldCfg.SystemID != newCfg.SystemID`, flushing the purge out open circuits.
4. **Phase: Functional & FRR interop proof** -- add `test/isis/isis-system-id-reload-purge.ci` and `test/interop/scenarios/isis-system-id-change-frr/`, proving the interop scenario fails with the purge reverted and passes with it applied.
5. **Phase: Docs** -- update `docs/architecture/isis/isis-4-component-config.md` and `docs/architecture/isis/isis-6-lsdb.md` to document `SystemID` reload purge ordering.

## Files to Modify
- `internal/plugins/isis/lsdb/origination.go`
- `internal/plugins/isis/lsdb/rfc1195_origination_test.go`
- `internal/plugins/isis/server.go`
- `internal/plugins/isis/lsdb_wiring.go`
- `internal/plugins/isis/rfc3786_hello_identity_test.go`
- `docs/architecture/isis/isis-4-component-config.md`
- `docs/architecture/isis/isis-6-lsdb.md`

## Files to Create
- `test/isis/isis-system-id-reload-purge.ci`
- `test/interop/scenarios/isis-system-id-change-frr/`

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| unwired-feature | 2026-09-20 | `buildCircuit` / `reconcile` on `net` or `system-id` change (circuit rebuild already landed; old-SystemID LSP purge remains) |

## Work Not Done

Sibling children of `plan/release/fixes/spec-single-owner-reload-reconcile.md`:
- `plan/release/fixes/spec-single-owner-reload-reconcile-1-tx-conformance.md` (reload-wide pre-autoload verification and registered conformance suite)
- `plan/release/fixes/spec-single-owner-reload-reconcile-2-iface-reconcile.md` (iface owed-apply guard, VPP learned-address proof, carrier resync epoch, DHCPv6 `IFLA_CARRIER_CHANGES`)
- `plan/release/fixes/spec-single-owner-reload-reconcile-3-firewall-nft-ownership.md` (nft `NFTA_TABLE_USERDATA` ownership marker and restart reclaim)
- `plan/release/fixes/spec-single-owner-reload-reconcile-4-web-form-atomic-save.md` (atomic web form save)

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "The RFC, then ExaBGP API compatibility, then ExaBGP's behavior, in that order (`ai/rules/rfc-compliance.md`)" | agrees | Purging old-SystemID LSPs with Remaining Lifetime 0 and stripped body follows ISO/IEC 10589 clause 7.3.16 and RFC 5304 section 2, verified against FRR in `test/interop/scenarios/isis-system-id-change-frr/`. |
| `docs/architecture/isis/isis-6-lsdb.md` | Trap: a purge must strip the body and be re-signed | the `// Design:` page of `origination.go`, `rfc1195_origination_test.go`; anchored to `server.go`, `lsdb_wiring.go`; a page this spec modifies | "RFC 5304 section 2 requires an originator that purges to remove the LSP body and add the authentication TLV. Every purge routes through the body strip and then the signer, so a purge can never carry a stray body and always carries a valid TLV 10 when signing is on." | agrees | `Originator.PurgeSystem` routes every ordinary fragment and pseudo-node purge through the existing `purgeLSP` path (body strip, lifetime 0, re-sign, re-checksum) and updates this page to document `PurgeSystem`. |
| `docs/architecture/wire/isis.md` | LSP origination, aging, and fragmentation (isis-6) | anchored to `internal/plugins/isis/lsdb/origination.go` | "The LSDB (`internal/plugins/isis/lsdb/`) stores every LSP per Ze's buffer-first model: the verbatim PDU bytes (a single owned copy, never an alias of the receive buffer) plus parsed freshness metadata (LSP ID, sequence, remaining lifetime, checksum, overload)." | agrees | `PurgeSystem` encodes standard ISO/IEC 10589 zero-lifetime purge LSP PDUs through the existing buffer-first `lsdb` encoder with no new wire format. |
| `ai/doors/routing-protocols.md` | RFC and interop obligations | the door covering `internal/plugins/isis/*` | "**A wire-visible change owes an interop scenario.** OSPF, OSPFv3, IS-IS, LDP and BFD scenarios against FRR and BIRD live in `test/interop/scenarios/`, named for the protocol, the case and the peer." | agrees | Emitting old-SystemID purges on the wire is proven against FRR by `test/interop/scenarios/isis-system-id-change-frr/`. |
| `docs/architecture/isis/isis-4-component-config.md` | Decision: a commit is written into the running circuit where it can be | the `// Design:` page of `server.go`; anchored to `server.go`, `lsdb_wiring.go`; a page this spec modifies | "The circuit is built from the node's system ID, NET and area addresses. Reconcile rebuilds existing circuits when the node's system ID, NET list or level changes, so the new identity reaches the running circuits." | agrees | Preserves circuit rebuild on `identityChanged` and adds `purgeIdentity(oldCfg)` immediately before circuit teardown when `oldCfg.SystemID != newCfg.SystemID`, updating this section to document the purge step. |
| `docs/architecture/isis/isis-10-auth.md` | Decision: one verify chokepoint, one sign hook per PDU class | anchored to `internal/plugins/isis/server.go` | — | silent | Covers IS-IS crypto and signing architecture; `PurgeSystem` automatically reuses the existing LSP sign hook via `purgeLSP` without changing auth code. |
| `docs/architecture/isis/isis-5-adjacency.md` | Decision: the state machine is pure | anchored to `server.go`; the `// Design:` page of `rfc3786_hello_identity_test.go` | — | silent | Covers adjacency FSM transitions; purging old-SystemID LSPs happens in LSDB/reconcile before circuit teardown and leaves adjacency FSM rules unchanged. |
| `docs/architecture/isis/isis-9-spf-rib.md` | Decision: install is Loc-RIB insertion, not a redistribute event | anchored to `internal/plugins/isis/lsdb_wiring.go` | — | silent | Covers SPF computation and Loc-RIB route insertion; purging old-SystemID LSPs removes stale LSDB entries via existing LSDB purge hooks without altering SPF/RIB logic. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/isis/isis-system-id-reload-purge.ci` | "A `.ci` that needs a Linux kernel feature carries `option=needs-linux`" | agrees | `test/isis/isis-system-id-reload-purge.ci` runs on the standard functional runner and follows `.ci` test conventions. |

## Design presentation

### What changes
- `lsdb.Originator` gains `PurgeSystem(level, oldSys)` to purge every own ordinary LSP fragment and DIS pseudo-node LSP belonging to `oldSys` (Remaining Lifetime = 0, stripped body, re-signed when auth is configured).
- `engine.reconcile` calls `e.purgeIdentity(oldCfg)` and flushes the zero-lifetime purges out active circuits before closing and reopening circuits under `newCfg.SystemID`.
- A named FRR interop scenario (`test/interop/scenarios/isis-system-id-change-frr/`) proves FRR drops the old System ID's LSP immediately without waiting for MaxAge.

### Before and after
- **Before:** Changing IS-IS `net` to a new `SystemID` rebuilt circuits and originated LSPs under the new `SystemID`, while old-`SystemID` LSPs lingered in local and neighbor LSDBs until MaxAge (1200 s).
- **After:** Before tearing down circuits under the old identity, Ze purges and floods all own ordinary and pseudo-node LSPs of the old `SystemID`, then rebuilds circuits and originates under the new `SystemID`.

### Alternatives set aside
- **Letting old-`SystemID` LSPs expire naturally at MaxAge:** Rejected because stale transit adjacencies and prefixes persist in every neighbor's LSDB for up to 20 minutes after an operator intentionally renumbers a router.
- **Purging only after reopening circuits under `newCfg.SystemID`:** Rejected because reopening circuits flaps adjacencies before the purge is sent, delaying or dropping delivery to neighbors; flushing the purge on the still-up circuits right before `closeCircuit` delivers it cleanly while adjacencies are still Up.

### Documents departed from
None.

### Choices for Thomas
none

## Review Gate

- [ ] `/ze-review` passed on diff
- [ ] `/ze-review-spec` confirmed all acceptance criteria and files

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Producer evidence re-read for every row
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-5 each proven by a named test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass in the main checkout: `./le test unit pkg path ./internal/plugins/isis/...`, `./le test isis`, and `./le test integration interop` with `INTEROP_SCENARIO=isis-system-id-change-frr`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` test and FRR interop scenario passing

### Closure
- [ ] Owned journal row marked `fixed:` with commit and test evidence
