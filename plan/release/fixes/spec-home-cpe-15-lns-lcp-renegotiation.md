# Spec: home-cpe-15-lns-lcp-renegotiation

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | Adds an LNS policy RFC 2661 permits but does not require (renegotiating LCP instead of accepting the LAC's proxy LCP). It corrects a guide sentence that claims the option already exists. |
| RFC gaps | - (Section 4.4.5 and Section 9.5 permit it; no MUST is added. The wrong "RFC 2661 Section 18" citations are corrected) |
| Depends | `spec-home-cpe-14-lac-realm-relay.md` (a LAC that sends proxy LCP, for the scenario), `spec-home-cpe-18-interoplab-multi-link-topology.md` and `spec-home-cpe-19-access-chain-scenario.md` (lab), `spec-home-cpe-12-pppoe-client-lcp-renegotiation.md` (a Ze CPE that survives the LNS's renegotiation, for `access-wholesale` only) |
| Phase | 0/3 |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: plan/home-cpe/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

All values in this spec are invented (documentation prefixes, `example.net`).

## Task

When the LAC sends all three proxy LCP AVPs (26, 27, 28) in ICCN, Ze's LNS
always short-circuits LCP to Opened with the proxied options, and then runs
its own authentication phase, so it re-challenges. The operator cannot make
the LNS ignore the proxy LCP and negotiate LCP itself. Wholesale access
networks differ: some retail LNSes trust the LAC's LCP, others renegotiate.
VyOS's accel-ppp LNS always renegotiates, because it ignores the proxy AVPs.
Its ICCN handler (`accel-pppd/ctrl/l2tp/l2tp.c`, ICCN attribute loop) accepts
`Init_Recv_LCP`, `Last_Sent_LCP`, `Last_Recv_LCP` and the five
`Proxy_Authen_*` ids with a bare `break` and never reads them.

Add an LNS leaf that selects between accepting the proxy LCP (today's
behaviour, the default) and renegotiating LCP from scratch. Authentication
already restarts at the LNS in both cases. Turning proxy authentication
itself on or off is owned by
`plan/post-release/features/spec-l2tp-proxy-authentication.md`; this spec
leaves its leaf and its RFC2661-9.5-2 row to it, and says how the two leaves
combine.

Also owed:
- `docs/guide/l2tp.md` l.340-341 says Ze "validates them and can
  renegotiate instead of accepting them". Nothing selects renegotiation, so
  the sentence is false until this lands, and it is corrected in the same
  change.
- RFC 2661 has no Section 18. Its sections run 1.0 to 13.0, and proxy LCP is
  Section 4.4.5, with proxy authentication in Section 9.5. Code comments and
  the guide cite "RFC 2661 Section 18" in `ppp/session_run.go` l.199,
  `ppp/proxy.go` l.1, 55, 88, `ppp/start_session.go` l.116,
  `kernel_event.go` l.37, 78 and `session.go` l.92, 98, and an annotation in
  `rfc/short/rfc1661.md` (RFC1661-3.1-1) does the same. The umbrella's
  child 15 row repeats it ("RFC 2661 Section 18 MAY"). Every citation in
  files this spec edits is corrected (`ai/rules/stale-comments.md`). The
  umbrella row is the main thread's to correct.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/l2tp.md` - PPP negotiation (l.210-260) and l.338-341
  → Constraint: LCP, authentication and the first NCP run on the channel;
  the unit is connected only once NCP is up. A renegotiation from Opened
  must follow the same channel path the normal path uses.
- [ ] `docs/architecture/l2tp/followup-l2tp-call.md` - LAC/LNS call flow
  → Decision: the ICCN parser (`session_fsm.go::parseICCN`) stores the proxy
  AVPs and the reactor hands them to `ppp.StartSession`; the PPP driver
  decides what to do with them.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2661.md` - exists. Section 4.4.5: "This information may
  be used to initiate the PPP LCP and authentication systems on the LNS,
  allowing PPP to continue without renegotiation of LCP. Note that the LNS
  policy may be to enter an additional round of LCP negotiation and/or
  authentication if the LAC is not trusted."
  → Decision: both behaviours are permitted; the RFC sets no default, so
  today's default (accept) stays.
- [ ] RFC 2661 Section 9.5: "If the LNS chooses to implement proxy
  authentication, it MUST be able to be configured off, requiring a new
  round a PPP authentication initiated by the LNS (which may or may not
  include a new round of LCP negotiation)."
  → Constraint: the proxy-authentication off switch belongs to
  `spec-l2tp-proxy-authentication.md` (row RFC2661-9.5-2). This spec's leaf is
  the "new round of LCP negotiation" half and must compose with it.
- [ ] RFC 2661 Section 4.4.5, Proxy Authen Type: "If it is not present, then
  it is assumed that this peer cannot perform proxy authentication, requiring
  a restart of the authentication phase at the LNS if the client has already
  entered this phase with the LAC (which may be determined by the Proxy LCP
  AVP if present)."
  → Constraint: an LNS that does not use proxy authentication restarts the
  authentication phase; Ze already does, by challenging after the
  short-circuit.
- [ ] `rfc/short/rfc1661.md` - exists. RFC 1661 Section 4.3, RCR event:
  "These events may occur on a connection which is already in the Opened
  state.  The implementation MUST be prepared to immediately renegotiate the
  Configuration Options."
  → Constraint: the subscriber is in Opened with the LAC when Ze's
  Configure-Request arrives; RFC 1661 obliges it to renegotiate. A Ze CPE
  fails this today (child 12).

**Key insights:**
- The renegotiate path needs no new negotiation code: it is the non-proxy
  path that already runs when `EvaluateProxyLCP` returns an error.
- After a renegotiation, the magic number, MRU and Auth-Protocol come from
  Ze's own negotiation, not from the LAC's.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/session_fsm.go` - `parseICCN` (l.740-765)
  stores AVPs 26-28 and 29-33; the proxy Authen Name becomes
  `sess.username` (l.237-238); nothing else uses the auth AVPs.
- [ ] `internal/component/l2tp/reactor_kernel.go` - copies the three LCP
  slices into `ppp.StartSession` (l.140-142).
- [ ] `internal/component/l2tp/ppp/start_session.go` - `ProxyLCP*` fields
  (l.116-122); `AuthMethod` comment: with proxy LCP the LAC's Last-Sent
  Auth-Protocol overrides the configured method.
- [ ] `internal/component/l2tp/ppp/proxy.go` - `EvaluateProxyLCP`: all three
  slices must be present and parse; returns MRU (Last-Received), Auth-Protocol
  (Last-Sent), peer and local magic.
- [ ] `internal/component/l2tp/ppp/session_run.go` - l.199-271: on a valid
  proxy, sets `negotiatedMRU`, `negotiatedAuthMethod`, state Opened, inherits
  both magic numbers, emits `EventLCPUp`, calls `afterLCPOpen`, which runs
  the auth phase with the proxied method (a fresh CHAP Challenge for CHAP,
  waiting for an Authenticate-Request for PAP). Otherwise it applies the
  synthetic Up and Open (Closed → Req-Sent, Configure-Request sent).
- [ ] `internal/component/l2tp/ppp/manager_test.go` -
  `TestDriverProxyLCPSkipsNegotiation` proves the short-circuit.
- [ ] `internal/component/l2tp/yang/ze-l2tp-conf.yang` - no proxy leaf.
- [ ] `docs/guide/l2tp.md` l.338-341 - the overstated sentence.

**Behavior to preserve:**
- Default: proxy LCP accepted exactly as today, including the re-challenge.
- Malformed or partial proxy AVPs still fall back to normal LCP.
- The username from Proxy Authen Name still seeds `sess.username` for logs
  and accounting until authentication replaces it.

**Behavior to change:**
- With the new leaf set to renegotiate, the LNS ignores AVPs 26-28 and runs
  the normal LCP path from the synthetic Open.
- The guide sentence and the "Section 18" citations.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `l2tp { proxy-lcp renegotiate; }` (values `accept`, the default,
  and `renegotiate`).
- Wire: ICCN with AVPs 26-28.

### Transformation Path
1. YANG → `ExtractParameters` → `Parameters.ProxyLCP`.
2. Reactor builds `ppp.StartSession` with a new `IgnoreProxyLCP` flag (or
   leaves the proxy slices empty; chosen in Key Design Decisions).
3. `ppp` session run: proxy evaluation skipped, synthetic Up and Open,
   Configure-Request to the subscriber through the LAC.
4. The subscriber (Opened with the LAC) treats the Configure-Request as RCR
   in Opened (RFC 1661 Section 4.3): This-Layer-Down, Configure-Request,
   Configure-Ack/Nak.
5. LCP Opened at the LNS → authentication → NCP as for any session.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → L2TP subsystem | `ExtractParameters`, reload | No |
| L2TP reactor → PPP driver | `ppp.StartSession` | No |
| Ze LNS ↔ subscriber through LAC | LCP over the L2TP data channel | No |

### Integration Points
- `session_run.go` proxy branch - gated by the new flag.
- `spec-home-cpe-16-lns-mru-and-auth-allow-list.md` - its proxy checks
  (method outside the allow-list, MRU above `mru`) force the same
  renegotiation path per session; this spec adds the operator's global
  choice.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The flag travels config → reactor → `StartSession` like `AuthMethod` |
| No unintended coupling (components stay isolated) | Yes | ppp reads a flag, not config |
| No duplicated functionality (extends existing, does not recreate) | Yes | Renegotiation is the existing non-proxy path |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No buffer change |
| Registration over hardcoding, outbound | Yes | No new handler or command |
| Registration over hardcoding, inbound | Yes | No list learns the leaf's name; the enumeration lives in YANG |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | pppd as subscriber renegotiates LCP on a Configure-Request received in Opened | RFC 1661 Section 4.3 MUST; pppd's FSM | Renegotiation scenario fails against pppd | interop `l2tp-lns-lcp-renegotiate-pppd` | unvalidated |
| A-2 | A Ze CPE cannot renegotiate today | umbrella row "broken: the request is dropped (child 12)" | `access-wholesale` with a Ze CPE fails until child 12 lands | child 12's tests; this spec's interop uses pppd | unvalidated |
| A-3 | Frames the subscriber sent to the LAC before the bridge (an Echo, an IPCP Configure-Request) do not confuse the LNS's fresh LCP | RFC 1661 Section 4.3 (non-LCP before Opened is silently discarded) | spurious Protocol-Reject | unit test feeding an IPCP frame before LCP Opened | unvalidated |
| A-4 | With `accept`, a proxied PAP session converges because the client keeps sending Authenticate-Request | RFC 1334 Section 2: "an Id/Password pair is repeatedly sent by the peer to the authenticator until authentication is acknowledged or the connection is terminated." | PAP sessions through a proxying LAC stall until the auth timeout | unit test `TestProxyLCPPAPWaitsForRetransmittedRequest`; if wrong, that is a defect in today's default and gets a journal row and an owning spec | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Renegotiation adds a round trip through the LAC per session | session setup time in the scenario | operator's choice; default unchanged |
| R-2 | The LAC's proxy Magic-Number and Ze's new one differ, and a loop check misfires | LCP Configure-Nak loop on Magic-Number | Ze draws its own magic on the renegotiate path and inherits nothing |
| R-3 | The two leaves (this spec's and the proxy-authentication spec's) combine into a state nobody intended | review of the combination table | Key Design Decisions holds the full table |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | New L2TP sessions from proxying LACs fail LCP when `renegotiate` is set; the default path is unchanged |
| How is it reverted? | Single commit revert; the leaf is new and optional |
| Who else touches this path? | `spec-home-cpe-16-lns-mru-and-auth-allow-list.md` (same proxy branch), `plan/post-release/features/spec-l2tp-proxy-authentication.md` (LNS proxy auth), `spec-home-cpe-14-lac-realm-relay.md` (the sender) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `l2tp { proxy-lcp renegotiate }` + ICCN with AVPs 26-28 | → | reactor `StartSession` → `session_run.go` non-proxy branch | `TestL2TPProxyLCPRenegotiateReachesDriver` (`internal/component/l2tp/reactor_ppp_linux_test.go`) |
| `ze config validate` with the leaf | → | `ExtractParameters` | `test/l2tp/lns-proxy-lcp-config.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No leaf, ICCN with valid AVPs 26-28 | LCP short-circuits to Opened as today; Ze then challenges |
| AC-2 | `proxy-lcp renegotiate`, ICCN with valid AVPs 26-28 | Ze sends an LCP Configure-Request first, carries its own Magic-Number, MRU and Auth-Protocol, and reaches Opened only after the subscriber's Configure-Ack |
| AC-3 | `proxy-lcp renegotiate`, LCP Opened | Ze runs authentication with its configured method (not the LAC's), then NCP; the session comes up |
| AC-4 | `proxy-lcp renegotiate`, no proxy AVPs | Identical to today's normal path |
| AC-5 | Reload changes the leaf | New sessions follow the new value; established sessions untouched |
| AC-6 | Session snapshot / `ze show l2tp session` | Shows whether the proxy LCP was accepted or renegotiated |
| AC-7 | The guide | No sentence claims a capability that config cannot select; every RFC 2661 proxy citation names Section 4.4.5 or 9.5 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `proxy-lcp renegotiate` on a retail LNS | config → reactor → PPP driver → Configure-Request through LAC → subscriber renegotiates → CHAP → IPCP | interop `l2tp-lns-lcp-renegotiate-pppd` |
| 2 | Runs the wholesale lab | Ze CPE → Ze LAC (realm, proxy LCP) → Ze LNS renegotiates and re-challenges → CPE up | umbrella scenario `access-wholesale` (child 19) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDriverProxyLCPRenegotiateSendsConfReq` | `internal/component/l2tp/ppp/manager_test.go` | AC-2: first frame out is a Configure-Request; no `EventLCPUp` before an Ack | |
| `TestDriverProxyLCPRenegotiateUsesConfiguredAuth` | `internal/component/l2tp/ppp/auth_dispatch_test.go` | AC-3 | |
| `TestDriverProxyLCPRenegotiateOwnMagic` | `internal/component/l2tp/ppp/manager_test.go` | R-2 | |
| `TestDriverProxyLCPAcceptUnchanged` | `internal/component/l2tp/ppp/manager_test.go` | AC-1 (beside `TestDriverProxyLCPSkipsNegotiation`) | |
| `TestNonLCPBeforeOpenedDiscarded` | `internal/component/l2tp/ppp/manager_test.go` | A-3 | |
| `TestProxyLCPPAPWaitsForRetransmittedRequest` | `internal/component/l2tp/ppp/auth_dispatch_test.go` | A-4 | |
| `TestProxyLCPLeafParse` | `internal/component/l2tp/config_test.go` | enumeration, default | |
| `TestReloadAppliesProxyLCPToNewSessions` | `internal/component/l2tp/subsystem_reload_test.go` | AC-5 | |
| `TestL2TPProxyLCPRenegotiateReachesDriver` | `internal/component/l2tp/reactor_ppp_linux_test.go` | wiring | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A (enumeration leaf) | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `lns-proxy-lcp-config` | `test/l2tp/lns-proxy-lcp-config.ci` | Operator validates and loads a config with `proxy-lcp renegotiate`; an unknown value is refused with the allowed values | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `l2tp-lns-lcp-renegotiate-pppd` | `test/interop-l2tp/scenarios/` | pppd PPPoE client behind a Ze LAC (child 14) | pppd logs a second LCP negotiation, initiated by Ze's LNS, then CHAP and IPCP; ping across | |
| `access-wholesale` | child 19's scenario set | Ze CPE, Ze LAC, Ze LNS, FreeRADIUS | the umbrella's wholesale variant | |

The renegotiate scenario is broken once (the leaf ignored) to show pppd then
logs no second negotiation and the assertion goes red.

## Files to Modify
- `internal/component/l2tp/yang/ze-l2tp-conf.yang` - `proxy-lcp` leaf
- `internal/component/l2tp/config.go` - parse
- `internal/component/l2tp/reactor_kernel.go`, `reactor_setters.go`, `subsystem_reload.go` - carry and hot-apply
- `internal/component/l2tp/ppp/start_session.go` - flag; correct citation
- `internal/component/l2tp/ppp/session_run.go` - gate the proxy branch; correct citation
- `internal/component/l2tp/ppp/proxy.go`, `kernel_event.go`, `session.go` - correct citations
- `internal/component/l2tp/snapshot.go` - record accepted/renegotiated (AC-6)
- `docs/guide/l2tp.md` - l.338-341 rewritten; reload table row; configuration example
- `rfc/short/rfc1661.md` - RFC1661-3.1-1 annotation citation
- `features/l2tpv2-bng.md` - scope

## Files to Create
- `test/l2tp/lns-proxy-lcp-config.ci`
- `test/interop-l2tp/scenarios/l2tp-lns-lcp-renegotiate-pppd/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `proxy-lcp` enumeration leaf |
| YANG validation constraints | Yes | enumeration `accept`, `renegotiate`, default `accept` |
| YANG custom validators | No | enumeration suffices |
| CLI commands/flags | No | config only |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | Yes | automatic for the enumeration |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output command |
| Env var registration | N-A | not under `environment` |
| Doctor check for runtime dependencies | N-A | none |
| Prometheus counters/metrics | Yes | a `proxy_lcp` label (accepted, renegotiated, invalid) on the session-establishment counter; exact metric confirmed from `l2tp/metrics.go` in Phase 1 |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/l2tpv2-bng.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/l2tp.md` |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2661.md` (Section 4.4.5 note), `rfc/short/rfc1661.md` (citation) |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | Yes | metrics section of `docs/guide/l2tp.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived with `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-15-lns-lcp-renegotiation.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/l2tp.md` l.22-34 |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- leaf, parameter, `StartSession`
   flag, failing wiring test.
   - Tests: `TestL2TPProxyLCPRenegotiateReachesDriver`, `lns-proxy-lcp-config`
   - Files: YANG, `config.go`, `reactor_kernel.go`, `start_session.go`
   - Verify: the wiring test fails while the driver ignores the flag.
2. **Phase: Driver** -- gate the proxy branch; unit tests; citations fixed.
3. **Phase: Docs and interop** -- guide, RFC summaries, feature file,
   interop scenario with a recorded red.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | On `renegotiate`, nothing from AVPs 26-28 (magic, MRU, auth) leaks into session state |
| Correctness | The default path is byte-for-byte today's (existing proxy tests unchanged) |
| Naming | `proxy-lcp` values kebab-case |
| Rule: stale-comments | every "Section 18" in a touched file corrected |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Leaf selects renegotiation | `TestDriverProxyLCPRenegotiateSendsConfReq`, interop scenario |
| Guide corrected | grep `docs/guide/l2tp.md` for "can renegotiate" and "S18" returns nothing stale |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Untrusted LAC | With `renegotiate`, a LAC cannot choose the subscriber's Auth-Protocol or MRU |
| Input validation | Proxy AVPs are still parsed on the default path only; malformed ones still fall back |

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

- RFC 2661 has no Section 18. The citation predates this spec and appears in
  five Go files, the guide and an RFC summary annotation.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

None: `grep -rn -i "proxy lcp\|Section 18\|S18" plan/journal/` found no open
row.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Default stays `accept` | Default to `renegotiate` like accel-ppp | RFC 2661 Section 4.4.5 states both as LNS policy and sets no default; the brief requires today's behaviour unless the RFC says otherwise |
| A boolean-like enumeration `proxy-lcp accept \| renegotiate` | A boolean `renegotiate-lcp` | An enumeration leaves room for a third policy without a second leaf, and reads in the config as the policy it is |
| The reactor passes a flag; the driver skips evaluation | The reactor drops the three slices | The session snapshot and logs keep the LAC's proxy data for diagnosis (AC-6); the driver decides in one place |
| Combination with `spec-l2tp-proxy-authentication.md`'s `proxy-authentication` leaf: `accept` + auth off → today (re-challenge); `accept` + auth on → no challenge, the proxy auth AVPs are verified; `renegotiate` + either → LCP renegotiated and Ze authenticates itself, because a renegotiated LCP may change the Auth-Protocol the proxied response was computed for | Let proxy auth apply after renegotiation | After renegotiation the subscriber's earlier CHAP response belongs to a different LCP lifetime; RFC 2661 Section 9.5 describes the off state as "a new round a PPP authentication initiated by the LNS (which may or may not include a new round of LCP negotiation)" |
| Per-session forced renegotiation (method not allowed, MRU too large) belongs to child 16 | Put it here | Those are policy checks on accepted config, a P1 fix; this spec adds the operator's global choice |

## Known Limitations

- Selecting the policy per LAC (per tunnel peer) is not offered; the leaf is
  global to the LNS. No home-CPE child needs it.

## RFC Documentation (Scope: protocol)

Add `// RFC 2661 Section 4.4.5: "... the LNS policy may be to enter an
additional round of LCP negotiation and/or authentication if the LAC is not
trusted."` above the gate. Replace every "RFC 2661 Section 18" in touched
files with Section 4.4.5.

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
