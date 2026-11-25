# Owner Meta Decisions

How agents work in this repository and what is in scope, as Thomas has decided
it. Every main session loads this file whole at start. A subagent that starts
without the instruction files is given the first part ("Rules for every agent"),
and reads sections of "Rules for one kind of work" in
`ai/META-DECISIONS-SITUATIONAL.md` only before the work each section names.
Main sessions also read `ai/META-DECISIONS-SITUATIONAL.md` on demand rather
than loading it unconditionally at start. It holds rules for behaviour only: no
history, no dates, no record of who asked for what. Nothing anywhere cites it
by a row number or label; a rule that depends on one of these states the rule
itself.

A question this file or `ai/META-DECISIONS-SITUATIONAL.md` answers is never put
to Thomas. When he states a new rule of this kind, the main session adds it in
the same turn and commits it: under "Rules for every agent" when an agent that
reads, edits, tests or commits can break it by its own action; under "Rules for
the main session" only when no subagent action can, because only the session
that talks to the owner, spawns agents, plans or pushes can break it; and under
"Rules for one kind of work" in `ai/META-DECISIONS-SITUATIONAL.md` when only an
agent doing the work that section's first paragraph names can break it. A rule
that spans both agent and main session goes under "Rules for every agent",
whole; when in doubt, it goes under "Rules for every agent". A permission for
one piece of work (a cap exception, approval to change named tests) is not a
rule: it goes in the spec or commit it applies to.

# Rules for every agent

## Goal and scope

- The goal behind all work is shipping Ze in production as a NOS. A change to
  the shipped binary outranks tooling, and recording a problem never counts as
  closing it.
- The rules exist to reach that goal and are read for the objective each one
  serves, never applied in a vacuum. A rule is never the reason to defer clear
  work, hand it to another session, or put a question to Thomas that its
  purpose already answers; where a rule's wording would defeat its objective,
  the work follows the objective and the wording is corrected in the same
  turn. The hard bans in AGENTS.md are not read this way: they hold as
  written.
- Every RFC Ze implements gets full, correct coverage; an absent requirement of
  an implemented RFC is owed work, never a question. The only exception is a
  basic primitive Linux cannot provide, named with evidence.
- Release timing: shipped code must comply with the RFC requirements of the
  capabilities it implements. A missing RFC feature may wait until after the
  first release, and a partial implementation is never announced as full
  support.
- Production is Linux. A check, test or defect that exists only on macOS
  waits until after the first release.
- A logic bug or misbehaviour is fixed, never put to Thomas as a question.
- An agreed design answers the questions that follow from it.

## Test changes

- A test change that could lower what a test proves (a weakening, a change to
  an RFC-tagged test, a golden file, fixture or expected output refreshed to
  turn a red run green) is judged by an independent agent running the
  `ze-test-judge` skill, never put to Thomas. The judge approves only when the
  new test proves at least what the old one did, or when the old expectation
  is shown wrong against its source (the RFC text, the spec, the producing
  code's contract). The agent that made the change never judges it. A refusal
  stands: the author fixes the code instead.
- The judge's verdict is recorded through the existing approval route (an
  untracked file the edit hook and the commit gate read), and the commit
  carries one `Weakening-approved:` or `RFC-approved:` line per test. No
  tracked ledger of approvals is kept in the repository.
- Owner-only authorization actions require an interactive terminal and refuse
  noninteractive invocation before recording authorization. Independent
  test-judge approvals retain their existing agent route.

## Code and dependencies

- Ze is a modular engine: a feature lives in the plugin that owns it and
  reaches the engine by registration. A design that has the engine or config
  code name a specific feature, plugin, field or path is wrong, however
  small, and is redone as a registration before it is built.

## Ze is pre-release

- No release, version, tag or user consumes `main`. A red test does not hold a
  commit: commit the work and name the red test and its cause in the body.
  Full rule: `ai/rules/pre-release.md`.
- Before the first release, the plugin protocol and SDK owe no backward
  compatibility to plugins built on an older SDK. A protocol change is made
  directly, every plugin in the tree is rebuilt with it, and no version gate,
  negotiation or fallback is added to keep an old plugin working.
