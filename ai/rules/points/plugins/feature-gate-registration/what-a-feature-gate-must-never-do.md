---
kind: directive
level:
stage:
---
- A hand-maintained second list of gate tags or gated packages anywhere. Declare the gate ONCE in `feature-gates.txt`; derive the rest.
- An always-on (untagged, non-test) `import` of a gated feature package. Route through the registry or a seam. `dep_audit.py --check` enforces this.
- A feature type in an always-on signature (`*zeweb.WebServer`, etc.). Use `Reconfigurable` or another always-on interface.
- Leaving a feature's borrowed helper in the feature package when always-on code needs it. Extract to `internal/core/*` first.
- Adding a gate without present/absent build-tag tests and an `nm` symbol check.
