# Repository Maintenance

**When:** adding or changing a feature, tool, gate, hook, runtime dependency, or generated file, looking up which check enforces a rule, or reporting development friction
**Severity:** blocking
**Related:** rule-format, writing, evidence, testing

## Directives

- **A change that adds or changes a surface future agents use, verify, document, or avoid MUST update the discovery path in the same work.** A private implementation change requires no prose only when it meets none of these triggers:
  - it changes user or agent behavior
  - it changes an architecture contract, an invariant, or a documented data flow
  - it makes existing documentation stale
  - it adds a discoverable surface, or sets a pattern future work MUST follow
- **Every feature that adds a new runtime dependency MUST register a `ze doctor` check so agents can verify readiness before starting the daemon.**
- **A generated file MUST NOT be edited. Edit the canonical source, then sync.**
- **Project behavior rules MUST belong in `ai/rules/` and project startup guidance MUST belong in `ai/INSTRUCTIONS.md`, so Claude, Codex, and other agents all discover the same rule through generated tool-specific files.**
- **The hook-to-rule mapping MUST be consulted BEFORE writing code, to comply in advance rather than to fix after rejection. For hook false positives and workarounds, see `plan/learned/HOOK-FRICTION.md`.**
- **A recurring problem pattern, repeated surprise, stale guidance, tooling friction, or wasted effort MUST be reported immediately, and you MUST say whether a new or changed rule would prevent it.**

## Discovery Updates

**Private refactors with no new surface still trigger this rule when they change a pattern future work MUST follow.**

**Each change below MUST carry the update named beside it, in the same work.**

| What changed | Required update |
|--------------|-----------------|
| Changed user-facing behavior | Specific file under `docs/`, with source anchors per `ai/rules/writing.md` |
| RFC support status (protocol behavior implemented, changed, or newly proven) | The matching `docs/features/rfc-status.md` row (Status, Implemented coverage, Remaining) with a source anchor to the producing `file:line`; reconcile `docs/comparison.md` and `docs/features.md` when the support level changes |
| Changed agent-facing command or contract | `docs/features/ai-first.md`, `docs/guide/mcp/overview.md` if MCP-visible, and `ai/rules/cli.md` if workflow changes |
| Architecture contract, invariant, or documented data flow | The owning `docs/architecture/` page or flow digest, with source anchors per `ai/rules/writing.md` |
| CLI command grammar or command availability | `ai/rules/cli.md` or `ai/rules/cli.md`, plus command validation docs if needed |
| New tool or native action | `ai/INDEX.md` Dev Tools or keyword map, plus the owning `docs/contributing/` or `docs/architecture/testing/` page |
| New verification gate or hook | `.claude/hooks/README.md`, including the `Check`/`Enforces` row in the table under the heading naming its Go source, and the rule the hook enforces |
| New doc or inventory checker | `docs/contributing/documentation-testing.md`, the owning `internal/le/<area>/actions.go`, and `ai/rules/writing.md` if policy changed |
| New test runner or format | `ai/rules/testing.md`, `ai/patterns/functional-test.md` if `.ci`, and the relevant `docs/architecture/testing/` page |
| New runtime dependency | The "Doctor Checks" section below, `docs/architecture/doctor-and-health-checks.md`, diagnostic code registration, and a `ze doctor` unit plus functional test |
| New registration or generated inventory | `ai/rules/evidence.md`, `ai/patterns/registration.md`, and registry-backed inventory checks |
| Existing documentation made stale by the change | Repair the stale claim in its current file and keep its source anchor valid |
| Recurring trap | `plan/journal/<class>.md` -- one row per occurrence; recurrence is the row count |
| New task category or search keyword | `ai/INDEX.md` (task navigation + keyword map) |
| Private implementation change that meets no trigger above and sets no pattern future work MUST follow | No prose update |

**An isolated rule or doc page that no existing navigation path links to MUST NOT be created. A rule that agents cannot discover is not a rule.**

