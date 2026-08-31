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
| RFC 3748 | spec Notification and NAK; MD5-Challenge is an authorized deviation needing a journal row |
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
