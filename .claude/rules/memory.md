# Ze Project Memory

Rationale: `.claude/rationale/memory.md`

## Maintenance (BLOCKING at session end)

Before committing:
1. **Dedup**: remove entries already in `.claude/rules/*.md`
2. **Stale**: remove entries referencing deleted files/functions
3. **Merge**: combine related bullets, heading + 1-3 lines max
4. **Overflow**: entries >5 lines → `.claude/rationale/memory.md`
5. **Cap**: 200 lines hard limit (system truncates after)

## When to Consult Rationale

Read `.claude/rationale/<name>.md` when a rule needs context, examples, or the compressed rule doesn't fully cover the situation.

## Project Knowledge (not in other rules)

### Family Registration
Families registered dynamically by plugins via `PluginRegistry.Register()` — not a static list.
Validate format (contains "/", non-empty parts) — never enumerate all families.

### Config Pipeline
File → Tree → `ResolveBGPTree()` → `map[string]any` → `reactor.PeersFromTree()`.
Key files: `internal/component/bgp/config/resolve.go`, `internal/component/bgp/config/peers.go`, `internal/component/bgp/reactor/config.go`.

### Linter Hook
`auto_linter.sh` runs goimports on Edit/Write. Add import + usage in same edit to avoid cascading removals.

### Architecture Restructuring (arch-0)
Umbrella spec: `plan/learned/425-arch-0-system-boundaries.md`. Six phases.
Key decisions agreed with user:
- **4 components:** Engine (supervisor), ConfigProvider, PluginManager, Subsystem
  (Bus removed 2026-04-07: stream system in PluginManager covers all concrete pub/sub
  needs with DirectBridge zero-copy. See `plan/learned/537-config-tx-protocol.md` rationale.)
- **Subsystem ≠ Plugin:** BGP daemon is a subsystem (owns TCP/FSM), bgp-rib/rs/gr are plugins
- **Stream system is the pub/sub backbone:** validated `(namespace, event-type)` events
  with DirectBridge zero-copy hot path. Located in `internal/component/plugin/server/dispatch.go`
  (`subscribeEvents`/`emitEvent`/`deliverEvent`). For plugin-to-plugin opaque messaging without
  registration, the future answer is an `open` namespace exemption -- not a separate bus.
  See `plan/deferrals.md` 2026-04-07 entry for `spec-stream-open-namespace`.
- **Interfaces in `pkg/ze/`** — public so external plugins can depend on them
- **ConfigManager is central authority** — editor (`ze config edit`), web UI, subsystems, plugins all use same interface
- **Performance matters** — user explicitly asked for performance-conscious design
- **`make ze-verify`** before closing spec/committing
- **Cross-check child specs against umbrella** after each phase

### YANG Choice/Case Validation Gaps
ze's YANG-to-Schema walker (`internal/component/config/yang_schema.go`) handles
ChoiceEntry/CaseEntry via `flattenChildren` -- but `mandatory true` on choice
statements is NOT enforced, and inner-choice mutual exclusivity (e.g. `local
{ ip ... }` vs `local { interface ... }`) is also NOT enforced. The flattener
only makes the data nodes visible to the parser; it does not implement
choice semantics. Plugin authors who use `choice` MUST add Go-side validation
in their config parser (e.g. `parseTunnelEntry` in `internal/component/iface/config.go` checks
both-locals and missing-local). Also: `ze config validate` does not invoke
plugin `OnConfigVerify` callbacks, so any Go-side validation only fires at
daemon reload time.

### Constants for Command/Status Names
String literals used as command names or status values must be constants -- compiler catches typos that `case "sett":` would silently miss. Editor commands live in `internal/component/cli/model.go`. Plugin status uses `plugin.StatusDone`/`plugin.StatusError`.

