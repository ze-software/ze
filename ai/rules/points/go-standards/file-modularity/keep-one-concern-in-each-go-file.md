---
kind: directive
level: MUST
stage:
rationale: ai/rationale/file-modularity.md
---
**Each `.go` source file MUST hold exactly one concern: a cohesive group of types and functions serving one responsibility.** The size thresholds and what to do at each one are in `docs/contributing/ze-go-style.md`, "The shape of a function".
**A split MUST be made only when the separation is RIGHT.** A forced mechanical split that scatters one concern across files is worse than one large cohesive file, which is why the post-edit size warning is non-blocking. Read it as a prompt to check cohesion, never as an order to cut.
**A `_test.go` file is NOT subject to a line-count threshold.** Tests grow with coverage and table-driven cases, and splitting them adds navigation cost without improving production code.
