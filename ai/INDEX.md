# Ze Documentation Index

## Understand Existing Code (not change it)

Cold-start orientation. Read these to answer "what is here / where does it live"
before grepping. All three are generated from the tree (`make ze-discovery-index`)
and gated fresh, so they never lie about the current code.

| Question | Read |
|----------|------|
| What does package X do? (one line each, all ~590 packages) | `ai/PACKAGE-MAP.md` |
| Which `.go` files implement design doc Y? | `ai/DOCS-TO-CODE.md` (inverse of the per-file `// Design:` headers) |
| Which docs describe code path Z? | `ai/CODE-TO-DOCS.md` (inverse of doc `<!-- source: -->` anchors) |
| Every past decision / trap by number | `ai/LEARNED-FULL-INDEX.md` (complete); `ai/LEARNED-INDEX.md` (curated by topic) |
| Why is the code shaped this way? | `plan/learned/DESIGN-HISTORY.md` |
| Which rule covers a topic? | `ai/rules/INDEX.md` |
| How does data flow through a subsystem? | `docs/architecture/core-design.md` (START HERE), then the subsystem doc below |
| Fast subsystem orientation (entry→exit, with `file:line`) | `ai/digests/<subsystem>.md` — living flow digests; index + list in `ai/digests/README.md`. Anchors gated by `make ze-digest-check` |

## I Want To...

| Task | Read first | Then |
|------|-----------|------|
| Understand the modular core | `ai/patterns/registration.md` | `docs/architecture/core-design.md` |
| Keep a plugin self-contained (removal test) | `ai/rules/plugin-self-containment.md` | Remove the plugin and ALL its features vanish; other plugins and core keep working |
| Call another package's function directly from a plugin (not through RPC) | `ai/rules/plugin-process-boundary.md` | Check `p.IsInternal()`; guard with refuse-or-warn depending on how much value survives running external. Gated by `make ze-plugin-boundary-check` |
| Choose internal/core vs internal/component vs internal/plugins for a new package | `ai/rules/module-tiers.md` | Tier = dependency direction; engine placement gated by `make ze-tier-check` (`scripts/dev/dep_audit.py --check`) |
| Test linux-only code (QEMU) | `ai/rules/qemu-testing.md` | `ai/rules/testing.md` (Linux-Only Tests section) |
| Fix a failing test, gate, demo, or user-visible problem | `ai/rules/no-workarounds-for-missing-behavior.md` | Implement the missing behavior at the source, never route around it |
| Modify wire encoding | `ai/rules/buffer-first.md` | `docs/architecture/buffer-architecture.md` |
| Add route processing | `ai/rules/architecture-summary.md` | `docs/architecture/core-design.md` |
| Detect and auto-mitigate a DDoS flood | `docs/guide/ddos-mitigation.md` | `ddos-detect` characterizes the attack (family + vector) from `traffic-usage`/`flow-export`; `ddos-local`/`ddos-flowspec` install surgical rules; `show flow recent` inspects the flow ring |
| Detect behavioral security anomalies (exfil, C2, scanning) | learned `1046`/`1048`/`1049` | Neutral facts in `internal/component/trafficfeature` (fan-out, out/in ratio, entropy, beaconing) on `internal/core/stats`; `anomaly/detect` (report-only) scores per-entity deviation + cohort rarity into incidents (`show anomaly`); `anomaly/shape` responds shadow-first (per-source rate-limit, arm/auto-revert/kill-switch, `show anomaly-shape`). Separate security domain from `ddos`. |
| Provide or extend first-hop gateway redundancy (VRRP) | `docs/guide/vrrp.md` | RFC 9568/3768 in `internal/plugins/vrrp/` (self-contained plugin) with the per-group virtual-MAC macvlan in `internal/component/iface/macvlan.go`; extend within the self-contained `internal/plugins/vrrp/` plugin |
| Implement an RFC | `ai/rules/rfc-compliance.md` | `docs/contributing/rfc-implementation-guide.md` |
| Prove an RFC MUST is enforced (tag a test, coverage gate) | `ai/skills/ze-rfc.md` | Tag the test `RFC requirement: <id> <polarity>` (both polarities); `make ze-rfc-check` gates coverage; ledger `ai/RFC-REQUIREMENTS.md` via `make ze-rfc-index`; audit with `/ze-rfc-audit` |
| Write a spec | `ai/rules/planning.md` | `plan/TEMPLATE.md` (design-time only; placeholders are legal at `skeleton`, blocked from `design` on) |
| Close a spec (audit, goal validation, review gate, pre-commit evidence) | `ai/rules/implementation-audit.md` | `plan/TEMPLATE-CLOSURE.md`, appended by `/ze-close` at step 1; every Pre-Commit sub-table needs an evidence row |
| Record design risks and assumptions | `ai/rules/planning.md` (Risks & Assumptions) | A-N/R-N tables in `plan/TEMPLATE.md`; validate during /ze-implement audit |
| Add a feature, tool, self-check, verification gate, or test infrastructure | `ai/rules/discovery-updates.md` | Update docs, rules, indexes, and verification paths in the same change |
| Compare Ze with other products | `ai/rules/comparison-honesty.md` | Cite every claim, link code or official feature docs, label uncertainty, and add hide-column controls for wide product matrices |
| Add or change an agent behavior rule | `ai/rules/canonical-sources.md` | Put shared Ze rules in `ai/rules/` and startup pointers in `ai/INSTRUCTIONS.md` |
| Reorganize YANG tree | `scripts/dev/yang_move.py --help` | Preview diff, then `--apply` |
| Move a package between tiers | `scripts/dev/migrate_module.py --help` | Dry-run plan, then `--apply` |
| Rename the module path (host or owner change) | `scripts/dev/rename_module_path.py --help` | Dry-run plan, then `--apply`; regenerate protobuf with `make ze-proto-gen` |
| See which rule covers a topic | `ai/rules/INDEX.md` | One-line overview of every rule; open the listed file before acting |
| Understand Ze vs standard Go | `ai/rules/ze-divergences.md` | Buffer-first, registration, YANG, etc. |
| Know which hooks will check my code | `ai/rules/hook-mapping.md` | Pre-flight compliance checklist |
| Edit the website or presentations | `docs/contributing/gh-pages.md` then `../gh-pages/AI.md` | Worktree layout, tooling, adding a talk |
| Write and publish the weekly update | `ai/skills/ze-weekly-update.md` | Draft in Zeledon voice, update `../gh-pages`, post the approved message to `ze-news`, and verify site/feed/homepage output |

## By Task Type

### Adding a Feature

