# Spec: traceroute-source-af

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-09-05 |

**Notes:** Promoted to ready per user instruction 2026-07-10 (followup-wave impact review session) authorizing conversion to ready. Scope precision applied 2026-07-10: the fix lands on the `ze-resolve:traceroute` path, the only entry point that accepts a source today (see Post-wave corrections and Known Limitations).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/traceroute/cmd/traceroute.go` - traceroute engine + arg parsing
4. `internal/component/traceroute/cmd/resolve.go` - target/source resolution
5. `internal/core/probe/icmp.go` - `ResolveTarget`

## Task

When an operator runs a traceroute toward a hostname and specifies a source-address
of a particular family (e.g. an IPv6 source), Ze resolves the destination hostname
without regard to that family and picks the first answer. If the first answer is an
A (IPv4) record, Ze opens an IPv4 socket and tries to bind the IPv6 source, which
fails. The source-address family must constrain destination resolution: a v6 source
forces v6 resolution, a v4 source forces v4.

Make the source-address's family drive the address family used to resolve the
destination hostname (and, consequently, the socket family).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - the verb-first API command paths, with JSON or text encoding
- [ ] `docs/architecture/diagnostics/active-probes.md` - ping, traceroute and route lookup, which validate a forwarding path
- [ ] `docs/architecture/resolve.md` - the resolution component, which consolidates external data resolution under one tree
- [ ] `docs/architecture/core-design.md` - probe/traceroute component placement.
  → Constraint: traceroute uses Ze's own ICMP engine (raw socket), not a shell-out; AF selection is internal.

**Key insights:**
- The destination AF is currently derived from the resolved destination address, and the source-address is parsed afterwards and only used as the bind address.
- The fix is ordering + intent: read the source-address family first, then resolve the destination in that family.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/traceroute/cmd/traceroute.go` - `doTracerouteCtx` derives the socket family from the destination: `isV6 := dest.Is6()` (traceroute.go); the `source` is used only for `bindAddr` (traceroute.go, :208). `parseTracerouteArgs` (traceroute.go) resolves the destination first; source has no bearing on it.
- [ ] `internal/component/traceroute/cmd/resolve.go` - the destination is resolved AF-agnostically before the source arg is read; `validateSourceIP` (resolve.go) checks only that the source is a valid IP, not its family vs the destination.
- [ ] `internal/core/probe/icmp.go` - `ResolveTarget` calls `net.DefaultResolver.LookupNetIP(ctx, "ip", s)` and returns `ips[0]` (icmp.go); network `"ip"` means either family, first answer wins.

### Post-wave corrections (2026-07-10)

All refs re-verified against current code. `doTracerouteCtx` (traceroute.go),
`isV6 := dest.Is6()` (:192), bindAddr (:202-204, ListenPacket :208),
`parseTracerouteArgs` (:60-116), `validateSourceIP` (resolve.go) and
`ResolveTarget` (icmp.go, `LookupNetIP` with network `"ip"` :58, first
answer :65) are all current. Two material precision corrections:

