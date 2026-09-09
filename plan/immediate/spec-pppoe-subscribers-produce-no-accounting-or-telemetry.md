# Spec: pppoe-subscribers-produce-no-accounting-or-telemetry

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**A PPPoE subscriber generates no RADIUS accounting record and no per-session
metric. An L2TP subscriber generates both, from the same collection.**

The counters exist and work. `iface.GetStats` reads the per-session `pppN`
netdev statistics over netlink, and `(*baselineStore).applyBaseline`
(`internal/component/iface/counters.go`) subtracts the interface's starting
values so a reused unit number cannot inflate a new session. Three consumers read
that: `buildAcctPacket`, which puts real Acct-Input-Octets, Acct-Output-Octets,
the packet counts and the RFC 2869 Gigawords into every Accounting-Request; the
`l2tpStatsPoller`, which publishes `ze_l2tp_session_rx_bytes_total` and its
siblings; and `show l2tp session traffic`.

Two wires reach only L2TP. `(*radiusAcct).subscribeEventBus`
(`internal/component/l2tp/plugins/authradius/acct.go`) subscribes to
`l2tpevents.SessionIPAssigned` and `l2tpevents.SessionDown` and to nothing else,
while the PPPoE subsystem emits `subevents.SessionUp`, `SessionIPAssigned` and
`SessionDown`. And `(*Subsystem).Start`
(`internal/component/l2tp/subsystem.go`) builds the poller with
`newL2TPStatsPoller(s.reactors, pollInterval)`, over the L2TP reactor list, which
holds no PPPoE session.

So a PPPoE subscriber is invisible to billing and to monitoring, while
`docs/features/rfc-status.md` publishes RFC 2866 as "Supported for subscriber
access" and RFC 2869's Gigaword counters likewise. One of the two subscriber
access types Ze presents as first-class produces nothing, which makes those rows
over-claim. `ai/rules/rfc-compliance.md` is explicit that a public row which
over-claims is repaired by PROVING what it says, so this spec implements the
missing half rather than lowering the rows.

**The unifying abstraction already exists and already carries what both consumers
need.** `subscriber.Session` (`internal/component/l2tp/subscriber/session.go`)
holds `AccessType`, `PppInterface`, `Username`, `IPv4Addr` and the identity
fields for both access types, and `subscriber.DefaultRegistry` is populated by
the PPPoE subsystem and the L2TP reactor alike. The repair is to consume that
layer rather than to add a second path beside it.

A second defect sits inside the working half. In `buildAcctPacket`, a failed
`acctGetStats` logs a warning and then appends `Acct-Input-Octets = 0` anyway, so
a billing system cannot tell a failed counter read from an idle subscriber. That
is a value that is silently wrong (`ai/rules/principles.md`), and it is repaired
here because this work makes the same path serve twice as many sessions.

The goal: a PPPoE subscriber produces the same accounting records and the same
per-session metrics an L2TP subscriber does, from one code path rather than two,
and a counter Ze could not read is never reported as a zero.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - the design document `subscriber/session.go`, `subscriber/events/events.go` and `subscriber/cmd/subscriber.go` declare
  → Decision: `subscriber.Session` is the access-type-neutral record, and `subscriber.DefaultRegistry` is the one place both access types register. Anything that must serve both belongs here, not beside it
  → Constraint: `PppInterface` is on that record, so the netdev name every counter read needs is already access-type-neutral. No new plumbing carries it
  → Constraint: the page is silent that accounting and the stats poller are still L2TP-only, so it gains that statement, and loses it again when this work lands
- [ ] `docs/architecture/l2tp/l2tp-10-metrics.md` - the design document `internal/component/l2tp/metrics.go` declares: the poller, its interval and its series
  → Constraint: the existing series are named `ze_l2tp_session_*` and labelled per session. Renaming a published series is a break; extending it to a second access type is not. Decide deliberately which this work does
- [ ] `docs/research/l2tpv2-ze-integration.md` - the design document `acct.go` and `internal/component/l2tp/subsystem.go` declare
  → Constraint: accounting is a plugin over the event bus, so moving which events it subscribes to is a change inside that plugin and touches no reactor
