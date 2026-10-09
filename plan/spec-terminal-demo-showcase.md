# Spec: terminal-demo-showcase

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling \| docs |
| Depends | `plan/immediate/spec-appliance-kernel-vpn-modules.md` (IPsec chapter needs esp4 and xfrm_interface on the render host) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner request (2026-10-09): "we need a new video for the readme and front site
which should [show] the unique feature of ze, like irr download, vpn, etc. all in
one big session ... identify the key things rpki, etc. and then have us configure
it and demonstrate each feature one after the other. It can be a longer video."

Owner amendment (2026-10-09): "we need a recording per topic and a
super-recording."

Two kinds of recording, each made on its own:

- **Topic recordings.** One per storyboard topic, chapters 2 to 9: RPKI, IRR,
  BFD, OSPF, IPsec, VRRP tracking, eBPF traffic, commit-confirmed. Each stands
  alone from a base lab: the topic is typed live in the SSH editor, committed and
  demonstrated. Each is placed on that feature's doc page and in the demo gallery
  (`docs/guide/terminal-demonstrations.md`).
- **Super-recording.** A separate continuous long session: one Ze daemon whose
  config builds up chapter by chapter (chapters 0 to 10 below), each chapter
  committed and then demonstrated. It is recorded on its own, never stitched
  from the topic casts and never sliced into them. Its home is the site
  (asciinema player, front-page hero); the GitHub README embeds it inline when
  GitHub can render it, else links to the site player.

Owner answer on placement (2026-10-09): "the content should be on the site but
if github can have it embedded that would be ideal".

### Owner decisions (2026-10-09, answers to the research questions)

| Decision | Answer |
|----------|--------|
| VPN chapter | Native IKEv2/IPsec in Go, far end a hidden second Ze daemon in its own netns. The kernel modules it needs must also be in the appliance kernel: `plan/immediate/spec-appliance-kernel-vpn-modules.md` |
| WireGuard | Needs runtime testing AND a demo: `plan/spec-wireguard-runtime-proof.md`. Not a chapter of this showcase until that spec lands |
| PPP / L2TP | Skip |
| Chapters | As listed below, in that order |
| Recordings (amendment) | One recording per topic (chapters 2 to 9) plus one super-recording. The super is a separate continuous session, recorded on its own, not stitched from nor sliced into the topic casts |
| Topic placement (amendment) | Each topic recording on its feature's doc page AND in the demo gallery `docs/guide/terminal-demonstrations.md` |
| Super placement (amendment) | Home is the site (asciinema player). The GitHub README embeds the super inline if GitHub can render it (animated SVG, or a video GitHub plays in markdown) within GitHub's size limits; otherwise the README shows a thumbnail linking to the site player. The carrier is a measured outcome of a Work Plan probe, not an owner question (R-4) |
| Length | Super about 7 minutes. The earlier "README gets a separate 60 to 90 s cut as an animated SVG" is superseded: the README carries the super itself, inline or as a linked thumbnail |
| Front page | Switch the hero from cli-dashboard to the super-recording (the earlier decision stands) |
| Configuration style | Everything typed live in the SSH editor and committed. Where a live commit fails to enable a feature, that is a Ze defect to fix, not to work around in the tape |

### Storyboard (from research, to be validated chapter by chapter)

| # | Chapter | Shown | Validator proof |
|---|---------|-------|-----------------|
| 0 | Intro card, ExaBGP migrate | `ze exabgp migrate <file>` | output carries `bgp {` and the peer |
| 1 | Base BGP, live dashboard | `monitor bgp`, sort key, quit | peers Established |
| 2 | RPKI | set rpki cache-server, commit, `show bgp rpki status`, `show bgp adj-rib-in` | VRPs synced; Invalid route held `ineligible` with state 3 |
| 3 | IRR | set plugin/irr server/as-set/import filter, commit, `show bgp irr`, `show bgp irr check` | in-set accepted, out-of-set refused and absent from Adj-RIB-In. Card says the IRR server is a local test server |
| 4 | BFD | set bfd + peer bfd, commit, `show bfd sessions`, link down, `show bgp peer list` | BFD Up, then BGP leaves Established well inside the hold time |
| 5 | OSPF | set ospf area/interface, commit, `show ospf neighbor`, `show ospf route` | Full; FRR loopback route present |
| 6 | IPsec | set vpn ipsec ike/esp groups and peer, commit, `show vpn ipsec sa`, `show vpn ipsec dataplane sa`, ping through tunnel | SA established; kernel SPI listed; packets-in > 0 |
| 7 | VRRP tracking | set vrrp group with `track`, commit, `show vrrp`, tracked link down, `show vrrp` | master, then backup with reduced effective priority. Do NOT reuse runVRRP failover (it kills Ze) |
| 8 | eBPF traffic | set traffic usage, commit, burst, `show traffic usage name traffic0` | source and port counted |
| 9 | Safety net | risky change, `commit confirmed 8`, wait, `show \| compare` | "automatically rolled back" |
| 10 | Recap card | | |

Optional extras the owner did not add: FlowSpec into nftables, config graph, MCP.

## Required Reading

