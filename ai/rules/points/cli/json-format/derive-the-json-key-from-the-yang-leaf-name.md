---
kind: directive
level: MUST
stage:
---
**Deriving the name:** the JSON key MUST match the YANG leaf name or config tree key.
A config key `remove-private-as` becomes `json:"remove-private-as"`, MUST NOT be
`remove-private` or `remove_private_as`. When no YANG leaf exists,
MUST use the same kebab-case convention: lowercase words separated by hyphens.