- [ ] `docs/architecture/l2tp/bng-1-radius-attributes.md` - what an Accounting-Request carries
  → Constraint: its "What every Accounting-Request carries" table omits the octets, packets and Gigawords rows that `buildAcctPacket` actually sends, so the page is already wrong and this work corrects it
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - the design document the PPPoE subsystem declares
  → Constraint: the PPPoE session's rate profile already reaches `subscriber.Session` through the shared metadata store "with no new plumbing and no second attribute path". That precedent is the shape this spec follows for counters

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2866.md` - RADIUS Accounting: Start, Stop and Interim-Update
  → Constraint: the Support row reads "Supported for subscriber access". PPPoE is a subscriber access type and produces no record, so the row is an over-claim this work repairs by implementing (`ai/rules/rfc-compliance.md`)
  → Constraint: RFC 2866 Section 5.1 defines Acct-Status-Type, and Section 5.10 the Acct-Terminate-Cause the Stop record already carries for L2TP. A PPPoE Stop owes the same cause, derived from the PPPoE teardown reason
- [ ] `rfc/short/rfc2869.md` - the Gigaword counters
  → Constraint: `splitGigawords` emits Input and Output Gigawords only when non-zero, which RFC 2869 Sections 5.1 and 5.2 permit. That behavior is correct and is preserved

**Key insights:**
- Nothing is missing from the collection. Every number this spec needs is already read, baseline-corrected and consumed three times over for one access type.
- The two wires that stop at L2TP are an event subscription and a constructor argument. Neither is a deep coupling.
- The zero-on-error is inside the path that is about to carry twice the traffic, which is why it is repaired here rather than journaled.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `subscribeEventBus` subscribes to `l2tpevents.SessionIPAssigned` and `l2tpevents.SessionDown` only; `onSessionIPAssigned` starts accounting and `onSessionDown` stops it; `buildAcctPacket` reads `acctGetStats` (= `iface.GetStats`) when the session's PPP interface is non-empty, and on error logs a warning and leaves the counts at zero before appending them; `splitGigawords` emits the RFC 2869 attributes only above 2^32
- [ ] `internal/component/l2tp/subscriber/events/events.go` - `SessionUp`, `SessionDown`, `SessionIPAssigned`, `SessionRateChange` and `SessionAuthResult`, each carrying a `subscriber.Session` or its id, in the `subscriber` namespace
- [ ] `internal/component/l2tp/subscriber/session.go` - `Session` carries `ID`, `AccessType`, `State`, `Username`, `PppInterface`, `IPv4Addr`, `IPv6Prefix`, the PPPoE identity pair (`PPPoESID`, `AccessIfIndex`) and the L2TP pair (`TunnelID`, `SessionID`)
- [ ] `internal/component/l2tp/metrics.go` - `l2tpStatsPoller`, `newL2TPStatsPoller`, `(*l2tpStatsPoller).poll` reading per-session stats and publishing `sessionRxBytesTotal`, `sessionUptimeSecs` and their siblings
- [ ] `internal/component/l2tp/subsystem.go` - `(*Subsystem).Start` constructs the poller over `s.reactors`
- [ ] `internal/component/l2tp/pppoe/subsystem.go` - `onSessionUp`, `onSessionIPAssigned` and `onSessionDown` update `subscriber.DefaultRegistry` and emit the `subevents` family
- [ ] `internal/component/iface/counters.go` - `(*baselineStore).applyBaseline` subtracts the interface's starting counters so a reused `pppN` index does not inflate a new session
- [ ] `internal/component/iface/dispatch.go` - `GetStats`, the access-type-neutral read every consumer already uses

**Behavior to preserve:**
- An L2TP subscriber's accounting records and metrics are byte-for-byte what they are today. This work adds a second producer, it does not re-shape the first.
- Baseline subtraction stays: a reused `pppN` unit must never inflate a new session's counts.
- Gigawords stay conditional on exceeding 2^32, per RFC 2869 Sections 5.1 and 5.2.
- The accounting plugin stays a plugin over the event bus and gains no reactor knowledge.
- `show l2tp session traffic` keeps working and keeps its output shape.

**Behavior to change:**
- Accounting subscribes to the subscriber event family, so a PPPoE session produces Start, Interim-Update and Stop records with real counters.
- The stats poller enumerates the subscriber registry rather than the L2TP reactor list, so a PPPoE session appears in the per-session series.
- A failed counter read omits the octet and packet attributes rather than sending zeros, and says so.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A subscriber session reaching the up state: for PPPoE, `handlePADR` through the PPP driver to `(*Subsystem).onSessionUp`; for L2TP, the reactor's session establishment. Both register a `subscriber.Session` in `subscriber.DefaultRegistry` and emit their event family.
- Format at entry: a `subscriber.Session` value carrying `AccessType`, `PppInterface` and the access-type-specific identity fields.

### Transformation Path
1. The access layer registers the session and emits `subevents.SessionUp`, then `SessionIPAssigned` once addresses are negotiated.
2. The accounting plugin's subscription starts an accounting session and sends Start.
3. On the interim timer and at teardown, `buildAcctPacket` reads `iface.GetStats` for the session's PPP interface, baseline-corrected, and encodes the RADIUS attributes.
4. Independently, the stats poller walks the registry on its interval and publishes the Prometheus series per session.
5. `subevents.SessionDown` stops accounting and sends the Stop record with the terminate cause.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Access layer → subscriber registry | `subscriber.DefaultRegistry` add and get, already done by both access types | No |
| Access layer → accounting plugin | The `subscriber` event namespace over the plugin event bus | No |
| Ze → kernel | `iface.GetStats` reading the `pppN` netdev statistics over netlink | No |
| Ze → RADIUS server | Accounting-Request UDP, attributes built by `buildAcctPacket` | No |
| Ze → operator | The Prometheus series and the existing traffic command | No |

### Integration Points
- `(*radiusAcct).subscribeEventBus` (`acct.go`) - the one subscription change.
- `newL2TPStatsPoller` (`internal/component/l2tp/metrics.go`) - its source becomes the registry; its name stops being accurate and is decided in phase 1.
- `subscriber.DefaultRegistry` - the enumeration both consumers move onto.
- `iface.GetStats` and `applyBaseline` - unchanged, and already access-type-neutral.

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
| A-1 | A PPPoE session's `pppN` interface name reaches `subscriber.Session.PppInterface` before the first accounting record is due | The field exists on the shared record and the PPPoE subsystem populates the session | Accounting for PPPoE reads an empty name and produces the zeros this spec exists to remove | Read `(*Subsystem).onSessionUp` and `onSessionIPAssigned` in `pppoe/subsystem.go` and confirm the assignment and its ordering | unvalidated |
| A-2 | The `subscriber` event family is emitted for L2TP as well as PPPoE, so one subscription can replace the `l2tpevents` one rather than being added beside it | Both access types register in `subscriber.DefaultRegistry` | Two subscriptions are needed and the L2TP path keeps its own, which leaves the duplication this spec is removing | `gopls references` on `subevents.SessionUp` and `SessionDown`, and read the L2TP reactor's emit sites | unvalidated |
| A-3 | Nothing outside the poller depends on `newL2TPStatsPoller` taking a reactor list | It has one caller, `(*Subsystem).Start` | Changing the source breaks another consumer | `gopls references` on `newL2TPStatsPoller` and on `l2tpStatsPoller` | unvalidated |
| A-4 | A RADIUS server tolerates an Accounting-Request that omits Acct-Input-Octets rather than sending zero | RFC 2866 lists the attribute as one a record MAY carry, not MUST | Omitting breaks a real billing system and the answer becomes a distinguishable sentinel or an error record | Read RFC 2866 Section 5 in `rfc/full/rfc2866.txt` and quote the sentence; then check the interop scenario against FreeRADIUS | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Moving accounting onto the subscriber events changes the L2TP records an operator's billing already depends on: different ordering, a duplicated Start, or a missing Stop | The L2TP accounting functional test sees a record it did not before | The L2TP records are asserted byte-for-byte before and after in the same test, and the emit sites are read to confirm the two families fire at the same points |
| R-2 | The poller walking the registry publishes a series for a session that has no `pppN` yet, so a gauge appears at zero and then jumps | A new session shows a zero sample before its first real one | The poller skips a session with an empty PPP interface rather than publishing zero, which is the same rule as the accounting fix |
| R-3 | Per-session Prometheus labels already include username and IP, so a busy concentrator with thousands of sessions multiplies cardinality when PPPoE joins | Scrape size and series count grow with subscriber count | The label set is reviewed while this work touches it, and any reduction is a deliberate, recorded decision rather than a side effect. A series per subscriber is a design choice worth restating, not inheriting |
| R-4 | The Stop record for PPPoE carries no Acct-Terminate-Cause, or the wrong one, because the PPPoE teardown reasons do not map onto the L2TP ones | The interop scenario's Stop record carries cause 0 | The mapping is written explicitly from the PPPoE teardown reasons, and a test asserts one cause per teardown path |
| R-5 | Omitting the octet attributes on a read failure is read by a billing system as a zero anyway, so the fix changes nothing observable | The interop scenario cannot tell the two apart | The record also carries an explicit signal the operator can see, and the counter-read failure is logged and counted; the decision on which signal is taken in phase 3 with the RFC text in hand |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator's billing changes for L2TP subscribers who were being billed correctly, which is worse than the gap this spec closes |
| How is it reverted? | Single commit revert. No config migration and no wire-format change |
| Who else touches this path? | The PPPoE specs closed on 2026-09-09 changed the same subsystem; `plan/spec-l2tp-ipv6-subscriber.md` will add IPv6 sessions the poller must also see |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A PPPoE session reaching the up state | → | `subscribeEventBus` → `onSessionIPAssigned` (`acct.go`) | `TestPPPoESessionSendsAccountingStart` |
| A PPPoE session torn down by PADT | → | `onSessionDown` → `sendAcctStop` (`acct.go`) | `TestPPPoESessionStopCarriesCountersAndCause` |
| A PPPoE session live on the poller's interval | → | `(*l2tpStatsPoller).poll` over the registry | `TestPollerPublishesPPPoESessionSeries` |
| A counter read that fails | → | `buildAcctPacket`'s stats branch | `TestAccountingOmitsCountersItCouldNotRead` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A PPPoE subscriber completes authentication and address assignment | An Accounting-Request with Acct-Status-Type Start is sent, carrying the same identity attributes an L2TP Start carries |
| AC-2 | That subscriber passes traffic and the interim timer fires | An Interim-Update carries Acct-Input-Octets, Acct-Output-Octets, Acct-Input-Packets and Acct-Output-Packets with the values `iface.GetStats` reports, baseline-corrected |
| AC-3 | That subscriber's counters exceed 2^32 octets | Acct-Input-Gigawords and Acct-Output-Gigawords appear, and the octet attributes carry the remainder, exactly as they do for L2TP |
| AC-4 | That subscriber is torn down | A Stop record carries the final counters, Acct-Session-Time, and an Acct-Terminate-Cause derived from the PPPoE teardown reason |
| AC-5 | A PPPoE session is live when the stats poller runs | The per-session Prometheus series carry that session's counters and uptime, with the same series names an L2TP session produces |
| AC-6 | An L2TP subscriber, before and after this change | Its accounting records and its metric samples are unchanged |
| AC-7 | `iface.GetStats` returns an error for a session's interface | The octet and packet attributes are omitted from the record rather than sent as zero, the failure is logged with the interface name, and a counter records it |
| AC-8 | A session whose PPP interface is still empty when the poller runs | No series is published for it, rather than a series of zeros |
| AC-9 | A PPPoE session and an L2TP session live at once | Both appear, and neither's counters are attributed to the other |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Bills a PPPoE subscriber from RADIUS accounting | PADR → PPP → subscriber event → Start, Interim, Stop with real octets | `radius-accounting-pppoe` interop scenario |
| 2 | Graphs a PPPoE subscriber's throughput from Prometheus | session registry → poller → the per-session rx and tx series | `TestPollerPublishesPPPoESessionSeries` plus the QEMU scrape assertion |
| 3 | Sees that a counter could not be read rather than a zero | `GetStats` error → attribute omitted, log line, counter | `TestAccountingOmitsCountersItCouldNotRead` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPPPoESessionSendsAccountingStart` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-1 | |
| `TestPPPoEInterimCarriesRealCounters` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-2 | |
| `TestPPPoECountersAboveGigawordBoundary` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-3 | |
| `TestPPPoESessionStopCarriesCountersAndCause` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-4 | |
| `TestL2TPAccountingUnchanged` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-6, asserting the encoded attribute set before and after | |
| `TestAccountingOmitsCountersItCouldNotRead` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-7 | |
| `TestPollerPublishesPPPoESessionSeries` | `internal/component/l2tp/metrics_test.go` | AC-5 | |
| `TestPollerSkipsSessionWithoutInterface` | `internal/component/l2tp/metrics_test.go` | AC-8 | |
| `TestPollerSeparatesTwoAccessTypes` | `internal/component/l2tp/metrics_test.go` | AC-9 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Acct-Input-Octets | 0 to 2^32-1 in the attribute | 4294967295 | N/A | 2^32 moves one into Gigawords |
| Acct-Input-Gigawords | 0 to 2^32-1 | 4294967295 | N/A | N/A |
| baseline-corrected counter | 0 upward | 0 on a fresh interface | a negative delta means the counter wrapped or the interface was reused, and must not underflow | N/A |
| poll interval | as `parsePollInterval` accepts | its documented maximum | 0 disables | above the maximum |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-accounting` | `test/pppoe/pppoe-accounting.ci` | An operator connects a PPPoE subscriber, passes traffic, and reads the accounting records and the metrics back | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `radius-accounting-pppoe` | `test/interop-pppoe/scenarios/` | FreeRADIUS, and a pppd or rp-pppoe client | A real RADIUS server accepts Ze's PPPoE Start, Interim and Stop records and records non-zero usage | |

## Files to Modify
- `internal/component/l2tp/plugins/authradius/acct.go` - the subscription, and the counter-read failure path
- `internal/component/l2tp/metrics.go` - the poller's source and its series naming
- `internal/component/l2tp/subsystem.go` - the poller's construction
- `docs/research/l2tpv2-ze-integration.md` - the design document `acct.go` and `internal/component/l2tp/subsystem.go` declare
- `docs/architecture/l2tp/l2tp-10-metrics.md` - the design document `internal/component/l2tp/metrics.go` declares: which access types the poller covers and what the series are named
- `docs/architecture/l2tp/subscriber-session-model.md` - the design document `subscriber/session.go` and `subscriber/events/events.go` declare: that accounting and telemetry now consume this layer
- `docs/architecture/l2tp/bng-1-radius-attributes.md` - its Accounting-Request table, which already omits the octets, packets and Gigawords rows the code sends
- `docs/architecture/l2tp/bng-5-pppoe.md` - the design document the PPPoE subsystem declares
- `docs/guide/l2tp.md` - the design document `internal/component/l2tp/cmd/l2tp.go` declares
- `rfc/short/rfc2866.md` and `rfc/short/rfc2869.md` - the requirement rows this work newly proves for a second access type

## Files to Create
- `test/pppoe/pppoe-accounting.ci` - the operator path, named in `netnsSelections` (`internal/le/qemu/netns_linux.go`), which is an explicit list and not a directory scan

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No operator choice is added; accounting is already configured per access concentrator |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | The operator-facing views are the sibling spec, `plan/immediate/spec-subscriber-utilisation-has-no-operator-view.md` |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/pppoe/pppoe-accounting.ci` |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or sysctl: the netlink read and the RADIUS socket both already exist |
| Prometheus counters/metrics | Yes | The per-session series extended to a second access type, plus a counter for failed stats reads. `internal/component/l2tp/metrics.go` |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: PPPoE subscriber accounting and telemetry |
| 2 | Config syntax changed? | No | No leaf changes |
| 3 | CLI command added/changed? | No | The sibling spec owns the views |
| 4 | API/RPC added/changed? | No | No RPC change |
| 5 | Plugin added/changed? | Yes | The authradius plugin's event subscription: `docs/guide/plugins.md` if it names the subscription |
| 6 | Has a user guide page? | Yes | `docs/guide/pppoe.md` and `docs/guide/l2tp.md` |
| 7 | Wire format changed? | No | The RADIUS attribute set is unchanged; a second producer now fills it |
| 8 | Plugin SDK/protocol changed? | No | The `subscriber` event family already exists and is unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2866.md` and `rfc/short/rfc2869.md`, with source anchors, plus the regenerated `docs/features/rfc-status.md` rows |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the new scenario |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: accel-ppp's per-session accounting is the comparison point |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/l2tp-10-metrics.md`, `subscriber-session-model.md`, `bng-1-radius-attributes.md`, `docs/research/l2tpv2-ze-integration.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | The series' access-type coverage and the failed-read counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The event family the accounting plugin subscribes to: `docs/plugin-overview.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/immediate/spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md`. The declared pages are named above |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/l2tp/bng-1-radius-attributes.md`'s attribute table is already wrong and is corrected here |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- validate the assumptions, then make one PPPoE session produce one Start record
   - Tests: `TestPPPoESessionSendsAccountingStart`
   - Files: `acct.go`, and the reading needed for A-1, A-2 and A-3
   - Verify: A-1, A-2 and A-3 flipped in the spec with evidence; the wiring test fails because the subscription does not yet cover PPPoE, never because the interface name was missing
