---
kind: directive
level: MUST
stage:
---
- `.claude/hooks/block-premature-stop.sh` scans the last assistant message against a phrase list and exits 2 on the first match. Exit 2 refuses the session an end and returns the turn to the model. The hook is live and first in the `Stop` array since 2026-07-31, after it sat on no event from 2026-06-29 (`41e5fa44f`).
- Two lists, and only one of them is unconditional. `PHRASES` covers ownership-dodging, premature handoff and permission-seeking, and it always blocks.
- `COMPLETION_PHRASES` covers `what next`, `what would you like` and `what do you want to do`. These join the scan ONLY when work remains, which the hook reads as a claimed spec still `in-progress` (the `OPEN_WORK` flag). Asking what to do next is not the same failure as asking permission to do what was already requested. `.claude/rules/session-start.md` REQUIRES the question once the original task is done. The phrases were split rather than deleted, so the same words still block while a spec is open.
- **The retry bound is scoped to this scan, and it disables nothing else.** When the harness sets `stop_hook_active`, the flag `STOP_RETRY` skips the scan loop alone. That bounds a refusal loop whose only escape is rewording. The spec-closure gate above it still blocks on a retry, because that gate has two escapes of its own: run commit B, or write `tmp/session/.closure-ack-<stem>`. You MUST NOT read a blocked stop as a licence to stop next turn. The hook also exits 0 on input it cannot parse.
- A banned phrase inside backticks or a closed fence is treated as QUOTED, not used, and does not block. You MAY write about the phrases freely. Four guards keep that exemption from becoming a bypass. An unclosed fence is not a code block. A fence closes only on a run at least as long as the opener. The hook scans an all-markup message raw. Inline spans are stripped only on a line whose backticks balance, so one stray backtick cannot swallow a real request.
- Neither list is exhaustive, so a green Stop is not proof you followed this rule. You MUST finish the work, then report.
- Fixtures: `python3 scripts/dev/hook-fixture-check.py --only delegation`. Full hook map: `ai/rules/repo-maintenance.md`.