### Proximity Principle & Handler Location
`bgp/handler/` is a middleman — command handlers belong in `bgp/plugins/` (self-contained).
ALL RPCs need YANG — no "command module" category. Missing YANG is a bug, not a design choice.
"Delete the folder" is a mechanical check for proximity. See `rules/plugin-design.md`.

### LSP Tool for Go Navigation
Load the LSP tool (`select:LSP`) at session start for Go code intelligence — goToDefinition,
findReferences, goToImplementation, incomingCalls, outgoingCalls. More precise than grep for
tracing call chains and finding interface implementations.

### Project Inventory Tool
`make ze-inventory` (or `--json`) runs `scripts/inventory/inventory.go`, which imports `plugin/all` to trigger
real registrations, then queries `registry.All()`, `yang.Modules()`, counts RPCs from .yang files,
.ci tests from `test/`, and Go stats from `internal/`+`pkg/`+`cmd/`. Always accurate, no regex.
Use when checking plugin counts, RPC totals, family coverage, or codebase size.

### SDK Type Aliases Are Intentional
`pkg/plugin/sdk/sdk_types.go` re-exports `rpc.*` types as `sdk.*` aliases. This is deliberate —
external plugin authors import only `sdk`, never `rpc`. Decouples public API from internal structure.
Do NOT flag these as "identity wrappers adding no value."

## Mistake Log

### Feature Not Wired (RECURRING — multiple specs, ZERO TOLERANCE)
- Write logic + unit tests, claim "done", but feature is NOT reachable from reactor/CLI/config.
- User cannot use the feature. Tests pass in isolation but nothing calls the code.
- Has happened on: SSH transport, and other specs. User is furious. This is the #1 failure mode.
- Root cause: treat unit tests as proof of completion. Skip wiring into reactor, skip functional tests.
- **BEFORE saying "done":** answer "Can a user reach this through config/CLI/API?" If no: say "not wired yet", never "done".
- **REQUIRED evidence:** name the user entry point + show .ci test or live demonstration. No evidence = not done.
- **Rule:** `rules/integration-completeness.md`. A unit test is not a wiring test. Library code is dead code until wired.

### Wrong Production Path (rib-04)
- Wrote spec pointing at `subsystem.go` stage-1 handler. Production path is `startup.go` (in `plugin/server/`).
- Root cause: found *a* handler, assumed it was *the* handler. Never traced the actual call chain.
- **Rule:** grep for ALL implementations of a protocol step, identify which one the consumer calls.

### Count-Only Test Assertions (addpath-rib)
- Test asserted `Len()==2` on map-backed store. Wrong parsing produced entries that deduped to same count.
- **Rule:** When testing wire parsing into map storage, assert on content (keys/values via Lookup) not just count.

### Wrapper Struct Pattern (alloc-4, three attempts)
- Attempt 1: eager `StructuredEvent` pre-computed FilterResult (N→1 when answer is N→0)
- Attempt 2: `UpdateHandle` wrapped raw data with lazy methods + cached fields (identity wrapper)
- Root cause: defaulted to "struct with accessor methods" instead of ze pattern: pass raw bytes, use existing iterators
- **Rule:** before creating any new type for data access, ask "can the consumer use existing wire types directly?"

### Tests-Pass-Equals-Done (prefix-limit, RECURRING)
- Said "all pass" after unit/functional tests, implying work is complete. Waited for user to ask "continue."
- Docs not written, spec not updated, learned summary not written, audit not filled. Tests are step 10 of 12.
- Root cause: treating test results as a natural stopping point with completion language.
- **Rule:** `rules/quality.md`. After tests pass, continue to the next checklist item immediately. Only stop when blocked or when every step is done.

