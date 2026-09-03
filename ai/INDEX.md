# Ze Documentation Index

## Understand Existing Code (not change it)

Cold-start orientation. Read these to answer "what is here / where does it live"
before grepping. All three are generated from the tree (`./le discovery-index update`)
and gated fresh, so they never lie about the current code.

| Question | Read |
|----------|------|
| Am I allowed to grep or spawn an agent for this yet? | `ai/rules/documentation.md` — not until the page below is read. The page answers most questions, and an investigation names the sentence that is silent or wrong |
| What does package X do? (one line each, all ~590 packages) | `ai/PACKAGE-MAP.md` |
| Which `.go` files implement design doc Y? | `ai/DOCS-TO-CODE.md` (inverse of the per-file `// Design:` headers) |
| Which docs describe code path Z? | `ai/CODE-TO-DOCS.md` (inverse of doc `<!-- source: -->` anchors) |
| Why is the code shaped this way? | `plan/learned/DESIGN-HISTORY.md` |
| Which problems recur? | `plan/journal/` (one file per class; `./le journal report` prints classes with 2+ rows) |
| Which rule covers a topic? | `ai/rules/INDEX.md` |
| How does data flow through a subsystem? | `docs/architecture/core-design.md` (START HERE), then the subsystem doc below |
| Fast subsystem orientation (entry→exit, with `file:line`) | `ai/digests/<subsystem>.md` — living flow digests; index + list in `ai/digests/README.md`. Anchors gated by `./le digest` |

## I Want To...

| Task | Read first | Then |
|------|-----------|------|
| Find out how a surface works, before any search, grep, or agent | `ai/rules/documentation.md` | `ai/CODE-TO-DOCS.md` (file to pages), `ai/DOCS-TO-CODE.md` (page to files), the keyword map below. Investigate only what the page leaves silent or gets wrong |
| Keep a page true after a behavior change | `ai/rules/documentation.md` | The page edit lands in the same work as the code (`ai/rules/repo-maintenance.md` says which page). `/ze-close` verifies those edits, it does not start them |
| Understand the modular core | `ai/patterns/registration.md` | `docs/architecture/core-design.md` |
| Keep a plugin self-contained (removal test) | `ai/rules/plugins.md` | Remove the plugin and ALL its features vanish; other plugins and core keep working |
| Call another package's function directly from a plugin (not through RPC) | `ai/rules/plugins.md` | Check `p.IsInternal()`; guard with refuse-or-warn depending on how much value survives running external. Gated by `./le plugin boundary check` |
| Choose internal/core vs internal/component vs internal/plugins for a new package | `ai/rules/architecture.md` | Tier = dependency direction; engine placement gated by `./le tier check` (`./le tier check`) |
| Test linux-only code (QEMU) | `ai/rules/platform-linux.md` | `ai/rules/testing.md` (Linux-Only Tests section) |
| Fix a failing test, gate, demo, or user-visible problem | `ai/rules/completion.md` | Implement the missing behavior at the source, never route around it |
| Decide how much machinery a fix or feature needs (KISS, MVP, over-engineering) | `ai/rules/simplicity.md` | The simplest FULLY CORRECT answer, nothing beyond it. Cuts machinery, never correctness. A second problem gets its own spec, never an extra branch in this fix |
| Modify wire encoding | `ai/rules/performance.md` | `docs/architecture/buffer-architecture.md` |
| Add route processing | `ai/rules/architecture.md` | `docs/architecture/core-design.md` |
| Detect and auto-mitigate a DDoS flood | `docs/guide/ddos-mitigation.md` | `ddos-detect` characterizes the attack (family + vector) from `traffic-usage`/`flow-export`; `ddos-local`/`ddos-flowspec` install surgical rules; `show flow recent` inspects the flow ring |
| Detect behavioral security anomalies (exfil, C2, scanning) | learned `1046`/`1048`/`1049` | Neutral facts in `internal/component/trafficfeature` (fan-out, out/in ratio, entropy, beaconing) on `internal/core/stats`; `anomaly/detect` (report-only) scores per-entity deviation + cohort rarity into incidents (`show anomaly`); `anomaly/shape` responds shadow-first (per-source rate-limit, arm/auto-revert/kill-switch, `show anomaly-shape`). Separate security domain from `ddos`. |
| Provide or extend first-hop gateway redundancy (VRRP) | `docs/guide/vrrp.md` | RFC 9568/3768 in `internal/plugins/vrrp/` (self-contained plugin) with the per-group virtual-MAC macvlan in `internal/component/iface/macvlan.go`; extend within the self-contained `internal/plugins/vrrp/` plugin |
| Implement an RFC | `ai/rules/rfc-compliance.md` | `docs/contributing/rfc-implementation-guide.md` |
| Enrol an RFC, or change its public support row | `ai/skills/ze-rfc.md` | Both are declared in the `## Meta` table of `rfc/short/<stem>.md`, step 6b. `./le rfc index-update` generates `rfc/enrolled.txt`, `rfc/not-enrolled.txt` and `docs/features/rfc-status.md` from every summary. A hand edit to one of the three is lost at the next run |
| Prove an RFC MUST is enforced (tag a test, coverage gate) | `ai/skills/ze-rfc.md` | Tag the test `RFC requirement: <id> <polarity>` (both polarities); `./le rfc check` gates coverage; `./le rfc index-update` writes `rfc/requirements/<stem>.md` for that RFC's rows and `ai/RFC-REQUIREMENTS.md` for the index over all of them; a NEW tag also owes a discrimination proof in the same change (`./le rfc discriminate-record`); audit with `/ze-rfc-audit` |
| Write a spec | `ai/rules/planning.md` | `plan/TEMPLATE.md` (design-time only; placeholders are legal at `skeleton`, blocked from `design` on) |
| Close a spec (audit, goal validation, review gate, pre-commit evidence) | `ai/rules/completion.md` | `plan/TEMPLATE-CLOSURE.md`, appended by `/ze-close` at step 1; every Pre-Commit sub-table needs an evidence row |
| Record design risks and assumptions | `ai/rules/planning.md` (Risks & Assumptions) | A-N/R-N tables in `plan/TEMPLATE.md`; validate during /ze-implement audit |
| Add a feature, tool, self-check, verification gate, or test infrastructure | `ai/rules/repo-maintenance.md` | Update docs, rules, indexes, and verification paths in the same change |
| Change a web or looking-glass template, handler, or route, or prove its bytes did not move | `ai/patterns/web-endpoint.md` | `go test ./internal/component/web ./internal/component/lg` runs the template, handler, and Go-markup captures. Capture a deliberate change with `go test -run 'Test(Web|LG).*GoldenOutput' ./internal/component/web ./internal/component/lg -update-golden`, then read the diff |
| Compare Ze with other products | `ai/rules/writing.md` | Cite every claim, link code or official feature docs, label uncertainty, and add hide-column controls for wide product matrices |
| Add or change an agent behavior rule | `ai/rules/repo-maintenance.md` | Put shared Ze rules in `ai/rules/` and startup pointers in `ai/INSTRUCTIONS.md` |
| Reorganize YANG tree | `./le yang migration path-refactor operation <remove|rename|move> ...` | Add `apply` only after reviewing the preview |
| Move a package between tiers | `./le module move source <path> destination <tier-or-path>` | Add `apply` only after reviewing the preview |
| Rename the module path (host or owner change) | `./le module rename to <module> [from <module>]` | Add `apply` after the preview, then follow the native command's reported regeneration list |
| See which rule covers a topic | `ai/rules/INDEX.md` | One-line overview of every rule; open the listed file before acting |
| Understand Ze vs standard Go | `ai/rules/architecture.md` | Buffer-first, registration, YANG, etc. |
| Know which hooks will check my code | `ai/rules/repo-maintenance.md` | Pre-flight compliance checklist |
| Edit the website or presentations | `docs/contributing/gh-pages.md` then `website/AI.md` | Source layout, generation target, adding a talk |
| Write and publish the weekly update | `ai/skills/ze-weekly-update.md` | Draft in Zeledon voice, update `website/`, post the approved message to `ze-news`, and verify site/feed/homepage output |

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
| Verification/self-check gate | `ai/rules/repo-maintenance.md` | `docs/contributing/documentation-testing.md` | `internal/le/doc/check`, `internal/le/docvalid`, and the owning native action |
| EventBus event | `ai/rules/plugins.md` (EventBus Typed Payloads) | `pkg/ze/eventbus.go` | Use `events.Register[T]`, not raw `bus.Subscribe` |
| DirectBridge handler | `ai/rules/plugins.md` (DirectBridge section) | `pkg/plugin/rpc/bridge.go`, `plan/learned/DESIGN-HISTORY.md` "Plugin system: architecture" (294, retired) | |
| New component | `docs/architecture/core-design.md` section 1 | `ai/rules/architecture.md`, `ai/rules/architecture.md` | Proximity principle in `ai/rules/plugins.md` |
| New subsystem | `docs/architecture/hub-architecture.md` | `docs/architecture/subsystem-wiring.md` | |
| Test runner or format | `ai/rules/testing.md` | `ai/patterns/functional-test.md`, `docs/architecture/testing/ci-format.md` | `ai/rules/repo-maintenance.md` |


