# Spec: ADD-PATH limit becomes a send/receive container, with ingress enforcement

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P0 |
| Release reason | Enforce advertised receive limits and correct best-path ordering under existing peer limits. |
| Scope | protocol |
| Depends | - |
| Phase | 8/8 |
| Handoff | - |
| Updated | 2026-10-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Implement receive-limit enforcement and correct best-path selection and demotion withdrawal under an existing peer-advertised PATHS-LIMIT. Preserve the approved scalar-to-container migration and receive-only compatibility corpus. The absent local `send` cap is separately owned. Existing peer-limit best-path correctness must not wait for that cap; common selection machinery belongs here and is reused by the feature.

The 2026-10-27 rework (see Disposition) moves best-path ranking out of the session writer: the reactor holds no candidate set and no comparator, so ranking is asked of the RIB through the existing `ribevents` election producer, which already answers "every remaining candidate in election order" for source-DOWN recovery.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer config carries `add-path { limit { receive 10; } }` at container level with no family entries | Ze advertises PATHS-LIMIT 10 for every family it advertises in ADD-PATH, byte-identical to what the scalar `limit 10` produced |
| AC-2 | A peer config carries the old scalar `add-path { limit 10; }` | The config is REFUSED with an error naming `limit` and stating the replacement spelling (`write: limit { receive VALUE; }`). It does not load and it is not silently ignored |
| AC-6 | A peer advertised PATHS-LIMIT 2 and the local config carries no `send` | Ze admits at most 2 paths per prefix outbound. The ceiling is unchanged; best-path selection and demotion withdrawals follow AC-17-peer and AC-18-peer |
| AC-7 | Neither the peer nor the local config states a send limit | Ze applies no outbound bound and runs no election, which is today's behavior unchanged |
| AC-8 | The local config says `limit { receive 2; }` and the peer announces a third distinct path identifier for one prefix, in an UPDATE that also carries other prefixes at or under their ceiling | The excess path identifier is dropped before installation (Q-1: O-1, decided); the two admitted paths and every other prefix in the same UPDATE install normally |
| AC-9 | Following AC-8, the peer sends a WITHDRAW for the identifier Ze refused | The withdrawal is accepted, no error is raised, the session survives, and no slot is freed that was never taken |
| AC-10 | Following AC-8, the peer withdraws one admitted identifier and re-announces the refused one | The re-announced path is admitted, because the withdrawal freed a slot |
| AC-11 | The local config states `limit { receive 2; }` for a family whose ADD-PATH direction does not include receive | No ceiling is applied and no PATHS-LIMIT tuple is advertised for that family |
| AC-12 | An ExaBGP config carrying `add-path { ipv4 unicast limit 10; }` is migrated | The output carries `family { ipv4/unicast { limit { receive 10; } } }` and no `send` leaf |
| AC-13 | An ExaBGP config carrying `add-path { ipv4 unicast; }` with no limit is migrated | The output carries the family with no `limit` container at all |
| AC-19 | The local config says `limit { receive 2; }` and the peer sends five excess paths for one prefix across separate UPDATEs | Ze logs the refusal once for that prefix, not once per refused path, and the tracked path-id set for that prefix never exceeds 3 (2 admitted + 1 headroom) |
| AC-20 | Following AC-19, the peer withdraws a tracked (refused) identifier that had saturated the tracking set, then sends a new excess path for the same prefix | The withdrawal frees the tracking slot and clears the warned marker; the new excess path produces a fresh log line rather than staying silently suppressed |
| AC-21 | The `weak` verdict of DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7 (§3: "An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender."), moved here from the "Blocked by" table of the BGP child spec-rfc-verdict-fix-bgp (P-3 of `plan/release/validation/spec-rfc-verdict-test-fix-pass.md`, BGP c31, 2026-10-02; ownership moved here from that child, so this AC alone owns the verdict) after this spec's ingress ceiling (AC-8 to AC-11, AC-19, AC-20) lands | The row's tagged units prove the receive ceiling in both polarities with discrimination records, and an agent other than the tests' author re-judges the verdict `enforced` in `rfc/audit/draft-abraitis-idr-addpath-paths-limit/DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7.json` |
| AC-3-receive | Per-family `limit { receive 2; }` and container `limit { receive 10; }` | Advertise and enforce 2 for the named family and 10 for every other family Ze advertises with ADD-PATH receive |
| AC-15-receive | The editor completes inside `add-path` and inside `add-path > limit` (container and family level) | The first offers `direction`, `limit`, `family`. The second offers `receive` only; no `send` leaf exists until `plan/post-release/features/spec-add-path-local-send-cap.md` adds it |
| AC-16-receive | `limit { receive 0; }` or `limit { receive 65536; }` | Refused at load by the YANG range `1..65535` |
| AC-17-peer | A prefix has five candidate paths in the RIB for a destination whose peer advertised PATHS-LIMIT 2, with no local cap, and the paths arrive worst-first | After convergence the destination holds exactly the two paths the RIB's election ranks first, not the first two to arrive |
| AC-18-peer | Under that peer limit of 2, Ze holds the best two, then a path arrives that outranks one of them | Ze WITHDRAWS the demoted identifier before announcing the new one, so the peer never holds more than 2; a path later demoted back below the bound is re-announced only when an election again ranks it within the bound, under the identifier it held before |

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split changes planning ownership only. Status, existing dependencies and recorded evidence are retained; no implementation, verification or closure is claimed. Original criteria in the pre-split sections below are provenance, not a second active owner. Every test, documentation and review obligation follows its behavior owner.

| Work | Implementation owner |
|------|----------------------|
| Original AC-1/AC-2/AC-6 to AC-13/AC-19 to AC-21; receive portions of AC-3/AC-15/AC-16; peer-only AC-17/AC-18; common selection and migration | `plan/release/fixes/spec-add-path-limit-send-receive.md` |
| Local-send portions of original AC-3/AC-4/AC-5/AC-14 to AC-18; new send setting and surfaces | `plan/post-release/features/spec-add-path-local-send-cap.md` |

Work assigned to `plan/post-release/features/spec-add-path-local-send-cap.md` is not performed or closed by this spec. That spec reuses the election machinery built here: its local cap becomes a second input to the same per-family bound.

## Files to Modify

These producer paths cover only the release-owned repair and shared machinery; the pre-split list below retains the separate local-send work.

- `internal/component/bgp/yang/ze-bgp-conf.yang` - the two scalar `limit` leaves become `limit` containers holding one `receive` leaf, at the add-path container and in each family entry.
- `internal/component/config/parser.go` - the generic "expected '{' after NAME" refusal for a non-presence container given a value appends the `write:` line `containerValueFix` (`parser_diagnostics.go`) already builds, so `limit 10;` answers `write: limit { receive VALUE; }` (AC-2).
- `internal/component/bgp/reactor/config_capabilities.go` - `parseAddPathFromTree` reads `limit > receive` at both levels. Tuple derivation from `addPath.Families` is unchanged.
- `internal/component/bgp/reactor/session_paths_limit.go` - the per-prefix admission state, section filter, body rewrite and rollback journal become direction-neutral and shared by send and receive; the send side gains the contested-prefix marker and the election trigger.
- `internal/component/bgp/reactor/session_paths_limit_recv.go` (new) - receive-only parts: headroom tracking, warn-once, the refused counter, and the `processMessage` hook.
- `internal/component/bgp/reactor/session_read.go` - `processMessage` runs the receive ceiling before `checkPrefixLimits`, and feeds the withdrawals of the RFC 7606 treat-as-withdraw extra bodies into it.
- `internal/component/bgp/reactor/session_negotiate.go` - builds the receive state from `EncodingCaps.PathsLimitRecv` where `initPathsLimit` builds the send state.
- `internal/component/bgp/reactor/relay_recovery.go` and a new `internal/component/bgp/reactor/session_paths_limit_elect.go` - the election request for a contested prefix, staged on the destination's forward worker, applied withdraw-first.
- `internal/component/bgp/reactor/reactor_metrics.go` - `ze_bgp_paths_limit_refused_total{peer,family}` and `ze_bgp_paths_limit_election_failed_total{peer,family}`.
- `internal/core/bgp/ribevents/recovery.go` - `RecoveryRequest` gains a `limit` field (data, not a mode): nonzero on an ADD-PATH destination asks for at most that many survivors in election order instead of WITHDRAW plus Sent.
- `internal/component/bgp/plugins/rib/rib_recovery.go` - answers a limited ADD-PATH request by election over all candidates, nobody excluded unless named; the IPC command gains the matching argument.
- `internal/core/bgp/capability/encoding.go` - the `PathsLimitRecv` comment says Ze also enforces it.
- `internal/exabgp/migration/migrate.go` - `migrateAddPathToUnified` writes `limit { receive N; }`.

## Historical Combined Specification

The pre-split body follows and runs to the end of the file at the template's heading levels. It was reworked on 2026-10-27 for the work this spec owns (see Disposition); sections named "Pre-Split ... (provenance)" keep the former mixed text for `plan/post-release/features/spec-add-path-local-send-cap.md` and do not override the active ownership above.

## Pre-Split Task (provenance)

The ADD-PATH block carries one scalar `limit N`, at the container level and again per family. One number cannot say two things, and today it says only one: it is the number Ze advertises in PATHS-LIMIT capability 76, which is a request to the peer. It bounds nothing locally. `docs/guide/add-path.md` states this plainly ("This local limit is a receiver request, not a guarantee that the peer obeys it. It does not set Ze's outbound limit."), and the `rfc/short/` Support coverage row repeats it.

Three things are missing behind that one word.

1. **No ingress ceiling.** The draft asks for one. Section 3: "An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender." Ze advertises a number and then accepts whatever arrives, so a peer that ignores capability 76 grows Ze's adj-RIB-in without bound.
2. **No local outbound cap** (owned by `plan/post-release/features/spec-add-path-local-send-cap.md`).
3. **Outbound truncation picks arrival order, not best-path order.** `Session.pathsLimitSection` admits path identifiers in the order they arrive: a peer capped at two paths gets whichever two Ze received first, and a path that later becomes best is never substituted in. `docs/guide/add-path.md` states it: "The first admitted paths keep their slots. Ze does not select the best paths or queue suppressed paths for later transmission."

The owner decided the shape (Thomas, 2026-09-11): the scalar `limit` is REPLACED by a container using the `direction` vocabulary, `limit { send N; receive N; }`, at the container and family levels. This spec delivers the container with its `receive` leaf; `send` is added by the local-send spec.

- `receive` is the number of paths per prefix Ze accepts FROM the peer. It is the number Ze advertises in capability 76, because the draft's Paths Limit field "indicates the maximum paths limit the receiver wants to receive from its peer." It is also enforced on ingress.
- `limit N` is deleted, not kept as a shorthand (`ai/rules/no-layering.md`). An existing `limit 10` becomes `limit { receive 10; }`.
- Outbound truncation changes from arrival order to best-path order for every peer with an effective outbound bound (owner decision, 2026-09-11). Ze sends the best-ranked paths and WITHDRAWS an already-sent path that the election demotes out of the kept set.

