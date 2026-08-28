---
kind: directive
level: MUST NOT
stage:
---
- A hand-maintained second list of gate tags or gated packages MUST NOT exist anywhere. Declare the gate ONCE in `feature-gates.txt`; derive the rest.
- An always-on (untagged, non-test) `import` of a gated feature package MUST NOT exist. Route through the registry or a seam. `./le tier check` enforces this.
- A feature type MUST NOT appear in an always-on signature (`*zeweb.WebServer`, etc.). Use `Reconfigurable` or another always-on interface.
- A feature's borrowed helper MUST NOT be left in the feature package when always-on code needs it. Extract to `internal/core/*` first.
- A gate MUST NOT be added without present/absent build-tag tests and an `nm` symbol check.
