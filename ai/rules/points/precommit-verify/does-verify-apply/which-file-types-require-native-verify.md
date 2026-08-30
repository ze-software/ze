---
kind: directive
level: MUST
stage:
---
**A commit whose files fall on a YES row MUST have the gate run over it; a
commit whose files all fall on the NO row MUST NOT pay for one.**

| Files in commit | Run `./le verify current mode full`? |
|-----------------|------------------|
| Any `.go`, `go.mod`, `go.sum`, `vendor/**` | YES |
| `internal/le/**`, `internal/test/**`, `internal/appliance/**`, build/CI config | YES |
| `*.yang`, generated code, codegen templates | YES |
| Anything that runs at build time or affects a binary | YES |
| `ai/**/*.md`, `.claude/**/*.md`, `plan/**/*.md`, `docs/**/*.md`, `README.md` | NO |
