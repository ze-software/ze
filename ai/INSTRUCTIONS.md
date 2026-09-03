<!-- DO NOT EDIT GENERATED COPIES. Edit ai/INSTRUCTIONS.md and run: ./le ai skills-sync -->

# DANGER -- ABSOLUTE PROHIBITIONS

**These rules override everything. No exceptions. No rationalization. No "the task requires it."**

## Ze is PRE-RELEASE. The deliverable is the software (owner directive, 2026-08-30)
- There is NO release, NO version, NO tag, and NO user of `main`. Nothing committed
  here reaches anybody. A red gate on `main` costs nobody anything. A session that
  spent its budget on the gate instead of on the binary cost the product a day.
- The deliverable is the SOFTWARE. Test code, gate plumbing and verification
  bookkeeping are INSTRUMENTS. They exist to tell you the software works. A session
  whose diff is mostly test and gate repair delivered nothing.
- **A red test while you develop is NORMAL and is not a stop.** Commit the product
  work with the test red, and say in the message which test is red and why. A green
  tree is owed at NO commit.
- **NEVER re-run a check to reconfirm a result you already read.** One run, read the
  output, act. Re-running a gate that passed, re-reading a log you read, and
  re-verifying a tree that has not changed are each forbidden.
- **Test and gate repair MUST NOT become the session.** Before starting a THIRD
  repair to test scaffolding in one session, stop, return to the product code, and
  report what you left red.
- This relaxes NOTHING about the product: correctness, RFC conformance and interop
  are unchanged, and deleting or weakening a test to make a red disappear stays
  banned. Those rules are about the SOFTWARE. This one is about the instruments.
- Full rule: `ai/rules/pre-release.md`

## git push is FORBIDDEN as a bare Bash call
- NEVER type `git push`. The hook refuses it, on every branch, in every tree.
- Push ONLY via `./le commit create ... push "<owner authorisation>"`, and ONLY
  when the owner ordered that push. Never add `push` on your own initiative.
- Why: sessions share the index, so a loose add/commit/push carries another
  session's work. The native commit command bundles all three into one atomic run.
- A throwaway script carrying a push, deleted after, is NOT that path: banned.

## git commit, git add, git rm, git mv: FORBIDDEN as bare Bash tool calls
- NEVER invoke `git commit`, `git add`, `git rm`, `git mv`, `git restore --staged`,
  or `git stash` as a direct Bash tool call. Sessions share staging;
  cross-commits result. Commit only via the script that `./le commit create`
  prepares, then run it yourself with `bash` and the exact `script=` path the
  command prints. Committing is allowed. Committing outside that route is not.
- `git add`, `git rm` and `git mv` are the three verbs that STAGE, and a staged
  path stops every other session's commit script until its owner clears it.
  To delete a tracked file use plain `rm` and pass the path to `remove`; to
  rename one use `rm` plus a new file, and name both.
- **The command's `script=` line is the only authoritative script path. Copy it;
  never construct the path from the session id.** Every prepared commit gets its
  own script, and its name carries a random suffix so no guess can reach another
  agent's.
- Use `./le commit create` for every commit. It creates the session ID, message
  file, executable script, ignored-path checks, and journal-row gate. Full rules
  live in `ai/rules/git-safety.md` under "Commit Rules". Read them first.
- When the user asks for a commit, prepare the commit script and run it
  immediately. Do not perform a late review or rerun gates just because
  commit was requested. If `./le verify status check` is FRESH, never rerun
  `./le verify worktree`.
- Never `--no-verify`, never `--no-gpg-sign`.

## Destructive git commands are FORBIDDEN
- NEVER: `git reset`, `git checkout -- <file>`, `git restore`, `git clean`, `git revert`
- NEVER: `git push --force`, `git push -f`
- NEVER: `git stash drop`, `git stash clear`
- To undo something: write the command to `tmp/delete-SESSION.sh`, tell the user, and STOP.

