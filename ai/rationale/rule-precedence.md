# Why the precedence ladder exists

Four directives give instructions about the same moment, and each was written
independently:

- no-asking in `ai/rules/completion.md`: finish the task, then report; ask only
  for destructive actions or genuine scope changes.
- model selection in `ai/rules/planning.md`: announce the boundary and stop
  rather than crossing it on the wrong model.
- spec delegation in `ai/rules/planning.md`: the main thread supervises only.
- `ai/rules/rfc-compliance.md`: implement full compliance, and ask only before
  doing LESS.

Each is right on its own. Read together with no ordering, they let an agent
justify almost any choice at the moment it is least able to reason carefully,
which is precisely when the wrong choice is expensive.

Rules that disagree almost always disagree about one thing: whether to keep
going. Naming the ladder costs one short rule and settles that question once,
so it is not re-litigated under context pressure.

Rule: `ai/rules/rule-precedence.md`.
