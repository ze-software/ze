# Handover: rfcgate-6 supported extraction sign-off

| Field | Value |
|-------|-------|
| Spec | `plan/spec-rfcgate-6-supported-extraction-signoff.md` (Status `in-progress`) |
| Date | 2026-08-30 |
| Session | ended at context exhaustion, not at a phase boundary |
| Commits | `17926c82f`, `0e5491e81`, `0e07e7296` |
| Pushed | no |

## Do this first

1. `git status`. Product fixes from six agents are in the tree, uncommitted. They are
   listed below. One commit pass over them is the first job.
2. Re-derive the in-scope set from `docs/features/rfc-status.md`. Do NOT trust the
   spec's tables: other sessions correct public rows continuously, and the set moved
   twice during this session. `TestSupportedRowsHaveDerivableScope`
   (`internal/le/rfc/check_test.go`) is the mechanical check.
3. `./le rfc extraction-status` for the live count.

## Where the spec got to

`signed` 6 to 15, `backlog` 165 to 156. The support-claiming set is 47 rows, not the
53 the spec's tables state.

**Landed (9):** `rfc2759`, `rfc2865`, `rfc3948`, `rfc4760`, `rfc5492`, `rfc6811`,
`rfc8203`, `rfc8671`, `rfc9234`.

**Walked but deliberately NOT landed (4).** Each has sites needing an owner decision.
The artifacts sit in the session scratch rather than redden the corpus for other
sessions, each with a replayable classification script beside it. Landing one is one
`mv` once its decision is taken.

| Stem | Held because | Decision lives in |
|---|---|---|
| `rfc4301` | 30 sites unclassified: 12 met-and-undeclared, 18 unmet | `plan/spec-rfc4301-architecture-gaps.md` (committed). Thomas held the 12 |
| `rfc3748` | 6 sites reclassified off a circular exclusion; EAP Types 2 and 3 unimplemented | the EAP Notification/NAK spec (agent may not have finished) |
| `rfc7947` | site `2.1:1` awaited the Adj-RIB-In ruling, now given | the route-server spec (agent may not have finished) |
| `rfc9069` | 24 sites unclassified; two summary rows state the OPPOSITE of the RFC | the BMP conformance fixit spec (agent may not have finished) |

Scratch path: `tmp/session/2026-08-30-7f961064-45bd-4f4c-9e8f-83e986058d66/scratch/rfc-extraction/`.
Most files there are unclassified skeletons from a sizing run and carry no work. Only
the four above hold real classification.

## Uncommitted product fixes in the tree

Each was found by a walk and each was red-probed before the fix. None is committed.

| Area | Files | What it fixes |
|---|---|---|
| RADIUS | `internal/component/radius/`, `internal/component/l2tp/plugins/authradius/` | six RFC 2865 defects, listed below |
| MS-CHAPv2 | `internal/component/ike/eap/peer.go`, `eap.go`, `eap_mschapv2.go` | the peer never verified the authenticator response; plus the Failure packet work via a grown `MethodResult` |
| BMP | `internal/component/bgp/plugins/bmp/sender.go` | the O flag was left set on a Statistics Report (RFC 8671 Section 6.2) |
| BGP roles | `internal/component/bgp/server/validate.go`, `validate_test.go` | the RFC 9234 fail-open |
| RFC 7607 | `internal/component/bgp/message/rfc7607*`, `session_open_as.go` | AS 0 processing, mid-TDD when the session ended |
| RFC 4760 | `internal/component/bgp/message/rfc7606.go`, `rib_test.go` | a void `{not-applicable}` closed by implementation |

**Check `rfc7607` and the MS-CHAPv2 Failure work before committing them.** Both agents
were mid-TDD at session end, and diagnostics showed undefined symbols in their test
files. They may be half-written. `TestRFC7607AS4PathReachesTheAttributeWalk` in
`internal/component/bgp/message/rfc7607_test.go` was red.

## Defects the walks found, as evidence the premise holds

The spec argued that a walk finds obligations no checklist carried. It did.

- **RADIUS Access-Challenge, an authentication bypass.** `(*radiusAuthenticator).Authenticate`
  returned a generic error for an Access-Challenge. `ChainAuthenticator`
  (`internal/component/aaa/types.go`) stops only on `errors.Is(err, ErrAuthRejected)` and
  otherwise tries the next backend, so enabling MFA on the RADIUS server handed the login
  to the local password database. Same shape as the RFC 8907 finding that motivated the spec.
