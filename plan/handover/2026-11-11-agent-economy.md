# Handover: agent-economy (cut token and wall-time waste in the development system)

Written 2026-11-11 by the session that measured the waste and wrote the spec
set, then rewritten the same day by the session that ran children 1 and 2.
Thomas's orders: "Can you resolve all those 8 issues?", "I would be fine to
have one spec using agents to run other specs", "I default to Opus for
development", "Make sure the wording is not anthropic specific", "Can you work
on one and three? as you indicated it should be the strongest model editing
them." Nothing is pushed; a push needs his order.

The umbrella holds the measurement, the owner's decisions, the WIP cap
exception and the model tier per child:
`plan/post-release/engineering/spec-agent-economy-0-umbrella.md`. Read it
first. This page says only where each child stands and what to do next.

## Start here

| # | Work | State | First action |
|---|------|-------|--------------|
| 1 | Child 1, spec-agent-economy-1-call-shape | Closed. 18f0f40a19 (identity line), 614ce4ebea (guards keep HEAD's verdict; a modelled reader line lifts it; repairs the identity exemption), 48a98fb9d4 (refusal messages, a refused call costs nothing, test-deletion narrowing), 70f70633b3 (fixtures, parity, seal), accde6836c and 391cc12bc6 (closure). Live probe passed on 18f0f40a19's build: 0 of 5 calls voided | None |
| 2 | Child 2, spec-agent-economy-2-scoped-test | Closed. Phases 1 to 7: 75907fdffc, 7e072d42d3, db7126e71e, bcb429469e, 7e66c69bdd, ae3022e659 (with AC-13, `./le commit create ... body-file <path>`, and child 1's transferred findings I1, N1 to N5), 6cc68aaba9 (pages and the rule point); a91061b8c7 (review fixes), then the closure record and the spec's removal. Review clean in 2 rounds | None |
| 3 | Child 3, `spec-agent-economy-3-preamble.md` | `ready`, not started | Phase 1 is the probe and needs a session started after `./le ai sync update`, because agent definitions load at session start. Its review, and the review of the decisions-file move, are strongest-tier work |
| 4 | Child 4, `spec-agent-economy-4-supervision.md` | `skeleton` with the research; its Depends row is cleared | Design it (development tier) |
| 5 | Child 5, `spec-agent-economy-5-scoped-gates.md` | `skeleton`; its Depends row is cleared | Last, by design: it changes what two gates prove. Child 2's wait record now exists (db7126e71e). Its design is strongest-tier work |
| 6a | Child 6, `spec-agent-economy-6-shell-parser.md` (the hook parses bash with the vendored `mvdan.cc/sh`) | `in-progress`. Phase 1 done: 1fdd96d4d6 (vendor; it also carried the ledger row, which belonged in the importing commit). Phase 2 committed: e7e1d95731 (pre-parser fixes, the lone `.` behind a wrapper), 04caac807e (recorded verdict table), 1c2220b392 (AC-33..35), 47be60c067 (AC-37), ff53c0b945 (table review fixes: reader lines waive only liftable checks, deletion verdicts recorded, 1345-row floor), 2ec3034f3d (one parse per call, panic-safe). Phase 2 strongest-tier review: 0 BLOCKER, its 3 ISSUEs fixed. Still owed in phase 2: word-value reading, `TestWordValuesMatchBash`, `TestParsedCommandsMatchBash`, `FuzzShellParse`, `BenchmarkHookParse`, the A-3 record, the AC-28 fork side | Finish phase 2's owed items, then phase 3 (delete the splitter and allow-list, reader lines from the tree), in a fresh session from the spec and its state file `tmp/session/2026-11-11-021635/state/session-state-agent-economy-6-shell-parser-f4d858bd-ad8f-4b19-8a8d-50f67a40ccf9.md`. Phases 3 and 4 need strongest-tier review. The claim is held under an agent of the old session: release it with `./le spec release` behind that agent's identity, or claim at count+1 |
| 6 | The umbrella's acceptance criteria | AC-6 met (one six-word command answers one line on green) and AC-7 met (wait recorded) by child 2; the others need children 3 and 4 and the two-week window | Child 4 owns a kept measure (`./le ai tokens`); repeat the baseline with it |

## What every brief must say now

- A subagent's Bash command made only of readers in modelled syntax runs as
  typed; any other command starts with the identity line the agent's start
  text prints. The hook judges the command, not the files it runs: the check
  catches an agent that forgets or mistypes its identity and is not a sandbox.
- A subagent writes commit bodies with `body-file <path>` (the file written
  with the Write tool). Inline prose holding `).`, an apostrophe, an
  environment-changing word or a heavy-command quote is refused by the hook.
- One test package: `./le test unit pkg path <dir>/ [run <regex>] [vet]`.
- `ZE_SPEC_WIP_CAP` is the in-progress count plus one, never more.
- Running a refused command from a script file to get past the hook is not a
  route agents take, even when the refusal is false: report it instead.

## Observations not yet verified (check at the producer before recording)

| Observation | Source |
|-------------|--------|
| The spec-edit gate's producer-read check was satisfied by a 3-line Read of each producer, so it may prove little about reading | Reported by a child 2 agent, not read at the gate's code |
| Only one of four judged Weakening-approved lines was appended to ae3022e659; the gate may append only tests its detector flags | Not read at `./le commit create`'s gate |

## Decisions taken (do not put them to Thomas)

| Point | Decision | Why |
|-------|----------|-----|
| How the command rewrite is replaced | The agent types its identity line and the hook checks it | A rewritten call is one auto mode holds no verdict for |
| Guard verdicts after phase 2 | Every guard keeps HEAD's text verdict or a stricter one; only a line the syntax allow-list models, made of readers whose allow-list judges accept them, lifts it | Seven independent reviews found fail-open holes in every looser design |
| Phase 2 review loop | Ended under the stated threat model (mistakes, not an adversary) | Further findings needed deliberate obfuscation the hook cannot stop |
| List form of the short test action | Repeated keyword `path` | The existing list pattern |
| Race detector on the short action | Always on | A package tested without it is not tested the way the gate tests it |
| The decisions file split (D-1) | One file, two parts, wording unchanged | One home every harness loads |
| The style gate (D-3) | The page is split; the gate's code is untouched | "Read in full" stays literally true |
| Two model fields in the spec template | Decided by Thomas: `Design tier` and `Implementation tier` rows in `plan/templates/TEMPLATE.md`, values `development tier` or `strongest tier`. Optional on every spec, an absent row means the development tier, and no gate reads them | No sibling workflow field (`Handoff`, `Phase`, `Scope`) is validated, so no gate pattern exists to copy |
| "One spec's agents run other specs" | Decided by Thomas: a standing rule in `ai/META-DECISIONS.md`, "How agents work". Each claim stays inside the WIP cap; the umbrella's own cap exception stays a one-off | His order: incorporate the improvements |
| Running a refused command from a script file to get past the hook | Decided by Thomas: a standing rule in `ai/META-DECISIONS.md`, "How agents work" | His order: incorporate the improvements |

## Open with Thomas

| Question | State |
|----------|-------|
| None open | Thomas ordered the parser ("incorporate all the improvements you have identified") and approved `mvdan.cc/sh/v3/syntax` for `internal/le/hookruntime`; the strongest-tier review of child 1's identity exemption ran (0 BLOCKER, 3 ISSUE), and its findings are child 6's ACs 26 to 32 |

## Strongest-tier work that remains

The design of child 5; the review of child 3's implementation, above all the
bullet-by-bullet move of `ai/META-DECISIONS.md`; the review of child 4's
skill-body trimming.