1. **Where would an agent look first?** The `ai/INDEX.md` keyword row, the `ai/INDEX.md` task row, or both MUST be added or updated.
2. **What rule or gate prevents regression?** Name the current rule or gate when one covers the behavior. Update it when this change makes it wrong. A NEW `ai/rules/*.md` MUST wait for a recurrence that exposes a missing instruction no current rule or gate gives.
3. **What source of truth prevents drift?** A registry, generated inventory, YANG schema, or live binary output MUST be used. A static list MUST NOT be copied.
4. **What verification proves it?** The native action, unit test, functional test, hook, or doc validator that catches drift MUST be named.
5. **What docs explain usage?** The exact file and section MUST be named. Source anchors MUST be added for factual `docs/` claims.
6. **What journal record preserves the decision?** A row MUST first be appended to the matching `plan/journal/<class>.md` when a recurring trap is hit. The row is the record, never the fix: a blocking or related defect MUST still be fixed (`ai/rules/completion.md`).

- **An existing discovery surface MUST be used before a new mechanism is invented.** `ai/INDEX.md` ("Which surface answers a discovery question") maps each need to the native action or generated index that already answers it.

**If a new feature cannot be found from one of those surfaces or from `ai/INDEX.md`, the missing discovery link MUST be added before claiming completion.**

## Doctor Checks

- **A new runtime dependency MUST get a registered doctor check**, declaring its phase, order, component, dependency, platform, diagnostic code, and check function. The package, component, or plugin that OWNS the dependency MUST own the registration, the check function, and the unit test. `docs/architecture/doctor-and-health-checks.md` says which check each dependency kind needs, where each owner registers it, and which two tests it carries.

- **`internal/component/doctor` owns the runner, output contract, functional coverage through the user entry point, and checks that have no narrower owner.**
- **New runtime dependency checks MUST NOT be added by appending another direct call to the central `runChecks` list.**
- **Owner-specific registrations MUST NOT be added in `internal/component/doctor` just because the runner lives there.**
- **Internal plugins (preferred path) MUST declare doctor checks in `registry.Registration.DoctorChecks`.** The doctor runner bridges these at execution time via `checks_plugin_registry.go`. The check function uses `registry.DoctorCheckContext` (Tree and Platform as `any`) and returns `[]rpc.DoctorCheckDiagnostic`. Component is set automatically from the plugin name. See `l2tpauthradius/register.go` for the reference example.
- **Components that are not plugins** (e.g., appliance, web, SSH) MUST use `diagnostic.RegisterDoctorCheck()` from the owning package's init().

**If no plugin, component, backend, or command package owns the dependency, the check and unit test MUST stay in `internal/component/doctor`. An owner package MUST NOT be invented just to satisfy proximity.**

- **All doctor codes MUST use the `doctor-` prefix: `doctor-<component>-<condition>`.**
- **Every new code MUST be registered in `internal/core/diagnostic/codes.go` with title, description, and examples. The code MUST be explainable via `ze explain`.**

- **After implementation, the check MUST be verified as registered and explainable: `go test ./internal/component/doctor -run 'TestDoctorCoverageCodesRegistered|TestRunChecksExecutesRegisteredPluginCheck'`**
- **If you added a runtime dependency and no registered doctor check declares its `doctor-*` code, you missed the readiness check or its diagnostic metadata.**

**Linux-only checks MUST still have Linux-tagged tests, and the package MUST be covered by the QEMU integration target when new `//go:build linux` code is added.**

## Canonical Sources and Sync Direction

**A generated file MUST NOT be edited. Edit its canonical source, then run its sync command.**

| Canonical source | Generates | Sync command |
|------------------|-----------|--------------|
| `ai/INSTRUCTIONS.md` | `CLAUDE.md`, `AGENTS.md` | `./le ai skills-sync` |
| `ai/skills/*.md` | `.claude/skills/*/SKILL.md`, `.codex/skills/*/SKILL.md`, `.agents/skills/*/SKILL.md` | `./le ai skills-sync` |
| `ai/rules/points/<rule>/` | `ai/rules/<rule>.md` | `./le rules render-update` |
| A rule's points or manifest | `ai/rules/TRIGGERS.md`, `ai/rules/CORE.md` | `./le rules render-update`, then `./le rules condensed-update` |
| A rule's points or manifest | `ai/rules/INDEX.md` | `./le rules render-update`, then `./le rules index-update` |