- **MS-CHAPv2 mutual authentication absent.** `handleMSCHAPv2Success`
  (`internal/component/ike/eap/peer.go`) checked only that 40 characters parse as hex,
  never recomputed, never compared, and fell through to ACK Success when the Message was
  absent. `GenerateAuthenticatorResponse` existed but the peer never called it.
- **RADIUS Request Authenticator reused** across servers in `(*Client).SendToServers`,
  repeating a request value under a secret two servers usually share.
- **Empty RADIUS shared secret accepted** by `(*Client).Exchange`; the YANG `key` leaf had
  no `mandatory` and no length bound.
- **Class read as an authorization profile**, which RFC 2865 Section 5.25 forbids the
  client to interpret locally. Fixed by removing `profile-attribute class`; Thomas
  confirmed keeping the conformant fix.
- **RFC 9234 role validator failed open.** `broadcastValidateOpen`
  (`internal/component/bgp/server/validate.go`) returns nil when the process manager is
  absent and continues on RPC failure, so a role-mismatched OPEN established whenever the
  role plugin was unreachable. The plugin's own `validateOpenRolePair` was correct.
- **BMP O flag** left set on Statistics Reports.

## The finding that matters most

`rfc9069`. `docs/features/rfc-status.md` publishes it `Supported`, remaining "None
outstanding", and `rfc/enrolled.txt` records "seven MUST-level requirements, all met".
Two rows of `rfc/short/rfc9069.md` state the OPPOSITE of the source, both marked `[MUST]`,
both pinned by passing tests in `bmp_locrib_test.go`:

| Row | Summary says | RFC 9069 says |
|---|---|---|
| `x-3` | "Peer Up for Loc-RIB has zero-length OPEN messages" | Section 5.2: "This is a fabricated BGP OPEN message. Capabilities MUST include the 4-octet ASN" |
| `x-6` | "Peer AS MUST be 0 for Loc-RIB" | Section 5.1: "Peer Autonomous System (AS): Set to the primary router BGP autonomous system number (ASN)." |

`ai/rules/rfc-compliance.md` names this: the violation with a green bar on top. Root cause
is recorded in `plan/deferrals/ad-hoc-2026-07-27-423eaa77.md` row 14, which predicted it:
the summary was written when `rfc/full/rfc9069.txt` did not exist, so it was derived from
the code rather than from the source.

Six unmet obligations sit behind that claim, including a Peer Down using reason code 2
where Section 5.3 requires 6.

## Second finding: the interop suite proves less than it claims

`verifyTunnelTraffic` (`internal/le/interoplab/ipsec/helpers.go`) pings Ze to strongSwan,
**discards the ping result**, then asserts ESP counters advanced on each peer. Traffic in
one direction satisfies both clauses, so the return path is never proven and 100 percent
packet loss passes. charon's `bypass-lan` PASS shunt beats the Child SA, so strongSwan has
apparently never encrypted toward Ze in that lab.

Ten call sites, not nine: `gopls references` finds nine in `checkers.go`, and
`checkESPFormChange` repeats the same aggregate assertions inline.

Specced in `plan/spec-fixit-tunnel-traffic-proof-is-one-directional.md` (committed). Its
Run B, re-running the affected scenarios under a strengthened assertion and reporting
which were passing one-way, is the valuable output.

## Owner decisions taken this session

Do not re-litigate these.

| Subject | Decision |
|---|---|
| WIP cap | raised to run this spec |
| Scope boundary (A-2) | the spec's 49, not the brief's 44; the brief counted `rfc/enrolled.txt` LINES |
| RFC 3948 Section 3.1.2 | prove the kernel path with an interop scenario; done, committed |
| RFC 2759 duplicate row | delete the `Supported within PPP and IPsec EAP` row, keep `Partial`; done |
| RFC 4301 | lower the row to `Partial` and spec the fixes; done. The 12 met-and-undeclared sites are HELD |
| RFC 9234 fail-open | fail closed for role-configured peers only |
| RFC 3748 | spec Notification and NAK; MD5-Challenge is an authorized deviation needing a journal row. VOID 2026-09-01: Thomas withdrew that deviation and ordered Type 4 (MD5-Challenge) implemented |
| RADIUS Class | keep the conformant fix |
| RFC 7947 Section 2.1 | insert into Adj-RIB-In before the ingress filter, marking the entry filtered |
| RFC 7947 x-4/x-5 | correct both levels; their source Section 2.3 is explicitly non-normative |
| RFC 2759 x-6/x-12 | grow `MethodResult` with a "request then terminate" outcome |
| RFC 6811 default | change it to accept. GoBGP, FRR and BIRD all keep Invalid and require explicit policy to drop |
| RFC 7607 | should be Supported: implement AS 0 processing |
| RFC 7296 Section 2.23.1 | spec it as a feature slice with a real-NAT scenario |