- [ ] `docs/guide/terminal-demonstrations.md`, `docs/contributing/gh-pages.md` - demo commands, image, assets. Silent on scenario design, validators and the README SVG.
- [ ] `internal/le/site/terminaldemo/` - `scenarios.go` (runScenario switch, constants block, `initText`, `demoInstance`, `scenarioConfigDir`), `scenarios_routing.go` (`runRPKI`, `runIRR`), `scenarios_network.go` (`runBFD`, `runOSPF`, `runTraffic`, `runVRRP`, `labCreatePair`, `startFRRPair`), `validate_runtime.go` (`demoValidators`), `render.go` (`containerCommand`, `--privileged` from the manifest), `cards.json`, `tape_wait_test.go`
- [ ] `demos/terminal/manifest.json`, `demos/terminal/Dockerfile` (frr, keepalived, iproute2, nftables; no strongSwan, no wireguard-tools)
- [ ] `docs/guide/ipsec.md`, `test/ipsec/ipsec-sa-installed.ci` (Ze-to-Ze IPsec config shape)
- [ ] `docs/guide/vrrp.md` "Tracking an interface"
- [ ] `docs/guide/config-reload.md` (what a live commit restarts), `internal/component/plugin/server/startup_autoload.go` (`autoLoadForNewConfigPaths`)
- [ ] `internal/le/site/home.go` (`homeHeroDemo`) and `internal/le/site/homebody.go` (hardcodes `cli-dashboard.terminal`, caption, `#live-bgp-dashboard`: a second declaration of the hero)

**Key insights:**
- Every tape wait must match output only its command prints; `TestTapeWaitsAreNotSatisfiedByTheTypedCommand` enforces it.
- `ze config set <file>` offline edits a file path, never a stored name; the store is edited through the SSH editor or `config cat | config set - | config import --yes --name ze.conf -`.
- Demo daemons start with bare `ze start` under `ZE_CONFIG_DIR=<state>/config`, instance name `ze` (= container hostname).
- An RPKI-Invalid route under `invalid reject` stays in Adj-RIB-In marked ineligible; an IRR-refused route is dropped at ingress.
- A loopback NEXT_HOP is treated as withdraw; demo peers announce 192.0.2.2.
- A released BFD session stays AdminDown about 9 s before it retires (RFC 5880 Section 6.8.1).
- The `demoWalkthrough` branches of runBFD/runOSPF/runTraffic/runVRRP print canned `$ ze show ...` lines that are not command output; do not reuse them.

## Current Behavior

**Source files read:**
- [ ] `internal/le/site/terminaldemo/render.go` - `containerCommand` builds the docker run; `--privileged` comes from the manifest; `--hostname` is `demoInstance`
- [ ] `internal/le/site/terminaldemo/scenarios_network.go` - `startFRRPair` runs one FRR protocol daemon under a hardcoded `/run/frr`
- [ ] `internal/le/site/home.go` - `homeHeroDemo = "cli-dashboard"`
- [ ] `internal/le/site/homebody.go` - hardcodes the hero file name, caption and anchor a second time

- 18 single-feature demos exist and were all re-recorded on 2026-10-09 (gh-pages 3252810ae7).
- The README shows `docs/demos/cli-dashboard.svg`, converted by hand from the site cast with svg-term-cli 2.1.1 (`--window --no-cursor --width 138`), after stripping `\x1b\[[>=<?][0-9;]*[mu]`, holding intro/outro cards 7 s, and embedding a JetBrains Mono subset as base64 WOFF2 (font-family ZeMono). No producer in the repo does this.
- A 58 s cast became a 96 KB SVG. A 7-minute SVG would be several MB.

## Work Plan (ordered)

1. Probes, before any tape:
   - live commit enables RPKI, IRR, BFD, OSPF, IPsec, VRRP, traffic usage on a running daemon (each a defect to fix if not);
   - esp4 and xfrm_interface load inside the privileged demo container on the render host.
2. Lab: one root-netns Ze managing veths to lab netns; FRR running bgpd+bfdd+ospfd together (`startFRRPair` hardcodes one daemon and `/run/frr`; generalize it); RTR cache, IRR server, HTTP source, keepalived peer, hidden second Ze for IKE.
3. Scenarios, each with a runner case, a validator in `demoValidators`, a manifest entry (`platform linux`, `privileged true` where the lab needs it), cards, tape and transcript:
   - one topic scenario per chapter 2 to 9 (RPKI, IRR, BFD, OSPF, IPsec, VRRP tracking, eBPF traffic, commit-confirmed), each starting from the base lab and typing its topic live. Seven topics already have a single-feature demo (`rpki`, `irr-filter`, `bfd-failover`, `ospf-adjacency`, `vrrp-failover`, `traffic-anomaly`, `commit-confirmed`) that loads a prepared `demos/terminal/<id>/ze.conf` instead of typing it, and `vrrp-failover` shows failover, not tracking. Whether each topic recording replaces its existing demo or sits beside it is settled at design (`ai/rules/no-layering.md` favours replacing). IPsec has no demo today;
   - the `showcase` super scenario, all chapters in one session, driven from the same per-topic declarations (R-5).
