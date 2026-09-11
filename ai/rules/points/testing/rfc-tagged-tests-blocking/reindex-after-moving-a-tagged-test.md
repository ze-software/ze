---
kind: directive
level: MUST
stage:
---
- **A tagged test that is added, moved, deleted or re-tagged MUST NOT be followed by a regeneration, and `ai/RFC-REQUIREMENTS.md`, `rfc/requirements/`, `rfc/enrolled.txt`, `rfc/not-enrolled.txt` and `docs/features/rfc-status.md` MUST NOT be committed**: the five are derived and untracked (`internal/le/rfc/register.go`). Writing a tag carrier REMOVES them, a command that names one REBUILDS it, and `./le rfc check` reads the summaries and the tags rather than any generated file. Until 2026-09-11 both outputs were owed in the same commit, and a session that regenerated from a shared checkout carried other sessions' tags into its own message.
- **Which carrier a tag MAY live in, and what evidence kind and tier it earns, is `docs/contributing/rfc-implementation-guide.md`.** A tier is derived from the carrier and MUST NOT be declared by the test.
