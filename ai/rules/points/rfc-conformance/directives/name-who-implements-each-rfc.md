---
kind: directive
level: MUST
stage:
rationale: ai/rationale/rfc-compliance.md
---
**Every summary MUST declare who implements the document, and a document Ze does not implement in Go MUST leave the conformance count and name what implements it instead.** The ledger answers one question: does Ze's Go code do what the RFC says. Four kinds answer it, and every summary declares exactly one.

| Kind | What it says | Counted |
|------|--------------|---------|
| `ze` | Ze implements the document's obligations in Go | yes |
| `mixed` | Ze implements part in Go and another layer performs the rest | Ze's part only |
| `third-party` | a layer under or beside Ze performs it and Ze holds no Go code for it | no |
| `foundation` | the document defines, registers or describes, and obliges no implementer | no |