- **Project-wide behavior rules, workflow rules, and agent rules MUST live under `ai/rules/`, not under a tool-specific home directory such as `~/.claude/rules/`.**
- **Tool-specific files are only for behavior that applies exclusively to that tool outside this repository.**
- **`ai/rules/*.md` are tool-agnostic and RENDERED from `ai/rules/points/<rule>/`. It MUST NOT be edited by hand. Edit the point file that carries the instruction, or the manifest that carries the title, the trigger and the reading order. Then run `./le rules render-update`. `.claude/rules/*.md` are Claude-specific originals and MUST NOT be used for shared Ze project behavior. These two directories are independent; neither generates the other.**
- **One instruction is one file, and its PATH is its id.** `ai/rules/points/<rule>/<slug>.md` holds one block of the rule, verbatim, behind a small frontmatter header. `ai/rules/points/<rule>/manifest.md` holds the rule's title, its `**When:**` trigger, its severity, and the ordered slug list the renderer concatenates. A point on disk that the manifest does not list is a hard render error, never a silent drop.
- **Second generation:** `ai/rules/INDEX.md` is generated by `internal/le/rules/index.go` from the RENDERED rule files' headings and summary lines. It MUST NOT be edited by hand; run `./le rules index-update`. To change a rule's one-line overview, edit the `when:` field in that rule's manifest, run `./le rules render-update`, then regenerate.
- **Second generation:** `internal/le/rules/artifacts.go` generates TWO artifacts from one parse of the RENDERED rule files. They MUST NOT be edited by hand; run `./le rules condensed-update`. To change what they contain, edit the rule's points, run `./le rules render-update`, then regenerate.

**Membership in `CORE.md` MUST NOT be edited, because it is never written down.** To make a rule always-on, change what the derivation reads: name it on rung 1 or 2 of the ladder in `ai/rules/rule-precedence.md`. A list of filenames in the generator would read identically until the ladder changed underneath it (`ai/rules/evidence.md`).

**Before editing any file listed in the "Generates" column above, STOP. You MUST find its canonical source in the left column and edit that instead.**

**Every native write action derives its output from the WORKING TREE, so in a shared checkout it can pick up other sessions' uncommitted sources. You MUST diff a regenerated artifact before you name it in a commit.** The bare `./le <area>` listing marks each write action explicitly.

The output is correct for the tree it read. It is wrong for the commit you are about to make, because that commit does not carry the sources the regeneration saw. What lands is a derived file that describes code nobody can see.

`internal/le/commit` refuses a commit whose regenerated artifact was derived from a tree holding sources the commit does not carry. That refusal is the only thing that catches this.

**The safe regeneration is HEAD plus your own files.** When an artifact is fully generated and yours was the only edit, `git show HEAD:<path>` written back over it restores the committed state, and the gate then agrees.

**The mirror image is worse and no gate catches it: committing a document that DESCRIBES uncommitted code.** A committed document that names a symbol still sitting in the working tree reddens `./le doc check links` for every session until that code lands. A check that you have not swept somebody's work IN does not check the other direction: prose you committed about work still sitting OUT.

**The `CLAUDE.md`, `AGENTS.md`, and skill mirrors are gitignored, so `git diff` can NEVER show drift for them.** `./le ai sync-check` compares them against a fresh generation; the session-start hook warns `generated agent files are stale` when a resync is needed. So drift in a mirror MUST be read from `./le ai sync-check` rather than from a
diff, and it MUST be fixed with `./le ai skills-sync`. `ai/rules/<rule>.md` is the one generated rule surface that IS tracked, so `git diff` shows its drift, and `./le rules render-check` reaches the same verdict without writing.

**A build MUST NOT write its own bookkeeping into the artifact it publishes.** A record of what a run did belongs to the checkout that ran it. The artifact holds what a reader came for and nothing else, so a build MUST resolve a record path from the repository root and never from the output root, whatever `ZE_REPO_ROOT` names at the time.

**The tell MUST be checked by hand rather than waited for from a gate: a path a build writes is joined to `paths.Output`, or to a root a caller supplied, while the thing written is evidence about the RUN rather than content for a reader.** The failure is quiet in the only place that matters, because the record is correct, the build succeeds, and the file is served. `ZE_REPO_ROOT` makes the route reachable from every sibling checkout, so a path that is correct in the ze tree publishes tooling state the moment it is pointed at a published one.