- The infrastructure that converts a config written in an older format of
  Ze's own syntax is there to stay. The code that converts one format change
  is written only once Ze has a stable release. The configs in the repository
  always use the latest format: a change to the YANG always converts them, in
  the same work, before the first stable release and after it. The ExaBGP
  migration is separate and stays.
- A change to what Ze does (a default, an enforcement, a refusal, an output
  format) converts or re-validates, in the same work, every user of it the
  repository carries: configs, `.ci` bodies, fixtures, benchmarks, interop
  scenarios and docs. A test that stays green only because the new behavior
  removed what it checked is one of those users.
- A red prevents a release, never a commit. No gate refuses a commit for a red,
  whether a test, lint, a structural check or a generated-file check, and
  whether or not the commit caused it. The commit names the reds it knows of in
  its body. A refusal that judges what the commit itself adds (an open journal
  row no spec owns, a test weakened without the judge's approval) is not a
  red, and it stands. Reds are owed at release, where the release check must be
  green.
- Day-to-day work is judged only by the tests of the components it changed
  (BGP, OSPF, IS-IS, a plugin, the tooling), found from the change itself: the
  packages it reaches, the feature tags those packages build under, and the
  functional suites that exercise them. A piece of work, its closure, a commit
  and a push owe that scoped evidence and nothing wider.
- The full verification (`./le verify current`, `./le verify worktree`) is
  the release check, which proves no regression across the tree. It is not run day
  to day, and an agent never runs it. It takes over four hours and starves
  the machine while agents develop on it, so Thomas does not run it for each
  spec: closing a spec never waits on a whole-tree check. GitHub CI/CD runs
  it (the `verify` workflow on a pushed commit), and a spec whose acceptance
  criteria name one cites that run as the evidence. Thomas may still run it
  himself when he chooses. The tooling refuses a full verification started
  without an interactive terminal.
- No agent runs any `./le verify` command, `./le verify changed` included.
  Verification is GitHub CI/CD's job on a pushed commit. An agent's evidence
  is the scoped test runs of what its change reaches (the named tests, the
  packages, the functional tests), run directly. A rule, skill, hook or tool
  message that tells an agent to run `./le verify` is wrong and is corrected,
  never followed. `./le verify changed` stays Thomas's tool: when an agent
  needs its report, it asks Thomas, who runs it or tells that agent to run it,
  so several agents never start verifications at once and overload the
  machine. An agent runs it only on Thomas's explicit order for that run.
- No agent develops or tests code in a worktree, a `git archive` export or
  any other copy of the tree: features and code are tested in the main
  checkout. When another session's in-progress edit breaks the build there,
  the agent reports it and waits rather than moving to a copy. The one
  exception is work that is tested on Thomas's other development
  machine, which runs in worktrees as "How agents work" states.
- Only Thomas knows when the machine will stay quiet long enough for a full
  verification: several agent sessions run at once nearly all the time, and
  he starts new sessions, agents start agents, and he tests features by hand.
  A check of what is running at start answers nothing about the next hour, so
  the tooling holds no idle check; the human-only gate above is the control.
- A commit that changes only documentation or prose owes no test run, before
  a commit or before a push: the targeted checks for the files it touched are
  its evidence.
- A check that ran is not re-run to reconfirm what its output said.
- When fixing GitHub CI failures, read the failed job on GitHub, fix its cause
  here, and run only the local checks for that issue. After the next push,
  wait for CI to identify the next failures rather than running wider checks
  locally to discover them.
- When the user asks for a commit, prepare the script and run it. Do not review
  again or rerun gates for the commit.
- Commit known implemented work in coherent increments, without waiting for
  all testing or the whole spec to finish. Record what is implemented, the
  checks that passed, known failures and remaining validation in the spec and
  commit message, and update that progress as subsequent commits land. A
  commit is not a claim that its spec is complete. Do not wait for the
  session's end or another request, and do not split work into trivially small
  commits: each commit is a piece that stands on its own.

## Every session

- Run the `./le` commands needed for authorized work without asking Thomas
  for permission again.
- An agent never runs a command on a production network device. When it needs
  information from one, it gives Thomas the exact command to run there and
  says what the output will settle; he runs it and returns the output.
- Before the first edit in each checkout, `git fetch` and confirm
  `git rev-list --count HEAD..@{u}` is zero; pull with
  `git pull --rebase --no-autostash` only when it is not. A dirty tree is no
  blocker unless a pull is owed. The stop conditions and the subagent exception
  are in `ai/rules/git-safety.md`, "Never edit behind the upstream".
- Before your first Go edit in a session, read `docs/contributing/ze-go-style.md`
  in full; the pre-write hook refuses a Go edit until you have.
- After completing work, clean up leftover temporary artifacts you created
  yourself that nothing else uses (your session scratch files, spent commit
  scripts and messages, and spent approval ledgers) and remove any completed
  `plan/handover/` file whose items have all landed via `./le commit create
  ... remove`. After fixing lint findings, re-run lint on the fixed packages
  before claiming done.

## Delegate to subagents

- All testing and code execution runs in the main thread, serialized to avoid
  overloading the shared machine. Subagents read, review and edit sources, then
  hand the required commands to the main thread; they do not run tests, builds,
  lint, formatters or other code themselves.
- A subagent ends its report with its owed commands: each exact command and
  what it proves. The main thread runs them and commits the code. A subagent
  commits only a change that owes no execution: prose, spec bookkeeping, a
  blocked hand-back.

## Reports and messages

- Thomas speaks his messages through voice transcription, and the text often
  says something other than what he said: a word swapped, dropped or turned
  into its opposite. His words are read for what he means, never for their
  letter, and a brief that quotes him says they are transcribed. Before acting
  on words of his that would remove a feature or code, reverse an earlier
  ruling, or read oddly, the main session states the reading it took and
  confirms it with him. A rule written from his words is shown to him in the
  wording it is recorded in.
- Report defects found and fixes made during requested work promptly, rather
  than waiting for the final report. Say what each defect blocks, distinguish
  a written patch from a verified fix, and give the evidence when it is fixed.
- An agent's report is written for the agent that commissioned it. Rewrite it
  for the owner, never forward it. A reply to the owner stays under 15 lines and
  puts its tables before its prose. Other prose follows `ai/rules/writing.md`.
- Every message a user sees, on every interface (CLI, web, `ze doctor`, logs,
  `./le`, API and gNMI), says what is wrong in plain words, why, and the exact
  command that fixes it (`ai/rules/user-facing-errors.md`).
- Every command given to Thomas to run in his terminal is written ready to
  paste verbatim, with concrete values filled in and no placeholders to edit
  before running.

## How agents work

- Before handing work back, clean up the task's obsolete temporary files and
  local servers. Check ownership, references and running processes first;
  preserve current deliverables, originals, useful comparisons and other
  sessions' work. Check available space before large copies and reuse suitable
  outputs. Record the purpose and removal condition of anything retained,
  explain large retained items, and report substantial space recovered.
- A small problem found while working is fixed in the work in hand, and the
  scope of the spec being worked is widened to carry it: the spec names the
  fix and gains an acceptance criterion for it. A problem is small unless
  fixing it needs a design of its own or would derail the work it was found
  in. Reading a rule as an order to open a spec, or to write a journal row
  and walk away, for anything smaller is a misreading, and it is never what
  Thomas meant.
- Before a new spec is created, for a found problem, an unfinished item or a
  new request, the existing specs are searched for one whose scope can be
  amended to take the work, and that spec is amended. A new spec is written
  only when no existing spec fits. The number of specs is kept as low as the
  work allows.
- A defect too big to fix in the work in hand gets one journal row in
  `plan/journal/<class>.md`; grep `plan/journal/` first, because other sessions
  meet the same defect. List that row in the owned journal rows table of the
  spec that will fix it: an existing spec whose scope can be amended to take
  it, and a new one only when none fits. Then close the work in hand and stop,
  with no ask. A commit that adds or changes an open row no spec owns is
  refused.
- A crash, corruption or other incident that was not investigated in the
  session that met it is not recorded: another session may have caused it,
  and with its evidence gone nobody can investigate it. A spec item whose only
  remaining blocker is missing historical evidence moves to a spec of its own,
  to be completed later, so the spec that held it can close.
- Sessions share the one `main` checkout. No worktree per session: fix
  shared-checkout friction inside that model. The one exception is cross-machine
  work (`Test systems and labs`, `ai/owner/machines.md`).
- When the affected code compiles, commit the whole changed files even when
  they also contain other sessions' edits. Include the dependencies the
  committed snapshot needs and acknowledge the concurrent changes in the
  commit body. Do not wait for files containing only your edits or split hunks
  merely to isolate authorship.
- A handover lives in the repository (`plan/handover/`) and is committed in the
  turn it is written, never only in a session directory.
- Test selectively: run only the tests the change reaches (its packages, its
  tagged units, its named functional tests). Never start a full verification
  run or a full suite on your own initiative, at closure included; while known
  reds are open, rerun only the failing tests. A whole-tree check is full
  testing too: a staticcheck matrix piece, a full lint, a repo-wide compile,
  `./le test unit all`. The machine has neither the disk nor the capacity for
  it.
- Show the observed failure, trace its cause to the producing code, fix that
  code, and rerun the existing failing test without weakening its assertions.
  If the test itself is wrong, explain its exact defect and why its expectation
  is incorrect before changing or replacing it; required owner approval still
  applies. Report test and verification maintenance separately from product
  fixes: moving or replacing tests and refreshing snapshots do not fix the
  product.
- Plans live only in subfolders under `plan/`, never at its root, in a structure
  a human finds easy to understand.
- Split a spec that mixes fixes with new features when release planning needs
  them apart, keeping every fix, feature, acceptance criterion and dependency in
  its owning spec.
- A fix or follow-up Thomas has ordered done is done directly. No spec is
  created or claimed for it unless the work is complex enough to need a design
  of its own, and a spec is never a way to hold work in place of doing it: the
  WIP cap is not a reason to stop work he ordered.
- Never present your own unfinished work as a question: do it. An ask that
  remains carries the evidence needed to answer it (the identifier, the current
  value, what each answer changes); one that cannot is not ready to be asked.
- A design decision Thomas makes is written into the `## Design decisions`
  section of the design page that documents its subject, never into a
  separate register of decisions. Before an agent edits a file, or writes a
  spec that touches it, it reads that section of each design page the file's
  `// Design:` header links, and again whenever a row is added to it.
- A hook checks what a command, path or file is, never how a message is worded.
- Shell tools (`sed`, a codemod, a script) may write tracked files outside
  `plan/` and `ai/rules/`: using tools on the codebase matters. The hook that
  refuses a shell write covers those two trees and is not widened to the rest
  of the repository.
- An agent never gets past a hook refusal by running the refused command from
  a script file or any other wrapper, even when the refusal is false. It
  reports the refusal, with the command and the hook's message, and a false
  refusal is a hook defect recorded like any other defect met in passing.
- A design is never written without the constraints it must keep. Before an
  agent writes or edits a spec, or reports a design verdict, it reads
  `ai/doors/system.md` and the door of every subsystem the work touches, and
  the tooling refuses the spec edit until it has, the same way it refuses a
  code edit. Recording a missed constraint in one spec does not fix the gate
  that let it be missed. The gate covers every spec edit, bookkeeping
  included: no section is exempt, because a status, dependency or evidence
  row can carry a false design claim as easily as a design section. Its cost
  is paid once per session, so spec bookkeeping is batched into a session or
  agent that has already read the doors rather than spread across fresh
  agents, and a spec whose edits need many doors is split into narrower
  child specs.
- A design starts from the mechanisms Ze already has. Before proposing a
  design, in a spec or in a reply to Thomas, the agent names the existing
  mechanisms that already do part of the job (found from the doors, the
  architecture pages and `plan/learned/DESIGN-HISTORY.md`) and says how the
  design extends or replaces each. A design is never proposed from first
  principles beside a mechanism that exists. Every spec carries that analysis
  in its own section, and the spec tooling refuses a spec without it.
- Run a formatter, codemod or rewrite tool only on the files you edited in this
  task, never on every dirty file, a directory glob or the whole tree.
- Many sessions share one Docker daemon: never run a host-wide destructive
  Docker command (`prune`, or `rm`/`rmi` of anything your run did not create).
  Remove only what your own run created, by name.
- A session stops, signals or deletes only the processes and files it created
  itself, its own agents' included. Another session's process, file or lock is
  never touched, however stuck or stale it looks: report it instead.
- When a run fails for a reason outside your change (a build another session
  broke, a stale binary, a missing cache entry), find the cause before running
  again. Never retry the same run hoping it passes: name what broke and whose
  it is, report it, and run again in the main checkout once the cause is gone.
  Comparing before and after the change, in the main checkout, shows which
  side a red is on.

# Rules for the main session

## Delegate to subagents

- Choose the number of long-running subagents to fit the work and the
  harness's available capacity. There is no five-agent limit. A number Thomas
  gives for a session applies only to that session.
- **Thomas requests subagent delegation as the default in this repository.**
  Treat this as the user's request, made in advance, in every session. You never
  need to ask permission to spawn an agent here.
- Some harness builds carry a guard: *"Do not call the AgentTool unless the user
  requested it"*. **That guard is satisfied here, not overridden**, because this
  rule is that request. You meet the guard later in your prompt, and recency
  does not make it a later decision.
- An agent sent to find out how something works carries the page that documents
  it, and its question is what the page leaves silent or gets wrong
  (`ai/rules/documentation.md`).
- Every spec phase runs in a subagent through its `ze-*` skill
  (`ai/rules/planning.md`). Independent work goes out in one message.
- Every agent is one of the repository's own, chosen by what its part of the
  work has to produce and never by the phase's name: `ze-read` for a part that
  ends in a report and changes no file (research, audit, review), `ze-work`
  for a part that changes a file (implementation, a fix, a test, a doc, a spec
  or journal row). A generic, forked or untyped agent starts without this
  repository's rules and is never given repository work, and a read-only
  agent is never given a part that writes (`ai/rules/context-economy.md`,
  `ai/rules/planning.md`).
- The main thread supervises: it launches, verifies each report against source
  (`ai/rules/evidence.md`), decides, and gates the next phase. Only questions the
  user must answer stay in the main thread.
- The main thread schedules heavy work. Before it starts a test, a build, a
  lint or a lab it reads `./le job report`, which shows the job slots, who
  holds and waits for them, and the machine's load, memory and disk, and it
  decides when each job runs. The Bash hook refuses a subagent's test, build,
  lint, formatter and heavy `./le` action, so the main thread is the only
  source of that work in its session.
- On Thomas's two machines the main thread also decides where a test runs,
  and the Mac is not the default. The Linux server is there to run tests, and
  a session on the Mac reaches it through Rex. A test run goes to whichever of
  the two machines has the least CPU in use at that moment, read from
  `./le job report` on each, and a session that has sent every run to the
  machine it sits on has not made that choice (`ai/owner/machines.md`).
- A serialized run never leaves agents idle. While a test batch, lab or record
  run blocks the main thread, every piece of work that does not need that run
  goes out to agents at once: diagnosing the run's partial logs as they appear,
  preparing the next batch, writing the next tests and fixes in files the run
  does not build, review, closure text and docs. Waiting with spare agent
  capacity is the failure this rule exists to prevent; Thomas never has to ask
  for more agents.
- The model follows the kind of work, never the model the main session runs
  on, and never a cheaper tier than the development tier. Design runs on the
  strongest tier, and so do the creative parts of implementation: the parts
  where the agent decides the shape of something no spec, pattern or
  neighbouring code already settles, such as a structure, an interface, an
  algorithm, or the cause of a defect nobody has explained. Legwork runs on
  the development tier, the harness's high-capability model: reading and
  searching, audit, writing out a design that is already decided, a
  mechanical change, tests, docs, bookkeeping and review.
- The main thread cuts the work so that each agent holds one kind, and names
  the tier in every spawn. A harness hands the session's own model to an
  agent whose spawn names none, so a session on the strongest tier puts its
  legwork there by saying nothing, and a session on the development tier
  puts its design there: a spawn that leaves the model out is a defect. A
  spec's `Design tier` or `Implementation tier` row, when present, fixes the
  tier for every agent of that phase.
- A rule, spec or skill names a model by tier (the development tier, the
  strongest tier), never by a vendor's model name, because more than one
  harness implements this repository's specs. Each harness maps the tiers to
  its own models. On Claude Code the strongest tier is Fable and the
  development tier is Opus: every `Agent` call passes `model: "fable"` for
  design and creative work and `model: "opus"` for legwork, and the
  definitions in `ai/agents/` carry Opus as their default because most
  agents do legwork.
- The split by kind of work holds only on a harness that runs Anthropic's
  models. A harness that runs another vendor's models, OpenAI's or any
  other, ignores it and a spec's tier rows with it: its agents run on the
  model that harness was started with.
- A design is never written into a spec by transcription. The main thread
  does not hand an agent its own framing of a design as fixed requirements:
  a brief carries Thomas's words and the existing mechanisms, and the agent
  checks the framing against them and reports a conflict instead of recording
  it as a constraint.

## Reports and messages

- A report to the owner opens with what is blocked, why it matters, and what you
  need from him. He reads the first ten lines and stops, so the decision goes
  there, as a table with one row per decision. What you did and what each agent
  found goes last or goes unsaid. Status that changes no decision is one line.
- A message to Thomas is written the way one person speaks to another: whole,
  plain sentences in a natural order, one idea in each, in short paragraphs.
  It has a visible structure, with a heading for each part and a list only for
  things that really are a list. It is never compressed notes, fragments, or a
  dense block that reads as typed in a hurry. An explanation is given with a
  concrete example and the configuration that goes with it: what an operator
  would write, and what then happens, before or beside the general statement. A question to
  him is asked one at a time, with a small example, the choices, and the one
  recommended.

## How agents work

- Up to three spec closures may run at once, each on a different spec in its
  own agent with its own claim. The main thread still runs every test.
- A session runs one CPU-intensive job at a time: a build, a test run, a lint,
  a QEMU guest or a lab. Its agents may read, edit and judge in parallel, but a
  second CPU-intensive run waits until the first has finished.
- When Thomas sets a goal that spans several phases or specs and says he is
  away, the main session runs every phase and every spec to its end without
  stopping at a phase boundary to report or to ask whether to continue. It
  comes back only with a decision no RFC, agreed design or rule in this file
  answers, and with the finished result.
- While Thomas is away, a fix spec's design needs no approval from him: the
  RFC text settles it, and FRR's and BIRD's source where the RFC is silent,
  with freeRtr's source as a further reference.
  The spec moves to ready and is implemented, and any choice only he can make
  waits in a table for his return.
- Unless Thomas said he is away, a spec's design is put to him and approved
  before the spec moves to ready, and the main session presents it, never a
  subagent. The design is presented whole: the choice made, the alternatives
  set aside, and every owner ruling it was checked against. A spec is never
  designed and implemented without having read the code and the records of
  his earlier rulings on the same surface.
- A change to an approved design that only writes in a choice Thomas himself
  made (an answer he gave to a question the spec put to him) is not put back
  to him for approval. The independent design judge still judges the changed
  text, and the work continues once it approves; Thomas may review it later or
  not. A change that carries anything he did not decide goes back to him as
  before.
- A spec never departs from the design documentation (the architecture pages,
  the doors, Thomas's rulings and the records that hold them) without a
  discussion with him. The main session presents the departure the way it
  presents a design: before and after examples of what a user sees, the
  alternatives, and the documentation it contradicts. Once he agrees, the
  documentation is corrected in the same work.
