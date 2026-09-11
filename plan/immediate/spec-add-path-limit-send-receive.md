# Spec: ADD-PATH limit becomes a send/receive container, with ingress enforcement

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The ADD-PATH block carries one scalar `limit N`, at the container level and again per family. One number cannot say two things, and today it says only one: it is the number Ze advertises in PATHS-LIMIT capability 76, which is a request to the peer. It bounds nothing locally. `docs/guide/add-path.md` line 100 states this plainly, and the `rfc/short/` Support coverage row repeats it: "The configuration knob states a request to the peer and does not bound what Ze accepts: a peer that ignores it is not policed."

Two things are missing behind that one word.

1. **No ingress ceiling.** The draft asks for one. Section 3: "An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender." Ze advertises a number and then accepts whatever arrives, so a peer that ignores capability 76 grows Ze's adj-RIB-in without bound. Requirement `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7` is published as `Supported` on a knob that accepts, not one that limits.
2. **No local outbound cap.** Ze's outbound path count is bounded only by what the PEER advertised (`Session.pathsLimitSection`, driven from `EncodingCaps.PathsLimitSend`). An operator who wants Ze to announce at most four paths per prefix, to a peer that advertised nothing, has no leaf to write.
3. **Outbound truncation picks arrival order, not best-path order.** `Session.pathsLimitSection` admits path identifiers in the order they arrive: a peer capped at two paths gets whichever two Ze received first, not the two Ze prefers, and a path that later becomes best is never substituted in. FRR 10.5.3 does the opposite: `subgrp_announce_addpath_best_selected` (`bgpd/bgp_updgrp_adv.c`) walks the best-path-ordered list, sends the first N, and actively WITHDRAWS the paths past N rather than merely skipping them.

The owner has decided the shape (Thomas, 2026-09-11). The scalar `limit` is REPLACED by a container carrying two leaves, `send` and `receive`, reusing the direction vocabulary the sibling `direction` leaf already uses, at both the container level and the per-family level: `add-path { direction send/receive; limit { send 4; receive 2; } family { ipv4/unicast { direction send; mode enable; limit { send 4; receive 2; } } } }`.

- `receive` is the number of paths per prefix Ze accepts FROM the peer. It is the number Ze advertises in capability 76, because the draft's capability is receiver-advertised, so the advertisement and the acceptance ceiling are one number. It is also enforced on ingress. Enforcement is the new behavior.
- `send` is a LOCAL cap on the paths per prefix Ze announces TO the peer. The effective outbound bound is the smaller of the peer's advertised limit and this leaf. Where only one exists, that one governs; where neither exists, there is no bound.
- `limit N` is deleted, not kept as a shorthand (`ai/rules/no-layering.md`). An existing `limit 10` becomes `limit { receive 10; }`.
- Outbound truncation also changes from arrival order to best-path order, for every peer with an effective outbound bound, not only peers newly configured with `send` (owner decision, 2026-09-11; see Q-1/Design Decisions and G-7). Where more qualifying paths exist than the effective bound allows, Ze sends the best-ranked paths and WITHDRAWS any already-sent path that best-path selection demotes out of the kept set, mirroring FRR's `addpath-tx-best-selected` behavior.

The release bucket is `immediate`: an operator meets all three halves as a bug on the first release. A configuration that asks for a receive ceiling does not get one, a published `Supported` row names a knob that does not police, and the adj-RIB-in carries no per-prefix bound against a peer that ignores the capability.

## Design Decisions (owner, 2026-09-11 -- resolves the open questions below)

Both open questions below are DECIDED. The reasoning is kept in full: it explains why, and the rejected alternatives still bound future work, since changing either decision needs the same case made against it.

### Q-1: what happens to a path over the receive ceiling (DECIDED: O-1, drop the excess path only)

**Decided: O-1, drop only the excess path identifiers, never the whole UPDATE.** The receive side admits paths up to the `receive` limit and discards the rest before installation. Every other prefix carried in the same UPDATE installs normally; only the path identifiers past the ceiling, for the prefix that exceeded it, are refused. The whole-message drop (O-2) is REJECTED: no implementation behaves that way, and it would discard prefixes that did nothing wrong.

Neither RFC in scope prescribes a remedy, so the standard leaves this to implementation choice. RFC 4271 Section 6.3: "All errors detected while processing the UPDATE message MUST be indicated by sending the NOTIFICATION message with the Error Code UPDATE Message Error." An over-limit path is not an error in the UPDATE: the message is well formed, and the subcodes Section 6.3 enumerates (Malformed Attribute List, Attribute Flags Error, Attribute Length Error, and the rest) name no such condition. RFC 4486 Section 4 is the nearest standardised mechanism and does not fit: it is a MUST about Cease subcode 1 when "the number of address prefixes received from the neighbor exceeds a locally configured upper bound", a PREFIX count whose optional data field carries a four-octet prefix upper bound, with no room for a per-prefix path count. RFC 7911 Section 8 names the memory exposure PATHS-LIMIT exists to bound, without prescribing a remedy. The draft's only sender-side sentence is a SHOULD ("A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability") and says nothing about the receiver's remedy.

| Option | What Ze does | Verdict |
|--------|--------------|---------|
| O-1 Rewrite and admit the rest | Mirror `filterPathsLimit` on ingress: rewrite the UPDATE body into a pooled buffer with the over-limit NLRIs removed, then deliver the rewritten body | DECIDED. Ze already carries this machinery, on the SEND side, in `Session.filterPathsLimit` (`internal/component/bgp/reactor/session_paths_limit.go`), with its own pooled buffer and rollback journal; the receive side mirrors it rather than building a new mechanism. The receive path stops being zero-copy for a message that exceeded the ceiling: `ContextID` no longer means "forward unchanged" for that message, so a forwarding consumer re-encodes it. Best-path then runs over the admitted subset, so which paths Ze prefers can depend on ARRIVAL ORDER; R-1 tracks this |
| O-2 Drop the whole message | Reuse the verdict `checkPrefixLimits` already returns: `drop=true`, the UPDATE reaches no plugin, the session survives, the HoldTimer restarts per RFC 4271 Section 8.2.2 Event 27 | REJECTED. One over-limit path would discard every other prefix batched into the same UPDATE, and no other implementation behaves that way |
| O-3 Count, warn, do not drop | What ExaBGP does today: `UpdateHandler._audit_announce` (`src/exabgp/reactor/peer/handlers/update.py`) counts per prefix through `IncomingRIB.track_path` (`src/exabgp/rib/incoming.py`) and logs `rib.paths_limit.peer_violation` once per prefix, gated by env `bgp.paths_limit_audit`. The set is bounded at `limit + 1` so the misbehaving peer cannot decide the audit's memory cost | Not chosen as the enforcement mechanism, but its AUDIT half is adopted alongside O-1: see "Warn once, bound the tracking set" below |
| O-4 Terminate the session | NOTIFICATION Cease subcode 1, reusing the `prefixTeardownCause` plumbing | REJECTED. No RFC authorizes it for a path count, and the draft makes the sender obligation a SHOULD; terminating over a SHOULD violation is harsher than the standard contemplates |

A later WITHDRAW for a path Ze never kept is accepted and frees no slot. The send side already answers this: `pathsLimitSection` states "Withdrawals always pass, including unknown identifiers; only a NEW path consumes a slot", and the ingress mirror inherits that rule.

**Warn once, bound the tracking set (owner decision, 2026-09-11).** Ze adopts ExaBGP's audit discipline alongside the O-1 drop. `IncomingRIB.track_path` (`src/exabgp/rib/incoming.py`) caps its per-prefix tracking set at `limit + AUDIT_PATHS_HEADROOM`, where the headroom is 1, so a peer ignoring the limit cannot decide how much memory the audit costs; Ze's ingress ceiling bounds its own per-prefix path-id set the same way (R-3). `_audit_announce` (`src/exabgp/reactor/peer/handlers/update.py`) logs once per prefix through `mark_warned`; Ze's ingress ceiling logs a refusal once per prefix per family, not once per refused path, so a peer sending many excess paths for one prefix produces one log line. When a WITHDRAW arrives for a prefix whose tracking set had saturated, the withdrawn identifier is removed from the set (freeing a slot, AC-10) and the warned marker clears, so a later excess on that same prefix warns again rather than staying silently suppressed.

### Q-2: the ExaBGP send-side spelling (DECIDED: no per-direction form exists)