## Hook-to-Rule Mapping

- **Every registered native check MUST declare the rule point it enforces**, with a `// ze point: <rule>/<section>/<slug>` line in its Go doc comment, or `// ze point: none -- <why>` when nothing written binds it. `nativeHookActions` in `internal/le/hookruntime/runtime.go` is the authority for what is registered; `./le rules gate-map-report` refuses a binding on an unwired function and a registered check with no binding.
- **The `Check` and `Enforces` columns of the tables in `.claude/hooks/README.md` MUST name exactly the registered checks and the rule stems their bindings name.** They are a gated mirror rather than a second roster: `hookTableProblems` in `internal/le/rules/hooktable.go` compares them against the Go registry. What each check triggers on and what it does is documentation, and it sits in the remaining column of the same table.

- **Changing a check MUST keep four things in agreement: the Go function in `internal/le/hookruntime`, its entry in `nativeHookActions`, its `// ze point:` binding, and its published row.** `./le hook-check unit` MUST run afterwards, an intentional fixture change MUST update the owned native golden in the same change, and the "Discovery Updates" section above MUST also be satisfied.

- **Every session marker is keyed by session ID, and every native hook consumer MUST resolve that ID through `internal/le/hookruntime/session.go`.** An absent ID and an invalid ID are distinct results: an invalid ID MUST be rejected rather than rewritten, and a dot entry MUST NOT be accepted.
- **A hook MUST NOT persist `$ZE_SESSION_ID`.** Native session and spec lifecycle commands resolve the current harness session themselves. `./le hook-check session-id` locks this behavior.

- **A `UserPromptSubmit` reminder that MUST land in the context writes to stdout; a banner that MUST cost no context tokens writes to stderr.** The reminders fire on every turn, so each one MUST stay a single line. `.claude/hooks/README.md` lists the lifecycle actions and their events.

## Gate Population

**Where a gate is named, you MUST state what it cannot see, not only what it
checks. A rule that says "also run the other check" is followed on the day
somebody remembers it. A rule that names the blind spot is followed by whoever
reads the gate and wonders what its green covers.**

**A check that reads another artifact's STRUCTURE MUST anchor on a marker that
artifact guarantees, never on a position inside it, and MUST resolve the
indirection that artifact's own format permits. A positional window stops seeing
data the moment the data moves past it. A reader blind to indirection reports
"not wired" for a subject that is wired. Both failures present as a verdict
about the subject when they are a verdict about the reader, which is why neither
is caught by re-running the check.**

**The agreement MUST be pinned by feeding a real, canonical instance of that
artifact through the reader in a test.** A reader and the artifact it parses
drift apart silently whenever nothing exercises the two together, and the drift
surfaces as a wrong answer rather than as an error. Where the read can fail
outright, the failure MUST stay distinguishable from a value that is legitimately
absent (`ai/rules/evidence.md`, "Fail-Closed Guards").

**When a second gate reuses a check another gate already runs, it MUST supply
that check the same INPUT SHAPE the first gate supplies, and the shared shape
MUST be stated where the check is defined.** Sharing one implementation is what
keeps two gates from disagreeing about a rule. It does not keep them from
disagreeing about the SUBJECT, because a check reads its subject through the
values its caller passes. A caller that builds those values differently gets
different answers out of identical code, and the sharing hides it: both gates
cite the same function, so the difference looks impossible.

**The failure MUST be treated as blocking rather than cosmetic, because a
later gate that refuses what an earlier gate allowed leaves no way forward.**
The earlier gate has already passed, its verdict cannot be revisited, and the
later refusal names a rule the author did not break. An exemption a check grants
is part of its contract exactly as much as the violation it reports, so an input
shape that silently voids an exemption converts a permitted subject into a
blocked one.

**A shared check MUST be exercised through the NEW caller, with the values that
caller really constructs.** A test that calls the check directly, or that
rebuilds its input by hand, proves the check and not the wiring. Reusing a
subject the check is known to exempt is what makes the test discriminate: an
input shape that agrees with the original caller reports the exemption, and one
that does not reports the violation.