### Mechanism-Not-Behavior Test (prefix-limit)
- Wrote `TestPrefixExceedWarnOnly` asserting `notif == nil` (no NOTIFICATION). Test passed. Claimed AC-4/AC-27 done.
- AC-4 says "further prefixes rejected." AC-27 says "NLRIs not installed in RIB." Test checked mechanism (no teardown), not behavior (routes blocked).
- A no-op implementation would also return `notif == nil`. The test was invalid from the start.
- Root cause: wrote test for the CODE PATH (teardown=false returns nil) instead of the AC TEXT (routes not delivered).
- **Rule:** `rules/tdd.md` AC-Linked Tests. Quote the AC. Assert the behavior. If a no-op passes the test, the test is wrong.
- **BEFORE marking AC done:** Read the AC text aloud. Read the test assertion. Does the assertion verify THAT TEXT? Not a proxy, not an absence, the actual behavior.

### Dismissing Test Failures as "Pre-existing" (RECURRING -- NOW RESOLVED)
- Original problem: said "pre-existing, skipping" and proposed committing with failing tests.
- Old bad behavior: report failures, block current work, never actually fix them. Created an endless loop.
- **Current rule:** Fix pre-existing failures in the same session, after primary task. Separate commit script.
  If fix needs >10 min, log to `.claude/known-failures.md` for next session. Never block, never skip.
- See `rules/anti-rationalization.md` Test Failures section.

### Plugin Placement Anchor Bias (jsonrpc gateway)
- Placed cross-cutting JSON-RPC gateway under `bgp/plugins/` by pattern-matching recently-read files.
- Root cause: anchored on "where I just read code" instead of reasoning from architecture.
- **Rule:** Before placing any new plugin, apply the "delete the folder" test: if the parent were deleted, should this plugin disappear? Domain plugins → `bgp/plugins/`. Cross-cutting services → `internal/component/<name>/`. Core infra → `internal/core/`.

### Documentation Written From Assumption (RECURRING)
- Wrote docs describing syntax, field names, and behavior from memory instead of reading actual code.
- User found many errors: old syntax, wrong data, stale information.
- Root cause: described what I *thought* the code does, not what it *actually* does. No verification step.
- **Rule:** `rules/documentation.md` Source Anchors section. Read the source file BEFORE writing any factual claim. Add `<!-- source: path -- symbol -->` HTML comments. Never describe code from memory.

### Spec Deleted Without Committing (lg-overhaul, ZERO TOLERANCE)
- Spec `spec-lg-overhaul` (see `plan/learned/498-lg-overhaul.md`) was created as untracked file, never committed, then `git rm -f` deleted it. Content lost forever. Audit tables, verification evidence, design decisions -- all gone.
- Root cause: treated spec deletion as a single step instead of two commits. Commit A should include the completed spec; Commit B should delete it and add the learned summary.
- **Rule:** `rules/spec-preservation.md`. TWO commits: (A) code + completed spec, (B) `git rm` spec + add learned summary. Never delete a spec that has not been committed.

### Reinventing What Exists in the Repo (lg-overhaul)
- Wrote a 40-line custom HTMX JS shim instead of using real htmx.min.js (v2.0.4) already vendored at `third_party/web/htmx/` and synced to the web UI via `scripts/vendor/sync_web.go`.
- ASN decorator framework was production-ready in `internal/component/web/decorator_asn.go` with Team Cymru DNS, but was never wired into the LG. The learned summary even said "future decorator wiring requires populating GraphNode.Name" while claiming the work was done.
- Root cause: did not read the existing codebase before writing new code. The web UI already solved both problems.
- **Rule:** `rules/before-writing-code.md` step 1: "Search for existing implementations -- extend if found." Before writing ANY new infrastructure, grep the repo for existing solutions. If `third_party/` or another component already has it, use it.

### Spec Claimed Complete With Features Unimplemented (lg-0 through lg-4)
- Five LG specs created in "design" status, deleted next day as "completed." Learned summary written as if all features worked. But: no real HTMX (custom shim), no ASN names (decorator never wired), no family selector, broken SSE (shim had no SSE support), SVG invisible in dark mode (hardcoded colors).
- The learned summary itself documented the gaps ("HTMX shim is minimal", "future decorator wiring") while the specs were deleted as done.
- Root cause: writing the learned summary before implementing the features. The summary became a description of intent, not of reality.
- **Rule:** `rules/implementation-audit.md`. The audit MUST verify each AC against running code, not against the spec's description. A learned summary that says "future X" is proof the spec is NOT done.