**Decided: ExaBGP carries no per-direction ADD-PATH limit.** The owner confirms there is no `send`/`receive` split anywhere in ExaBGP; the earlier report of one landing this week was wrong, and refusing to invent a spelling for it was correct. The single `ipv4 unicast limit 10` grammar is the RECEIVE direction: confirmed in the local checkout at `/Users/thomas/Code/github.com/exa-networks/exabgp/main`, where `src/exabgp/configuration/neighbor/__init__.py` sets `paths_limit_per_family` from that same parsed value (the upstream read on 2026-09-11, at commit `c2ce4d20ddff78bdbbe7ffd50276bdbb47646a3e` and branches `pr-1393`, `5.0`, `master` and `4.2`, found the identical grammar and reached the same conclusion; see `ParseAddPath._parse_addpath_family` in `src/exabgp/configuration/neighbor/family.py`). The migration is settled: `add-path { ipv4 unicast limit 10; }` becomes `limit { receive 10; }`, and the `send` half has no source in an ExaBGP config, so a migrated config carries no `send` leaf.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/add-path.md` - the page documenting the config surface being replaced, and the only page describing PATHS-LIMIT behavior for an operator
  → Constraint: the page's Local limit table (lines 98-111) states the limit is a receiver request that does not set Ze's outbound limit. Both halves of that sentence stop being true, so the page changes in the same work as the code, not at closure
  → Decision: the page carries a source anchor on `config_capabilities.go -- parseAddPathFromTree` at lines 50 and 109, and on `session_paths_limit.go` at line 133. Both files change, so both anchors are re-verified
- [ ] `docs/architecture/wire/capabilities.md` - the anchor target for `session_paths_limit.go`, per `ai/CODE-TO-DOCS.md`
  → Constraint: capability 76's wire shape does not change. Only the meaning of the number Ze puts in it gains a second job
- [ ] `docs/architecture/testing/interop.md` - the discrimination walk and the four vacuity traps
  → Constraint: a tagged test added to already-working code has no red phase, so a receive-side ceiling test is observed RED through `./le rfc discriminate-record`, which refuses a red it did not see
- [ ] `ai/patterns/config-option.md` - the structural template for a YANG leaf
  → Constraint: every leaf takes maximum native validation. Both new leaves keep `type uint16 { range "1..65535"; }`, which is what the scalar carried, because a zero is an absent container rather than an operator asking for none
- [ ] `docs/features/exabgp-compatibility.md` - the page anchored on `migrate.go`
  → Constraint: a migration that silently changes what a config asks the peer for is the failure mode. The page states what the bridge converts

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` - the enrolled summary, seven requirements, published `Supported`
  → Constraint: requirement 3-7, "[SHOULD] An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender", is the one this spec implements. Its `Support coverage` row currently admits Ze does not police, so the row and the coverage sentence both change
  → Constraint: 3-6 is the SENDER SHOULD. The new `send` leaf is local policy stricter than 3-6 asks for, so it cannot make 3-6 false, but a test asserting 3-6 must keep passing when no local `send` is configured
- [ ] `rfc/full/rfc4486.txt` Section 4 - the only Cease subcode near this condition
  → Constraint: the subcode is bound to a PREFIX upper bound, not a per-prefix path count. Q-1 option O-4 cannot cite it as authority
- [ ] `rfc/full/rfc4271.txt` Section 6.3 - UPDATE error handling
  → Constraint: the enumerated subcodes name no over-limit-path condition, so a well-formed UPDATE carrying an excess path is not an UPDATE Message Error