**Where the shape is derived rather than passed through, the derivation MUST NOT
import context from outside the artifact under test.** Widening a value until
the check accepts it can make the check depend on the environment the tool runs
in, which turns one gate's verdict into a property of the machine.

**`./le repository tracked-build check` is the only gate that compiles what git holds,
and it compiles no `_test.go`. Its green therefore says nothing about the test
build. Before you treat work as committable, you MUST also compile the test
binaries of every package you touched, without running them.**

**A gate that refuses a COMMIT MUST derive its verdict from the paths that
commit names, never from the state of the working tree or of a file on disk. A
gate that reads the repository answers a question nobody asked: it refuses work
that is correct, for a fact about somebody else's work, and it does so in a
checkout several sessions share.**

The failure is not a false positive about the subject. It is a verdict about the
wrong subject, and the author it blocks usually has no permitted action: the
offending state is not theirs to commit, not theirs to delete, and not theirs to
carry.

Two readings of the same kind of ledger, measured on 2026-08-24, show the whole
distinction:

| Reader | Derives from | Effect |
|--------|--------------|--------|
| `CheckCommit` in `internal/le/testweakened` | recomputes the weakenings of the paths the commit NAMES | refuses only a commit that actually weakens a test |
| `rfc_changed_problems` | reads `test/rfc-changed.md` from DISK | one open row refuses every commit in the repository until its author lands it |

Both files are per-commit by their own contract. Only the second turns that
contract into a repository-wide lock. On the day it was measured it held a
227-path change hostage to a five-line assertion in another session's package,
and the blocked author's three available moves were all forbidden: adding a
foreign hunk, deleting an owner-approved row, or waiting.

**The same defect wears a second face: a gate that infers INTENT from a
by-product instead of reading the act.** `spec_audit_problems`
(`internal/le/commit`) asks whether a journal row names the claimed
spec, and treats that as the spec's closure. A row naming a spec is mandated for
every defect an agent finds, and an agent finds most of them inside its own
spec, so the ordinary mid-spec commit is read as a closure and refused for
lacking a closure section. The act itself is in the same function's arguments:
`remove_paths` already says whether the commit removes `plan/<spec>.md`, and a
commit that adds no learned summary and removes no spec closes nothing.

**Before adding or changing a commit gate, its doc comment MUST answer one
question: what does this refuse that the commit did not do?** A gate that cannot
answer it is reading the world in place of the diff.

Three consequences follow, and each MUST hold:

- **A shared per-commit ledger MUST be keyed to the commit that carries it.** A
  row about file A MUST NOT refuse a commit touching only file B.
- **An escape hatch MUST reach the gate that fires.** `--review-override`
  clears `review_gate_problems` and `spec_audit_problems` fires afterwards, so
  the documented escape does not reach the refusal an author actually meets.
  A gate with no reachable escape and no permitted action is a stop, not a gate.
- **A gate MUST NOT be satisfiable only by a statement its author knows to be
  false.** Where the routes past a refusal are gaming it (releasing a spec claim
  so the gate returns early) or asserting something untrue (filling a
  verification section for a spec that is not verified), the gate is asking the
  wrong question, and the author's correct move is to leave the work
  uncommitted and say so (`ai/rules/completion.md`).

**A change that moves files INTO a tree a gate reads, or that widens what a gate
requires, MUST leave that gate green over the whole affected population, in the
same change.** A gate's population is derived from the tree, so it grows the
moment a file lands inside it. Code that was correct where it lived can be red
the instant it moves, without a line of it changing, and a requirement that
ratchets makes every file already in the population red at once.

**The red that results belongs to nobody, which is what makes it expensive.** It
is charged to the next author who prepares a commit, and it names a rule that
author did not break, in files that author did not write. Every session after
the change pays to rediscover the same cause, and the gate that was built to
report a real defect now reports only its own migration.

**A gate that runs on WRITE turns an unmigrated file into a file nobody can
edit.** Where the ratchet is enforced after each edit rather than at commit time,
a file that does not yet satisfy the new requirement refuses every edit made to
it, whatever that edit was about. The block is invisible from the side that
authored the change, because anything created after it already conforms.