### Preparing a Commit

| Task | Read first | Then use |
|---|---|---|
| Generate and run a commit script | `ai/rules/git-safety.md` | Fast path: use `./le commit create`, then run it with `bash` and the path its `script=` line prints. if verification is considered, run `./le verify status check` first and never rerun verify when FRESH (`ai/rules/precommit-verify.md`) |
| Record a problem the work uncovered | `ai/rules/planning.md` ("Writing Journal Rows") | Append a row to `plan/journal/<class>.md`, then `--file` it on commit A; `./le journal report` prints every class that repeated |

### Modifying Existing Code

| Area | Read first | Key concerns |
|---|---|---|
| Reactor / session | `docs/architecture/core-design.md` sections 1-5 | `ai/rules/goroutine-lifecycle.md`; run `go test -race -count=20 ./internal/component/bgp/reactor/...` |
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
6. Run ./le ste review-changed to read your own prose back
7. Run ./le doc check verify after editing docs/
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
| Linux-only kernel code | `_test.go` | `internal/<pkg>/` | `./le qemu all-tests` |

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
| Every word you write | `ai/rules/writing.md`, guide: `docs/contributing/writing-style.md` | Rule one. ASD-STE100 Issue 9 for all repository writing: docs, comments, error messages, CLI output, YANG descriptions, specs, commit and PR text. Six banned habits. Gate: `./le ste check`. Report: `./le ste review` |
| Every line of Go you write | `ai/rules/go-standards.md`, guide: `docs/contributing/ze-go-style.md` | The reasoning behind the Go rules: safety, performance, developer experience, in that order. Adapted from TigerStyle. Read it once, then use the rule files |
| How much you write | `ai/rules/writing.md` | Any subagent report, rule, doc, commit body, or learned summary. Per-artifact budgets. A report to the owner routes to `ai/INSTRUCTIONS.md`, "Say it once, say it short" |
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
| How much work is already in flight | `./le spec session wip` | In-progress specs, stalest first, against `ZE_SPEC_WIP_CAP` (default 12); `claim` refuses a new `ready` spec over the cap |
| Who executes this phase (main thread vs subagent) | `ai/rules/planning.md` | Any spec work: the main thread supervises, each phase runs in a subagent through its `ze-*` skill |

## Dev Tools

`le` is the native development-tool personality of `cmd/ze`. The root `./le`
launcher executes `bin/le` and builds that personality only when the cached
binary is absent. The root `./ze` launcher does the same for `bin/ze`.

That cache is an existence test, never a freshness test, and `rm bin/le` is
banned because peer sessions execute the same file. When you need a binary that
carries your edits, put `./le --name <name>` first: it builds `bin/le-<name>/le`
on every call, leaves the shared binary alone, and refuses to answer if some
other binary is reached instead (`ai/rules/commands.md`,
`docs/guide/developer-setup.md`).

Both personalities use `internal/component/command`, but their composition roots
are deliberately different. A normal `ze` build imports no `internal/le`
package. A build with the non-default `ze_le` tag imports
`internal/le/register.go` once and exposes the same inventory below under
`ze le`. Standalone `./le <area> <action>` and `ze le <area> <action>` therefore
cross through one command engine without putting development tools in the
shipped binary.

Every first-party development and test workflow is a compiled Go action or a
direct Go binary. Run `./le <area>` to list an area's exact action words,
whether each writes, and its purpose. Data such as patches, templates,
Dockerfiles, and copied fixtures can live outside Go packages; executable
repository tooling lives under `internal/le`, `internal/test`, or
`internal/appliance`.