**Key insights:** (minimal context to resume after compaction)
- The ingress admission point exists and there is exactly one: `Session.checkPrefixLimits` (`internal/component/bgp/reactor/session_prefix.go`), called from `Session.processMessage` (`internal/component/bgp/reactor/session_read.go`) BEFORE the plugin delivery callback at `session_read.go`. Its granularity is per-family COUNT with a whole-message verdict. No per-prefix, per-path-identifier state exists anywhere on the receive path.
- `collectPrefixSections` (`session_prefix.go`) already splits one UPDATE into up to four family sections and already resolves each section's ADD-PATH receive state. The NLRI bytes and the path identifiers are in hand at the exact point a per-prefix path count would be taken.
- `Session.pathsLimitSection` (`session_paths_limit.go`) is the working model for per-prefix, per-path-id admission with a rollback journal. The receive side is its mirror.
- ExaBGP chose audit-only on ingress, not a drop; Ze adopts the audit (warn once per prefix, bounded tracking set) alongside a drop, not instead of one (owner decision, 2026-09-11).
- Outbound truncation changes from arrival order to best-path order, with withdraw-the-loser, for every already-capped peer, not only new `send` configurations (owner decision, 2026-09-11; G-7).

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/session_paths_limit.go` - outbound enforcement. `pathsLimitFamily` holds a per-family limit plus a `prefixes` map of prefix key to path-id set; `initPathsLimit` builds it from `encoding.PathsLimitSend` alone, and only for families whose negotiated ADD-PATH mode carries `AddPathSend`; `pathsLimitSection` admits or drops one NLRI section; `filterPathsLimit` rewrites a whole UPDATE body; `pathsLimitChanges` is the rollback journal; `recordAnnounced` keeps the API duplicate cache from remembering a withheld path
- [ ] `internal/component/bgp/reactor/session_read.go` - `processMessage`: family validation, then `checkPrefixLimits` for UPDATEs, then ROUTE-REFRESH shape validation, then `onMessageReceived`. The prefix-limit branch has two outcomes, a NOTIFICATION teardown and a silent whole-message drop that keeps the session and restarts the HoldTimer
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `prefixSection`, `collectPrefixSections`, `checkPrefixLimits`, the installed-family journal and its rollback. Withdrawals are collected before announcements so one UPDATE replacing a prefix never reads as an overflow
- [ ] `internal/component/bgp/reactor/session_handlers.go` - `handleUpdate` validates families and advances the FSM. It installs nothing, so it is not an admission point
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - `installStructuredNLRIs` iterates the NLRI bytes, builds a route key from family, prefix and path id, and stores or queues each one. It applies no count and no ceiling: it installs what the event stream hands it
- [ ] `internal/component/bgp/reactor/config_capabilities.go` - `parseAddPathFromTree`: reads container `direction` and scalar `limit`, then per-family `direction`, `mode` and `limit`; builds `capability.AddPath`, then derives the PATHS-LIMIT tuples from the SAME `addPath.Families` slice that reaches the wire
- [ ] `internal/core/bgp/capability/negotiated.go` - `negotiatePathsLimit` and `negotiatePathsLimitDirection`: `pathsLimitSend` takes the REMOTE capability filtered by negotiated `AddPathSend`, `pathsLimitRecv` takes the LOCAL capability filtered by negotiated `AddPathReceive`. First tuple wins, zero is skipped
- [ ] `internal/core/bgp/capability/encoding.go` - `PathsLimitSend` is "the remote peer's advertised limits (constrains our send)", `PathsLimitRecv` is "our advertised limits (constrains peer's send)". The second comment describes a request, not an enforcement
- [ ] `internal/core/bgp/context/context.go` - `EncodingContext.pathsLimit` is derived from both maps by direction and feeds the context hash
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - the two `limit` leaves, both `uint16 { range "1..65535"; }`, both documented as a request that "does not enforce a local receive-side limit"
- [ ] `internal/exabgp/migration/migrate.go` - `migrateAddPathToUnified` and `parseLimit`: the ExaBGP freeform key `ipv4 unicast limit 10` is split on whitespace, the first two fields become the family key, and any remaining numeric field sets a single per-family `limit`. It emits no `mode` and no per-family `direction`
- [ ] `internal/component/bgp/plugins/cmd/peer/fields.go` - `addPathsLimitFields` emits `paths-limit` with `send` and `receive` sub-maps, from the two negotiated maps. The words already exist in the CLI and JSON; only their meaning shifts
- [ ] `internal/le/interoplab/bgp/check_paths_limit.go` - the FRR scenario. Ze advertises 10, FRR advertises 2, and seven phases assert exact (path id, MED) sets at FRR. FRR 10.3.1's `addpath-rx-paths-limit 2` is what FRR SENDS, not a local inbound filter
- [ ] `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` - the seven requirements and the `Support` metadata block
- [ ] `test/plugin/paths-limit-live.ci` - the live operator admission test. Ze's receive limit is 9 and the synthetic peer independently advertises IPv4 1 and IPv6 2. It drives a controllable peer through `ze-test fixture`, the only over-sender Ze can command today
- [ ] `test/interop/scenarios/bgp-paths-limit-frr/ze.conf` - uses `family { ipv4/unicast { limit 10; } }`
- [ ] `test/encode/paths-limit.ci` - asserts OPEN carries code 76 for ipv4/unicast limit 10, configured with the scalar
- [ ] `test/decode/bgp-paths-limit.ci` - asserts the decoded JSON for a received capability 76
- [ ] `test/ui/cli-completion-peer-with-capability.ci` - carries `limit 10` in a config block
- [ ] `test/ui/cli-completion-addpath-fields.ci` - asserts the family list shows a `limit` field
- [ ] `test/ui/cli-completion-capability-addpath.ci` - asserts the container shows `direction, limit, family`
- [ ] `test/editor/completion/bgp-peer-capability-addpath.et` - asserts completion offers `direction,limit,family`
- [ ] `test/exabgp-compat/etc/conf-paths-limit.conf` - ExaBGP-native input carrying `ipv4 unicast limit 10`. The input stays; what Ze migrates it INTO changes

**Behavior to preserve:**
- Outbound enforcement against the peer's advertised limit, exactly as `pathsLimitSection` performs it today: withdrawals always pass including unknown identifiers, replacement of an existing identifier passes at capacity, a withdrawal frees a slot, a new connection starts empty, and the route-server fast path shares the state.
- The four draft MUSTs already proven: one coalesced instance in OPEN, ignore PATHS-LIMIT without ADD-PATH, ignore a tuple whose family the ADD-PATH capability did not carry, first duplicate tuple wins.
- The `paths-limit` CLI and JSON key with its `send` and `receive` sub-maps. The words stay; their meaning gains the local cap.
- The PATHS-LIMIT tuples stay derived from `addPath.Families`, so the two sets cannot disagree.
- Every peer with no `limit` block behaves exactly as it does today.

**Behavior to change:**
- The scalar `limit` leaf is deleted at both levels and replaced by a `limit` container carrying `send` and `receive`.
- `receive` becomes an enforced ingress ceiling as well as the advertised number.
- `send` becomes a local outbound cap, combined with the peer's advertised limit by minimum.
- `docs/features.md`, `docs/guide/add-path.md` and the `rfc/short/` Support coverage each state that the knob does not police. They stop being true.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Two entry points, one per direction.

- Config: the `add-path` container under `session > capability` in a peer's config file, resolved to `map[string]any` and read by `parseAddPathFromTree`. Format at entry: nested maps. `limit` arrives as a map rather than a string once it is a container, which is the parse change.
- Wire, receive direction: a BGP UPDATE arriving on the session read goroutine, as raw bytes wrapped in a `wireu.WireUpdate`. Format at entry: wire octets, lazily parsed, with ADD-PATH path identifiers prefixed to each NLRI when the family negotiated receive.

### Transformation Path
1. `parseAddPathFromTree` reads container `limit { send; receive; }` and each family's `limit { send; receive; }`, resolving each family's effective pair by inheritance from the container.
2. The `receive` half builds `capability.PathsLimitEntry` tuples exactly as the scalar does today, derived from `addPath.Families`, and appends them to `ps.Capabilities`. Unchanged on the wire.
3. The `send` half is carried on `PeerSettings` as a new per-family map. It never reaches the wire: it is local policy.
4. `Negotiate` stores the peer's tuples in `pathsLimitSend` and Ze's own in `pathsLimitRecv`, both filtered by the matching negotiated ADD-PATH direction.
5. Outbound: `initPathsLimit` takes the minimum of `encoding.PathsLimitSend[fam]` and the configured local send cap for that family, per family, and stores it as `pathsLimitFamily.limit`. Where only one exists, that one is the limit. `pathsLimitSection` changes its admission order from ARRIVAL order to BEST-PATH order (owner decision, 2026-09-11): it admits the best-ranked `limit` path identifiers for a prefix and WITHDRAWS any already-admitted identifier that best-path selection demotes out of that set, mirroring FRR's `subgrp_announce_addpath_best_selected` (`bgpd/bgp_updgrp_adv.c`).
6. Inbound: `processMessage` calls the new per-prefix path check for families whose `pathsLimitRecv` is nonzero, in the same place and with the same verdict shape `checkPrefixLimits` already returns. The check drops the excess path identifiers only (Q-1: O-1), rewriting the UPDATE body into a pooled buffer with a rollback journal, mirroring `filterPathsLimit`; every other prefix in the message installs normally.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree to reactor | `map[string]any` under `session > capability > add-path`, read by `parseAddPathFromTree` | No |
| Reactor to wire | `capability.PathsLimit` entries encoded into OPEN; the `send` cap crosses nothing | No |
| Session to plugin | `onMessageReceived` carries body, `wireUpdate`, `ContextID` and buffer. The ingress verdict is taken BEFORE this boundary, so a refused UPDATE crosses nothing | No |
| Reactor to adj-RIB-in plugin | JSON route events; `installStructuredNLRIs` stores what it is given and applies no ceiling | No |
| ExaBGP config to Ze config | `migrateAddPathToUnified` writes the `limit` container into the destination tree | No |
| Reactor to CLI and JSON | `NegotiatedPathsLimitSend` and `NegotiatedPathsLimitReceive`, rendered by `addPathsLimitFields` | No |

### Integration Points
- `Session.checkPrefixLimits` (`session_prefix.go`) - the sibling ingress ceiling, same call site, same verdict shape. The new check runs beside it, not inside it: prefix count and path count answer different questions, and an installed-family rollback must not be entangled with a path-id journal.
- `Session.pathsLimitSection` (`session_paths_limit.go`) - the working per-prefix, per-path-id admission model. The receive side mirrors it (Q-1: O-1, decided): the same pooled-buffer rewrite and rollback journal shape, applied on ingress instead of egress.
- `nlrisplit.Get`, `nlrisplit.GetWithdraw` and `nlrisplit.GetPrefixKey` - the family-indexed splitters and prefix-key functions the send side already uses. The receive side needs the same three for the same families.
- `applyCapMode` and `ps.RequiredAddPathFamilies` - unchanged. The limit container does not participate in capability modes.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `Session.checkPrefixLimits` at `session_prefix.go`, reached from `session_read.go`, is the only ingress admission point before plugin delivery | Read `processMessage` (`session_read.go`) and `installStructuredNLRIs` (`adj_rib_in/rib.go`); the latter applies no ceiling | A second point exists and the check lands in the wrong place, or lands twice | grep every caller of `onMessageReceived` and every writer into `AdjRIBInManager.ribIn` | unvalidated |
| A-2 | The `receive` number Ze advertises and the ceiling Ze enforces are one number, so no second leaf is needed | Draft Section 3: the Paths Limit field "indicates the maximum paths limit the receiver wants to receive from its peer"; owner decision 2026-09-11 | An operator wants to advertise one number and enforce a different one, which the shape forbids | owner confirmation. The shape is his decision and is not reopened here | unvalidated |
| A-3 | No ExaBGP release carries a per-direction ADD-PATH limit as of 2026-09-11 | Read `ParseAddPath._parse_addpath_family` in `src/exabgp/configuration/neighbor/family.py` at `main` commit `c2ce4d2` (2026-09-09) and on branches `pr-1393`, `5.0`, `master`, `4.2`; no `send` or `receive` token appears. Owner CONFIRMED 2026-09-11: no per-direction form exists anywhere in ExaBGP | The migration maps the wrong half and silently changes what a migrated config asks peers for | Q-2: owner confirmation received. No further validation needed | confirmed |
| A-4 | ExaBGP's single per-family `limit` is the RECEIVE direction | It populates `NeighborCapability.paths_limit_per_family` (`src/exabgp/bgp/neighbor/capability.py:81`), is read at `neighbor.py:484` to build the OPEN, and is audited against `negotiated.advertised_paths_limit` on ingress (`reactor/peer/handlers/update.py:40`). Owner CONFIRMED 2026-09-11 against the local checkout at `/Users/thomas/Code/github.com/exa-networks/exabgp/main`, `src/exabgp/configuration/neighbor/__init__.py` | Every migrated ExaBGP config gets the wrong direction | a migration unit test asserting `limit { receive 10; }` from `ipv4 unicast limit 10`, plus the `test/exabgp-compat` fixture | confirmed |
| A-5 | FRR 10.3.1 can be made to over-send, so a receive-side interop scenario is not vacuous | Unverified. The existing checker's comment establishes only that FRR's `addpath-rx-paths-limit` is what FRR sends, and says nothing about whether FRR caps its tx at a received limit | The interop scenario can never observe a red, and only the `.ci` synthetic peer can prove the ceiling | read FRR 10.3.1 `bgpd/bgp_addpath.c` and `bgpd/bgp_updgrp_adv.c` for a consumer of the received paths-limit, before writing the scenario | unvalidated |
| A-6 | A YANG leaf becoming a container is a breaking config change with no in-tree consumer outside the files named in Files to Modify | `grep -rln limit` over `test/` and `etc/` intersected with `add-path`, listed in Current Behavior | An operator config or a doc example breaks silently | `./le verify worktree` plus a grep of `docs/` for `add-path` config examples | unvalidated |
| A-7 | The outbound minimum can be taken once, at `initPathsLimit`, because both inputs are fixed for the life of a connection | `initPathsLimit` runs once during OPEN negotiation with `writeMu` held, and config is not reloaded into a live session's encoding caps | A config reload changes the local cap mid-session and the session keeps the old one | read the config-reload path for a peer whose capability block changed | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The ingress ceiling changes which paths best-path selection sees, so route selection becomes arrival-order dependent | An interop or `.ci` scenario where the preferred path arrives last and is refused | O-1 is decided (owner, 2026-09-11), so this cannot be avoided by choosing O-2. State the ingress admission order (first `receive` identifiers to arrive) in `docs/guide/add-path.md` and prove it with a test delivering the operator's preferred path last |
| R-2 | A rewritten ingress body invalidates `ContextID`, silently defeating the forward-unchanged fast path | Route-server forwarding benchmarks regress; `forward_context.go` consumers re-encode | O-1 is decided (owner, 2026-09-11). Measure the ContextID/forwarding-fast-path regression during implementation, since a rewritten body no longer qualifies for the forward-unchanged fast path |
| R-3 | Per-prefix path-id state on ingress is unbounded memory owned by the peer, the very thing the ceiling exists to prevent | A peer sending many prefixes with one path each grows the map | Bound the tracked set as ExaBGP does, at `limit + 1` per prefix (`IncomingRIB.AUDIT_PATHS_HEADROOM`), and release on withdrawal |
| R-4 | A WITHDRAW arrives for a path the ceiling refused, and the receive state double-frees or errors | A functional test where the peer withdraws a refused identifier | Inherit the send side's rule: withdrawals always pass, unknown identifiers included, and free only a slot that was taken |
| R-5 | The migration silently changes what a migrated config asks peers for, invisible until a peer behaves differently | No test exists on this path today, so the signal is a customer report | Q-2 is answered (RECEIVE direction, no send-side source; owner, 2026-09-11). The migration unit test and the `test/exabgp-compat` assertion still land before the parser change |
| R-9 | The outbound admission-order switch to best-path is a routing-visible behavior change for every peer that already has an effective outbound bound, not only newly configured `send` peers | An interop or `.ci` scenario where announced paths change after the upgrade with no config change | Document the reorder in `docs/guide/add-path.md` and `docs/comparison.md`; prove withdraw-the-loser and re-admission with `TestPathsLimitSectionWithdrawsDemotedPath` and `paths-limit-outbound-best-path.ci` (AC-17, AC-18) |
| R-6 | The `send` cap and the peer's advertised limit disagree and the minimum is taken in the wrong place, so Ze under-announces to a peer that asked for more | A peer's received path count drops after an upgrade with no config change | The minimum is taken once in `initPathsLimit`, and a unit test covers all four combinations of present and absent |
| R-7 | Deleting the scalar leaf breaks every existing operator config with no migration and no error message | A config that loaded yesterday is refused today | The YANG validator names the old spelling in its error and states the replacement, and `docs/guide/add-path.md` carries the before and after |
| R-8 | The published `Supported` row over-claims 3-7 until the ceiling ships and is proven | Already true today; the `rfc/short/` coverage admits it in prose | The repair is the tagged test plus the discrimination record, not a downgrade of the row (`ai/rules/rfc-compliance.md`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Three things an operator sees. A wrong migration changes what a converted config asks every peer for, silently. A wrong ingress ceiling drops routes the operator expected to hold, which is a forwarding outage rather than a cosmetic fault. A wrong outbound minimum under-announces to peers. The outbound best-path reorder (owner decision, 2026-09-11) also changes truncation order for every peer that already has an effective outbound bound today, independent of whether the operator configures `send`, since `pathsLimitSection` is the single admission path for both |
| How is it reverted? | The code is a single-commit revert. Operator configs written in the new spelling are NOT revertible: once `limit { receive 10; }` is in a config file, a reverted binary refuses it |
| Who else touches this path? | `session_prefix.go` is the prefix-limit surface and is heavily anchored in docs. `session_paths_limit.go` landed at commit `29102e14f8`, and its outbound behavior is asserted by the FRR interop scenario and `test/plugin/paths-limit-live.ci`. The config parser is shared with every other capability |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer config `add-path { limit { receive 2; } }` loaded from file | → | `parseAddPathFromTree` emits a `capability.PathsLimit` entry and records the receive ceiling on `PeerSettings` | `TestParseAddPathLimitContainerReceive` |
| Peer config `add-path { limit { send 4; } }` loaded from file | → | `initPathsLimit` sets `pathsLimitFamily.limit` to 4 with no peer capability present | `TestInitPathsLimitUsesLocalSendCapAlone` |
| Peer advertises limit 2 while local config says `send 4` | → | `initPathsLimit` sets the family limit to 2 | `TestInitPathsLimitTakesMinimumOfPeerAndLocal` |
| A peer UPDATE carrying a third path for a prefix whose receive ceiling is 2 | → | the ingress check reached from `processMessage`, before `onMessageReceived` | `TestProcessMessageRefusesPathOverReceiveCeiling` |
| An operator loads a config and runs `show bgp peer <ip> capabilities` | → | `addPathsLimitFields` renders `paths-limit` with `send` and `receive` | `test/plugin/paths-limit-receive-ceiling.ci` |
| An ExaBGP config carrying `add-path { ipv4 unicast limit 10; }` is migrated | → | `migrateAddPathToUnified` writes `limit { receive 10; }` | `TestMigrateAddPathLimitBecomesReceiveContainer` |
| A prefix has more qualifying paths than the effective outbound bound | → | `pathsLimitSection` admits the best-ranked identifiers, not the first-arrived ones | `TestPathsLimitSectionAdmitsBestPathOrder` |
| A newly received path outranks an already-sent path at the outbound bound | → | `pathsLimitSection` withdraws the demoted identifier and sends the new one | `TestPathsLimitSectionWithdrawsDemotedPath` |
| The local config says `limit { receive 2; }` and a peer repeatedly over-sends on one prefix | → | the ingress ceiling logs the refusal once per prefix, not once per path, and bounds its tracking set | `TestReceiveCeilingWarnsOncePerPrefix` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer config carries `add-path { limit { receive 10; } }` at container level with no family entries | Ze advertises PATHS-LIMIT 10 for every family it advertises in ADD-PATH, byte-identical to what the scalar `limit 10` produced |
| AC-2 | A peer config carries the old scalar `add-path { limit 10; }` | The config is REFUSED with an error naming `limit` and stating the replacement spelling. It does not load and it is not silently ignored |
| AC-3 | A peer config carries `family { ipv4/unicast { limit { receive 2; send 4; } } }` and a container `limit { receive 10; }` | ipv4/unicast advertises 2 and caps outbound at 4. Every other advertised family advertises 10 and has no outbound cap |
| AC-4 | A peer advertised PATHS-LIMIT 2 for a family and the local config says `limit { send 4; }` for it | Ze admits at most 2 paths per prefix outbound for that family |
| AC-5 | A peer advertised no PATHS-LIMIT and the local config says `limit { send 4; }` | Ze admits at most 4 paths per prefix outbound for that family |
| AC-6 | A peer advertised PATHS-LIMIT 2 and the local config carries no `send` | Ze admits at most 2 paths per prefix outbound, which is today's behavior unchanged |
| AC-7 | Neither the peer nor the local config states a send limit | Ze applies no outbound bound, which is today's behavior unchanged |
| AC-8 | The local config says `limit { receive 2; }` and the peer announces a third distinct path identifier for one prefix, in an UPDATE that also carries other prefixes at or under their ceiling | The excess path identifier is dropped before installation (Q-1: O-1, decided); the two admitted paths and every other prefix in the same UPDATE install normally |
| AC-9 | Following AC-8, the peer sends a WITHDRAW for the identifier Ze refused | The withdrawal is accepted, no error is raised, the session survives, and no slot is freed that was never taken |
| AC-10 | Following AC-8, the peer withdraws one admitted identifier and re-announces the refused one | The re-announced path is admitted, because the withdrawal freed a slot |
| AC-11 | The local config states `limit { receive 2; }` for a family whose ADD-PATH direction does not include receive | No ceiling is applied and no PATHS-LIMIT tuple is advertised for that family |
| AC-12 | An ExaBGP config carrying `add-path { ipv4 unicast limit 10; }` is migrated | The output carries `family { ipv4/unicast { limit { receive 10; } } }` and no `send` leaf |
| AC-13 | An ExaBGP config carrying `add-path { ipv4 unicast; }` with no limit is migrated | The output carries the family with no `limit` container at all |
| AC-14 | `show bgp peer <ip> capabilities` on a session with both a negotiated peer limit and a local send cap | The `paths-limit` payload distinguishes what the peer asked for from what Ze enforces, and the json, yaml and table pipes each render it |
| AC-15 | The editor is asked to complete inside `add-path` and inside `add-path > limit` | The first offers `direction`, `limit`, `family`. The second offers `send` and `receive` |
| AC-16 | `limit { receive 0; }` or `limit { send 0; }` | Refused by the YANG range `1..65535` at load time |
| AC-17 | A prefix carries 5 candidate paths and the effective outbound bound (peer limit combined with local `send`) is 2 | Ze announces the 2 BEST-ranked paths by best-path selection, not the first 2 received (owner decision, 2026-09-11) |
| AC-18 | A prefix is already announcing its best 2 paths under an effective outbound bound of 2, and a newly received path outranks one of the two currently sent | Ze WITHDRAWS the demoted path and announces the newly best one in its place. A path later demoted back below the bound is not re-announced unless it again becomes best |
| AC-19 | The local config says `limit { receive 2; }` and the peer sends five excess paths for one prefix across separate UPDATEs | Ze logs the refusal once for that prefix, not once per refused path, and the tracked path-id set for that prefix never exceeds 3 (2 admitted + 1 headroom) |
| AC-20 | Following AC-19, the peer withdraws a tracked (refused) identifier that had saturated the tracking set, then sends a new excess path for the same prefix | The withdrawal frees the slot and clears the warned marker; the new excess path produces a fresh log line rather than staying silently suppressed |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `limit { receive 2; }` and peers with a daemon that ignores capability 76 | config tree -> `parseAddPathFromTree` -> OPEN capability 76 -> peer over-sends -> `processMessage` ingress check -> adj-RIB-in holds 2 | `test/plugin/paths-limit-receive-ceiling.ci` |
| 2 | Writes `limit { send 4; }` for a peer that advertised nothing, and announces 6 paths for one prefix | config tree -> `PeerSettings` -> `initPathsLimit` -> `pathsLimitSection` -> 4 paths on the wire | `test/plugin/paths-limit-local-send-cap.ci` |
| 3 | Migrates an ExaBGP config carrying `ipv4 unicast limit 10` and loads the result | `migrateAddPathToUnified` -> `limit { receive 10; }` -> `parseAddPathFromTree` -> OPEN carries 10 | `TestMigrateAddPathLimitBecomesReceiveContainer` plus `test/exabgp-compat/encoding/conf-paths-limit.ci` |
| 4 | Runs `show bgp peer <ip> capabilities` and reads what is enforced in each direction | reactor negotiated state -> `addPathsLimitFields` -> text, JSON, YAML and table renderings | `test/plugin/paths-limit-receive-ceiling.ci` |
| 5 | Loads a config still carrying the scalar `limit 10` and reads the error | config tree -> YANG validation -> refusal naming the replacement | `test/ui/cli-config-addpath-scalar-limit-refused.ci` |
| 6 | Peers with an outbound bound of 1, is already announcing one path for a prefix, then receives a second path that outranks it | `pathsLimitSection` ranks by best-path -> WITHDRAW old path -> ANNOUNCE new best path | `TestPathsLimitSectionWithdrawsDemotedPath` plus `test/plugin/paths-limit-outbound-best-path.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseAddPathLimitContainerReceive` | `internal/component/bgp/reactor/config_capabilities_test.go` | container `limit { receive N; }` yields the same PATHS-LIMIT entries the scalar did (AC-1) | |
| `TestParseAddPathLimitContainerPerFamilyOverride` | `internal/component/bgp/reactor/config_capabilities_test.go` | a family `limit` container overrides the container one per direction, independently (AC-3) | |
| `TestParseAddPathLimitSendDoesNotReachWire` | `internal/component/bgp/reactor/config_capabilities_test.go` | a `send` value produces no capability tuple (AC-5) | |
| `TestParseAddPathLimitReceiveWithoutReceiveDirection` | `internal/component/bgp/reactor/config_capabilities_test.go` | no tuple and no ceiling for a family whose direction excludes receive (AC-11) | |
| `TestInitPathsLimitTakesMinimumOfPeerAndLocal` | `internal/component/bgp/reactor/session_paths_limit_test.go` | peer 2 and local 4 give 2 (AC-4) | |
| `TestInitPathsLimitUsesLocalSendCapAlone` | `internal/component/bgp/reactor/session_paths_limit_test.go` | local 4, peer absent, limit 4 (AC-5) | |
| `TestInitPathsLimitUsesPeerLimitAlone` | `internal/component/bgp/reactor/session_paths_limit_test.go` | peer 2, local absent, limit 2 (AC-6) | |
| `TestInitPathsLimitNeitherSideLimits` | `internal/component/bgp/reactor/session_paths_limit_test.go` | no family entry is created and no bound is applied (AC-7) | |
| `TestProcessMessageRefusesPathOverReceiveCeiling` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | the third distinct identifier for a prefix at ceiling 2 does not reach the delivery callback (AC-8) | |
| `TestReceiveCeilingAcceptsWithdrawOfRefusedPath` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | a WITHDRAW for a refused identifier is accepted and frees nothing (AC-9) | |
| `TestReceiveCeilingReadmitsAfterWithdraw` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | a freed slot readmits the previously refused identifier (AC-10) | |
| `TestReceiveCeilingBoundsTrackedPaths` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | the tracked set per prefix never exceeds the limit plus one, so a misbehaving peer cannot decide the memory cost (R-3) | |
| `TestReceiveCeilingWarnsOncePerPrefix` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | repeated excess paths on one prefix produce one log line, not one per path (AC-19) | |
| `TestReceiveCeilingClearsWarnedMarkerOnWithdraw` | `internal/component/bgp/reactor/session_paths_limit_recv_test.go` | a withdrawal of a tracked identifier from a saturated set clears the warned marker, so a later excess warns again (AC-20) | |
| `TestPathsLimitSectionAdmitsBestPathOrder` | `internal/component/bgp/reactor/session_paths_limit_test.go` | more qualifying paths than the outbound bound admits the best-ranked ones, not the first-arrived ones (AC-17) | |
| `TestPathsLimitSectionWithdrawsDemotedPath` | `internal/component/bgp/reactor/session_paths_limit_test.go` | a newly best path replaces an already-sent one, and Ze withdraws the demoted identifier (AC-18) | |
| `TestMigrateAddPathLimitBecomesReceiveContainer` | `internal/exabgp/migration/migrate_addpath_test.go` | `ipv4 unicast limit 10` becomes `limit { receive 10; }` (AC-12) | |
| `TestMigrateAddPathWithoutLimitEmitsNoContainer` | `internal/exabgp/migration/migrate_addpath_test.go` | a bare family emits no `limit` (AC-13) | |
| `TestMigrateAddPathDirectionAndLimitTogether` | `internal/exabgp/migration/migrate_addpath_test.go` | capability-level `add-path send/receive` plus a per-family limit produce both `direction` and `limit` | |
| `TestAddPathsLimitFieldsNamesBothDirections` | `internal/component/bgp/plugins/cmd/peer/fields_test.go` | the rendered payload distinguishes the peer's request from Ze's enforced cap (AC-14) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `add-path > limit > receive` | 1-65535 | 65535 | 0 | 65536 |
| `add-path > limit > send` | 1-65535 | 65535 | 0 | 65536 |
| `add-path > family > limit > receive` | 1-65535 | 65535 | 0 | 65536 |
| `add-path > family > limit > send` | 1-65535 | 65535 | 0 | 65536 |
| effective outbound bound, peer 1 and local 65535 | 1-65535 | 1 | N/A | N/A |
| paths per prefix admitted inbound at ceiling 1 | 0-1 | 1 | N/A | 2 refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `paths-limit-receive-ceiling` | `test/plugin/paths-limit-receive-ceiling.ci` | the operator sets `limit { receive 2; }`, the synthetic peer sends three paths for one prefix, and the RIB holds two (AC-8, AC-9, AC-10) | |
| `paths-limit-local-send-cap` | `test/plugin/paths-limit-local-send-cap.ci` | the operator sets `limit { send 4; }` against a peer advertising nothing, announces six paths, and four reach the wire (AC-5) | |
| `paths-limit-outbound-best-path` | `test/plugin/paths-limit-outbound-best-path.ci` | the operator sets an outbound bound of 1, Ze holds two candidate paths, and when a third path becomes best, Ze withdraws the previously-sent path and announces the new best one (AC-17, AC-18) | |
| `cli-config-addpath-scalar-limit-refused` | `test/ui/cli-config-addpath-scalar-limit-refused.ci` | the old scalar spelling is refused with an error naming the replacement (AC-2) | |
| `paths-limit-live` (edit) | `test/plugin/paths-limit-live.ci` | existing outbound admission, reconfigured to `limit { receive 9; }` so the existing proof survives the config change | |
| `conf-paths-limit` (edit) | `test/exabgp-compat/encoding/conf-paths-limit.ci` | the ExaBGP-native config still produces the same wire output after the migration target changes | |
| `cli-completion-capability-addpath` (edit) | `test/ui/cli-completion-capability-addpath.ci` | the container still offers `direction`, `limit`, `family`, and `limit` now descends (AC-15) | |
| `cli-completion-addpath-fields` (edit) | `test/ui/cli-completion-addpath-fields.ci` | the family list shows the `limit` container (AC-15) | |
| `bgp-peer-capability-addpath` (edit) | `test/editor/completion/bgp-peer-capability-addpath.et` | editor completion inside `limit` offers `send` and `receive` (AC-15) | |
| `paths-limit` encode (edit) | `test/encode/paths-limit.ci` | OPEN still carries code 76 for ipv4/unicast limit 10, configured through the container | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-paths-limit-frr` (edit) | `test/interop/scenarios/` | FRR 10.3.1 | the existing seven-phase outbound admission, reconfigured to the container spelling. Proves the config change did not change the wire | |
| `bgp-paths-limit-receive-frr` | `test/interop/scenarios/` | FRR 10.3.1 | Ze advertises a ceiling of 2, FRR is driven to hold more than two paths for one prefix, and Ze's adj-RIB-in holds two. BLOCKED ON A-5: if FRR caps its tx at Ze's advertised limit the scenario can never observe a red, and the `.ci` synthetic peer is the only over-sender Ze can command | |