### Stale Deferrals Waste Sessions (redistribution-filter-phase2)
- Phase-2 spec listed 5 open deferrals. On investigation, 2 (AC-13 undeclared attribute validation, AC-15 raw mode) were already fully implemented in the original spec. Deferrals were never closed.
- Root cause: deferrals written from spec intent ("requires X") without grepping for actual implementation. Nobody re-verified before creating the phase-2 spec.
- **Rule:** `rules/deferral-tracking.md` "Verify Before Deferring." Before starting work on ANY open deferral, grep for the feature in code. If it exists, mark the deferral done and move on. Do not assume a deferral is valid just because it says "open."

### Worktree File Copy Overwrites Main Repo (ZERO TOLERANCE)
- Worktree agent copied files directly from `.claude/worktrees/agent-*/` into the main repo.
- Overwrote uncommitted changes made by other concurrent Claude sessions.
- Root cause: worktree code was not committed, so the agent tried to preserve it by copying files.
- **Rule:** NEVER copy files from a worktree into the main repo. Worktree agents must commit their work. Use `git merge` or `git cherry-pick` to bring changes into main. Hook `block-worktree-copy.sh` (exit 2) enforces this.

### Same-Day Fix After Feature (cmd-4, RECURRING)
- Feature `cmd-4` (bgp-filter-prefix, `6af9820a`) committed at 12:31 with a closed spec and learned summary. By 13:44 the same day, fix `1fc98747` had to land THREE BLOCKERs and SIX ISSUEs caught in post-ship `/ze-review`: filter never fired (rpki crashed before dispatch), filter input had no NLRI (encoder incomplete), and the .ci tests were silent false-positives (observer-exit antipattern).
- Same shape: structured events (`089dc7a5` + `f2cf4b5f`, 16-17 days after `26f8da00`), BGP-as-plugin Phase 2 (`938df51d` + `d029a94d`, 6 days after `440b160a`), bufio races (`d5843235` + `8dffd422`, 47 days after `4ad73c47`).
- Root cause: marking spec done after unit tests pass and a single happy-path .ci run. Skipping the "deliberately break it and watch the test fail" step. Skipping the "grep every consumer of the renamed string" step. Skipping the "race -count=20 on touched concurrency code" step.
- **Rule:** before marking ANY spec done, the adversarial review (`rules/quality.md`) must be run for real, not as a checklist tick. Specifically:
  - Ran `make ze-race-reactor` if any reactor concurrency code changed (`rules/testing.md`).
  - Grepped every consumer of any renamed plugin/subsystem/log/dispatch name (`rules/plugin-design.md` "Renaming a Registered Name").
  - Grepped every sibling call site of any function that got a new guard/fallback (`rules/before-writing-code.md` "Sibling Call-Site Audit").
  - For .ci tests with Python observers: deliberately broke the production code path to confirm the test FAILS. A test that passes after the production logic is broken is invalid (`rules/testing.md` "Observer-Exit Antipattern").
  - The adversarial review questions in `rules/quality.md` were each answered with a specific finding or "checked, none."
- **Posture:** treat a same-day or next-day blocker fix on your own feature as a process incident, not normal churn. The fix is fine; the gap is that the original commit shipped without those checks. Note in the fix commit message which check would have caught the bug, and update the relevant rule if a NEW check is needed.

