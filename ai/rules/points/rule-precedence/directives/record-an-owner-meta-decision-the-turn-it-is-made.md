---
kind: directive
level: MUST
stage:
---
**When the owner states a decision about how agents work or what is in scope, the main session MUST append it to `ai/META-DECISIONS.md` in the same turn, with the date and his words, and commit it then.** A memory entry, a handover, a spec, a session state file and a Claude-only `.claude/` file MUST NOT be its only home: none of them loads into every agent's main session, so the decision is lost to the next one. A rule that applies the decision points at its row id rather than restating it, and a rule that contradicts the new row is rewritten in the same commit.