## Files to Modify

- `internal/component/bgp/yang/ze-bgp-conf.yang` - replace both `limit` leaves with a `limit` container carrying `send` and `receive`, both `uint16 { range "1..65535"; }`, each with its own `ze:help` naming the direction it governs. The container `ze:help` states the inheritance rule
- `internal/component/bgp/reactor/config_capabilities.go` - `parseAddPathFromTree` reads the container at both levels. The receive half feeds the PATHS-LIMIT tuples unchanged; the send half becomes a new per-family map on `PeerSettings`
- `internal/component/bgp/reactor/config.go` - wherever `PeerSettings` is constructed, for the new send-cap map
- `internal/component/bgp/reactor/session_paths_limit.go` - `initPathsLimit` takes the minimum of the peer's advertised limit and the local send cap, per family; `pathsLimitSection` changes its admission order from arrival order to best-path order and withdraws any identifier that best-path selection demotes out of the kept set (owner decision, 2026-09-11)
- `internal/component/bgp/reactor/session_read.go` - `processMessage` calls the receive ceiling and acts on its verdict, in the same window as `checkPrefixLimits`
- `internal/component/bgp/reactor/session_prefix.go` - only if the receive ceiling reuses `collectPrefixSections`. The prefix count and the path count stay separate journals
- `internal/core/bgp/capability/encoding.go` - the comment on `PathsLimitRecv` stops saying it constrains only the peer
- `internal/exabgp/migration/migrate.go` - `migrateAddPathToUnified` writes the `limit` container
- `internal/component/bgp/plugins/cmd/peer/fields.go` - `addPathsLimitFields` keeps `send` and `receive` coherent once `send` can be a local cap rather than only the peer's number
- `internal/component/bgp/format/decode.go`, `internal/component/bgp/format/json.go` - the same coherence for the JSON event stream. Design doc: `docs/architecture/api/json-format.md`
- `docs/guide/add-path.md` - the config surface, the local limit table, the enforcement table, and the three source anchors
- `docs/config-reference.md` - the ADD-PATH row names `limit N`
- `docs/features.md` - the PATHS-LIMIT row states the knob does not enforce
- `docs/architecture/wire/capabilities.md` - the anchor target for `session_paths_limit.go`
- `docs/features/exabgp-compatibility.md` - what the bridge converts
- `docs/guide/command-reference.md` - anchored on `addPathsLimitFields`
- `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` - requirement 3-7's proof, the `Support coverage` sentence, and the Ze Implementation config paragraph
- `test/interop/scenarios/bgp-paths-limit-frr/ze.conf`, `test/plugin/paths-limit-live.ci`, `test/encode/paths-limit.ci`, `test/ui/cli-completion-peer-with-capability.ci`, `test/ui/cli-completion-addpath-fields.ci`, `test/ui/cli-completion-capability-addpath.ci`, `test/editor/completion/bgp-peer-capability-addpath.et` - the config spelling

