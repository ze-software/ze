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
Key files: `internal/component/bgp/config/resolve.go`, `internal/component/bgp/config/peers.go`, `reactor/config.go`.

### Linter Hook
`auto_linter.sh` runs goimports on Edit/Write. Add import + usage in same edit to avoid cascading removals.

### Architecture Restructuring (arch-0)
Umbrella spec: `plan/spec-arch-0-system-boundaries.md`. Six phases.
Key decisions agreed with user:
- **5 components:** Engine (supervisor), Bus (content-agnostic pub/sub), ConfigProvider, PluginManager, Subsystem
- **Subsystem ≠ Plugin:** BGP daemon is a subsystem (owns TCP/FSM), bgp-rib/rs/gr are plugins
- **Bus is content-agnostic:** payload always `[]byte`, bus never type-asserts. Like RabbitMQ/Kafka.
- **Topics:** hierarchical with `/` separator (`bgp/update`, `bgp/events/peer-up`). Prefix-based subscription matching.
- **Interfaces in `pkg/ze/`** — public so external plugins can depend on them
- **ConfigManager is central authority** — editor (`ze config edit`), web UI, subsystems, plugins all use same interface
- **Performance matters** — user explicitly asked for performance-conscious design
- **`make ze-verify`** before closing spec/committing
- **Cross-check child specs against umbrella** after each phase

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
`make ze-inventory` (or `--json`) runs `scripts/inventory.go`, which imports `plugin/all` to trigger
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
- Wrote spec pointing at `subsystem.go` stage-1 handler. Production path is `server_startup.go`.
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

### Dismissing Test Failures as "Pre-existing" (RECURRING, ZERO TOLERANCE)
- Said "pre-existing slogutil failures unrelated" and proposed committing with failing tests.
- Has happened MANY times across multiple sessions. User explicitly said it causes distress.
- Root cause: treating "I didn't cause it" as permission to ignore it. The rules are explicit: investigate and fix.
- **Rule:** NEVER say "pre-existing" or "unrelated" to justify not fixing a test failure. Investigate every failure. Fix it. Then commit.

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
- Spec `plan/spec-lg-overhaul.md` was created as untracked file, never committed, then `git rm -f` deleted it. Content lost forever. Audit tables, verification evidence, design decisions -- all gone.
- Root cause: treated spec deletion as a single step instead of two commits. Commit A should include the completed spec; Commit B should delete it and add the learned summary.
- **Rule:** `rules/spec-preservation.md`. TWO commits: (A) code + completed spec, (B) `git rm` spec + add learned summary. Never delete a spec that has not been committed.

### Reinventing What Exists in the Repo (lg-overhaul)
- Wrote a 40-line custom HTMX JS shim instead of using real htmx.min.js (v2.0.4) already vendored at `third_party/web/htmx/` and synced to the web UI via `scripts/sync-vendor-web.sh`.
- ASN decorator framework was production-ready in `internal/component/web/decorator_asn.go` with Team Cymru DNS, but was never wired into the LG. The learned summary even said "future decorator wiring requires populating GraphNode.Name" while claiming the work was done.
- Root cause: did not read the existing codebase before writing new code. The web UI already solved both problems.
- **Rule:** `rules/before-writing-code.md` step 1: "Search for existing implementations -- extend if found." Before writing ANY new infrastructure, grep the repo for existing solutions. If `third_party/` or another component already has it, use it.

### Spec Claimed Complete With Features Unimplemented (lg-0 through lg-4)
- Five LG specs created in "design" status, deleted next day as "completed." Learned summary written as if all features worked. But: no real HTMX (custom shim), no ASN names (decorator never wired), no family selector, broken SSE (shim had no SSE support), SVG invisible in dark mode (hardcoded colors).
- The learned summary itself documented the gaps ("HTMX shim is minimal", "future decorator wiring") while the specs were deleted as done.
- Root cause: writing the learned summary before implementing the features. The summary became a description of intent, not of reality.
- **Rule:** `rules/implementation-audit.md`. The audit MUST verify each AC against running code, not against the spec's description. A learned summary that says "future X" is proof the spec is NOT done.

### Worktree File Copy Overwrites Main Repo (ZERO TOLERANCE)
- Worktree agent copied files directly from `.claude/worktrees/agent-*/` into the main repo.
- Overwrote uncommitted changes made by other concurrent Claude sessions.
- Root cause: worktree code was not committed, so the agent tried to preserve it by copying files.
- **Rule:** NEVER copy files from a worktree into the main repo. Worktree agents must commit their work. Use `git merge` or `git cherry-pick` to bring changes into main. Hook `block-worktree-copy.sh` (exit 2) enforces this.