**So the change MUST carry the migration, and the migration MUST cover the
population rather than the examples the author happened to open.** Deriving the
population from the same source the gate derives it from is the only way to know
the two agree. A sample that passes is not evidence about the rest.

**Where a record states what a gate REPORTED under an older name or an older
requirement, that record MUST NOT be rewritten to satisfy the new one.** It
describes a run that happened. Migrating it forward replaces a fact with a claim
that was never true, which costs more than the red it clears.

**A gate that judges an INTENT MUST key on the artifact only that intent
produces, never on one that merely accompanies it.** An accompanying artifact is
a proxy, and a proxy is wider than the thing it stands for: it matches the
intended act and every other act that happens to leave the same trace. The gate
then refuses correct work, and each refusal costs a session the time to
discover that the rule it seems to state is not the rule it enforces.

**The tell is a gate that fires on work nobody would describe the way the gate
describes it.** When that happens, MUST ask what the intended act would
uniquely leave behind, and key on that instead. Widening the proxy to admit the
case in hand is the wrong repair: it trades one wrong population for another.

**A carve-out MUST be tested from both sides before it lands.** A test that the
excluded case now passes proves only that the gate got weaker. The test that
matters asserts the gate still catches what it was built for, because a
carve-out written from the refused case alone is indistinguishable from
disarming the gate.

**A gate whose input is a per-commit scratch file MUST be cleared by the commit
that used it.** Such a file states something about ONE commit, so a stale entry
is a false statement about the next one, and it refuses that commit for a
reason its author cannot act on. Leaving the file dirty moves the cost onto
whoever commits next, which is the property that makes it worth a rule.

## Ze Project Knowledge