## Files to Create

- `internal/component/bgp/reactor/session_paths_limit_recv.go` - the ingress ceiling: a pooled-buffer UPDATE rewrite with a rollback journal mirroring `filterPathsLimit`, dropping only the excess path identifiers per prefix (Q-1: O-1), plus the once-per-prefix warn log and the bounded per-prefix tracking set (`limit + 1`) (owner decision, 2026-09-11)
- `internal/component/bgp/reactor/session_paths_limit_recv_test.go` - its unit tests
- `internal/exabgp/migration/migrate_addpath_test.go` - the migration tests this path has never had
- `test/plugin/paths-limit-receive-ceiling.ci` - the operator-facing ingress proof
- `test/plugin/paths-limit-local-send-cap.ci` - the operator-facing local outbound cap proof
- `test/plugin/paths-limit-outbound-best-path.ci` - the operator-facing best-path truncation and withdraw-the-loser proof
- `test/ui/cli-config-addpath-scalar-limit-refused.ci` - the refusal of the deleted spelling
- `test/interop/scenarios/bgp-paths-limit-receive-frr/` - pending A-5

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang`, the `add-path` container and its `family` list |
| YANG validation constraints | Yes | both new leaves keep `type uint16 { range "1..65535"; }`. The container takes no presence statement, because an empty `limit` block states nothing |
| YANG custom validators | Yes | a `ze:validate` on the deleted scalar spelling, so an old config is refused with an error naming the replacement rather than an unknown-leaf message (AC-2) |
| CLI commands/flags | N-A | no new command. `show bgp peer <ip> capabilities` renders new meaning through an existing path |
| CLI grammar (keyword before value) | Yes | `limit { send 4; receive 2; }` is keyword before value at both levels |
| Editor autocomplete | Yes | automatic for the two new YANG leaves. `test/editor/completion/bgp-peer-capability-addpath.et` asserts it |
| Functional test for new RPC/API | Yes | `test/plugin/paths-limit-receive-ceiling.ci`, `test/plugin/paths-limit-local-send-cap.ci` |
| Pipe completeness | Yes | the `paths-limit` payload is structured data rendered through the standard pipes. AC-14 asserts all three |
| Env var registration | N-A | no leaf under `environment/`. ExaBGP's `bgp.paths_limit_audit` is an audit knob for an option Ze is not taking |
| Doctor check for runtime dependencies | N-A | no new file path, socket, service, module, port, procfs entry, netlink use, binary or certificate |
| Prometheus counters/metrics | Yes | a counter of paths refused by the receive ceiling, per peer and family, so a fleet can see a peer ignoring capability 76. Name and labels are fixed at implementation, beside the existing PATHS-LIMIT withheld counters |
| BGP family surface (new SAFI / capability / attribute) | N-A | capability 76 already exists and its wire shape does not change. No new SAFI and no new attribute, so `ai/patterns/bgp-family.md` does not apply |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, the PATHS-LIMIT row |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/config-reference.md`, `docs/guide/add-path.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, the `paths-limit` payload meaning |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`, the negotiated capability payload |
| 5 | Plugin added/changed? | N-A | no plugin gains or loses a command |
| 6 | Has a user guide page? | Yes | `docs/guide/add-path.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/capabilities.md`. The format does not change, but the meaning of Ze's advertised number does |
| 8 | Plugin SDK/protocol changed? | Yes | `internal/component/plugin/types_bgp.go` carries `NegotiatedPathsLimitSend`. If a local cap is exposed there, `docs/architecture/api/process-protocol.md` follows |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` requirement 3-7 plus the `Support`, `Support coverage` and `Enrolment reason` rows. `docs/features/rfc-status.md` regenerates from them |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the new `.ci` files introduce a fixture shape; otherwise named as unaffected |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: a receive-side ceiling is a capability FRR and ExaBGP do not have, and ExaBGP audits rather than enforces. FRR 10.5.3 already has the outbound `send`-equivalent (`addpath-tx-best-selected`) and best-path truncation with withdraw-the-loser, which this spec adopts. BIRD 2.19 implements no capability 76 at all; its channel-wide route limits (`receive limit`, `import limit`) are a different mechanism |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` or the BGP subsystem doc, for the second ingress admission point |
| 13 | Route metadata keys added/changed? | N-A | no route metadata key is added |
| 14 | Prometheus counters added/changed? | Yes | the refused-path counter, in the BGP telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/features/plugins.md` and `docs/guide/status.md`, only if the negotiated payload shape changes. Verify rather than assume |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec <this spec path>`. Known from `ai/CODE-TO-DOCS.md`: `config_capabilities.go` declares `docs/guide/add-path.md`; `session_paths_limit.go` declares `docs/architecture/wire/capabilities.md`, `docs/features.md`, `docs/guide/add-path.md`; `session_prefix.go` declares eight pages; `migrate.go` is mentioned by `docs/DESIGN.md`, `docs/guide/configuration.md`, `docs/features/configuration.md`, `docs/features/exabgp-compatibility.md`; `fields.go` by `docs/guide/command-reference.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `limit N` example in `docs/guide/add-path.md` (lines 20, 38, 67) and `docs/config-reference.md`. The journal row in `plan/journal/documentation-shows-config-the-parser-refuses.md` records that this page already shipped an example the loader refuses, so each edited block is loaded rather than eyeballed |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the container reachable and prove the refusal of the old spelling
   - Tests: `TestParseAddPathLimitContainerReceive`, `test/ui/cli-config-addpath-scalar-limit-refused.ci`
   - Files: `internal/component/bgp/yang/ze-bgp-conf.yang`, `internal/component/bgp/reactor/config_capabilities.go`
   - Verify: a config with the container loads and advertises what the scalar did; a config with the scalar is refused by name. The receive ceiling is still a stub, so the ingress tests fail
2. **Phase: Migration** -- Q-2 is answered (RECEIVE direction, no send-side source; owner, 2026-09-11), so this phase is unblocked
   - Tests: `TestMigrateAddPathLimitBecomesReceiveContainer`, `TestMigrateAddPathWithoutLimitEmitsNoContainer`, `TestMigrateAddPathDirectionAndLimitTogether`, `test/exabgp-compat/encoding/conf-paths-limit.ci`
   - Files: `internal/exabgp/migration/migrate.go`, `internal/exabgp/migration/migrate_addpath_test.go`
   - Verify: the tests go red against today's scalar output, then green. This path has never had a test, so the red phase is real rather than forced
3. **Phase: Local outbound cap and best-path truncation** -- the smaller of the two numbers, then the best-ranked paths within it, with withdraw-the-loser (owner decision, 2026-09-11)
   - Tests: `TestInitPathsLimitTakesMinimumOfPeerAndLocal`, `TestInitPathsLimitUsesLocalSendCapAlone`, `TestInitPathsLimitUsesPeerLimitAlone`, `TestInitPathsLimitNeitherSideLimits`, `TestPathsLimitSectionAdmitsBestPathOrder`, `TestPathsLimitSectionWithdrawsDemotedPath`, `test/plugin/paths-limit-local-send-cap.ci`, `test/plugin/paths-limit-outbound-best-path.ci`
   - Files: `internal/component/bgp/reactor/session_paths_limit.go`, `internal/component/bgp/reactor/config_capabilities.go`
   - Verify: all four combinations for the minimum, plus best-path admission and withdraw-the-loser. The existing FRR interop scenario stays green because its local config states no `send`, but its admission order now follows best-path rather than arrival order
4. **Phase: Ingress ceiling** -- implements the decided O-1 remedy (drop the excess path, not the whole message; owner, 2026-09-11), plus the once-per-prefix warn and bounded tracking set
   - Tests: `TestProcessMessageRefusesPathOverReceiveCeiling`, `TestReceiveCeilingAcceptsWithdrawOfRefusedPath`, `TestReceiveCeilingReadmitsAfterWithdraw`, `TestReceiveCeilingBoundsTrackedPaths`, `TestReceiveCeilingWarnsOncePerPrefix`, `TestReceiveCeilingClearsWarnedMarkerOnWithdraw`, `test/plugin/paths-limit-receive-ceiling.ci`
   - Files: `internal/component/bgp/reactor/session_paths_limit_recv.go`, `internal/component/bgp/reactor/session_read.go`
   - Verify: only the excess path identifiers are refused, every other prefix in the UPDATE installs normally, the tracked set stays bounded, the warn log fires once per prefix, and withdrawal and readmission behave as the send side does
5. **Phase: Surfaces** -- CLI, JSON, telemetry, and every in-tree config
   - Tests: `TestAddPathsLimitFieldsNamesBothDirections`, the four edited completion tests, `test/encode/paths-limit.ci`, `test/plugin/paths-limit-live.ci`
   - Files: `fields.go`, `format/decode.go`, `format/json.go`, and the seven config files listed above
   - Verify: the json, yaml and table pipes each render the payload, and no in-tree config still carries the scalar
6. **Phase: RFC proof** -- the tagged test and its discrimination record
   - Tests: the tagged carrier for `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7`, recorded through `./le rfc discriminate-record`
   - Files: `rfc/short/draft-abraitis-idr-addpath-paths-limit.md`, `rfc/discrimination/`
   - Verify: `./le rfc check` accepts the tagged unit, and the `Support coverage` sentence no longer says the knob does not police
7. **Phase: Interop** -- gated on A-5
   - Tests: `bgp-paths-limit-frr` reconfigured, and `bgp-paths-limit-receive-frr` if FRR can be made to over-send
   - Files: `test/interop/scenarios/`, `internal/le/interoplab/bgp/`
   - Verify: the revert, rebuild and red walk in `docs/architecture/testing/interop.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The outbound bound is the MINIMUM, and an absent value on either side is absence rather than zero. A zero must never read as "no paths" |
