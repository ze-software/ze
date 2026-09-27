---
kind: directive
level: MUST
stage:
rationale: ai/rationale/go-standards.md
---
**`docs/contributing/ze-go-style.md` MUST be read in full before a session's first Go edit.** `writeStyleGuideRead` in `internal/le/hookruntime/writeedit.go` refuses a Go Write, Edit or MultiEdit from a session or a subagent with no record of that read; Go written through a Bash heredoc never reaches it. The guide names every place Ze diverges from standard Go, and it carries the one obligation no rule file repeats: a peer MUST NOT be able to panic the daemon, so `panic("BUG:")` marks only a state a Ze defect reaches and a malformed message from a socket returns an error. Where the guide and a rule file disagree, the rule file wins.