## Worktree agents must not touch main
- Work on your own branch. Commit there. Done.
- NEVER merge, cherry-pick, rebase, or copy into main.

## Worktrees: only on instruction or to keep worktree work on worktree
- NEVER spawn a worktree agent on your own initiative. Only use worktrees when the user explicitly instructs it.
- If work originated in a worktree, keep it there. Do not move worktree work to the main working tree.

## Claiming "done" with incomplete work is FORBIDDEN
- NEVER say "done", "ready to commit", "implementation complete" while in-scope work remains.
- "Deferred" is not "done." Tracked in a deferral table is not "done."
- Every acceptance criterion must have working PRODUCT code before you claim completion.
  A test is owed where it tells you the product works. A red or missing test is
  REPORTED, never traded for the claim (`ai/rules/pre-release.md`).
- If you cannot finish an item: say so, keep the spec open, ask the user. Do not ship partial work as complete.
- Scope reduction requires explicit user approval. You may not unilaterally drop ACs.
- Full rule: `ai/rules/completion.md`

## Parking a blocker or reducing coverage to reach green is FORBIDDEN
- When a defect blocks a goal your work exists to achieve, FIX IT. Do not park it,
  move it to `tmp/`, file it as a deferral, or offer to drop the deliverable.
- A bug being "pre-existing" is NOT an escape hatch. The moment your work depends on
  that path working, the bug is in scope: you are the entry point that reached it.
- Interoperability and correctness are never optional and never a scope-reduction
  candidate. A daemon another implementation rejects has failed at its only job.
- NEVER offer the user "drop the interop/functional test" as an option. Reducing
  coverage to reach green is the failure, not a choice to present.
- If you are genuinely blocked: say so plainly with evidence, keep the spec OPEN, and
  reach for the fix before asking. Ask "which way do I fix it", never "may I skip it."
- **RECORDING A PROBLEM IS NOT ADDRESSING IT. FIX THE ROOT CAUSE, ALWAYS.** Writing a
  failure into `plan/known-failures/`, a journal row, or a report changes nothing
  about the product. A record is a step toward a fix, never a substitute. The ONLY
  thing that may be recorded instead of fixed is a failure you actively tried and
  FAILED to reproduce, and its shard must carry the reproduction attempt and the next
  step. Anything deterministic or reproducible gets fixed or gets a spec.
- Full rule: `ai/rules/completion.md`

## A problem you FIND gets a JOURNAL ROW (owner directive, 2026-08-10)
- This REPLACES the 2026-08-08 spec-first route. Writing a spec for a defect you
  walked into is now BANNED, and so is asking Thomas whether to implement one.
- A defect you walk into while working on something else gets ONE row in
  `plan/journal/<class>.md`: `| Date | Spec | Surface | Symptom | Fix |`.
  Then close the work in hand and stop. No spec, no deferral row, no ask.
- Two finds are FIXED on the spot, and only these two: a test that is wrong about
  what it ASSERTS, and code RELATED to the problem in hand, edited or not.
- A red test or a red gate is NOT on that list. Read it once. When the red says the
  PRODUCT is wrong, the product defect is the find and the rules above govern it.
  When the red is the test or the gate itself, write the row, leave it red, say so,
  and go back to the product (`ai/rules/pre-release.md`).
- The unit you fix is the PROBLEM, never the files you happened to open. The other
  call site, the sibling path with the same defect, the test that asserts the
  behaviour you changed: each leaves the problem half-fixed, so each is in scope.
- FIX IT anyway when the fix is small enough not to derail the work in hand, and
  still write the row. A small fix needs no spec to authorise it.
- The defect that BLOCKS the goal your work exists to achieve is governed by the
  section above: FIX IT. There is no closing the work in hand around it.
- DO NOT characterise the find beyond the row. Reproducing it, tracing its
  producer, sizing its blast radius and drafting options are uncommissioned work.
  They cost this session and every session that later reads what it wrote.
