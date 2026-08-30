---
kind: directive
level: MUST NOT
stage:
---
- **An always-on (untagged, non-test) package MUST NOT import a gated feature package, for ANY reason: lifecycle or a borrowed helper.** One direct import pins the package into every binary and defeats the compile-out, and `./le tier check` fails on it. Always-on code reaches a feature only through build-tag-gated registration.