### Which surface answers a discovery question

Reach for one of these before inventing a new mechanism.

| Need | Existing surface |
|------|------------------|
| Changed-file-aware wiring, doc, command, and inventory gate | `./le doc wiring` |
| Documentation drift and YANG command contracts | `./le doc check verify` |
| Source-to-document reverse index | `./le docs-to-code index-update`; read `ai/CODE-TO-DOCS.md` |
| Which tests enforce an RFC MUST, and what the backlog is | `./le rfc index-update`. Read `rfc/requirements/<stem>.md` for one RFC, `ai/RFC-REQUIREMENTS.md` for the rollup over all of them. Coverage is gated by `./le rfc check`, freshness by `./le doc check verify`. Which of those tests is PROVEN to discriminate its claim is `./le rfc discriminate stem <stem>` |
| What each package does | `./le discovery-index update`; read `ai/PACKAGE-MAP.md` |
| Which `.go` files implement a design doc | read `ai/DOCS-TO-CODE.md`, the inverse of `// Design:` |
| Which problems recur | `./le journal report`; read `plan/journal/`, one file per class, where the row count is the recurrence |
| Whether every path a tracked file names still resolves | `./le doc check links`. It is its own `./le verify current mode full` stage, and `sweepTracked` in `internal/le/doc/check/links.go` sweeps every tracked file, not only the instruction corpus. Repair the reference, or mark its line with a `doc-links: ignore` marker that states why the path cannot resolve. `vendor/`, `third_party/` and `plan/handover/` are excluded |
| Whether every symbol a `docs/` source anchor names is declared where the anchor points | `./le doc check verify` (`checkAnchors` in `internal/le/docstocode/codetodocs.go`) |
| How data flows through a subsystem | read `ai/digests/<subsystem>.md`; `ai/digests/README.md` lists them and `./le digest` validates their anchors |
| Plugin, command, YANG, and test inventory | `./le inventory` |
| Command inventory | `./le command list` |
| Spec progress | `./le spec status` |
| Generated plugin imports | `./le plugin imports check` |
| Whether the tree git holds compiles | `./le repository tracked-build check`. It runs in both full verification modes and is a structural gate in `internal/le/commit` |
| Runtime readiness | `ze doctor --json` and `ze explain <diagnostic-code>` |

Each of these answers with structured data, so `| json`, `| yaml` and `| table`
all render it.

### Add a development tool

1. Add one package at the path the command name predicts: a space in the name
   is a directory level, so `le verify lint` lives at
   `internal/le/verify/lint/`. Give it callable behavior and an
   `Answer(args []string) (any, int)` command boundary. Use a space, never a
   hyphen, when the left word is an object with members: `./le cli-grammar`
   refuses `verify-lint` and names the split.
2. Declare every action once in that package's action table. Action words are
   keywords and precede any value.
3. Register the area with `leroot.Register`, naming the group help files it
   under (`leroot.GroupWorkflow`, `GroupGate`, `GroupGenerate`, `GroupSuite`,
   or `GroupReport`); return structured answers so the shared pipe renderers
   remain available.
4. Blank-import the package exactly once from `internal/le/register.go`. Do not
   import it from normal `cmd/ze` composition.
5. Add its operator and producer row here. Remove the retired implementation
   and migrate every caller to the native action.

### Native command inventory