- **Family registration** is dynamic via `PluginRegistry.Register()` -- never enumerate, validate format only.
- **Config pipeline**: File -> Tree -> `ResolveBGPTree()` -> `map[string]any` -> `reactor.PeersFromTree()`. Files: `internal/component/bgp/config/{resolve,peers}.go`, `.../reactor/config.go`.
- **Linter hook**: `postFormatGo` in `internal/le/hookruntime/postwrite.go` runs gofmt, `goimports -format-only`, and changed-code lint on Edit/Write. Imports are not auto-removed, so add an import and its use in the same edit.
- **Arch-0**: 4 components (Engine, ConfigProvider, PluginManager, Subsystem). Subsystem != Plugin (BGP daemon = subsystem; bgp-rib/rs/gr = plugins). Stream system = pub/sub backbone (`internal/component/plugin/server/dispatch.go`). Interfaces in `pkg/ze/`.
- **YANG choice/case**: `mandatory true` and inner-choice exclusivity NOT enforced by the walker. Plugins using `choice` add Go-side validation in their parser. `ze config validate` does not invoke `OnConfigVerify`.
- **Constants for command/status names** -- literals catch typos at compile time. Editor commands: `internal/component/cli/model.go`. Plugin status: `plugin.StatusDone`/`StatusError`.
- **Proximity**: `bgp/handler/` is a middleman; handlers belong in `bgp/plugins/`. ALL RPCs need YANG. See `ai/rules/plugins.md`.
- **LSP** at session start for Go nav -- more precise than grep for call chains and interface impls.
- **Inventory**: `./le inventory [--json]` imports `plugin/all` and queries real registries. Use for plugin counts, RPC totals, family coverage.
- **SDK type aliases** (`pkg/plugin/sdk/sdk_types.go` re-exporting `rpc.*`) are intentional -- external plugins import only `sdk`. Not identity wrappers.
- **No filtered/noexport route tracking** -- Ze does not store import-filtered or export-filtered routes (unlike BIRD's "import keep filtered on"): the RIB pipeline has scope keywords (sent/received/sent-received) and filter stages, but no "filtered" scope. The birdwatcher-compatible endpoints `/routes/filtered/{name}` and `/routes/noexport/{name}` return empty lists for compatibility; if filtered tracking ever lands, point them at the real store.
- **Gokrazy appliance owns process lifecycle** -- ze deploys as a gokrazy appliance: no systemd, no init system, no package manager. Any external process ze depends on (VPP or future dependencies) MUST be exec'd, supervised, and cleaned up by ze itself; ze MUST NOT be designed around an OS-level process manager.
- **Stress tooling is native Go**: `internal/le/integration/stress.go` owns stress orchestration, and the BGP UPDATE stream is generated inside `ze-test peer --mode inject`. Extend the Go injector for a new scenario with a pool-friendly byte builder, one pre-allocated buffer, one TCP writer, and a keepalive goroutine. Run it through `./le integration stress`.
- **CLI dispatch discoverability gaps**: (1) no one-shot command against a RUNNING daemon (`ze cli -c "summary"` shape). `ze show` and `ze run` use SSH (`sshclient.ExecCommand`) internally but expose no shell one-liner. The offline-config half is covered by `ze config show <file> [path...]`. (2) `ze help --ai --api` prints YANG RPC names (`ze-bgp:overview`), not the dispatch strings users type. (3) No way to list the Dispatcher's match keys. `reactor.ExecuteCommand()` accepts strings undiscoverable without reading source. The highest-value fix is the one-shot daemon command (SSH port 2222, credentials from the zefs database).

- **A mistake-log entry MUST be one line: the lesson, then the rule it points at.** The full root cause MUST live in the linked `plan/journal/<class>.md` row's Fix cell, never here.

- **"Linux-only tests cannot run on this macOS host" is false** (RECURRING, ZERO TOL). Mark kernel-dependent `.ci` cases with `option=needs-linux`, use `./le qemu netns-test suites <names>` for a focused pass, and use `./le qemu all-tests` for the full runtime-kernel guest proof. A Linux-only test that fails on native Darwin needs the correct marker and a QEMU run, never an "environmental" dismissal. `ai/rules/platform-linux.md`.
- **Feature not wired** (RECURRING, ZERO TOL). Unit tests != wiring. The user entry point MUST be named. `ai/rules/completion.md`.
- **Daemon command without offline CLI** (sysctl-0). Every `CommandDecl` plugin MUST have a `cmd/ze/<name>/` offline entry point.
- **Wrong production path** (rib-04). ALL implementations MUST be grepped; the consumer's call chain MUST be traced.
- **Count-only test assertions** (addpath-rib). Assertions MUST be on content (keys/values), not `Len()`.
- **Wrapper struct pattern** (alloc-4). Raw bytes and existing iterators MUST be passed. Data MUST NOT be wrapped in accessor types.
- **Tests-pass != done** (RECURRING). Tests are step 10 of 12. Work MUST continue to docs/spec/summary/audit. `ai/rules/quality.md`.
- **Mechanism-not-behavior test** (prefix-limit). The AC MUST be asserted, not a code-path proxy. No-op passes = wrong test. `ai/rules/testing.md`.
- **"Pre-existing" failures** (RESOLVED). Blocks your goal: it MUST be fixed now. Does not: spec it, close the work in hand, ask Thomas whether that spec runs. `ai/rules/completion.md`.
- **Plugin placement anchor bias** (jsonrpc). "Delete the folder" test. Cross-cutting -> `internal/component/`. Domain -> `bgp/plugins/`. Infra -> `internal/core/`.
- **Docs from assumption** (RECURRING). Source MUST be read before any factual claim. `ai/rules/writing.md` Source Anchors.
- **Spec deleted without committing** (lg-overhaul, ZERO TOL). TWO commits MUST be made: (A) code+spec, (B) `git rm` spec + add summary. `ai/rules/planning.md`.
- **Reinventing repo contents** (lg-overhaul). Existing code MUST be grepped before writing new infra; `third_party/` and components often already have it. `ai/rules/architecture.md`.
- **Spec claimed complete with gaps** (lg-0..4). Learned summary with "future X" = spec NOT done. Each AC MUST be audited. `ai/rules/completion.md`.
- **Stale deferrals** (redist-phase2). Code MUST be grepped before a phase-N spec is created from open deferrals. `ai/rules/planning.md`.
- **Worktree copy into main** (ZERO TOL). Work MUST be committed in the worktree, and it MUST reach main only via merge or cherry-pick. `bashWorktreeCopy` in `internal/le/hookruntime/bash.go` enforces.
- **Same-day blocker fix** (cmd-4, RECURRING). A real adversarial review MUST race on reactor code, grep renamed-name consumers, grep sibling call sites, and break production to confirm the `.ci` test fails. `ai/rules/quality.md`.
- **Substring collision in bulk edits** (iface-tunnel). The longest prefix MUST be matched first, or non-name context MUST be added. Mangled names MUST be grepped for afterward.
- **Vendor != upstream** (iface-tunnel). Behavior MUST be verified against `vendor/<lib>/`, not upstream docs. The vendor path MUST be cited in the spec.
- **Naive reconciliation drops live state** (iface-tunnel). The new config MUST be diffed against the previous config, and the delta MUST be acted on. `previous` MUST be passed explicitly.
- **Invented config shape** (iface-tunnel). Existing `*-conf.yang` files MUST be grepped for the closest analog before new endpoint shapes are defined.
- **Scratch `.go` in `tmp/`** (iface-tunnel). `go test ./...` walks `tmp/`. Research agents MUST use `.txt` or build-tagged dirs.
- **CLI grammar from container nesting, not wire method** (as112-cli-audit). Operator-facing command words come from the YANG `container` tree; `ze:command "ze-X:Y"` is the INTERNAL RPC name and is deliberately different (e.g. `ze-bgp:peer-teardown` = command `request peer teardown`). Command syntax MUST NOT be inferred from wire-method names. Top-level operational verb is `request` (`request <object> <action>`); reads are `show`/`monitor`. `ai/rules/writing.md`.
- **ExaBGP migration sync** (exabgp-compat-sync). When ExaBGP adds a new SAFI or route type, three things MUST be updated: (1) `exabgp.yang` schema container, (2) `flexSafis` list or a dedicated `convert*ToUpdate` in `migrate_routes.go`, (3) compat test files (`.ci` + `.conf`). `ai/patterns/bgp-family.md` Section 5b.

## Your Own Mistakes

**The rule you write MUST state the root cause and the general practice that
prevents it. It MUST NOT carry the example, the file, or the specifics of the
occurrence that produced it.**

**When preventing the recurrence needs code rather than prose, you MUST home
that work as a spec under `plan/future/` in the same session. A rule binds
whoever reads it; a gate binds everyone.**

## Friction Reporting

**A friction report MUST state five things: what happened, why it is likely to recur, what it cost, whether a rule change would prevent it, and the specific fix. Any category below MUST be reported.**

| Category | Examples |
|----------|----------|
| **Problem pattern** | The same mistake, rejected edit, missing wiring, misunderstood boundary, or unexpected failure appears more than once or is likely to recur |
| **Rule gap** | Existing rules did not say what to do, gave conflicting guidance, or made the wrong path look valid |
| **Missing docs** | Had to investigate something a page was owed: file purpose, data flow step, registration pattern, gate behavior |
| **Stale info** | Rule or doc references deleted/renamed files, describes a pattern the code no longer follows |
| **Tooling friction** | Hook rejects valid code, linter config does not match rules, native action behaves unexpectedly |
| **Wasted effort** | Searched in the wrong place, duplicated existing functionality, misunderstood a layer boundary |

- The pattern MUST be reported as soon as you can describe it. You MUST NOT wait until the end of the session.
- **Reporting in chat is not filing.** Chat scrolls away and the next session never sees it, so hook and tooling friction is not reported until it is written to `plan/learned/HOOK-FRICTION.md` with the five parts above; a finding you only pass to the next agent in a handoff is folklore, not a record.
- If the user task is still in progress, work MUST continue after reporting unless blocked or the rule change would alter scope.
- **When a pattern recurs, a row MUST first be appended to the matching `plan/journal/<class>.md`.** A rule MUST be added or updated only when the recurrence exposes a missing actionable instruction and no current rule or gate covers it.
- Blocking and related defects MUST still be fixed. This recording order changes no defect-fix obligation.

The following MUST NOT be treated as friction:
- Things that are simply unfamiliar before reading the relevant docs.
- Intentional deviations already documented in specs or rationale files.
- One-off issues that will not recur and expose no rule gap.