### Design Decisions (owner, 2026-09-11 -- resolves the open questions below)

Both open questions below are DECIDED. The reasoning is kept in full: it explains why, and the rejected alternatives still bound future work.

### Q-1: what happens to a path over the receive ceiling (DECIDED: O-1, drop the excess path only)

**Decided: O-1, drop only the excess path identifiers, never the whole UPDATE.** The receive side admits paths up to the `receive` limit and discards the rest before installation. Every other prefix carried in the same UPDATE installs normally.

Neither RFC in scope prescribes a remedy. RFC 4271 Section 6.3: "All errors detected while processing the UPDATE message MUST be indicated by sending the NOTIFICATION message with the Error Code UPDATE Message Error." An over-limit path is not an error in the UPDATE: the message is well formed, and the enumerated subcodes name no such condition. RFC 4486 Section 4 binds Cease subcode 1 to "the number of address prefixes received from the neighbor exceeds a locally configured upper bound", a PREFIX count, not a per-prefix path count. The draft's only sender-side sentence is a SHOULD ("A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability.") and says nothing about the receiver's remedy. FRR does not police its own advertised limit on receive (no reader of `addpath_paths_limit[afi][safi].send` exists in `bgpd/bgp_route.c`), and BIRD implements no capability 76.

| Option | What Ze does | Verdict |
|--------|--------------|---------|
| O-1 Rewrite and admit the rest | Rewrite the UPDATE body with the over-limit NLRIs removed, using the same section filter and rollback journal as the send side's `filterPathsLimit`, then deliver the rewritten body | DECIDED |
| O-2 Drop the whole message | Reuse the `checkPrefixLimits` drop verdict | REJECTED. One over-limit path would discard every other prefix batched into the same UPDATE |
| O-3 Count, warn, do not drop | ExaBGP's `UpdateHandler._audit_announce` with `IncomingRIB.track_path` | Its AUDIT half is adopted alongside O-1: warn once per prefix, bounded tracking set |
| O-4 Terminate the session | NOTIFICATION Cease subcode 1 | REJECTED. No RFC authorizes it for a path count |

A later WITHDRAW for a path Ze never kept is accepted and frees no slot, as the send side already rules: "Withdrawals always pass, including unknown identifiers; only a NEW path consumes a slot".

**Warn once, bound the tracking set (owner decision, 2026-09-11).** ExaBGP's `IncomingRIB.track_path` caps its per-prefix tracking set at `limit + AUDIT_PATHS_HEADROOM` (headroom 1), so a peer ignoring the limit cannot decide how much memory the audit costs. Ze logs a refusal once per prefix per family. When a WITHDRAW arrives for a tracked identifier, it leaves the set and the warned marker clears, so a later excess warns again.

### Q-2: the ExaBGP send-side spelling (DECIDED: no per-direction form exists)

**Decided: ExaBGP carries no per-direction ADD-PATH limit.** The single `ipv4 unicast limit 10` grammar is the RECEIVE direction (`ParseAddPath._parse_addpath_family` in `src/exabgp/configuration/neighbor/family.py`, upstream commit `c2ce4d20ddff78bdbbe7ffd50276bdbb47646a3e` and branches `5.0`, `master`, `4.2`; owner confirmed 2026-09-11). `add-path { ipv4 unicast limit 10; }` becomes `limit { receive 10; }`, and a migrated config carries no `send` leaf.

### Design Decisions (rework, 2026-10-27 -- settled by the draft, FRR and BIRD; Thomas away, fix-spec rule in `ai/META-DECISIONS.md`)