- **Entry-point correction (supersedes A-2's basis and the wiring wording).**
  The source option exists ONLY on the `ze-resolve:traceroute` RPC path, and
  its keyword is `source` (not `source-address`): `handleResolveTraceroute`
  (resolve.go) resolves the destination FIRST via `probe.ResolveTarget`
  (resolve.go) and only THEN parses `source` (resolve.go) -- that
  ordering is the bug site. `parseTracerouteArgs` has NO source option at all;
  the `ze-show:traceroute` handler (`handleTraceroute`, traceroute.go),
  the offline `show traceroute` (`showTracerouteLocal`, register.go) and
  the monitor paths all pass an empty `tracerouteOpts` (traceroute.go,
  register.go). So the entry points do NOT share one parse path, and only
  the resolve path can express a source today. The fix (source-first parse +
  family hint + clear mismatch error) lands on `handleResolveTraceroute`;
  `ResolveTarget` still gains the family-aware capability as designed.
- **`ResolveTarget` blast radius.** Six non-test callers: traceroute
  resolve.go, traceroute.go, stream.go; ping resolve.go,
  ping.go, stream.go. A family-aware variant (or an added parameter
  updated at all six sites) must keep ping compiling with unchanged behaviour.
- **Adjacent observation (out of scope).** Ping's resolve path has the same
  source-after-resolve pattern (`internal/component/ping/cmd/resolve.go`:
  destination resolved at :31, `source` parsed at :44-52). The same bug class
  exists there; this spec does not fix it -- flag it to the user for a
  follow-up decision at implement time.

**Behavior to preserve:**
- With no source-address, resolution stays AF-agnostic (current behaviour, first answer wins).
- Traceroute continues to use the internal ICMP engine (no shell-out).
- A literal IP destination is unaffected (no resolution needed).

**Behavior to change:**
- When a source-address is given, its family constrains destination hostname resolution.

## Data Flow (MANDATORY)

### Entry Point
- ~~CLI: `resolve traceroute` / `show traceroute` with a `source-address <ip>` option and a hostname target, parsed in `parseTracerouteArgs` (traceroute.go).~~ (Superseded 2026-07-10: wrong producer.) The `ze-resolve:traceroute` RPC (`handleResolveTraceroute`, resolve.go) with a `source <ip>` option and a hostname target; the source is parsed in that handler's own arg loop (resolve.go), not in `parseTracerouteArgs`. `show traceroute` has no source option today.

### Transformation Path
1. Parse args; extract the source-address (if any) before resolving the destination.
2. Derive an address-family hint from the source-address family (v4 → `"ip4"`, v6 → `"ip6"`, none → `"ip"`).
3. Resolve the destination hostname with that family hint (`ResolveTarget` gains a family parameter, or a family-aware variant is used).
4. Open the ICMP socket for the resolved family and bind the source-address.
5. On family mismatch that cannot be satisfied (e.g. v6 source, hostname has no AAAA), return a clear error rather than a bind failure.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ traceroute engine | source-address family passed into resolution | [ ] |
| Engine ↔ resolver | family hint → `LookupNetIP(ctx, "ip4"|"ip6"|"ip", …)` | [ ] |
| Engine ↔ socket | resolved family selects v4/v6 socket | [ ] |

### Integration Points
- `ResolveTarget` (`probe/icmp.go`) - accept a family hint (six non-test callers, incl. ping; see Post-wave corrections).
- `handleResolveTraceroute` (`resolve.go`, source parse :47-54, destination resolve :32) - order source parse before destination resolve; pass the hint (corrected 2026-07-10: this is where the source is parsed, not `parseTracerouteArgs`).
- `doTracerouteCtx` (`traceroute.go`) - socket family from the resolved destination, unchanged mechanics.
- `validateSourceIP` (`resolve.go`) - optionally assert source family matches the resolved destination.

### Architectural Verification
- [ ] No bypassed layers (resolution still through the probe resolver)
- [ ] No unintended coupling (family hint is a parameter, not global state)
- [ ] No duplicated functionality (reuse `ResolveTarget`, extend it)
- [ ] Registration over hardcoding — traceroute command remains registry-registered; no central AF switch.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `LookupNetIP` supports `"ip4"`/`"ip6"` networks for family-constrained resolution | Go stdlib net semantics | need manual filtering of results | unit test with a dual-stack name | confirmed (2026-09-05: probe run before any feature code. `"ip4"`/`"ip6"` constrain the answer; a name with no address in the asked family fails rather than answering the other family. Two failure shapes, not one: a name whose records are all of the other family gives `*net.AddrError` "no suitable address found", a name with no record in the asked family gives `*net.DNSError` with `IsNotFound`. `familyAbsent` (`internal/core/probe/icmp.go`) classifies both, and reports every other failure as itself) |
| A-2 | ~~Both `resolve traceroute` and `show traceroute` share the same parse path~~ | ~~traceroute.go~~ | must fix both call sites | grep both entry points during audit | broken (2026-07-10: they do NOT share a parse path; only the resolve path accepts a source -- see Post-wave corrections; fix scoped to `handleResolveTraceroute`) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | v6 source + hostname with only A records → no resolvable target | traceroute errors | return a clear "no AAAA for source family" message, not a bind failure |
| R-2 | Regression for the no-source case | v4-only hosts change behaviour | family hint defaults to `"ip"` when no source given (unchanged path) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| ~~`show traceroute <dualstack-host> source-address <v6>`~~ `resolve traceroute <dualstack-host> source <v6>` (corrected 2026-07-10: only the ze-resolve:traceroute path accepts a source; keyword is `source`) | → | destination resolved as v6, v6 socket | `test/plugin/traceroute-source-af.ci` |
| ~~`... source-address <v4>`~~ `... source <v4>` | → | destination resolved as v4 | `test/plugin/traceroute-source-af.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | dual-stack hostname + IPv6 source-address | destination resolved to its AAAA; v6 socket bound to the v6 source |
| AC-2 | dual-stack hostname + IPv4 source-address | destination resolved to its A record; v4 socket |
| AC-3 | hostname with only A records + IPv6 source | clear error (no target in source family), not a bind failure |
| AC-4 | no source-address | AF-agnostic resolution unchanged (first answer wins) |
| AC-5 | literal IP destination | unaffected |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | traceroutes a dual-stack host from a v6 source | source parsed first → v6 resolve → v6 socket | `test/plugin/traceroute-source-af.ci` |
| 2 | uses a v6 source toward a v4-only name | clear error message | `test/plugin/traceroute-source-af.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveTargetFamilyHint` | `internal/core/probe/icmp_test.go` | a family-constrained lookup never answers with the other family | pass |
| `TestFamilyOf`, `TestFamilyNetwork` | `internal/core/probe/icmp_test.go` | source address → family → resolver network name | pass |
| `TestResolveTargetLiteralFamilyMismatch` | `internal/core/probe/icmp_test.go` | a literal target of the other family returns `ErrFamilyMismatch` | pass |
| `TestResolveTargetUnmapsLiteral`, `TestResolveTargetUnmapsLookup` | `internal/core/probe/icmp_test.go` | the answer is unmapped, so the socket family read off it is right | pass |
| `TestHandleResolveTraceroute_SourceFamilyDrivesResolution` | `internal/component/traceroute/cmd/resolve_test.go` | both polarities of a conflict are refused naming the source and the target, before a socket is opened | pass |
| `TestParseResolveTracerouteArgs` | `internal/component/traceroute/cmd/resolve_test.go` | the option parser that now runs BEFORE resolution still reads every keyword | pass |
| `TestParseSourceIP_Valid`, `TestParseSourceIP_Invalid` | `internal/component/traceroute/cmd/resolve_test.go` | one parse validates and produces the source, so a rejected one cannot reach the engine unset | pass |

The unit test names in the design (`TestTracerouteSourceForcesV6`,
`TestTracerouteSourceFamilyMismatch`) landed on `resolve_test.go` rather than
`traceroute_test.go`, because `handleResolveTraceroute` is the producer and it
lives in `resolve.go`. The v6-source success polarity needs a dual-stack name
and a raw socket, so it is proven by the functional test rather than by a unit
test that would depend on the developer machine's `/etc/hosts`.

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `traceroute-source-af` | `test/plugin/traceroute-source-af.ci` | source family drives destination resolution, in both polarities, plus the named conflict | pass |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - operational tool, no peer protocol | - | - | validated by functional tests | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/traceroute/cmd/traceroute.go` - parse source before resolving destination; pass family hint
- `internal/component/traceroute/cmd/resolve.go` - family-aware resolution; optional source/dest family assertion
- `internal/core/probe/icmp.go` - `ResolveTarget` accepts a family hint

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI grammar | [ ] no (existing options) | `ai/rules/cli.md` |
| Functional test for new behaviour | [ ] yes | `test/plugin/traceroute-source-af.ci` |
| Pipe completeness | [ ] yes | traceroute output already routes through pipes; keep it |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | [ ] no new command; the `source` option changed meaning | `internal/plugins/traceroute-cmd/yang/ze-traceroute-cmd.yang` (the `ze:help` and the `source` leaf description are the operator-facing text for `resolve traceroute`; `docs/guide/command-reference.md` documents `show traceroute` and `monitor traceroute` only, and neither takes a source) |
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/diagnostics/active-probes.md`, new section "The source address decides the family" |

## Files to Create
- `test/plugin/traceroute-source-af.ci` - functional test
- (unit tests extend existing test files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add a family-hint parameter to `ResolveTarget` (default `"ip"`, no behaviour change); failing `test/plugin/traceroute-source-af.ci`.
2. **Phase: Source-first parse + hint** -- parse the source before resolving the destination; derive and pass the family hint in `handleResolveTraceroute` (resolve.go) ~~in both `resolve traceroute` and `show traceroute`~~ (corrected 2026-07-10: show/monitor paths accept no source today).
   - Tests: `TestResolveTargetFamilyHint`, `TestTracerouteSourceForcesV6`
3. **Phase: Mismatch error** — clear error when the source family has no resolvable target.
   - Tests: `TestTracerouteSourceFamilyMismatch`
4. **Functional test**
5. **Full verification** → `./le verify current mode full`
6. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | no-source path byte-for-byte unchanged; the resolve entry point fixed (~~both entry points~~ corrected 2026-07-10: only the resolve path takes a source); all six `ResolveTarget` callers still compile with unchanged behaviour |
| Data flow | family hint threaded as a parameter |
| Registration over hardcoding | no central AF switch introduced |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| family-aware resolution | `go test ./internal/core/probe -run FamilyHint` |
| entry point fixed | grep shows source parsed before resolve in `handleResolveTraceroute` (~~resolve+show paths~~ corrected 2026-07-10) |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | source-address parsed as a valid IP before use |
| Error leakage | resolution errors do not leak internal resolver detail |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-2: resolve and show traceroute share one parse path with a source option | Only `ze-resolve:traceroute` accepts a source (keyword `source`, resolve.go); `parseTracerouteArgs` has no source option; show/monitor paths pass empty opts | Design-stage re-verification 2026-07-10 (read handlers in resolve.go, traceroute.go, register.go) | Fix scoped to `handleResolveTraceroute`; wiring/phase wording corrected before implementation started |

## Known Limitations
- Only the `ze-resolve:traceroute` path accepts a source today, so only that path gains family-driven resolution. `show traceroute` / `monitor traceroute` (parseTracerouteArgs-based, no source option) are unaffected; adding a source option to them is out of scope for this spec.
- Ping's resolve path has the same source-after-resolve pattern (`handleResolvePing`, `internal/component/ping/cmd/resolve.go`) and is NOT fixed here. Flagged for a user decision as this spec instructed, and recorded as one row in `plan/journal/option-read-after-the-decision-it-governs.md`. The seam it needs already exists: `probe.ResolveTarget` takes a family and `probe.FamilyOf` derives one, so the fix is the same reorder plus one call.

## Design Insights
<!-- LIVE -->

- **`LookupNetIP` answers an IPv4 address in the IPv4-mapped IPv6 form.** So
  `ResolveTarget("localhost")` returned `::ffff:127.0.0.1`, `dest.Is6()` read
  true on it, and `doTracerouteCtx` opened an ICMPv6 socket for an IPv4
  destination. That is the same family-selection defect this spec exists for,
  reached without a source address at all, so `ResolveTarget` now unmaps the
  address it answers with and every caller reads the right family off it.
- **"No address in this family" arrives in two error shapes, not one.** A name
  whose records are all of the other family gives `*net.AddrError` ("no suitable
  address found"), and a name with no record in the asked family gives
  `*net.DNSError` with `IsNotFound`. A SERVFAIL or a timeout is neither, and
  reporting one of those as a family conflict would blame the source address for
  a broken resolver. `familyAbsent` splits them.
- **The conflict is named for a literal target too.** A literal needs no
  resolution, so AC-5 leaves it alone, but `resolve traceroute 127.0.0.1 source
  ::1` is the same conflict and used to reach the operator as a bind failure.
  One wording answers both routes.

## Implementation Summary
### What Was Implemented
- `probe.Family` (`internal/core/probe/icmp.go`) is the typed family a
  resolution is held to, `FamilyAny` is its zero value, and `probe.FamilyOf`
  derives one from a source address. `ResolveTarget(s, family)` holds both its
  routes (literal and lookup) to that family, unmaps what it answers, and
  reports `probe.ErrFamilyMismatch` when the target carries no address in it.
- `handleResolveTraceroute` (`internal/component/traceroute/cmd/resolve.go`)
  parses its options first, through the new `parseResolveTracerouteArgs`, and
  resolves the target after, in the family of the source. A conflict answers
  `traceroute: source <addr> is <family> but target "<t>" has no <family>
  address`, before any socket is opened.
- `parseSourceIP` replaced `validateSourceIP`: one parse validates the source
  and produces it. The old pair validated with `net.ParseIP` and then discarded
  the error of a second `netip.ParseAddr`, so a source that parsed differently
  in the two libraries would have been dropped in silence. It also accepts a
  zone (`fe80::1%eth0`), which the YANG pattern always allowed and `net.ParseIP`
  always refused.
- The other six `ResolveTarget` call sites (traceroute `traceroute.go` and
  `stream.go`, ping `ping.go` twice, `resolve.go` and `stream.go`) pass
  `probe.FamilyAny` and keep their behavior, apart from the unmapping fix above,
  which they all gain.

### Discrimination (RED observed, then GREEN)

The fix was reverted (`family := probe.FamilyAny` in `handleResolveTraceroute`,
and `ResolveTarget` back to `"ip"` with no unmapping), the tests were run, the
fix was restored, and the tests were run again.

| Test | RED under the revert | GREEN restored |
|------|----------------------|----------------|
| `TestResolveTargetLiteralFamilyMismatch` | `ResolveTarget("192.0.2.1", IPv6) = 192.0.2.1, <nil>; want ErrFamilyMismatch` | pass |
| `TestResolveTargetFamilyHint` | `ResolveTarget(localhost, IPv6) = ::ffff:127.0.0.1, which is IPv4` | pass |
| `TestResolveTargetUnmapsLiteral`, `TestResolveTargetUnmapsLookup` | both answered the `::ffff:` form | pass |
| `TestHandleResolveTraceroute_SourceFamilyDrivesResolution` | `traceroute: listen ip4:icmp: address 2001:db8::1: no suitable address found (requires CAP_NET_RAW)`, which names the socket and neither argument | pass |
| `test/plugin/traceroute-source-af.ci` (QEMU guest, real CAP_NET_RAW) | `ZE-OBSERVER-FAIL: resolve traceroute localhost source 127.0.0.1 ...: status=error want done: traceroute: listen ip6:ipv6-icmp: address 127.0.0.1: no suitable address found` | `7.2s 1/1 PASS 723 traceroute-source-af` |

The guest resolved `localhost` to `::1` first, so under the revert the IPv6
polarity passed by accident and the IPv4 one failed. Asserting both polarities
is what caught it: one alone would have gone green against the defect.

Guest evidence for the premise: `cat /etc/hosts` in the booted guest answers
`127.0.0.1 localhost localhost.localdomain` and `::1 localhost
localhost.localdomain`, and `lo` carries `::1/128`.

### Verification still owed (deferred by owner instruction, 2026-09-05)

The targeted unit runs and the QEMU functional run below are done and green.
The whole-tree gate is NOT run: the owner deferred heavy testing until this
checkout has one user. The command that produces it is
`./le verify current mode full`, and the functional evidence re-runs with
`./le qemu run packages "go" timeout 40m command "cd /workspace && mkdir -p
/root/gocache && export GOCACHE=/root/gocache && go run -tags ze_test ./cmd/ze
bgp plugin traceroute-source-af"`.

`./le verify lint run` was run before the deferral and reported one finding in
this diff (`errorsastype` on `familyAbsent`), which is fixed. The other files
in its failure group belong to other sessions in this shared checkout.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A)
- [ ] Functional tests for end-to-end behavior
