---
kind: table
level:
stage:
---
| Files in commit | Run `ze-verify`? |
|-----------------|------------------|
| Any `.go`, `go.mod`, `go.sum`, `vendor/**` | YES |
| `Makefile`, `scripts/**`, build/CI config | YES |
| `*.yang`, generated code, codegen templates | YES |
| Anything that runs at build time or affects a binary | YES |
| `ai/**/*.md`, `.claude/**/*.md`, `plan/**/*.md`, `docs/**/*.md`, `README.md` | NO |
