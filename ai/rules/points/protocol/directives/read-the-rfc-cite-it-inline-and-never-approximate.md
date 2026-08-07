---
kind: directive
level: MUST
stage:
---
- **When a spec lists RFC summaries in its Required Reading section, read ALL of them before making any design recommendations or protocol claims.**
- **Code that implements external APIs or protocols MUST reference the upstream spec inline.**
- **If the implementation cannot deliver EXACTLY what the operator's config asks for, `ze config verify` / `ze config commit` MUST fail with a clear error. Silent approximation, truncation, or "best-effort" mapping are bugs.**
- **One learned layout should fit every protocol (the holo-routing lesson: a fixed per-protocol skeleton makes each protocol navigable once you know one).** The skeleton below uses the package-naming glossary (`ai/rules/go-standards.md` "Package-Naming Glossary") and maps every existing protocol to it.
- **The skeleton is ADVISORY for existing code: no moves, no renames, no build gate. New protocols follow it; touched code adopts it opportunistically.**
- Conformance to the RFC text itself is governed by `ai/rules/rfc-compliance.md`, which stays a separate always-on rule.
