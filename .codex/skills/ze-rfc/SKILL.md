---
name: ze-rfc
description: Use when working in the Ze repo and the user asks for ze-rfc or wants an RFC turned into an implementation summary. Read the RFC text from the repo, write a structured summary under docs, include wire formats, requirements, errors, constants, and errata, and verify the summary against the source text.
---

# Ze RFC

This skill turns an RFC text file in the repo into an implementation-oriented summary.

## Workflow

1. Read `rfc/<id>.txt` from the repository.
2. Draft `docs/architecture/rfc/<id>.md` with sections for metadata, wire formats, encoding and decoding rules, validation, MUST/SHOULD/MAY requirements, errors, state, timers, constants, algorithms, pitfalls, and compatibility.
3. Check the RFC errata for the same document.
4. Re-read both the RFC and the summary to verify field sizes, section references, requirements, and edge cases.

## Rules

- Read the RFC text directly; do not rely on memory.
- Keep only the implementation-relevant parts.
- Copy ASCII wire diagrams exactly when the RFC provides them.