2. **Phase: accounting over the subscriber layer** -- Start, Interim and Stop with real counters and a terminate cause
   - Tests: `TestPPPoEInterimCarriesRealCounters`, `TestPPPoECountersAboveGigawordBoundary`, `TestPPPoESessionStopCarriesCountersAndCause`, `TestL2TPAccountingUnchanged`
   - Files: `acct.go`
   - Verify: the L2TP records are identical before and after, asserted on the encoded attribute set rather than by inspection
3. **Phase: the read that fails** -- omit rather than zero, log, and count
   - Tests: `TestAccountingOmitsCountersItCouldNotRead`
   - Files: `acct.go`, the metrics registration
   - Verify: RFC 2866 Section 5 read and quoted for A-4 before the shape is chosen
4. **Phase: the poller over the registry** -- both access types, no zero series
   - Tests: `TestPollerPublishesPPPoESessionSeries`, `TestPollerSkipsSessionWithoutInterface`, `TestPollerSeparatesTwoAccessTypes`
   - Files: `internal/component/l2tp/metrics.go`, `internal/component/l2tp/subsystem.go`
   - Verify: the series names decided deliberately, with the rename-versus-extend choice recorded; R-3's label cardinality reviewed and the decision written down
5. **Phase: proof against a real server** -- the interop scenario and the operator path
   - Tests: `radius-accounting-pppoe`, `pppoe-accounting.ci`
   - Files: `test/interop-pppoe/scenarios/`, `test/pppoe/`, `internal/le/qemu/netns_linux.go`
   - Verify: revert the subscription change, watch the scenario go red, restore it, confirm green, and record that red. Where a unit carries an `RFC requirement:` tag, the walk runs through `./le rfc discriminate-record`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | A PPPoE subscriber reaches RADIUS and Prometheus, not only one of them |
