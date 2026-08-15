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
| Why is the code shaped this way? | `plan/learned/DESIGN-HISTORY.md` |
| Which problems recur? | `plan/journal/` (one file per class; `make ze-journal` prints classes with 2+ rows) |
| Which rule covers a topic? | `ai/rules/INDEX.md` |
| How does data flow through a subsystem? | `docs/architecture/core-design.md` (START HERE), then the subsystem doc below |
| Fast subsystem orientation (entry→exit, with `file:line`) | `ai/digests/<subsystem>.md` — living flow digests; index + list in `ai/digests/README.md`. Anchors gated by `make ze-digest-check` |

## I Want To...

| Task | Read first | Then |
|------|-----------|------|
| Understand the modular core | `ai/patterns/registration.md` | `docs/architecture/core-design.md` |
| Keep a plugin self-contained (removal test) | `ai/rules/plugins.md` | Remove the plugin and ALL its features vanish; other plugins and core keep working |
| Call another package's function directly from a plugin (not through RPC) | `ai/rules/plugins.md` | Check `p.IsInternal()`; guard with refuse-or-warn depending on how much value survives running external. Gated by `make ze-plugin-boundary-check` |
| Choose internal/core vs internal/component vs internal/plugins for a new package | `ai/rules/architecture.md` | Tier = dependency direction; engine placement gated by `make ze-tier-check` (`scripts/dev/dep_audit.py --check`) |
| Test linux-only code (QEMU) | `ai/rules/platform-linux.md` | `ai/rules/testing.md` (Linux-Only Tests section) |
| Fix a failing test, gate, demo, or user-visible problem | `ai/rules/completion.md` | Implement the missing behavior at the source, never route around it |
| Decide how much machinery a fix or feature needs (KISS, MVP, over-engineering) | `ai/rules/simplicity.md` | The simplest FULLY CORRECT answer, nothing beyond it. Cuts machinery, never correctness. A second problem gets its own spec, never an extra branch in this fix |
| Modify wire encoding | `ai/rules/performance.md` | `docs/architecture/buffer-architecture.md` |
| Add route processing | `ai/rules/architecture.md` | `docs/architecture/core-design.md` |
| Detect and auto-mitigate a DDoS flood | `docs/guide/ddos-mitigation.md` | `ddos-detect` characterizes the attack (family + vector) from `traffic-usage`/`flow-export`; `ddos-local`/`ddos-flowspec` install surgical rules; `show flow recent` inspects the flow ring |
| Detect behavioral security anomalies (exfil, C2, scanning) | learned `1046`/`1048`/`1049` | Neutral facts in `internal/component/trafficfeature` (fan-out, out/in ratio, entropy, beaconing) on `internal/core/stats`; `anomaly/detect` (report-only) scores per-entity deviation + cohort rarity into incidents (`show anomaly`); `anomaly/shape` responds shadow-first (per-source rate-limit, arm/auto-revert/kill-switch, `show anomaly-shape`). Separate security domain from `ddos`. |
| Provide or extend first-hop gateway redundancy (VRRP) | `docs/guide/vrrp.md` | RFC 9568/3768 in `internal/plugins/vrrp/` (self-contained plugin) with the per-group virtual-MAC macvlan in `internal/component/iface/macvlan.go`; extend within the self-contained `internal/plugins/vrrp/` plugin |
| Implement an RFC | `ai/rules/rfc-compliance.md` | `docs/contributing/rfc-implementation-guide.md` |
| Prove an RFC MUST is enforced (tag a test, coverage gate) | `ai/skills/ze-rfc.md` | Tag the test `RFC requirement: <id> <polarity>` (both polarities); `make ze-rfc-check` gates coverage; `make ze-rfc-index` writes `rfc/requirements/<stem>.md` for that RFC's rows and `ai/RFC-REQUIREMENTS.md` for the index over all of them; audit with `/ze-rfc-audit` |
| Write a spec | `ai/rules/planning.md` | `plan/TEMPLATE.md` (design-time only; placeholders are legal at `skeleton`, blocked from `design` on) |
| Close a spec (audit, goal validation, review gate, pre-commit evidence) | `ai/rules/completion.md` | `plan/TEMPLATE-CLOSURE.md`, appended by `/ze-close` at step 1; every Pre-Commit sub-table needs an evidence row |
| Record design risks and assumptions | `ai/rules/planning.md` (Risks & Assumptions) | A-N/R-N tables in `plan/TEMPLATE.md`; validate during /ze-implement audit |
| Add a feature, tool, self-check, verification gate, or test infrastructure | `ai/rules/repo-maintenance.md` | Update docs, rules, indexes, and verification paths in the same change |
| Change a web or looking-glass template, a handler, or a route, or prove its bytes did not move | `ai/patterns/web-endpoint.md` | `make ze-web-golden-check` runs THREE captures. The template one compares each rendered template against `testdata/golden/`. The handler one issues an HTTP request to every route and compares the response against `testdata/handler/`. The markup one renders the HTML builders that no template holds, over fixed input, against `testdata/markup/`. Capture a DELIBERATE change with `make ze-web-golden-update` and read the diff |
| Compare Ze with other products | `ai/rules/writing.md` | Cite every claim, link code or official feature docs, label uncertainty, and add hide-column controls for wide product matrices |
| Add or change an agent behavior rule | `ai/rules/repo-maintenance.md` | Put shared Ze rules in `ai/rules/` and startup pointers in `ai/INSTRUCTIONS.md` |
| Reorganize YANG tree | `scripts/dev/yang_move.py --help` | Preview diff, then `--apply` |
| Move a package between tiers | `scripts/dev/migrate_module.py --help` | Dry-run plan, then `--apply` |
| Rename the module path (host or owner change) | `scripts/dev/rename_module_path.py --help` | Dry-run plan, then `--apply`; regenerate protobuf with `make ze-proto-gen` |
| See which rule covers a topic | `ai/rules/INDEX.md` | One-line overview of every rule; open the listed file before acting |
| Understand Ze vs standard Go | `ai/rules/architecture.md` | Buffer-first, registration, YANG, etc. |
| Know which hooks will check my code | `ai/rules/repo-maintenance.md` | Pre-flight compliance checklist |
| Edit the website or presentations | `docs/contributing/gh-pages.md` then `../gh-pages/AI.md` | Worktree layout, tooling, adding a talk |
| Write and publish the weekly update | `ai/skills/ze-weekly-update.md` | Draft in Zeledon voice, update `../gh-pages`, post the approved message to `ze-news`, and verify site/feed/homepage output |

## By Task Type

### Adding a Feature

