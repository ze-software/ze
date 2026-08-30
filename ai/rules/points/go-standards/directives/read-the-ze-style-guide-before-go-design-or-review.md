---
kind: directive
level: MUST
stage:
rationale: ai/rationale/go-standards.md
---
**`docs/contributing/ze-go-style.md` MUST be read at the START of every session, before any code (owner directive, 2026-08-18).** It names every place Ze diverges from standard Go, and it carries the one obligation no rule file repeats: a peer MUST NOT be able to panic the daemon, so `panic("BUG:")` marks only a state a Ze defect reaches and a malformed message from a socket returns an error. Where the guide and a rule file disagree, the rule file wins.