## Specs commissioned, landing state unknown

`plan/spec-fixit-tunnel-traffic-proof-is-one-directional.md` and
`plan/spec-rfc4301-architecture-gaps.md` are committed. Five more agents were still
writing when the session ended. Check `plan/` for:

- RPKI accept-by-default plus a filter matching on validation result
- RFC 7296 Section 2.23.1 traffic-selector address substitution
- BMP conformance (rfc9069's six obligations plus the rfc8671 pre-policy question)
- EAP Notification and NAK, with the MD5-Challenge deviation journal row
- Route-server Adj-RIB-In transparency plus the two level corrections

## Two chores

- `plan/journal/declared-format-contradicts-payload.md` and
  `plan/journal/gate-excludes-part-of-its-population.md` carry rows the commit gate
  refuses as malformed. The third line of the second file has twelve pipes where six are
  expected. Both are excluded from the commits and still sit in the tree.
- An empty scratch directory was created under a mistyped session id:
  `tmp/session/2026-08-30-7f961064-45bd-4f4c-9e8f-83e096058d66/`. Untracked, empty.

## Process notes for whoever runs the remaining 32 walks

The shared walk brief is at
`tmp/session/2026-08-30-7f961064-45bd-4f4c-9e8f-83e986058d66/scratch/WALK-BRIEF.md`.
**Copy it somewhere durable before that scratch is swept.** It carries the workflow, the
arithmetic, the exclusion-kind rules and the hazards, and handing it to an agent instead
of re-writing a brief each time saved most of a context window.

Three things it teaches that cost real budget to learn:

1. **`extraction-create` does not merge into scratch.** `createExtraction`
   (`internal/le/rfc/extraction_create.go`) reads `previous` only from the LANDED
   `rfc/extraction/<stem>.json`, never from the scratch skeleton, and
   `newExtractionDocument` carries a disposition forward only when the landed document
   holds the same site id with an unchanged quote. A refresh mid-walk overwrites the
   classification. Classify once, then `mv`. `rfc/extraction/README.md` said otherwise and
   was corrected in `17926c82f`.
2. **`binds-another-role` is the kind that gets misused, and it failed twice.** The
   rfc4301 walk had to cut it from 43 to 2, and rfc3748 had to reclassify six sites. Two
   tests separate the cases: ask who is BOUND, not who is named (a "MUST permit the
   administrator to X" binds the implementation), and remember that an unimplemented MUST
   cannot excuse itself ("Ze is not a general-purpose X" describes the shortfall the
   sentence forbids).
3. **A new requirement row lands with BOTH test tags in the same edit.** `evaluate`
   (`internal/le/rfc/check_core.go`) demands both polarities for every gated requirement
   of an enrolled RFC, so a row without tests reds the whole corpus and blocks every other
   session's commit in this shared checkout.

Budget: roughly 150k to 330k tokens per stem, driven by how many unlisted obligations the
walk finds rather than by site count. One agent per stem. The Class B stems that still
claim support (`rfc4302`, `rfc5282`, `rfc2385`, `rfc5082`, `rfc9687`, `rfc9384`,
`rfc5798`) need a summary and enrolment before a walk, and enrolment gates every MUST at
once, so each needs its tests in the same commit.

## RFC 2759 Failure packet

DONE, uncommitted, and the whole `internal/component/ike/...` tree passes
(`go test ./internal/component/ike/...`, exit 0, `engine` included). No mutation marker
survives; `grep -rn MUTATION internal/component/ike/eap/` is empty.

- `MethodResult` (`eap.go`) grew `FinalRequest *Packet`, read before `Err` by
  `Session.handleMethod` and handled by the new `Session.finalRequest`. A nil pointer is
  not the outcome, so no existing construction site changed meaning; `finalRequest`
  panics `BUG:` if `Err` is nil beside it.
- `sendFailure` (`eap_mschapv2.go`) now builds the Section 6 packet: OpCode 4 and
  `E=691 R=0 C=<32 uppercase hex, fresh> V=3 M=`. `mschapv2OpFailure` replaced the blank
  `_ uint8 = 4`.
- `Session` gained `stateLastWord`. RFC 3748 Section 4.2 obliges an EAP-Failure "regardless
  of the response from the peer", and RFC 7296 Section 2.16 allows one EAP payload per
  IKE_AUTH, so the refusal costs two rounds. Four RFC-tagged tests
  (`TestEapResultFailureIsSent`, `TestRFC3748PeerEndsAnUnsuccessfulConversation`,
  `TestRFC3748PeerActsOnTheTerminalPacket`, `TestRFC3748MethodCompletionSendsResult`)
  caught the one-round design and are green again.
- `handleMSCHAPv2Failure` and `parseMSCHAPv2Failure` (`peer.go`) acknowledge the packet
  with OpCode 4, park `fmt.Errorf("%w: %w", ErrEAPFailure, failure)` on `pendingErr`, and
  refuse a `C=` field that is absent or not 32 hex digits (`errFailureChallenge`).
  `internal/component/ike/engine/responder_eap.go` is UNCHANGED and still correct.
- Tags: `internal/component/ike/eap/rfc2759_failure_packet_test.go` (new, untracked)
  carries x-6 and x-12 in both polarities. RED and GREEN both recorded, including a
  mutation run that reddened each.
- The `{gap}` removal in `rfc/short/rfc2759.md` was swept into another session's commit
  (it matches HEAD); `rfc/requirements/rfc2759.md` is the regenerated shard. The RFC 2759
  row of `docs/features/rfc-status.md` and
  `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` were updated in the same work.

HALF-DONE: `./le verify lint run` returned "parallel golangci-lint is running" on every
flavour, so only `gofmt` and `go vet` (both clean) cover style. `./le rfc check` was run
once and its only remaining violations are another session's unknown `RFC7607-2-2/3/4/5`
ids in `internal/component/bgp/`.

NEXT: rerun `./le verify lint run` when the lock clears, then commit the five files above
plus the new test.

OPEN QUESTION for Thomas: Ze's peer acknowledges the Failure packet and terminates; it
does not RETRY behind the fresh `C=` challenge. RFC 2759 Section 6 mandates the field,
not the retry, and a retry needs "the peer to prompt the user for new credentials", which
an IKEv2 exchange in a router has not got. Confirm that reading before closure.

## RFC 9234 role validator fail-open

DONE, and green. The owner's 2026-08-30 fail-closed decision is implemented end to end.
`broadcastValidateOpen` (`internal/component/bgp/server/validate.go`) now takes a
`policyPlugins []string`, strikes each plugin off as it answers in `askOpenValidators`,
and returns `OpenValidationUnavailableError` from `unansweredOpenValidation` for whatever
is left. That error's `NotifyCodes` returns 2/11, so `runOpenValidator`
(`internal/component/bgp/reactor/session_open_validation.go`) refuses the OPEN and logs
the silent plugin by name. The scoping fact is the plugin's own per-peer capability
declaration: `openPolicyPlugins` (`internal/component/bgp/reactor/peer.go`) filters
`GetPluginCapabilitiesForSelectors` on `InjectedCapability.PeerAddr != ""`, and bgp-role
publishes one for exactly the peers carrying role config (`extractRoleCapabilities`,
`internal/component/bgp/plugins/role/config.go`). No new cross-component field, no plugin
name in a core package. `peerCapabilitySelectors` (peer.go) is the one declaration of the
name/address/group selector rule, shared with `getPluginCapabilities`.

Left modified, uncommitted, all building and gofmt-clean: `bgp/server/validate.go`,
`bgp/server/validate_test.go`, `bgp/server/event_dispatcher.go`, `bgp/reactor/peer.go`,
`bgp/reactor/session_open_validation.go`, `docs/architecture/meta/role.md`,
`rfc/requirements/*` and `ai/RFC-REQUIREMENTS.md` (regenerated by `./le rfc index-update`).

Tests carry both polarities on `RFC9234-4.2-2` and were proven to discriminate: red with
`unansweredOpenValidation` returning nil, green after. `./le test-unit bgp` reports
`ok internal/component/bgp/server`. `./le rfc check` exits 2 on 17 violations, all in
`rfc7607` and `ike/eap`, none in rfc9234. Three reactor failures
(`TestSendOpenRefusesLocalASZero`, two link-local ones) sit in another session's open
edits to `session_negotiate.go` and `peer_initial_sync_test.go`; my code is absent from
the panic stack.

HALF-DONE: `./le verify lint run` was killed at its 9m50s wall clock before reaching the
default flavour's verdict, so my five Go files are unlinted. Next action: rerun
`./le verify lint run` and read it, then commit.

## rfc5176 (Tier 1 walk)

No mutation left in the tree: checked for `if false` and `MUTATION` in `coa.go` and
`packet.go` after restoring both from scratch copies. `go test` on
`./internal/component/l2tp/plugins/authradius/` and `./internal/component/radius/`
passes; `gofmt` and `go vet` clean. Lint not run (another session holds the lock).

DONE, product. Eight RFC 5176 defects found and fixed, each with a positive and a
negative test tag, red phase proven by reverting each fix:
1. `radius.VerifyCoAMessageAuthenticator` (was `VerifyMessageAuthenticator`) now zeroes
   the Request Authenticator field as RFC 5176 §3.4 requires, and `VerifyCoARequestAuth`
   no longer zeroes the Message-Authenticator. **Ze could not interoperate with any
   conformant Dynamic Authorization Client before this**, since it demands the attribute.
2. `coaListener.handlePacket` silently discards a stale Event-Timestamp (§6.3); it NAKed.
3. `coaListener.sendResponse` echoes Proxy-State (§3.1) and State (§3.3); it echoed neither.
4. `coaListener.oneSession` NAKs Error-Cause 508 on a multi-session match (§2.3); it
   applied the change to the first match only.
5. `unsupportedAttr` + `coaSupportedAttrs`/`disconnectSupportedAttrs` NAK 401 (§2.3, §3).
6. `handlePacket` NAKs 405 on any Service-Type in a CoA-Request (§2.2, §3.2).
7. `handleCoA`/`applySubscriberCoA` NAK 506 when the change cannot be dispatched (§2.3);
   they sent CoA-ACK with a nil bus, an emit error, or a skipped CoS profile.
8. `coaListener.isAllowedSource` fails closed on an empty list and `startCoAListener`
   (register.go) refuses to start with no resolved server: it trusted everybody.
`findSession` became `findSessions`, combining identification attributes with AND (§3).

DONE, artifacts. 15 new requirements in `rfc/short/rfc5176.md`, `./le rfc index-update`
run, all 20 rows now carry both polarities in `rfc/requirements/rfc5176.md`. Doc updated:
`docs/guide/l2tp.md` CoA section.

HALF-DONE and unsafe to trust: `rfc/extraction/rfc5176.json` DOES NOT EXIST. All 72
sites and 28 sections are classified in my head and in the blocked script text, but the
skeleton at
`tmp/session/2026-08-30-7f961064-45bd-4f4c-9e8f-83e986058d66/scratch/rfc-extraction/rfc5176.json`
is still fully unclassified. It is in scratch, so it reds nothing. `./le rfc check` was
never run this session.

Next action: re-derive the classification and land the sign-off. Forward split would be
43 mapped / 29 excluded (ratio 0.40): 16 `binds-another-role` (5 DAC retransmission
sites in §2.3, 7 forwarding-proxy sites in §3.1, plus 3.2:1, 3.3:9, 3.4:2, 4:1), 9
`advisory-in-context` (the §3.2 Authorize Only OPTIONAL block, 3.3:4/5/7, 3.4:4, 6.1:2),
2 `cross-document` (3.3:2 and 3.3:3 quote RFC 2865 §5.44), 1 `not-a-requirement` (3.6:1
is the table notation legend), 1 `duplicate-of` (6.3:3 restates 2.3:3).
`unsourced-ids` on section 2.3: `RFC5176-3.5-1`, `-3.5-2`, `-3.5-3`.

ASK for Thomas, recorded in `rfc/short/rfc5176.md` under "Known deviation": five ids are
anchored to the wrong section (`RFC5176-3.5-1`, `-3.5-2`, `-3.5-3`, `-3.5-4` state §2.3
obligations, `-3.3-1` states a §3 one). `parseRequirementID` in `internal/le/rfc/summary.go`
refuses an id disagreeing with its cited section, and `check_retired_requirements` refuses
a vanishing id, so correcting them needs an owner decision. Second ask: Ze sends no
Error-Cause 403 and never checks a NAS identification attribute against its own identity
(no site states it as a MUST, so the forward arithmetic is unaffected).

## RFC 7607

DONE, and both packages BUILD. All 5 MUST rows implemented, enrolled, signed off.
`./le rfc check` reports 31 violations and NONE names rfc7607; all 31 are another
session's `rfc5176_walk_test.go` tags. `rfc/requirements/rfc7607.md` shows 5/5 gated with
both polarities. Product: `validateAS4PathAttr` and `validateAS4AggregatorAttr` in
`internal/component/bgp/message/rfc7607.go` (first validators codes 17/18 ever had);
AS 0 tests added inside `validateASPath` and `validateAggregatorAttr` in
`internal/component/bgp/message/rfc7606.go`; `Session.validateOpenPeerAS` in
`internal/component/bgp/reactor/session_open_as.go`, called from `handleOpen`
(`session_handlers.go`) and `processOpen` (`session_connection.go`); `sendOpen` guard in
`session_negotiate.go`; `ErrBadPeerAS` / `ErrLocalASZero` in `session.go`. RED then GREEN
recorded for every positive; negatives pass in both phases, so they discriminate.

ALSO FIXED, unrelated to AS 0 and worth a look: `writeMessageWithin`
(`internal/component/bgp/reactor/session_write.go`) guarded `conn == nil` but not its
paired `s.bufWriter`, so a session that never ran `connectionEstablished` segfaulted on a
nil `*bufio.Writer` instead of returning `ErrNotConnected`. Now guarded.

HALF-DONE, do not trust: NO interop scenario. I had read
`docs/architecture/testing/interop.md` and chosen the shape (raw injector at 172.30.0.9
sending an AS_PATH holding AS 0, plus a control prefix, asserting FRR gets the control and
not the poisoned one, via `opFRRRoute` / `opFRRRouteAbsent` in
`internal/le/interoplab/bgp/check_engine.go`) but wrote nothing.
`ai/rules/interop-and-goal-validation.md` is unsatisfied.

TWO REDS ARE MINE and both are the TEST being wrong, not the product:
`TestRFC7607AS4PathReachesTheAttributeWalk` (`message/rfc7607_test.go`) asserts
attribute-discard where treat-as-withdraw is correct, because its UPDATE omits the
mandatory attributes; and `TestSendOpenRefusesLocalASZero/real_local_AS`
(`reactor/session_open_as_test.go`) calls `sendOpen` on a never-connected session. Both
carry `RFC requirement:` tags, so `Proposed` in `internal/le/testweakened/proposed.go`
refuses every repair without an owner row in `test/rfc-changed.md`; its `oldText` comes
from disk, so a file authored in the same session gets no carve-out. I wrote correct
replacements instead, and every requirement has a PASSING pair:
`TestRFC7607CompleteUpdateAS4PathZero` and `TestSendOpenSendsWithRealLocalAS`. Four other
reactor reds are NOT mine: I neutralised my own checks and all four still failed.

NEXT ACTION: get the owner's `test/rfc-changed.md` rows for those two test names, delete
them, then write the interop scenario.

## Route-server Adj-RIB-In

`plan/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md` is written and COMPLETE.
`./le hook-check validate-spec` exits 0 on it. Status is `design`: the `/ze-spec` SCOPE,
RESEARCH, DESIGN and WRITE gates were never held with the owner, because a subagent holds
no dialogue. The main thread must hold them and flip Status to `ready`. Nothing else in
the spec is unfinished.

Files left modified: only that one new spec file. No source file, no `rfc/` file, no
`docs/` file was touched. `rfc/extraction/rfc7947.json` is still unlanded, in this
session's scratch beside `classify-rfc7947.py`; the spec's phase 7 is what lands it, with
its Section 2.1 site mapped to a new `RFC7947-2.1-1`.

Verified while researching, and worth carrying: `go vet` on both
`internal/component/bgp/reactor/...` and `internal/component/bgp/plugins/adj_rib_in/...`
was CLEAN at 2026-08-30, so the concurrent `SignalPeerAPIReady` edit did not leave those
packages unbuildable.

NEXT ACTION: the main thread runs the `/ze-spec` gates over that spec with the owner, then
sets Status `ready`.
