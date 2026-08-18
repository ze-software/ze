---
kind: directive
level: MUST NOT
stage:
---
**A summary's PROSE is a protocol-only reference document: it MUST NOT contain Ze-specific information -- no Ze implementation notes, no Ze file paths, no "Ze does/does not" statements, no "for ze" sections.** Implementation decisions belong in specs (`plan/`), architecture docs (`docs/architecture/`), or code comments. A reader SHOULD be able to read any `rfc/short/` file's prose as a standalone protocol reference with no knowledge of Ze.

**The `{...}` annotation on a requirement line is a SEPARATE register, and the constraint above does not reach it.** An annotation exists to say why this codebase owes less than the literal requirement, and `ai/rules/evidence.md` requires that justification to name the producing function. So a `{gap: ...}`, `{not-applicable: ...}`, `{partial: ...}` or `{single-polarity: ...}` MUST name the code it judges, file and symbol, and a search it relied on MUST be recorded the same way. An annotation that names no code is an assertion, and the two registers MUST NOT be traded against each other: stripping a path out of an annotation to satisfy the prose rule destroys the evidence the annotation exists to carry.

**The boundary is the line, not the file.** A requirement line and its wrapped continuation are annotation; every other line is prose. A Ze path that has escaped into a bullet, a heading or a paragraph is what this rule was written for, and it stays forbidden.
