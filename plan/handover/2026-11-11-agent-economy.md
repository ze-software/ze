# Handover: agent-economy (cut token and wall-time waste in the development system)

Written 2026-11-11 by the session that measured the waste, wrote the spec set
and landed its first pieces. Thomas's orders in that session: "Can you resolve
all those 8 issues?", "I would be fine to have one spec using agents to run
other specs", "I default to Opus for development", "Make sure the wording is
not anthropic specific", "Can you work on one and three? as you indicated it
should be the strongest model editing them." Nothing is pushed; a push needs
his order.

The umbrella holds the measurement, the owner's decisions, the WIP cap
exception and the model tier per child:
`plan/post-release/engineering/spec-agent-economy-0-umbrella.md`. Read it
first. This page says only where each child stands and what to do next.

## Start here

| # | Work | State | First action |
|---|------|-------|--------------|
| 1 | Child 1, `spec-agent-economy-1-call-shape.md`, phase 1 (a subagent types its identity line; the Bash hook checks it and never rewrites a command) | Code, tests and pages written by an agent, reviewed whole by the strongest tier, one hardening applied. Red observed with the check unregistered, green with it registered, over `internal/le/hookruntime`, `internal/le/ai/rules` and `internal/le/le/path`. Four old tests that asserted the rewrite were changed with the test judge's recorded approvals (`hookruntime.<TestName>`, in `tmp/commit-weakening-approved-7d3dd3bf.md`). Rendered rules regenerated | If `git log` shows no commit whose subject starts `hooks: a subagent types its identity line`, land it: the file set is the row below. Then run the live probe |
| 2 | Child 1, the live probe (assumptions A-1, A-2, A-4 of the spec) | Not run: it needs the phase 1 commit, because the shared `le` follows HEAD | Spawn one `ze-read` agent after the commit. Ask it whether its start text holds the full identity line, to run three reader commands and one `./le spec current` with the line, and to report any "no verdict" or identity refusal. Record the answers in the spec |
| 3 | Child 1, phases 2 to 4 (guards judge commands through the one line splitter; refusal messages, test deletion and the budget; hook fixture seal) | Not started. Phase 1 already built the splitter (`lineCommands`, `internal/le/hookruntime/shellline.go`), so phase 2 only points the guards at it | One `ze-work` agent per phase, development tier, through `/ze-implement`. `./le ai hooks unit` is red on `hook-source-drift` until phase 4 re-derives the seal: expected |
| 4 | Child 2, `spec-agent-economy-2-scoped-test.md` (`./le test unit pkg path <p>...`, the wait record, nearest-name answer, commit body) | `ready`, committed. Designed by an agent, reviewed twice by an independent agent | Phases 1 to 5 touch no Go file child 1 owns and may start now, one `ze-work` agent per phase. Phase 6 waits for child 1's phases 3 and 4 |
| 5 | Child 3, `spec-agent-economy-3-preamble.md` (phase agents start without the instruction files; the decisions file in two parts; the core shows always-on points; the style page split) | `ready`, committed. Designed by the strongest tier | Phase 1 is the probe and needs a session started after `./le ai sync update`, because agent definitions load at session start. Its review, and the review of the decisions-file move, are strongest-tier work |
| 6 | Child 4, `spec-agent-economy-4-supervision.md` | `skeleton` with the research. Depends on child 1 (same hook file) | Design it (development tier). Two of its three probes are answered by the harness documentation quoted in child 3's Design Insights: a pre or post tool hook can return additional context that reaches the model, and SubagentStart carries the agent id and type |
| 7 | Child 5, `spec-agent-economy-5-scoped-gates.md` | `skeleton` with the research | Last, by design: it changes what two gates prove, and waits for child 2's wait record. Its design is strongest-tier work |
| 8 | The umbrella's acceptance criteria | Not measurable until the children land | Child 4 owns a kept measure (`./le ai tokens`); repeat the baseline with it |

## Phase 1 file set (row 1)

Commit these together, under the child spec's claim so the commit carries its
`Spec-work:` line. The implementing agent holds that claim; a main session
that commits it instead claims the child spec first
(`ZE_SPEC_WIP_CAP=<in-progress count + 1> ./le spec claim ...`, the owner's
cap exception recorded in the umbrella).

| Kind | Files |
|------|-------|
| Code | `internal/le/hookruntime/session.go`, `shellline.go` (new), `bash.go`, `runtime.go`, `lifecycle.go`, `internal/le/le/path/commitsession.go` |
| Tests | `internal/le/hookruntime/session_identity_test.go` (new), `lifecycle_test.go`, `runtime_test.go`, `budget_test.go`, `one_off_proof_test.go` |
| Pages and rules | `docs/contributing/running-commands.md`, `.claude/hooks/README.md`, `plan/learned/HOOK-FRICTION.md`, `ai/rules/points/planning/directives/brief-every-subagent-with-its-spec-phase-and-rules.md`, `ai/rules/planning.md` (rendered) |
| Spec | spec-agent-economy-1-call-shape (closed) |

The commit body names the one known red: `./le ai hooks unit` fails
`hook-source-drift` until phase 4 re-derives the seal. The scoped lint
(`./le go lint run scope "./internal/le/hookruntime/..."`) was started by the
writing session; if its result is not in the commit body, run it once.

Other sessions hold uncommitted work in the tree. Name only these files.

## What changes for every agent once row 1 lands

A subagent's Bash command made only of readers (grep, sed, cat, a git read
verb, the list in `readerCommands`) runs as typed. Any other command must
start with the identity line the agent's start text prints, and is refused
with that exact line when it is missing. Every brief written after the commit
should say so in one sentence; the start text and the refusal carry the line.

## Decisions taken in the writing session (do not put them to Thomas)

| Point | Decision | Why |
|-------|----------|-----|
| How the command rewrite is replaced | The agent types its identity line and the hook checks it | A rewritten call is one auto mode holds no verdict for; the harness documents that |
| List form of the short test action | Repeated keyword `path`, no comma list, no parser change | The existing list pattern; the parser already refuses a dash-leading value |
| Race detector on the short action | Always on | A package tested without it is not tested the way the gate tests it |
| `changed` widening to the whole tree, `./...`, a path spanning groups | Refused before admission | An agent starts no whole-tree check |
| A test filter that matches nothing | The action answers failure | A run that tested nothing must not read as a pass |
| The decisions file split (D-1) | One file, two parts, wording unchanged | One home every harness loads; no citation moves |
| The style gate (D-3) | The page is split; the gate's code is untouched | "Read in full" stays literally true |

## Open with Thomas

| Question | State |
|----------|-------|
| Two model fields in the spec template (design tier, implementation tier), review never weaker than the stronger | Recommended to him; he has not said yes. The tiers per child are recorded in the umbrella meanwhile. Do not change the template without his word |
| Whether "one spec's agents run other specs" is a standing rule | Recorded as a permission for this umbrella only |

## Strongest-tier work that remains

Everything else is development-tier. These three want the strongest model the
harness offers, by the umbrella's tier table: the design of child 5; the
review of child 3's implementation, above all the bullet-by-bullet move of
`ai/META-DECISIONS.md`; the review of child 4's skill-body trimming.
