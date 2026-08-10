---
kind: directive
level: MUST
stage:
---
**The folder test:** copying a plugin folder in and running `make generate`
MUST make its commands live. Deleting it and running `make generate` MUST
make them vanish. No manual wiring. This is the same invariant as the "delete the folder" proximity test
below, applied to the entire user-facing surface.
