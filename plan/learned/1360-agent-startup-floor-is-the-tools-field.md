# 1360 -- An Agent's Startup Floor Is Set by Its `tools:` Field, and `Explore` Is Cheap Because It Is Ignorant

## Context

`make ze-token-economy` reads the transcript store and says that subagents feed 69% of
all context, and that `general-purpose` agents are 96.7% of that. Every `ze-*` skill said
"spawn an agent" and named no `subagent_type`, so every phase ran on `general-purpose`.
`.claude/agents/` held one file.

Four probes with an identical prompt and zero tool calls looked decisive: `Explore` 8,515
tokens and `Plan` 8,637 against `general-purpose` 49,140 and `lsp-probe` 48,752. The two
cheap ones carry a `tools:` list. The two expensive ones do not. The conclusion drawn was
that the field was worth about 40k, and it was written into eleven files.

It was wrong. A concurrent session picked up the new `ze-read` definition and its real
startup was 43,106, not 8,600. Comparing agents ACROSS populations had varied two things
at once, and the 40k was mostly the other one.

## Decisions

- Re-measured INSIDE one session, which holds the hooks, the preamble and the machine
  constant. Reading of session 5f9f4faa: `general-purpose` 49,537, `lsp-probe` 48,726,
  `ze-work` 43,566, `ze-read` 42,956, `Plan` 8,612, `Explore` 8,507. The `tools:` field
  is worth about 6k, not 40k. Corrected every file.
- Added `--session` to `token_economy.py` so that reading is reproducible from the
  command the rule names. Without it the table medians over every session, and the
  preamble's own growth swamps the term being measured.
- **Made the column subtract the spawn prompt, because round 2 of review caught the
  number DRIFTING.** The first table medianed raw first-call context over every agent of
  a type, working agents included, so it moved whenever another one spawned. A figure
  cited at 49,715 read 50,181 an hour later. `Agent.harness_floor` now takes the prompt
  back out, which makes the column a property of the agent TYPE and makes two runs of
  the command agree. A `fork` reports 0 there, because it inherits its parent's context.
- Found what the other 34k is by asking each agent what its own instructions contain.
  `general-purpose` answers yes to `ABSOLUTE PROHIBITIONS`, to the STE line and to
  `Ze Rules -- Always-On Core`. `Explore` answers no to all three. It is cheap because it
  receives none of this repository's rules.
- So routing review or research to `Explore` is now BANNED reasoning, not an
  optimization. It buys tokens by making the agent ignorant of `evidence.md`,
  `rfc-compliance.md` and `completion.md`. `context-economy.md` already refused that
  trade under "What This Rule Never Targets". The banned-reasoning table now names this
  specific form of it.
- Added `ai/agents/`, synced to `.claude/agents/` by `scripts/dev/skill_sync.sh`, with
  `ze-read` for read-only phases and `ze-work` for editing ones. `ze-work` keeps Edit,
  Write and NotebookEdit and pays the same lower floor. The field's PRESENCE is the term,
  never the list's length.
- `ai/agents/lsp-probe.md` keeps NO `tools:` field, and says why in the file. It exists to
  measure what the harness serves by default, so listing tools would make it prove only
  that the list was written.
- **A `tools:` name the harness does not serve is dropped in SILENCE.** Review asked
  whether the names were real. The probe said six of the twelve were not. `LSP`,
  `ReportFindings`, `TaskCreate`, `TaskUpdate`, `TaskList` and `TaskGet` were all listed
  and none arrived. The agent loads either way, so the only symptom is a capability that
  is quietly absent. Both lists were cut to the names a probe confirmed. `Monitor` and
  `TaskOutput` were proposed in review as the fix for a real gap, and probed too: this
  harness serves neither, so each agent file carries a directive in place of the tool.
- No LSP means every symbol question in a phase agent goes to `gopls` from Bash. That is
  the second route `context-economy.md` already blesses, so the capability survives.
- `make ze-token-economy` now prints the floor per agent type, because
  `context-economy.md` requires every figure in it to be reproducible from a named
  command, and a probe result is not.

## Consequences

The saving is about 6k times a subagent's call count, which measured at 53 calls per
agent is roughly 320k per agent, lossless. The dominant term is not the tools field: it is the ~34k
preamble that `general-purpose` carries and `Explore` does not, re-fed on every call. That
one is not free to cut, because the preamble is the rules.

A new agent definition takes effect in the NEXT session. Claude Code reads
`.claude/agents/` at session start, so the session that writes one cannot spawn it, and
cannot measure it either.

**The reusable lesson is about the measurement, not the agents.** Four clean probes with
an identical prompt still produced a wrong number. They compared across populations that
differed in more than the variable under test. A controlled comparison
holds the environment fixed and varies one thing. When the two are available, the
cross-population one is a hypothesis and the same-session one is the measurement.

## Files

Hand-edited sources only. The generated artifacts that follow from them
(`ai/rules/context-economy.md`, `.claude/agents/`, `.claude/skills/`,
`ai/LEARNED-FULL-INDEX.md`) are regenerated and committed alongside.

- `ai/agents/ze-read.md` -- new read-only phase agent, tool list cut to the names a probe confirmed
- `ai/agents/ze-work.md` -- new editing phase agent, same treatment, plus the no-Monitor directive
- `ai/agents/lsp-probe.md` -- moved from `.claude/agents/`, keeps no `tools:` field on purpose
- `scripts/dev/skill_sync.sh` -- syncs `ai/agents/` to `.claude/agents/` beside the skills
- `scripts/dev/token_economy.py` -- `Agent.harness_floor`, `agent_type_startup`, `--session`
- `scripts/dev/token_economy_test.py` -- 9 tests over the new function and the filter
- `scripts/dev/check_doc_links.py` -- `ai/agents/*.md` joins the checked corpus
- `scripts/dev/commit_helper.py` -- `ai/agents/` is a lesson-worthy surface
- `mk/inventory.mk` -- `ZE_SESSION` for the session-scoped reading
- `ai/rules/points/context-economy/directives/agent-startup-floor-is-set-by-the-tools-field.md` -- the directive
- `ai/rules/points/context-economy/banned-reasoning/banned-excuses-and-what-the-numbers-say.md` -- three rows
- `ai/skills/ze-explore.md` -- routes to `ze-read`
- `ai/skills/ze-review.md` -- routes to `ze-read`
- `ai/skills/ze-review-spec.md` -- routes to `ze-read`
- `ai/skills/ze-audit.md` -- routes to `ze-read`
- `ai/skills/ze-verify.md` -- routes to `ze-read`
- `ai/skills/ze-hunt.md` -- routes to `ze-read`, and its two LSP steps gained the gopls route
- `ai/skills/ze-implement.md` -- routes to `ze-work`
- `ai/skills/ze-debug.md` -- routes every lens to `ze-work`, because each one lands its own fix
- `ai/skills/ze-close.md` -- routes nowhere, and says why: step 5 spawns reviewers
- `.gitignore` -- `.claude/agents/` joins the generated targets