| Feature kind | Read first | Then read | Cross-cutting |
|---|---|---|---|
| CLI command | `ai/patterns/cli-command.md` | `ai/rules/cli.md`, `ai/rules/cli.md`, `docs/architecture/cli/command-namespacing.md` (rooting + filters-not-flags) | `ai/rules/evidence.md` if it lists things |
| Web page/endpoint | `ai/patterns/web-endpoint.md` | `docs/architecture/web-interface.md`, `docs/architecture/web-components.md` | SSE: `docs/architecture/web-components.md` SSE section |
| Plugin | `ai/patterns/plugin.md` | `ai/rules/plugins.md`, `ai/rules/goroutine-lifecycle.md` | `ai/rules/go-standards.md` for registered names; `ai/rules/plugins.md` if it calls another package's function directly; `ai/rules/evidence.md` because registration metadata feeds the generated website plugin catalog |
| Config option | `ai/patterns/config-option.md` | `ai/rules/config.md` (listener pattern if network endpoint) | `ai/rules/config.md` (YANG vs env var), `ai/rules/config.md` (naming), `ai/rules/go-standards.md` env var section |
| NLRI family | **`ai/patterns/bgp-family.md`** (BLOCKING) | `docs/architecture/wire/nlri.md`, `ai/rules/performance.md` | `ai/rules/plugins.md` family registration |
| Capability | **`ai/patterns/bgp-family.md`** (BLOCKING) | `docs/architecture/wire/capabilities.md` | |
| Attribute | **`ai/patterns/bgp-family.md`** (BLOCKING) | `docs/architecture/wire/attributes.md` | `docs/architecture/encoding-context.md` |
| Functional test | `ai/patterns/functional-test.md` | `docs/architecture/testing/ci-format.md` | `ai/rules/testing.md` for format selection (.ci vs .et vs Go) |
| Editor test | `ai/rules/testing.md` (Editor Tests section) | `test/editor/` existing examples | |
| Telemetry/metrics | `docs/guide/monitoring.md` | `ai/digests/observation-telemetry.md` | Registration in loader_create.go |
| Observation feed, traffic observation, multi-subscriber fan-out | `docs/architecture/observation-feed.md` | `ai/digests/observation-telemetry.md` | `internal/core/observation/` (Feed, Observation); `iface/rate.go` (SubscribeCollectNotify) |
| Debug flags for a plugin | `ai/patterns/debug-registration.md` | `internal/component/bgp/yang/register_debug.go` (example) | One file per plugin: `register_debug.go` in yang/ |
| Diagnostic command | `docs/guide/production-diagnostics.md` | `internal/core/diagnostic/codes.go` | `ai/rules/repo-maintenance.md` |
| Agent-facing command/tool | `ai/rules/cli.md` | `docs/features/ai-first.md`, `docs/guide/mcp/overview.md` | `ai/rules/repo-maintenance.md` for indexes and verification |
| Verification/self-check gate | `ai/rules/repo-maintenance.md` | `ai/rules/repo-maintenance.md`, `docs/contributing/documentation-testing.md` | `mk/inventory.mk` for doc/inventory targets |
| EventBus event | `ai/rules/plugins.md` (EventBus Typed Payloads) | `pkg/ze/eventbus.go` | Use `events.Register[T]`, not raw `bus.Subscribe` |
| DirectBridge handler | `ai/rules/plugins.md` (DirectBridge section) | `pkg/plugin/rpc/bridge.go`, `plan/learned/DESIGN-HISTORY.md` "Plugin system: architecture" (294, retired) | |
| New component | `docs/architecture/core-design.md` section 1 | `ai/rules/architecture.md`, `ai/rules/architecture.md` | Proximity principle in `ai/rules/plugins.md` |
| New subsystem | `docs/architecture/hub-architecture.md` | `docs/architecture/subsystem-wiring.md` | |
| Test runner or format | `ai/rules/testing.md` | `ai/patterns/functional-test.md`, `docs/architecture/testing/ci-format.md` | `ai/rules/repo-maintenance.md` |


### Preparing a Commit

| Task | Read first | Then use |
|---|---|---|
| Generate and run a commit script | `ai/rules/git-safety.md` | Fast path: use `scripts/dev/commit_helper.py create`, then run it with `bash` and the path its `script=` line prints. if verification is considered, run `scripts/dev/verify-status.sh check` first and never rerun verify when FRESH |
| Record a problem the work uncovered | `ai/rules/planning.md` ("Writing Journal Rows") | Append a row to `plan/journal/<class>.md`, then `--file` it on commit A; `make ze-journal` prints every class that repeated |

### Modifying Existing Code

| Area | Read first | Key concerns |
|---|---|---|
| Reactor / session | `docs/architecture/core-design.md` sections 1-5 | `ai/rules/goroutine-lifecycle.md`, `make ze-race-reactor` required |
| Wire encoding/decoding | `ai/rules/performance.md`, `ai/rules/performance.md` | No `make()`, no `append()`, `WriteTo(buf, off) int`, caller-owned buffers |
| RIB / route storage | `docs/architecture/route-types.md`, `docs/architecture/rib-transition.md` | Pool dedup, lazy iterators |
| Route selection | `docs/architecture/route-selection.md` | `plan/learned/DESIGN-HISTORY.md` ("BGP engine: wire encoding and RIB") |
| Config pipeline | `docs/architecture/config/yang-config-design.md` | File -> Tree -> ResolveBGPTree -> map[string]any |
| Plugin SDK | `ai/rules/plugins.md` (SDK Is Generic) | No plugin-specific code in SDK |
| Hub / engine | `docs/architecture/hub-architecture.md` | Protocol-agnostic, Coordinator pattern |
| Forward pool | `docs/architecture/forward-congestion-pool.md` | Two-tier model, per-peer workers |
| YANG schemas | `ai/rules/config.md` | Augment vs grouping, listener pattern, `ai/rules/config.md` (leaf naming), `ai/rules/config.md` (YANG vs env var) |
| Registration code | `ai/patterns/registration.md` | `init()` + registry + blank import pattern |
| Show enricher | `ai/patterns/registration.md` (Show Enricher Registry) | `internal/core/show/` -- in-process via `show.MustRegister()`; external via `EnricherDecl` + `ze-plugin-callback:enrich-show`; web via explicit `show.Enrich()` |

### Fixing a Bug

```
1. Read ai/rules/architecture.md (sibling call-site audit)
2. Read ai/rules/completion.md (no rationalizing test failures)
3. Grep ALL implementations of the function/protocol step (ai/rules/completion.md)
4. Check plan/learned/RECURRING-PATTERNS.md for known traps in this area
5. After fixing: ai/rules/testing.md iteration workflow
```

### Writing Documentation

```
1. Read docs/contributing/writing-style.md (rule one: the six banned habits, the limits)
2. Read ai/rules/writing.md (categories, source anchors)
3. Read ai/rules/repo-maintenance.md if the doc adds or changes a feature, tool, check, gate, or test path
4. Read the actual source before any factual claim
5. Add <!-- source: path -- symbol --> anchors
6. Run make ze-ste-review-changed to read your own prose back
7. Run make ze-doc-test after editing docs/
```

### Working with IPsec / IKE

Read `ai/digests/ipsec-ike.md`: it carries the data model, the crypto, the engine,
the child SA and the EAP/NAT-T paths with their file and symbol anchors.

### Working with CPE / Subscriber