| ID | Decision | Source that settles it |
|----|----------|------------------------|
| D-1 | Under a peer limit Ze sends the best-ranked paths, not the first arrivals, and withdraws a demoted path | Draft Section 1: "This indication allows the sender to optimize the transmission of BGP routes by selectively relaying pertinent routes instead of the entire set." FRR `subgrp_announce_addpath_best_selected` (`bgpd/bgp_updgrp_adv.c`, master fetched 2026-10-27) walks `bgp_dest_get_bgp_path_info(dest)`, which `bgp_best_selection` (`bgpd/bgp_route.c`) keeps sorted in preference order, announces while `paths_count++ < paths_limit` under the comment "If we have Paths-Limit capability, we MUST not send more than the number of paths expected by the peer.", and otherwise calls `subgroup_process_announce_selected(subgrp, NULL, ...)`, which withdraws that identifier. BIRD 3 has no capability 76 (`bgp_read_capabilities`, `proto/bgp/packets.c`, has no case 76) and with `add paths tx` sets `c->c.ra_mode = RA_ANY` (`proto/bgp/bgp.c`), sending every path, so it offers no competing rule |
| D-2 | Ranking is asked of the RIB, never computed in the reactor | The comparator (`SelectBest`, `internal/component/bgp/plugins/rib/bestpath.go`) and the candidate set live in the RIB plugin. `ribevents.RecoveryOutcome` already returns "Survivors are in election order: repeated SelectBest over the remainder, because the decision comparator is not transitive under MED and cannot sort." `reactorAPIAdapter.lookupRecovery` reaches it in process or over the `request bgp rib recovery` IPC command. Both forwarders that produce outbound multi-path traffic declare `bgp-rib` a dependency (`rs/register.go`, `rr/register.go`) |
| D-3 | An election runs only for a contested prefix: one whose admission refused a new path at the bound. A prefix at or under its bound never asks the RIB | `ai/rules/performance.md`; the common case (no limit, or fewer paths than the limit) keeps today's cost exactly (AC-7) |
| D-4 | The election never runs under `writeMu`; it is staged on the destination's forward worker, coalesced per prefix, and its result is re-validated at write against a per-prefix generation, re-queued when stale | `RecoveryOwner` contract: "Selection never runs under a destination write lock." `admitRecovery` stages recovery the same way |
| D-5 | Withdraw before announce, so the peer never holds more than its limit | FRR runs `subgrp_withdraw_stale_addpath` before `subgrp_announce_addpath_best_selected`; `filterPathsLimit` already applies "every withdrawal before admitting any announcement" |
| D-6 | A re-announced survivor keeps the destination identifier the forward rail minted for its source path (`fwdPathIDs`, `forward_path_id.go`) | RFC 7911 Section 5 treats a repeated (prefix, identifier) as replacement; reusing the minted identifier keeps that identity stable |
| D-7 | A path the RIB does not report (an API or transaction route that bypassed the RIB) keeps its slot and is never withdrawn by an election; elections fill only the slots RIB paths hold | No peer daemon has this case (FRR's path list holds every route). Withdrawing an operator-directed route on a rank the RIB cannot give would invent a ranking |
| D-8 | When the election cannot answer (no RIB plugin, lookup error), admission stays arrival order for that prefix, Ze logs one warning per session and family naming the cause and the fix (load `bgp-rib`), and counts `ze_bgp_paths_limit_election_failed_total` | `ai/rules/principles.md`: code that cannot answer MUST say so. The peer limit is still honoured, so no draft sentence is broken |
| D-9 | The receive ceiling and the send bound share one direction-neutral admission type (state, section filter, body rewrite, rollback journal); receive adds only headroom tracking and warn-once | `ai/rules/principles.md`, declared once; the Pre-Split O-1 text already calls the receive side a mirror of `filterPathsLimit` |
| D-10 | The receive ceiling runs after RFC 7606 handling and family validation and BEFORE `checkPrefixLimits`, so the prefix count sees the rewritten message; a prefix-limit drop or teardown rolls the path journal back | The path-count journal and the installed-family journal stay separate (Pre-Split Key Design Decision); ordering decides which one counts the excess path |
| D-11 | The rewritten receive body keeps the session's `ContextID`: only whole NLRIs are removed and lengths fixed, attributes are untouched, so the bytes are still encoded in that context. The cost is one copy for an over-limit message only | `docs/architecture/encoding-context.md`: `ContextID` names the encoding context. Corrects Pre-Split R-2 |
| D-12 | AC-2 is delivered by the parser's generic refusal of a value on a non-presence container, not by a validator on the deleted leaf | `containerValueFix` (`internal/component/config/parser_diagnostics.go`) already prints `name { child VALUE; }` for the leaf-list case; a per-leaf validator would keep the old spelling alive (`ai/rules/no-layering.md`) |
| D-13 | The receive-ceiling interop peer is BIRD 3, not FRR | FRR caps its TX at the received limit (D-1 quote), so it can never over-send and a scenario against it has no red phase. BIRD ignores capability 76 and sends every path (D-1) |
| D-14 (implementation, Phase 6) | A family Ze cannot split path by path (`nlrisplit.Supported` false) is answered by ONE predicate, `pathsLimitEnforceable` (`session_paths_limit.go`), for both advertise and enforce. A receive limit reaching such a family with ADD-PATH receive is REFUSED at config load (`parseAddPathFromTree`, error naming the family and three fixes). A send limit the peer advertises for it is negotiated, not enforced, and `warnPathsLimitUnenforced` logs one warning per session | Draft Section 3: the Paths Limit field "indicates the maximum paths limit the receiver wants to receive from its peer.", so advertising a receive limit Ze cannot hold states a ceiling that does not exist; the system door's exact-or-reject rule refuses that config. Before this spec, a missing splitter made `section` return an error on every UPDATE of the family (receive: whole UPDATE dropped; send: write error), which broke the family entirely; 3-6 is a SHOULD, so the send side names the gap instead of stopping the family |
| D-15 (implementation, Phase 6) | The owed election is written while the writer still holds the adj-RIB-Out table mutex: `owePathsLimitElections` and `owePathsLimitRoutes` are methods of `*adjOutWrite`, owe into `w.table`, and take no lock. `beginAdjOut` states the window as a MUST on its side | The first build called `table.mu.Lock()` from inside `beginAdjOut`..`release`, a self-deadlock on a non-reentrant mutex with `writeMu` held (every later write and the shutdown notification hung; `paths-limit-live.ci` timed out at 8m30s). Lock order everywhere else is `writeMu` then the table mutex (`pathsLimitHeld`) |
| D-16 (implementation, Phase 4) | The rewritten receive body is one heap copy (`make(len(payload))`) wrapped in a new `WireUpdate` with the session's ContextID; the original `BufHandle` stays with the caller and the recent-UPDATE cache, unchanged. This departs from R-2's "fresh pooled read buffer" wording | Ownership is the same as the existing ingress rewrites (`collapseASPathFamily`, `applyRFC7606`, `withdrawLegacyAnnouncements`), which also replace `wireUpdate` and leave `buf` alone; a pooled buffer handed onward with the WireUpdate would have no releaser (engine door: pooling a buffer handed back to the caller is a use-after-free). Only an UPDATE that refused a path pays the copy (D-11), named in `docs/architecture/buffer-architecture.md` |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/add-path.md` - the config surface being replaced and the only operator page describing PATHS-LIMIT
  → Constraint: "The first admitted paths keep their slots. Ze does not select the best paths or queue suppressed paths for later transmission." and "This local limit is a receiver request, not a guarantee that the peer obeys it." both stop being true; each changes in the phase that changes the code
  → Decision: the page's anchors on `config_capabilities.go -- parseAddPathFromTree` and `session_paths_limit.go -- pathsLimitFamily, pathsLimitSection, filterPathsLimit` are re-verified when those symbols move
- [ ] `docs/architecture/plugin/rib-storage-design.md` - the recovery election ("Survivors ... in election order"; "ADD-PATH destinations run no election")
  → Decision: the limited request is a data extension of that election; the page gains the limited ADD-PATH case
- [ ] `docs/architecture/bgp/structural-forwarding.md` - forward rails, minted identifiers
  → Constraint: a survivor re-announced by an election uses the identifier `fwdPathIDs` minted for its source path
- [ ] `docs/architecture/wire/capabilities.md` - anchor target for `session_paths_limit.go`
  → Constraint: capability 76's wire shape does not change
- [ ] `docs/architecture/testing/interop.md` - the discrimination walk and the vacuity traps
  → Constraint: each new ceiling test is observed RED by reverting the producer; tagged units go through `./le rfc record`
- [ ] `ai/patterns/config-option.md` - the YANG leaf template
  → Constraint: `receive` keeps `type uint16 { range "1..65535"; }`; the container has no presence
- [ ] `docs/features/exabgp-compatibility.md` - anchored on `migrate.go`
  → Constraint: the migration must not change what a converted config asks the peer for
- [ ] `ai/doors/bgp-engine.md` - engine traps
  → Constraint: "ADD-PATH is per family and per direction, never one bool"; "Pooling a buffer that is handed back to the caller is a use-after-free that every test passes" (the rewritten receive body)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` and `rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt`
  → Constraint: requirement 3-7 is implemented here; its Support coverage sentence changes when the ceiling ships
  → Constraint: 3-6 (sender SHOULD) must keep passing with no local cap; the election only reorders within the same bound
- [ ] `rfc/full/rfc4486.txt` Section 4 and `rfc/full/rfc4271.txt` Section 6.3
  → Constraint: no NOTIFICATION for an over-limit path (Q-1)
- [ ] `rfc/full/rfc7911.txt` Section 5
  → Decision: a repeated (prefix, identifier) is a replacement, so a re-announced survivor reuses its identifier (D-6)

**Key insights:**
- One ingress admission point exists before plugin delivery: `processMessage` (`session_read.go`), where `checkPrefixLimits` runs. Two extra receive dispatches exist at the treat-as-withdraw sites in the same function; both carry withdrawals only.
- The send side already solves per-prefix admission with a rollback journal; receive reuses it (D-9).
- Ranking exists only in the RIB plugin and is reachable from the reactor only through `ribevents` (D-2).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_paths_limit.go` (318L) - `pathsLimitFamily` (limit, split, withdrawSplit, key, prefixes), `pathsLimitPrefix` (key, ids), `pathsLimitChanges` journal with `rollback`/`release`; `initPathsLimit` builds state from `encoding.PathsLimitSend` for families with `AddPathSend`; `pathsLimitSection` admits in arrival order and drops a new path at the limit; `filterPathsLimit` rewrites a body, drops an MP_REACH whose NLRIs were all removed, and returns zero length when nothing remains
- [ ] `internal/component/bgp/reactor/session_read.go` - `processMessage`: RFC 7606 handling and two treat-as-withdraw extra dispatches (around lines 330 and 415), family validation, then `checkPrefixLimits` (teardown or silent drop with HoldTimer restart), then `onMessageReceived`
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `collectPrefixSections`, `checkPrefixLimits`
- [ ] `internal/component/bgp/reactor/session_write.go` - the writer calls `filterPathsLimit` under the write path and logs "PATHS-LIMIT withheld additional paths" at debug
- [ ] `internal/component/bgp/reactor/relay_recovery.go` - `lookupRecovery` (in process via `ribevents.RecoveryProvider`, else IPC `request bgp rib recovery`), `admitRecovery` stages outcomes on the destination's forward worker
- [ ] `internal/core/bgp/ribevents/recovery.go` - `RecoveryRequest` (destination, family, keys with excluded sources, add-path, sent-add-path), `RecoveryOutcome` (withdraw or survivors in election order, sent identifiers), `RecoveryRoute` (source, NLRI, attributes, next hop, message ID, path id), `CheckRecoveryAnswer`
- [ ] `internal/component/bgp/plugins/rib/rib_recovery.go` - `recoverySurvivorsLocked`, `recoverySentAddPathLocked`; an ADD-PATH destination today gets WITHDRAW plus Sent, no election
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - `Candidate`, `SelectBest`, `comparePair`
- [ ] `internal/component/bgp/reactor/config_capabilities.go` - `parseAddPathFromTree` (from line 310) reads container `limit` via `flexString` and per-family `limit` via `mapString`, then derives tuples from `addPath.Families`
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - two `leaf limit` (`uint16 { range "1..65535"; }`) around lines 1111 and 1135, documented as not enforcing
- [ ] `internal/component/config/parser.go` - `parseContainer` answers "expected '{' after NAME, got word" for a value on a non-presence container
- [ ] `internal/component/config/parser_diagnostics.go` - `containerValueFix` builds `name { child VALUE; }`
- [ ] `internal/core/bgp/capability/negotiated.go` - `negotiatePathsLimitDirection`: send takes the remote tuples under negotiated `AddPathSend`, receive takes the local ones under `AddPathReceive`
- [ ] `internal/core/bgp/capability/encoding.go` - `PathsLimitRecv` "our advertised limits (constrains peer's send)"
- [ ] `internal/exabgp/migration/migrate.go` - `migrateAddPathToUnified` and `parseLimit` emit a per-family scalar `limit`
- [ ] `internal/le/interoplab/bgp/check_paths_limit.go` - the FRR scenario's phases assert exact (path id, MED) sets in arrival order
- [ ] `internal/component/bgp/reactor/draft_abraitis_idr_addpath_paths_limit_config_test.go` - RFC-tagged; fixtures use the scalar `"limit": "3"`
- [ ] `test/plugin/paths-limit-live.ci`, `test/encode/paths-limit.ci`, `test/ui/cli-completion-peer-with-capability.ci`, `test/ui/cli-completion-addpath-fields.ci`, `test/ui/cli-completion-capability-addpath.ci`, `test/editor/completion/bgp-peer-capability-addpath.et`, `test/interop/scenarios/bgp-paths-limit-frr/ze.conf`, `test/exabgp-compat/etc/conf-paths-limit.conf` - in-tree scalar spellings
- [ ] `docs/architecture/config/syntax.md` - carries scalar `limit 10;` and `limit 5;` examples

**Behavior to preserve:**
- Send admission invariants: withdrawals always pass including unknown identifiers, replacement of an admitted identifier passes at capacity, a withdrawal frees a slot, a new connection starts empty, all writers including the route-server fast path share the state.
- The four proven draft MUSTs: one coalesced instance in OPEN, ignore PATHS-LIMIT without ADD-PATH, ignore a tuple whose family ADD-PATH did not carry, first duplicate wins.
- The `paths-limit` CLI and JSON key with `send` (peer's number) and `receive` (Ze's number); the meanings stay, `receive` is now also enforced.
- Tuples stay derived from `addPath.Families`.
- With no limit on either side: no state, no election, no rewrite.

**Behavior to change:**
- Scalar `limit` deleted at both levels; `limit { receive N; }` replaces it.
- `receive` is enforced on ingress.
- Under a peer limit, outbound converges on the RIB's election order, with withdraw-first demotion.
- `docs/features.md`, `docs/guide/add-path.md` and the `rfc/short/` Support coverage stop saying the knob does not police.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `session > capability > add-path` read by `parseAddPathFromTree`; `limit` arrives as a nested map.
- Wire, receive: an UPDATE on the session read goroutine as a `wireu.WireUpdate`, path identifiers prefixed to NLRIs in families with negotiated ADD-PATH receive.
- Wire, send: any writer reaching `filterPathsLimit` for a destination with negotiated ADD-PATH send and a peer limit.

### Transformation Path
1. `parseAddPathFromTree` resolves each family's `receive` by inheritance and builds `capability.PathsLimitEntry` tuples from `addPath.Families`, unchanged on the wire.
2. Negotiation stores the peer's tuples as `PathsLimitSend` and Ze's as `PathsLimitRecv`, each filtered by the matching negotiated direction.
3. Receive: at negotiation the session builds receive admission state for each family with a nonzero `PathsLimitRecv`. `processMessage` runs the ceiling after RFC 7606 handling and family validation: withdrawals first (free admitted slots and tracked refused identifiers, clear warned markers), then announcements (admit up to the limit, refuse the rest, track at most one refused identifier per prefix, warn once). If anything was refused, the body is rewritten into a pooled read buffer and a new `WireUpdate` replaces the original; the original buffer is returned. `checkPrefixLimits` then runs on the rewritten message; on its drop or teardown the path journal rolls back. A message left with no NLRI and no withdrawal is consumed like a prefix-limit drop (HoldTimer restart, no delivery), never delivered as an empty body that reads as End-of-RIB.
4. The treat-as-withdraw extra bodies pass their withdrawals through the ceiling before dispatch.
5. Send: `pathsLimitSection` is unchanged for admission; when it refuses a new path at the bound it marks the prefix contested and queues one election per prefix on the destination's forward worker.
6. Election: the worker asks `lookupRecovery` with the destination, family, the prefix key, nobody excluded, sent-add-path true and `limit` = the bound. The RIB answers at most `limit` survivors in election order. The worker computes withdraw = admitted RIB identifiers not in the answer, announce = answer members not admitted, maps each survivor to its minted identifier, and stages one admission that writes withdrawals before announcements. At write the prefix generation is compared; a stale result is discarded and re-queued. The contested marker clears when the RIB reports no more candidates than the bound.
7. Further triggers on a contested prefix: withdrawal of an admitted identifier, and replacement of an admitted identifier's attributes, each queue an election.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree to reactor | nested map under `add-path > limit` | No |
| Reactor to wire | capability 76 tuples in OPEN, unchanged | No |
| Session to plugins | `onMessageReceived` gets the rewritten body; refused NLRIs cross nothing | No |
| Reactor to RIB | `ribevents.RecoveryRequest` in process, or `request bgp rib recovery` over IPC with the new limit argument | No |
| ExaBGP config to Ze config | `migrateAddPathToUnified` writes the container | No |

### Integration Points
- `pathsLimitFamily`, `pathsLimitSection`, `filterPathsLimit`, `pathsLimitChanges` - become the shared admission type (D-9).
- `checkPrefixLimits` - runs after the ceiling; its rollback triggers the path-journal rollback (D-10).
- `lookupRecovery`, `admitRecovery`, `ribevents.CheckRecoveryAnswer` - the election transport and staging, reused.
- `fwdPathIDs` (`forward_path_id.go`) - destination identifiers for survivors (D-6).
- `containerValueFix` - the refusal text (D-12).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | ingress ceiling sits at the one pre-delivery admission point; elections write through the ordinary staged rail like recovery |
| No unintended coupling (components stay isolated) | Yes | the reactor reaches the RIB only through `ribevents` or IPC, never by import |
| No duplicated functionality (extends existing, does not recreate) | Yes | one admission type for both directions (D-9); the existing election producer (D-2) |
| Zero-copy preserved where applicable (refs, not copies) | Yes | only an over-limit message is copied (D-11) |
| Registration over hardcoding (`ai/rules/plugins.md`) | Yes | the RIB stays a published provider; no switch or list added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `processMessage` is the only receive-direction admission point before plugin delivery | grep of `onMessageReceived(` in `internal/component/bgp/reactor`: three receive-direction calls, all in `processMessage` (main body and the two treat-as-withdraw extras); the other two are `DirectionSent` in `session_write.go` | the ceiling misses a path | grep, 2026-10-27 | confirmed |
| A-2 | The advertised `receive` and the enforced ceiling are one number | Draft Section 3 Paths Limit field text; owner decision 2026-09-11 | an operator wants two numbers | owner decision, not reopened | confirmed |
| A-3 | No ExaBGP release carries a per-direction limit | Q-2 | migration maps the wrong half | owner, 2026-09-11 | confirmed |
| A-4 | ExaBGP's per-family `limit` is the RECEIVE direction | Q-2 | wrong direction migrated | owner, 2026-09-11 | confirmed |
| A-5 | FRR can be made to over-send | FRR `subgrp_announce_addpath_best_selected` caps TX at the received limit | n/a | read FRR master 2026-10-27 | broken: FRR cannot over-send; the receive scenario uses BIRD 3 (D-13) |
| A-6 | The in-tree scalar consumers are the files listed in Current Behavior | grep of numeric `limit` in add-path contexts over `test/`, `docs/`, `etc/`, `internal/`, 2026-10-27 | a config breaks unseen | the grep in the Deliverables Checklist, rerun in Phase 1 | confirmed |
| A-7 | Every forwarder that emits multi-path to an ADD-PATH destination has the RIB loaded | `rs/register.go` Dependencies `bgp-rib`; `rr/register.go` Dependencies `bgp-adj-rib-in`, `bgp-rib` | elections fail for those destinations | read 2026-10-27 | confirmed |
| A-8 | BIRD 3.4.0 with `add paths tx` sends several paths for one prefix when several protocols feed it | `proto/bgp/bgp.c` sets `ra_mode = RA_ANY` when ADD-PATH TX is negotiated; the master table holds one route per source | the BIRD scenario cannot over-send | Phase 8 builds the scenario and observes more than two paths offered (a pcap or BIRD's `show route export`) before asserting Ze's ceiling | confirmed 2026-10-27 (Phase 8): with the ceiling bypassed by overlay, Ze held 4 paths of 10.70.0.0/24 from BIRD 3.4.0; with it, held 2 and refused 2 (`checkPathsLimitReceive`, `internal/le/interoplab/bgp/check_paths_limit_receive.go`) |
| A-9 | The RIB can answer a limited ADD-PATH election from `gatherCandidatesLocked` without new storage | the recovery path already gathers candidates per key for non-ADD-PATH destinations | Phase 5 needs a storage change | Phase 5 first step reads `recoverySurvivorsLocked` | confirmed 2026-10-27: `gatherPrefixCandidatesLocked`/`gatherKeyCandidatesLocked` (`rib_commands.go`) return every stored path of every peer for the prefix with `PathID` and `AddPath` set, path id dropped from the key; `recoverySurvivorsLocked` already elects by repeated `SelectBest`; Phase 5 needed no storage change |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The ingress ceiling makes received-path selection arrival-order dependent | the preferred path arrives last and is refused | Decided by O-1; documented in `docs/guide/add-path.md`, proven by a test that delivers the preferred path last |
| R-2 | A rewritten receive body breaks buffer ownership | race detector or a use-after-free under the forward cache | rewrite into a fresh pooled read buffer, return the original, hand the new `BufHandle` to `onMessageReceived` (D-11; engine door trap on pooled buffers) |
| R-3 | Per-prefix receive state is peer-sized memory | map growth under a flood | at most limit+1 identifiers per prefix; prefixes bounded by the mandatory per-family `prefix { maximum }` that `checkPrefixLimits` enforces; state freed on withdrawal |
| R-4 | WITHDRAW of a refused identifier double-frees | AC-9 test | withdrawals always pass; only a tracked or admitted identifier is removed |
| R-5 | Migration changes what a converted config asks for | no test today | migration unit tests and the exabgp-compat fixture land in Phase 2 |
| R-6 | An election storm: a peer flapping many contested prefixes floods RIB lookups | lookup latency, worker queue depth | one pending election per prefix (coalesced), elections only on contested prefixes, keys batched per destination and family as `RecoveryRequest` already allows |
| R-7 | Deleting the scalar breaks operator configs | a config that loaded yesterday is refused | the refusal prints the replacement line (AC-2); the add-path page carries before and after |
| R-8 | The published `Supported` row over-claims 3-7 until the ceiling is proven | already true | tagged test plus discrimination record (AC-21), not a downgrade |
| R-9 | Best-path order changes what a peer-capped destination receives with no config change | FRR scenario phase expectations change | documented in `docs/guide/add-path.md` and `docs/comparison.md`; the changed FRR checker expectations go to `ze-test-judge` |
| R-10 | An election result goes stale between lookup and write (a new path, a withdrawal) | test with interleaved writes | per-prefix generation compared at write; stale results re-queue (D-4) |
| R-11 | A transient over-limit announcement if announce precedes withdraw | peer sees limit+1 | withdraw-first within the staged admission (D-5); unit test asserts write order |
| R-12 | The election fails silently and arrival order looks like best order | none, by construction | D-8 warning plus counter; unit test with a failing provider asserts both |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A wrong migration changes what a converted config asks peers for. A wrong ingress ceiling drops routes the operator expected. A wrong election withdraws good paths from a peer-capped destination. The best-path reorder changes outbound selection for every peer that already advertises a limit |
| How is it reverted? | Code reverts per commit. Configs written as `limit { receive N; }` are refused by a reverted binary |
| Who else touches this path? | `session_prefix.go` (prefix limits), `session_paths_limit.go` (FRR interop scenario, `paths-limit-live.ci`), `relay_recovery.go` and `rib_recovery.go` (source-DOWN recovery) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer config `add-path { limit { receive 2; } }` loaded from file | → | `parseAddPathFromTree` emits the capability 76 tuple | `TestParseAddPathLimitContainerReceive` and `test/encode/paths-limit.ci` |
| Config file still carrying `add-path { limit 10; }` | → | `parseContainer` refusal with `containerValueFix` | `test/parse/addpath-scalar-limit-refused.ci` |
| A peer UPDATE carrying a third path for a prefix at ceiling 2 | → | the receive ceiling reached from `processMessage` before `onMessageReceived` | `TestProcessMessageRefusesPathOverReceiveCeiling` and `test/plugin/paths-limit-receive-ceiling.ci` |
| A peer repeatedly over-sends on one prefix | → | warn-once and the bounded tracking set | `TestReceiveCeilingWarnsOncePerPrefix` |
| Peer advertised limit 2; a third, better path reaches the destination's writer | → | contested marker, election via `lookupRecovery`, withdraw-first admission | `TestPathsLimitElectionWithdrawsDemotedPath` and `test/plugin/paths-limit-outbound-best-path.ci` |
| The reactor asks the RIB for a limited ADD-PATH election | → | `rib_recovery.go` answers survivors in election order | `TestRecoveryLimitedAddPathElectionOrder` |
| An ExaBGP config carrying `ipv4 unicast limit 10` is migrated | → | `migrateAddPathToUnified` | `TestMigrateAddPathLimitBecomesReceiveContainer` and `test/exabgp-compat/encoding/conf-paths-limit.ci` |

## Pre-Split Acceptance Criteria (provenance)

Rows AC-3, AC-4, AC-5, AC-14, the `send` halves of AC-15, AC-16, AC-17 and AC-18 are owned by `plan/post-release/features/spec-add-path-local-send-cap.md`. The active criteria are the table at the top of this file.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-3 | `family { ipv4/unicast { limit { receive 2; send 4; } } }` and container `limit { receive 10; }` | ipv4/unicast advertises 2 and caps outbound at 4; others advertise 10, no outbound cap |
| AC-4 | Peer advertised 2, local `send 4` | at most 2 outbound |
| AC-5 | Peer advertised none, local `send 4` | at most 4 outbound |
| AC-14 | `show bgp peer <ip> capabilities` with both a peer limit and a local send cap | the payload distinguishes the peer's request from Ze's cap, in json, yaml and table |
| AC-15 | completion inside `add-path` and `add-path > limit` | `direction`, `limit`, `family`; then `send` and `receive` |
| AC-16 | `receive 0` or `send 0` | refused by the range |
| AC-17 | 5 candidates, bound 2, from a local `send` cap or a peer cap | the 2 best-ranked paths |
| AC-18 | a better path arrives at the bound, both cap sources | withdraw the demoted, announce the new best |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `limit { receive 2; }` and peers with a daemon that ignores capability 76 | config -> OPEN capability 76 -> peer over-sends -> `processMessage` ceiling -> RIB holds 2 | `test/plugin/paths-limit-receive-ceiling.ci`, interop `bgp-paths-limit-receive-bird` |
| 2 | Migrates an ExaBGP config carrying `ipv4 unicast limit 10` | `migrateAddPathToUnified` -> `limit { receive 10; }` -> OPEN carries 10 | `TestMigrateAddPathLimitBecomesReceiveContainer`, `test/exabgp-compat/encoding/conf-paths-limit.ci` |
| 3 | Loads a config still carrying `limit 10` | parser refusal naming the replacement | `test/parse/addpath-scalar-limit-refused.ci` |
| 4 | Runs a route server whose ADD-PATH client advertised limit 2, and the best path arrives last | rs forward -> writer refuses at bound -> election -> withdraw worst, announce best | `test/plugin/paths-limit-outbound-best-path.ci`, interop `bgp-paths-limit-best-path-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseAddPathLimitContainerReceive` | `internal/component/bgp/reactor/config_capabilities_test.go` | container `limit { receive N; }` yields the tuples the scalar did (AC-1) | |
| `TestParseAddPathLimitContainerPerFamilyOverride` | `internal/component/bgp/reactor/config_capabilities_test.go` | family `receive` overrides the container value (AC-3-receive) | |
| `TestParseAddPathLimitReceiveWithoutReceiveDirection` | `internal/component/bgp/reactor/config_capabilities_test.go` | no tuple and no ceiling for a send-only family (AC-11) | |
| `TestContainerGivenValueNamesReplacement` | `internal/component/config/parser_test.go` | a value on a non-presence container answers `write: NAME { child VALUE; }` (AC-2) | |
| `TestReceiveCeilingAdmitsUpToLimit` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | ceiling 2: two identifiers admitted, the third refused, other prefixes untouched (AC-8) | |
| `TestProcessMessageRefusesPathOverReceiveCeiling` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | the refused identifier never reaches the delivery callback; the rewritten body keeps every other NLRI (AC-8) | |
| `TestReceiveCeilingAcceptsWithdrawOfRefusedPath` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | WITHDRAW of a refused identifier passes and frees no admitted slot (AC-9) | |
| `TestReceiveCeilingReadmitsAfterWithdraw` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | a freed slot admits the previously refused identifier (AC-10) | |
| `TestReceiveCeilingBoundsTrackedPaths` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | tracked set per prefix never exceeds limit+1 (AC-19, R-3) | |
| `TestReceiveCeilingWarnsOncePerPrefix` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | five excess paths, one log line, five counter increments (AC-19) | |
| `TestReceiveCeilingClearsWarnedMarkerOnWithdraw` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | withdraw of the tracked identifier clears the marker; next excess warns again (AC-20) | |
| `TestReceiveCeilingRollsBackOnPrefixLimitDrop` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | a `checkPrefixLimits` drop after admission leaves the receive state as before the message (D-10) | |
| `TestReceiveCeilingSeesTreatAsWithdrawExtras` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | a treat-as-withdraw extra body frees the admitted slot (AC-10 via RFC 7606 path) | |
| `TestReceiveCeilingEmptyResultIsNotEndOfRIB` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | an UPDATE whose only NLRIs are refused is consumed, not delivered as an empty body | |
| `TestReceiveCeilingAbsentWithoutReceiveDirection` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | no state for a family without negotiated ADD-PATH receive (AC-11) | |
| `TestPathsLimitSectionSharedAdmission` | `internal/component/bgp/reactor/session_paths_limit_test.go` | send-side invariants unchanged after the shared extraction (Behavior to preserve) | |
| `TestRecoveryLimitedAddPathElectionOrder` | `internal/component/bgp/plugins/rib/rib_recovery_test.go` | limited ADD-PATH request returns at most N survivors in `SelectBest` order, including MED non-transitive sets | |
| `TestRecoveryUnlimitedAddPathUnchanged` | `internal/component/bgp/plugins/rib/rib_recovery_test.go` | limit 0 keeps WITHDRAW plus Sent for ADD-PATH destinations | |
| `TestRecoveryCommandParsesLimit` | `internal/component/bgp/plugins/rib/rib_recovery_test.go` | the IPC command carries the limit argument, refuses a malformed one | |
| `TestPathsLimitElectionAdmitsBestOrder` | `internal/component/bgp/reactor/session_paths_limit_elect_test.go` | worst-first arrival of 5 paths at bound 2 converges on the election's top 2 (AC-17-peer) | |
| `TestPathsLimitElectionWithdrawsDemotedPath` | `internal/component/bgp/reactor/session_paths_limit_elect_test.go` | a better path causes a withdraw then announce, never limit+1 held (AC-18-peer, D-5) | |
| `TestPathsLimitElectionReusesMintedIdentifier` | `internal/component/bgp/reactor/session_paths_limit_elect_test.go` | a re-elected path returns under its earlier identifier (AC-18-peer, D-6) | |
| `TestPathsLimitElectionStaleResultRequeues` | `internal/component/bgp/reactor/session_paths_limit_elect_test.go` | generation mismatch at write discards and re-queues (D-4, R-10) | |
| `TestPathsLimitElectionKeepsNonRIBPaths` | `internal/component/bgp/reactor/session_paths_limit_elect_test.go` | an admitted path the RIB does not report is never withdrawn (D-7) | |
| `TestPathsLimitElectionFailureWarnsOnce` | `internal/component/bgp/reactor/session_paths_limit_elect_test.go` | provider error: arrival order kept, one warning, counter increments (D-8) | |
| `TestPathsLimitNoBoundNoElection` | `internal/component/bgp/reactor/session_paths_limit_elect_test.go` | no peer limit: no state, provider never called (AC-7) | |
| `TestPathsLimitUncontestedPrefixNoElection` | `internal/component/bgp/reactor/session_paths_limit_elect_test.go` | at or under the bound the provider is never called (AC-6, D-3) | |
| `TestMigrateAddPathLimitBecomesReceiveContainer` | `internal/exabgp/migration/migrate_addpath_test.go` | `ipv4 unicast limit 10` becomes `limit { receive 10; }` (AC-12) | |
| `TestMigrateAddPathWithoutLimitEmitsNoContainer` | `internal/exabgp/migration/migrate_addpath_test.go` | a bare family emits no `limit` (AC-13) | |
| `TestMigrateAddPathDirectionAndLimitTogether` | `internal/exabgp/migration/migrate_addpath_test.go` | capability-level direction plus a per-family limit produce both | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `add-path > limit > receive` | 1-65535 | 65535 | 0 | 65536 |
| `add-path > family > limit > receive` | 1-65535 | 65535 | 0 | 65536 |
| paths per prefix admitted inbound at ceiling 1 | 0-1 | 1 | N/A | 2 refused |
| tracked identifiers per prefix at ceiling 2 | 0-3 | 3 | N/A | 4 never reached |
| election survivors at bound 1 | 1 | 1 | N/A | 2 never written |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `paths-limit-receive-ceiling` | `test/plugin/paths-limit-receive-ceiling.ci` | `limit { receive 2; }`; the test peer sends three paths for one prefix plus another prefix in the same UPDATE, then withdraws the refused one, then an admitted one, then re-announces; the RIB and `show bgp peer` reflect AC-8 to AC-10, and the log carries one refusal line | |
| `paths-limit-outbound-best-path` | `test/plugin/paths-limit-outbound-best-path.ci` | a route-server destination advertises limit 2; three sources announce paths for one prefix worst-first; the destination's received UPDATE sequence shows the withdraw of the worst before the announce of the best (AC-17-peer, AC-18-peer) | |
| `addpath-scalar-limit-refused` | `test/parse/addpath-scalar-limit-refused.ci` | `ze config validate` refuses `limit 10;` with `write: limit { receive VALUE; }` (AC-2) | |
| `paths-limit-live` (edit) | `test/plugin/paths-limit-live.ci` | spelling `limit { receive 9; }`; outbound admission proof unchanged (its prefixes stay uncontested or are re-judged) | |
| `conf-paths-limit` (edit) | `test/exabgp-compat/encoding/conf-paths-limit.ci` | same OPEN bytes after migration | |
| `paths-limit` encode (edit) | `test/encode/paths-limit.ci` | OPEN still carries code 76 limit 10 through the container | |
| completion tests (edit) | `test/ui/cli-completion-capability-addpath.ci`, `test/ui/cli-completion-addpath-fields.ci`, `test/ui/cli-completion-peer-with-capability.ci`, `test/editor/completion/bgp-peer-capability-addpath.et` | `limit` descends and offers `receive` (AC-15-receive) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-paths-limit-frr` (edit, spelling only) | `test/interop/scenarios/bgp-paths-limit-frr/` | FRR (lab latest stable) | the container spelling negotiates the same limits; checker `check_paths_limit.go` unchanged. Its routes are API-originated and bypass the RIB, so D-7 pins them and no election can run there | done (Phase 1) |
| `bgp-paths-limit-best-path-frr` (new, replaces the planned best-path phase above) | `test/interop/scenarios/bgp-paths-limit-best-path-frr/` | FRR (lab latest stable) receiving, BIRD 3 feeding Ze | FRR advertises PATHS-LIMIT 2; BIRD's MED 300/200/100 paths are enabled worst-first; FRR ends holding the two best MEDs, and a path demoted then re-elected returns under its earlier identifier (AC-17-peer, AC-18-peer, D-6). Red: owePathsLimitElections made a no-op, FRR kept MED 300 and 200 after MED 100 | done (Phase 8) |
| `bgp-paths-limit-receive-bird` | `test/interop/scenarios/bgp-paths-limit-receive-bird/` | BIRD 3.4.0 | BIRD ignores capability 76 and sends four paths for one prefix; Ze with `limit { receive 2; }` holds two, and every other prefix installs (AC-8, AC-21 goal). Red phase: revert the ceiling, rebuild the Ze image, observe four | |

## Pre-Split Files to Modify (provenance)

The local-send spec's surfaces: `internal/component/bgp/reactor/config.go` (send-cap map on `PeerSettings`), `internal/component/bgp/plugins/cmd/peer/fields.go`, `internal/component/bgp/format/decode.go`, `internal/component/bgp/format/json.go` (payload coherence once `send` can be local), `internal/component/plugin/types_bgp.go`, `test/plugin/paths-limit-local-send-cap.ci`, and the `send` leaf in YANG.

## Files to Create

- `internal/component/bgp/reactor/session_paths_limit_recv.go` and `session_paths_limit_recv_test.go`
- `internal/component/bgp/reactor/session_paths_limit_elect.go` and `session_paths_limit_elect_test.go`
- `internal/exabgp/migration/migrate_addpath_test.go`
- `internal/component/bgp/reactor/config_capabilities_test.go` (absent at HEAD; the three `TestParseAddPathLimit*` units)
- `test/plugin/paths-limit-receive-ceiling.ci`, `test/plugin/paths-limit-outbound-best-path.ci`, `test/parse/addpath-scalar-limit-refused.ci`
- `test/interop/scenarios/bgp-paths-limit-receive-bird/` (`ze.conf`, `bird.conf`) and its checker under `internal/le/interoplab/bgp/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-bgp-conf.yang` add-path container and family list |
| YANG validation constraints | Yes | `receive` keeps `uint16 { range "1..65535"; }`; container without presence |
| YANG custom validators | N-A | AC-2 uses the parser's generic refusal (D-12) |
| CLI commands/flags | N-A | no command added; `request bgp rib recovery` gains an argument (internal command, documented in `rib-storage-design.md`) |
| CLI grammar (keyword before value) | Yes | `limit { receive 2; }` |
| Editor autocomplete | Yes | automatic from YANG; completion tests edited |
| Functional test for new RPC/API | Yes | `paths-limit-receive-ceiling.ci`, `paths-limit-outbound-best-path.ci` |
| Pipe completeness | N-A | the `paths-limit` payload shape does not change |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Prometheus counters/metrics | Yes | `ze_bgp_paths_limit_refused_total{peer,family}`, `ze_bgp_paths_limit_election_failed_total{peer,family}` in `reactor_metrics.go` |
| BGP family surface (new SAFI / capability / attribute) | N-A | capability 76 exists, wire shape unchanged |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` PATHS-LIMIT row (Phase 4) |
| 2 | Config syntax changed? | Yes | `docs/guide/add-path.md`, `docs/config-reference.md`, `docs/architecture/config/syntax.md`, `docs/guide/configuration.md` (Phase 1) |
| 3 | CLI command added/changed? | N-A | no operator command changes |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/plugin/rib-storage-design.md` recovery section, limited ADD-PATH election (Phase 5) |
| 5 | Plugin added/changed? | Yes | the RIB recovery command gains an argument, same page as row 4 |
| 6 | Has a user guide page? | Yes | `docs/guide/add-path.md` enforcement sections (Phases 4 and 6) |
| 7 | Wire format changed? | N-A | wire unchanged; `docs/architecture/wire/capabilities.md` anchor re-verified when symbols move |
| 8 | Plugin SDK/protocol changed? | N-A | `pkg/` untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` 3-7 and Support coverage (Phase 7) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` scenario list for `bgp-paths-limit-receive-bird` (Phase 8) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: receive ceiling (FRR does not police receive, BIRD has no capability 76), best-path under peer limit as FRR does (Phase 6) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` or the BGP subsystem doc names the second ingress step (Phase 4) |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | Yes | the BGP telemetry doc (Phases 4 and 6) |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | payload shapes unchanged; verify in Phase 5 for the RIB command |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/release/fixes/spec-add-path-limit-send-receive.md` in each phase |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `limit N` example in `docs/guide/add-path.md`, `docs/config-reference.md`, `docs/architecture/config/syntax.md`; each edited block is loaded with `ze config validate` (`plan/journal/documentation-shows-config-the-parser-refuses.md`) |

## Implementation Steps

Each phase is sized for one agent in about 60 tool calls. Each phase carries its own doc edits (`ai/rules/documentation.md`) and lands its own commit. Scoped tests run with `./le job run label <label> command go test -tags ze_bgp <package>` and `./le test bgp <suite> <name>`.

1. **Phase 1: Config surface** - AC-1, AC-2, AC-3-receive (advertise half), AC-11 (advertise half), AC-15-receive, AC-16-receive
   - Tests: `TestParseAddPathLimitContainerReceive`, `TestParseAddPathLimitContainerPerFamilyOverride`, `TestParseAddPathLimitReceiveWithoutReceiveDirection`, `TestContainerGivenValueNamesReplacement`, `test/parse/addpath-scalar-limit-refused.ci`, the four completion tests, `test/encode/paths-limit.ci`
   - Files: `ze-bgp-conf.yang`, `config/parser.go`, `config_capabilities.go`, every in-tree scalar spelling in Current Behavior including `bgp-paths-limit-frr/ze.conf` and `paths-limit-live.ci`, the RFC-tagged `draft_abraitis_idr_addpath_paths_limit_config_test.go` fixtures (spelling only; `ze-test-judge` approval and an `RFC-approved:` commit line), docs rows 2 and 17
   - Verify: container loads and advertises what the scalar did; scalar refused with the replacement line; `./le test bgp encode paths-limit`
2. **Phase 2: Migration** - AC-12, AC-13
   - Tests: the three `TestMigrateAddPath*` units, `test/exabgp-compat/encoding/conf-paths-limit.ci`
   - Files: `internal/exabgp/migration/migrate.go`, `migrate_addpath_test.go`, `docs/features/exabgp-compatibility.md`
   - Verify: red against the scalar output first, then green
3. **Phase 3: Shared admission and receive state** - AC-9, AC-10, AC-19, AC-20 at unit level
   - Tests: `TestPathsLimitSectionSharedAdmission`, `TestReceiveCeilingAdmitsUpToLimit`, `TestReceiveCeilingAcceptsWithdrawOfRefusedPath`, `TestReceiveCeilingReadmitsAfterWithdraw`, `TestReceiveCeilingBoundsTrackedPaths`, `TestReceiveCeilingWarnsOncePerPrefix`, `TestReceiveCeilingClearsWarnedMarkerOnWithdraw`
   - Files: `session_paths_limit.go` (extract the direction-neutral type, D-9), `session_paths_limit_recv.go`, `reactor_metrics.go` (refused counter)
   - Verify: every existing `session_paths_limit` and `draft_abraitis_*` reactor test stays green unchanged
4. **Phase 4: Ingress wiring** - AC-8, AC-11 (enforce half), AC-3-receive (enforce half)
   - Tests: `TestProcessMessageRefusesPathOverReceiveCeiling`, `TestReceiveCeilingRollsBackOnPrefixLimitDrop`, `TestReceiveCeilingSeesTreatAsWithdrawExtras`, `TestReceiveCeilingEmptyResultIsNotEndOfRIB`, `TestReceiveCeilingAbsentWithoutReceiveDirection`, `test/plugin/paths-limit-receive-ceiling.ci`
   - Files: `session_read.go`, `session_negotiate.go`, `session_paths_limit_recv.go`, `capability/encoding.go` comment, docs rows 1, 6, 12, 14
   - Verify: revert the hook and see the `.ci` go red, restore green; race detector on the reactor package
5. **Phase 5: RIB limited election** - the oracle for AC-17-peer and AC-18-peer
   - Tests: `TestRecoveryLimitedAddPathElectionOrder`, `TestRecoveryUnlimitedAddPathUnchanged`, `TestRecoveryCommandParsesLimit`
   - Files: `internal/core/bgp/ribevents/recovery.go` (`limit` field, `CheckRecoveryAnswer` accepts at most `limit` survivors), `internal/component/bgp/plugins/rib/rib_recovery.go`, `relay_recovery.go` (IPC argument), `docs/architecture/plugin/rib-storage-design.md`
   - Verify: existing recovery tests unchanged and green
6. **Phase 6: Reactor best-path admission** - AC-6, AC-7, AC-17-peer, AC-18-peer
   - Tests: the eight `TestPathsLimitElection*` / `TestPathsLimitNoBoundNoElection` / `TestPathsLimitUncontestedPrefixNoElection` units, `test/plugin/paths-limit-outbound-best-path.ci`
   - Files: `session_paths_limit.go` (contested marker, generation), `session_paths_limit_elect.go`, `relay_recovery.go` staging reuse, `reactor_metrics.go` (election-failed counter), `docs/guide/add-path.md` outbound section, `docs/comparison.md`
   - Verify: revert the election trigger and see the `.ci` go red; `paths-limit-live.ci` re-run, any changed expectation judged by `ze-test-judge`
7. **Phase 7: RFC proof** - AC-21
   - Tests: tag the receive-ceiling carriers (`TestReceiveCeilingAdmitsUpToLimit`, `paths-limit-receive-ceiling.ci`) for DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7, both polarities; `./le rfc record` per carrier; re-record 3-6 carriers whose producer moved in Phase 3
   - Files: `rfc/short/draft-abraitis-idr-addpath-paths-limit.md`, `rfc/discrimination/`
   - Verify: `./le rfc check`; the main thread then spawns a separate agent (not the Phase 3, 4 or 7 author) to re-judge `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7.json`
8. **Phase 8: Interop** - AC-8 and AC-17-peer/AC-18-peer against other daemons
   - Tests: `bgp-paths-limit-receive-bird` (new), `bgp-paths-limit-frr` best-path phase
   - Files: `test/interop/scenarios/bgp-paths-limit-receive-bird/`, `internal/le/interoplab/bgp/` checker and registration, `check_paths_limit.go`, docs row 10
   - Verify: validate A-8 first; the revert, rebuild and red walk of `docs/architecture/testing/interop.md` for both scenarios; `./le test integration interop` scoped to the two scenarios is named for the main thread, not run in the agent

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every active AC has an implementation at file:line |
| Correctness | The receive ceiling counts distinct path identifiers per prefix per family |
| Correctness | No write ever leaves the peer holding more than its limit (withdraw-first) |
| Guard | A family with no splitter or prefix-key function refuses, never admits uncounted |
| Guard | An election failure is logged and counted, never silent (D-8) |
| Naming | `receive` matches the `direction` vocabulary; counters use `ze_bgp_paths_limit_*` |
| Data flow | The reactor never ranks; only the RIB elects (D-2) |
| Rule: `ai/rules/no-layering.md` | The scalar leaf is deleted; nothing reads it |
| Rule: `ai/rules/performance.md` | No allocation per UPDATE for an uncontested or under-limit message; one copy only for a rewritten one |
| Rule: `ai/rules/goroutine-lifecycle.md` | Elections run on the existing destination forward worker; no goroutine per election |
| Rule: `ai/rules/rfc-compliance.md` | 3-7 proven with a discrimination record; new functions quote the draft sentence they implement |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Scalar leaf gone | `grep -n "leaf limit" internal/component/bgp/yang/ze-bgp-conf.yang` finds none; `leaf receive` appears twice |
| No in-tree scalar config | the A-6 grep returns nothing |
| Migration tested | `internal/exabgp/migration/migrate_addpath_test.go` exists and passes |
| Ingress ceiling reachable | `paths-limit-receive-ceiling.ci` passes and fails with the ceiling reverted |
| Best-path admission reachable | `paths-limit-outbound-best-path.ci` passes and fails with the election reverted |
| 3-7 discrimination record | `./le rfc check` |
| Anchors verified | `./le spec citation anchors spec plan/release/fixes/spec-add-path-limit-send-receive.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | receive tracking at limit+1 per prefix; prefixes bounded by the prefix maximum |
| Resource exhaustion | election queue holds one entry per contested prefix per destination |
| Input validation | a path identifier needs four octets; a short NLRI is refused as `pathsLimitSection` does |
| Fail closed | a family missing a splitter refuses the message |
| Error leakage | the refusal log names peer, family, prefix and limit, never the message body |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test setup |
| Test fails on behavior mismatch | Re-read Current Behavior; if misunderstood, back to design |
| A-8 broken (BIRD does not offer several paths) | Feed BIRD from several protocols or a second BIRD peer; never weaken the assertion |
| A-9 broken (RIB cannot elect for ADD-PATH keys) | Record the storage change needed and extend Phase 5 |
| 3 fix attempts failed | STOP and report all three |

## Goal Validation

| Goal | Evidence that proves it |
|------|------------------------|
| G-1 The scalar becomes `limit { receive N; }` and the scalar is refused | `TestParseAddPathLimitContainerReceive`, `test/parse/addpath-scalar-limit-refused.ci` |
| G-2 `receive` bounds what Ze accepts | `paths-limit-receive-ceiling.ci` and interop `bgp-paths-limit-receive-bird`, each with a recorded red; the 3-7 discrimination record |
| G-4 A migrated ExaBGP config asks for what it asked before | `TestMigrateAddPathLimitBecomesReceiveContainer`, `test/exabgp-compat/encoding/conf-paths-limit.ci` |
| G-5 The OPEN is unchanged by the restructure | `test/encode/paths-limit.ci`, `bgp-paths-limit-frr` negotiation (spelling-only edit) |
| G-6 The public ledger stops over-claiming | 3-7 re-judged `enforced` by an independent agent; `docs/features.md` and `docs/guide/add-path.md` updated |
| G-7 Under a peer limit Ze sends the best paths and withdraws a demoted one | `paths-limit-outbound-best-path.ci` and interop `bgp-paths-limit-best-path-frr`, each with a recorded red |

## Design Insights

- The advertised number and the ceiling are one leaf because the capability is receiver-advertised.
- The reactor is a writer, not a decider: ranking stays in the RIB and reaches the writer through the election producer recovery already uses, so the local-send spec later only adds a second input to the bound.
- FRR polices its send side and not its receive side; BIRD has neither. Ze does both, and the receive side follows ExaBGP's bounded audit.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `limit` becomes a container (receive now, send later) | second scalar `receive-limit`; keep `limit` as shorthand | Owner decision 2026-09-11; one vocabulary; no shorthand (no-layering) |
| Ranking by the RIB's election through `ribevents` | rank in the session writer with a copied comparator; keep every candidate in a per-destination store | The writer has no candidates and no IGP cost; a copy of `SelectBest` would be a second declaration; a per-destination candidate store duplicates the RIB |
| Elections only for contested prefixes | elect on every write | cost stays zero for the common case |
| Ingress ceiling in its own file, before `checkPrefixLimits` | a mode inside `checkPrefixLimits` | different questions and journals |
| Receive interop against BIRD | FRR | FRR cannot over-send |

## Known Limitations

- An API or transaction route that bypasses the RIB is not ranked; it keeps its slot (D-7).
- Without a RIB plugin, admission under a peer limit stays arrival order, loudly (D-8).
- Received-path selection under the ingress ceiling is arrival order by owner decision (O-1, R-1).

## RFC Documentation (Scope: protocol)

- Above the receive admission function: draft Section 3, "An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender."
- Above the election trigger and above the withdraw-first write: draft Section 3, "A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability." and Section 1, "This indication allows the sender to optimize the transmission of BGP routes by selectively relaying pertinent routes instead of the entire set."
- Above the O-1 drop: why no NOTIFICATION, citing RFC 4271 Section 6.3 and RFC 4486 Section 4 as quoted in Q-1.
- Callers name `draft-abraitis-idr-addpath-paths-limit Section 3` at the call.

## Disposition

| Field | Value |
|-------|-------|
| Date | 2026-10-27 |
| Checked at | 0723d7d26a |
| Verdict | REWORK (reworked in place the same day; Status set to ready) |
| Targets | every Files to Modify path exists at HEAD; `initPathsLimit`, `pathsLimitSection`, `filterPathsLimit` in `session_paths_limit.go`; `checkPrefixLimits`, `collectPrefixSections` in `session_prefix.go`; `parseAddPathFromTree` in `config_capabilities.go` |
| Not solved | YANG still has two scalar `leaf limit` (around lines 1111, 1135); no reader of `PathsLimitRecv` drops a path (callers: `context.go`, `format/json.go`, `format/decode.go`, `reactor/negotiated.go`, `forward_context.go`); `pathsLimitSection` still admits in arrival order |
| Ground shifted | the 2026-10-22 split moved the local `send` cap to `plan/post-release/features/spec-add-path-local-send-cap.md`; ranking lives only in the RIB (`plugins/rib/bestpath.go`) and is reachable from the reactor only through `ribevents` recovery (`relay_recovery.go::lookupRecovery`) |
| Stale references | the 3-7 audit is `rfc/audit/draft-abraitis-idr-addpath-paths-limit/DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7.json`, not a single JSON file; the scalar refusal test belongs under `test/parse/` |

The goal still holds and none of it has landed. The pre-split design put best-path ranking inside `pathsLimitSection`, which runs on the destination writer with no candidate set and no comparator, so that design could not be built in today's tree. The rework asks the RIB for election order through the producer source-DOWN recovery already uses, only for prefixes that overflow the peer's limit. The receive half and the migration needed no redesign beyond sharing the send side's admission type and ordering the ceiling before the prefix count. FRR cannot over-send, so the receive interop scenario moved to BIRD 3.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] Every active AC demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped evidence green: `./le test bgp plugin paths-limit-receive-ceiling`, `./le test bgp plugin paths-limit-outbound-best-path`, `./le test bgp encode paths-limit`, the reactor, rib, config and migration package tests, then `./le verify changed` per commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Architectural Verification table holds at closure
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append the closure template and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written under `plan/learned/`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** remove the spec file only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- Phase 1 (3b5941cd0a): YANG `limit { receive N; }` at add-path and family level, scalar leaves deleted; `parseContainer` refuses a value on a non-presence container with `write: limit { receive VALUE; }` (`Parser.containerGivenValue`); `addPathReceiveLimit` in `config_capabilities.go`; every in-tree scalar respelled.
- Phase 2 (d1972ffb5c): `migrateAddPathToUnified` writes `limit { receive N; }`; `serializeAddPathFamilies` writes family sub-containers.
- Phase 3 (9b7673180c): direction-neutral `pathsLimitAdmission` (`filter`, `section`, `admitPath`, journal kinds), receive refused set with headroom 1 and warn-once, `ze_bgp_paths_limit_refused_total`.
- Phase 4 (c7d2faf055): `receiveCeiling`, `settleReceiveCeiling`, `receiveCeilingWithdrawals` wired in `processValidatedMessage` before `checkPrefixLimits` and in both treat-as-withdraw extra loops; `initPathsLimitRecv` in `negotiateWith`.
- Phase 5 (d5f95d5ab2): `RecoveryRequest.Limit`, `AppendRecoveryArgs`/`ParseRecoveryArgs`, `CheckRecoveryAnswer` refuses survivors over the limit; `recoverySurvivorsLocked` stops at the limit.
- Phase 6 (5a65ff5ee7): contested marker (`contestPath`), `adjOutWrite.owePathsLimitElections`, `recoveryWorker.decide` limited lookup with Limit-0 fallback, `queuePathsLimitElection`, `planPathsLimitElection`, `pathsLimitWithdrawFirst`, demotion withdrawal in `adjOutWrite.allow`, `ze_bgp_paths_limit_election_failed_total`, `pathsLimitEnforceable` (D-14).
- Phase 7 (b86d879434, 58f3cc856e): 3-6 records re-taken; 3-7 tagged on the receive ceiling units and `.ci`, recorded both polarities, re-judged `enforced`; recovery fallback and contested-clear tests.
- Phase 8 (44533b6b89, 787bebf89f, 4a6feae298, 7b7b078cf6, e31f5d0836, fc6a4a554a, ad773c205b, ca0caf1f40, f7309ee807): interop scenarios `bgp-paths-limit-receive-bird` and `bgp-paths-limit-best-path-frr` with checkers, green and red recorded; `interoplab.Wait` keeps the last completed probe's failure.

### Bugs Found/Fixed
- Self-deadlock in the first election build: the writer re-locked the adj-RIB-Out table mutex inside `beginAdjOut`..`release` (D-15). Covered by `test/plugin/paths-limit-outbound-best-path.ci` and `paths-limit-live.ci`, which hung before.
- A family with no NLRI splitter made `section` fail every UPDATE of that family on both directions (D-14). Covered by `TestPathsLimitUnsplittableFamilyIsNamedNotEnforced`, `TestParseAddPathLimitRefusesUnenforceableFamily`.
- BIRD stripped MED on eBGP export in the best-path scenario, and `interoplab.Wait` reported only the cut-short last probe. Fixed in ad773c205b and fc6a4a554a; `TestWaitCancellationPreservesEvidence` covers the harness.

### Documentation Updates
- `docs/guide/add-path.md` (config, Inbound Enforcement, Best paths under the peer's limit, unenforceable family), `docs/config-reference.md`, `docs/architecture/config/syntax.md`, `docs/guide/configuration.md`, `docs/features/exabgp-compatibility.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/update-building.md`, `docs/architecture/behavior/fsm-established.md`, `docs/architecture/buffer-architecture.md`, `docs/plugin-development/metrics.md`, `docs/architecture/testing/interop.md` rows 56 and 57, `ai/digests/bgp-reactor.md`, YANG descriptions, each with its `<!-- source: -->` anchor.
- Closure: `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` Support coverage, Enforcement, Receive and Config lines (stated the knob does not police and used the scalar spelling); `features/paths-limit.md` Description, Real-path tests and Interop (stated first admitted paths keep their slots); `capability/encoding.go` `PathsLimitRecv` comment.
- `./le rfc update` regenerated the derived ledger; `./le spec citation anchors` lists no stale anchor for this spec's files.

### Deviations from Plan
- Interop: the planned best-path phase of `bgp-paths-limit-frr` became the separate scenario `bgp-paths-limit-best-path-frr`, because that scenario's routes are API-originated and D-7 pins them; `check_paths_limit.go` is unchanged.
- R-2: the rewritten receive body is a heap copy, not a pooled read buffer (D-16).
- Security checklist "a family missing a splitter refuses the message": receive refuses the config at load instead, and send warns and does not enforce (D-14).
- `docs/comparison.md` unchanged: the Paths-Limit row already reads Yes for Ze and FRR, and FRR ranks paths under the limit only in its `addpath-tx-best-selected` mode (`subgrp_announce_addpath_best_selected`, `bgpd/bgp_updgrp_adv.c`), so a "best paths like FRR" note would misstate FRR.
- Phase 6's change to `TestPathsLimitSectionSharedAdmission` (journal counts 2 to 3 and 4, plus the `pathsLimitContested` entry assertion) was refused once by `ze-test-judge`, strengthened, then approved; the commit gate did not classify it as a weakening, so 5a65ff5ee7 carries no `Weakening-approved:` line.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5: FRR could be made to over-send for the receive interop test | FRR caps TX at the received limit (`subgrp_announce_addpath_best_selected`) | read FRR master during the rework | receive scenario moved to BIRD 3 (D-13) |
| assumption | A-9 was unvalidated: the RIB might need storage for limited ADD-PATH elections | confirmed: `gatherPrefixCandidatesLocked` returns every path with its id | Phase 5 first step | none needed |
| approach | The pre-split design ranked inside `pathsLimitSection`, which has no candidates and no comparator | ranking lives only in bgp-rib | ze-disposition REWORK at 0723d7d26a | design reworked to ask the RIB (D-2) |
| approach | The first election owe re-locked the table mutex inside the writer's window | `beginAdjOut` holds it until `release` | the outbound `.ci` hung; bisected with debug lines | D-15; MUST stated on both sides (closure added `beginAdjOut`'s) |
| escalation | Phase docs left `rfc/short/` and `features/paths-limit.md` claiming the receive knob does not police and that first arrivals keep their slots | both changed in Phases 4 and 6 | closure documentation review | repaired in commit A; the RFC summary's Meta and prose lines are part of the phase that changes the behavior |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Receive-limit enforcement | Done | `session_paths_limit_recv.go::receiveCeiling`, called from `session_read.go::processValidatedMessage` | |
| Best-path selection and demotion withdrawal under a peer limit | Done | `session_paths_limit_elect.go::queuePathsLimitElection`, `recovery_owed.go::decide` | |
| Scalar-to-container migration | Done | `internal/exabgp/migration/migrate.go::migrateAddPathToUnified` | |
| Common selection machinery reusable by the local send cap | Done | `pathsLimitBound`, `RecoveryRequest.Limit` | the local cap is a second input to the bound |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestParseAddPathLimitContainerReceive`, `test/encode/paths-limit.ci` | OPEN bytes unchanged |
| AC-2 | Done | `TestContainerGivenValueNamesReplacement`, `test/parse/addpath-scalar-limit-refused.ci` | |
| AC-3-receive | Done | `TestParseAddPathLimitContainerPerFamilyOverride`, `TestReceiveCeilingAbsentWithoutReceiveDirection` | advertise and enforce halves |
| AC-6 | Done | `TestPathsLimitUncontestedPrefixNoElection`, `paths-limit-outbound-best-path.ci`, `bgp-paths-limit-best-path-frr` | |
| AC-7 | Done | `TestPathsLimitNoBoundNoElection`, `paths-limit-live.ci` | |
| AC-8 | Done | `TestProcessMessageRefusesPathOverReceiveCeiling`, `paths-limit-receive-ceiling.ci`, `bgp-paths-limit-receive-bird` | |
| AC-9 | Done | `TestReceiveCeilingAcceptsWithdrawOfRefusedPath`, `paths-limit-receive-ceiling.ci` | |
| AC-10 | Done | `TestReceiveCeilingReadmitsAfterWithdraw`, `paths-limit-receive-ceiling.ci` | |
| AC-11 | Done | `TestParseAddPathLimitReceiveWithoutReceiveDirection`, `TestReceiveCeilingAbsentWithoutReceiveDirection` | |
| AC-12 | Done | `TestMigrateAddPathLimitBecomesReceiveContainer`, `conf-paths-limit.ci` | |
| AC-13 | Done | `TestMigrateAddPathWithoutLimitEmitsNoContainer` | |
| AC-15-receive | Done | `test/ui/cli-completion-capability-addpath.ci`, `cli-completion-addpath-fields.ci` | `.et` unchanged: it carried no scalar limit |
| AC-16-receive | Done | `addpath-scalar-limit-refused.ci` (0 and 65536 refused, 1 and 65535 load), `TestParseAddPathLimitRefusesInvalidReceive` | |
| AC-17-peer | Done | `TestPathsLimitElectionAdmitsBestOrder`, `TestRecoveryLimitedAddPathElectionOrder`, `paths-limit-outbound-best-path.ci`, `bgp-paths-limit-best-path-frr` | |
| AC-18-peer | Done | `TestPathsLimitElectionWithdrawsDemotedPath`, `TestPathsLimitElectionReusesMintedIdentifier`, `bgp-paths-limit-best-path-frr` step 8 | |
| AC-19 | Done | `TestReceiveCeilingBoundsTrackedPaths`, `TestReceiveCeilingWarnsOncePerPrefix` | |
| AC-20 | Done | `TestReceiveCeilingClearsWarnedMarkerOnWithdraw` | |
| AC-21 | Done | `rfc/discrimination/.../DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7.json` (6 records), `rfc/audit/.../DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7.json` verdict `enforced` | judged by an agent other than the tests' author (58f3cc856e) |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| all 30 units of the TDD table | Done | files named in the table; `TestReceiveCeilingAdmitsUpToLimit` and `TestProcessMessageRefusesPathOverReceiveCeiling` moved verbatim to `draft_abraitis_idr_addpath_paths_limit_receive_test.go` | each found by `grep -rln "func <name>("` at closure |
| added: `TestReceiveCeilingSeesTypedWithdrawExtras`, `TestPathsLimitUnsplittableFamilyIsNamedNotEnforced`, `TestParseAddPathLimitRefusesUnenforceableFamily`, `TestParseAddPathLimitRefusesInvalidReceive`, `TestPathsLimitRecoveryFallsBackOnLookupError`, `TestPathsLimitRecoveryClearsContestedUnderBound`, `TestRecoveryArgsRoundTrip`, `TestCheckRecoveryAnswerRefusesSurvivorsOverLimit`, `TestMigrateAddPathLimitLoadsInZe` | Done | reactor, ribevents, migration | |
| functional: `paths-limit-receive-ceiling`, `paths-limit-outbound-best-path`, `addpath-scalar-limit-refused`, edits of `paths-limit-live`, `conf-paths-limit`, `paths-limit` encode, completion tests | Done | `test/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| every Files to Modify and Files to Create path | Done | `internal/core/bgp/capability/encoding.go` comment written at closure |
| `internal/le/interoplab/bgp/check_paths_limit.go` | Changed | unchanged by design; best-path proof in a new scenario |

### Audit Summary
- **Total items:** 19 ACs, 39 units, 7 functional tests, 2 interop scenarios
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** 4 (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| G-1 scalar becomes `limit { receive N; }` and the scalar is refused | functional | `test/parse/addpath-scalar-limit-refused.ci` |
| G-2 `receive` bounds what Ze accepts | functional + interop | `paths-limit-receive-ceiling.ci` (red with the ceiling call removed, Phase 4); `bgp-paths-limit-receive-bird` green ("interop: 1 passed") and red with the ceiling bypassed ("ze holds 4 paths of 10.70.0.0/24, want the advertised ceiling 2") |
| G-4 a migrated ExaBGP config asks for what it asked before | functional | `conf-paths-limit.ci` OPEN carries code 76 limit 10; red when the migration wrote no limit |
| G-5 the OPEN is unchanged by the restructure | functional | `test/encode/paths-limit.ci` OPEN bytes unchanged |
| G-6 the public ledger stops over-claiming | RFC record | 3-7 `enforced` with six observed-red discrimination records; `rfc/short` Support coverage and `features/paths-limit.md` corrected at closure |
| G-7 under a peer limit Ze sends the best paths and withdraws a demoted one | functional + interop | `paths-limit-outbound-best-path.ci` (red with the owe disabled and with the withdrawal loop emptied); `bgp-paths-limit-best-path-frr` green twice, red with `owePathsLimitElections` a no-op (FRR kept MED 300 and 200 after MED 100) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Local `send` cap and its surfaces (original AC-3/4/5/14 to 18 send halves) | split out on 2026-10-22 | `plan/post-release/features/spec-add-path-local-send-cap.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | written by `./le spec review record` in this session's state directory; the path is in commit A's body |
| `./le spec review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | AC-to-test mapping, wiring from config and wire entry points, receive hot path and buffer ownership, election lock order, bounded state, docs against code, RFC proof records, security checklist |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | RFC summary Support coverage, Enforcement and Config lines said the receive knob does not police and used the scalar spelling | `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` | rewritten against `receiveCeiling`, `queuePathsLimitElection`, `pathsLimitEnforceable` |
| 2 | ISSUE | Feature declaration said first admitted paths keep their slots, and listed neither new `.ci` nor new scenario | `features/paths-limit.md` | Description, Real-path tests, Interop updated |
| 3 | ISSUE | `beginAdjOut` did not state that its table mutex is held until `release` and MUST NOT be taken in that window (the D-15 contract had a MUST on one side only) | `session_ownership.go::beginAdjOut` | doc paragraph added |
| 4 | ISSUE | The receive drop carried no comment on why no NOTIFICATION, which the spec's RFC Documentation section required | `session_paths_limit.go::refusePath` | RFC 4271 Section 6.3 and RFC 4486 Section 4 quoted byte-exact |
| 5 | ISSUE | `PathsLimitRecv` comment did not say Ze enforces it (Files to Modify) | `internal/core/bgp/capability/encoding.go` | comment extended |
| 6 | ISSUE | Audit verdicts RFC7911-2-1, RFC7911-2-2 (`allow`, `writeUpdateBody`) and RFC8654-5-1 (`negotiateWith`) staled by 9b7673180c, 5a65ff5ee7 and c7d2faf055 | `rfc/audit/rfc7911/`, `rfc/audit/rfc8654/` | re-judged `enforced` with `audit-stamp mode rejudge`; RFC6286-2.1-1 (`config.go::parsePeerSettings`) left alone: `config.go` is unchanged since 68adaa90cf |

NOTEs (do not block): `planPathsLimitElection` reduces the slots for a graceful-restart-retained path and also keeps it if the RIB lists it, which can leave a slot unfilled until the next election (never over the limit); `settled` holds only when the RIB returns fewer survivors than the limit, so a route with exactly limit candidates stays contested and costs one more election; `session_ownership.go` is over 1000 lines (1027 before this spec).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| the 13 created files (recv, elect, migrate and capability tests, three `.ci`, two scenario `ze.conf`, two checkers) | Yes | `ls` of the 13 paths at closure counts 13 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-21 | every named test exists | `grep -rln "func <name>("` found all 36 unit names at closure |
| AC-2 | scalar leaf gone | `grep -c 'leaf limit' ze-bgp-conf.yang` = 0; two `leaf receive` in the add-path and family `limit` containers (the third hit is `receive-hold-time`) |
| AC-8 | ceiling wired | `session_read.go::processValidatedMessage` calls `receiveCeiling`; `session_negotiate.go::negotiateWith` calls `initPathsLimitRecv` |
| AC-21 | 3-7 enforced | `rfc/audit/.../DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7.json` `"verdict": "enforced"`; `./le rfc check` reports no finding for the draft |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| peer UPDATE over the receive ceiling | `test/plugin/paths-limit-receive-ceiling.ci` | its two 3-7 tags assert identifiers 3 and 4 never reach the plugin |
| better path at the peer limit | `test/plugin/paths-limit-outbound-best-path.ci` | promoted from draft in Phase 6, red under both breaks |
| scalar config | `test/parse/addpath-scalar-limit-refused.ci` | Phase 1 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 to A-4, A-6, A-7 | confirmed | rows above |
| A-5 | broken | Mistake Log; D-13 |
| A-8 | confirmed | Phase 8 green and red runs |
| A-9 | confirmed | Phase 5 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| add-path guide inbound and best-path sections | `receiveCeiling`, `queuePathsLimitElection`, `pathsLimitElectionFailed` fix text | Yes |
| RFC summary and feature declaration | rewritten at closure against the same producers | Yes |
| comparison page | FRR `subgrp_announce_addpath_best_selected` read in Phase 6 | No change needed |
