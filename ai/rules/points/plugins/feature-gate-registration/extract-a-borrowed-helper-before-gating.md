---
kind: note
level:
stage:
---
If always-on code needs a non-lifecycle helper the feature happens to export
(e.g. web exported cert generation to the installer), **extract that helper to an
always-on home FIRST** (`internal/core/*` leaf), then gate the feature. This is
"extract-then-gate"; the registry-ize is the easy half.