Read `ai/digests/subscriber.md` for the session model, `ai/digests/iface.md` for the
DHCP ranges, and `ai/digests/firewall.md` for the firewall global options.

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
grep -rn "keyword" plan/journal/ --include="*.md" -l
grep -rn "keyword" ai/rules/ --include="*.md" -l
```

## Cross-Cutting Rules (Apply Regardless of Area)

These rules are frequently missed because they don't map to a single
artifact type. Check them whenever your work touches the described concern.

| Concern | Rule | When it applies |
|---|---|---|
| Every word you write | `ai/rules/writing.md`, guide: `docs/contributing/writing-style.md` | Rule one. ASD-STE100 Issue 9 for all repository writing: docs, comments, error messages, CLI output, YANG descriptions, specs, commit and PR text. Six banned habits. Gate: `make ze-ste-check`. Report: `make ze-ste-review` |
| How much you write | `ai/rules/writing.md` | Any report, rule, doc, commit body, or learned summary. Per-artifact budgets |
| Listing/enumerating things | `ai/rules/evidence.md` | Help text, usage strings, error messages, any output that enumerates items |
| Goroutine lifecycle | `ai/rules/goroutine-lifecycle.md` | Any `go func()`, any `OnStarted` callback, any worker pattern |
| File size | `ai/rules/go-standards.md` | Modified file exceeds 1000 lines |
| Pipe operators | `ai/rules/cli.md` | Any command producing output |
| Registered names | `ai/rules/plugins.md` "Renaming" section | Changing any plugin/subsystem/dispatch/log name |
| Same-process-only calls | `ai/rules/plugins.md` | Any plugin calling another `internal/component/*` package's exported function directly, not through DirectBridge/DispatchCommand |
| Sibling call sites | `ai/rules/architecture.md` "Sibling Call-Site Audit" | Adding a guard/fallback/retry to ANY call site |
| Buffer allocation / memory | `ai/rules/performance.md`, `ai/rules/performance.md`, `ai/rules/performance.md` | Any allocation, pool use, string building, or wire encoding |
| Map keys / dispatch keys | `ai/rules/go-standards.md` | Any new `map[string]` or string-based dispatch on a hot path |
| JSON keys | `ai/rules/cli.md` | Any new JSON output |
| Env vars | `ai/rules/go-standards.md` env section | Any env var access |
| Error handling | `ai/rules/go-standards.md` forbidden section | Any `_` on error return |
| Error / failure message content | `ai/rules/cli.md` | Any error, log line, or failure output: name the subject + offending value + corrective action; greppable phrase; fail closed |
| Discoverability | `ai/rules/repo-maintenance.md` | Any feature, tool, self-check, verification gate, test infrastructure, or agent workflow |
| Which model runs this phase | `ai/rules/planning.md` | Review runs on Opus 5. Implementation carries no model requirement |
| Two rules point in different directions | `ai/rules/rule-precedence.md` | The ladder: irreversible action > outside-facing correctness > scope integrity > phase boundaries > autonomy |
| How much work is already in flight | `scripts/dev/spec-session.sh wip` | In-progress specs, stalest first, against `ZE_SPEC_WIP_CAP` (default 12); `claim` refuses a new `ready` spec over the cap |
| Who executes this phase (main thread vs subagent) | `ai/rules/planning.md` | Any spec work: the main thread supervises, each phase runs in a subagent through its `ze-*` skill |

## Dev Tools

| Tool | Location | Purpose |
|------|----------|---------|
| `commit_helper.py` | `scripts/dev/` | Generate commit message files and executable commit scripts that Claude runs itself, one script per prepared commit at the path the `script=` line prints. Reuses `tmp/commit-session-id`, rejects ignored/generated paths, uses `git commit -F`, and gates on the verify status and the review gate. |
| `session-scratch.sh` | `scripts/dev/` | Print (and create) this session's private scratch dir, the `scratch/` subdirectory of `tmp/session/<YYYY-MM-DD>-<session-id>/`. Use for ad-hoc command output instead of fixed names at the `tmp/` root, which collide between concurrent sessions. Nothing under `tmp/session/` is ever removed automatically; `make ze-clean-sessions BEFORE=<YYYY-MM-DD>` is the operator's cleanup. See `ai/rules/commands.md`. |
| `make ze-path` | `mk/session.mk` | Print THIS session's `ze` binary path. Under an AI session every canonical binary is built under its BARE name into this session's own directory, `tmp/session/<YYYY-MM-DD>-<session-id>/bin/` (`ZE_BIN_DIR`), so a sibling session's `make ze` cannot overwrite the binary you are testing against; off-session the path is the plain `bin/ze` it always was. **Never hardcode `bin/ze`** in a command, script, or doc -- use `$(make ze-path)`. Test binaries go to a private `bin/` subdir of a throwaway directory under `$(ZE_SCRATCH_DIR)`, because `.ci` tests exec them by bare name. See `ai/rules/commands.md` "Your Binaries Live In This Session's Directory". |
| `journal.py` | `scripts/dev/` | Read `plan/journal/*.md` at git HEAD and report problem classes with 2+ rows. Each class file is one problem pattern, each row one occurrence. `make ze-journal` prints the class, its row count, and the date span. Folded into `make ze-doc-test`. |
| `digest_check.py` | `scripts/dev/` | Validate the `file:line` anchors in `ai/digests/*.md`: each resolves to a real file (subsystem-relative via the digest's `<!-- digest-base: -->` header) and an in-range line. Keeps the hand-maintained flow digests honest as code moves. Gate: `make ze-digest-check`, folded into `make ze-doc-test`. |
| `check_doc_links.py` | `scripts/dev/` | Five checks over the path references in the tree. (1) Every backticked path and markdown link in `ai/`, `.claude/rules/` and the `plan/` meta documents resolves. `plan/learned/DESIGN-HISTORY.md` is scanned, not exempt. (2) Every `// Design:` target resolves. (3) Every backticked `*.sh` filename and `c_*`/`check_*` function name in the hook-describing documents names something in the tree. Those names resolve against top-level `def` names, not against the `CHECKS` registry. (4) Every `doc-links: ignore` marker states a reason: `<!-- doc-links: ignore (why) -->`. Checks 1, 3 and 5 skip a line only for a marker that states one, and check 4 sweeps every TRACKED file, so a marker outside the walked corpus is audited rather than left as decoration. (5) Every path reference in every OTHER tracked file resolves, under check 1's grammar. `vendor/` and `third_party/` are excluded: their references point into another repository. `plan/handover/` is excluded too: it records the tree as it was. `scripts/dev/doc_citation_baseline.txt` grandfathers the pairs that predate check 5. `check_baseline_growth` refuses every pair the baseline holds and HEAD does not, so that file only shrinks. It compares the pairs, never their number: a repair and a new dead citation in one commit leave the total unmoved and are still refused. Checks 4 and 5 share one read of each tracked file. Gate: `make ze-doc-links`, its own `ze-verify` stage. `make ze-doc-test` runs no part of it; `ze-regen-check`'s recipe ends with the `--md-only` subset, which drops check 2 and keeps 1, 3, 4 and 5. |
| `spec-closure-check.py` | `scripts/dev/` | Detect specs implemented but never closed. `--list` shows the backlog in two tiers (high-confidence vs NEEDS VERIFICATION); `--spec <s>` exits 3 only for high-confidence (a committed `plan/journal/<class>.md` row whose Spec cell equals the spec stem, spec `in-progress`, not an umbrella). Backs the Stop-hook closure gate. See `ai/rules/planning.md` "Closure Enforcement". |
| `ci_observer_recover_check.py` | `scripts/dev/` | Guard the `.ci` observer fail-closed property: no engine-touching call may sit inside a **recovering** `except` handler, because an exception unwinding through the observer's `finally` shutdown lands its sentinel on a stderr nothing relays -- the ordering defect that let real RPC errors pass as green in 332 of 346 observer files. The flagged call set is DERIVED (transitive closure over `ze_api.py` to `_call_engine`/`wait_for_shutdown`), so a new engine-touching helper is covered the day it is written. Its Go test runs the real scan and asserts zero, so `make ze-unit-test` enforces it with no make target to forget. See `ai/rules/testing.md` "Observer-Exit Antipattern". |
| `docker_exec_checked.py` | `scripts/dev/` | Refuse the next test-harness call site that reads a fail-open return value without testing it. `docker_exec_quiet` (`test/interop/interop.py`) returns `""` on ANY non-zero exit, so `"DIS" in ""` answers False and a command that FAILED reads as a passing assertion over nothing. The flagged set is DERIVED to a fixpoint (a function that `return`s a fail-open call is itself fail-open), which is what covers the WRAPPERS scenarios actually call: 20 functions, 255 call sites. A bare-statement call is discarded, not flagged. Opt out with `# fail-open-ok: <reason>`; a bare marker with no reason does not count. The floor in `test/health/docker-exec-baseline.json` may only go DOWN. Gate: `make ze-docker-exec-check`, routed onto the verify path when a `test/**/*.py` scenario or lab changes, and re-run by `TestRepoRatchet` in `scripts/dev/docker_exec_checked_test.py`. |
| `ste_check.py` | `scripts/dev/` | Review the repository's writing against ASD-STE100 Simplified Technical English, rule one (`ai/rules/writing.md`). It finds the six banned habits: synonym rotation, hedging, frozen verbs, marketing adjectives, run-ons, and phrasal verbs. Surfaces are Markdown, Go comments, YANG descriptions, and piped text. `make ze-ste-review` prints every finding with its `file:line` and the replacement. `make ze-ste-review-changed` limits that to changed files. `make ze-ste-check` compares each changed file against its own HEAD version and fails when a habit grew, printing the file and only the new findings. The BLOCKING form is `ste_problems` in `commit_helper.py`, scoped to the files of one commit. No baseline file exists to re-bless, so the one way to green is to rewrite the prose. |
| `netlab_render_check.py` | `scripts/dev/` | Render `contrib/netlab/` with a real netlab and compare the result against `contrib/netlab/golden/`, then run `ze config validate` on each golden file. `contrib/netlab/` mirrors the netlab daemon integration (the daemon YAML, the Jinja2 templates that emit ze config, one reference topology), so it drifts the first time ze config syntax changes and nothing renders it. It builds a scratch lab from the mirror and never writes to the operator's netlab install. Gate: `make ze-netlab-render-check` (`mk/test-integration.mk`), outside `ze-verify` because it needs netlab installed. A missing netlab is an ERROR exit, never a skip. `ARGS=--update` rewrites the golden files. |
| `go_extract.go` | `scripts/dev/` | Move Go symbols between files |
| `replace.py` | `scripts/dev/` | Bulk find-and-replace with diff preview (run without `--apply` to review, then `--apply` to write). Supports `--regex` and `--all`. |
| `yang_move.py` | `scripts/dev/` | Format-aware YANG path refactoring. When YANG nodes move, updates slash paths, set commands, brace blocks, and GetContainer chains across the codebase. `remove <seg> --under <path>`, `rename <old> <new> --under <path>`, `move <src> <dst>`. Preview by default, `--apply` to write. Run `--test` for self-tests. |
| `stress-repro.py` | `scripts/dev/` | Reproduce load-dependent / flaky-in-full-verify test failures WITHOUT the full suite: CPU+GC burners oversubscribe every core while many concurrent `ze-test <suite>` runs loop, capturing the first failure's untruncated output (`GOTRACEBACK=all`; optional `--race`). `<suite>` and `--test` are whitespace-split, so `"bgp plugin" --test 97` works. Add `--any-failure` for assertion flakes (a missed `expect=` never carries a crash signature, so the default crash-only mode reports "not reproduced" and discards the evidence). Honours `ZE_TEST_NO_BUILD`, so rebuild this session's `ze` (`make ze`, path from `make ze-path`) after changing daemon source or the verdict is against a stale binary. Writes `tmp/stress-repro/<slug>-<ts>.log`. See `ai/rules/testing.md`. |
| `rename_module_path.py` | `scripts/dev/` | Rename the Go module path repo-wide: every import, `go.mod`/`replace` target, goimports `local-prefixes`, build config, AND the directories whose names mirror the module path (`gokrazy/ze/builddir/<module>/`). Re-sorts import groups with `goimports -format-only -local <new>` because the module-local group is keyed by the module path. REFUSES `*.pb.go` (the rawDesc encodes `go_package` with a varint length prefix, so a textual rewrite corrupts it silently) and points at `make ze-proto-gen`; reports, never silently drops, occurrences that are not module paths (an absolute checkout path) and leftover references to the old HOST (hosting URLs, history). Re-stamps the `rfc/audit/*.json` verdict fingerprints the rename staled (they hash the whole enclosing test file, so one rewritten import line invalidates a verdict about untouched assertions), re-sealing ONLY where it can prove per file that HEAD's content under the rename equals the current content, and refusing anything else. Dry-run by default, `--apply` to execute; does no git operations. |
| `make ze-proto-gen` | `Makefile` | Regenerate `api/proto/*.pb.go` from `api/proto/ze.proto` using the vendored `protoc-gen-go` / `protoc-gen-go-grpc` (versions pinned by `go.mod`; needs `protoc` on PATH). Required after a module-path rename. |
| `bundle-html.py` | `gh-pages: presentations/tools/` | Inline local images, slides.md, and embeds into HTML as a self-contained file. Output: `<name>-inlined.html`. Accepts multiple files. |
| `make ze-verify-wiring-docs` | `mk/inventory.mk` | Changed-file-aware wiring, documentation, command, and inventory gate used by `make ze-verify`. |
| `go run ./scripts/status/verify_run.go ze-verify` | `scripts/status/verify_run.go` | Verify protocol runner used by `make ze-verify`. Writes `tmp/ze-verify.log`, per-stage logs, compact failure indexes, and `tmp/ze-verify.status`. |
| `verify-status.sh` | `scripts/dev/` | Checks whether the current tree is byte-identical to the last passing `ze-verify` run. Commit preparation must treat FRESH as authoritative and skip rerunning verify. |
| `make ze-doc-test` | `mk/inventory.mk` | Documentation drift, YANG command handler contracts, stale source anchors, the symbols those anchors name (`check_anchor_symbols` in `scripts/dev/code_to_docs.py`, which resolves each token after an anchor's `--` against the anchored file's own top-level declarations), the six rule-corpus gates, the discovery indexes, the problem journal, and the `ai/digests/` anchors. It does NOT run `check_doc_links.py`: that is `make ze-doc-links`, its own `ze-verify` stage. |
| `make ze-rfc-check` | `scripts/dev/rfc_requirements.py` | Gate: every MUST-level requirement of an enrolled RFC (`rfc/enrolled.txt`) is bound to a positive AND a negative test, or carries a reasoned annotation. It also ratchets against HEAD (`ai/rules/rfc-compliance.md`, "the five ratchets"). Enrolment cannot shrink. A requirement cannot lose a polarity it had. A NEW summary with gated MUSTs must be enrolled. An extraction sign-off cannot disappear. Enrolling a stem not enrolled at HEAD requires `rfc/extraction/<stem>.json`. It validates `rfc/audit/*.json` on read: closed verdict enum, required fields, no dangling rid, no verdict claiming proof without a cited test. It ratchets the verdicts too. A `weak` or `wrong` finding cannot be deleted or silently upgraded. No verdict that existed at HEAD can vanish. |
| `make ze-rfc-index` | `scripts/dev/rfc_requirements.py` | Regenerates one file per RFC under `rfc/requirements/`, each mapping that RFC's requirements to their enforcing tests, and `ai/RFC-REQUIREMENTS.md`, the index over them. `--show <stem>` prints one of those files. The index adds the Coverage-by-RFC backlog, which names what is still owed. It adds the Extraction sign-off table, which names what is still unbounded. It adds the Audit coverage section. That section names what is audited. It names what is PROVEN, which means a fresh `enforced` verdict. It names every requirement whose verdict says otherwise. Writes those two outputs and nothing else, and deletes a `rfc/requirements/` file whose RFC no longer renders -- never `rfc/audit/` (see `make ze-rfc-reseal`). |
| `make ze-rfc-extract STEM=<stem>` | `scripts/dev/rfc_requirements.py` | Writes an UNCLASSIFIED extraction skeleton to `rfc/extraction/<stem>.json`: every normative site of the RFC's own text and every section, each disposition null. A reviewer classifies them by hand; an unclassified site FAILS `ze-rfc-check`, so generating skeletons makes the gate redder, never greener. Required before enrolling a new stem. Contract: `rfc/extraction/README.md`. |
| `make ze-rfc-extraction-status` | `scripts/dev/rfc_requirements.py` | JSON envelope of the extraction counts the drain quota consumes: signed and enrolled counts, the per-register split (`rfc2119`/`prose`/`manual-walk`), and the unsigned backlog list. |
| `make ze-rfc-reseal` | `scripts/dev/rfc_requirements.py` | Re-stamps the `rfc/audit/*.json` verdicts that `ze-rfc-check` reports as SHIFTED. The tagged unit is the enclosing top-level function of each tagged test. SHIFTED means that unit is byte-identical. Only the file around it moved, through a line shift, a sibling test, or an import rewrite. Nothing was re-judged, so no human re-read is owed. It rewrites the `tests` fingerprints and nothing else. A verdict whose unit, cited producer code, or requirement text moved is REFUSED. That verdict stays stale for `/ze-rfc-audit`. **The only thing that writes `rfc/audit/` without a human editing it** -- `ze-rfc-check` is read-only and `ze-rfc-index` touches the ledger alone, deliberately, so re-stamping cannot happen as a side effect of unrelated work (`ai/skills/ze-rfc-audit.md`). Run `make ze-rfc-index` afterwards. |
| `make ze-inventory` / `make ze-inventory-json` | `mk/inventory.mk` | Registry-backed plugin, command, YANG, and test inventory. |
| `make ze-command-list` / `make ze-command-list-json` | `mk/inventory.mk` | Live command inventory generated from registered handlers and schemas. |
| `make ze-cli-grammar-check` / `-json` | `mk/inventory.mk` | CLI grammar gate: every built-in command obeys the verb-first rules R1-R9 (`ai/rules/cli.md`; R9 = compound-vs-namespace split) and no `.yang` carries a `--flag`. In `make ze-verify`. |
| `make ze-doc-index` | `mk/inventory.mk` | Regenerate `ai/CODE-TO-DOCS.md`, the source-to-document reverse index. |
| `make ze-rules-render` / `make ze-rules-render-check` | `scripts/dev/rules_points.py` | Render `ai/rules/<rule>.md` from `ai/rules/points/<rule>/`. One instruction is one file at `ai/rules/points/<rule>/<section>/<slug>.md`, and its PATH is its id. A `##` heading is the section DIRECTORY; a `###` or `####` heading stays a point inside it, so the depth is fixed at two. The rendered rule is what every agent Reads. An edit to it is refused by the `rendered-rules` check in `.claude/hooks/pretool-writeedit.py`. Edit the point, then render. `--check` reaches the same verdict but writes nothing. It runs inside `make ze-doc-test` and `make ze-regen-check-readonly`. The render FAILS on a point the manifest does not list. It fails the same way on a listed slug with no file, and on a duplicate or unsafe slug. Each one silently drops an instruction. |
| `make ze-rules-points-roundtrip` | `scripts/dev/rules_points.py` | Split every rendered rule into a scratch directory and render it back, then compare bytes. A lossy split is silent instruction loss. This reports any rule whose round trip is not byte-identical, and writes nothing under `ai/rules/`. It runs inside `make ze-doc-test`. The render check does not subsume it: `render --check` asks whether the rendered rule matches the points, and this asks whether the rendered rule still splits back into points. |
| `make ze-rules-gate-map` | `scripts/dev/rules_points.py` | Which rule point each hook check enforces. The `# ze point: <rule>/<section>/<slug>` comments in the PreToolUse dispatchers are joined against `ai/rules/points/`, and five sets come out. The dispatchers it reads are derived from the PreToolUse entries in `.claude/settings.json`. A fourth one joins by being wired up, and one that no entry runs is reported. **Gated** and **ungated** are measurements and exit 0: an ungated point is a rule no machine enforces yet. **Dangling** exits non-zero, and it is what a reworded rule looks like from under its gate. **Regressed** exits non-zero too. The gated set is monotonic against HEAD, so a point that carried a binding at HEAD and carries none now fails. That is the one route from gated to ungated that leaves every other gate green. A check that enforces no written point declares `# ze point: none -- <why>`. The reason is required, so "nobody bound this" and "there is nothing to bind" stay apart. The ungated denominator excludes the `heading` and `fence` kinds, which are structural. Runs inside `make ze-doc-test` with `--quiet`. |
| `make ze-rules-condensed` | `scripts/dev/rules_condensed.py` | Regenerate both rule-digest artifacts from one parse: `ai/rules/TRIGGERS.md` (one routing line per rule, loaded every session) and `ai/rules/CORE.md` (always-on directives, membership derived from the rung 1/2 ladder in `ai/rules/rule-precedence.md`). |
| `make ze-token-economy` | `scripts/dev/token_economy.py` | Where this repository's agent sessions spend their tokens, from the machine-local Claude Code transcript store (`~/.claude/projects/<slug>/<session>.jsonl` plus `<session>/subagents/agent-*.jsonl`). Reports API calls (main against subagent), cache-read/cache-write/output, mean and max context per call, a histogram saying at which context size the tokens were fed, a capped-context counterfactual (`ZE_CONTEXT_CAP`, default 200,000), and a per-phase split keyed on each spawn's `.meta.json` description. **One API call is written as several assistant records repeating the same usage**, so every figure is deduped by `message.id`; counting records doubles the report. The store is machine-local and read-only: a checkout with no transcripts prints the path it looked for and exits 0 rather than reporting zeros. Token counts only, never money. |
| `make ze-rules-payload` | `scripts/dev/rules_condensed.py` | What a session actually loads: `ai/INSTRUCTIONS.md` + `TRIGGERS.md` + `CORE.md`, in tokens, against the 40,000 budget and the digest it replaces. |
| `make ze-rules-router-report` / `-json` | `scripts/dev/rules_router.py` | Trigger-routing coverage over every past task description in `plan/`. It reports which rules the trigger index surfaces per task, and which BLOCKING rules no task surfaces at all. The generator derives the core from that second set, so a rule named here is already always-on. |
| `make ze-ai-sync` | `scripts/dev/skill_sync.sh` | Sync canonical `ai/skills/*.md` to `.claude/skills/`, `.codex/skills/`, and `.agents/skills/`; also regenerates `CLAUDE.md` and `AGENTS.md` from `ai/INSTRUCTIONS.md`. |
| `make ze-spec-status` / `make ze-spec-status-json` | `mk/inventory.mk` | Spec progress overview (committed backlog vs skeleton idea capture, stale-skeleton flags) plus a non-blocking completed-but-not-closed advisory. |
| `make ze-spec-citation-check` | `mk/inventory.mk` | Spec citation freshness: fails on a `plan/spec-*.md` citing a sibling spec absent on disk (grandfathered via `plan/.citation-baseline`); WARNs on `path:line` line-token drift. Runs on the verify path when a `plan/` file changes. A closure feeds it: the author who removes a spec clears every sibling that cites it, inside commit A (`ai/rules/planning.md`, "Spec Closure"). |
| `make ze-mutation-test` | `mk/test-mutation.mk` | Mutation testing via gomu on all non-excluded packages (advisory, not gating). Vendored, no install needed. |
| `make ze-mutation-changed` | `mk/test-mutation.mk` | Incremental mutation testing on changed files only. |
| `make ze-mutation-report` | `mk/test-mutation.mk` | Mutation testing with HTML report output to `tmp/mutation-report.html`. |
| `make ze-test-health` | `scripts/dev/testing_health.py` | Regenerates `docs/features/test-health.md` and `test/health/latest.json`: whether a regression would be caught, not how many tests exist. Read it before claiming the suite is healthy. |
| `make ze-test-health-record` | `scripts/dev/testing_health.py` | Appends one KPI sample to `test/health/history.ndjson` (committed), then regenerates the page so trends stay in step. |
| `make ze-test-sensitivity-check` | `scripts/checks/inert_tests.go` | Ratchets tests that cannot fail and test files no `go test` target builds. Stage 10 of `ze-verify`, both modes. |
| `make ze-web-golden-check` / `make ze-web-golden-update` | `Makefile` | Golden-output gate over every web and looking-glass template AND every route. It renders each one and compares the bytes against the fixtures in `internal/component/web/testdata/golden/` and `internal/component/lg/testdata/golden/`. The HTML suites assert with `strings.Contains`, which cannot see a byte change that keeps the asserted substring. This gate is what proves the rendered output is unchanged. The capture plan is `webGoldenSpec` and `lgGoldenSpec`, in each package's `golden_test.go`. The check fails when that plan and the embedded FS disagree, so a new template file fails until it is captured. Both packages share one harness, `internal/test/golden`. The HANDLER capture is the second, and it proves what the first cannot. `TestWebHandlerGoldenOutput` and `TestLGHandlerGoldenOutput` issue a real HTTP request to every route. Each response lands in `testdata/handler/`. The template capture executes a parsed template against data the test writes. It therefore bypasses `RenderFragment`, `RenderConfigToHTML`, `RenderField` and `RenderL2TPTemplate`. It never sees the view model a handler builds. The route list is derived, never typed. It comes from `RegisteredWebRoutes()`, from the literal patterns in `cmd/ze/hub/service_web.go` and `internal/component/lg/server.go`, and from `sections()` for the workbench pages. A route no case reaches fails the check by name. The MARKUP capture is the third, and it covers what neither of the others reaches. `TestWebMarkupGoldenOutput` renders the builders that write HTML in Go, `buildHostHardwareHTML` among them, over input the test fixes. No template holds that markup, and the handler capture normalizes it, because the live panel follows what `host.Detect` finds on the machine. Every capture also fails on a fixture on disk that no case writes. `-check` refuses to pass over zero tests (`require-go-test` in the `Makefile`). `go test -run` exits 0 over an empty selection, so a renamed test would otherwise turn the gate green. `-update` recaptures a DELIBERATE markup change. Read its diff: every byte it rewrites is a byte an operator receives. |
| `make ze-templ-port-check REF=<sha>` | `Makefile` | Compares every captured fixture against the bytes it held at REF, through `golden.NormalizeHTML`. It answers what `ze-web-golden-check` cannot. That gate proves the fixtures ARE the current render. Once a fixture is recaptured, it compares a port against itself. Five differences fold. They are whitespace layout, doctype case, the attribute-quote delimiter, a character reference, and a bare `&` against `&amp;`. Each one is an encoding a browser decodes the same way. Nothing else folds. Whitespace inside `<pre>` and `<textarea>` is content. A response is split first, so its status and headers are compared byte for byte. Only a `text/html` body is normalized. A unit that existed at REF and has no fixture today is a finding. That is what stops a failing unit being deleted instead of repaired. So is a declared difference that no longer differs. `REF` is empty by default, and an empty `REF` means `golden.PrePortRef`. `TestWebTemplPortFidelity` and `TestLGTemplPortFidelity` are neither gated nor skipped, so both also run under `ze-unit-test`. |
| `make ze-templ-generate-check` | `Makefile` | Refuses a `*_templ.go` that its `.templ` source no longer produces. It runs `templ generate -check -keep-orphaned-files`, which reports a stale file rather than rewriting it. `make generate` writes the same output for real. **`-keep-orphaned-files` is what makes the check read-only.** Without it templ deletes an orphaned `*_templ.go` and exits 0. It does that in `-check` mode as much as in generate mode. `HandleEvent` (`vendor/github.com/a-h/templ/cmd/templ/generatecmd/eventhandler.go`) calls `os.Remove` before any writer is consulted. Only `keepOrphanedFiles` gates that call. The same branch also returns before the check writer sees the file. So with the flag templ says nothing about the orphan. `scripts/dev/templ_orphan_check.py` is the only report of one, through the prerequisite target `ze-templ-orphan-check`. `make generate` does NOT carry that prerequisite. Deleting a `*_templ.go` whose source is gone is what regeneration means, and a fresh checkout catches it through `ze-templ-orphan-check`, because nothing runs `ze-regen-check`. The walk is scoped to `internal/`, where every renderer in ze lives. The orphan check also fails a `.templ` outside it. A repo-root walk would descend into `gokrazy/` and `tmp/`, which hold module and build caches. `-path internal` is load-bearing for file CONTENT too. `FSEventHandler.generate` builds each filename with `filepath.Rel` against the walk root. A bare `templ generate` therefore rewrites every `*_templ.go` and reds the gate, and a bare run is what an editor-on-save integration makes. Runs inside `ze-regen-check-readonly`, and `make ze-verify-wiring-docs` routes a changed `.templ` to it. |
| `internal/test/markupcheck` | `markupcheck.go`, `inline.go`, `assets.go` | Three static scans over a package that renders markup. `TestNoGoFileBuildsMarkup`, `TestTemplatesAvoidInlineScriptAndStyle` and `TestTemplAssetsResolve` call them, in `internal/component/web` and in `internal/component/lg`. They run under `ze-unit-test` and under `make ze-test-pkg`, with no target of their own. `AssertNoMarkup` reads Go string LITERALS, so a tag in a comment is not a finding. It reads the FORM of a tag, so `usage: set <leaf> <value>` is not one either. It knows HTML's void elements, because nothing else tells `strings.Join(x, "<br>")` from a placeholder. `command` and `source` are left out of that list, since ze writes both as CLI usage text. `AssertNoInlineScriptOrStyle` reads the `.templ` sources for the three things `'self'` refuses at the browser. They are an inline `<script>`, an inline `style=`, and an `on*` or `hx-on` attribute. It found two dead `onclick` handlers in `lg/route_table.templ`. The web package's copy of the same scan never saw them, because each scan walks its own directory. `AssertAssetsResolve` resolves each `src` and `href` against the SERVED sub-FS. A renamed asset is a red test rather than a 404 only a browser sees. Each of the three takes a floor, and the exemption table takes an exact size. A walk that reads nothing reports nothing, and widening an exemption is otherwise the cheapest route from red to green. |
| `internal/test/templcheck` | `templcheck.go` | The sibling guard, over what a templ component TAKES rather than what a Go file builds. `AssertTyped` is called by `TestWebViewDataIsTyped` and `TestLGViewDataIsTyped`. It refuses a `map[string]any`, a named map type, a slice or pointer to one, a bare `any`, and a struct field wrapping any of them. It is fail-closed, so a type it cannot resolve is reported. `html/template` renders a missing MAP key as empty output and reports success. An untyped component is therefore a blank panel nothing detects. The component count each caller passes is the vacuity floor. |
| `make ze-relax-census` | `scripts/dev/relax-census.py` | Holds the `test-relax:` token count under the ceiling in `test/relax-ceiling.txt`. A token is a test that stopped proving something, excused by the agent that weakened it; 751 had accumulated unread at HEAD by 2026-08-10 (`TEST-RELAX-AUDIT.md`). Counted at HEAD, because a shared checkout's working tree moves under whoever reads it. `--list` prints every token with its whole justification, `--by-area` the per-area counts, `--lower` moves the ceiling down and never up. Runs in `ze-verify`, both modes. |
| `make ze-tracked-build-check` | `scripts/checks/tracked_build.go` | Compiles the tree GIT HOLDS, not the working tree every other gate reads: catches a consumer committed without its producer. `REV=<sha>` judges any commit, `ARGS=--keep` keeps the extracted tree. Runs in `ze-verify`, both modes. |
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

Structural decisions, patterns, and gotchas. Recurrence data: `plan/journal/` (`make ze-journal`).
Aggregates: `plan/learned/DESIGN-HISTORY.md`, `plan/learned/HOOK-FRICTION.md`, `plan/learned/RECURRING-PATTERNS.md`.

## Architecture Docs

| Area | Doc |
|------|-----|
| **Core Design** | `docs/architecture/core-design.md` **(START HERE)** |
| Architecture index | `docs/architecture/README.md` |
| **System Architecture** | `docs/architecture/system-architecture.md` |
| Design references | `docs/architecture/design-refs-map.md` |
| **Overview** | `docs/architecture/overview.md` |
| **Hub Architecture** | `docs/architecture/hub-architecture.md` |
| MCP overview | `docs/architecture/mcp/overview.md` |
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
| Performance round 3 | `docs/architecture/perf-round-3.md` |
| Memory pools | `docs/architecture/pool-architecture.md` |
| Pool review | `docs/architecture/pool-architecture-review.md` |
| Memory lifetimes | `docs/architecture/memory/lifetime-contracts.md` |
| Zero-copy | `docs/architecture/encoding-context.md` |
| RIB transition | `docs/architecture/rib-transition.md` |
| RIB storage | `docs/architecture/plugin/rib-storage-design.md` |
| Plugin relationships | `docs/architecture/plugin/plugin-relationships.md` |
| Route types | `docs/architecture/route-types.md` |
| MRT | `docs/architecture/mrt.md` |
| Route selection | `docs/architecture/route-selection.md` |
| FSM | `docs/architecture/behavior/fsm.md` |
| FSM Active | `docs/architecture/behavior/fsm-active.md` |
| FSM Connect | `docs/architecture/behavior/fsm-connect.md` |
| FSM Established | `docs/architecture/behavior/fsm-established.md` |
| FSM Idle | `docs/architecture/behavior/fsm-idle.md` |
| FSM OpenConfirm | `docs/architecture/behavior/fsm-open-confirm.md` |
| FSM OpenSent | `docs/architecture/behavior/fsm-open-sent.md` |
| Peer lifecycle | `docs/architecture/behavior/peer-lifecycle.md` |
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
| Deprecated config options | `docs/architecture/config/deprecated-options.md` |
| Config transactions | `docs/architecture/config/transaction-protocol.md` |
| Config environment | `docs/architecture/config/environment.md` |
| Environment block | `docs/architecture/config/environment-block.md` |
| Config tokenizer | `docs/architecture/config/tokenizer.md` |
| YANG design | `docs/architecture/config/yang-config-design.md` |
| ExaBGP syntax | `docs/architecture/config/exabgp-syntax.md` |
| VyOS research | `docs/architecture/config/vyos-research.md` |
| Plugin modes | `docs/architecture/cli/plugin-modes.md` |
| CLI color system | `docs/architecture/cli/color-system.md` |
| Plugin testing | `docs/architecture/debugging/plugin-testing.md` |
| Edge: ASN4 | `docs/architecture/edge-cases/as4.md` |
| Edge: Confederation AS_PATH loop | `docs/architecture/edge-cases/confederation-aspath-loop.md` |
| Edge: ADD-PATH | `docs/architecture/edge-cases/addpath.md` |
| Edge: Extended msg | `docs/architecture/edge-cases/extended-message.md` |
| Route metadata | `docs/architecture/meta/README.md` |
| Role metadata | `docs/architecture/meta/role.md` |
| Forward pool | `docs/architecture/forward-congestion-pool.md` |
| Congestion industry | `docs/architecture/congestion-industry.md` |
| Subsystem wiring | `docs/architecture/subsystem-wiring.md` |
| Plugin mgr wiring | `docs/architecture/plugin-manager-wiring.md` |
| Command ownership | `docs/architecture/command-ownership.md` |
| Hub API commands | `docs/architecture/hub-api-commands.md` |
| RFC MAY decisions | `docs/architecture/rfc-may-decisions.md` |
| Architecture decisions | `docs/architecture/decisions/README.md` |
| Decision: pull-model metrics | `docs/architecture/decisions/001-pull-model-metrics.md` |
| ZeFS format | `docs/architecture/zefs-format.md` |
| Fleet config | `docs/architecture/fleet-config.md` |
| Web interface | `docs/architecture/web-interface.md` |
| Web components | `docs/architecture/web-components.md` |
| Chaos dashboard | `docs/architecture/chaos-web-dashboard.md` |
| CI format | `docs/architecture/testing/ci-format.md` |
| Interop testing | `docs/architecture/testing/interop.md` |
| QEMU integration | `docs/architecture/testing/qemu-integration.md` |
| Test runner | `docs/architecture/testing/runner-architecture.md` |
| ExaBGP mapping | `docs/exabgp/exabgp-code-map.md` |
| ExaBGP compat | `docs/exabgp/exabgp-differences.md` |

## Keyword → Architecture Doc

| Keywords | Docs |
|----------|------|
| buffer, iterator, parse, wire | `core-design.md`, `buffer-architecture.md`, `ai/rules/performance.md` |
| encode, Pack, WriteTo, alloc | `ai/rules/performance.md`, `buffer-architecture.md` |
| UPDATE, message, build, route | `core-design.md`, `update-building.md`, `encoding-context.md` |
| attribute, AS_PATH, NEXT_HOP, MED | `core-design.md`, `wire/attributes.md`, `update-building.md` |
| community, ext community, large community | `wire/attributes.md` |
| NLRI, prefix, MP_REACH, MP_UNREACH | `core-design.md`, `wire/nlri.md` |
| multiprotocol, AFI, SAFI, new family, new SAFI | `ai/patterns/bgp-family.md`, `wire/nlri.md`, `wire/capabilities.md` |
| capability, OPEN, negotiate | `wire/capabilities.md` |
| pool, memory, dedup, zero-copy, lifecycle | `ai/rules/performance.md`, `core-design.md`, `pool-architecture.md`, `encoding-context.md` |
| textbuf, string building, AppendTo, alloc-free | `ai/rules/performance.md`, `ai/rules/performance.md`, `internal/core/textbuf/` |
| error message, actionable error, corrective action, remediation, fail closed | `ai/rules/cli.md`, `ai/rules/protocol.md`, `ai/rules/evidence.md` |
| guard, fail open, fail closed, silent no-op, zero value, valid-looking zero, bare map read, permissive default, inert constraint, dead guard | `ai/rules/evidence.md`, `ai/digests/aaa-auth.md` (the authorizer chokepoint and its fail-open bootstrap window) |
| sync.Pool, buffer pool, ring buffer, peerPool | `ai/rules/performance.md`, `forward-congestion-pool.md` |
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
| templ, `.templ`, generated markup, view-model type safety | `make ze-templ-generate-check` (Dev Tools above), `tools.go`, `internal/component/web/templ_typesafety_test.go` |
| golden fixture, rendered markup, template bytes, byte-for-byte HTML | `make ze-web-golden-check` (Dev Tools above), `internal/test/golden`, `web-interface.md` |
| rendering-engine port, pre-port bytes, normalized HTML comparison | `make ze-templ-port-check REF=<sha>` (Dev Tools above), `internal/test/golden/portcheck.go`, `internal/test/golden/normalize.go` |
| HTML in a Go string, inline script, inline style, `onclick`, `hx-on`, CSP `'self'`, asset 404, `map[string]any` in a component | `internal/test/markupcheck`, `internal/test/templcheck` (Dev Tools above), `ai/rules/architecture.md` ("Server-Rendered Markup") |
| subsystem, wiring, plugin manager | `subsystem-wiring.md`, `plugin-manager-wiring.md` |
| bridge, direct call, request/response, sync handler | `core-design.md` (section 9), `ai/rules/plugins.md` (DirectBridge), `plan/learned/DESIGN-HISTORY.md` "Plugin system: architecture" |
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
| RFC requirement coverage, RFC MUST tests, rfc-requirements, RFC requirement tag, ze-rfc-check, ze-rfc-index | `make ze-rfc-check`, `rfc/requirements/<stem>.md` (one RFC's rows), `ai/RFC-REQUIREMENTS.md` (the index), `ai/skills/ze-rfc.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/functional-tests.md` (RFC Requirement Tags) |
| RFC extraction sign-off, extraction completeness, what the summary MISSED, unextracted obligation, normative site, extraction register, rfc2119/prose/manual-walk, drain budget, ze-rfc-extract, ze-rfc-extraction-status | `rfc/extraction/README.md`, `make ze-rfc-extract`, `make ze-rfc-extraction-status`, `rfc/drain-budget.txt`, `ai/rules/rfc-compliance.md` (Extraction Completeness, the five ratchets), `ai/RFC-REQUIREMENTS.md` (Extraction sign-off) |
| RFC audit verdict, rfc/audit schema, enforced/weak/wrong/unimplemented/not-applicable, no_code_path, upgrade_reason, units map, code map, SHIFTED verdict, stale audit verdict, ze-rfc-reseal, audit coverage | `ai/skills/ze-rfc-audit.md` (the verdict vocabulary and the four freshness states), `make ze-rfc-reseal`, `ai/RFC-REQUIREMENTS.md` (Audit coverage), `scripts/dev/rfc_tagged_scope.py` (the one definition of "the tagged unit"), `ai/rules/rfc-compliance.md` (a verdict is never authority) |
| payload-predicate waits, sleep elimination, ci-sleep ratchet, ci-sleep justification, time.sleep comment, wait_until, dispatch_until, wait_for_event predicate, engine-step predicates (matches=/absent=/json=) | `docs/functional-tests.md` ("Payload-predicate waits"), `docs/architecture/testing/ci-format.md` ("Engine Steps"), `ai/rules/testing.md` (Observer API), `ai/rules/testing.md`, `test/scripts/ze_api.py`, `internal/test/runner/engine_steps.go` |
| poll loop, wait loop, waiting for a background command, watcher, pgrep loop, waiting for a QEMU boot, until/while + sleep blocked | `ai/rules/commands.md`, `ai/rules/repo-maintenance.md` (poll-loop), `.claude/hooks/pretool-bash.py` (`check_poll_loop`) |
| netdata, telemetry, prometheus, metrics, monitoring, collector | `docs/guide/monitoring.md`, `docs/features.md`, `ai/digests/observation-telemetry.md` |
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
| IPsec, IKE, IKEv2, SA, child SA | `ai/digests/ipsec-ike.md`, `internal/component/ike/` |
| EAP, NAT-T, MOBIKE | `ai/digests/ipsec-ike.md`, `internal/component/ike/` |
| XFRM, xfrm interface, VTI | `ai/digests/ipsec-ike.md`, `internal/component/ike/` |
| subscriber, session, PPPoE, L2TP | `ai/digests/subscriber.md`, `internal/component/l2tp/pppoe/` |
| editor, TUI, completion, headless | `internal/component/cli/`, `test/editor/`, `ai/rules/testing.md` (Editor Tests section) |
| diagnostic, doctor, health, readiness | `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/diagnostics/production-diagnostics.md`, `ai/rules/repo-maintenance.md` |
| EventBus, event, pub/sub, subscribe, emit | `pkg/ze/eventbus.go`, `ai/rules/plugins.md` (EventBus Typed Payloads), `internal/core/events/typed.go` |
| DirectBridge, bridge, direct call, typed handler | `pkg/plugin/rpc/bridge.go`, `ai/rules/plugins.md` (DirectBridge), `plan/learned/DESIGN-HISTORY.md` "Plugin system: architecture" |
| BFD, bidirectional forwarding | `docs/architecture/bfd.md` |
| resolve, origin, pipe, pipe operator | `docs/architecture/resolve.md`, `ai/rules/cli.md` |
| MCP, model context protocol | `docs/architecture/mcp/`, `internal/component/mcp/` |
| self-update, manifest, auto-update | `docs/architecture/appliance/self-update.md`, `docs/guide/self-update.md` |
| ASPA, path verification, RTR | `docs/guide/rpki.md` (ASPA Path Verification), `internal/component/bgp/plugins/rpki/`, `docs/features/rfc-status.md` (draft-ietf-sidrops-aspa-verification) |
| BMP, monitoring protocol | `docs/guide/bmp.md`, `internal/component/bgp/plugins/bmp/`, `docs/architecture/api/commands.md` (bmp-sessions, bmp-peers) |
| docker, container, scratch, lab image | `docs/guide/docker.md` (both images), `docker/Dockerfile`, `docker/Dockerfile.lab` |
| netlab, containerlab, lab topology, daemon integration, contrib | `docs/guide/netlab.md`, `contrib/netlab/README.md`, `contrib/netlab/ze.yml` (the daemon definition), `contrib/netlab/ze/` (the Jinja2 templates), `make ze-netlab-render-check`, `make ze-docker-lab`, `docker/Dockerfile.lab` |
| chaos, fault injection, scheduler | `docs/architecture/chaos-web-dashboard.md`, `docs/guide/chaos-testing.md` |
| commit, commit script, commit message, verified commit, verify freshness, owner override, commit no test | `scripts/dev/commit_helper.py`, `scripts/dev/verify-status.sh`, `ai/rules/git-safety.md`, `ai/skills/ze-commit.md`, `ai/skills/ze-commit-check.md` |
| weekly update, Zeledon, ze-news, Discord announcement, gh-pages changes, homepage latest updates | `ai/skills/ze-weekly-update.md`, `../gh-pages/AI.md`, `../gh-pages/tools/render-index.py`, `scripts/zeledon/STYLE.md` |
| self-improvement, discoverability, discovery, new tool, self-check, verification gate | `ai/rules/repo-maintenance.md`, `ai/rules/repo-maintenance.md`, `docs/contributing/documentation-testing.md` |
| inventory, command-list, doc drift, source anchor, doc index | `ai/rules/repo-maintenance.md`, `ai/rules/writing.md`, `docs/contributing/documentation-testing.md`, `mk/inventory.mk` |
| clear, clear command, clear dns, clear interface, clear ipsec | `internal/component/resolve/cmd/` (dns), `internal/component/iface/cmd/` (interface), `internal/component/ike/cmd/` (ipsec), `internal/component/cmd/clear/` (verb root) |
| command grammar, verb-first, command alias, deprecated alias, grammar gate | `ai/rules/cli.md` (Mechanical Enforcement), `make ze-cli-grammar-check`, `docs/architecture/cli/root-namespace-grammar.md` |
| DispatchCommandArgs, typed inter-plugin dispatch, tokenizer bypass | `docs/architecture/api/process-protocol.md`, `ai/digests/plugin-transport.md`, `ai/rules/plugins.md` |
| RawMessage, double marshal, callback passthrough, SDK callback | `docs/architecture/api/process-protocol.md`, `ai/digests/api-ipc.md` |
| pipe first, pipe last, pipe metadata | `ai/rules/cli.md` (The Rule (pipes)), `docs/guide/command-reference.md`, `docs/features/formatting.md` |
| RIB dump, bounded dump, replay batching, update cursor | `docs/architecture/bgp/replay-cursor.md`, `ai/digests/rib.md` |
| plugin internal keyword, in-process plugin config | `docs/guide/plugins.md` (the `internal` keyword), `ai/patterns/plugin.md` |
| appliance auth, local admin, bootstrap auth, RBAC | `docs/guide/operator-access-rbac.md`, `ai/digests/aaa-auth.md`, `internal/component/authz/`, `internal/component/aaa/` |
| appliance, appliance iso, appliance build, appliance init | `internal/appliance/`, `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `scripts/evidence/effective-install-iso-qemu.py`, `mk/test-integration.mk` |
| Dependabot alert on vendored go.mod, gokrazy/modcache manifest, bump gokrazy init, appliance dependency bump, CVE on vendored appliance dep | `ai/rules/platform-linux.md`, `mk/gokrazy.mk` (`ze-gokrazy-deps`), `.github/dependabot.yml` |
| installer initrd QEMU evidence, R-6 fault injection, ze.mac pin, rescue console, Ventoy ISO-on-FAT, ze_installer_fault, ZE_INITRD_FAULT | `scripts/evidence/effective-install-scenarios-qemu.py`, `scripts/evidence/effective-install-ventoy-qemu.py`, `internal/install/disk/fault_linux.go`, `mk/test-integration.mk` (`ze-install-scenarios-qemu-test`, `ze-install-ventoy-qemu-test`), `docs/functional-tests.md` |
| VPP hugepage boot reservation, poll-sleep-microseconds, image.hugepages, doctor-vpp-hugepages, hugepage QEMU evidence | `internal/appliance/kernelargs.go`, `internal/component/vpp/doctor_linux.go`, `internal/component/vpp/startupconf.go`, `scripts/evidence/effective-vpp-hugepages-qemu.py`, `mk/test-integration.mk` (`ze-vpp-hugepages-qemu-test`), `docs/guide/vpp.md`, `docs/guide/appliance.md` |
| VPP semantics, linux-cp, LCP, LCP netns, lcp_itf_pair_create, default netns, binapi, lcp.ba.go, foreign system semantics | `third_party/vpp-linux-cp/` -- vendored VPP C (v25.10, read-only reference). Read this BEFORE claiming what VPP does; the generated stub `vendor/go.fd.io/govpp/binapi/lcp/lcp.ba.go` says a field exists, never what VPP does with it (`ai/rules/evidence.md`) |
| .ci test prerequisite, option=needs-path, caps=net-raw, caps=net-admin, caps=bpf, test skips instead of failing, missing modcache, ze-gokrazy-deps prerequisite | `docs/architecture/testing/ci-format.md` (Options table), `ai/rules/platform-linux.md`, `internal/test/runner/caps.go`, `internal/test/runner/needs_path.go` |
| test passes on macOS but fails in CI, works locally red in CI, unprivileged runner, 4-vCPU runner | `ai/rules/platform-linux.md` (skip-os is not a capability declaration), `ai/rules/completion.md` |
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

Per-session: `tmp/session/<YYYY-MM-DD>-<SID>/state/session-state-<spec-stem>-<SID>.md` (gitignored),
in the directory that also holds this session's `bin/` and `scratch/`. Each session gets its own file.
Session markers: `tmp/session/.session-<ID>` map sessions to specs. See `hooks/lib/state-file.sh`.
On startup, `_find_latest_state_for_spec()` finds the most recent state file for a spec from any previous session.
It walks every session directory's `state/`, and still reads the flat `tmp/session/` location that digests
written before 2026-08-10 occupy.
