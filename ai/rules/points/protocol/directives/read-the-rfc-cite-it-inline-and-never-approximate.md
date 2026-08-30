---
kind: directive
level: MUST
stage:
---
- **When a spec lists RFC summaries in its Required Reading section, you MUST read ALL of them before making any design recommendations or protocol claims.**
- **Code that implements external APIs or protocols MUST reference the upstream spec inline.**
- **If the implementation cannot deliver EXACTLY what the operator's config asks for, `ze config verify` / `ze config commit` MUST fail with a clear error. Silent approximation, truncation, or "best-effort" mapping are bugs.**
- **One learned layout SHOULD fit every protocol, so a reader who knows one protocol can navigate the next.** The skeleton speaks the package-naming glossary in `ai/rules/go-standards.md`, and `docs/architecture/protocol-skeleton.md` holds its modules and how each existing protocol maps to them.
- **The skeleton is ADVISORY for existing code: no moves, no renames, no build gate. New protocols SHOULD follow it; touched code MAY adopt it opportunistically.**
- Conformance to the RFC text itself is governed by `ai/rules/rfc-compliance.md`, which stays a separate always-on rule.