4. Hero: derive the front-page demo, caption and anchor from one declaration (`homeHeroDemo` plus manifest), then point it at the super.
5. README carrier probe, once the super is recorded: render the super as an animated SVG and measure its size; test whether GitHub renders it inline in the README, and likewise a video GitHub plays in markdown; check each against GitHub's size limits. Record the measurements in this spec.
6. README: embed the super inline with the carrier the probe proved; when neither renders within limits, show a thumbnail linking to the site player. Its producer is a native `./le` action (`internal/le/site/...`), not a hand recipe. Reword README line 10.
7. Gallery: `docs/guide/terminal-demonstrations.md` (the manifest's `gallery-page`) lists each topic recording by name, and the super.
8. Feature pages: embed each topic recording in its feature doc page through the manifest's per-demo `page` and `anchor` (today `guide/rpki.md`, `guide/irr-filtering.md`, `guide/bfd.md`, `guide/ospf.md`, `guide/vrrp.md`, `guide/traffic-usage.md`, `guide/config-editor.md`; the new IPsec recording goes on `guide/ipsec.md`).

## Risks

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | A feature does not enable on a live commit | Fix in Ze (owner decision 5) |
| R-2 | Render host kernel lacks esp4/xfrm_interface | Load in container; else record where available; appliance spec covers the product side |
| R-3 | Long session flakes (plugin stage stall under load seen 2026-10-08) | Per-chapter waits on output; journal and fix the stall if it recurs |
| R-4 | A 7 minute animated SVG is likely too large for GitHub to render (a 58 s cast gave 96 KB; 7 minutes is several MB) | Not an owner question: a measured outcome. Work Plan step 5 measures the SVG and tests inline rendering (SVG, then a video GitHub plays in markdown); if neither renders within GitHub's limits, the fallback is a README thumbnail linking to the site player |
| R-5 | Each topic is recorded twice, as its own cast and as a super chapter, and the two drift (config lines, commands, expected output) | Declare each topic's typed config and demonstration once and drive both recordings from it. No such declaration exists today: a manifest `Demo` (`internal/le/site/terminaldemo/types.go`) carries only id, title, description, page, anchor, platform, kind, engine, source tape, validate id, duration and flags; the config is a per-demo `demos/terminal/<id>/ze.conf` loaded at prepare, the steps are that demo's `demo.tape`, and each validator in `demoValidators` is a Go function keyed by demo id that starts its own runner. The tape format has an include (every tape opens with `Source common.tape`), so a per-topic tape fragment sourced by both the topic tape and the showcase tape is one candidate; each topic's validator checks must likewise be one function that both validators call |

## Work Not Done (owned elsewhere)

| Item | Spec |
|------|------|
| Appliance kernel carries esp4, xfrm_interface, wireguard | `plan/immediate/spec-appliance-kernel-vpn-modules.md` |
| WireGuard runtime test, interop and demo | `plan/spec-wireguard-runtime-proof.md` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [Stage 1: for example "Wire parsing in internal/component/bgp/message/"]
2. [Stage 2: ...]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | [JSON format, command syntax] | No |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | [what this design assumes] | [where the assumption comes from] | [impact on design] | [test/grep/user confirmation] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what goes wrong] | [how we notice it] | [what we do about it] |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [live sessions dropped / routes mis-encoded / config rejected / nothing user-visible] |
| How is it reverted? | [single commit revert / needs config migration / not revertible once peers see it] |
| Who else touches this path? | [other plugins, components, or specs working the same files] |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [what triggers the behavior] | [observable outcome] |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [for example "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-feature-peer` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP/strongSwan] | [protocol behavior validated] | |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/...` - [feature changes]

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves. Dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/cli.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | | Any new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink, binary, or certificate: owning-package check + `internal/core/diagnostic/codes.go` + unit and functional test (`ai/rules/repo-maintenance.md`) |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | | The 12-section checklist in `ai/patterns/bgp-family.md` -- read it and record the answers there, not inline |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | | `features/<id>.md` (renders `docs/features.md`) |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfcNNNN.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED, do not answer from memory: `./le spec citation anchors spec plan/<this-spec>.md` lists them. A doc DECLARED by a changed file's `// Design:` header BLOCKS until named here; a doc that only `<!-- source: -->` mentions it is advisory. Naming it as unaffected, with the reason, satisfies the check |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: [wiring test names from the Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: the entry point exists and is reachable. The wiring test fails because the feature is a stub
2. **Phase: [name]** -- [what to implement]
   - Tests: [test names from the TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | [feature-specific, for example "merge order correct", "error messages name the offending value"] |
| Naming | [feature-specific, for example "JSON keys kebab-case", "YANG leaf matches env var leaf"] |
| Data flow | [feature-specific, for example "resolution in X only, reactor unaware of Y"] |
| Rule: [relevant rule] | [what to check] |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

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
<!-- LIVE: write immediately when you learn something. Route each lesson to its
     governing surface under ai/rules/planning.md. A problem-class journal row
     is appropriate only when no surface governs the lesson yet; closure alone
     requires no lesson artifact. -->

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- [What was deliberately not done and why]

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
