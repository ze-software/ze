# Repository Audit — Methodology and Status

Date: 2026-07-16
Owner: Claude session f01e3645 (audit requested by Thomas)
Status: **Fable dimensions (1-3) COMPLETE** (report at `tmp/repo-audit-2026-07-16.md`). Opus dimensions (4-10) deferred pending instruction.

## Outcome (2026-07-16)

Ran dimensions 1-3, each with two independent Fable passes (harness duplicated the finders — used as cross-check). Phase 2 ran 7 adversarial verifiers. Confirmed: 3 BLOCKER (hold-timer permanent-disarm; forward-path buffer leak; gNMI unauthenticated), 6 HIGH (bcrypt hash-as-token; MCP boot-path fail-open; insecure-web env bypass; RFC 7606 treat-as-withdraw; keepalive/Session retention; no component-edge dependency gate). Verification downgraded 3 finder claims: "grammar gate never runs" HIGH→LOW (refuted — it runs via unit-test feeder); "hold-time-0 flap" HIGH→LOW (refuted — recentRead absorbs it, but same verifier found the real hold-timer BLOCKER); bfd→diag HIGH→MEDIUM. Raw finder outputs and verifier verdicts in session scratchpad (dim1/1b/2/2b/3/3b + verification-results.md).

## Goal

Produce a prioritized report on what should be done to make the project better.
Audit against three baselines:

1. The project's own stated rules (`ai/rules/`, CLAUDE.md) — where practice drifts from doctrine.
2. General engineering practice for a project of this size.
3. Operational/product readiness for a network OS run in production.

## Scope facts (Phase 0 inventory, established 2026-07-16)

- ~2.05M lines of Go in 8,700 tracked files; 2,414 test files; 2,360 files under `test/`
- 253 docs pages; 88 files in `ai/rules/`; 145 top-level `plan/` specs (134 active: 18 in-progress, 26 ready, 48 design, 40 skeleton, 2 blocked); 1,155 learned entries
- CI at the audit date used Woodpecker `verify.yml` and GitHub Actions. The retired `mk/*.mk` and `scripts/dev/` gate machinery now lives under `internal/le` and runs through `./le verify`.
- Working tree at audit time: 90 uncommitted files, 14 unpushed commits on main

## Ground rules

- Strictly read-only: no code changes, no commits, no heavy gates rerun (no `./le verify current mode full`).
- Agents may run cheap read-only commands (grep, git log, single-package `go vet`); no full builds or test suites. Current cheap analysis runs through the named `./le` action.
- Every finding must cite the producing code as `file:line` (`ai/rules/evidence.md`); uncited claims are dropped or labeled unverified.
- BLOCKER/HIGH findings must survive adversarial verification (Phase 2) before entering the report.

## Severity scale

- **BLOCKER**: correctness/security defect likely to bite in production or corrupt state.
- **HIGH**: real defect or systemic risk that needs scheduling.
- **MEDIUM**: quality/maintainability issue.
- **LOW**: polish.

## Phase 1 — ten parallel dimension audits

| # | Dimension | Model | Scope | Status |
|---|-----------|-------|-------|--------|
| 1 | Architecture & module boundaries | fable | tier discipline (`internal/core` vs component vs plugins), registration pattern adherence, dependency direction, oversized/god packages, generated-file hygiene | RUNNING 2026-07-16 |
| 2 | BGP engine correctness & concurrency | fable | `reactor`, `fsm`, `wire`/`wireu`, `message`, `attribute`: error paths, locking, lifecycle, protocol-conformance risks | RUNNING 2026-07-16 |
| 3 | Security surfaces | fable | SSH server/client, web UI + API auth, AAA (tacacs/radius/passwd), wire-input parsing robustness, fuzz coverage of parsers, secret handling | RUNNING 2026-07-16 |
| 4 | Test suite health | opus | untested packages, count-only/weak assertions, sleep-based sync, functional-vs-unit balance, mutation/fuzz gate coverage, flakiness | DONE 2026-07-16 |
| 5 | Performance & memory discipline | opus | buffer-first / no-sprintf-alloc adherence in hot paths, pool/refcount correctness risks, allocation hotspots | DONE 2026-07-16 |
| 6 | Error handling & robustness | opus | ignored errors, panics in non-init paths, fail-closed guard violations, degraded-mode behavior | DONE 2026-07-16 |
| 7 | Documentation & discoverability | opus | docs-vs-code drift, index completeness, onboarding path, docs/CLI help consistency | DONE 2026-07-16 |
| 8 | Build, CI & developer tooling | opus | mk/ fragment sprawl, CI depth vs local gates, hook system cost/benefit, gate runtimes | DONE 2026-07-16 |
| 9 | Dependencies & supply chain | opus | go.mod currency, vulnerability scanning, fork/patch risk, license posture | DONE 2026-07-16 |
| 10 | Process & backlog health | opus | 134 open specs vs throughput, learned/rules knowledge system ROI, spec staleness, unpushed-work risk | DONE 2026-07-16 |