| Correctness | The L2TP records are unchanged; the PPPoE Stop carries a real terminate cause; baseline correction still applies per session |
| Naming | The series name and the poller's own name say which access types they cover; an `l2tp` prefix over a PPPoE session would be a lie in a published metric |
| Data flow | One consumer path serving both access types, not two paths with a shared helper |
| Rule: `ai/rules/principles.md` | A counter Ze could not read is never reported as a zero, on any surface |
| Rule: `ai/rules/rfc-compliance.md` | The `rfc/short/` rows claim exactly what the tests check, for both access types |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| PPPoE accounting reaches a real server | The interop scenario's recorded red and green |
| L2TP accounting unchanged | `TestL2TPAccountingUnchanged` passes |
| The poller covers both access types | `gopls references` on the poller's source shows the registry, not the reactor list |
| No zero-on-failure remains | `grep` for the octet attribute append shows it inside the success branch only |
| The RFC rows are honest | `./le rfc check` passes over rfc2866 and rfc2869 |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The counters come from the kernel, not from a peer; the peer-supplied value in this path is the session identity, already validated at admission |
| Fail-closed guard | A failed stats read must not produce a billable zero. That is the guard this spec adds |
| Error leakage | The log line names the interface and the error, not subscriber traffic content |
| Resource exhaustion | The poller walks the registry on an interval, so its cost grows with session count: confirm the walk holds no lock across the netlink read |
| Authorization | Accounting records carry a subscriber's identity to an external server, so confirm no additional attribute is added that the operator did not configure |

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

