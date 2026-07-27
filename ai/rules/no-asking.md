# Don't Ask, Do

**When:** Never use phrases like "would you like me to", "want me to", "shall I",
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
