---
kind: directive
level:
stage:
---
**Deriving the name:** the JSON key must match the YANG leaf name or config tree key.
A config key `remove-private-as` becomes `json:"remove-private-as"`, never
`remove-private` or `remove_private_as`. When no YANG leaf exists,
use the same kebab-case convention: lowercase words separated by hyphens.