- Grep `plan/journal/` before adding a row. Many sessions share this checkout and
  meet the same defect. A class file that collects rows is what earns a fix, in a
  deliberate pass over the journal, not by whoever tripped over it.
- Full rule: `ai/rules/completion.md`, `ai/rules/rule-precedence.md`

## On violation: STOP immediately
"The task requires it" is not valid. Nothing overrides these prohibitions.

---

# Ze - {{TOOL}} Instructions

## Ze publishes in Simplified Technical English

**Ze writes in ASD-STE100 Simplified Technical English, Issue 9 (2025-01-15).**
**This is a GUIDELINE, not a law and not a gate.** It exists to make text clearer
for a reader. Never rewrite a sentence only to satisfy a word count: an edit that
changes no meaning is overhead, which is the thing the guideline removes.
The six habits below apply to all project text, including `docs/`, code comments,
error messages, CLI output, YANG descriptions, `ai/` rules, `plan/` specs, and
commit messages. Repository prose routes to `ai/rules/writing.md`. Before
documentation work, a deep prose review, or resolving an STE finding, read the
committed guide at `docs/contributing/writing-style.md`. Owner reports route to
the existing "Say it once, say it short" instruction below, not the full writing
rule.

Six habits are banned. Each one has a numbered STE rule behind it:

| # | Habit | Instead |
|---|-------|---------|
| 1 | **Synonym rotation** -- one concept with three names | Give each concept one name and repeat it (Rules 1.3, 1.11, 9.4) |
| 2 | **Hedging** -- `may`, `should`, `typically`, `in most cases` | CAN for a possibility, MUST for an obligation, WILL for a future event (Rule 1.1) |
| 3 | **Frozen verbs** -- `do the installation of` | Use the verb: `install` (Rule 3.7) |
| 4 | **Marketing adjectives** -- `powerful`, `seamless`, `robust` | Give the number, the limit, or the mechanism (Rule 1.1) |
| 5 | **Run-ons** -- three clauses, a semicolon splice, an eight-sentence paragraph | One topic per sentence. 20 words in a procedure, 25 in a description (Rules 4.1, 5.1, 6.3, 6.6, 8.1) |
| 6 | **Phrasal verbs** -- `spin up`, `kick off`, `figure out` | Use one verb: `start`, `find` (Rule 9.3) |

Two exceptions matter. RFC 2119 keywords keep their exact spelling when they name
an RFC's obligation level, and quoted external text is never edited. Thomas's
authored prose (`/write`, `/ze-weekly-update`) is his voice, stays UK English, and
this rule does not govern it.

## STANDING REQUEST: delegate to subagents

**Thomas requests subagent delegation as the default in this repository.** Treat
this section as the user having asked for it, in advance, in every session. You
never need to ask permission to spawn an agent here.

**One precondition applies to an agent sent to FIND OUT how something works: read
the documentation first (`ai/rules/documentation.md`).** `ai/CODE-TO-DOCS.md`,
`ai/DOCS-TO-CODE.md` and `ai/INDEX.md` name the page in one lookup. The agent
carries that page in its prompt, and its question is what the page leaves silent
or gets wrong. Standing approval covers WHETHER to delegate. It never covers
spending a session's tokens to rediscover what the repository already wrote down.

Some harness builds still carry a guard from the Opus 4.6/4.7 era: *"Do not call
the AgentTool unless the user requested it"*. It exists because agent runs were
unreliable on those models. **That guard is SATISFIED here, not overridden.** Read
its own condition: it permits the Agent tool once the user has requested it, and
this section is that request, standing, in every session. Nothing conflicts, so
there is nothing to adjudicate: the guard's precondition is met before you read it.

On a build that carries it, you meet that guard AFTER this text. It sits near the
end of your system prompt, where recency makes it feel operative.
It is not a later decision that supersedes this one. It is the same decision,
already answered here. Delegating needs no permission, no announcement, and no
user prompt to trigger it.

