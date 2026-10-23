# Owner Meta Decisions

How agents work in this repository and what is in scope, as Thomas has decided
it. Every main session loads this file at start. It holds rules for behaviour
only: no history, no dates, no record of who asked for what. Nothing anywhere
cites it by a row number or label; a rule that depends on one of these states
the rule itself.

A question this file answers is never put to Thomas. When he states a new rule
of this kind, the main session adds it here in the same turn and commits it.
A permission for one piece of work (a cap exception, approval to change named
tests) is not a rule: it goes in the spec or commit it applies to.

## Goal and scope

- The goal behind all work is shipping Ze in production as a NOS. A change to
  the shipped binary outranks tooling, and recording a problem never counts as
  closing it.
- Every RFC Ze implements gets full, correct coverage; an absent requirement of
  an implemented RFC is owed work, never a question. The only exception is a
  basic primitive Linux cannot provide, named with evidence.
- Release timing: shipped code must comply with the RFC requirements of the
  capabilities it implements. A missing RFC feature may wait until after the
  first release, and a partial implementation is never announced as full
  support.
- Full YANG (RFC 7950) coverage runs after the first release. Before it, YANG
  work is limited to what blocks something Ze needs.
- A question the RFC text answers is not asked. Where the RFC is silent or
  unclear, the goal is interoperability: Ze must work with BIRD and FRR, so read
  their source and do what lets Ze interoperate with both. Before a protocol
  design is approved, read FRR's and BIRD's source and quote the deciding
  function.
- A logic bug or misbehaviour is fixed, never put to Thomas as a question.
- An agreed design answers the questions that follow from it.

## ExaBGP compatibility

- ExaBGP 6.0, 5.0 and 4.2 backward compatibility is agreed, so what it needs is
  done, not asked. It lives only in the bridge and the migration.
- The RFC outranks ExaBGP API compatibility, which outranks ExaBGP's behaviour.
  Where ze is stricter than ExaBGP without an RFC MUST behind it, ExaBGP's
  behaviour wins; where the RFC says MUST, ze keeps it and documents the
  difference.
- Compatibility means the same command produces a valid BGP message with the
  same effect on the remote router, not a byte-for-byte copy.
- AS_SET: a route from an ExaBGP script through the bridge may carry one, and a
  received one is accepted only on a peer migrated from an ExaBGP config. Ze's
  own API and plugins never originate one.
- Ze does not support the BGP OPERATIONAL message or BGP multi-session. An
  ExaBGP test that depends on either meets a documented difference, which is not
  owed work.

## Test systems and labs

- VMs, test guests and the lab Docker host run Ze's own kernel and system, never
  Alpine or a stock distribution kernel. A problem that exists only because a
  guest runs another system is out of scope: the fix is moving that guest to
  Ze's system. Colima may manage the VM on macOS, booting Ze's image only.
- The Linux host is too busy for full test runs: there, run scoped package
  tests only, never `./le verify worktree`, a full functional suite or a full
  `./le go lint run`. Full runs happen on another host.
- Kernel work uses the architecture of the machine the agent is on. Never
  cross-compile or emulate the other architecture's kernel locally; its proof
  comes from a host that runs it natively.
- Third-party software in the interop and integration labs runs its latest
  stable upstream release; BIRD runs the BIRD 3 line. Software with no stable
  release (tac_plus-ng, freeRtr) is built from source, first updated to
  upstream's latest when the source in use is more than a month old.

## Code and dependencies

- A change Ze needs in a third-party Go module is kept here as a diff against
  the upstream release, applied at build time. A fork exists only to open an
  upstream pull request and is never a dependency.
- An RFC sentence quoted in a comment stays byte-exact, its full stop inside the
  quote included, and is never edited to satisfy a linter.

## Ze is pre-release

- No release, version, tag or user consumes `main`. A red test does not hold a
  commit: commit the work and name the red test and its cause in the body.
  Full rule: `ai/rules/pre-release.md`.