| Feature kind | Read first | Then read | Cross-cutting |
|---|---|---|---|
| CLI command | `ai/patterns/cli-command.md` | `ai/rules/cli-grammar.md`, `ai/rules/pipe-completeness.md`, `docs/architecture/cli/command-namespacing.md` (rooting + filters-not-flags) | `ai/rules/derive-not-hardcode.md` if it lists things |
| Web page/endpoint | `ai/patterns/web-endpoint.md` | `docs/architecture/web-interface.md`, `docs/architecture/web-components.md` | SSE: `docs/architecture/web-components.md` SSE section |
| Plugin | `ai/patterns/plugin.md` | `ai/rules/plugin-design.md`, `ai/rules/goroutine-lifecycle.md` | `ai/rules/naming.md` for registered names; `ai/rules/plugin-process-boundary.md` if it calls another package's function directly; `ai/rules/derive-not-hardcode.md` because registration metadata feeds the generated website plugin catalog |
| Config option | `ai/patterns/config-option.md` | `ai/rules/config-design.md` (listener pattern if network endpoint) | `ai/rules/config-surface.md` (YANG vs env var), `ai/rules/config-naming.md` (naming), `ai/rules/go-standards.md` env var section |
| NLRI family | **`ai/patterns/bgp-family.md`** (BLOCKING) | `docs/architecture/wire/nlri.md`, `ai/rules/buffer-first.md` | `ai/rules/plugin-design.md` family registration |
| Capability | **`ai/patterns/bgp-family.md`** (BLOCKING) | `docs/architecture/wire/capabilities.md` | |
| Attribute | **`ai/patterns/bgp-family.md`** (BLOCKING) | `docs/architecture/wire/attributes.md` | `docs/architecture/encoding-context.md` |
| Functional test | `ai/patterns/functional-test.md` | `docs/architecture/testing/ci-format.md` | `ai/rules/testing.md` for format selection (.ci vs .et vs Go) |
| Editor test | `ai/rules/testing.md` (Editor Tests section) | `test/editor/` existing examples | |
| Telemetry/metrics | `plan/learned/653-netdata-os-collectors.md` | `plan/learned/736-iface-rate.md` | Registration in loader_create.go |
| Observation feed, traffic observation, multi-subscriber fan-out | `docs/architecture/observation-feed.md` | `plan/learned/1016-observation-feed.md` | `internal/core/observation/` (Feed, Observation); `iface/rate.go` (SubscribeCollectNotify) |
| Debug flags for a plugin | `ai/patterns/debug-registration.md` | `internal/component/bgp/yang/register_debug.go` (example) | One file per plugin: `register_debug.go` in yang/ |
| Diagnostic command | `plan/learned/727-diag-core.md` | `plan/learned/755-ze-doctor.md` | `ai/rules/doctor-checks.md` |
| Agent-facing command/tool | `ai/rules/agent-tooling.md` | `docs/features/ai-first.md`, `docs/guide/mcp/overview.md` | `ai/rules/discovery-updates.md` for indexes and verification |
| Verification/self-check gate | `ai/rules/discovery-updates.md` | `ai/rules/hook-mapping.md`, `docs/contributing/documentation-testing.md` | `mk/inventory.mk` for doc/inventory targets |
| EventBus event | `ai/rules/plugin-design.md` (EventBus Typed Payloads) | `pkg/ze/eventbus.go` | Use `events.Register[T]`, not raw `bus.Subscribe` |
| DirectBridge handler | `ai/rules/plugin-design.md` (DirectBridge section) | `pkg/plugin/rpc/bridge.go`, `plan/learned/DESIGN-HISTORY.md` "Plugin system: architecture" (294, retired) | |
| New component | `docs/architecture/core-design.md` section 1 | `ai/rules/design-principles.md`, `ai/rules/architecture-summary.md` | Proximity principle in `ai/rules/plugin-design.md` |
| New subsystem | `docs/architecture/hub-architecture.md` | `docs/architecture/subsystem-wiring.md` | |
| Test runner or format | `ai/rules/testing.md` | `ai/patterns/functional-test.md`, `docs/architecture/testing/ci-format.md` | `ai/rules/discovery-updates.md` |


### Preparing a Commit

| Task | Read first | Then use |
|---|---|---|
| Generate and run a commit script | `ai/rules/git-safety.md` | Fast path: use `scripts/dev/commit_helper.py create`, then run it yourself (`bash tmp/commit-<SESSION>.sh`); if verification is considered, run `scripts/dev/verify-status.sh check` first and never rerun verify when FRESH |
| Duplicate learned numbers after a merge or rebase | `ai/rules/git-safety.md` (learned-next does not span branches) | `make ze-learned-numbers-check` to detect, `make ze-learned-numbers-fix` to resolve; then `make ze-discovery-index` |

### Modifying Existing Code

| Area | Read first | Key concerns |
|---|---|---|
| Reactor / session | `docs/architecture/core-design.md` sections 1-5 | `ai/rules/goroutine-lifecycle.md`, `make ze-race-reactor` required |
| Wire encoding/decoding | `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` | No `make()`, no `append()`, `WriteTo(buf, off) int`, caller-owned buffers |
| RIB / route storage | `docs/architecture/route-types.md`, `docs/architecture/rib-transition.md` | Pool dedup, lazy iterators |
| Route selection | `docs/architecture/route-selection.md` | `ai/LEARNED-INDEX.md` (RIB/Routing section) |
| Config pipeline | `docs/architecture/config/yang-config-design.md` | File -> Tree -> ResolveBGPTree -> map[string]any |
| Plugin SDK | `ai/rules/plugin-design.md` (SDK Is Generic) | No plugin-specific code in SDK |
| Hub / engine | `docs/architecture/hub-architecture.md` | Protocol-agnostic, Coordinator pattern |
| Forward pool | `docs/architecture/forward-congestion-pool.md` | Two-tier model, per-peer workers |
| YANG schemas | `ai/rules/config-design.md` | Augment vs grouping, listener pattern, `ai/rules/config-naming.md` (leaf naming), `ai/rules/config-surface.md` (YANG vs env var) |
| Registration code | `ai/patterns/registration.md` | `init()` + registry + blank import pattern |
| Show enricher | `ai/patterns/registration.md` (Show Enricher Registry) | `internal/core/show/` -- in-process via `show.MustRegister()`; external via `EnricherDecl` + `ze-plugin-callback:enrich-show`; web via explicit `show.Enrich()` |

### Fixing a Bug

```
1. Read ai/rules/before-writing-code.md (sibling call-site audit)
2. Read ai/rules/anti-rationalization.md (no rationalizing test failures)
3. Grep ALL implementations of the function/protocol step (ai/rules/integration-completeness.md)
4. Check plan/learned/RECURRING-PATTERNS.md for known traps in this area
5. After fixing: ai/rules/testing.md iteration workflow
```

### Writing Documentation

```
1. Read docs/contributing/writing-style.md (rule one: the six banned habits, the limits)
2. Read ai/rules/documentation.md (categories, source anchors)
3. Read ai/rules/discovery-updates.md if the doc adds or changes a feature, tool, check, gate, or test path
4. Read the actual source before any factual claim
5. Add <!-- source: path -- symbol --> anchors
6. Run make ze-ste-review-changed to read your own prose back
7. Run make ze-doc-test after editing docs/
```

### Working with IPsec / IKE

Read in order: `plan/learned/734` (data model), `plan/learned/739` (crypto),
`plan/learned/740` (engine), `plan/learned/742` (child SA), `plan/learned/744` (EAP/NAT-T).

### Working with CPE / Subscriber

Read: `plan/learned/760` (subscriber session model), `plan/learned/725` (DHCP ranges),
`plan/learned/746` (firewall global options).

### Writing Tests (Which Type?)

| What to test | Test format | Directory | Runner |
|---|---|---|---|
| Config parses correctly | `.ci` | `test/parse/` | `ze-test bgp parse` |
| BGP wire encoding | `.ci` | `test/encode/` | `ze-test bgp encode` |
| BGP wire decoding | `.ci` | `test/decode/` | `ze-test bgp decode` |
| Plugin behavior / API | `.ci` | `test/plugin/` | `ze-test bgp plugin` |
| Config reload via SIGHUP | `.ci` | `test/reload/` | `ze-test bgp reload` |
| CLI show/monitor output | `.ci` | `test/ui/` | `ze-test ui` |
| Web HTTP endpoints | `.wb` | `test/web/` | `ze-test web` |
| Editor TUI interactions | `.et` | `test/editor/` | `ze-test editor` |
| Pure logic (no daemon) | `_test.go` | `internal/<pkg>/` | `go test` |
| Linux-only kernel code | `_test.go` | `internal/<pkg>/` | `make ze-qemu-integration-test` |

Key docs: `ai/patterns/functional-test.md` (directories + runner commands),
`docs/functional-tests.md` (verify artifacts and rerun workflow),
`docs/architecture/testing/ci-format.md` (full format reference),
`ai/rules/testing.md` (observer API, iteration workflow).

## When You Don't Know Which Area

Use keyword search in `ai/INDEX.md`. If the keyword isn't there:

```
grep -rn "keyword" docs/architecture/ --include="*.md" -l
grep -rn "keyword" plan/learned/ --include="*.md" -l
grep -rn "keyword" ai/rules/ --include="*.md" -l
```

## Cross-Cutting Rules (Apply Regardless of Area)

These rules are frequently missed because they don't map to a single
artifact type. Check them whenever your work touches the described concern.