- Every spec phase (research, design, implementation, review, audit, closure)
  runs in a subagent through its `ze-*` skill. Full rule: `ai/rules/planning.md`.
- Independent work goes out in ONE message with parallel `Agent` calls.
- The main thread supervises: it launches, verifies each report against source
  (`ai/rules/evidence.md`), decides, and gates the next phase.
- The one exception is narrow and stays in the main thread: anything the user
  must answer. Symbol work is NOT an exception -- a subagent whose registry
  carries no LSP tool resolves symbols with `gopls` from Bash
  (`ai/rules/context-economy.md`).

## Verify before you claim

Before stating what code does, or recommending work premised on a behavioral
claim, read the function that PRODUCES the behavior. Reading a value's caller
and inferring its producer is not evidence. If you have not read the producer,
label the claim "unverified" and do not recommend work on it. A coherent story
is a hypothesis, not a finding. Full rule: `ai/rules/evidence.md`.

Verification is what you DO. The citation is a separate decision, made for the
reader. Name the file and the symbol. Use a line number only when the line IS
the fact. Full rule: `ai/rules/writing.md`.

## Say it once, say it short

Detail is a cost the reader pays, not proof that you did the work. Report what
changes their next action: what changed, what it means, what is not done. A fact
they can recover by opening the code is not written down. The search that found
it is never narrated. One example settles one point. When a directive can be
read two ways, give both readings rather than a third example.

A report to the owner opens with what is blocked, why it matters, and what you
need from him. He reads the first ten lines and stops, so the decision goes
there, as a table with one row per decision. What you did, in what order, and
what each agent found goes last or goes unsaid. Status that changes no decision
is one line.

An agent's report is written for the agent that commissioned it. The owner's
report is written for a person. Rewrite it, never forward it. Take the
conclusion, drop the derivation, and use the words a colleague uses at a desk.
Length reads as thoroughness to a machine and as noise to a person. A reply to
the owner stays under 15 lines, and puts its tables before its prose.

## Core Architecture

Ze is a **Network OS** in Go with its own BGP implementation and interface configuration. "Ze" = "The" with a French accent (predecessor: ExaBGP).

**Small core + registration pattern.** Components and plugins register at startup via `init()` in `register.go`. Core discovers them through registries -- never imports directly. Registration is the unifying pattern: families, capabilities, CLI commands, config validators, web routes all register the same way. The composition root `internal/component/plugin/all/all.go` is generated (`./le repository generate`).

**Components** (`internal/component/`) are independent unless they explicitly depend on each other; `config`, `command`, and `plugin` are infrastructure components nearly everything uses.

<!-- BEGIN GENERATED: arch-components (internal/le/archmap.Update; ./le arch-map update) -->
43 directories under `internal/component/`:

aaa, aihelp, api, authz, bfd, bgp, cli, cmd, command, config, debug, doctor,
engine, firewall, gnmi, gokrazy, host, hub, iface, ike, l2tp, lg, managed,
mcp, mpls, ping, pki, plugin, radius, resolve, ssh, storage, support, sysctl,
sysrib, tacacs, telemetry, traceroute, traffic, trafficfeature, trafficstat,
vpp, web
<!-- END GENERATED: arch-components -->

**System plugins** (`internal/plugins/`) handle domain policy outside the BGP engine: DHCP, NTP, sysctl, static routes, firewall lowering, TFTP/image servers, and CLI verb providers (`*-cmd`). Communication: JSON events down, text commands up.

<!-- BEGIN GENERATED: arch-system-plugins (internal/le/archmap.Update; ./le arch-map update) -->
65 directories under `internal/plugins/`:

