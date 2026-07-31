# 1310 -- Phase gates: skills and model

## Context

Two rules were being ignored, and neither had anything to catch it. A session ran
five hand-written agent prompts where `/ze-explore` and `/ze-review` already
existed, and did every implementation edit on the review model without once
announcing the boundary. Both rules said the right thing. `model-selection.md`
even admitted "No hook or gate checks the running model."

## Decisions

- **Gate the model at the edit, not at session start.** A session cannot change its own model, so a start-of-session banner is read once and forgotten. The boundary is crossed at the first implementation edit, so that is where the check sits.
- **Read the model from the transcript.** The hook payload has no model field. It does carry `transcript_path`, and the transcript records the model on every assistant message. Reading a bounded tail of it is the only source available.
- **Block, with a recorded escape.** The escape is a file the operator's decision goes into, not a flag: `tmp/session/.model-ack-<sid>`. Same contract as the spec-closure ack.
- **Never gate `.md`.** Specs, designs and learned summaries are written during planning and review. Blocking them would break `/ze-spec` on the model it belongs to.
- **Stand down when the model is unknown.** An unreadable transcript must never stop work.
- **Match the ASK, not the subject, for the skills gate.** "Review this diff" gets routed to `/ze-review`. "Explain how review works" does not. Naming any `ze-*` skill in the prompt satisfies the gate, because that is deliberate routing.
- **Do not over-block delegation.** Plenty of agent work has no covering skill. A gate that made delegating harder would trade one failure for a worse one.
- **Put the skill names in the per-turn reminder.** The reminder already made delegation happen. It said nothing about skills, so the session delegated and improvised.

## Consequences

- Implementation on a review-tier model now stops at the first `.go`, `.py`, `.sh`, `.ci`, `.et`, `.yang`, `.mk`, `.tmpl` or `.rego` edit.
- Spawning a raw agent for research, review, spec review, debugging, implementation, hunting or auditing now fails and names the skill.
- The gate cannot see the phase, only the model. A one-line mechanical fix on the review model needs the ack too, which the rule's own exception would have allowed.
- Nothing checks the reverse direction. Reviewing on the implementation model is still on the session to notice.

## Gotchas

- **A "has it been routed already" test must check against the real names.** The first version matched any `ze-<word>`, so the repo path `ze-software` and any `make ze-verify` in a prompt switched the gate off. It now requires a slash and a skill that exists in `ai/skills/`.
- **A new gate breaks the workflows that do the very thing it watches for.** Three of the eleven fan-out prompts in `/ze-review-deep` tripped it. Those prompts now say which skill they serve, which is true and was worth stating anyway.
- **The harness passes absolute paths.** A `startswith("tmp/")` exclusion matched nothing, so scratch writes blocked, including the commit script `/ze-close` must write.

- **A fixture that shares live session state can pass while the gate does nothing.** The model fixtures first ran green because the session's own ack file was releasing the gate. They now move it aside and put it back.
- **Everything a gate reads must be isolated in its test, not just its inputs.** The transcript was faked from the start; the ack was not, because it lives in the real tree.

## Files

- `.claude/hooks/pretool-writeedit.py` -- `c_model_phase`, and the transcript reader
- `.claude/hooks/pretool-agent-skill.py` -- new, blocks a raw agent where a skill exists
- `.claude/hooks/delegation-reminder.sh` -- the per-turn line now names the skills
- `.claude/settings.json` -- the new `PreToolUse Task|Agent` registration
- `scripts/dev/hook-fixture-check.py` -- the `phase-gates` section, 17 fixtures
- `ai/rules/model-selection.md`, `ai/rules/agent-tooling.md`, `ai/rules/hook-mapping.md`
- `ai/rules/detail-budget.md` -- write like a person
