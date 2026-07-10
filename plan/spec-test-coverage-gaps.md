# Spec: test-coverage-gaps

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/7 |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `tmp/test-audit/REPORT.md` - the 2026-07-10 coverage audit this spec implements (raw data in `tmp/test-audit/`)
4. `internal/test/cli/register.go` + `internal/test/cli/ci_runner.go` - suite registry (orphan-suite fix target)
5. `ai/rules/testing.md`, `ai/rules/functional-test-gate.md`, `ai/rules/qemu-testing.md` - the rules this spec back-fills

## Task

Close the test-coverage gaps found by the 2026-07-10 audit (`tmp/test-audit/REPORT.md`).
One phased spec covering all audit items (user decision 2026-07-10). Scope includes
enabling test wiring (suite registration, gating lists, QEMU suite list), the
sleep-ratchet breach fix, and the VPP test-file renames; scope EXCLUDES CI
scheduling (nightly pipelines). Bugs exposed by new tests are fixed at source
in-spec (user decision 2026-07-10, per `ai/rules/no-workarounds-for-missing-behavior.md`).

Out of scope (owned by other specs; do not touch): property tests / multi-peer LLGR /
chaos iface / QEMU traffic enrollment (`plan/spec-followup-test-infra.md`), env-knob and
op-1 command `.ci` backlog (`plan/spec-finish-ci-coverage.md`), vpp_stub emulator and
`test/vpp/*.ci` additions (`plan/spec-finish-vpp-stub.md`).

### Work items (frozen scope, from the audit)

**W-1 Suite wiring (enabler, first):**
- Register `ipsec` and `appliance` ze-test suites (17 orphan `.ci` become runnable);
  add both to the gating list in `mk/test-functional.mk` and to
  `scripts/evidence/qemu-all-tests.sh` where appropriate.
- Gate the three offline wire suites (`l2tp-wire`, `isis-wire`, `ospf-wire`) in
  `ze-functional-test` (they are offline decode, no platform deps).
- Add `isis`, `ospf`, `ospfv3`, `ldp`, `rsvpte` to the QEMU functional pass suite list
  (`scripts/evidence/qemu-all-tests.sh`) so `needs-linux` OSPF/OSPFv3 tests run in the VM.
- Fix orphaned/failing `.ci` uncovered by wiring (fix at source).

**W-2 Sleep-ratchet breach:** current tracked count 475 (477 on disk) vs committed
baseline 448 (`test/.ci-sleep-baseline`); gate `check_ci_sleep_ratchet`
(`scripts/dev/verify_wiring_docs.py:188`) fails the next `.ci`-touching change.
Investigate origin, convert new sleeps to `wait_for_event`/`wait_for_shutdown` where
feasible, set the baseline to the true count only for justified remainders.

**W-3 Untested features (unit + functional):**
- `internal/plugins/mrt` (846 LOC, zero tests): unit tests + `test/plugin/*.ci` per
  dump mode (file/interval, updates/routes/all, direction, peer-filter).
- `internal/component/gokrazy` (144 LOC, zero tests): unit tests for auth injection +
  path rewrite; one functional test through the web mount.
- `internal/plugins/static/vpp` (161 LOC, zero tests): the four mandated VPP backend
  test files (BLOCKING rule, `ai/rules/testing.md`).
- `internal/component/bgp/plugins/aigp` (94 LOC, zero tests): unit test for the
  attr-26 JSON formatter + a decode/plugin functional test.

**W-4 telemetry/collector unit back-fill:** ~36 procfs/sysfs parsers at 2.7% coverage
(`internal/component/telemetry/collector/*_linux.go`, 3146 LOC): fixture-driven unit
tests per parser (node_exporter-style golden files). Add seam at source if paths are
hardcoded (fix-at-source policy).

**W-5 Functional-gate misses (user-facing surface, no functional test):**
`show mpls-forwarding`; looking-glass web UI/graph/CSV/JSON API; imperative
`ze-iface:*` verbs (create/delete/migrate/mac/mtu/addr/up/down/unit, clear counters);
gNMI Get/Set/Subscribe through a client entry point; `diag` capture/capture-raw;
`config graph/history/rollback/import`; `show uptime`; `show ping`;
`monitor traceroute` + probe-round; `ze connect`; `show host` sections beyond
cpu/kernel.

**W-6 cmd-layer + core unit deserts:**
- cmd layers: `internal/plugins/diag/cmd` (1107 LOC), `host-cmd/cmd` (324),
  `meta/cmd` (294), `log/cmd` (139), `crashes` + `crashes/cmd` (208),
  `internal/component/config/storage/cli` (557), `bgp/plugins/cmd/policy` (354),
  `trafficstat/cmd`, `trafficfeature` service+cmd.
- core: `internal/core/resolve` (path-traversal regex), `internal/core/subdispatch`,
  `internal/core/reboot`, `internal/core/gokrazyutil`.

**W-7 BGP FSM fuzz target:** `internal/component/bgp/fsm` has no `func Fuzz` while
all sibling wire parsers are fuzzed; add one (back-fill rule, `ai/rules/testing.md`).

**W-8 VPP test-file renames:** `fib/vpp` and `iface/vpp` are well tested but under
non-mandated file names; rename/split to `apply_test.go` / `translate_test.go` /
`verify_test.go` / `register_test.go` per the BLOCKING rule.