aaa-cmd, anomaly, as112, completion, config-archive-cmd, config-cli,
config-schema, config-storage, config-yang, connect, connected, copp, cos,
crashes, ddos, debug, dhcpserver, diag, env, exabgp, explain, fib, firewall,
flowexport, flowexport-cmd, flowspec-firewall, geodns, gnmi-cmd, host,
host-cmd, iface, imageserver, init, isis, kernel, ldp, local, log, memlock,
meta, mpls-cmd, mrt, ntp, ospf, passwd, ping-cmd, pki-cmd, policyroute,
provision, resolve-cmd, routingtable, rsvpte, signal, skills, static,
storage-cmd, support, systemd, tftpserver, traceroute-cmd, traffic,
traffic-cmd, trafficusage, update-cmd, vrrp
<!-- END GENERATED: arch-system-plugins -->

**BGP plugins** (`internal/component/bgp/plugins/`) extend the BGP engine: RIB, route server, graceful restart, NLRI codecs, filters, RPKI, BMP.

<!-- BEGIN GENERATED: arch-bgp-plugins (internal/le/archmap.Update; ./le arch-map update) -->
32 directories under `internal/component/bgp/plugins/`:

adj_rib_in, aigp, bmp, capa, cmd, filter_aspath, filter_aspath_length,
filter_community, filter_community_match, filter_family, filter_irr,
filter_modify, filter_path_asn, filter_prefix, filter_remove_private_as, gr,
healthcheck, hostname, llnh, nlri, persist, redistribute_egress,
redistribute_ingress, rib, role, route_refresh, rpki, rpki_decorator, rr, rs,
softver, watchdog
<!-- END GENERATED: arch-bgp-plugins -->

**CLI** -- SSH-accessible network OS CLI: YANG-modeled config editor with modes, completion, diff, commit, history, dashboard, monitoring.

**Web** -- HTMX-based UI: config editor, admin, SSE live updates, ASN decorators.

**Looking Glass** -- peer/route viewer with birdwatcher-compatible API, topology graph, SSE streaming.

**Config** -- YANG-modeled. File -> Tree -> `ResolveBGPTree()` -> `map[string]any` -> `reactor.PeersFromTree()`.

**Key wire abstractions:** `WireUpdate` (lazy-parsed, zero-copy), `EncodingContext` (negotiated capabilities), `ContextID` (same = forward unchanged), pool dedup (per-attribute, refcounted), buffer-first (`WriteTo(buf, off) int`).

## Programs

| Binary | Purpose |
|--------|---------|
| `ze` | Network OS: bgp, cli, config, hub, iface, exabgp migrate, plugin, schema, signal, completion |
| `ze-chaos` | Chaos testing orchestrator: fault injection, scheduling |
| `ze-perf` | Performance benchmarking: UPDATE throughput tracking |
| `ze-analyse` | MRT/RIB analysis: attributes, communities, density, dump |
| `ze-test` | Functional test runner: bgp, editor, peer, mcp, web, rpki, managed |
| `ze-gok` | gokrazy appliance image build wrapper (`cmd/ze-gok/`) |

### Binary naming convention

Binaries fall into two families, and the distinction is load-bearing:

- **Host binaries** run on the operator / build / dev machine and are compiled for
  the host (no `GOOS`/`GOARCH` override). These are the CLIs in the table above (one
  `cmd/ze/` codebase selected by build tag) plus `ze-gok` (`cmd/ze-gok/`). A build or
  test action that must RUN one of these to drive `ze appliance ...` on the build
  host compiles `cmd/ze` (tags `ze_core,ze_setup`) and names it `ze-host` by
  convention (for example, `internal/le/qemu.(*Installer).buildHostZe`).
- **Target binaries** run on the appliance or inside an image and are cross-compiled
  `GOOS=linux GOARCH=<arch> CGO_ENABLED=0`: `cmd/ze-installer` (the busybox-free
  installer initrd's PID 1, build tag `ze_installer`, packed into the initrd as
  `/init`; `./le build-artifacts installer-amd64` and `installer-arm64` write
  standalone cross-builds to `bin/ze-installer-<arch>`), `cmd/ze-serial-shell`
  (appliance serial console), and `cmd/ze`
  itself when gokrazy packs it into the image.