| Correctness | The receive ceiling counts distinct PATH IDENTIFIERS per prefix per family, not NLRIs and not prefixes |
| Guard | The receive ceiling fails CLOSED. A family whose splitter or prefix-key function is missing refuses, and never admits silently (`ai/rules/evidence.md`) |
| Naming | The YANG leaf names match the `direction` vocabulary exactly: `send` and `receive`, never `tx`, `rx`, `out` or `in` |
| Naming | JSON keys stay kebab-case, and `paths-limit` keeps its existing spelling |
| Data flow | The `send` cap never reaches the wire. Only the `receive` number becomes a capability tuple |
| Data flow | The PATHS-LIMIT tuples are still derived from `addPath.Families`, so requirement 3-3 cannot be violated by the restructure |
| Rule: `ai/rules/no-layering.md` | The scalar leaf is DELETED. No fallback reads it, no shim accepts it, and the only mention of the old spelling is the validator that refuses it |
| Rule: `ai/rules/performance.md` | The ingress path allocates nothing per UPDATE that it did not allocate before. The per-prefix set is per session, not per message |
| Rule: `ai/rules/rfc-compliance.md` | Requirement 3-7 moves from claimed to proven, with a discrimination record. The row is not downgraded |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The scalar leaf is gone from YANG | `grep -n "leaf limit" internal/component/bgp/yang/ze-bgp-conf.yang` returns only the two new leaves inside the container |
| No in-tree config still uses the scalar | a grep for a numeric `limit` value across `test/`, `docs/` and `etc/`, intersected with `add-path`, returns nothing |
| The migration has tests | `ls internal/exabgp/migration/migrate_addpath_test.go` and `go test ./internal/exabgp/migration/ -run AddPath` |
| The ingress ceiling is reachable from the wire | `test/plugin/paths-limit-receive-ceiling.ci` passes, and fails when the ceiling is reverted |
| Requirement 3-7 carries a discrimination record | `./le rfc check` and `ls rfc/discrimination/draft-abraitis-idr-addpath-paths-limit.json` |
| Every changed doc anchor is verified | `./le spec citation anchors spec <this spec path>` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | The per-prefix path-id set on ingress is sized by the PEER. Bound it at the limit plus one per prefix, as ExaBGP does, so a peer ignoring the capability cannot decide Ze's memory cost |
| Resource exhaustion | The number of tracked PREFIXES is also peer-controlled. Confirm the existing prefix maximum bounds it, and say so, rather than assuming |
| Input validation | A path identifier is four peer-supplied octets. The extraction rejects a short NLRI rather than indexing past the end, as `pathsLimitSection` does with its five-octet guard |
| Fail closed | A family with a missing splitter or prefix-key function refuses the message, and never admits it uncounted |
| Error leakage | A refusal log line names the peer, the family and the prefix. It must not log the whole message body per refused path, which a misbehaving peer would use to fill the disk |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| FRR will not over-send (A-5) | Do not weaken the scenario. Record the finding, keep the `.ci` proof, and ask which peer daemon can be made to over-send |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Goal Validation