- The subscriber layer was built to unify identity and it already carries everything accounting and telemetry need. Two consumers simply never moved onto it, and each kept the access-type-specific wire it was born with.
- A gap like this is invisible to every gate: both access types have tests, both pass, and no test asks whether the two produce the same observability.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Consume the subscriber layer | Add a second `subevents` subscription beside the `l2tpevents` one | Two subscriptions is the duplication that produced this defect, one access type at a time. The shared layer exists and carries the PPP interface name already |
| Omit the octet attributes on a read failure | Send zero, as today, or send an explicit sentinel | A zero is indistinguishable from an idle subscriber, which is the defect. The final shape is chosen in phase 3 against RFC 2866's own text, since A-4 is unvalidated |
| Extend the existing series rather than add a parallel set | A second PPPoE-specific series family | A parallel family makes every dashboard ask which access type it is graphing, and the counters are the same measurement from the same source |

## Known Limitations
- The operator-facing views are NOT in this spec. Counters on `show subscriber`, a per-session id form, the web UI and gNMI are `plan/immediate/spec-subscriber-utilisation-has-no-operator-view.md`, and this spec is that one's dependency.
- The shaper still keeps no byte counts (`internal/component/traffic/model.go` holds rates and burst only), so a "bytes dropped by the rate limiter" figure is not reachable from this work and is not attempted.
- Per-session Prometheus labels remain per subscriber. R-3 requires the cardinality decision to be recorded, not that it be changed here.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT. The Gigaword split cites
RFC 2869 Sections 5.1 and 5.2, and the Stop record's terminate cause cites
RFC 2866 Section 5.10.

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
- [ ] AC-1..AC-9 all demonstrated
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
