---
kind: directive
level: MUST
stage:
---
- `hookStop` in `internal/le/hookruntime/lifecycle.go` scans the last assistant message and refuses a permission-seeking stop.
- The unconditional phrase list covers ownership-dodging and premature handoff. Completion questions join only while a claimed spec remains in progress.
- The harness retry flag skips the phrase scan alone. Spec-closure checks remain active, so a blocked stop is not permission to stop on the next turn.
- A banned phrase inside backticks or a closed fence is treated as QUOTED, not used, and does not block. You MAY write about the phrases freely. Four guards keep that exemption from becoming a bypass. An unclosed fence is not a code block. A fence closes only on a run at least as long as the opener. The hook scans an all-markup message raw. Inline spans are stripped only on a line whose backticks balance, so one stray backtick cannot swallow a real request.
- **A blocked Stop is not an instruction to do the work you just offered.** The block asks who wanted that work. The user wanted it: finish it, and do not ask again. You thought of it: DROP it, and MUST NOT start it, size it, or offer it a second time. The remedy read `Continue without asking permission` until 2026-08-19, which answered permission-seeking and misread an offer: a turn ending `Want me to spec the streaming writer?` was refused its end and then went and wrote that spec, so the gate against uncommissioned work was producing it.
- Neither list is exhaustive, so a green Stop is not proof you followed this rule. You MUST finish the work, then report.
- Fixtures: `./le hook-check unit`. Full hook map: `ai/rules/repo-maintenance.md`.