| Command | Producer | Purpose |
|---------|----------|---------|
| `./le ai` | `internal/le/ai.Answer` | the generated agent files: sync every tool's copy of the skills and instructions, or check them |
| `./le arch-map` | `internal/le/archmap.Answer` | the generated architecture lists in ai/INSTRUCTIONS.md: check them against the tree, or rewrite them |
| `./le build-artifacts` | `internal/le/buildartifacts.Answer` | build the host appliance driver and the amd64 or arm64 installer initrd |
| `./le changed` | `internal/le/changed.Answer` | what this checkout changed: the test groups it touches, and the packages a scoped verify must cover |
| `./le ci-dispatch` | `internal/le/cidispatch.Answer` | every command string this repository sends to its own daemon still routes, so a renamed command tree cannot leave a test passing against a key that is gone |
| `./le cli-grammar` | `internal/le/cligrammar.Answer` | every built-in command, every registered root, every demo call site and every offline flag still obeys the CLI grammar: keyword before value, no flag in the command model, no dead launch form, and each flag in its own register |
| `./le command list` | `internal/le/command/list.Answer` | every registered command, by verb, read from the live handlers and schemas |
| `./le command ownership` | `internal/le/command/ownership.Answer` | each command is owned by exactly one plugin or component: owners are cmd/ze-free, root handlers are internal, and every central root states why it has no owner |
| `./le commit` | `internal/le/commit.Answer` | prepare explicit commits without touching the shared staging index |
| `./le config claims` | `internal/le/config/claims.Answer` | every config subtree an operator can write is delivered to a plugin, a hub handler, or a recorded exception |
| `./le config coercion` | `internal/le/config/coercion.Answer` | config parsers coerce the string form every YANG leaf is delivered as, so an operator's value is never silently replaced by the default |
| `./le consistency` | `internal/le/consistency.Answer` | where the code and the documentation disagree: design refs, cross-refs, JSON tags, file sizes |
| `./le dash-stdio` | `internal/le/dashstdio.Answer` | every command that takes a filename routes it through the helper that resolves "-", so an operator can always pipe into and out of one |
| `./le deployment` | `internal/le/deployment.Answer` | ze against a real peer daemon in a container: the protocol proofs that need another implementation to mean anything |
| `./le digest` | `internal/le/digest.Answer` | every file:line anchor in ai/digests/*.md resolves to a real file and an in-range line |
| `./le discovery-index` | `internal/le/discoveryindex.Answer` | the generated package map in ai/PACKAGE-MAP.md: check it against the tree, or rewrite it |
| `./le doc check` | `internal/le/doc/check.Answer` | native documentation links, aggregate verification, and templ output checks |
| `./le doc wiring` | `internal/le/doc/wiring.Answer` | the changed-file wiring, documentation, command and inventory gate |
| `./le docs-to-code` | `internal/le/docstocode.Answer` | the two generated doc indexes, ai/DOCS-TO-CODE.md and its reverse ai/CODE-TO-DOCS.md: check either against the tree, or rewrite it |
| `./le docvalid` | `internal/le/docvalid.Answer` | the documentation gates: the YANG command contract, the doc drift check, and the generated operator table |
| `./le evidence` | `internal/le/evidence.Answer` | release-candidate evidence: run the verify gate over a clean clone of this checkout, inside a container |
| `./le feature-tags` | `internal/le/featuretags.Answer` | the build-tag lists derived from feature-gates.txt: check the four files that carry one, or rewrite them |
| `./le fs-persistence` | `internal/le/fspersistence.Answer` | daemon runtime state is persisted through the managed zefs store, never as a loose file a reimage would drop |
| `./le functional` | `internal/le/functional.Answer` | functional suites, fail-open Docker-exec analysis, and ExaBGP compatibility |
| `./le fuzz` | `internal/le/fuzz.Answer` | Go fuzzing: every `func Fuzz` under internal/, discovered at run time |
| `./le go-extract` | `internal/le/goextract.Answer` | move named declarations from one Go file to another, comments and formatting intact |
| `./le gokrazy-gosum` | `internal/le/gokrazygosum.Answer` | the packed gokrazy/ze/builddir/**/go.sum files agree with the root module about what a version contains |
| `./le hook-check` | `internal/le/hookcheck.Answer` | native hook dispatcher golden and behavioral fixture selftests |
| `./le htmx-upgrade` | `internal/le/htmxupgrade.Answer` | htmx 4 upgrade findings: check the explained list against every htmx-bearing package, or report every scanner issue |
| `./le iana-asn` | `internal/le/ianaasn.Answer` | the shipped RIR delegation seed: fetch the five registries' files and rewrite the ASN-to-RIR delegation table |
| `./le iface-resolution` | `internal/le/ifaceresolution.Answer` | no Ze code resolves a configured interface name straight against the kernel: every logical name goes through the shared resolver |
| `./le integration` | `internal/le/integration.Answer` | integration, interop, stress, and live proofs that need Docker, root, a namespace, or internet access |
| `./le inventory` | `internal/le/inventory.Answer` | what ze is made of: plugins, families, YANG modules, RPCs, tests and package sizes |
| `./le job` | `internal/le/job.Answer` | admit a heavy job before it runs, so the sessions sharing this machine do not oversubscribe it |
| `./le journal` | `internal/le/journal.Answer` | report recurring problem classes from the committed journal |
| `./le module` | `internal/le/module.Answer` | preview or apply package-tree moves and repository Go module-path renames |
| `./le mutation` | `internal/le/mutation.Answer` | combine mutation reports and append their per-package scores to committed history |
| `./le netlab` | `internal/le/netlab.Answer` | render and validate the netlab daemon integration |
| `./le perf-bench` | `internal/le/perfbench.Answer` | suggest a perf run when BGP data-plane code changed since the last one |
| `./le platform-vet` | `internal/le/platformvet.Answer` | vet the host and interface trees against their Darwin and FreeBSD implementations |
| `./le plugin boundary` | `internal/le/plugin/boundary.Answer` | no plugin reaches engine state through a plain in-process call, so moving that plugin to an external subprocess cannot silently disable it |
| `./le plugin imports` | `internal/le/plugin/imports.Answer` | the generated composition root: check that internal/component/plugin/all names every package the tree registers, or write it |
| `./le port-defaults` | `internal/le/portdefaults.Answer` | the Go listener-default table and the YANG refine port defaults still agree, service by service |
| `./le protocol-skeleton` | `internal/le/protocolskeleton.Answer` | which protocol implementations are still a skeleton rather than a daemon, classified against ai/rules/protocol.md |
| `./le qemu` | `internal/le/qemu.Answer` | proofs that boot a real appliance image in a virtual machine and ask it what it did |
| `./le repository` | `internal/le/repository.Answer` | the post-verify repository checks: source anchors resolve, exported symbols have a cross-package caller, CLI commands have a .ci test, and an in-progress spec's acceptance criteria say how they are demonstrated |
| `./le repository tracked-build` | `internal/le/repository/trackedbuild.Answer` | the tree git holds compiles in every shipped flavor, so a consumer committed without its producer is caught before anybody else builds the commit |
| `./le rfc` | `internal/le/rfc.Answer` | RFC conformance: bind every MUST-level requirement of an enrolled RFC to the tests that enforce it, prove each binding with a recorded break under which the tagged test goes red, and bound what the summaries missed |
| `./le rules` | `internal/le/rules.Answer` | the rule corpus in ai/rules/: lint and render it, map hook enforcement, and report matched rules unread in a session transcript |
| `./le scratch` | `internal/le/scratch.Answer` | keep disposable scratch and durable caches outside the checkout without overwriting existing work, and empty both Go build caches when the cache disk fills |
| `./le session` | `internal/le/session.Answer` | manage this development session's isolated state |
| `./le setup` | `internal/le/setup.Answer` | install and verify every tool a Ze dev or test workflow needs |
| `./le site facts` | `internal/le/site/facts.Answer` | the numbers the website publishes about this repository: derive them into website/data/repo-facts.json, or check what has gone stale in it |
| `./le source-rewrite` | `internal/le/sourcerewrite.Answer` | deterministic repository rewrites: rules, BGP expectations, replacements, and activity HTML |
| `./le spec citation` | `internal/le/spec/citation.Answer` | a plan/spec-*.md citing a sibling spec absent on disk fails, unless the target is grandfathered in plan/.citation-baseline; a path:line citation whose backtick-quoted token drifted off that line warns |
| `./le spec session` | `internal/le/spec/session.Answer` | spec ownership, per-spec state paths, transcript model facts, and independent review artifacts |
| `./le spec status` | `internal/le/spec/status.Answer` | the spec inventory: status, bucket and stale-skeleton flag for every plan/spec-*.md |
| `./le staticcheck-feature-matrix` | `internal/le/staticcheckfeaturematrix.Answer` | the tree type-checks in every feature-tag combination Ze can be built in, so a package the default build compiles out is judged too |
| `./le ste` | `internal/le/ste.Answer` | the repository's writing, against ASD-STE100 Simplified Technical English: review every surface, and gate each changed file against its own HEAD version |
| `./le stress-repro` | `internal/le/stressrepro.Answer` | reproduce load-dependent functional-test failures under bounded CPU, GC, and process pressure |
| `./le terminal-demo` | `internal/le/terminaldemo.Answer` | build, validate, verify, and render the published terminal demonstrations |
| `./le test-chaos` | `internal/le/testchaos.Answer` | chaos simulator tests, reduced-tag CLI tests, and lint |
| `./le test-health` | `internal/le/testhealth.Answer` | the project's testing state as one generated page: what is measured, what is ratcheted, and which structural facts are gated |
| `./le test-sensitivity` | `internal/le/testsensitivity.Answer` | no more tests than the committed floor pass unconditionally or sit behind a build tag nothing supplies, which no count of tests can reveal |
| `./le test-unit` | `internal/le/testunit.Answer` | the five race-instrumented component-group Go test suites, and the installer initrd behind its own tag |
| `./le test-weakened` | `internal/le/testweakened.Answer` | detect and record test weakenings against a commit baseline |
| `./le tier` | `internal/le/tier.Answer` | module-tier placement: a config-driven engine lives in internal/component/ when a feature depends on it and in internal/plugins/ otherwise, internal/core/ imports neither, and no always-on package imports a compile-out-able feature |
| `./le token-economy` | `internal/le/tokeneconomy.Answer` | where this repository's sessions spend their tokens: API calls, the context carried at each one, the size histogram and a capped-context counterfactual, read from the machine-local transcript store |
| `./le tracked` | `internal/le/tracked.Answer` | does le still work when built from what git holds, rather than from the working tree |
| `./le vendor-web` | `internal/le/vendorweb.Answer` | the vendored web assets: check every consumer copy against third_party/web/, sync them, or ask npm what is newer |
| `./le verify` | `internal/le/verify.Answer` | the full pre-commit gate against a fixed commit in a detached worktree |
| `./le verify deps` | `internal/le/verify/deps.Answer` | the Go-tool and dependency stages used only by native pre-commit verification |
| `./le verify lint` | `internal/le/verify/lint.Answer` | run golangci-lint over every Go build flavor and prove tracked-file coverage |
| `./le verify lock` | `internal/le/verify/lock.Answer` | run a verify-class command through the shared heavy-job admission |
| `./le verify status` | `internal/le/verify/status.Answer` | read and write the verification certificate for the current checkout |
| `./le verify summary` | `internal/le/verify/summary.Answer` | append one stage failure block to the verification failure index |
| `./le web-assets` | `internal/le/webassets.Answer` | the per-page web asset sets derived from the markup each page renders: check them, write them, or print them |
| `./le weekly` | `internal/le/weekly.Answer` | publish the weekly update to Discord; the bare command shows what would be sent |
| `./le wiki-catalog` | `internal/le/wikicatalog.Answer` | the generated command-catalog Markdown: check it against live registries, or rewrite it |
| `./le working-tree` | `internal/le/workingtree.Answer` | how wide the uncommitted tree is, grouped by area. Advisory unless max-areas names a ceiling |
| `./le worktree` | `internal/le/worktree.Answer` | bring a linked git worktree up to date with main, stashing and restoring its uncommitted work |
| `./le yang glue` | `internal/le/yang/glue.Answer` | the generated YANG glue: check that every embed.go and register.go agrees with the .yang files beside it, or write them |
| `./le yang leaf-mentions` | `internal/le/yang/leafmentions.Answer` | which YANG config leaves the owning plugin package never names, so a leaf that is delivered but never read is visible |
| `./le yang migration` | `internal/le/yang/migration.Answer` | repository-wide YANG ownership and path migrations |

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