## Dimensions 4-10 outcome (2026-07-16)

Report: `tmp/repo-audit-dimensions-4-10-2026-07-16.md`. Dominant theme across 6 of 7 dims: **gate theatre** — heavy tooling exists (fuzz/mutation/perf/alloc/doc-links/integration/QEMU/govulncheck-worthy) but CI runs only `./le verify current mode full`, whose stage list omits most. Two BLOCKERs: (1) CI depth — no integration/QEMU/fuzz/mutation/interop in any CI [verified: grep .woodpecker+.github = none]; (2) uncommitted/unpushed — 53 unpushed commits + ~15 untracked planning-only specs, one an untracked dependency of a committed spec [verified: git]. HIGHs: ISIS/OSPF fuzzers written-but-not-enumerated; no govulncheck gate; doc gates dark + 16 broken discovery-index refs; verify-stage-list drift + ./le yang-glue check unwired; spec citation rot + done-but-unclosed + 65 skeletons. One genuine code bug: LDP Hello-starvation (register.go, read reaches ReadFromUDP only after 5s hello ticker) [verified: direct read]. Resolved: peer_contract.go test-masking lead REFUTED (hole closed, peer_contract.go). Side effect: docs_to_code.py regenerated ai/DOCS-TO-CODE.md in working tree.

Each agent returns a fixed schema: 2-3 genuine strengths; findings ranked by severity, each with `file:line` citation(s), concrete failure scenario, suggested fix; and its single highest-leverage recommendation. Max ~12 findings per agent, quality over quantity.

## Phase 2 — adversarial verification

Every BLOCKER/HIGH finding goes to an independent skeptic agent (same model family as the finder) instructed to refute it against the actual code. Findings that do not survive are demoted to "unverified" or dropped. MEDIUM/LOW findings that drive recommendations are spot-checked by the orchestrating session.

## Phase 3 — synthesis

Merge, dedupe cross-dimension findings, write the report:
executive summary with prioritized action list → per-dimension detail → roadmap
(quick wins < a day / medium efforts / strategic items).
Report location: `tmp/repo-audit-2026-07-16.md` (gitignored), also sent to Thomas as a file.
While only dimensions 1-3 have run, the report marks dimensions 4-10 as pending.

## Resolution — finding → spec mapping (2026-07-16, "resolve all", spec-first)

User decisions (2026-07-16): **spec-first for all**; scope **BLOCKER + HIGH + MEDIUM**; bcrypt
resolution **restrict + mask**. LOWs and the multi-session architectural refactors tracked for later.

Diligence done: grepped the whole `plan/` backlog (145 specs) for overlaps before authoring, per
`ai/rules/planning.md` "Check existing spec". No existing spec covers the hold-timer disarm,
forward-buffer leak, RFC 7606, gNMI/MCP/insecure-web auth, or bcrypt hash-as-token. Several
findings ARE already owned by existing specs (do not duplicate).

### Design pass (2026-07-16): all 8 specs taken skeleton → design

One agent per spec (Fable ×6, Opus ×2) re-verified every `file:line` against the current tree,
read full producer context, and filled the design. All 8 now `Status: design`, hook-clean,
awaiting Thomas's approval to move to `ready`. Design pass CORRECTED several audit citations and
one audit finding (see the consolidated report / spec Notes). Notable outcomes:
- session-fsm: single `defer StopAll()` covers all 8 leak sites; found 3 more teardown-leak siblings + a connection-leak family (AC-7).
- forward-readbuf: fix = adopt handles onto ReceivedUpdate, drain at eviction (proven post-write); perf-next-1 lockfree code already committed (b5ad2cabe); found 2 more out-of-scope leaks.
- rfc7606: split Class-A (drop) vs Class-B (withdraw) producers; reuse existing withdraw delivery.
- mgmt-guard: corrected the audit — doctor already probes MCP, gNMI is schema-discovered; real gaps narrower; found SIGHUP reload re-exposure (AC-7); LG made an explicit exemption.
- bcrypt: found a 4th remote surface (REST/gRPC bearer `hub/api.go`) the audit missed; transport signal via aaa.AuthRequest; round-trip-safe masking via display clone + upload guard (AC-8).
- concurrency: all 4 leads confirmed REAL (lead 2 sent-path only); lead 4 has real protocol impact (RIB silently drops MP routes on duplicate-ORIGIN UPDATE).