- During development, run only the tests that cover what you changed: the
  package test, the component group, the functional suite for the behavior.
  The full gate (`./le verify current`, `./le verify worktree`) runs once per
  piece of work, over its final commit, never between phases, review rounds or
  fixes.
- A commit that changes only documentation or prose owes no full gate, before
  a commit or before a push: the targeted checks for the files it touched are
  its evidence. A push gate compares the pushed tree with the last green tree
  ignoring such paths.
- There is no verification-debt ledger. Debt is derived from git and the last
  green full verify, and `./le commit push` refuses while it is owed. The
  session-start line is the only report of it, and it is never work to do
  before the single full gate.
- A check that ran is not re-run to reconfirm what its output said.
- When the user asks for a commit, prepare the script and run it. Do not review
  again or rerun gates for the commit. If `./le verify status check` is FRESH,
  never rerun `./le verify worktree`.

## Every session

- Before the first edit in each checkout, `git fetch` and confirm
  `git rev-list --count HEAD..@{u}` is zero; pull with
  `git pull --rebase --no-autostash` only when it is not. A dirty tree is no
  blocker unless a pull is owed. The stop conditions and the subagent exception
  are in `ai/rules/git-safety.md`, "Never edit behind the upstream".
- Before your first Go edit in a session, read `docs/contributing/ze-go-style.md`
  in full; the pre-write hook refuses a Go edit until you have.

## Delegate to subagents

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
- The main thread supervises: it launches, verifies each report against source
  (`ai/rules/evidence.md`), decides, and gates the next phase. Only questions the
  user must answer stay in the main thread.

## Reports and messages

- A report to the owner opens with what is blocked, why it matters, and what you
  need from him. He reads the first ten lines and stops, so the decision goes
  there, as a table with one row per decision. What you did and what each agent
  found goes last or goes unsaid. Status that changes no decision is one line.
- An agent's report is written for the agent that commissioned it. Rewrite it
  for the owner, never forward it. A reply to the owner stays under 15 lines and
  puts its tables before its prose. Other prose follows `ai/rules/writing.md`.
- Every message a user sees, on every interface (CLI, web, `ze doctor`, logs,
  `./le`, API and gNMI), says what is wrong in plain words, why, and the exact
  command that fixes it (`ai/rules/user-facing-errors.md`).

## Website design

- Website design follows the approved principles in
  [the website style guide](../website/style-guide/index.html#sg-design-principles),
  including its information-preservation map and owner approval for removals.
  This decision applies to the website only.

## How agents work

- Before handing work back, clean up the task's obsolete temporary files and
  local servers. Check ownership, references and running processes first;
  preserve current deliverables, originals, useful comparisons and other
  sessions' work. Check available space before large copies and reuse suitable
  outputs. Record the purpose and removal condition of anything retained,
  explain large retained items, and report substantial space recovered.
- A defect you walk into while working on something else gets one journal row in
  `plan/journal/<class>.md`; grep `plan/journal/` first, because other sessions
  meet the same defect. List that row in the owned journal rows table of the
  spec that will fix it: an existing spec whose mechanism fits, or a new one.
  Then close the work in hand and stop, with no ask. A commit that adds or
  changes an open row no spec owns is refused.
- Sessions share the one `main` checkout. No worktree per session: fix
  shared-checkout friction inside that model.
- Close in-progress specs before opening new ones. The WIP cap is lowered as
  specs close, never raised.
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
- Never present your own unfinished work as a question: do it. An ask that
  remains carries the evidence needed to answer it (the identifier, the current
  value, what each answer changes); one that cannot is not ready to be asked.
- A hook checks what a command, path or file is, never how a message is worded.
- Run a formatter, codemod or rewrite tool only on the files you edited in this
  task, never on every dirty file, a directory glob or the whole tree.
- Many sessions share one Docker daemon: never run a host-wide destructive
  Docker command (`prune`, or `rm`/`rmi` of anything your run did not create).
  Remove only what your own run created, by name.
- `sudo` is allowed only for the stress harness (`./le test integration
  stress`): one root run at a time, alone on the host, removing what it
  created, and leaving anything in a shared location usable by the owner's
  user.