Structural decisions, patterns, and gotchas. Recurrence data: `plan/journal/` (`./le journal report`).
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
| Plugin system reference | `docs/architecture/plugin/plugin-system.md` |
| Feature gates | `docs/architecture/plugin/feature-gates.md` |
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
| CI workflows: which suite runs where, and what blocks | `docs/architecture/testing/ci-workflows.md` |
| Test runner | `docs/architecture/testing/runner-architecture.md` |
| ExaBGP mapping | `docs/exabgp/exabgp-code-map.md` |
| ExaBGP compat | `docs/exabgp/exabgp-differences.md` |

## Keyword → Architecture Doc

| Keywords | Docs |
|----------|------|
| buffer, iterator, parse, wire | `core-design.md`, `buffer-architecture.md`, `ai/rules/performance.md` |
| encode, Pack, WriteTo, alloc | `ai/rules/performance.md`, `buffer-architecture.md` |
| string building, textbuf, Sprintf, concatenation | `textbuf-string-building.md`, `ai/rules/performance.md` |
| pool, BufHandle, BufMux, peerPool, copy-on-modify | `buffer-architecture.md`, `pool-architecture.md`, `forward-congestion-pool.md` |
| tier, placement, core vs component vs plugin, compile-out | `module-tiers.md`, `ai/rules/architecture.md` |
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
| templ, `.templ`, generated markup, view-model type safety | `./le doc check templ-output` (Dev Tools above), `tools.go`, `internal/component/web/templ_typesafety_test.go` |
| per-page asset imports, `page_assets.go`, `pageAssets`, `//ze:page`, htmx extension per page | `./le web-assets check` (Dev Tools above), `internal/le/webassets.Write`, `internal/test/markupcheck/head.go` |
| golden fixture, rendered markup, template bytes, byte-for-byte HTML | `go test ./internal/component/web ./internal/component/lg`, `internal/test/golden`, `web-interface.md` |
| rendering-engine port, pre-port bytes, normalized HTML comparison | `go test -run 'Test(Web|LG)TemplPortFidelity' ./internal/component/web ./internal/component/lg -port-ref=<sha>`, `internal/test/golden/portcheck.go`, `internal/test/golden/normalize.go` |
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
| RFC requirement coverage, RFC MUST tests, rfc-requirements, RFC requirement tag, ./le rfc check, ./le rfc index-update | `./le rfc check`, `rfc/requirements/<stem>.md` (one RFC's rows), `ai/RFC-REQUIREMENTS.md` (the index), `ai/skills/ze-rfc.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/functional-tests.md` (RFC Requirement Tags) |
| RFC extraction sign-off, extraction completeness, what the summary MISSED, unextracted obligation, normative site, extraction register, rfc2119/prose/manual-walk, drain budget, ./le rfc extraction-create, ./le rfc extraction-status | `rfc/extraction/README.md`, `./le rfc extraction-create`, `./le rfc extraction-status`, `rfc/drain-budget.txt`, `ai/rules/rfc-compliance.md` (Extraction Completeness, the five ratchets), `ai/RFC-REQUIREMENTS.md` (Extraction sign-off) |
| RFC audit verdict, rfc/audit schema, enforced/weak/wrong/unimplemented/not-applicable, no_code_path, upgrade_reason, units map, code map, SHIFTED verdict, stale audit verdict, ./le rfc reseal, audit coverage | `ai/skills/ze-rfc-audit.md` (the verdict vocabulary and the four freshness states), `./le rfc reseal`, `ai/RFC-REQUIREMENTS.md` (Audit coverage), `internal/le/rfc.UnitAt` (the definition of the tagged unit), `ai/rules/rfc-compliance.md` (a verdict is never authority) |
| claim discrimination, over-claiming tag, does the test prove what the tag says, proof route, mutant/revert, no-break escape, rfc/discrimination, ./le rfc discriminate, ./le rfc discriminate-record, claim-sha | `docs/contributing/rfc-conformance-gates.md` (The discrimination record; the two proof routes; the escape), `rfc/discrimination/README.md` (the artifact contract), `./le rfc discriminate` (propose), `./le rfc discriminate-record` (observe and write), `ai/RFC-REQUIREMENTS.md` (Claim discrimination), `ai/skills/ze-rfc.md` |
| payload-predicate waits, sleep elimination, ci-sleep ratchet, ci-sleep justification, engine-step predicates (`matches=`, `absent=`, `json=`) | `docs/functional-tests.md` ("Payload-predicate waits"), `docs/architecture/testing/ci-format.md` ("Engine Steps"), `ai/rules/testing.md`, `internal/test/runner/engine_steps.go` |
| poll loop, wait loop, waiting for a background command, watcher, waiting for QEMU boot | `ai/rules/commands.md`, `ai/rules/repo-maintenance.md` (poll-loop), `internal/le/hookruntime/bash.go` |
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
| XFRM, xfrm interface, VTI, netlink, go mod vendor | `ai/digests/ipsec-ike.md`, `internal/component/ike/`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (vendored netlink patch, recovery command, and owning gate), `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch` |
| subscriber, session, PPPoE, L2TP | `ai/digests/subscriber.md`, `internal/component/l2tp/pppoe/` |
| editor, TUI, completion, headless | `internal/component/cli/`, `test/editor/`, `ai/rules/testing.md` (Editor Tests section) |
| diagnostic, doctor, health, readiness, plugin setup, setup outcome, show plugins, why is this feature absent | `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/diagnostics/production-diagnostics.md`, `ai/rules/repo-maintenance.md` |
| EventBus, event, pub/sub, subscribe, emit | `pkg/ze/eventbus.go`, `ai/rules/plugins.md` (EventBus Typed Payloads), `internal/core/events/typed.go` |
| DirectBridge, bridge, direct call, typed handler | `pkg/plugin/rpc/bridge.go`, `ai/rules/plugins.md` (DirectBridge), `plan/learned/DESIGN-HISTORY.md` "Plugin system: architecture" |
| BFD, bidirectional forwarding | `docs/architecture/bfd.md` |
| resolve, origin, pipe, pipe operator | `docs/architecture/resolve.md`, `ai/rules/cli.md` |
| MCP, model context protocol | `docs/architecture/mcp/`, `internal/component/mcp/` |
| self-update, manifest, auto-update | `docs/architecture/appliance/self-update.md`, `docs/guide/self-update.md` |
| ASPA, path verification, RTR | `docs/guide/rpki.md` (ASPA Path Verification), `internal/component/bgp/plugins/rpki/`, `docs/features/rfc-status.md` (draft-ietf-sidrops-aspa-verification) |
| BMP, monitoring protocol | `docs/guide/bmp.md`, `internal/component/bgp/plugins/bmp/`, `docs/architecture/api/commands.md` (bmp-sessions, bmp-peers) |
| docker, container, scratch, lab image | `docs/guide/docker.md` (both images), `docker/Dockerfile`, `docker/Dockerfile.lab` |
| netlab, containerlab, lab topology, daemon integration, contrib | `docs/guide/netlab.md`, `contrib/netlab/README.md`, `contrib/netlab/ze.yml`, `contrib/netlab/ze/`, `./le netlab render-check`, `docker/Dockerfile.lab` |
| chaos, fault injection, scheduler | `docs/architecture/chaos-web-dashboard.md`, `docs/guide/chaos-testing.md` |
| changed set, scoped verify, which packages changed, change-set selector, reverse dependency depth, feature scope, staticcheck matrix rows, `ZE_VERIFY_SCOPE_TAGS` | `./le changed scope`, `internal/le/changed.Answer`, `internal/le/staticcheckfeaturematrix.Answer`, `docs/architecture/testing/verify-freshness-scope.md`, `ai/rules/commands.md` |
| declared failure group, VERIFY FAILURE GROUP, whose red is this, attributing a structural red, failure index | `internal/le/verify/engine.RunMode`, `internal/le/doc/wiring.Group`, `internal/le/commit.Answer`, `docs/architecture/testing/verify-freshness-scope.md`, `ai/rules/precommit-verify.md` |
| commit, commit script, commit message, verified commit, verify freshness, owner override, commit no test, verification debt, gate owed, push refused | `internal/le/commit.Answer`, `internal/le/verify/status.Answer`, `ai/rules/git-safety.md`, `ai/rules/precommit-verify.md`, `ai/skills/ze-commit.md`, `ai/skills/ze-commit-check.md` |
| weekly update, Zeledon, ze-news, Discord announcement, website changes, homepage latest updates | `ai/skills/ze-weekly-update.md`, `website/AI.md`, `website/changes/discord/STYLE.md`, `internal/le/weekly`, `internal/le/site` |
| spec status, spec metadata, spec closure, deferral row, deferral shard, executive summary, session handoff, handover | `ai/rules/planning.md`, `docs/contributing/spec-workflow.md`, `plan/TEMPLATE.md`, `./le spec status` |
| self-improvement, discoverability, discovery, new tool, self-check, verification gate | `ai/rules/repo-maintenance.md`, `docs/contributing/documentation-testing.md` |
| inventory, command-list, doc drift, source anchor, doc index | `ai/rules/repo-maintenance.md`, `ai/rules/writing.md`, `docs/contributing/documentation-testing.md`, `./le inventory`, `./le docvalid`, `./le docs-to-code` |
| clear, clear command, clear dns, clear interface, clear ipsec | `internal/component/resolve/cmd/` (dns), `internal/component/iface/cmd/` (interface), `internal/component/ike/cmd/` (ipsec), `internal/component/cmd/clear/` (verb root) |
| command grammar, verb-first, command alias, deprecated alias, grammar gate | `ai/rules/cli.md` (Mechanical Enforcement), `./le cli-grammar`, `docs/architecture/cli/root-namespace-grammar.md` |
| flag or keyword, --flag, flag register, flag registry, offline flag, client flag to the daemon, --json versus pipe | `ai/rules/cli.md` (`--flag` or Keyword), `./le cli-grammar`, `internal/component/command/grammar/flags.go`, `docs/architecture/cli/root-namespace-grammar.md` |
| DispatchCommandArgs, typed inter-plugin dispatch, tokenizer bypass | `docs/architecture/api/process-protocol.md`, `ai/digests/plugin-transport.md`, `ai/rules/plugins.md` |
| RawMessage, double marshal, callback passthrough, SDK callback | `docs/architecture/api/process-protocol.md`, `ai/digests/api-ipc.md` |
| pipe first, pipe last, pipe metadata | `ai/rules/cli.md` (The Rule (pipes)), `docs/guide/command-reference.md`, `docs/features/formatting.md` |
| RIB dump, bounded dump, replay batching, update cursor | `docs/architecture/bgp/replay-cursor.md`, `ai/digests/rib.md` |
| plugin internal keyword, in-process plugin config | `docs/guide/plugins.md` (the `internal` keyword), `ai/patterns/plugin.md` |
| appliance auth, local admin, bootstrap auth, RBAC | `docs/guide/operator-access-rbac.md`, `ai/digests/aaa-auth.md`, `internal/component/authz/`, `internal/component/aaa/` |
| appliance, appliance iso, appliance build, appliance init | `internal/appliance/`, `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `./le build-artifacts`, `./le qemu` |
| Dependabot alert on vendored go.mod, gokrazy/modcache manifest, bump gokrazy init, appliance dependency bump, CVE on vendored appliance dep | `ai/rules/platform-linux.md`, `./le setup install`, `internal/le/setup/`, `.github/dependabot.yml` |
| installer initrd QEMU evidence, R-6 fault injection, ze.mac pin, rescue console, Ventoy ISO-on-FAT, ze_installer_fault, ZE_INITRD_FAULT | `internal/le/qemu/`, `internal/install/disk/fault_linux.go`, `./le qemu install-scenarios-test`, `./le qemu install-ventoy-test`, `docs/functional-tests.md` |
| VPP hugepage boot reservation, poll-sleep-microseconds, image.hugepages, doctor-vpp-hugepages, hugepage QEMU evidence | `internal/appliance/kernelargs.go`, `internal/component/vpp/doctor_linux.go`, `internal/component/vpp/startupconf.go`, `internal/le/qemu/`, `./le qemu vpp-hugepages-test`, `docs/guide/vpp.md`, `docs/guide/appliance.md` |
| VPP semantics, linux-cp, LCP, LCP netns, lcp_itf_pair_create, default netns, binapi, lcp.ba.go, foreign system semantics | `third_party/vpp-linux-cp/` -- vendored VPP C (v25.10, read-only reference). Read this BEFORE claiming what VPP does; the generated stub `vendor/go.fd.io/govpp/binapi/lcp/lcp.ba.go` says a field exists, never what VPP does with it (`ai/rules/evidence.md`) |
| .ci test prerequisite, option=needs-path, caps=net-raw, caps=net-admin, caps=bpf, test skips instead of failing, missing modcache, setup install prerequisite | `docs/architecture/testing/ci-format.md` (Options table), `ai/rules/platform-linux.md`, `internal/test/runner/caps.go`, `internal/test/runner/needs_path.go`, `./le setup install` |
| test passes on macOS but fails in CI, works locally red in CI, unprivileged runner, 4-vCPU runner | `ai/rules/platform-linux.md` (skip-os is not a capability declaration), `ai/rules/completion.md` |
| code-to-docs, reverse index, which docs | `ai/CODE-TO-DOCS.md` (generated, `./le docs-to-code index-update`) |
| mutation testing, gomu, mutation score, mutant | `./le mutation`, `internal/le/mutation/`, `ai/rules/testing.md` (Mutation Testing section) |
| test health, testing dashboard, proof density, assert-nothing, tests that cannot fail, tag-orphan, test KPI, is our testing correct | `docs/features/test-health.md`, `docs/architecture/testing/test-health.md` (architecture), `test/health/README.md`, `internal/le/testhealth.Answer`, `internal/le/testsensitivity.Answer`, `ai/rules/testing.md` (Test Sensitivity Ratchets) |
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

RFC summaries: `rfc/short/`. Full RFCs: `rfc/full/`. Each summary's `## Meta` table
declares its own enrolment and its own row on `docs/features/rfc-status.md`.

## Session State

Per-session: `tmp/session/<YYYY-MM-DD>-<SID>/state/session-state-<spec-stem>-<SID>.md` (gitignored),
in the directory that also holds this session's `bin/` and `scratch/`. Each session gets its own file.
Session markers: `tmp/session/.session-<ID>` map sessions to specs. `./le spec session current`
reads the claim, `./le spec session state current` locates this session's state, and
`./le spec session state latest spec <spec-file>` locates the newest prior state. The
producer is `internal/le/spec/session/`.

## Project Facts (no rule carries these)

Trivia that costs a session to rediscover. Moved here from `ai/rules/repo-maintenance.md`
on 2026-08-30, because a lookup is not a rule.

- **Family registration** is dynamic via `PluginRegistry.Register()` -- never enumerate, validate format only.
- **Config pipeline**: File -> Tree -> `ResolveBGPTree()` -> `map[string]any` -> `reactor.PeersFromTree()`. Files: `internal/component/bgp/config/{resolve,peers}.go`, `.../reactor/config.go`.
- **Linter hook**: `postFormatGo` in `internal/le/hookruntime/postwrite.go` runs gofmt, `goimports -format-only`, and changed-code lint on Edit/Write. Imports are not auto-removed, so add an import and its use in the same edit.
- **Arch-0**: 4 components (Engine, ConfigProvider, PluginManager, Subsystem). Subsystem != Plugin (BGP daemon = subsystem; bgp-rib/rs/gr = plugins). Stream system = pub/sub backbone (`internal/component/plugin/server/dispatch.go`). Interfaces in `pkg/ze/`.
- **YANG choice/case**: `mandatory true` and inner-choice exclusivity are NOT enforced by the walker. Plugins using `choice` add Go-side validation in their parser. `ze config validate` does not invoke `OnConfigVerify`.
- **Constants for command/status names** -- literals catch typos at compile time. Editor commands: `internal/component/cli/model.go`. Plugin status: `plugin.StatusDone`/`StatusError`.
- **Proximity**: `bgp/handler/` is a middleman; handlers belong in `bgp/plugins/`. ALL RPCs need YANG.
- **Inventory**: `./le inventory [--json]` imports `plugin/all` and queries real registries. Use it for plugin counts, RPC totals, family coverage.
- **SDK type aliases** (`pkg/plugin/sdk/sdk_types.go` re-exporting `rpc.*`) are intentional -- external plugins import only `sdk`. They are not identity wrappers.
- **No filtered/noexport route tracking** -- Ze does not store import-filtered or export-filtered routes (unlike BIRD's "import keep filtered on"): the RIB pipeline has scope keywords (sent/received/sent-received) and filter stages, but no "filtered" scope. The birdwatcher-compatible endpoints `/routes/filtered/{name}` and `/routes/noexport/{name}` return empty lists for compatibility; if filtered tracking ever lands, point them at the real store.
- **Gokrazy appliance owns process lifecycle** -- ze deploys as a gokrazy appliance: no systemd, no init system, no package manager. Any external process ze depends on (VPP or future dependencies) is exec'd, supervised, and cleaned up by ze itself; ze is never designed around an OS-level process manager.
- **Stress tooling is native Go**: `internal/le/integration/stress.go` owns stress orchestration, and the BGP UPDATE stream is generated inside `ze-test peer --mode inject`. Extend the Go injector for a new scenario with a pool-friendly byte builder, one pre-allocated buffer, one TCP writer, and a keepalive goroutine. Run it through `./le integration stress`.
- **CLI dispatch discoverability gaps**: (1) no one-shot command against a RUNNING daemon (`ze cli -c "summary"` shape). `ze show` and `ze run` use SSH (`sshclient.ExecCommand`) internally but expose no shell one-liner. The offline-config half is covered by `ze config show <file> [path...]`. (2) `ze help --ai --api` prints YANG RPC names (`ze-bgp:overview`), not the dispatch strings users type. (3) No way to list the Dispatcher's match keys. `reactor.ExecuteCommand()` accepts strings undiscoverable without reading source. The highest-value fix is the one-shot daemon command (SSH port 2222, credentials from the zefs database).

### Mistakes that recur, and their corrections

One line each. The full class, with reproduction and fix, is in `plan/learned/RECURRING-PATTERNS.md`
and the matching `plan/journal/` class file. A mistake-log entry is one line: the lesson, then the
rule it points at.

- **"Linux-only tests cannot run on this macOS host" is false** (RECURRING, ZERO TOL). Mark kernel-dependent `.ci` cases with `option=needs-linux`, use `./le qemu netns-test suites <names>` for a focused pass, and `./le qemu all-tests` for the full guest proof. Never dismiss such a failure as environmental.
- **Feature not wired** (RECURRING, ZERO TOL). Unit tests are not wiring. Name the user entry point.
- **Daemon command without offline CLI** (sysctl-0). Every `CommandDecl` plugin needs a `cmd/ze/<name>/` offline entry point.
- **Wrong production path** (rib-04). Grep ALL implementations; trace the consumer's call chain.
- **Count-only test assertions** (addpath-rib). Assert on content (keys/values), not `Len()`.
- **Wrapper struct pattern** (alloc-4). Pass raw bytes and existing iterators; do not wrap data in accessor types.
- **Tests-pass is not done** (RECURRING). Tests are step 10 of 12: docs, spec, summary and audit follow.
- **Mechanism-not-behavior test** (prefix-limit). Assert the AC, not a code-path proxy. A no-op that passes is the wrong test.
- **Plugin placement anchor bias** (jsonrpc). Apply the "delete the folder" test. Cross-cutting -> `internal/component/`. Domain -> `bgp/plugins/`. Infra -> `internal/core/`.
- **Docs from assumption** (RECURRING). Read the source before any factual claim.
- **Reinventing repo contents** (lg-overhaul). Grep existing code before writing new infra; `third_party/` and the components often already have it.
- **Spec claimed complete with gaps** (lg-0..4). A learned summary saying "future X" means the spec is NOT done.
- **Stale deferrals** (redist-phase2). Grep the code before creating a phase-N spec out of open deferrals.
- **Same-day blocker fix** (cmd-4, RECURRING). An adversarial review races on reactor code, greps renamed-name consumers and sibling call sites, and breaks production to confirm the `.ci` test fails.
- **Substring collision in bulk edits** (iface-tunnel). Match the longest prefix first, or add non-name context, then grep for mangled names afterwards.
- **Vendor is not upstream** (iface-tunnel). Verify behavior against `vendor/<lib>/`, not upstream docs, and cite the vendor path.
- **Naive reconciliation drops live state** (iface-tunnel). Diff the new config against the previous one and act on the delta; pass `previous` explicitly.
- **Invented config shape** (iface-tunnel). Grep the existing `*-conf.yang` files for the closest analog before defining a new endpoint shape.
- **Scratch `.go` in `tmp/`** (iface-tunnel). `go test ./...` walks `tmp/`; research agents write `.txt` or build-tagged directories.
- **CLI grammar from container nesting, not wire method** (as112-cli-audit). Operator-facing command words come from the YANG `container` tree; `ze:command "ze-X:Y"` is the INTERNAL RPC name and is deliberately different (`ze-bgp:peer-teardown` is the command `request peer teardown`). The top-level operational verb is `request` (`request <object> <action>`); reads are `show`/`monitor`.
- **ExaBGP migration sync** (exabgp-compat-sync). When ExaBGP adds a SAFI or route type, update three things: the `exabgp.yang` schema container, the `flexSafis` list or a dedicated `convert*ToUpdate` in `migrate_routes.go`, and the compat test files (`.ci` + `.conf`). `ai/patterns/bgp-family.md` Section 5b.