### New specs authored (all `plan/spec-fixit-*.md`, now Status: design)

| Spec | Findings covered | Severity |
|------|------------------|----------|
| `spec-fixit-bgp-session-fsm-lifecycle.md` | B1 hold-timer permanent-disarm; H5 keepalive/Session retention leak; M1 OPEN-in-Established; FSM.Event sentinel + policy-teardown spin | BLOCKER+HIGH+MEDIUM |
| `spec-fixit-forward-readbuf-leak.md` | B2 forward-path read-buffer leak (6 sites). **Depends: spec-perf-next-1-ebgp-wire-lockfree** (in-progress, same code) | BLOCKER |
| `spec-fixit-rfc7606-treat-as-withdraw.md` | H4 RFC 7606 treat-as-withdraw drops routes | HIGH |
| `spec-fixit-mgmt-listener-auth-guard.md` | B3 gNMI unauth; H2 MCP boot fail-open; H3 insecure-web env bypass; M3 LG unauth/TLS. Unified boot-time fail-closed guard (mirrors API `main.go`) | BLOCKER+HIGH+MEDIUM |
| `spec-fixit-bcrypt-hash-credential.md` | H1 bcrypt hash-as-token (restrict-to-local + mask-on-export); M4 SSH exec log credential redaction | HIGH+MEDIUM |
| `spec-fixit-parser-fuzz-gaps.md` | M5 fuzz harnesses for BMP TLV, RADIUS VSA, DHCP | MEDIUM |
| `spec-fixit-bgp-concurrency-races.md` | 4 UNVERIFIED concurrency leads (FSM.change callback overlap, peer.session unlocked read, dynamic-peer settings race, duplicate-attribute policy). Verify-first. | MEDIUM |
| `spec-fixit-cli-view-registry.md` | M6 CLI Model per-feature anti-pattern (implement the view registry the rule prescribes) | MEDIUM |

### Findings already covered by existing specs (referenced, not duplicated)

| Finding | Existing spec | Status | Audit contribution |
|---------|--------------|--------|--------------------|
| M2 authz terminal fallback fails open | `spec-fixit-authz-admin-fallthrough.md` | design | Audit adds reachability evidence (V2): `ValidateAuthzConfig` runs only on BGP-startup + manual validate, so SSH-only appliance deployments reach the fallthrough with an unvalidated config. Recommend adding this data point to that spec. |
| LOW reactor 27.6k-line package | `spec-reactor-split.md` | skeleton | Corroborates; LOW, out of this pass's scope. |

### OPEN ITEMS — route into the in-progress tiers umbrella (do NOT duplicate its gate work)

`spec-tiers-0-umbrella.md` (in-progress) owns the three-tier taxonomy + placement audit. The
following belong there; the umbrella owner should confirm scope and add child specs if the
component-edge direction check is not already planned:

| Finding | Severity | Note |
|---------|----------|------|
| H6 no component→component / component→plugins dependency-direction gate | HIGH | Triple-confirmed at the time by the retired `dep_audit.py`. Its current producer is `internal/le/tier/gates.go`, reached through `./le tier check`. Verify umbrella scope before adding a child spec. |
| bfd → diag functional edge breaks removal invariant | MEDIUM | Becomes a tracked baseline row once the component-edge gate lands; fix = invert the seam to `bfd/api` atomic.Pointer. |
| config → bgp coupling; editor engine in `cli`; sysrib → rib global setter; web → feature-component imports; BGP spelling in the generic plugin framework; MRT hand-wired in hub; protocol validators hardcoded in central config; `internal/core/diagnostic` mis-tiered + yang_glue regen | MEDIUM | All facets of the component-boundary domain the umbrella owns. Route as umbrella child specs. |

### Deliberately deferred (LOW, per scope decision)

hold-time-0 spurious log; wireu display-path decoders (ParsePrefixes bound, NextHop RFC 5549);
verify stage-naming inconsistency + `./le yang-glue check` has no automatic caller; SSH client
host-key TOFU; SSH server username-in-log bounding.

### Coverage check

Every BLOCKER (3/3), HIGH (6/6 — H6 routed to tiers umbrella), and in-scope MEDIUM now has a
home: 8 new specs + 2 existing specs + tiers-umbrella for the boundary cluster. Nothing in scope
was silently dropped.

## Open knobs (defaults applied for the fable run)

1. BGP core depth: single agent (not split reactor vs wire) — default kept.
2. Cheap analysis gates use `./le tier check`; it replaced the retired `dep_audit.py`. Agents may attempt them with a short timeout and abandon them if slow.
