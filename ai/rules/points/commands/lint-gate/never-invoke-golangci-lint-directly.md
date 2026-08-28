---
kind: directive
level: MUST
stage:
rationale: plan/journal/gate-verdict-depends-on-the-machine.md
---
**You MUST lint through `./le verify-lint run`, never by calling
`golangci-lint` directly.** The native action derives the pinned toolchain and
every build flavor through `internal/le/verifylint`; a bare invocation inherits
host defaults and can report an environment failure as a code finding.

The same rule applies to every tool whose native action configures its
environment. The action is the interface; reaching past it drops the
configuration that makes the result representative.