Rule: NEVER cross-compile a host binary. A target-arch `ze-host` cannot exec on the
build host ("exec format error"). Apply `GOARCH=<target>` only to the build of a target
binary, or to the `ze appliance initrd` invocation that cross-compiles one internally,
never to the build of the host tool that runs it.

## Source Layout

| Area | Location |
|------|----------|
| Components | `internal/component/` (one directory per component; generated list above) |
| BGP engine | `internal/component/bgp/` (reactor, fsm, wire, wireu, message, attribute, capability) |
| BGP plugins | `internal/component/bgp/plugins/` (rib, rs, gr, nlri, filters, rpki, bmp, ...) |
| System plugins | `internal/plugins/` (generated list above) |
| Plugin host infra | `internal/component/plugin/` (registry, server, manager, generated `all/`) |
| Plugin SDK (external API) | `pkg/plugin/`, `pkg/ze/` |
| Core leaf packages | `internal/core/` (events, family, env, diagnostic, metrics, clock, textbuf, ...) |
| Appliance | `internal/appliance/` (gokrazy image, installer, updater) |
| Programs | `cmd/ze/` (build tags: `ze_core`, `ze_test`, `ze_chaos`, `ze_perf`, `ze_analyze`, `ze_setup`, `ze_distro`, `ze_appliance`; and `ze_le`, which adds le's development commands under `ze le` and is never set by a shipped build) |
| Tests | `test/` (.ci), `*_test.go` |

## Before You...

This table keeps only the highest-stakes actions. **For everything else the
dispatch is `ai/rules/INDEX.md`** (one line per rule -- scan it, read the
listed file in full before acting on a topic it covers) **and `ai/INDEX.md`**
(task navigation, keyword -> doc, dev tools). Absence from this table NEVER
means "no rule applies".

| Action | Read first |
|--------|-----------|
| Find out how ANY surface works, before a search, a grep, or an agent | `ai/rules/documentation.md` -- read the page FIRST: `ai/CODE-TO-DOCS.md` (file to pages), `ai/DOCS-TO-CODE.md` (page to files), `ai/INDEX.md` (keyword to page). You investigate only what the page leaves SILENT or gets WRONG, and you name which |
| Change any behavior a page describes | `ai/rules/documentation.md` -- the page edit lands in the SAME work as the code, before the next code edit. Never at review, never at closure, never in a follow-up commit. `ai/rules/repo-maintenance.md` says which page |
| Write repository prose: docs, comments, error messages, CLI output, specs, commit messages | `ai/rules/writing.md` -- apply US English and the six habits. Read the full style guide only for documentation work, a deep prose review, or resolving an STE finding |
| Start a session | **Read `docs/contributing/ze-go-style.md` in full, EVERY session, before any code (owner directive, 2026-08-18).** Then `.claude/rules/session-start.md` for the {{TOOL}}-specific checklist |
| Edit CLAUDE.md, AGENTS.md, any synced file, or add an agent behavior rule | `ai/rules/repo-maintenance.md` -- never edit generated files; shared rules go in `ai/rules/` |
| Design or implement anything | `ai/rules/architecture.md` -- grep ze before proposing, never default to trained instincts |
| Choose the shape of a fix, or add an abstraction, option, layer, or parameter | `ai/rules/simplicity.md` -- the fix MUST be the simplest FULLY CORRECT answer. Simplicity cuts machinery, never correctness: quality is 0% compromise. The simplest design is usually the hardest to find, so budget the thinking. Another problem you see gets its own spec, never an extra branch here |
| Start a planning, implementation, or review phase | `ai/rules/planning.md` -- review runs on Opus 5 and is INDEPENDENT of the author; implementation carries no model requirement |
| Work on ANY spec (research, design, implement, review, close) | `ai/rules/planning.md` -- the main thread supervises only; each phase runs in a subagent through its `ze-*` skill, and the main thread verifies the report rather than relaying it |
| Make a behavioral claim about code, or recommend work based on one | `ai/rules/evidence.md` -- read the producer, not the caller. Name the file and the symbol. If you did not read it, label it unverified |
| Write an owner report | "Say it once, say it short" above -- report only what changes the owner's next action |
| Write a rule, a doc, a commit body, or a journal row | `ai/rules/writing.md` -- write what changes the reader's next action, then stop. One example for one point. Two readings beat a third example. Budgets for each artifact |
| Find recurring development friction or problem patterns | `ai/rules/repo-maintenance.md` -- report the pattern and decide whether a new or changed rule would prevent it |
| Write any code | `ai/rules/architecture.md`, relevant `ai/patterns/`, `ai/rules/repo-maintenance.md` (which checks will fire) |
| Write or review a guard (auth check, validator, constraint, ratchet, lookup that gates behavior) | `ai/rules/evidence.md` -- fail closed or say something; a zero value must never be a valid-looking answer; drive the guard's test from its entry point, never the helper alone |
| Add or change a CLI command, its output, or its JSON | `ai/rules/cli.md` -- keyword before value, every command supports all pipe operators, and the response payload is structured data so `\| json`, `\| yaml` and `\| table` each render it. Structural template: `ai/patterns/cli-command.md` |
| Add terminal colors or TUI styling | `docs/architecture/cli/color-system.md` -- 7 semantic roles, consistent palette across all surfaces |
| Touch wire encoding, allocate memory, or build strings | `ai/rules/performance.md`, `ai/rules/performance.md`, `ai/rules/performance.md` -- load-bearing divergence from standard Go |
| Add a YANG leaf, env var, or config option | `ai/rules/config.md` (YANG vs env var decision), `ai/rules/config.md` (naming), `ai/patterns/config-option.md` (structural template) |
| Add or move a plugin's command, schema, help, or doctor check | `ai/rules/plugins.md` -- remove the plugin and ALL its features vanish; no plugin spelling in generic/central packages |
| Create a new package (pick internal/core vs component vs plugins) | `ai/rules/architecture.md` -- tier = dependency direction; a misplaced config-driven engine fails `./le tier check` |
| Add a feature, tool, self-check, verification gate, or test infrastructure | `ai/rules/repo-maintenance.md` -- update rules, docs, indexes, and verification paths so future agents discover and use it |
| Write tests | `ai/rules/testing.md`, `ai/rules/testing.md`, `ai/rules/testing.md`, `ai/rules/interop-and-goal-validation.md` |
| Meet a WAVE of red: unrelated packages failing to build, `no space left on device`, `cache entry not found`, a whole suite red at once | `docs/contributing/running-commands.md`, "When the disk is full" -- the cache disk is full before this is a code defect, once for each row in `plan/journal/full-disk-false-red.md`. Read `stat -f cache/go-cache`, never `df` on the checkout: `cache/` is a symlink onto another filesystem, so `df` answers about the wrong device. `./le scratch cache-clean` empties BOTH Go build caches and prints what each returned |
| Meet a red test, a red gate, or a broken demo | `ai/rules/pre-release.md` -- ask ONE question: does the red say the PRODUCT is wrong? Yes: the product defect is the work, fix it at the source and never by weakening the test. No: the red is scaffolding, leave it red, say so in one line, and go back to the product |
| Hit a defect in the PRODUCT (yours or not) | `ai/rules/completion.md` -- ROOT-CAUSE IT. Blocks your goal: FIX IT NOW. Does not block it: one journal row, close the work in hand, stop. Never park it, never offer to drop the deliverable, never weaken a test to make it disappear. "Pre-existing" says when it started, not whose it is |
| Touch any protocol behavior an RFC governs, or judge whether it is conformant | `ai/rules/rfc-compliance.md` -- conformance is not negotiable. When full compliance AND full testing of it is reachable, that IS the answer: IMPLEMENT it and prove it with a tagged test, and do NOT ask Thomas to choose between it and something narrower. Ask only when you are about to do LESS (`{gap}`, `{not-applicable}`, "partial", untested MUST, deferral), and then ask which way to fix it, never whether to skip it. Every earlier answer pointing away from full compliance is VOID (2026-07-27) and must be re-raised, not cited |
| Write linux-only code | `ai/rules/platform-linux.md` -- QEMU integration tests are mandatory, never skip for "needs hardware" |
| Write a spec | `ai/rules/planning.md`, `plan/TEMPLATE.md` |
| Write code identifiers, comments, docs, CLI text, or error messages | `ai/rules/writing.md` -- project language is US English; only Thomas's authored prose (`/write`) is UK English |
| Claim work is done | `ai/rules/completion.md` -- every AC implemented and wired; every exported symbol has a non-test caller. A red or missing test is stated in the report, not repaired to earn the claim (`ai/rules/pre-release.md`) |
| Review code, or close a spec | `ai/rules/planning.md` -- review is the central deliverable and is INDEPENDENT; your own inline reasoning about code you wrote is NOT a review. Independence is a property of the CONTEXT, so ONE closure agent running every lens itself satisfies it and MUST NOT spawn readers of its own. Loop to zero, record the `./le spec session review record` artifact (`./le commit create` enforces it) |
| Finish Go edits | `ai/rules/commands.md` -- run `./le verify lint run` before claiming done |
| Commit | `ai/rules/git-safety.md` -- the native `./le commit create` route. A commit owes NO green gate (`ai/rules/pre-release.md`); the gate is owed before a push |
| Run any test/build/lint command | `ai/rules/commands.md` -- use the registered `./le` action so feature tags and job admission are preserved; no lossy pipes, read the log after; write it under `$(./le session scratch ensure)`, never at the `tmp/` root |
| Delete / overwrite any user-visible file | `ai/rules/never-destroy-work.md` -- ask first for user-visible or uncommitted work; this is the standing exception to "don't ask" |
| Complete work autonomously | `ai/rules/completion.md` -- finish the task, then report; ask only for destructive actions or genuine scope changes |
| Decide whether to stop, ask, delegate, or continue when two rules disagree | `ai/rules/rule-precedence.md` -- one ladder: irreversible action > outside-facing correctness > scope integrity > phase boundaries > autonomy. Stopping at a phase boundary is not asking permission; a forced question is always "which way", never "may I skip it" |
| Understand architecture or how Ze diverges from standard Go | `docs/architecture/core-design.md`, `ai/rules/architecture.md` |
| Check past decisions or known traps | `plan/learned/RECURRING-PATTERNS.md`, `plan/learned/DESIGN-HISTORY.md`, `plan/learned/HOOK-FRICTION.md`, `plan/journal/` (recurrence data) |

## Every Rule's Trigger (loaded below)

Two generated files load here. Never edit either one by hand.

`ai/rules/TRIGGERS.md` names **every** rule under `ai/rules/`, one line each,
carrying the rule's path, its severity, and the situation that makes it apply.
It is a routing index. It does not carry the rules.

**When a trigger matches the work in hand, READ that rule's file at
`ai/rules/<name>.md` before you act on its topic.** The trigger line is all this
session holds about that rule. Its directives are one Read away, and they are
not loaded until you open them. A rule you never read is a rule you never
followed.

`ai/rules/CORE.md` carries the full directives of the always-on rules. Those
apply before the shape of a task is known, so they sit behind no trigger. The
index marks them `always-on`, and such a rule needs no read.

Both come from one parse by `./le rules condensed-update`, in the canonical
rule format (`ai/rules/rule-format.md`). The "Before You..." dispatch above
still applies, and so does `ai/rules/INDEX.md`.

@ai/rules/TRIGGERS.md
@ai/rules/CORE.md