| Concern | Rule | When it applies |
|---|---|---|
| Every word you write | `ai/rules/simplified-technical-english.md`, guide: `docs/contributing/writing-style.md` | Rule one. ASD-STE100 Issue 9 for all repository writing: docs, comments, error messages, CLI output, YANG descriptions, specs, commit and PR text. Six banned habits. Gate: `make ze-ste-check`. Report: `make ze-ste-review` |
| How much you write | `ai/rules/detail-budget.md` | Any report, rule, doc, commit body, or learned summary. Per-artifact budgets |
| Listing/enumerating things | `ai/rules/derive-not-hardcode.md` | Help text, usage strings, error messages, any output that enumerates items |
| Goroutine lifecycle | `ai/rules/goroutine-lifecycle.md` | Any `go func()`, any `OnStarted` callback, any worker pattern |
| File size | `ai/rules/file-modularity.md` | Modified file exceeds 1000 lines |
| Pipe operators | `ai/rules/pipe-completeness.md` | Any command producing output |
| Registered names | `ai/rules/plugin-design.md` "Renaming" section | Changing any plugin/subsystem/dispatch/log name |
| Same-process-only calls | `ai/rules/plugin-process-boundary.md` | Any plugin calling another `internal/component/*` package's exported function directly, not through DirectBridge/DispatchCommand |
| Sibling call sites | `ai/rules/before-writing-code.md` "Sibling Call-Site Audit" | Adding a guard/fallback/retry to ANY call site |
| Buffer allocation / memory | `ai/rules/memory-architecture.md`, `ai/rules/buffer-first.md`, `ai/rules/no-sprintf-alloc.md` | Any allocation, pool use, string building, or wire encoding |
| Map keys / dispatch keys | `ai/rules/enum-over-string.md` | Any new `map[string]` or string-based dispatch on a hot path |
| JSON keys | `ai/rules/json-format.md` | Any new JSON output |
| Env vars | `ai/rules/go-standards.md` env section | Any env var access |
| Error handling | `ai/rules/go-standards.md` forbidden section | Any `_` on error return |
| Error / failure message content | `ai/rules/error-messages.md` | Any error, log line, or failure output: name the subject + offending value + corrective action; greppable phrase; fail closed |
| Discoverability | `ai/rules/discovery-updates.md` | Any feature, tool, self-check, verification gate, test infrastructure, or agent workflow |
| Which model runs this phase | `ai/rules/model-selection.md` | Crossing between planning/design, implementation, and review: planning and review run on Opus 5, implementation on Opus 4.8 |
| Two rules point in different directions | `ai/rules/rule-precedence.md` | The ladder: irreversible action > outside-facing correctness > scope integrity > phase boundaries > autonomy |
| How much work is already in flight | `scripts/dev/spec-session.sh wip` | In-progress specs, stalest first, against `ZE_SPEC_WIP_CAP` (default 12); `claim` refuses a new `ready` spec over the cap |
| Who executes this phase (main thread vs subagent) | `ai/rules/spec-delegation.md` | Any spec work: the main thread supervises, each phase runs in a subagent through its `ze-*` skill |

## Dev Tools

