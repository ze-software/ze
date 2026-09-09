# Spec: pppoe-padr-replay-allocates-unbounded-sessions

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-09-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**One host on an access interface exhausts the access concentrator's file
descriptors by replaying a single valid PADR.**

The per-MAC discovery limiter runs in the PADI handler only. The PADR handler
answers a retransmission from its dedup branch only while the earlier session is
still in `StateDiscovery`. Once that session moves into PPP, the next PADR from
the same MAC address takes the allocation path again: a fresh session ID, a fresh
`AF_PPPOX` socket, a fresh `/dev/ppp` channel and a fresh unit. The AC-Cookie is
a stateless HMAC with a default 5-second validity and no single-use property, so
one captured PADR is replayable for its whole lifetime and no authentication step
stands between the frame and the allocation.

Each replay also overwrites the session table's MAC index, because `Add` assigns
`byMAC[key]` unconditionally. The previous session stays in the SID map holding
its descriptors and is no longer reachable by MAC, so `handlePADT` can never
release it and the drain path can never find it.

The declared bound is `max-sessions`, default 65535 per interface. Three file
descriptors per session reach the process limit long before that count, and the
sessions that are already orphaned from the MAC index are not reclaimable by
anything the subscriber can send.

The goal: an unauthenticated host on an access interface must not be able to make
the AC allocate session resources without bound, and a session must never become
unreachable from the MAC index while it still holds descriptors.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - the AC's declared design: per-interface session tables, the HMAC AC-Cookie, PADS after kernel setup, the PADI rate limit
  → Decision: each interface owns an independent session table, its own limiter and its own cookie key, so any cap this spec adds is scoped to one interface and never global
  → Constraint: the page describes the limiter as a PADI control. It is silent on PADR admission, on the per-MAC session count, and on what happens when a MAC already holds a session, so the page gains those statements in this work
  → Constraint: PADS is sent only after kernel setup succeeds, so a refusal decided before allocation must be answered with a PADS error tag rather than by silence

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2516.md` - PPPoE discovery, PADR admission and the PADS error tags
  → Constraint: RFC 2516 Section 5.4 states the AC "generates a unique SESSION_ID" on PADR and, where it "does not like the Service-Name", sends a PADS carrying a Service-Name-Error tag with SESSION_ID set to `0x0000`. A refusal is therefore expressed as a PADS carrying an error tag, never as a dropped frame
  → Constraint: RFC 2516 places no limit on the number of concurrent sessions one peer MAC address may hold, so a cap of exactly one would refuse a conformant multi-session CPE. Any cap is configurable and defaults above one

### Other Implementations (read 2026-09-08, source quoted)
- [ ] accel-ppp `accel-pppd/ctrl/pppoe/pppoe.c`, `pppoe_recv_PADR`, `find_channel`, `check_cookie`, `check_padi_limit`
  → Constraint: its PADR dedup is keyed on the COOKIE, not the MAC: `conn = find_channel(serv, (uint8_t *)ac_cookie_tag->tag_data)`, and `find_channel` compares `memcmp(conn->cookie, cookie, COOKIE_LENGTH - 4)`. **Ze's comment at `handlePADR` claims its MAC-keyed dedup "Matches accel-ppp's find_channel check", and that claim is false.** The comment is repaired in this work (`ai/rules/stale-comments.md`)
  → Constraint: accel-ppp has NO per-peer session limit. It bounds globally with `conf_max_sessions`, off by default, and its one MAC-keyed check, `connlimit_check(cl_key_from_mac(addr))`, is reached only from `pppoe_recv_PADI`. So the PADI-only limiter is not a Ze oversight, it is the shape Ze inherited from the reference implementation, and bounding PADR is new
  → Decision: its cookie is stateless and NOT consumed. `check_cookie` returns only on expiry or MD5 mismatch, with a 5-second default, so one captured PADR is replayable for its whole life there too. That confirms the lever: a per-peer cap, never cookie consumption
  → Decision: MAC dedup is a STRONGER replay bound than cookie dedup, and it forbids what accel-ppp permits, several concurrent sessions from one MAC. That is exactly why the cap is configurable and defaults above one rather than being a hard one-per-MAC rule

**Key insights:**
- Ze would be the first of the two to bound sessions per peer. That is a reason to keep the mechanism small and configurable, not a reason to drop it: accel-ppp's answer to this flood is an optional module on the PADI path only, which does not bound PADR at all.
- The limiter's per-MAC arm is a one-per-second dedup, not a token bucket. Reusing the same limiter instance for PADR would drop the legitimate PADR that follows its own PADI inside the same second, which breaks normal session setup.
- The cookie proves a PADI round trip happened. It does not prove this PADR is the first use of that round trip.
- Orphaning is a second, independent defect: it survives any admission fix, because a legitimate re-established session from the same MAC also overwrites the index.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/pppoe/server.go` - `handlePADI` calls `limiter.Check`; `handlePADR` does not. `handlePADR` verifies the cookie, matches the Service-Name, then returns the existing SID only when `existing.State == StateDiscovery`, and otherwise allocates a SID, builds a `Session`, calls `sessions.Add`, and drives the PPP driver
- [ ] `internal/component/l2tp/pppoe/session.go` - `SessionTable.Add` inserts into `sessions[SID]` and assigns `byMAC[key]` with no check for an existing entry; `Remove` deletes the MAC entry only when the stored session's SID matches the one being removed; `lookupByMAC` returns the single indexed session
- [ ] `internal/component/l2tp/pppoe/ratelimit.go` - `PADILimiter.Check` enforces a per-interface count inside a one-second window and refuses a second frame from the same MAC inside that window
- [ ] `internal/component/l2tp/pppoe/cookie.go` - `GenerateCookie` and `VerifyCookie`: a 20-byte value, 16 bytes of truncated HMAC-SHA256 over the two MAC addresses plus 4 bytes of timestamp, verified against a per-interface key and the configured `cookie-timeout`. No consumption, no replay record
- [ ] `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - `padi-rate-limit` (default 100, range 1..10000), `cookie-timeout` (default 5), global `max-sessions` (default 65535) and the per-interface `max-sessions` override
- [ ] `internal/component/l2tp/pppoe/subsystem.go` - `discoveryReader` reads one frame per iteration and dispatches through `HandleDiscovery`, which routes PADI, PADR and PADT to their handlers

**Behavior to preserve:**
- A PADR retransmitted while its session is still in `StateDiscovery` is answered with a PADS carrying the existing session ID. A subscriber that misses a PADS must not be given a second session.
- The cookie stays stateless: a per-interface key, no table, no per-cookie record.
- Each interface keeps its own session table, limiter and cookie key.
- The PADI limiter's current behavior and its `padi-rate-limit` leaf semantics are unchanged for PADI.
- `AllocSID` exhaustion is still answered with a PADS carrying `TagACSystemError`.

**Behavior to change:**
- A PADR from a MAC address that already holds sessions is admitted only while that MAC is below a per-MAC session cap; over the cap it is answered with a PADS error rather than allocated.
- The PADR dedup branch answers a retransmission in every session state, not only `StateDiscovery`.
- `SessionTable.Add` no longer silently replaces an existing MAC index entry, and the index resolves a MAC to every session that MAC holds.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An Ethernet frame of EtherType `0x8863` arriving on a configured access interface, read by `discoveryReader` from the subsystem's `AF_PACKET` socket.
- Format at entry: a PPPoE discovery frame, code `0x19` (PADR), carrying an AC-Cookie tag, a Service-Name tag and optionally Host-Uniq and Relay-Session-Id.

### Transformation Path
1. `discoveryReader` (`subsystem.go`) reads the frame and resolves the per-interface server from the ifindex.
2. `ParseDiscovery` (`discovery.go`) validates the header and walks the tags.
3. `HandleDiscovery` (`server.go`) dispatches on the code to `handlePADR`.
4. `handlePADR` verifies the AC-Cookie, matches the Service-Name, applies the dedup branch, and otherwise allocates.
5. `SessionTable.AllocSID` and `SessionTable.Add` (`session.go`) take a session ID from the bitmap and index the session by SID and by MAC.
6. The PPP driver opens the `AF_PPPOX` socket, the `/dev/ppp` channel and the unit, and the session enters PPP.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire → AC | `AF_PACKET` read of a discovery frame, parsed by `ParseDiscovery` | No |
| AC → kernel | `AF_PPPOX` connect plus `/dev/ppp` channel and unit per admitted session | No |
| Config → AC | YANG `pppoe` container resolved by `ExtractParameters` (`config.go`) into the subsystem's per-interface parameters | No |
| AC → operator | PADS error tag on refusal, and the session count reported by the PPPoE show command (`pppoe/cmd/`) | No |

### Integration Points
- `SessionTable` (`session.go`) - the cap and the multi-session index live here, so the admission decision reads one table rather than a new structure beside it.
- `handlePADR` (`server.go`) - the single admission point; the refusal is expressed with the existing `BuildPADSError` builder.
- `ExtractParameters` (`config.go`) - carries the new leaf from YANG to the per-interface server, alongside `max-sessions`.
- `drain.go` - the drain path walks the session table, so a MAC index that holds several sessions must not change what drain iterates.

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
| A-1 | Reusing `PADILimiter` for PADR would refuse the legitimate PADR that follows its own PADI inside the same one-second window | `PADILimiter.Check` (`ratelimit.go`) refuses a MAC seen inside the current window | A rate-limit-only fix is viable and the session cap is unnecessary machinery | A unit test that drives PADI then PADR from one MAC through a shared limiter and asserts the PADR is refused | confirmed (phase 4/5, 2026-09-09): `TestPADILimiterRefusesPADRInSameWindow` (`ratelimit_test.go`) calls `Check(mac)` once for the PADI, then again for the PADR that follows it in the same window, on ONE shared `PADILimiter`. The second call returns `false`: the per-MAC dedup arm (`ratelimit.go` line 57, `now-lastSeen < nanoPerSec`) refuses it. Broke the dedup check (`&& false &&`), watched the test fail for that exact reason, restored it, confirmed green. The Key Design Decisions row rejecting limiter reuse rests on a true premise |
| A-2 | A conformant subscriber CPE may hold more than one concurrent session from one MAC address | RFC 2516 places no per-peer session limit; `rfc/short/rfc2516.md` | A cap of exactly one is safe and no configuration leaf is needed | Read RFC 2516 Sections 4 and 5.4 in `rfc/full/rfc2516.txt` and quote the absence | confirmed (phase 1, 2026-09-08; Status cell corrected phase 4/5, 2026-09-09): Required Reading already recorded the read -- RFC 2516 places no limit on the number of concurrent sessions one peer MAC may hold, so a cap of exactly one would refuse a conformant multi-session CPE. This row's Status cell was never flipped even though every later phase already treated it as confirmed; corrected here, no new evidence needed |
| A-3 | Three file descriptors per session (`AF_PPPOX` socket, `/dev/ppp` channel, `/dev/ppp` unit) is the real bound, reached far below 65535 sessions | `kernel_linux.go` opens each per admitted session; the default process limit is well under 196605 | The exhaustion claim overstates the severity, though the orphaning defect is unchanged | Count the descriptors an admitted session holds in `kernel_linux.go`, and drive the QEMU test to the limit | confirmed (phase 4/5, 2026-09-09), by reading the producers rather than running the QEMU test (this host cannot run PPPoE): `pppoeCreate` (`kernel_linux.go`) opens exactly one `AF_PPPOX` socket, stored as `sess.PppoxFD`. `ppp.DevPPPSetup` (`internal/component/l2tp/ppp/devppp_linux.go`) opens `/dev/ppp` twice through `openDevPPP` (open, dup, close the original -- one net fd per call), returning `chanFD` and `unitFD`. `handlePADR` (`server.go`) stores all three on the session and hands `chanFD`/`unitFD` to `ppp.StartSession`, held until the session ends -- exactly 3 fds for the local-PPP-termination path this spec's AC-6 flood test exercises. (The L2TP-relay branch closes `unitFD` immediately, so a relayed session holds 2, not 3; that path is unaffected by this spec.) The exhaustion claim is accurate, not overstated |
| A-4 | Nothing outside `lookupByMAC` depends on one MAC resolving to exactly one session | `lookupByMAC` callers in `server.go` and `drain.go` | Changing the index to hold several sessions breaks a caller that assumes uniqueness | `gopls references` on `lookupByMAC` and `byMAC`, each caller read | confirmed (phase 2, 2026-09-09): `gopls references` finds exactly one production caller of `lookupByMAC`, the PADR dedup branch in `server.go`'s `handlePADR`, and zero callers of the `byMAC` field outside `session.go` itself. `drain.go` (`startPPPoEAuthDrain`, `startPPPoEPoolDrain`) never touches `SessionTable` at all -- it drains PPP driver auth/IP events keyed by `(TunnelID, SessionID)`, not by MAC, so the "drain path" language in this row's Basis and in the Integration Points section overstates drain.go's coupling to the MAC index; it is unaffected by this phase's change. The one real caller was updated to iterate the new `sessionsByMAC` slice and pick the session in `StateDiscovery`, so no caller's contract broke |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A per-MAC cap refuses a legitimate multi-session CPE | The interop scenario's second session gets a PADS error | Default the cap above one, and make it configurable per interface as `max-sessions` already is |
| R-2 | Answering a retransmitted PADR in every state returns a SID for a session that is tearing down, so the subscriber binds to a dying session | The subscriber sends LCP into a session whose PPP unit is closing | The dedup branch answers only sessions the table still holds and whose PPP unit is live; a session in teardown is treated as absent |
| R-3 | A multi-session MAC index leaks entries when a session is removed | The index grows across a churn test while the SID map does not | `Remove` deletes the session from the MAC entry's set and drops the entry when the set empties; a churn unit test asserts both maps return to their starting size |
| R-4 | The refusal PADS becomes an amplification vector: one small PADR draws a larger PADS | Response bytes exceed request bytes under a flood | The refusal path is itself subject to the discovery rate limit, so refusals are bounded per second per interface |
| R-5 | A cap enforced only at PADR leaves the PADI path free to consume the limiter budget of other subscribers | Legitimate subscribers stop getting PADO under a flood from one MAC | Out of scope here, and named as a limitation: the PADI limiter already dedups per MAC per second |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A subscriber is refused a session they are entitled to, or a live session becomes unreachable by MAC and cannot be torn down by PADT |
| How is it reverted? | Single commit revert. No config migration: the new leaf takes a default, and an absent leaf keeps the current behavior at its cap |
| Who else touches this path? | `plan/spec-l2tp-ipv6-subscriber.md` (skeleton) and the other PPPoE specs in this bucket; `drain.go` walks the same table |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A PADR frame on the AC's `AF_PACKET` socket from a MAC at the cap | → | `handlePADR` admission branch (`server.go`) | `TestPADRAtPerMACCapAnswersError` |
| A PADR frame replayed after its session entered PPP | → | `handlePADR` dedup branch (`server.go`) | `TestPADRReplayAfterPPPReturnsExistingSID` |
| `pppoe interface <name> max-sessions-per-mac` in the config tree | → | `ExtractParameters` → `InterfaceServer` cap field | `TestExtractParametersCarriesPerMACCap` |
| A second session added for one MAC | → | `SessionTable.Add` MAC index | `TestSessionTableIndexesEverySessionOfOneMAC` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A MAC holding sessions equal to the per-interface per-MAC cap sends a further valid PADR | No session ID is allocated, no socket, channel or unit is opened, and the AC answers with a PADS carrying an error tag and session ID `0x0000` |
| AC-2 | A MAC replays a valid PADR after its session has left `StateDiscovery` and is live in PPP | The AC answers with a PADS carrying the existing session ID and allocates nothing |
| AC-3 | A MAC opens two sessions while the cap is above two | Both sessions exist in the table, and a MAC lookup resolves both |
| AC-4 | A session of a MAC that holds several sessions is removed | Only that session leaves the table, the other stays reachable by MAC, and the MAC entry disappears once the last session goes |
| AC-5 | A PADT names a session of a MAC that holds several sessions | That session is torn down and the others are untouched |
| AC-6 | 10000 valid PADR replays arrive from one MAC in one second | The process descriptor count at the end is within one session's worth of the count before, and the AC is still answering a new subscriber's PADI |
| AC-7 | `max-sessions-per-mac` is absent from the configuration | The cap takes its documented default and no behavior depends on the leaf being written |
| AC-8 | `max-sessions-per-mac` is set outside its YANG range | The configuration is refused at validation with a message naming the leaf and the range |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Connects a CPE and gets one session | PADI → PADO → PADR → admission → PADS → PPP | `01-pppoe-chap-ipv4` interop scenario |
| 2 | Floods PADR replays from one MAC and the AC keeps serving other subscribers | PADR → admission refusal → PADS error, while a second MAC completes discovery | `TestPADRFloodDoesNotStarveASecondSubscriber` |
| 3 | Sets a per-MAC session cap and sees it enforced and reported | config tree → `ExtractParameters` → `InterfaceServer` → PADS error | `test/pppoe/pppoe-per-mac-cap.ci` |
| 4 | Tears down one of a MAC's sessions with PADT and keeps the other | PADT → `lookupByMAC` → `Remove` | `TestPADTRemovesOnlyTheNamedSession` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPADRAtPerMACCapAnswersError` | `internal/component/l2tp/pppoe/server_test.go` | AC-1: refusal is a PADS error with session ID 0, and no allocation happened | |
| `TestPADRReplayAfterPPPReturnsExistingSID` | `internal/component/l2tp/pppoe/server_test.go` | AC-2: the dedup branch covers a session past `StateDiscovery` | |
| `TestPADRReplayDuringTeardownIsNotDeduped` | `internal/component/l2tp/pppoe/server_test.go` | R-2: a session in teardown is treated as absent rather than handed back | |
| `TestSessionTableIndexesEverySessionOfOneMAC` | `internal/component/l2tp/pppoe/session_test.go` | AC-3: the MAC index resolves every session of a MAC | |
| `TestSessionTableRemoveKeepsSiblingSessions` | `internal/component/l2tp/pppoe/session_test.go` | AC-4: removal is per session, and the MAC entry drops at zero | |
| `TestSessionTableChurnLeavesNoIndexEntries` | `internal/component/l2tp/pppoe/session_test.go` | R-3: add and remove 1000 sessions across 10 MACs, both maps return to size zero | |
| `TestPADTRemovesOnlyTheNamedSession` | `internal/component/l2tp/pppoe/server_test.go` | AC-5 | |
| `TestExtractParametersCarriesPerMACCap` | `internal/component/l2tp/pppoe/config_test.go` | AC-7: the default applies when the leaf is absent, and the per-interface value overrides the global one | |
| `TestPADILimiterRefusesPADRInSameWindow` | `internal/component/l2tp/pppoe/ratelimit_test.go` | A-1: records why the existing limiter is not reused for PADR | done |
| `TestValidateTree_PPPoEMaxSessionsPerMACRange` | `internal/component/config/validator_yang_test.go` | AC-8: `max-sessions-per-mac` outside its YANG range is refused with a message naming the leaf and the range | done |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `max-sessions-per-mac` | 1-65535 | 65535 | 0 | 65536 |
| sessions held by one MAC at admission | 0-cap | cap-1 admitted | N/A | cap refused |
| `max-sessions` interaction | 1-65535 | per-MAC cap above the interface cap yields the interface cap | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-per-mac-cap` | `test/pppoe/pppoe-per-mac-cap.ci` (corrected phase 4/5, 2026-09-09: NOT `test/plugin/` -- that suite is never routed into per-test netns mode, so a `netns-link` test there runs nowhere) | An operator sets the cap, reads it back, and sees the refusal counted | written, unexecuted -- this host cannot run PPPoE (`./le qemu pppoe-test` requires Linux) |
| `pppoe-padr-flood` | `test/pppoe/pppoe-padr-flood.ci` (corrected phase 4/5, 2026-09-09: NOT `test/qemu/`, which does not exist -- `test/pppoe/` plus `option=needs-linux`/`option=netns-link` is the actual convention every other PPPoE functional test in this repo uses) | A PADR flood from one MAC leaves descriptor use flat and a second subscriber still connects | written, unexecuted -- this host cannot run PPPoE (`./le qemu pppoe-test` requires Linux) |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-padr-replay` | `test/interop-pppoe/scenarios/pppoe-padr-replay` (corrected phase 5/5, 2026-09-09: no numeric prefix, per `ai/rules/interop-and-goal-validation.md` and the `pppoe-empty-service-name` precedent) | pppd (rp-pppoe) client (corrected phase 5/5, 2026-09-09: this lab's Ze-as-AC role always faces `pppd`; accel-ppp appears only as the AC when Ze is the client, in `01-pppoe-chap-ipv4`) | A replayed PADR gets the existing session ID and a second, distinct dial at the per-MAC cap is refused with a wire-level PADS error | written, unrun -- this host's Docker lab preflight refuses with `host kernel missing PPPoE requirements: pppoe (PPPoE pppox kernel module)`, and `./le qemu pppoe-test` exits 1 requiring Linux |
| `pppoe-chap-ipv4` | `test/interop-pppoe/scenarios/01-pppoe-chap-ipv4` | accel-ppp | The admission change did not break ordinary session setup | |

## Files to Modify
- `internal/component/l2tp/pppoe/server.go` - PADR admission: the per-MAC cap check, the state-independent dedup branch, and the repair of the `handlePADR` comment that wrongly claims the MAC dedup matches accel-ppp's cookie-keyed `find_channel`
- `internal/component/l2tp/pppoe/session.go` - the MAC index holds every session of a MAC; `Add` refuses to orphan; `Remove` maintains the set
- `internal/component/l2tp/pppoe/config.go` - carry the new leaf into the per-interface server parameters
- `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - the `max-sessions-per-mac` leaf, global and per interface
- `internal/component/l2tp/pppoe/drain.go` - the drain walk against a multi-session MAC index
- `docs/architecture/l2tp/bng-5-pppoe.md` - the design document `server.go`, `session.go`, `config.go`, `cookie.go`, `ratelimit.go`, `discovery.go` and `subsystem.go` declare: PADR admission, the per-MAC cap, and what the MAC index now holds
- `docs/architecture/l2tp/subscriber-session-model.md` - the design document `drain.go` declares: how the drain goroutines find a session now that one MAC resolves to several
- `docs/guide/pppoe.md` - the operator-facing leaf and the refusal an operator sees

## Files to Create
- `test/pppoe/pppoe-per-mac-cap.ci` - the operator path for the new leaf (corrected phase 4/5, 2026-09-09; see Functional Tests table)
- `test/pppoe/pppoe-padr-flood.ci` - AC-6's flood proof (`test/qemu/` does not exist; corrected phase 4/5, 2026-09-09)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang`: `max-sessions-per-mac`, global and per interface |
| YANG validation constraints | Yes | `range "1..65535"` on the new leaf, matching the `max-sessions` shape |
| YANG custom validators | N-A | A native range is sufficient; no cross-leaf constraint is needed because the interface `max-sessions` already bounds the total |
| CLI commands/flags | N-A | The leaf is configuration, reached through the existing config editor; no new verb |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | Yes | Automatic for a typed YANG leaf; no `CompleteFn` needed |
| Functional test for new RPC/API | Yes | `test/pppoe/pppoe-per-mac-cap.ci` |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | The leaf is under `pppoe`, not under `environment/` |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or sysctl: the descriptors this spec bounds are already opened by the current code |
| Prometheus counters/metrics | Yes | A refused-PADR counter, named and registered with the existing PPPoE metrics, so an operator can see a flood |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: the per-MAC session cap |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` if the PPPoE container is shown there; `docs/guide/pppoe.md` carries the leaf |
| 3 | CLI command added/changed? | No | No new verb |
| 4 | API/RPC added/changed? | No | No RPC change |
| 5 | Plugin added/changed? | No | The AC is a component subsystem, not a plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/pppoe.md` |
| 7 | Wire format changed? | No | The PADS error tag is already in the wire description |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2516.md`: the PADR admission and PADS error rows, with source anchors |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if a new QEMU scenario is registered |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: accel-ppp's own PADR handling is the comparison point |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/bng-5-pppoe.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | The refused-PADR counter, in the PPPoE telemetry section |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/immediate/spec-pppoe-padr-replay-allocates-unbounded-sessions.md`. `server.go`, `session.go`, `discovery.go`, `subsystem.go` and `ratelimit.go` all declare `docs/architecture/l2tp/bng-5-pppoe.md`, which this spec names and edits |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify the PPPoE examples in `docs/guide/pppoe.md` against the YANG after the leaf lands |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the cap reaches the admission point from configuration
   - Tests: `TestExtractParametersCarriesPerMACCap`, `TestPADRAtPerMACCapAnswersError`
   - Files: `yang/ze-pppoe-conf.yang`, `config.go`, `server.go`
   - Verify: the leaf validates and resolves, the admission branch exists as a stub, and the wiring test fails because the stub admits everything
2. **Phase: the MAC index holds every session** -- `Add` stops orphaning, `Remove` maintains the set, `lookupByMAC` answers for a set
   - Tests: `TestSessionTableIndexesEverySessionOfOneMAC`, `TestSessionTableRemoveKeepsSiblingSessions`, `TestSessionTableChurnLeavesNoIndexEntries`
   - Files: `session.go`, and every `lookupByMAC` caller found by `gopls references`
   - Verify: the churn test returns both maps to zero, and no caller reads a single session where a set now exists
3. **Phase: PADR admission** -- the cap decides, the dedup branch covers every live state, the refusal is a PADS error
   - Tests: `TestPADRAtPerMACCapAnswersError` (written RED in phase 1 and turned GREEN here, because the cap deciding is what makes it pass and it needs phase 2's multi-session index to count correctly), `TestPADRReplayAfterPPPReturnsExistingSID`, `TestPADRReplayDuringTeardownIsNotDeduped`, `TestPADTRemovesOnlyTheNamedSession`
   - Files: `server.go`, `drain.go`
   - Verify: a replay allocates nothing and a legitimate second session still succeeds under a cap above one
4. **Phase: the operator path** -- the functional test over the whole config-to-refusal chain
   - Tests: `pppoe-per-mac-cap.ci`
   - Files: `test/pppoe/pppoe-per-mac-cap.ci`
   - Verify: an operator sets the leaf, reads it back and sees refusals counted
   - The counter itself moved into phase 3, where the refusal branch lands. A refusal that increments nothing is the defect the Review Gate caught on the sibling spec, and separating them by a phase is what let it through
5. **Phase: the flood proof and the interop guard** -- the QEMU descriptor test and the replay scenario
   - Tests: `pppoe-padr-flood`, `pppoe-padr-replay`, `01-pppoe-chap-ipv4`
   - Files: `test/pppoe/pppoe-padr-flood.ci` (corrected phase 4/5, 2026-09-09: `test/qemu/` does not exist), `test/interop-pppoe/scenarios/`
   - Verify: revert the admission change, watch the flood test go red, restore it, watch it go green, and record that red
   - Phase 4/5 (subagent, 2026-09-09) wrote and recorded-red `pppoe-padr-flood.ci` (this host cannot execute it; see Goal Validation / closure notes) but did NOT write the `pppoe-padr-replay` interop scenario -- its brief scoped the phase to exactly two artifacts (the operator `.ci` and the flood `.ci`) and did not name it.
   - Phase 5/5 (subagent, 2026-09-09) wrote `test/interop-pppoe/scenarios/pppoe-padr-replay/` (role `ze-ac`, `max-sessions-per-mac 1`) and its checker, `internal/le/interoplab/pppoe/check_padr_replay.go`, registered in `checkers()` (`pppoe.go`). It asserts on captured discovery frames: a replayed PADR's PADS carries the SAME session id, and a second dial's PADS at the cap carries session id `0x0000` with an AC-System-Error tag. `01-pppoe-chap-ipv4` needed no new work -- it is the pre-existing Ze-as-client scenario the End-to-End User Stories table already cites for ordinary session setup. Neither scenario ran: this host's Docker lab preflight refuses (`host kernel missing PPPoE requirements: pppoe (PPPoE pppox kernel module)`), and `./le qemu pppoe-test` exits 1 requiring Linux, so the revert-to-red discrimination walk `ai/rules/interop-and-goal-validation.md` requires could not be performed here.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | The cap is reachable from configuration, enforced at admission, visible in a counter and provable in QEMU |
| Correctness | The refusal is a PADS error with session ID `0x0000`, never a dropped frame; a session in teardown is not handed back as live |
| Naming | The YANG leaf, its Go field and the counter name agree, and the leaf reads as a limit rather than as a rate |
| Data flow | Admission is decided in one place; no second cap check appears in the PPP driver or the drain path |
| Rule: `ai/rules/simplicity.md` | The cap is one leaf and one comparison. No new limiter type, no cookie state, no replay table |
| Rule: `ai/rules/principles.md` | A cap of zero must not read as "unlimited" by accident: the YANG range starts at 1, and the resolved value is never a bare zero standing for a decision |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The `max-sessions-per-mac` leaf exists and validates | `grep -n "max-sessions-per-mac" internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` |
| PADR admission calls the cap | `gopls references` on the cap field shows a caller in `handlePADR` |
| The MAC index holds several sessions | `TestSessionTableIndexesEverySessionOfOneMAC` passes |
| The flood test discriminates | The recorded red from the reverted-change run |
| The design page states the new admission rule | `grep -n "per-MAC" docs/architecture/l2tp/bng-5-pppoe.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The PADR's cookie, Service-Name and Host-Uniq are already bounds-checked; the cap must be applied before any allocation, never after |
| Resource exhaustion | Every allocation the PADR path performs (SID, socket, channel, unit) sits behind the cap, and the refusal path allocates nothing beyond the response frame |
| Fail-open guard | The cap is a guard: a resolution failure must refuse rather than admit, and a zero must not read as unlimited |
| Amplification | The PADS error is not larger than the PADR that provoked it, and the refusal path is rate limited |
| Error leakage | The PADS error tag names no internal state beyond the standard error tag |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The AC-Cookie answers "did this MAC complete a PADI round trip", and it was read as if it answered "is this PADR fresh". A stateless proof of a past event cannot bound a present allocation, so the bound belongs where the resource is taken.
- The orphaning defect is independent of the flood: a subscriber that reconnects normally after an unclean teardown also overwrites its own index entry.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Bound the resource, not the packet | Apply the existing `PADILimiter` to PADR | The limiter's per-MAC arm is a one-per-second dedup, so it would refuse the legitimate PADR that follows its own PADI inside the same second (A-1). A rate also still permits steady unbounded growth |
| A configurable per-MAC cap defaulting above one | A hard one-session-per-MAC rule | RFC 2516 places no per-peer session limit, so a hard rule refuses a conformant multi-session CPE (A-2) |
| Keep the cookie stateless | Make the AC-Cookie single-use | A single-use cookie needs a per-cookie table on the AC, which is the state the HMAC design exists to avoid, and it becomes its own exhaustion target. accel-ppp's `check_cookie` reaches the same conclusion: its cookie is replayable for its whole 5-second life and the dedup is the only replay bound |
| Keep the dedup keyed on the MAC | Key it on the cookie, as accel-ppp's `find_channel` does | MAC dedup bounds a replay that cookie dedup does not: a peer holding two valid cookies defeats the cookie key. The cost is that it would forbid concurrent sessions from one MAC, which the configurable cap defaulting above one gives back |
| Extend the dedup branch to every live state | Leave the branch at `StateDiscovery` and rely on the cap alone | The cap would refuse a legitimate retransmission from a MAC at the cap, where returning the existing session ID is what RFC 2516 expects the AC to do |

## Known Limitations
- The PADI path keeps its current limiter, so a flood of PADIs from many spoofed MAC addresses can still consume the per-interface budget and starve legitimate subscribers of a PADO. That is a different admission problem, and it is not addressed here.
- The cap counts sessions, not bandwidth or CPU, so a subscriber inside the cap can still churn sessions at the rate the PADI limiter allows.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT. The admission refusal
cites RFC 2516 Section 5.4 for the PADS error form, and the retransmission
answer cites the same section for the unique session identifier.

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
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
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `max-sessions-per-mac`, a `uint16` leaf with `range "1..65535"`, global (default 8) and per interface (override), in `yang/ze-pppoe-conf.yang` under a new `revision 2026-09-09`.
- `ExtractParameters` (`config.go`) resolves it global-then-override into `Parameters.MaxSessionsPerMAC` and `InterfaceConfig.MaxSessionsPerMAC`, and `Subsystem.Start` (`subsystem.go`) carries it into `InterfaceServer.maxSessionsPerMAC`.
- `admitPerMACCap` (`server.go`) refuses a PADR whose source MAC already holds the cap, answering with `BuildPADSError` carrying `TagACSystemError` and SESSION_ID `0x0000`, allocating nothing, and counting `ze_pppoe_discovery_refusals_total{reason="per-mac-cap-reached"}` through the existing closed reason set and `countRefusal`.
- `SessionTable.byMAC` is now `map[[6]byte]map[uint16]*Session`. `Add` joins the set instead of overwriting, `Remove` drops the entry at zero, and `sessionsByMAC` returns every session a MAC holds.
- The PADR dedup branch is rekeyed onto the AC-Cookie (`SessionTable.matchLiveCookie`), answers in every live session state, breaks a tie on the lowest SID so the answer never depends on map iteration order, and excludes `StateTeardown`.
- `Session.Cookie` stores the admitting PADR's AC-Cookie so the correlation above has something to compare against.
- `SessionTable.markTeardown` and `SessionTable.markSession` put every write of `Session.State` under the table's lock. `handleSessionDown` marks teardown, `handlePADR` marks the session live, and `markSession` refuses to move a session out of `StateTeardown`.
- Tests: `TestPADRAtPerMACCapAnswersError`, `TestPADRReplayAfterPPPReturnsExistingSID`, `TestPADRReplayDuringTeardownIsNotDeduped`, `TestPADTRemovesOnlyTheNamedSession`, `TestSessionTableIndexesEverySessionOfOneMAC`, `TestSessionTableRemoveKeepsSiblingSessions`, `TestSessionTableChurnLeavesNoIndexEntries`, `TestSessionTableSessionsByMAC`, `TestMarkSessionDoesNotResurrectATeardownSession`, `TestExtractParametersCarriesPerMACCap`, `TestPADILimiterRefusesPADRInSameWindow`, `TestValidateTree_PPPoEMaxSessionsPerMACRange`.
- Written and NEVER EXECUTED on this host: `test/pppoe/pppoe-per-mac-cap.ci`, `test/pppoe/pppoe-padr-flood.ci`, their fixtures under `internal/test/fixture/`, `test/interop-pppoe/scenarios/pppoe-padr-replay/`, and its checker `internal/le/interoplab/pppoe/check_padr_replay.go`.
- `interoplab.SendFrameInNamespace` (`internal/le/interoplab/rawframe_linux.go`, with a `!linux` stub beside it) is the namespace-entering AF_PACKET injector extracted from the BGP lab's IS-IS purge sender, which now delegates to it.

### Bugs Found/Fixed
- **`Session.State` was written outside the lock its two other accessors take.** `handlePADR` set `sess.State = StateSession` as a bare field write while `markTeardown` (PPP event-consumer goroutine) wrote it and `matchLiveCookie` read it under `st.mu`. R-2's new guard, which excludes a `StateTeardown` session from the dedup answer, therefore rested on an unsynchronized field: a SID freed by `Remove` is reallocated at once, so a session-down event still queued for that SID's previous holder reaches the next one while `handlePADR` is still setting it up. Found by the closure review, fixed with `SessionTable.markSession`, and covered by `TestMarkSessionDoesNotResurrectATeardownSession`, which was observed RED against a `markSession` that overwrote any state.
- **`TestPADRAtPerMACCapAnswersError`'s doc comment described the phase-1 stub as current** ("this fails today"). Corrected to state the break that reproduces the red, and that break was then performed and observed.
- **`markTeardown` and `markSession` were exported with no cross-package caller**, adding two rows to `./le repository check`. Unexported, taking the check from 83 to 81 issues (the remaining 81 are other sessions' and predate this spec).

### Documentation Updates
- `docs/architecture/l2tp/bng-5-pppoe.md`: five new statements (a MAC holds several sessions, PADR admission is bounded by count, why `PADILimiter` is not reused, the cookie-keyed dedup, and every `Session.State` access under the table's lock), with `<!-- source: -->` anchors on `session.go` and `server.go`. The refusal reason set and the refusal call-site count were updated in place.
- `docs/guide/pppoe.md`: the leaf in the config example, a "Per-MAC session cap" bullet, and the `per-mac-cap-reached` row in the refusal-reason table.
- `docs/labs/pppoe-interop.md`: the `pppoe-padr-replay` scenario, its checker file, its `ZE_PPPOE_INTEROP_SCENARIO` invocation, and what it proves.
- `./le doc check verify` exits 1 on 8318 findings, every one a command-catalog HTML index row absent from the live catalog. It names none of the three pages above and none of the files this spec changed; this spec adds no command. Left red (`ai/rules/pre-release.md`).

### Deviations from Plan
- The dedup branch is keyed on the AC-Cookie rather than on the MAC. The spec's Key Design Decisions row "Keep the dedup keyed on the MAC" was written before the cap existed; once a MAC may legitimately hold several sessions, MAC-and-state alone answers a genuinely distinct second PADR with the first session's SID. The cookie is the only thing that tells the two apart. Recorded in the Mistake Log.
- The spec's comment repair (the false "Matches accel-ppp's find_channel check" claim) landed as part of that rekey rather than as a separate edit, because the rekey made Ze's dedup actually cookie-keyed, which is what accel-ppp does.
- `internal/component/l2tp/pppoe/drain.go` is named in "Files to Modify" and was NOT modified. A-4 established that `drain.go` never touches `SessionTable`: it keys on `(TunnelID, SessionID)` through the PPP driver, so the MAC index change cannot reach it.
- `interoplab.SendFrameInNamespace` and its `isis_inject_linux.go` delegation were not planned. The interop checker needs to replay a captured frame from inside the client container's namespace, the BGP lab already held that mechanism, and copying it would have been a second declaration of one fact (`ai/rules/principles.md`).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The spec planned to keep the PADR dedup keyed on the MAC and rely on the cap for the rest | With a cap above one, a MAC may hold several legitimate sessions, so MAC-and-state answers a distinct second PADR with the first session's SID | Implementation, while writing `TestPADRReplayAfterPPPReturnsExistingSID` beside `TestPADRAtPerMACCapAnswersError` | Rekeyed onto the AC-Cookie (`matchLiveCookie`), with the tie broken on the lowest SID because a cookie's HMAC is bucketed to the second |
| escalation | A new guard (`matchLiveCookie` excluding `StateTeardown`) was written with two of the three accessors of `Session.State` under the lock, and the third left as a bare field write | Adding a lock to two accessors of a field does not synchronize it; the third writer is what decides | Closure review, reading every `.State` site in the package rather than the diff | `markSession` added, `handlePADR` routed through it, and the fact stated on the design page. Journal row in `plan/journal/false-synchronization-claim.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| An unauthenticated host must not make the AC allocate session resources without bound | Done | `admitPerMACCap`, `handlePADR` (`internal/component/l2tp/pppoe/server.go`) | Bounded at 8 sessions per MAC by default; the replay path is absorbed by `matchLiveCookie` before the cap is consulted |
| A session must never become unreachable from the MAC index while it still holds descriptors | Done | `SessionTable.Add`, `Remove`, `sessionsByMAC` (`session.go`) | `Add` joins the per-MAC set; the orphaning overwrite is gone |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPADRAtPerMACCapAnswersError` | PADS, SID 0, `TagACSystemError`, `Count() == 1`, counter scraped |
| AC-2 | Done | `TestPADRReplayAfterPPPReturnsExistingSID` | Session in `StateSession`; reply carries the existing SID |
| AC-3 | Done | `TestSessionTableIndexesEverySessionOfOneMAC` | Both sessions resolve by MAC and by SID |
| AC-4 | Done | `TestSessionTableRemoveKeepsSiblingSessions` | Sibling survives; the MAC entry disappears at zero |
| AC-5 | Done | `TestPADTRemovesOnlyTheNamedSession` | Driven through `handlePADT`, the real entry point |
| AC-6 | **Not demonstrated** | `test/pppoe/pppoe-padr-flood.ci` written, NOT EXECUTED | This host is darwin: `./le qemu pppoe-test` exits 1 with "qemu guest evidence requires Linux". No descriptor measurement exists |
| AC-7 | Done | `TestExtractParametersCarriesPerMACCap` | Default applies when absent; per-interface overrides global |
| AC-8 | Done | `TestValidateTree_PPPoEMaxSessionsPerMACRange` | YANG `range "1..65535"` refuses 0 and 65536 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPADRAtPerMACCapAnswersError` | Done | `server_test.go` | RED reproduced at closure by disabling the `admitPerMACCap` call |
| `TestPADRReplayAfterPPPReturnsExistingSID` | Done | `server_test.go` | |
| `TestPADRReplayDuringTeardownIsNotDeduped` | Done | `server_test.go` | |
| `TestSessionTableIndexesEverySessionOfOneMAC` | Done | `session_test.go` | |
| `TestSessionTableRemoveKeepsSiblingSessions` | Done | `session_test.go` | |
| `TestSessionTableChurnLeavesNoIndexEntries` | Done | `session_test.go` | 1000 cycles over 10 MACs; both maps return to zero |
| `TestPADTRemovesOnlyTheNamedSession` | Done | `server_test.go` | |
| `TestExtractParametersCarriesPerMACCap` | Done | `config_test.go` | |
| `TestPADILimiterRefusesPADRInSameWindow` | Done | `ratelimit_test.go` | |
| `TestValidateTree_PPPoEMaxSessionsPerMACRange` | Done | `internal/component/config/validator_yang_test.go` | |
| `TestMarkSessionDoesNotResurrectATeardownSession` | Changed | `session_test.go` | Added at closure for the guard the review found; not in the original plan |
| `pppoe-per-mac-cap` | Partial | `test/pppoe/pppoe-per-mac-cap.ci` | Written, never executed: no Linux host |
| `pppoe-padr-flood` | Partial | `test/pppoe/pppoe-padr-flood.ci` | Written, never executed: no Linux host |
| `pppoe-padr-replay` | Partial | `test/interop-pppoe/scenarios/pppoe-padr-replay/` | Written, never run: the Docker preflight refuses with `host kernel missing PPPoE requirements: pppoe (PPPoE pppox kernel module)` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/l2tp/pppoe/server.go` | Done | Cap, cookie-keyed dedup, `markSession`, comment repair |
| `internal/component/l2tp/pppoe/session.go` | Done | Set-valued MAC index, `Cookie`, `matchLiveCookie`, `markTeardown`, `markSession` |
| `internal/component/l2tp/pppoe/config.go` | Done | Leaf resolution |
| `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` | Done | Leaf, global and per interface, with a revision |
| `internal/component/l2tp/pppoe/drain.go` | Changed | Not modified: A-4 showed `drain.go` never touches `SessionTable` |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Done | |
| `docs/architecture/l2tp/subscriber-session-model.md` | Changed | Not modified, for the same reason as `drain.go` |
| `docs/guide/pppoe.md` | Done | |
| `test/pppoe/pppoe-per-mac-cap.ci` | Done | Created |
| `test/pppoe/pppoe-padr-flood.ci` | Done | Created |

### Audit Summary
- **Total items:** 34
- **Done:** 27
- **Partial:** 4 (the three unexecuted `.ci` and interop artifacts and AC-6, all blocked by this host being darwin, all reported rather than traded away)
- **Skipped:** 0
- **Changed:** 3 (`drain.go`, `subscriber-session-model.md`, and one test added at closure) -- recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An unauthenticated host on an access interface cannot make the AC allocate session resources without bound | functional (flood) | **NOT ACHIEVED AS PROOF.** `test/pppoe/pppoe-padr-flood.ci` asserts `process_open_fds` grows by at most one session's worth across 10000 replays, and it has NEVER RUN: `./le qemu pppoe-test` exits 1 with "qemu guest evidence requires Linux" on this darwin host. What exists is the unit-level bound: `TestPADRReplayAfterPPPReturnsExistingSID` proves a replayed cookie allocates nothing (`Count()` stays at one after `handlePADR`), and `TestPADRAtPerMACCapAnswersError` proves a distinct PADR at the cap allocates nothing. Neither measures descriptors, and neither runs the AC end to end. The vulnerability is bounded on paper and in unit tests, and is NOT proven closed |
| A session must never become unreachable from the MAC index while it still holds descriptors | unit | `TestSessionTableIndexesEverySessionOfOneMAC`: a second `Add` for one MAC leaves both sessions resolvable by MAC and by SID, where the old single-pointer assignment dropped the first. `TestSessionTableChurnLeavesNoIndexEntries`: 1000 add/remove cycles over 10 MACs leave both maps at size zero, so the set-valued index leaks nothing |
| A retransmitted PADR is answered rather than allocated, in every live session state | unit + interop (unrun) | `TestPADRReplayAfterPPPReturnsExistingSID` (green). The wire-level proof, `test/interop-pppoe/scenarios/pppoe-padr-replay` against a real pppd client, is WRITTEN AND UNRUN: this host's Docker lab preflight refuses with `host kernel missing PPPoE requirements: pppoe (PPPoE pppox kernel module)`. The revert-to-red discrimination walk `ai/rules/interop-and-goal-validation.md` requires was therefore not performed for that scenario |
| The refusal is a PADS error, never a dropped frame | unit | `TestPADRAtPerMACCapAnswersError` parses the emitted frame: `CodePADS`, session id zero, `FindTag(TagACSystemError)` non-nil |
| A session in teardown is never handed back to a concurrent PADR | unit, discriminated | `TestPADRReplayDuringTeardownIsNotDeduped` and `TestMarkSessionDoesNotResurrectATeardownSession`. The second was observed RED against a `markSession` that overwrote any state, failing on both its assertions |
| The cap is reachable from operator configuration | unit + functional (unrun) | `TestExtractParametersCarriesPerMACCap` and `TestValidateTree_PPPoEMaxSessionsPerMACRange` are green. The operator path end to end, `test/pppoe/pppoe-per-mac-cap.ci`, is WRITTEN AND UNEXECUTED for the same host reason |

**Honest verdict.** Every unit-level goal is proven and two of them were proven by an observed red. The two goals that need a running AC, AC-6's descriptor-flat flood proof and the wire-level interop replay, have artifacts and no execution, and this closure does not claim otherwise. Nothing was weakened, deleted, or marked N/A to reach a green.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| AC-6's flood proof and the two functional `.ci` runs | This host is darwin. `./le qemu pppoe-test` exits 1 with "qemu guest evidence requires Linux", and the Docker PPPoE lab preflight refuses with `host kernel missing PPPoE requirements: pppoe (PPPoE pppox kernel module)`. The artifacts are committed and correct; only their execution is owed | No new spec: the artifacts exist and are registered (`netnsSelections` in `internal/le/qemu/netns_linux.go`, and the `checkers()` map in `internal/le/interoplab/pppoe/pppoe.go`), so a run on a Linux host executes them with no further work. This is verification debt, not a missing deliverable |
| The `pppoe-padr-replay` discrimination walk (revert, rebuild the image, observe red) | Same host limitation: the scenario cannot start, so no red can be observed | Same as above |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/pppoe-padr-replay-allocates-unbounded-sessions-8a072de8-ce8e-47de-bdd2-015f5b44ca51.md` (31 files, verdict=clean) |
| `./le spec session review check` | clean, hashes match |
| Rounds | 3 |
| Reviewer lenses used | Round 1: (a) concurrency and guard correctness, reading every producer of `Session.State`, `byMAC` and the admission branch rather than their callers; (b) Ze Go style over every changed Go file (`docs/contributing/ze-go-style.md`), plus the `SendFrameInNamespace` extraction reviewed for a dropped guard. Round 2: the fixes round 1 made, plus `./le repository check` over the new exported surface. Round 3: the round-2 rename and the design-page paragraph, re-verified against `SessionTable.Remove` |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `Session.State` written outside the lock its two other accessors take, so R-2's new `StateTeardown` exclusion rests on an unsynchronized field. A SID freed by `Remove` is reallocated at once, so a queued session-down event for the previous holder can reach the new session while `handlePADR` is still setting it up | `handlePADR` (`internal/component/l2tp/pppoe/server.go`), `SessionTable` (`session.go`) | `SessionTable.markSession`, which writes under `st.mu` and refuses to move a session out of `StateTeardown`. Every `Session.State` access now sits in `session.go` or `snapshot.go`, all under the lock. Covered by `TestMarkSessionDoesNotResurrectATeardownSession`, observed RED |
| 2 | ISSUE | `TestPADRAtPerMACCapAnswersError`'s doc comment stated the test "fails today" and that `admitPerMACCap` always admits, describing the phase-1 stub as the current tree (`ai/rules/stale-comments.md`) | `server_test.go` | Comment rewritten to name the break that reproduces the red; the break was then performed and the red observed |
| 3 | ISSUE | `markTeardown` (this spec) and `markSession` (fix 1) were exported with no cross-package caller, adding two rows to `./le repository check` | `session.go` | Unexported. The check went from 83 to 81 issues; the design page's symbol names and anchor were updated in the same edit |

### Findings recorded, not fixed (NOTE)
| # | Finding | Location | Why not fixed |
|---|---------|----------|---------------|
| 1 | `waitFixed` is a fixed 5-second sleep in two places that decide when to stop the capture. The sibling checker `checkZeAccessConcentratorEmptyServiceName` waits on state (`waitZeSession`, `checkLCPAuthIPCP`) and never sleeps, and for the over-cap dial a pollable condition exists: the `ze_pppoe_discovery_refusals_total{reason="per-mac-cap-reached"}` counter this spec added | `check_padr_replay.go`, `checkReplayReturnsExistingSID` and `checkSecondDialAtCapIsRefused` | The scenario cannot execute on this host, so a change to its timing could not be verified either way. Named here with its fix so the first Linux run has it |
| 2 | `admitPerMACCap` calls `sessionsByMAC`, which allocates a slice, only to take its length. It is the sole production caller | `admitPerMACCap` (`server.go`), `sessionsByMAC` (`session.go`) | PPPoE discovery is control plane, and `sessionsByMAC`'s set-returning shape is what AC-3, AC-4 and AC-5 assert against. A `countByMAC` beside it would add machinery for one small allocation on a non-data-plane path |
| 3 | The per-interface `max-sessions-per-mac` `ze:help` says Ze takes the global value when the leaf is "absent or 0", but the YANG `range "1..65535"` refuses 0, so no validated configuration reaches that arm | `yang/ze-pppoe-conf.yang` | `ExtractParameters` genuinely handles 0 defensively, and the sibling `max-sessions` leaf is worded the same way. Changing one of the pair without the other would make them disagree |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/pppoe/pppoe-per-mac-cap.ci` | Yes | `ls -l`: 2.4K, Sep 9 06:39 |
| `test/pppoe/pppoe-padr-flood.ci` | Yes | `ls -l`: 2.8K, Sep 9 06:41 |
| `test/interop-pppoe/scenarios/pppoe-padr-replay/role` | Yes | `ls -l`: 6 bytes, Sep 9 07:39 |
| `test/interop-pppoe/scenarios/pppoe-padr-replay/ze.conf` | Yes | `ls -l`: 1.5K, Sep 9 07:39 |
| `internal/le/interoplab/pppoe/check_padr_replay.go` | Yes | `ls -l`: 13K, Sep 9 07:39 |
| `internal/le/interoplab/rawframe_linux.go` | Yes | `ls -l`: 4.5K, Sep 9 07:26 |
| `internal/le/interoplab/rawframe_other.go` | Yes | `ls -l`: 507 bytes, Sep 9 07:26 |
| `internal/test/fixture/tunnel_fixture_pppoe_per_mac_cap_linux.go` | Yes | `ls -l`: 4.0K, Sep 9 07:07 |
| `internal/test/fixture/tunnel_fixture_pppoe_padr_flood_linux.go` | Yes | `ls -l`: 7.7K, Sep 9 06:41 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Refusal is a PADS error, session id zero, nothing allocated, counted | `--- PASS: TestPADRAtPerMACCapAnswersError (0.00s)`; RED reproduced by replacing the `admitPerMACCap` call's condition with a constant false: `handlePADR sent 0 frame(s) for a PADR at the per-MAC cap, want exactly 1` |
| AC-2 | A replay past `StateDiscovery` gets the existing session id | `--- PASS: TestPADRReplayAfterPPPReturnsExistingSID (0.00s)` |
| AC-3 | A MAC lookup resolves both sessions | `--- PASS: TestSessionTableIndexesEverySessionOfOneMAC (0.00s)` |
| AC-4 | Removal is per session; the MAC entry drops at zero | `--- PASS: TestSessionTableRemoveKeepsSiblingSessions (0.00s)` |
| AC-5 | PADT tears down only the named session | `--- PASS: TestPADTRemovesOnlyTheNamedSession (0.00s)` |
| AC-6 | Descriptor count flat under 10000 replays | **No evidence.** `./le qemu pppoe-test` exits 1: "qemu guest evidence requires Linux" |
| AC-7 | The default applies when the leaf is absent | `--- PASS: TestExtractParametersCarriesPerMACCap (0.11s)` |
| AC-8 | Out-of-range value refused at validation | `--- PASS: TestValidateTree_PPPoEMaxSessionsPerMACRange (0.19s)` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A PADR from a MAC at the cap | `test/pppoe/pppoe-per-mac-cap.ci` | Read: sets `max-sessions-per-mac 1`, runs `ze-test fixture pppoe/per-mac-cap`, expects the refusal line, then reads `ze_pppoe_discovery_refusals_total{reason="per-mac-cap-reached"} 1` off `/metrics`. NOT EXECUTED (no Linux host) |
| A PADR replayed after PPP | `test/pppoe/pppoe-padr-flood.ci` | Read: 10000 replays, asserts `process_open_fds` growth and a post-flood PADI answer. NOT EXECUTED (no Linux host) |
| `max-sessions-per-mac` in the config tree | (unit) | `TestExtractParametersCarriesPerMACCap` drives `ExtractParameters` over real config text through `extractFromConfigText` |
| A second session added for one MAC | (unit) | `TestSessionTableIndexesEverySessionOfOneMAC` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestPADILimiterRefusesPADRInSameWindow`: the second `Check(mac)` in one window returns false. Broken with a constant false, watched fail, restored |
| A-2 | confirmed | RFC 2516 places no per-peer session limit; recorded in Required Reading from `rfc/full/rfc2516.txt` |
| A-3 | confirmed | Read at the producers: `pppoeCreate` (`kernel_linux.go`) opens one `AF_PPPOX` socket; `ppp.DevPPPSetup` (`internal/component/l2tp/ppp/devppp_linux.go`) opens `/dev/ppp` twice. Three descriptors per locally-terminated session |
| A-4 | confirmed | `gopls references`: one production caller of `lookupByMAC` (the PADR dedup branch), zero callers of `byMAC` outside `session.go`. `drain.go` keys on `(TunnelID, SessionID)` and never touches `SessionTable` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "the eight call sites that count a refusal" (`bng-5-pppoe.md` anchor) | `grep -c "countRefusal(" server.go` returns 8 | Yes |
| The refusal reason set includes `per-mac-cap-reached` | `metrics.go`: `reasonPerMACCapReached` sits in the same const block as the other five reasons | Yes |
| `docs/guide/pppoe.md` config example shows the leaf global and per interface | YANG: global `default 8`, `range "1..65535"` in both places, so the example's values validate | Yes |
| "every `Session.State` access goes through the table's lock" (`bng-5-pppoe.md`) | A grep for `.State` outside tests returns only `session.go` and `snapshot.go`, and every enclosing function there takes `st.mu` | Yes |
| Symbol names on the design page and its anchor | `markTeardown`, `markSession`, `matchLiveCookie`, `Session.Cookie`, `admitPerMACCap` all resolve in `session.go` and `server.go` | Yes |
| Categories 3 (CLI), 4 (RPC), 5 (plugin), 7 (wire format), 8 (SDK), 13 (route metadata), 15 (registered inventory): No | This spec adds one YANG leaf and no verb, no RPC, no event, no capability. The diff names no command module | Yes |
| Category 9 (RFC status) | `rfc/short/rfc2516.md` is UNMODIFIED: this spec adds a Ze policy cap, and RFC 2516 Section 9 only names the concept without setting a bound, so no requirement's support level changed. `git status rfc/short/` is clean | Yes |
| `./le doc check verify` | Exits 1 on 8318 command-catalog HTML index findings, none naming a file this spec touched (a grep for the three page names and the leaf over the log returns nothing) | Yes, red is elsewhere |

## Core Insight

Adding a lock to two of a field's three accessors does not synchronize the field, and the two that took it read as proof that it is synchronized. `matchLiveCookie` and `markTeardown` were written together, each carrying a comment about the lock they share, and that pair is exactly what stopped anyone looking for the third writer. The check that finds it is not "did I lock this new code" but "list every access to this field", which is a grep over the package rather than a read of the diff.
