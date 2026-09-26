---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/principles.md
---
**A value that is silently wrong MUST NOT be reachable: code that cannot answer MUST say so, and MUST NOT return zero, nil, false, empty, or the default in place of an answer.** A caller cannot tell a real zero from a failure that produced one, so the defect surfaces far from its cause and reads as data. Its shapes: a type assertion that fails and disables a feature with no log line, a cross-boundary call that no-ops when the plugin runs external, a search whose zero hits are read as absence, a test whose passing assertion would also pass against a stub. **A zero that another branch relies on is a guard, and it MUST be named as one.** A test asking which value to use can also answer whether the action is permitted, purely because the value stays zero until permission exists: nothing names the second job, no test covers it, and the guard vanishes the moment a legitimate zero arrives, with no line deleted and no test red.