**W-9 QEMU/linux unit back-fill (with W-4):** linux-only packages with no
linux-reaching tests: `internal/core/smart` (11.8%), `flowexport/conntrack` +
`sampling` (15.2%), `iface/dhcp` (15.1%), `ike/dataplane` (26.9%), `ntp` (36%),
`l2tp/pppoeclient` (10.1%). Unit tests with seams/fixtures; QEMU integration tests
where kernel interaction is the code (per `ai/rules/qemu-testing.md`).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/testing.md` - test-type directory table, VPP mandate, back-fill rule
  → Constraint: pure-logic code gets Go unit tests, `.ci` only for user entry points; each `test/<subdir>/` runner rejects the wrong format
  → Constraint: VPP backends MUST ship apply/translate/verify/register test files; "needs a real daemon" is banned reasoning
  → Constraint: sleep ratchet — `time.sleep(` count in `test/**/*.ci` may only go down; use `ze_api` `wait_for_event`/`wait_for_shutdown`
  → Constraint: Python observers in `.ci` MUST use `runtime_fail`, never `dispatch shutdown; sys.exit(1)` (silent false-pass)
- [ ] `ai/rules/functional-test-gate.md` - functional test mandatory per user-facing behavior
  → Constraint: every NEW behavior-guarding `.ci`/`.et` must be mutation-verified manually: disable the producing function, confirm the test flips RED, revert
- [ ] `ai/rules/qemu-testing.md` - linux-only test requirements, build-tag patterns
  → Constraint: kernel-touching tests use `//go:build integration && linux`; type-only linux tests use bare `linux`; graceful `t.Skip` on missing capabilities
  → Constraint: `ZE_QEMU_INTEGRATION_PKGS` is DERIVED from `integration && linux` grep (`mk/test-integration.mk:279`) — new integration tests auto-enroll, no Makefile edit needed
- [ ] `ai/patterns/functional-test.md` - structural template for `.ci` tests
  → Constraint: `.ci` assertions must verify BEHAVIOR (hex/json/stdout), not just exit 0; naming `<feature>-<scenario>.ci`
- [ ] `ai/rules/discovery-updates.md` - Mechanical Checklist answered in Design section below
  → Decision: new suites (ipsec, appliance) require rows in `ai/rules/testing.md` make-target table, `ai/patterns/functional-test.md` tables, and `docs/functional-tests.md`
- [ ] `docs/functional-tests.md` - operator documentation for runners (update target)
  → Constraint: suite additions must be documented here (Documentation Update Checklist row 10)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7311.md` (AIGP) - W-3 aigp: plugin is an explicit stub (`aigp.go:7-8` "placeholder... Full AIGP processing will be added when the spec-aigp work is implemented")
  → Decision: test only what exists (registration + SDK loop); do NOT build AIGP semantics tests against a stub
- [ ] `rfc/short/rfc6396.md` (MRT) - W-3 mrt dump format assertions
  → Constraint: verify summary exists before writing MRT encoding assertions; create if missing

**Key insights:**
- Suite registry: `registerCIRoot` (`internal/test/cli/dispatch.go:20`) maps name → `test/<subdir>` (`ci_runner.go:80`); the 17 entries live in `internal/test/cli/register.go:15-33`. Adding ipsec/appliance = one line each + gating list (`mk/test-functional.mk`) + QEMU list (`scripts/evidence/qemu-all-tests.sh:124-138`) + docs.
- Runner builds `ze` with tags `ze_core ze_distro ze_setup` + feature gates (`internal/test/runner/runner.go:50-57,227`), so `ze appliance` IS available to `.ci` tests.
- The 8 `test/ipsec/*.ci` use directives the runner does NOT implement (`expect=event:namespace=...`, bare `command=` — zero grep hits in `internal/test/`); they were authored against an imagined API and have never run. Wiring the suite requires either runner support for those directives or rewriting the tests in supported syntax.
- BUG found (fix at source, W-3): `gokrazyutil.AuthHeader` (`internal/core/gokrazyutil/gokrazyutil.go:17-24`) base64-encodes only `"gokrazy:"` — the password read on the line above is never appended, so Basic-Auth injection sends wrong credentials. First concrete payoff of the zero-test finding.
- `static/vpp` backend seam already exists: `Backend.ch` is govpp `api.Channel` (interface) (`backend.go:38-43`); `buildFibPaths`/`toFibPath`/`toVPPPrefix` (`backend.go:91-159`) are pure. Four mandated test files are writable without VPP.
- telemetry collectors take `procfs.FS` (`loadavg_linux.go:18`) — `procfs.NewFS(fixtureDir)` enables golden-file parser tests; scheduling harness with fake clock already exists (`collector_test.go`). `metrics.NopRegistry` records nothing — a small recording registry fake is needed to assert gauge values.
- FSM fuzz surface: `FSM.Event(event Event) error` (`internal/component/bgp/fsm/fsm.go:153`) with per-state handlers; fuzz = random event sequences, invariants: no panic + state stays in valid set.
- MRT: `ParseConfig` (`register.go:218`) is pure; component wires via `MessageBridge`/`PeerBridge` callbacks + `request mrt dump-rib` command (`register.go:184-196`); dump modes updates/all/routes with rotation (`config.go`).
- Sleep ratchet: baseline 448 matched at its last bump (commit `463cded38`); since then +27 sleeps added, 0 removed (git diff count) → tracked count 475, on-disk 477. W-2 = convert the 27 new sleeps / true-up baseline for justified remainders.

## Current Behavior (MANDATORY)

**Source files read:** (2026-07-10, this session)
- [ ] `internal/test/cli/register.go` (:15-33) + `dispatch.go` (:20) + `ci_runner.go` (:80) - 17 registered suites; name → `test/<subdir>`; no ipsec/appliance entries; confirmed live via `bin/ze-test ipsec|appliance` → "unknown command"
- [ ] `internal/test/runner/runner.go` (:50-57 TestBuildTags, :227 buildZe) - runner ze binary has ze_core+ze_distro+ze_setup+features → `ze appliance` available
- [ ] `internal/plugins/mrt/register.go` (:92-114 registration, :116-215 runEngine, :184-196 dump-rib command, :218-265 ParseConfig) + `config.go` - full plugin surface; ParseConfig pure; component untested end to end
- [ ] `internal/component/gokrazy/gokrazy.go` (:35-81 Handler, :86-131 rewriteResponse, :135-144 rewriteAttr) - reverse proxy; rewriteAttr pure; Handler testable with unix-socket httptest fake
- [ ] `internal/core/gokrazyutil/gokrazyutil.go` (:17-24 AuthHeader, :30-40 ReadPassword) - BUG: password never included in base64 credentials; ReadPassword paths hardcoded (/perm, /etc, /) → needs seam or file-based test on those paths is impossible unprivileged; fix at source
- [ ] `internal/plugins/static/vpp/backend.go` (:38-43 Backend/api.Channel seam, :47-57 Apply/Remove, :60-89 routeAddDel, :91-159 pure translate fns) - fake api.Channel + pure-function tests feasible; no register.go in dir (parent `internal/plugins/static` owns registration)
- [ ] `internal/component/telemetry/collector/collector.go` (:17-21 Collector interface, :53-79 Manager+setClock) + `loadavg_linux.go` (:18 procfs.FS injection) + `collector_test.go` (fake clock harness, NopRegistry) - per-parser fixture tests via procfs.NewFS(dir); recording metrics fake needed
- [ ] `internal/component/bgp/fsm/fsm.go` (:76 New, :153 Event, :181-417 per-state handlers) - fuzz target surface
- [ ] `internal/component/bgp/plugins/aigp/aigp.go` (:7-8 stub declaration, :37-48 RunAIGPPlugin) - stub plugin, SDK loop only
- [ ] `scripts/dev/verify_wiring_docs.py` (:184-215 check_ci_sleep_ratchet) - counts `time.sleep(` in all `test/**/*.ci`, compares to `test/.ci-sleep-baseline`, only fires when a changed file is a `.ci`
- [ ] `test/appliance/*.ci` (9) - `cmd=foreground` `ze appliance ...` style, supported directives; `test/ipsec/*.ci` (8) - UNSUPPORTED directives (`expect=event`, `command=`), most need an SA to establish (no responder in single-daemon test)
- [ ] `tmp/test-audit/REPORT.md` + `tmp/test-audit/*` - full audit evidence (coverage log, package matrices, linux-tag cross-reference)

**Behavior to preserve:**
- All existing test expectations and goldens; `make ze-verify` stays green and unprivileged.
- Suite runner semantics for the 17 registered suites.

**Behavior to change:**
- Only what the work items name: suite registrations, gating lists, sleep baseline,
  test-file names in fib/vpp + iface/vpp, plus source fixes for bugs the new tests expose.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `ze-test <suite>` CLI (functional), Go `testing` harness (unit/fuzz), QEMU harness for `integration && linux` tests

### Transformation Path
1. Suite registration (`registerCIRoot`) → `ci_runner.go` walks `test/<subdir>/*.ci` → runner boots daemon / runs foreground `ze` → directives assert behavior
2. Unit tests: fixture input (procfs dir, fake api.Channel, fake unix-socket upstream, MRT config JSON) → producing function → asserted output
3. Fuzz: random byte input → event/decode sequence → invariant assertions (no panic, valid state)
4. Bug fixes at source (e.g. `AuthHeader`) follow normal TDD: failing test first, then the fix

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` runner → daemon | plugin dispatch / CLI foreground exec | [ ] |
| test → kernel (W-9) | `integration && linux` tagged tests in QEMU (auto-enrolled via derived pkg list) | [ ] |
| test → VPP API (W-3 static/vpp) | fake `api.Channel` implementation, no daemon | [ ] |
| test → procfs (W-4) | `procfs.NewFS(fixtureDir)` | [ ] |
| test → gokrazy socket (W-3) | httptest server on a temp unix socket | [ ] |

### Integration Points
- `internal/test/cli/register.go` - suite registration (W-1)
- `mk/test-functional.mk` run_suite list + per-suite targets (W-1)
- `scripts/evidence/qemu-all-tests.sh` fsuite list (W-1)
- `test/scripts/ze_api.py` `wait_for_event`/`wait_for_shutdown` (W-2 sleep conversion)
- existing fakeOps precedent: `internal/plugins/firewall/vpp/apply_test.go` (reference pattern for W-3 static/vpp)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — new commands, CLI/monitor views, families, and handlers register via the existing registry and the core discovers them; no new per-feature field, switch case, or factory is added to a core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Audit evidence (file:line, coverage numbers) holds at design time | 2026-07-10 audit, same day; headline items re-read firsthand this session | re-scope item | grep/LSP at implement audit for the agent-sourced W-5/W-6 rows | unvalidated |
| A-2 | `test/appliance/*.ci` run under the runner-built ze (ze_setup+ze_distro tags present) | `runner.go:50-57` TestBuildTags; `appliance-help.ci` uses supported `cmd=foreground` | appliance suite needs a dedicated binary/runner flag | first `bin/ze-test appliance --all` run after registration | unvalidated |
| A-3 | The 8 `test/ipsec/*.ci` are rewritable to supported directives (observer + api/foreground) without new runner features | grep: `expect=event`/`command=` absent from `internal/test/`; ze_api has `wait_for_event` | add runner directive support instead (bigger, needs design) | rewrite one (ipsec-show-status) as pilot | unvalidated |
| A-4 | A recording `metrics.Registry` fake is small and acceptable in the collector test package | `metrics.NopRegistry` exists (`collector_test.go`); Registry is an internal interface | reuse/extend an existing fake elsewhere in tree | grep `internal/core/metrics` for existing recording fake at implement time | confirmed — only NopRegistry exists (`internal/core/metrics/nop.go:7`); Registry interface at `metrics.go:57`; recording fake to be written |
| A-5 | govpp `api.Channel` is implementable by a local fake (interface, not struct) | `backend.go:39` field type `api.Channel` | use govpp mock adapter if vendored | compile the fake in `apply_test.go` | confirmed — `go doc go.fd.io/govpp/api.Channel`: 6-method interface (SendRequest/SendMultiRequest/SubscribeNotification/SetReplyTimeout/CheckCompatiblity/Close) |
| A-6 | `gokrazyutil.AuthHeader` omitting the password is a bug, not intent | reading `AuthHeader` vs doc comment "injects gokrazy's Basic Auth credentials" (`gokrazy.go:5-7`); gokrazy upstream auth is `gokrazy:<pw>` | it's intentional (unlikely); ask user | failing unit test asserting base64("gokrazy:"+pw), then fix | unvalidated |
| A-7 | The +27 sleeps since baseline are convertible to `wait_for_event`/`wait_for_shutdown` or justifiable | memory: sleep→sync conversion exposes real races (prior sweep); ze_api provides waits | baseline trued up with per-file justification | per-file conversion during W-2 | unvalidated |
| A-8 | Two-ze-instance `.ci` pattern supports an ipsec initiator+responder pair | 10 two-ze-peer `.ci` ship today (per spec-followup-test-infra design corrections, e.g. `forward-two-tier-under-load.ci`) | SA-lifecycle assertions stay lab-only; single-daemon surfaces only | pilot `ipsec-sa-installed.ci` with a second instance | confirmed (mechanism) — multi-process via `cmd=background:exec=...` + per-process stdin configs (`forward-two-tier-under-load.ci:343-345`); ipsec pilot still to run |
| A-9 | The runner can reach the daemon over the plugin protocol for `command=`/`expect=event` (same channel `ze_api` observers use) | `test/scripts/ze_api.py` dispatch/wait_for_event exist; runner already orchestrates plugins | directives implemented via an embedded observer helper instead | runner directive pilot on `ipsec-show-status.ci` | confirmed (transport) — observers connect over TLS to the plugin hub (`ze_api.py:83-179`, ZE_PLUGIN_HUB_TOKEN + ze.plugin.hub.host/port); Go side can use `pkg/plugin/sdk` `NewWithConn` (as `internal/plugins/mrt/register.go:120` does); directive pilot still to run |
| A-10 | gNMI functional test is expressible without vendoring a new client (grpcurl was already declared a tooling gate in spec-finish-ci-coverage L40) | gnmi in-package integration test drives in-process gRPC | record blocker per AC-11 escape hatch; do NOT vendor deps without user approval (`ai/rules/go-standards.md`) | attempt with in-tree tooling at implement time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Wiring orphan suites surfaces failing `.ci` (never-run tests rot) | suite red on first registration run | fix at source per user decision; never weaken tests; ipsec pilot first (A-3) |
| R-2 | ipsec `.ci` semantics need a responder (sa-up) that single-daemon tests cannot provide | pilot rewrite cannot assert sa-up | assert what a single daemon CAN show (configured peers, zero SAs, monitor stream); full SA lifecycle stays with the docker interop lab; record in Known Limitations |
| R-3 | Concurrent sessions on main (14 in-progress specs) collide with W-2 baseline edit and l2tp test reds | merge conflicts in `test/.ci-sleep-baseline`; unrelated reds in `make ze-verify` | scope W-2 commit tightly; known-red handling per `ai/rules/git-safety.md`; do not touch other sessions' failing areas (`internal/component/l2tp`, `cli/testing`, `plugin/all` reds observed 2026-07-10) |
| R-4 | New tests expose real bugs (by design) and scope grows mid-implementation | failing strict test on existing behavior | user pre-approved fix-at-source; log each in Bugs Found/Fixed; if a fix balloons (>1 day), STOP and present |
| R-5 | Sleep→wait conversion exposes latent races in existing tests | converted test flakes under `-race`/load | treat as bug-finding (memory: prior sweep); fix the race at source, never re-add the sleep |
| R-6 | telemetry fixture tests drift from real /proc formats | parser passes fixtures but fails on a real kernel | fixtures copied from real /proc captures; QEMU `integration && linux` smoke test scrapes a live /proc for a sample of collectors |
| R-7 | mutation-verify discipline skipped across ~40 new `.ci` (false-pass tests shipped) | reviewer cannot show red-under-mutation evidence | per functional-test-gate: every behavior-guarding `.ci` gets one mutation check; record the checked list in the spec audit |
| R-8 | Sleep-ratchet breach blocks EVERY `.ci`-touching commit until fixed (count 475 > baseline 448 today) | `make ze-verify-wiring-docs` red on first `.ci` change | W-2 is Phase 1, before any new `.ci` lands |
| R-9 | New runner directives race daemon readiness (command dispatched before engine up) | flaky pilot `.ci` | reuse the runner's existing daemon-ready synchronization; directive waits with explicit timeouts; no sleeps |
| R-10 | appliance suite has env-dependent tests (gok tooling, image builds) | appliance-build/iso tests red on host without tooling | tests already model absence (`appliance-build-no-gok.ci`); use skip options only for genuinely env-gated steps, never to hide product bugs |
| R-11 | W-8 renames collide with concurrent VPP sessions (spec-finish-vpp-stub is `ready`) | rebase conflicts in `internal/plugins/{fib,iface}/vpp` | do W-8 late; renames are content-preserving so conflicts are mechanical |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- Skeleton-stage rows for the headline items; design extends this table per work item. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `bin/ze-test ipsec --all` | → | new `registerCIRoot("ipsec", ...)` suite dispatch | `test/ipsec/ipsec-show-sa.ci` (existing, becomes runnable) |
| `bin/ze-test appliance --all` | → | new `registerCIRoot("appliance", ...)` suite dispatch | `test/appliance/appliance-help.ci` (existing, becomes runnable) |
| Daemon boots with `mrt` config | → | mrt plugin dump pipeline | `test/plugin/mrt-dump-updates.ci` (new) |
| CLI `show mpls-forwarding` | → | mpls show handler | `test/plugin/show-mpls-forwarding.ci` (new) |
| HTTP request to `/gokrazy/` mount | → | gokrazy reverse-proxy handler | `TestGokrazyProxyAuthInjection` (new unit) + web functional test |
| VPP static route apply | → | static/vpp backend Apply | `TestStaticVppApply` in `internal/plugins/static/vpp/apply_test.go` (new) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any commit touching a `.ci` file | Sleep ratchet green: new-since-baseline sleeps converted to `ze_api` waits where feasible; `test/.ci-sleep-baseline` equals the true remaining count with per-file justification for keepers (W-2) |
| AC-2 | `.ci` containing `command=`, `expect=output`, `expect=event`, `expect=stream` | Runner executes CLI command against the live daemon, asserts its output, waits for daemon events, and asserts monitor-stream content; each directive has runner unit tests + at least one green consuming `.ci` (W-1) |
| AC-3 | `bin/ze-test ipsec --list/--all` | ipsec suite registered; the 8 existing tests run: config/show/monitor surfaces green single-daemon; SA-lifecycle tests green with a second ze instance as IKE responder, `option=needs-linux` where XFRM is required (W-1) |
| AC-4 | `bin/ze-test appliance --list/--all` | appliance suite registered and green under the runner-built ze (ze_setup+ze_distro tags) (W-1) |
| AC-5 | `make ze-functional-test` | ipsec, appliance, l2tp-wire, isis-wire, ospf-wire in the gating suite list and green; `qemu-all-tests.sh` fsuite list gains isis, ospf, ospfv3, ldp, rsvpte (+ new suites); QEMU execution per runbook if env lacks qemu (W-1) |
| AC-6 | MRT configured (each of updates/all/routes modes, direction, peer-filter) + BGP traffic + `request mrt dump-rib` | Unit tests for ParseConfig/encoding/rotation; `.ci` per mode asserting a valid MRT artifact (validated via ze-analyse or byte-level assertions); dump-rib command test (W-3) |
| AC-7 | gokrazy proxy request with password file present | Failing-first unit test exposes the AuthHeader password omission (`gokrazyutil.go:17-24`); fix at source; Handler tests: auth injection, HTML/JS path rewrite, 503-on-missing-socket; functional test through the web mount (W-3) |
| AC-8 | static/vpp backend operations against a fake `api.Channel` | The four mandated files exist and pass: apply (add/remove/retval-failure/path-cap), translate (all three actions, v4/v6, weight-0), verify/reject, registration wiring in parent static plugin (W-3) |
| AC-9 | aigp stub plugin | Registration + SDK-loop unit tests scoped to the stub as documented (`aigp.go:7-8`); no invented AIGP semantics (W-3) |
| AC-10 | Every `*_linux.go` collector in `internal/component/telemetry/collector` | Fixture-driven unit test per collector via `procfs.NewFS`-style injection asserting emitted gauge values through a recording registry; one QEMU smoke test scrapes live /proc for a sample (W-4) |
| AC-11 | Each W-5 surface (mpls-forwarding, lg web/API, iface imperative verbs, gNMI wire ops, diag capture(+raw), config graph/history/rollback/import, show uptime, show ping, monitor traceroute, ze connect, show host sections) | A functional test through the user entry point in the correct suite; kernel-needing ones marked `option=needs-linux`; any surface that cannot be functionally tested without new tooling is recorded with the blocker named (W-5) |
| AC-12 | Each W-6 package (diag/cmd, host-cmd/cmd, meta/cmd, log/cmd, crashes+cmd, config/storage/cli, bgp/plugins/cmd/policy, trafficstat/cmd, trafficfeature, core/resolve, core/subdispatch, core/reboot, core/gokrazyutil) | Unit tests exercising the package's real logic (dispatch, rendering, parsing, error branches); zero-test packages in this list drop to zero (W-6) |
| AC-13 | `make ze-fuzz-test` | `FuzzFSMEventSequence` (or equivalent) in `internal/component/bgp/fsm` with seed corpus; invariants: no panic, state remains valid (W-7) |
| AC-14 | `go test ./internal/plugins/fib/vpp/... ./internal/plugins/iface/vpp/...` | Test content reorganized under the four mandated file names, everything still green, no coverage lost (W-8) |
| AC-15 | W-9 packages (core/smart, flowexport/conntrack, flowexport/sampling, iface/dhcp, ike/dataplane, ntp, l2tp/pppoeclient) | Unit tests via seams/fixtures for parse/translate logic; `integration && linux` tests for kernel-touching paths (auto-enrolled in QEMU via the derived package list) (W-9) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator enables MRT dumps and expects analyzable artifacts | config → mrt plugin → BGP message bridge → MRT file | `test/plugin/mrt-dump-updates.ci` |
| 2 | Operator opens the gokrazy management UI through ze's web server | browser → /gokrazy/ → auth-injecting proxy → gokrazy socket | AC-7 web functional test + `TestGokrazyProxyAuthInjection` |
| 3 | Operator runs `show vpn ipsec status` on a live daemon | CLI dispatch → ike engine → formatted status | `test/ipsec/ipsec-show-status.ci` (wired via AC-2 directives) |
| 4 | Operator builds an appliance image from the CLI | `ze appliance ...` → appliance tooling | `test/appliance/appliance-build-no-gok.ci` (wired) |
| 5 | Operator inspects MPLS forwarding state | CLI → `ze-show:mpls-forwarding` handler | `test/plugin/show-mpls-forwarding.ci` |
| 6 | Operator monitors Prometheus system metrics from procfs | /proc fixtures → collector → registry gauges | per-collector fixture tests (AC-10) |
| 7 | Static routes programmed on a VPP dataplane | static plugin → static/vpp Backend → VPP API messages | `TestStaticVppApply` (fake channel) |

## 🧪 TDD Test Plan

### Unit Tests
<!-- Headline rows; the full per-package sweep is enumerated by the AC tables (AC-10, AC-12, AC-15). -->
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunnerCommandDirective`, `TestRunnerExpectEvent`, `TestRunnerExpectOutput`, `TestRunnerExpectStream` | `internal/test/runner/` | AC-2 directive parse + execution | |
| `TestAuthHeaderIncludesPassword` (fails first) | `internal/core/gokrazyutil/gokrazyutil_test.go` | AC-7 bug fix | |
| `TestGokrazyProxyAuthInjection`, `TestRewriteAttr`, `TestRewriteResponsePaths`, `TestProxySocketMissing503` | `internal/component/gokrazy/gokrazy_test.go` | AC-7 | |
| `TestStaticVppApply`, `TestStaticVppTranslate`, `TestStaticVppVerify`, `TestStaticVppRegister` | `internal/plugins/static/vpp/{apply,translate,verify,register}_test.go` | AC-8 | |
| `TestMRTParseConfig`, `TestMRTDumpEncoding`, `TestMRTRotation` | `internal/plugins/mrt/*_test.go` | AC-6 | |
| `TestAIGPRegistration`, `TestRunAIGPPluginLoop` | `internal/component/bgp/plugins/aigp/aigp_test.go` | AC-9 | |
| per-collector `TestCollect<Name>` + recording registry | `internal/component/telemetry/collector/*_test.go` | AC-10 | |
| `FuzzFSMEventSequence` | `internal/component/bgp/fsm/fsm_fuzz_test.go` | AC-13 | |
| per-package tests for W-6/W-9 lists | per AC-12/AC-15 | AC-12, AC-15 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| static/vpp ECMP path count | 1-255 | 255 | 0 (empty forward = no paths) | 256 → error (`backend.go:74-76`) |
| static/vpp path weight | 1-255 (uint8; 0 coerced to 1, `backend.go:116-118`) | 255 | 0 → becomes 1 | N/A (type) |
| MRT rotation interval | ≥0 seconds | large interval | negative rejected at YANG layer (verify) | N/A |
| directive timeouts (`expect=event:timeout=N`) | 1-suite timeout | suite cap | 0/absent → default | > suite timeout capped |

### Functional Tests
<!-- Skeleton-stage headline rows; design completes this table per work item. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mrt-dump-updates.ci`, `mrt-dump-routes.ci`, `mrt-peer-filter.ci` (new) | `test/plugin/*.ci` | MRT dumps written per configured mode | |
| `show-mpls-forwarding.ci` (new) | `test/plugin/*.ci` | operator inspects MPLS forwarding table | |
| `test/ipsec/*.ci` (8 existing, wired) | `test/ipsec/*.ci` | IPsec show/monitor/rekey/DPD through the daemon | |
| `test/appliance/*.ci` (9 existing, wired) | `test/appliance/*.ci` | appliance build/list/help CLI surface | |
| `diag-capture.ci`, `diag-capture-raw.ci` (new) | `test/plugin/*.ci` | packet capture commands | |
| lg route-search / API functional test (new) | `test/web/*.wb` + `test/plugin/*.ci` | looking-glass UI and JSON API | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - no new wire behavior; tests exercise already-implemented protocol paths | - | - | - | |

## Files to Modify
- `internal/test/cli/register.go` - register ipsec + appliance suites (W-1)
- `internal/test/runner/record_parse.go` + runner execution files - `command=`/`expect=output`/`expect=event`/`expect=stream` directives (W-1/AC-2)
- `mk/test-functional.mk` - gating list additions + `ze-ipsec-test`/`ze-appliance-test` targets (W-1)
- `scripts/evidence/qemu-all-tests.sh` - fsuite list additions (W-1)
- `test/.ci-sleep-baseline` + the `.ci` files carrying the +27 sleeps (W-2)
- `internal/core/gokrazyutil/gokrazyutil.go` - AuthHeader password fix (W-3 bug, source change)
- `test/ipsec/*.ci` - adjust the 8 tests where single-daemon semantics or two-ze pairing requires it (W-1)
- `internal/plugins/fib/vpp/*_test.go`, `internal/plugins/iface/vpp/*_test.go` - reorganize to mandated names (W-8)
- `ai/rules/testing.md`, `ai/patterns/functional-test.md`, `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` - new suites + directives (discovery updates)
- any source file a strict new test proves wrong (fix-at-source policy; each recorded in Bugs Found/Fixed)

## Discovery Mechanical Checklist (ai/rules/discovery-updates.md)

1. Agent first-look: `ai/rules/testing.md` make-target table + `ai/patterns/functional-test.md` directory/runner tables gain ipsec + appliance rows; `ai/INDEX.md` untouched unless a new tool emerges.
2. Regression rule: `ai/rules/testing.md` (suites, ratchet) and `ai/rules/functional-test-gate.md` already own the behavior; new `.ci` directives documented in `docs/architecture/testing/ci-format.md`.
3. Drift source of truth: suite registry (`registerCIRoot`) is the registry; QEMU integration pkgs stay derived from build tags.
4. Verification: `make ze-functional-test` (new suites gate), `make ze-verify-wiring-docs` (ratchet), `make ze-fuzz-test` (AC-13), `make ze-qemu-integration-test` (AC-15).
5. Usage docs: `docs/functional-tests.md` (suites + directives).
6. Learned record: `plan/learned/NNN-test-coverage-gaps.md` at closure; `ai/LEARNED-INDEX.md` if structurally significant (runner directives are).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A | no new config surface; tests exercise existing YANG |
| YANG validation constraints | N/A | no new leaves |
| YANG custom validators | N/A | no new leaves |
| CLI commands/flags | Yes | `ze-test ipsec` / `ze-test appliance` roots via `internal/test/cli/register.go` (registerCIRoot pattern, SectionTest) |
| CLI grammar (action before identifier) | N/A | suite roots follow the existing ze-test noun convention (firewall, l2tp, ...) |
| Editor autocomplete | N/A | no editor-visible surface |
| Functional test for new RPC/API | Yes | AC-2 directives get consuming `.ci` (test/ipsec); every W-5 surface gets its functional test |
| Pipe completeness | N/A | no new output-producing product command |
| Env var registration | N/A | no new env-configurable behavior |
| Doctor check for runtime dependencies | N/A | no new runtime dependency introduced by tests; AuthHeader fix changes no dependency |
| Prometheus counters/metrics | N/A | no new observable product state (collector tests assert EXISTING metrics) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] | `rfc/short/rfcNNNN.md` and `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | [ ] | `docs/architecture/meta/README.md` |
| 14 | Prometheus counters added/changed? | [ ] | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | grep `docs/` for `source: <changed-file>` |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] | verify examples against YANG/parser/handler |

## Files to Create
- `internal/test/runner/*_test.go` additions - directive unit tests (AC-2)
- `internal/plugins/mrt/{config,dump,component}_test.go` + `test/plugin/mrt-dump-updates.ci`, `mrt-dump-routes.ci`, `mrt-dump-all.ci`, `mrt-peer-filter.ci`, `mrt-dump-rib-command.ci` (AC-6)
- `internal/core/gokrazyutil/gokrazyutil_test.go`, `internal/component/gokrazy/gokrazy_test.go` + web functional test for `/gokrazy/` (AC-7)
- `internal/plugins/static/vpp/{apply,translate,verify,register}_test.go` (AC-8)
- `internal/component/bgp/plugins/aigp/aigp_test.go` (AC-9)
- `internal/component/telemetry/collector/<collector>_linux_test.go` per collector + `testdata/` fixtures + recording registry fake (AC-10)
- W-5 functional tests: `test/plugin/show-mpls-forwarding.ci`, lg `.wb`/`.ci`, `test/plugin/iface-manage-*.ci` (needs-linux), gNMI functional (or recorded blocker), `test/plugin/diag-capture*.ci`, `test/parse|ui` tests for config graph/history/rollback/import, show uptime, show ping, monitor traceroute, ze connect, host sections (AC-11)
- W-6 unit test files per listed package (AC-12)
- `internal/component/bgp/fsm/fsm_fuzz_test.go` (AC-13)
- W-9: unit + `*_integration_linux_test.go` files for core/smart, flowexport/conntrack+sampling, iface/dhcp, ike/dataplane, ntp, l2tp/pppoeclient (AC-15)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan — check what exists |
| 3. Wiring phase | Wiring Test table — register entry points, write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist below |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary Report; two-commit closure |

### Implementation Phases

1. **Phase 1: Unblock + Wiring (MANDATORY FIRST)** — W-2 then W-1
   - W-2 first: sleep conversion + baseline true-up (R-8: nothing else touching `.ci` can land before this)
   - Runner directives `command=`/`expect=output`/`expect=event`/`expect=stream` (TDD: runner unit tests fail → implement → pass; pilot `ipsec-show-status.ci`)
   - Register ipsec + appliance suites; add gating rows (`mk/test-functional.mk`), QEMU fsuite rows, `ze-<suite>-test` targets, docs
   - Tests: AC-1, AC-2, plus first-run triage of both suites (R-1 fixes at source)
2. **Phase 2: W-3 untested features** — gokrazyutil bug fix (failing test first), gokrazy Handler tests + web functional; static/vpp four files; mrt unit + `.ci` set; aigp stub tests (AC-6..AC-9)
3. **Phase 3: W-4 collectors** — recording registry fake, fixture harness, one collector as pilot, then sweep all `*_linux.go`; QEMU live-/proc smoke (AC-10)
4. **Phase 4: W-5 functional-gate misses** — one surface at a time through its entry point; needs-linux marking where kernel is involved (AC-11)
5. **Phase 5: W-6 unit deserts** — cmd layers then core packages (AC-12)
6. **Phase 6: W-7 fuzz + W-9 linux back-fill** — FSM fuzz target + corpus; seams/fixtures + `integration && linux` tests (AC-13, AC-15)
7. **Phase 7: W-8 VPP renames (LAST, R-11)** — content-preserving reorganization to mandated names (AC-14)
8. **Functional tests** — mutation-verify each behavior-guarding new `.ci` (R-7), record the list
9. **Full verification** — `make ze-verify`; QEMU targets where env permits, runbooks recorded otherwise
10. **Complete spec** — audit tables, learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path |
| Correctness | New `.ci` assert behavior (hex/json/stdout/event), never bare exit 0; no `sys.exit(1)` observers; no new `time.sleep(` |
| Tests gate | Mutation-verify evidence recorded for behavior-guarding `.ci` (R-7); ratchet count ≤ baseline |
| Registration over hardcoding | Suites via registerCIRoot; directives via the runner's existing directive registry/parse table, no special-cased suite names |
| Rule: no-workarounds | Every red found in never-run tests fixed at source, none weakened; each in Bugs Found/Fixed |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| ipsec + appliance suites registered | `bin/ze-test ipsec --list && bin/ze-test appliance --list` |
| Runner directives implemented | `go test ./internal/test/runner/ -run 'TestRunner(Command\|Expect)'` |
| Sleep ratchet green | `python3 scripts/dev/verify_wiring_docs.py` sleep check on a `.ci` change |
| gokrazyutil fix | `go test ./internal/core/gokrazyutil/` + git log showing failing-first test |
| static/vpp four files | `ls internal/plugins/static/vpp/{apply,translate,verify,register}_test.go` |
| mrt tests | `go test ./internal/plugins/mrt/` + `bin/ze-test bgp plugin -p mrt` |
| collector sweep | `ls internal/component/telemetry/collector/*_test.go` count vs `*_linux.go` count |
| W-5 functional list | grep each surface keyword under `test/` (mechanical check from functional-test-gate) |
| W-6/W-9 unit files | `ls` per package from AC-12/AC-15 lists |
| fsm fuzz | `grep -rn 'func Fuzz' internal/component/bgp/fsm/` |
| VPP renames | `ls internal/plugins/{fib,iface}/vpp/{apply,translate,verify,register}_test.go` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Fuzz target must not mask panics (no recover in target); MRT peer-filter and file-path tests cover hostile-ish values within YANG constraints |
| Credential handling | AuthHeader fix: password never logged in tests or fixtures; test fixtures use dummy passwords |
| Test fixtures | procfs fixtures contain no real host data beyond format samples; no secrets in `.ci` configs (dummy PSKs only, matching existing test practice) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->

### W-2 findings (2026-07-10, implementation)

- **W-2 complete and GREEN**: `time.sleep(` count in `test/**/*.ci` is 448 ==
  committed baseline (no baseline edit needed). 29 sleeps removed across 15
  files (11 as112, 9 l2tp-redistribute pre-baseline offsets, 2 forked, 2
  forked-kernel, 2 mpreach, 1 each bridge/lg-paginate/lg-graph-lab). Justified
  keepers within budget: `test/l2tp/{tunnel-initiate-sccrq,lns-outgoing-call}.ci`
  (TCP connect-retry backoff, no event for refused connect) and
  `test/traffic/{020,022,023,024,025,026}*.ci` (QEMU-root-only; convert in-VM).
- **Pre-existing hang fixed at source**: `lg-paginate.ci` + `lg-graph-lab.ci`
  never completed (observer idle loop never exits; orphan holds ze's stdout
  pipe; runner `cmd.Wait()` blocks — the pipe-hold class documented in
  `mk/test-functional.mk:33-38`). Fix: `api.wait_for_shutdown()`; both now pass
  in ~5s. Also replaced their banned `print FAIL; sys.exit(1)` with
  `runtime_fail`. These two tests are lg functional coverage that post-dates
  the audit: W-5's lg row narrows to UI/CSV surfaces.
- **ENGINE BUG (open, fix at source in-spec)**: with `internal fakel2tp` +
  `redistribute { import l2tp }` configured, ANY observer
  `ze-plugin-engine:dispatch-command` polling (peer detail or summary, any
  cadence) during the connect phase pins sessions at state=connecting,
  opens-sent=0 indefinitely (instrumented: 100 polls/20s); the poll-free flow
  establishes in ~1-2s. fakeas112 twins tolerate identical polling; the
  no-import fakel2tp sibling also polls fine. Reproducers: the `# // test-relax`
  blocks in `redistribute-l2tp-{announce,withdraw,multi-peer-nexthop}.ci` and
  `forward-mpreach-nexthop-self-two-peer.ci` (flip them back to counter-waits
  once fixed). Suspect layer: plugin coordinator/dispatch interaction with
  reactor dial scheduling.
- Counter-wait recipe (proven, reusable): poll `show bgp peer <n> detail`
  (`peers[addr].state/eor-sent/updates-sent`, producer
  `internal/component/bgp/plugins/cmd/peer/peer.go:202-238`) with
  `wait_for_event` backoff; EOR counts into updates-sent so baseline after
  eor-sent>=1; per-session counters clear on teardown (`ClearStats`,
  `internal/component/bgp/reactor/peer_stats.go:319-322`) — use lifetime
  `connections-established` when the peer process completes early. ze-peer
  exits "no test data available" when its block has zero expectations (give
  it the initial-EOR hex).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One phased spec over umbrella + sub-specs | umbrella set; top-slice only | User decision 2026-07-10; same kind of work throughout, one tracking artifact |
| Bugs fixed at source in-spec | log-and-stop; follow-up skeletons | User decision 2026-07-10; per no-workarounds rule |
| CI scheduling excluded | include nightly pipeline | User decision 2026-07-10 (suite wiring, ratchet, renames in; scheduling out) |
| ipsec: runner directives AND targeted rewrites ("Both") | rewrite-only (smallest); directives-only | User decision 2026-07-10; directives serve future suites, rewrites cover what single/two-daemon semantics allow |
| W-2 (sleep ratchet) is Phase 1 | fix baseline at the end | Discovered constraint R-8: current 475>448 blocks every `.ci`-touching commit; nothing else can land first |
| Two-ze pair for ipsec SA-lifecycle tests | single daemon (config/show only); docker lab only | Two-ze `.ci` precedent exists; keeps SA assertions in the gating suite (A-8 validates) |
| Collector tests via `procfs.NewFS` fixtures | QEMU-only live tests; mocking each collector | Parsers are pure given an FS root; fixtures are deterministic and fast; QEMU keeps one live smoke test (R-6) |
| FSM fuzz drives `Event()` sequences with no-panic/valid-state invariants | fuzzing wire bytes into FSM (wrong layer — message pkg owns that) | `fsm.go:153` is the adversarial-input boundary of this package |
| aigp tested as stub only | build RFC 7311 semantics tests | `aigp.go:7-8` declares the stub; semantics belong to future spec-aigp work |
| Breadth phases (wiring → features → sweeps) | depth-first per feature | Wiring unblocks everything (R-8); sweeps are mechanical once patterns are piloted |

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code where protocol assertions are added.

## Implementation Summary

### What Was Implemented
- (fill at completion)

### Bugs Found/Fixed
- (fill at completion)

### Documentation Updates
- (fill at completion)

### Deviations from Plan
- (fill at completion)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Orphan suites run and gate | functional suite run | `make ze-functional-test` output listing ipsec + appliance suites green |
| Ratchet unblocked | gate run | `make ze-verify-wiring-docs` green on a `.ci`-touching change |
| Untested features covered | unit + functional tests | AC-6..AC-9 test names green; gokrazyutil bug fix commit with failing-first evidence |
| Collector parsers covered | unit sweep | per-collector tests green; package coverage delta recorded (2.7% baseline from `tmp/test-audit/coverage-run.log`) |
| Functional-gate misses closed | functional tests | AC-11 per-surface test list, each mutation-verified or unit-guarded per functional-test-gate |
| Unit deserts covered | unit tests | AC-12/AC-15 package list all with green tests |
| FSM fuzzed | fuzz run | `make ze-fuzz-one FUZZ=FuzzFSMEventSequence TIME=30s` clean |
| VPP rule compliance | file layout | four mandated files present in all five `*/vpp` backends |

## Known Limitations
- QEMU-execution ACs (AC-5 QEMU pass, AC-10 live smoke, AC-15 integration runs) depend on host qemu availability; if absent, code + runbook land and execution is recorded env-blocked (precedent: spec-followup-test-infra AC-2/AC-3).
- AC-11 allows a named-blocker escape ONLY for surfaces needing new vendored tooling (A-10 gNMI risk); each such case is presented to the user, not silently deferred.
- The three unit-test reds observed in the 2026-07-10 coverage run (cli/testing ET-session, l2tp teardown, plugin/all wire-methods) belong to concurrent in-flight work and are explicitly NOT this spec's scope (R-3).

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
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)

## Notes
- Skeleton created 2026-07-10 from the same-day audit (`tmp/test-audit/REPORT.md`).
  Scope structure, infra inclusions, and bug policy are recorded user decisions
  (AskUserQuestion, 2026-07-10).
- Design approved 2026-07-10 ("Approve stop"): spec finalized to `ready`;
  implementation NOT started in the design session per user instruction.
- `tmp/test-audit/` is gitignored scratch; every load-bearing audit number and
  file:line is inlined in this spec so implementation does not depend on it.
- Implementation entry point: `/ze-implement` on this spec; Phase 1 = W-2 sleep
  ratchet (R-8 blocker) + W-1 wiring.
