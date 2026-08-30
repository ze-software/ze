---
kind: directive
level: MUST NOT
stage:
---
YANG schemas describe command structure and semantics, not CLI presentation.
`--flag` syntax MUST NOT appear anywhere in a `.yang` file: not in a `description`,
not in a `//` comment, not in examples.