### Substring Collision in Bulk YANG Edit (spec-iface-tunnel)
- Replaced `gre-local-address` -> `local-address` with `replace_all` then ran the same on `gretap-`, `ip6gre-`, `ip6gretap-`. The `gre-local-address -> local-address` substitution mangled `ip6gre-local-address` into `ip6local-address` because `gre-local-address` is a substring of the longer name. Took ~10 minutes to unwind.
- Root cause: the `Edit` tool does literal substring matching, no word boundaries. Bulk-stripping prefixes from a family of related names is unsafe when the prefixes nest.
- **Rule:** when bulk-stripping a prefix from leaf names, do the *longest* prefix first (e.g. `ip6gre-` before `gre-`), or include surrounding non-name context in the `old_string` so substring overlap cannot fire. After every batch, grep the file for the suffix you stripped to verify no mangled names appeared.

### Vendored Library != Upstream (spec-iface-tunnel)
- Research subagent reported `vishvananda/netlink` exposes `IgnoreDf` on `Gretap` and `EncapLimit` on `Gretun`. The vendored v1.3.1 in `vendor/` does not. Two YANG leaves had to be dropped after I'd already wired them through the parser, struct, and YANG schema. Recorded as deferrals.
- Root cause: the research agent fetched upstream master from github; the vendor copy is older. I built on the assumption that "upstream supports it" implies "ze can use it."
- **Rule:** when designing on top of a vendored third-party library, always verify field names and method signatures against `vendor/<lib>/`, not against upstream documentation. Spec sections that name external types should cite the file path under `vendor/` so the verification is mechanical.

### Naive Reconciliation Recreates Live State on Every Reload (spec-iface-tunnel)
- First implementation of `applyTunnels` did unconditional `DeleteInterface` then `CreateTunnel` on every reload. Tunnels carrying live BGP traffic would briefly drop on every SIGHUP, even when the SIGHUP only changed an unrelated knob. Caught only by `/ze-review`, not by my own adversarial review or the .ci tests.
- Root cause: I optimised for correctness on the modify case (key change must take effect) and ignored the unchanged case (most common). The .ci modify-key test passed without verifying the daemon had any traffic to drop, so the operational impact was invisible.
- **Rule:** any reconciliation step that touches *running* state (netdevs, sessions, sockets, listeners) MUST diff against the previously applied state and only act on the delta. Pass the previous config explicitly. The fix here was to add `previous *ifaceConfig` to `applyConfig` and an `indexTunnelSpecs` helper that compares specs by value. Add this question to the adversarial review checklist: "Does this reload disturb anything that wasn't actually changed?"

### Mirror Existing Config Shape Before Inventing One (spec-iface-tunnel)
- Built tunnel YANG with flat `local-address`, `local-interface`, `remote-address` leaves. User pointed at `bgp peer connection { local { ip ... } remote { ip ... } }` and asked why I had not used the same shape. Restructured the YANG, parser, tests, and `.ci` files mid-stream. ~15 minutes of mechanical edits, all avoidable.
- Root cause: I copied the discriminator shape from VyOS/Junos research and did not grep ze for an existing analog before defining the leaf names.
- **Rule:** before defining new YANG endpoint shapes (local/remote, source/destination, listener, peer), grep the existing `*-conf.yang` files for the closest analog and copy its grammar verbatim. Cross-component consistency in config syntax is more valuable than matching the upstream protocol's terminology.

### Scratch .go Files in tmp/ Break go test ./... (spec-iface-tunnel)
- `make ze-verify` failed with build errors in `tmp/netlink-research/link.go` and `tmp/vendor-pull/origin-frame.go` -- snippets of vendored third-party code dropped there by research agents in earlier sessions. `go test ./...` walks the whole module root including `tmp/`, so any `.go` file there must compile.
- Root cause: research subagents that download library source for inspection put it in `tmp/<topic>/` with the original `.go` extension. The Go toolchain treats those as packages.
- **Rule:** research agents that fetch third-party Go source for inspection must save it with a `.txt` extension or under a build-tagged directory. The `tmp/` tree is for scratch artefacts, not buildable Go code. If you find `.go` files in `tmp/` that you did not put there, they are safe to delete (verified by `git status` -- they are untracked).