| Goal | Evidence that proves it |
|------|------------------------|
| G-1 The `limit` leaf becomes a container carrying `send` and `receive` at both levels, and the scalar is deleted | `TestParseAddPathLimitContainerReceive`, `TestParseAddPathLimitContainerPerFamilyOverride`, and `test/ui/cli-config-addpath-scalar-limit-refused.ci`, which proves the old spelling no longer loads |
| G-2 `receive` bounds what Ze accepts, not only what Ze advertises | `test/plugin/paths-limit-receive-ceiling.ci`: a peer sends three paths for one prefix at ceiling 2 and the RIB holds two. The tagged carrier for requirement 3-7 with a `./le rfc discriminate-record` artifact proving it goes RED when the ceiling is removed |
| G-3 `send` caps what Ze announces, combined with the peer's limit by minimum | `test/plugin/paths-limit-local-send-cap.ci` for the peer-silent case, plus the four `TestInitPathsLimit` unit tests covering every combination of present and absent |
| G-4 A migrated ExaBGP config asks peers for exactly what it asked for before | `TestMigrateAddPathLimitBecomesReceiveContainer` and `test/exabgp-compat/encoding/conf-paths-limit.ci`, which drives the ExaBGP-native config end to end and asserts the same wire output |
| G-5 The wire is unchanged for every config that states no `send` | `test/encode/paths-limit.ci` unchanged in its assertion, and the `bgp-paths-limit-frr` interop scenario green after the config restructure |
| G-6 The public ledger stops over-claiming | `rfc/short/` requirement 3-7 carries a tagged test and a discrimination record. `docs/features.md` and `docs/guide/add-path.md` no longer state that the knob does not police |
| G-7 Outbound truncation picks the best paths, not the first-arrived ones, and a demoted path is withdrawn rather than silently dropped (owner decision, 2026-09-11) | AC-17 and AC-18, proven by `TestPathsLimitSectionAdmitsBestPathOrder`, `TestPathsLimitSectionWithdrawsDemotedPath`, and `test/plugin/paths-limit-outbound-best-path.ci`, which also proves a path that later becomes best is announced in place of the one it demotes |