| Tool | Location | Purpose |
|------|----------|---------|
| `commit_helper.py` | `scripts/dev/` | Generate commit message files and executable commit scripts that Claude runs itself (`bash tmp/commit-<SESSION>.sh`). Reuses `tmp/commit-session-id`, rejects ignored/generated paths, uses `git commit -F`, and requires a learned summary or explicit no-lesson reason for workflow/tooling/rule changes. |
| `session-scratch.sh` | `scripts/dev/` | Print (and create) this session's private scratch dir `tmp/s/<session-id>/`. Use for ad-hoc command output instead of fixed names at the `tmp/` root, which collide between concurrent sessions. Removed at session end by `.claude/hooks/session-end-scratch.sh` (24h backstop in `session-start.sh`), which also sweeps this session's suffixed binaries `bin/*-<session-id>`. See `ai/rules/bash-output.md`. |
| `make ze-path` | `mk/session.mk` | Print THIS session's `ze` binary path. Under an AI session every canonical binary is built as `bin/<name>-<session-id>` (`ZE_BIN_SUFFIX`), so a sibling session's `make ze` cannot overwrite the binary you are testing against; off-session the name is the plain `bin/ze` it always was. **Never hardcode `bin/ze`** in a command, script, or doc -- use `$(make ze-path)`. Test binaries instead go to a private `bin/` subdir under `$(ZE_SCRATCH_DIR)` (`tmp/s/<id>/`), because `.ci` tests exec them by bare name. See `ai/rules/bash-output.md` "Your Binaries Are Session-Suffixed". |
| `learned_numbers.py` | `scripts/dev/` | Keep `plan/learned/NNN-*.md` numbering sound: no two summaries share a number, and each H1 number matches its filename. `learned-next` allocates max(existing prefixes)+1 against the local tree only, so parallel branches collide and only a merge or rebase reveals it. `--check` (gate: `make ze-learned-numbers-check`, folded into `make ze-doc-test` and `ze-regen-check`); `--fix` (`make ze-learned-numbers-fix`) keeps the most-referenced summary at the contested number, renumbers the rest above the highest, and rewrites references. Run after any merge/rebase touching `plan/learned/`. |
| `digest_check.py` | `scripts/dev/` | Validate the `file:line` anchors in `ai/digests/*.md`: each resolves to a real file (subsystem-relative via the digest's `<!-- digest-base: -->` header) and an in-range line. Keeps the hand-maintained flow digests honest as code moves. Gate: `make ze-digest-check`, folded into `make ze-doc-test`. |
| `learned_staleness.py` | `scripts/dev/` | Detect decay in `plan/learned/`: every path a summary lists in its `## Files` section still resolves, and every `plan/learned/NNN` citation still names a summary. Reads `## Files` AND every `## Files <qualifier>` section, so a summary spelling it `## Files Modified` is checked rather than skipped. A summary with no `## Files` section, or one that cannot be read, is a FINDING, never a silent pass. A `..` token or a symlink leaving the tree is reported, never resolved. Counted against the shrink-only ceiling `plan/.learned-staleness-baseline` (the `plan/.citation-baseline` idiom): more fails, fewer rewrites it down. Gate: `make ze-learned-staleness`, folded into `make ze-doc-test`. |
| `learned_normalise.py` | `scripts/dev/` | Normalise the section headings of `plan/learned/NNN-*.md` to the names `ai/rules/planning.md` gives them. A reader and `learned_staleness.py` then find `## Files` under one name. `--check` (`make ze-learned-normalise-check`) reports. `--fix` (`make ze-learned-normalise-fix`) rewrites in place. It sits outside `make ze-doc-test` on purpose. Heading drift breaks nothing, and a gate that reddens on a colleague's in-flight summary is soon disabled. |
| `learned_repath.py` | `scripts/dev/` | Repair the dead `## Files` paths that `learned_staleness.py` reports, for the common case where the code MOVED rather than went away. Three resolvers run, strongest evidence first: a git-recorded rename, a directory move collapsed from many such renames, and a unique three-segment path suffix. A path with several plausible successors, and a path with none, are both LEFT ALONE and counted. A citation repointed at the wrong file reads as true, and a dead one stays visible to the gate. The dead-path set comes from `learned_staleness.check` itself, so the tool and the gate cannot disagree about which citation is dead. `make ze-learned-repath-check` reports. `make ze-learned-repath-apply` writes. |
| `learned_retire.py` | `scripts/dev/` | **A completed one-shot with no make target, on purpose.** Band 1-400 is retired, so every future run selects nothing or is refused. It removed the numbered summaries of that band after `plan/learned/DESIGN-HISTORY.md` absorbed their surviving knowledge. `RETIREMENT_CEILING` is a module constant no argument raises. `ai/rules/never-destroy-work.md` requires operator permission to delete user-visible files, and that permission covered band 1-400 alone (2026-08-01). Three guards call `enforce_ceiling`, not one, so the ceiling is a property of the module and not of its argument parser. It never runs `git rm`. It unlinks in the working tree and prints the paths for `commit_helper.py --remove` to name. Read it before you write another corpus-pruning tool. Do not run it on a new band without fresh permission. |
| `check_doc_links.py` | `scripts/dev/` | Four checks over the instruction corpus. (1) Every backticked path and markdown link in `ai/`, `.claude/rules/` and the `plan/` meta documents resolves. `plan/learned/DESIGN-HISTORY.md` is scanned, not exempt. (2) Every `// Design:` target resolves. (3) Every `ai/LEARNED-INDEX.md` entry stays under the 120-character pointer budget (`ai/rules/detail-budget.md`). (4) Every backticked `*.sh` filename and `c_*`/`check_*` function name in the hook-describing documents names something in the tree. Those names resolve against top-level `def` names, not against the `CHECKS` registry. Gate: `make ze-doc-links`, folded into `make ze-doc-test` and `ze-regen-check`. |
| `spec-closure-check.py` | `scripts/dev/` | Detect specs implemented but never closed. `--list` shows the backlog in two tiers (high-confidence vs NEEDS VERIFICATION); `--spec <s>` exits 3 only for high-confidence (committed `plan/learned/NNN-<slug>.md` whose slug exactly equals the spec stem, spec `in-progress`, not an umbrella). Backs the Stop-hook closure gate. See `ai/rules/planning.md` "Closure Enforcement". |
| `ci_observer_recover_check.py` | `scripts/dev/` | Guard the `.ci` observer fail-closed property: no engine-touching call may sit inside a **recovering** `except` handler, because an exception unwinding through the observer's `finally` shutdown lands its sentinel on a stderr nothing relays -- the ordering defect that let real RPC errors pass as green in 332 of 346 observer files. The flagged call set is DERIVED (transitive closure over `ze_api.py` to `_call_engine`/`wait_for_shutdown`), so a new engine-touching helper is covered the day it is written. Its Go test runs the real scan and asserts zero, so `make ze-unit-test` enforces it with no make target to forget. See `ai/rules/testing.md` "Observer-Exit Antipattern". |
| `ste_check.py` | `scripts/dev/` | Review the repository's writing against ASD-STE100 Simplified Technical English, rule one (`ai/rules/simplified-technical-english.md`). It finds the six banned habits: synonym rotation, hedging, frozen verbs, marketing adjectives, run-ons, and phrasal verbs. Surfaces are Markdown, Go comments, YANG descriptions, and piped text. `make ze-ste-review` prints every finding with its `file:line` and the replacement. `make ze-ste-review-changed` limits that to changed files. `make ze-ste-check` compares each changed file against its own HEAD version and fails when a habit grew, printing the file and only the new findings. The BLOCKING form is `ste_problems` in `commit_helper.py`, scoped to the files of one commit. No baseline file exists to re-bless, so the one way to green is to rewrite the prose. |
| `go_extract.go` | `scripts/dev/` | Move Go symbols between files |
| `replace.py` | `scripts/dev/` | Bulk find-and-replace with diff preview (run without `--apply` to review, then `--apply` to write). Supports `--regex` and `--all`. |
| `yang_move.py` | `scripts/dev/` | Format-aware YANG path refactoring. When YANG nodes move, updates slash paths, set commands, brace blocks, and GetContainer chains across the codebase. `remove <seg> --under <path>`, `rename <old> <new> --under <path>`, `move <src> <dst>`. Preview by default, `--apply` to write. Run `--test` for self-tests. |
| `stress-repro.py` | `scripts/dev/` | Reproduce load-dependent / flaky-in-full-verify test failures WITHOUT the full suite: CPU+GC burners oversubscribe every core while many concurrent `ze-test <suite>` runs loop, capturing the first failure's untruncated output (`GOTRACEBACK=all`; optional `--race`). `<suite>` and `--test` are whitespace-split, so `"bgp plugin" --test 97` works. Add `--any-failure` for assertion flakes (a missed `expect=` never carries a crash signature, so the default crash-only mode reports "not reproduced" and discards the evidence). Honours `ZE_TEST_NO_BUILD`, so rebuild `bin/ze` after changing daemon source or the verdict is against a stale binary. Writes `tmp/stress-repro/<slug>-<ts>.log`. See `ai/rules/flaky-under-load.md`. |
| `rebase_learned.py` | `scripts/dev/` | Drive an in-progress rebase that keeps re-conflicting on the generated learned index (`ai/LEARNED-FULL-INDEX.md`): regenerates that derivable file mechanically at each stop and halts on anything needing judgment. Judgment flags `--take-theirs/--take-ours PATH`, `--accept-incoming-delete` (all logged). The human starts/aborts the rebase; the script only resolves. See `ai/rules/git-safety.md` "Rebase Onto Diverged main". |
| `rename_module_path.py` | `scripts/dev/` | Rename the Go module path repo-wide: every import, `go.mod`/`replace` target, goimports `local-prefixes`, build config, AND the directories whose names mirror the module path (`gokrazy/ze/builddir/<module>/`). Re-sorts import groups with `goimports -format-only -local <new>` because the module-local group is keyed by the module path. REFUSES `*.pb.go` (the rawDesc encodes `go_package` with a varint length prefix, so a textual rewrite corrupts it silently) and points at `make ze-proto-gen`; reports, never silently drops, occurrences that are not module paths (an absolute checkout path) and leftover references to the old HOST (hosting URLs, history). Re-stamps the `rfc/audit/*.json` verdict fingerprints the rename staled (they hash the whole enclosing test file, so one rewritten import line invalidates a verdict about untouched assertions), re-sealing ONLY where it can prove per file that HEAD's content under the rename equals the current content, and refusing anything else. Dry-run by default, `--apply` to execute; does no git operations. |
| `make ze-proto-gen` | `Makefile` | Regenerate `api/proto/*.pb.go` from `api/proto/ze.proto` using the vendored `protoc-gen-go` / `protoc-gen-go-grpc` (versions pinned by `go.mod`; needs `protoc` on PATH). Required after a module-path rename. |
| `bundle-html.py` | `gh-pages: presentations/tools/` | Inline local images, slides.md, and embeds into HTML as a self-contained file. Output: `<name>-inlined.html`. Accepts multiple files. |
| `make ze-verify-wiring-docs` | `mk/inventory.mk` | Changed-file-aware wiring, documentation, command, and inventory gate used by `make ze-verify`. |
| `go run ./scripts/status/verify_run.go ze-verify` | `scripts/status/verify_run.go` | Verify protocol runner used by `make ze-verify`. Writes `tmp/ze-verify.log`, per-stage logs, compact failure indexes, and `tmp/ze-verify.status`. |
| `verify-status.sh` | `scripts/dev/` | Checks whether the current tree is byte-identical to the last passing `ze-verify` run. Commit preparation must treat FRESH as authoritative and skip rerunning verify. |
| `make ze-doc-test` | `mk/inventory.mk` | Documentation drift, stale source anchors, and YANG command handler contract checks. |
| `make ze-rfc-check` | `scripts/dev/rfc_requirements.py` | Gate: every MUST-level requirement of an enrolled RFC (`rfc/enrolled.txt`) is bound to a positive AND a negative test, or carries a reasoned annotation. It also ratchets against HEAD (`ai/rules/rfc-compliance.md`, "the five ratchets"). Enrolment cannot shrink. A requirement cannot lose a polarity it had. A NEW summary with gated MUSTs must be enrolled. An extraction sign-off cannot disappear. Enrolling a stem not enrolled at HEAD requires `rfc/extraction/<stem>.json`. It validates `rfc/audit/*.json` on read: closed verdict enum, required fields, no dangling rid, no verdict claiming proof without a cited test. It ratchets the verdicts too. A `weak` or `wrong` finding cannot be deleted or silently upgraded. No verdict that existed at HEAD can vanish. |
| `make ze-rfc-index` | `scripts/dev/rfc_requirements.py` | Regenerates `ai/RFC-REQUIREMENTS.md`, which maps a requirement to its enforcing tests. It adds the Coverage-by-RFC backlog, which names what is still owed. It adds the Extraction sign-off table, which names what is still unbounded. It adds the Audit coverage section. That section names what is audited. It names what is PROVEN, which means a fresh `enforced` verdict. It names every requirement whose verdict says otherwise. Writes the ledger and nothing else -- never `rfc/audit/` (see `make ze-rfc-reseal`). |
| `make ze-rfc-extract STEM=<stem>` | `scripts/dev/rfc_requirements.py` | Writes an UNCLASSIFIED extraction skeleton to `rfc/extraction/<stem>.json`: every normative site of the RFC's own text and every section, each disposition null. A reviewer classifies them by hand; an unclassified site FAILS `ze-rfc-check`, so generating skeletons makes the gate redder, never greener. Required before enrolling a new stem. Contract: `rfc/extraction/README.md`. |
| `make ze-rfc-extraction-status` | `scripts/dev/rfc_requirements.py` | JSON envelope of the extraction counts the drain quota consumes: signed and enrolled counts, the per-register split (`rfc2119`/`prose`/`manual-walk`), and the unsigned backlog list. |
| `make ze-rfc-reseal` | `scripts/dev/rfc_requirements.py` | Re-stamps the `rfc/audit/*.json` verdicts that `ze-rfc-check` reports as SHIFTED. The tagged unit is the enclosing top-level function of each tagged test. SHIFTED means that unit is byte-identical. Only the file around it moved, through a line shift, a sibling test, or an import rewrite. Nothing was re-judged, so no human re-read is owed. It rewrites the `tests` fingerprints and nothing else. A verdict whose unit, cited producer code, or requirement text moved is REFUSED. That verdict stays stale for `/ze-rfc-audit`. **The only thing that writes `rfc/audit/` without a human editing it** -- `ze-rfc-check` is read-only and `ze-rfc-index` touches the ledger alone, deliberately, so re-stamping cannot happen as a side effect of unrelated work (`ai/skills/ze-rfc-audit.md`). Run `make ze-rfc-index` afterwards. |
| `make ze-inventory` / `make ze-inventory-json` | `mk/inventory.mk` | Registry-backed plugin, command, YANG, and test inventory. |
| `make ze-command-list` / `make ze-command-list-json` | `mk/inventory.mk` | Live command inventory generated from registered handlers and schemas. |
| `make ze-cli-grammar-check` / `-json` | `mk/inventory.mk` | CLI grammar gate: every built-in command obeys the verb-first rules R1-R9 (`ai/rules/cli-grammar.md`; R9 = compound-vs-namespace split) and no `.yang` carries a `--flag`. In `make ze-verify`. |
| `make ze-doc-index` | `mk/inventory.mk` | Regenerate `ai/CODE-TO-DOCS.md`, the source-to-document reverse index. |
| `make ze-rules-condensed` | `scripts/dev/rules_condensed.py` | Regenerate all three rule-digest artifacts from one parse: `ai/rules/TRIGGERS.md` (one routing line per rule, loaded every session), `ai/rules/CORE.md` (always-on directives, membership derived from the rung 1/2 ladder in `ai/rules/rule-precedence.md`), and `ai/rules/CONDENSED.md` (every rule's directives, read on demand). |
| `make ze-rules-payload` | `scripts/dev/rules_condensed.py` | What a session actually loads: `ai/INSTRUCTIONS.md` + `TRIGGERS.md` + `CORE.md`, in tokens, against the 40,000 budget and the digest it replaces. |
| `make ze-rules-router-report` / `-json` | `scripts/dev/rules_router.py` | Trigger-routing coverage over every past task description in `plan/`. It reports which rules the trigger index surfaces per task, and which BLOCKING rules no task surfaces at all. The generator derives the core from that second set, so a rule named here is already always-on. |
| `make ze-ai-sync` | `scripts/dev/skill_sync.sh` | Sync canonical `ai/skills/*.md` to `.claude/skills/`, `.codex/skills/`, and `.agents/skills/`; also regenerates `CLAUDE.md` and `AGENTS.md` from `ai/INSTRUCTIONS.md`. |
| `make ze-spec-status` / `make ze-spec-status-json` | `mk/inventory.mk` | Spec progress overview (committed backlog vs skeleton idea capture, stale-skeleton flags) plus a non-blocking completed-but-not-closed advisory. |
| `make ze-spec-citation-check` | `mk/inventory.mk` | Spec citation freshness: fails on a `plan/spec-*.md` citing a sibling spec absent on disk (grandfathered via `plan/.citation-baseline`); WARNs on `path:line` line-token drift. Runs on the verify path when a `plan/` file changes. |
| `make ze-mutation-test` | `mk/test-mutation.mk` | Mutation testing via gomu on all non-excluded packages (advisory, not gating). Vendored, no install needed. |
| `make ze-mutation-changed` | `mk/test-mutation.mk` | Incremental mutation testing on changed files only. |
| `make ze-mutation-report` | `mk/test-mutation.mk` | Mutation testing with HTML report output to `tmp/mutation-report.html`. |
| `make ze-test-health` | `scripts/dev/testing_health.py` | Regenerates `docs/features/test-health.md` and `test/health/latest.json`: whether a regression would be caught, not how many tests exist. Read it before claiming the suite is healthy. |
| `make ze-test-health-record` | `scripts/dev/testing_health.py` | Appends one KPI sample to `test/health/history.ndjson` (committed), then regenerates the page so trends stay in step. |
| `make ze-test-sensitivity-check` | `scripts/checks/inert_tests.go` | Ratchets tests that cannot fail and test files no `go test` target builds. Stage 10 of `ze-verify`, both modes. |
| `make ze-test-health-check` | `scripts/dev/testing_health.py` | Fails `ze-verify` when a STRUCTURAL fact drifts (an orphaned test file, an unproven RFC, a metric status). Volume counters are published, not gated. The target a developer meets when verify goes red on this feature. Runs inside `ze-regen-check-readonly`. |
| `make ze-setup` | `scripts/dev/dev-setup.py` | Unified dev setup: installs all build deps, linters, and appliance/evidence tools (qemu, e2fsprogs, xorriso, grub, uv; optional Linux L2TP-evidence deps xl2tpd, ppp). Also two Linux system-state prerequisites that are not packages: `userns-unrestricted` (Chrome sandbox) and `kvm-access` (the `kvm` group, without which QEMU evidence cannot start). OS autodetect (brew/apt). `CHECK=1` for probe-only mode. Drift-guarded against `applianceDoctorChecks()`. |

## Pattern Cookbooks

Mechanical recipes for creating common artifacts. Read before coding.

| Pattern | File | What it covers |
|---------|------|---------------|
| **Registration** | `ai/patterns/registration.md` | **All registries, startup flow, modular core architecture** |
| **BGP Family** | `ai/patterns/bgp-family.md` | **New SAFI, capability, or attribute: exhaustive 12-section checklist** |
| CLI Command | `ai/patterns/cli-command.md` | Offline/online dispatch, grammar, YANG tree, exit codes |
| Web Endpoint | `ai/patterns/web-endpoint.md` | Handler sequence, templates, HTMX OOB, route registration |
| Plugin | `ai/patterns/plugin.md` | register.go, logger, SDK protocol, filters, codecs |
| Config Option | `ai/patterns/config-option.md` | YANG leaf, env var, validator, naming across layers |
| Functional Test | `ai/patterns/functional-test.md` | .ci format, test directories, templates, expectations |

## Learned Summaries (Curated)

Structural decisions, patterns, and gotchas extracted from 500+ completed specs.
Full index: `ai/LEARNED-INDEX.md`. All summaries: `plan/learned/`.

## Architecture Docs

| Area | Doc |
|------|-----|
| **Core Design** | `docs/architecture/core-design.md` **(START HERE)** |
| **System Architecture** | `docs/architecture/system-architecture.md` |
| **Overview** | `docs/architecture/overview.md` |
| **Hub Architecture** | `docs/architecture/hub-architecture.md` |
| Buffer-first | `docs/architecture/buffer-architecture.md` |
| Message buffers | `docs/architecture/message-buffer-design.md` |
| Wire formats | `docs/architecture/wire/messages.md` |
| NLRI types | `docs/architecture/wire/nlri.md` |
| NLRI BGP-LS | `docs/architecture/wire/nlri-bgpls.md` |
| NLRI EVPN | `docs/architecture/wire/nlri-evpn.md` |
| NLRI FlowSpec | `docs/architecture/wire/nlri-flowspec.md` |
| NLRI qualifiers | `docs/architecture/wire/qualifiers.md` |
| MP NLRI ordering | `docs/architecture/wire/mp-nlri-ordering.md` |
| UPDATE packing | `docs/architecture/wire/update-packing.md` |
| Buffer writer | `docs/architecture/wire/buffer-writer.md` |
| Attributes | `docs/architecture/wire/attributes.md` |
| BGP-LS attr naming | `docs/architecture/wire/bgpls-attribute-naming.md` |
| Capabilities | `docs/architecture/wire/capabilities.md` |
| UPDATE building | `docs/architecture/update-building.md` |
| UPDATE cache | `docs/architecture/update-cache.md` |
| UPDATE density | `docs/architecture/update-density-analysis.md` |
| Memory pools | `docs/architecture/pool-architecture.md` |
| Pool review | `docs/architecture/pool-architecture-review.md` |
| Zero-copy | `docs/architecture/encoding-context.md` |
| RIB transition | `docs/architecture/rib-transition.md` |
| RIB storage | `docs/architecture/plugin/rib-storage-design.md` |
| Route types | `docs/architecture/route-types.md` |
| Route selection | `docs/architecture/route-selection.md` |
| FSM | `docs/architecture/behavior/fsm.md` |
| Signals | `docs/architecture/behavior/signals.md` |
| API | `docs/architecture/api/architecture.md` |
| API Capabilities | `docs/architecture/api/capability-contract.md` |
| API Commands | `docs/architecture/api/commands.md` |
| API JSON format | `docs/architecture/api/json-format.md` |
| IPC protocol | `docs/architecture/api/ipc_protocol.md` |
| Process protocol | `docs/architecture/api/process-protocol.md` |
| MuxConn wire format | `docs/architecture/api/wire-format.md` |
| UPDATE syntax | `docs/architecture/api/update-syntax.md` |
| Text format | `docs/architecture/api/text-format.md` |
| Text parser | `docs/architecture/api/text-parser.md` |
| Text coverage | `docs/architecture/api/text-coverage.md` |
| Config syntax | `docs/architecture/config/syntax.md` |
| Config environment | `docs/architecture/config/environment.md` |
| Environment block | `docs/architecture/config/environment-block.md` |
| Config tokenizer | `docs/architecture/config/tokenizer.md` |
| YANG design | `docs/architecture/config/yang-config-design.md` |
| ExaBGP syntax | `docs/architecture/config/exabgp-syntax.md` |
| VyOS research | `docs/architecture/config/vyos-research.md` |
| Plugin modes | `docs/architecture/cli/plugin-modes.md` |
| Plugin testing | `docs/architecture/debugging/plugin-testing.md` |
| Edge: ASN4 | `docs/architecture/edge-cases/as4.md` |
| Edge: ADD-PATH | `docs/architecture/edge-cases/addpath.md` |
| Edge: Extended msg | `docs/architecture/edge-cases/extended-message.md` |
| Route metadata | `docs/architecture/meta/README.md` |
| Role metadata | `docs/architecture/meta/role.md` |
| Forward pool | `docs/architecture/forward-congestion-pool.md` |
| Congestion industry | `docs/architecture/congestion-industry.md` |
| Subsystem wiring | `docs/architecture/subsystem-wiring.md` |
| Plugin mgr wiring | `docs/architecture/plugin-manager-wiring.md` |
| Hub API commands | `docs/architecture/hub-api-commands.md` |
| RFC MAY decisions | `docs/architecture/rfc-may-decisions.md` |
| ZeFS format | `docs/architecture/zefs-format.md` |
| Fleet config | `docs/architecture/fleet-config.md` |
| Web interface | `docs/architecture/web-interface.md` |
| Web components | `docs/architecture/web-components.md` |
| Chaos dashboard | `docs/architecture/chaos-web-dashboard.md` |
| CI format | `docs/architecture/testing/ci-format.md` |
| Interop testing | `docs/architecture/testing/interop.md` |
| ExaBGP mapping | `docs/exabgp/exabgp-code-map.md` |
| ExaBGP compat | `docs/exabgp/exabgp-differences.md` |

## Keyword → Architecture Doc

| Keywords | Docs |
|----------|------|
| buffer, iterator, parse, wire | `core-design.md`, `buffer-architecture.md`, `ai/rules/buffer-first.md` |
| encode, Pack, WriteTo, alloc | `ai/rules/buffer-first.md`, `buffer-architecture.md` |
| UPDATE, message, build, route | `core-design.md`, `update-building.md`, `encoding-context.md` |
| attribute, AS_PATH, NEXT_HOP, MED | `core-design.md`, `wire/attributes.md`, `update-building.md` |
| community, ext community, large community | `wire/attributes.md` |
| NLRI, prefix, MP_REACH, MP_UNREACH | `core-design.md`, `wire/nlri.md` |
| multiprotocol, AFI, SAFI, new family, new SAFI | `ai/patterns/bgp-family.md`, `wire/nlri.md`, `wire/capabilities.md` |
| capability, OPEN, negotiate | `wire/capabilities.md` |
| pool, memory, dedup, zero-copy, lifecycle | `ai/rules/memory-architecture.md`, `core-design.md`, `pool-architecture.md`, `encoding-context.md` |
| textbuf, string building, AppendTo, alloc-free | `ai/rules/no-sprintf-alloc.md`, `ai/rules/memory-architecture.md`, `internal/core/textbuf/` |
| error message, actionable error, corrective action, remediation, fail closed | `ai/rules/error-messages.md`, `ai/rules/exact-or-reject.md`, `ai/rules/derive-not-hardcode.md` |
| guard, fail open, fail closed, silent no-op, zero value, valid-looking zero, bare map read, permissive default, inert constraint, dead guard | `ai/rules/fail-closed-guards.md`, `plan/learned/1157-fail-open-auth-empty-profiles.md` |
| sync.Pool, buffer pool, ring buffer, peerPool | `ai/rules/memory-architecture.md`, `forward-congestion-pool.md` |
| forward, reflect, wire cache | `core-design.md`, `encoding-context.md`, `update-building.md` |
| route, rib, storage | `core-design.md`, `route-types.md`, `rib-transition.md`, `plugin/rib-storage-design.md` |
| route selection, best path | `route-selection.md` |
| FSM, state, session, peer | `behavior/fsm.md` |
| signal, SIGHUP, SIGUSR | `behavior/signals.md` |
| reload fence, reload generation, show reload-status, wait for reload, reject/no-op observability | `docs/architecture/api/commands.md` ("show reload-status"), `internal/component/plugin/server/reload_generation.go`, `cmd/ze/hub/main_reload.go` |
| API, command, announce, withdraw | `docs/architecture/api/architecture.md`, `docs/architecture/api/capability-contract.md`, `docs/architecture/api/commands.md` |
| text format, IPC, formatter, parser | `docs/architecture/api/text-format.md`, `docs/architecture/api/text-parser.md`, `docs/architecture/api/text-coverage.md` |
| IPC, wire format, muxconn | `docs/architecture/api/ipc_protocol.md`, `docs/architecture/api/wire-format.md`, `docs/architecture/api/process-protocol.md` |
| JSON, event format | `docs/architecture/api/json-format.md` |
| config, load | `config/syntax.md`, `config/tokenizer.md` |
| environment, env vars | `config/environment.md`, `config/environment-block.md` |
| web, dashboard, UI | `web-interface.md`, `web-components.md`, `chaos-web-dashboard.md` |
| subsystem, wiring, plugin manager | `subsystem-wiring.md`, `plugin-manager-wiring.md` |
| bridge, direct call, request/response, sync handler | `core-design.md` (section 9), `ai/rules/plugin-design.md` (DirectBridge), `plan/learned/DESIGN-HISTORY.md` "Plugin system: architecture" |
| forward pool, congestion | `forward-congestion-pool.md`, `congestion-industry.md` |
| hub, API commands | `hub-architecture.md`, `hub-api-commands.md` |
| cache, update cache | `update-cache.md`, `update-density-analysis.md` |
| metadata, route meta | `meta/README.md` |
| interop, test infra, raw injector, inject.msg sidecar, python speaker, speaker-args, independent bgp peer | `testing/interop.md`, `testing/ci-format.md`, `../plan/spec-bgp-plugin-speaker.md` |
| zefs, blob, netcapstring, storage | `zefs-format.md`, `fleet-config.md` |
| fleet, managed, server, backup, bootstrap | `fleet-config.md` |
| FlowSpec | `wire/nlri.md`, `wire/nlri-flowspec.md` |
| VPN, L3VPN, MPLS-VPN, 6PE | `wire/nlri.md` |
| EVPN, MAC-IP | `wire/nlri.md`, `wire/nlri-evpn.md` |
| BGP-LS, link-state | `wire/nlri-bgpls.md`, `wire/bgpls-attribute-naming.md` |
| ExaBGP, migrate, exabgp.yang | `exabgp/exabgp-code-map.md`, `exabgp/exabgp-differences.md`, `ai/patterns/bgp-family.md` (Section 5b) |
| ASN4, AS4 | `edge-cases/as4.md` |
| ADD-PATH | `edge-cases/addpath.md` |
| extended message | `edge-cases/extended-message.md` |
| test, functional, .ci, verify failures | `docs/functional-tests.md` (top-level, not architecture/), `testing/ci-format.md` |
| RFC requirement coverage, RFC MUST tests, rfc-requirements, RFC requirement tag, ze-rfc-check, ze-rfc-index | `make ze-rfc-check`, `ai/RFC-REQUIREMENTS.md`, `ai/skills/ze-rfc.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/functional-tests.md` (RFC Requirement Tags) |
| RFC extraction sign-off, extraction completeness, what the summary MISSED, unextracted obligation, normative site, extraction register, rfc2119/prose/manual-walk, drain budget, ze-rfc-extract, ze-rfc-extraction-status | `rfc/extraction/README.md`, `make ze-rfc-extract`, `make ze-rfc-extraction-status`, `rfc/drain-budget.txt`, `ai/rules/rfc-compliance.md` (Extraction Completeness, the five ratchets), `ai/RFC-REQUIREMENTS.md` (Extraction sign-off) |
| RFC audit verdict, rfc/audit schema, enforced/weak/wrong/unimplemented/not-applicable, no_code_path, upgrade_reason, units map, code map, SHIFTED verdict, stale audit verdict, ze-rfc-reseal, audit coverage | `ai/skills/ze-rfc-audit.md` (the verdict vocabulary and the four freshness states), `make ze-rfc-reseal`, `ai/RFC-REQUIREMENTS.md` (Audit coverage), `scripts/dev/rfc_tagged_scope.py` (the one definition of "the tagged unit"), `ai/rules/rfc-compliance.md` (a verdict is never authority) |
| payload-predicate waits, sleep elimination, ci-sleep ratchet, ci-sleep justification, time.sleep comment, wait_until, dispatch_until, wait_for_event predicate, engine-step predicates (matches=/absent=/json=) | `docs/functional-tests.md` ("Payload-predicate waits"), `docs/architecture/testing/ci-format.md` ("Engine Steps"), `ai/rules/testing.md` (Observer API), `ai/rules/ci-sleep-justification.md`, `test/scripts/ze_api.py`, `internal/test/runner/engine_steps.go` |
| netdata, telemetry, prometheus, metrics, monitoring, collector | `docs/guide/monitoring.md`, `docs/features.md`, `plan/learned/653-netdata-os-collectors.md` |
| DHCP, dhcp-server, lease, pool | `internal/plugins/dhcpserver/` (plugin), `ze-dhcp-server-conf.yang` |
| NTP, time sync | `internal/plugins/ntp/` (plugin), `ze-ntp-conf.yang` |
| AS112, anycast DNS, EMPTY.AS112.ARPA, reverse DNS sink, RFC 7534, RFC 7535 | `internal/plugins/as112/` (plugin), `internal/core/dnsserver/` (shared DNS harness), `internal/component/iface/address_owner.go` (address-ownership registry), `ze-as112-conf.yang`, `docs/guide/as112.md` |
| sysctl, kernel tuning, profile | `internal/component/sysctl/` (plugin), `ze-sysctl-conf.yang` |
| firewall, nftables, NAT, masquerade | `internal/component/firewall/` (component), `ze-firewall-conf.yang` |
| PPPoE, pppoe-client, access concentrator | `internal/component/l2tp/pppoe/` (AC), `internal/component/iface/` (client), `ze-pppoe-conf.yang` |
| wireguard, WireGuard, wg | `internal/component/iface/wireguard.go`, `ze-iface-conf.yang` |
| VRRP, first-hop redundancy, virtual router, keepalived, gateway redundancy, virtual MAC | `internal/plugins/vrrp/` (plugin), `internal/component/iface/macvlan.go` (per-group virtual-MAC macvlan), `ze-vrrp-conf.yang`, `docs/guide/vrrp.md`, `docs/features/rfc-status.md` (First-hop redundancy) |
| static route, default route | `internal/plugins/static/` (plugin), `ze-static-conf.yang` |
| conntrack, connection tracking | `internal/component/config/system/conntrack.go`, `ze-system-conf.yang` |
| archive, config backup, revision | `internal/component/config/archive/`, `ze-system-conf.yang` |
| SSH, authentication, user, public-key | `internal/component/ssh/`, `ze-ssh-conf.yang` |
| IPsec, IKE, IKEv2, SA, child SA | `plan/learned/734` (data model), `plan/learned/739` (crypto), `plan/learned/740` (engine), `plan/learned/742` (child SA) |
| EAP, NAT-T, MOBIKE | `plan/learned/744` (EAP/NAT-T), `plan/learned/737` (EAP extension) |
| XFRM, xfrm interface, VTI | `plan/learned/735` (XFRM interfaces) |
| subscriber, session, PPPoE, L2TP | `plan/learned/760-subscriber-session-model.md`, `internal/component/l2tp/pppoe/` |
| editor, TUI, completion, headless | `internal/component/cli/`, `test/editor/`, `ai/rules/testing.md` (Editor Tests section) |
| diagnostic, doctor, health, readiness | `plan/learned/755-ze-doctor.md`, `ai/rules/doctor-checks.md`, `plan/learned/727-diag-core.md` |
| EventBus, event, pub/sub, subscribe, emit | `pkg/ze/eventbus.go`, `ai/rules/plugin-design.md` (EventBus Typed Payloads), `internal/core/events/typed.go` |
| DirectBridge, bridge, direct call, typed handler | `pkg/plugin/rpc/bridge.go`, `ai/rules/plugin-design.md` (DirectBridge), `plan/learned/DESIGN-HISTORY.md` "Plugin system: architecture" |
| BFD, bidirectional forwarding | `docs/architecture/bfd.md` |
| resolve, origin, pipe, pipe operator | `docs/architecture/resolve.md`, `ai/rules/pipe-completeness.md` |
| MCP, model context protocol | `docs/architecture/mcp/`, `internal/component/mcp/` |
| self-update, manifest, auto-update | `plan/learned/748-cpe-6-self-update.md` |
| ASPA, path verification, RTR | `plan/learned/721-bgp-2-aspa.md`, `plan/learned/722-spec-bgp-4-aspa-policy.md` |
| BMP, monitoring protocol | `plan/learned/574-bgp-4-bmp.md`, `plan/learned/647-bmp-5-sender-compliance.md` |
| docker, container, scratch | `plan/learned/753-docker-go126.md`, `docs/guide/docker.md` |
| chaos, fault injection, scheduler | `plan/learned/723-chaos-actions-v2.md`, `docs/architecture/chaos-web-dashboard.md` |
| commit, commit script, commit message, lesson learned, verified commit, verify freshness, owner override, commit no test | `scripts/dev/commit_helper.py`, `scripts/dev/verify-status.sh`, `ai/rules/git-safety.md`, `ai/skills/ze-commit.md`, `ai/skills/ze-commit-check.md` |
| weekly update, Zeledon, ze-news, Discord announcement, gh-pages changes, homepage latest updates | `ai/skills/ze-weekly-update.md`, `../gh-pages/AI.md`, `../gh-pages/tools/render-index.py`, `scripts/zeledon/STYLE.md` |
| self-improvement, discoverability, discovery, new tool, self-check, verification gate | `ai/rules/discovery-updates.md`, `ai/rules/hook-mapping.md`, `docs/contributing/documentation-testing.md` |
| inventory, command-list, doc drift, source anchor, doc index | `ai/rules/discovery-updates.md`, `ai/rules/documentation.md`, `docs/contributing/documentation-testing.md`, `mk/inventory.mk` |
| clear, clear command, clear dns, clear interface, clear ipsec | `internal/component/resolve/cmd/` (dns), `internal/component/iface/cmd/` (interface), `internal/component/ike/cmd/` (ipsec), `internal/component/cmd/clear/` (verb root) |
| command grammar, verb-first, command alias, deprecated alias, grammar gate | `ai/rules/cli-grammar.md` (Mechanical Enforcement), `make ze-cli-grammar-check`, `plan/learned/829-command-verb-first.md` |
| DispatchCommandArgs, typed inter-plugin dispatch, tokenizer bypass | `plan/learned/830-typed-inter-plugin-dispatch.md`, `ai/rules/plugin-design.md` |
| RawMessage, double marshal, callback passthrough, SDK callback | `plan/learned/826-ipc-dispatch-data-raw.md`, `plan/learned/827-dispatch-response-passthrough.md`, `plan/learned/828-codec-callback-passthrough.md` |
| pipe first, pipe last, pipe metadata | `ai/rules/pipe-completeness.md`, `plan/learned/822-pipe-first-last.md` |
| RIB dump, bounded dump, replay batching, update cursor | `plan/learned/823-rib-show-bounded-dump.md`, `plan/learned/824-rib-feed-replay-batch.md` |
| plugin internal keyword, in-process plugin config | `plan/learned/1145-plugin-internal-keyword.md`, `ai/patterns/plugin.md` |
| appliance auth, local admin, bootstrap auth, RBAC | `plan/learned/831-appliance-auth-hardening.md`, `internal/component/authz/`, `internal/component/aaa/` |
| appliance, appliance iso, appliance build, appliance init | `internal/appliance/`, `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `scripts/evidence/effective-install-iso-qemu.py`, `mk/test-integration.mk` |
| Dependabot alert on vendored go.mod, gokrazy/modcache manifest, bump gokrazy init, appliance dependency bump, CVE on vendored appliance dep | `ai/rules/appliance-dep-bumps.md`, `mk/gokrazy.mk` (`ze-gokrazy-deps`), `.github/dependabot.yml` |
| installer initrd QEMU evidence, R-6 fault injection, ze.mac pin, rescue console, Ventoy ISO-on-FAT, ze_installer_fault, ZE_INITRD_FAULT | `scripts/evidence/effective-install-scenarios-qemu.py`, `scripts/evidence/effective-install-ventoy-qemu.py`, `internal/install/disk/fault_linux.go`, `mk/test-integration.mk` (`ze-install-scenarios-qemu-test`, `ze-install-ventoy-qemu-test`), `docs/functional-tests.md` |
| VPP hugepage boot reservation, poll-sleep-microseconds, image.hugepages, doctor-vpp-hugepages, hugepage QEMU evidence | `internal/appliance/kernelargs.go`, `internal/component/vpp/doctor_linux.go`, `internal/component/vpp/startupconf.go`, `scripts/evidence/effective-vpp-hugepages-qemu.py`, `mk/test-integration.mk` (`ze-vpp-hugepages-qemu-test`), `docs/guide/vpp.md`, `docs/guide/appliance.md` |
| VPP semantics, linux-cp, LCP, LCP netns, lcp_itf_pair_create, default netns, binapi, lcp.ba.go, foreign system semantics | `third_party/vpp-linux-cp/` -- vendored VPP C (v25.10, read-only reference). Read this BEFORE claiming what VPP does; the generated stub `vendor/go.fd.io/govpp/binapi/lcp/lcp.ba.go` says a field exists, never what VPP does with it (`ai/rules/no-fabrication.md`) |
| .ci test prerequisite, option=needs-path, caps=net-raw, caps=net-admin, caps=bpf, test skips instead of failing, missing modcache, ze-gokrazy-deps prerequisite | `docs/architecture/testing/ci-format.md` (Options table), `ai/rules/qemu-testing.md`, `internal/test/runner/caps.go`, `internal/test/runner/needs_path.go` |
| test passes on macOS but fails in CI, works locally red in CI, unprivileged runner, 4-vCPU runner | `plan/learned/1275-fixit-ci-green.md`, `ai/rules/qemu-testing.md` (skip-os is not a capability declaration), `ai/rules/fix-dont-record.md` |
| code-to-docs, reverse index, which docs | `ai/CODE-TO-DOCS.md` (generated, `make ze-doc-index`) |
| mutation testing, gomu, mutation score, mutant | `mk/test-mutation.mk`, `ai/rules/testing.md` (Mutation Testing section) |
| test health, testing dashboard, proof density, assert-nothing, tests that cannot fail, tag-orphan, test KPI, is our testing correct | `docs/features/test-health.md`, `docs/architecture/testing/test-health.md` (architecture), `test/health/README.md`, `scripts/dev/testing_health.py`, `scripts/checks/inert_tests.go`, `ai/rules/testing.md` (Test Sensitivity Ratchets) |
| find bugs, hunt bugs, bug classes, latent bugs, recurring traps, taxonomy sweep, silent fall-through, unwired feature | `ai/skills/ze-hunt.md`, `plan/learned/RECURRING-PATTERNS.md` |

All architecture docs in `docs/architecture/` unless noted.

## Keyword → RFC

| Keywords | Primary RFC | Related |
|----------|-------------|---------|
| open, capability | `rfc5492` | `rfc9072` |
| update, nlri, prefix | `rfc4271` | `rfc4760` |
| multiprotocol, mp-bgp | `rfc4760` | |
| notification, error | `rfc4271` | `rfc7606`, `rfc9003` |
| route-refresh | `rfc2918` | `rfc7313` |
| community | `rfc1997` | |
| extended community, RT | `rfc4360` | `rfc5701` |
| large community | `rfc8092` | `rfc8195` |
| 4-byte AS, ASN4 | `rfc6793` | |
| add-path | `rfc7911` | |
| graceful restart | `rfc4724` | |
| extended message | `rfc8654` | |
| label, mpls | `rfc8277` | `rfc3032` |
| vpn, l3vpn, 6pe | `rfc4364` | `rfc4659`, `rfc4798` |
| flowspec | `rfc8955` | `rfc8956` |
| evpn | `rfc7432` | `rfc9136` |
| vpls | `rfc4761` | `rfc4762` |
| bgp-ls | `rfc7752` | `rfc9085`, `rfc9514` |
| role, otc | `rfc9234` | |
| ipv6 next hop | `rfc8950` | |
| shutdown | `rfc9003` | `rfc8203` |
| treat-as-withdraw | `rfc7606` | |

RFC summaries: `rfc/short/`. Full RFCs: `rfc/full/`.

## Session State

Per-session: `tmp/session/session-state-<spec-stem>-<SID>.md` (gitignored). Each session gets its own file.
Session markers: `tmp/session/.session-<ID>` map sessions to specs. See `hooks/lib/state-file.sh`.
On startup, `_find_latest_state_for_spec()` finds the most recent state file for a spec from any previous session.
