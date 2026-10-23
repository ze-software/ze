---
kind: directive
level: MUST
stage:
---
**When the owner states a rule about how agents work or what is in scope, the main session MUST add it to `ai/META-DECISIONS.md` as a plain rule in the right topic section, in the same turn, and commit it then. The entry MUST NOT carry an id, a date or a quote.** A memory entry, a handover, a spec, a session state file and a Claude-only `.claude/` file MUST NOT be its only home: none of them loads into every agent's main session, so the rule is lost to the next one. A one-off permission for a single piece of work, such as a cap exception or approval to change named tests, is not such a rule: it goes in the spec or commit it applies to, never in that file. A rule elsewhere that depends on it MUST state the rule rather than cite it, and a rule that contradicts it is rewritten in the same commit.
