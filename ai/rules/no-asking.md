# Don't Ask, Do

**When:** when you are about to ask permission instead of finishing the work
**Severity:** advisory

## Directives

Never use phrases like "would you like me to", "want me to", "shall I",
or "I can" before completing work. Finish the task first, then report
what was done. The user delegated the work; asking for permission to
start it wastes a round-trip.

Exception: genuinely ambiguous scope or destructive actions that require
confirmation per the git safety rules.

Standing exceptions, where asking is MANDATORY and this rule does not apply:

- **RFC compliance.** When full RFC compliance and full testing of that compliance is one of the answers on the table, stop and ask Thomas rather than choosing anything narrower (`ai/rules/rfc-compliance.md`, "Ask Thomas Whenever Full Compliance Is On The Table"). Asking is required only when you are about to do LESS; doing more never needs permission.
- **Deleting or overwriting user-visible or uncommitted work** (`ai/rules/never-destroy-work.md`).
- **Reducing the scope of a spec or dropping an acceptance criterion** (`ai/rules/no-partial-completion.md`).

## Enforcement

This rule is hook-enforced. Breaking it costs a blocked Stop, not a note.

- `.claude/hooks/block-premature-stop.sh` scans the last assistant message against a phrase list and exits 2 on the first match (`:232-239`, `:258-267`). Exit 2 refuses the session an end and returns the turn to the model. The hook is live and first in the `Stop` array since 2026-07-31, after it sat on no event from 2026-06-29 (`41e5fa44f`).
- Two lists, and only one of them is unconditional. `PHRASES` (`:92-121`) covers ownership-dodging, premature handoff and permission-seeking, and it always blocks.
- `COMPLETION_PHRASES` (`:133-138`) covers `what next`, `what would you like` and `what do you want to do`. These join the scan ONLY when work remains, which the hook reads as a claimed spec still `in-progress` (`OPEN_WORK=1` at `:193`, assembled at `:225-228`). Asking what to do next is not the same failure as asking permission to do what was already requested. `.claude/rules/session-start.md:72` REQUIRES the question once the original task is done. The phrases were split rather than deleted, so the same words still block while a spec is open.
- **The retry bound is scoped to this scan, and it disables nothing else.** When the harness sets `stop_hook_active`, the flag `STOP_RETRY` (`:26-29`) skips the scan loop alone (`:232-239`). That bounds a refusal loop whose only escape is rewording. The spec-closure gate above it still blocks on a retry, because that gate has two escapes of its own: run commit B, or write `tmp/session/.closure-ack-<stem>`. Do not read a blocked stop as a licence to stop next turn. The hook also exits 0 on input it cannot parse (`:36-37`).
- A banned phrase inside backticks or a closed fence is treated as QUOTED, not used, and does not block (`:69-84`). Write about the phrases freely. Four guards keep that exemption from becoming a bypass (`:51-63`). An unclosed fence is not a code block. A fence closes only on a run at least as long as the opener. The hook scans an all-markup message raw. Inline spans are stripped only on a line whose backticks balance, so one stray backtick cannot swallow a real request.
- Neither list is exhaustive, so a green Stop is not proof you followed this rule. Finish the work, then report.
- Fixtures: `python3 scripts/dev/hook-fixture-check.py --only delegation`. Full hook map: `ai/rules/hook-mapping.md`.
