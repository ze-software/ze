---
kind: note
level:
stage:
---
`internal/le/protocolskeleton/protocolskeleton.go` lists each protocol's modules classified against the skeleton (canonical / RFC-named state / version dir / domain / legacy exception) and prints a one-line summary by default (`--verbose` for the full table). It ALWAYS exits 0 in report mode: it is an advisory lens, not a gate (an enforced skeleton today would need a large allowlist; see the tiers Path B lesson in `spec-tiers-0-umbrella`, closed). It runs as the last, non-enforcing line of `./le tier check`.
<!-- source: internal/le/protocolskeleton/protocolskeleton.go -- classifier and manifest -->