## Design Insights

- The receive ceiling and the advertised number being ONE leaf is not a simplification: it follows from the draft, whose capability is receiver-advertised. A speaker that advertised 2 and accepted 5 would be advertising a number it does not mean.
- The send side already solved the three hard cases of per-prefix path admission: withdrawal of an unknown identifier, replacement at capacity, and rollback when the write fails. The receive side inherits all three answers rather than re-deriving them.
- ExaBGP reached for an audit rather than a drop on ingress, and bounded its audit set at the limit plus one with a comment naming why: "Without this a peer which ignores the limit, the only peer the audit exists to catch, is also the one which decides how much memory the audit costs." Ze takes both: the O-1 drop and the audit's bound and once-per-prefix warn (owner decision, 2026-09-11).
- FRR 10.5.3 already has the local send cap Ze is adding, `addpath-tx-best-selected (1-6)`, and combines it with the peer's advertised limit as `MIN(paths_limit, addpath_best_selected)`, the same combination rule this spec already applies to `send` (AC-4 through AC-7). BIRD 2.19 does not implement capability 76 at all; its `receive limit` / `import limit` count routes channel-wide rather than paths per prefix, with an action vocabulary (`warn`, `block`, `restart`, `disable`, `disable` the unwritten default) unlike Ze's per-path ceiling.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `limit` becomes a container with `send` and `receive` | A second scalar leaf named `receive-limit`; keeping `limit` as a shorthand for `receive` | Owner decision, 2026-09-11. The container reuses the vocabulary `direction` already uses in the same block, so an operator learns one pair of words. A shorthand would be a second declaration of the same fact (`ai/rules/no-layering.md`) |
| The outbound bound is the minimum of the two numbers | Local cap always wins; peer limit always wins | The peer's number is what it asked for and the draft's SHOULD says to honor it. The local number is what the operator will tolerate. Neither can be exceeded, so both bind, so the answer is the smaller |
| The minimum is taken once, in `initPathsLimit` | Taken per write, in `pathsLimitSection` | Both inputs are fixed for the life of a connection, and `pathsLimitSection` is on the hot write path |
| The ingress ceiling is a new file beside `session_prefix.go`, not a branch inside `checkPrefixLimits` | Extending `checkPrefixLimits` with a path-count mode | The prefix count and the path count answer different questions and carry different rollback state. Entangling the installed-family journal with a path-id journal makes both harder to reason about, and `ai/rules/simplicity.md` cuts machinery, not separation that is carrying weight |
| Outbound truncation admits best-path order, with withdraw-the-loser (owner decision, 2026-09-11) | Keep arrival order (today's behavior); rank only at initial admission and never re-rank | FRR 10.5.3 already does this (`subgrp_announce_addpath_best_selected`, `bgpd/bgp_updgrp_adv.c`). An operator who configures an outbound bound expects Ze's BEST paths, not whichever arrived first, and a path that later becomes best must replace a worse one Ze is already announcing |

## Known Limitations

- The `send` cap is per family and per peer. A cap across all peers, or across the whole RIB, is not in scope and no leaf is added for it.
- PATHS-LIMIT stays meaningful only where ADD-PATH is negotiated for the family, which the draft requires and `negotiatePathsLimitDirection` already enforces.
- Whether FRR can be driven to over-send is unresolved (A-5). If it cannot, the receive-side interop scenario does not exist and the ingress proof rests on the synthetic peer. That is a gap in EVIDENCE, not a reduction of scope, and it is reported rather than closed.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. For this spec:

- Above the receive ceiling's admission function: draft-abraitis-idr-addpath-paths-limit-04 Section 3, "An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender."
- Above the outbound minimum in `initPathsLimit`: Section 3, "A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability." The local cap is stricter than this asks for and therefore cannot violate it.
- Above the O-1 excess-path drop (Q-1, decided): the reason no NOTIFICATION is sent, citing RFC 4271 Section 6.3's enumeration and RFC 4486 Section 4's scope, both quoted in Design Decisions above.
- The existing citations above the PATHS-LIMIT tuple derivation in `parseAddPathFromTree` stay, because requirements 3-3 and 3-5 still hold there.

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
- [ ] AC-1 through AC-20 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
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
