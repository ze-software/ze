---
kind: note
level:
stage:
---
For fingerprint/grouping key builders with repeated `|` separators, embed
`strings.Builder` in a package-local `keyBuilder` with typed methods:
