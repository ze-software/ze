---
kind: note
level:
stage:
---
A feature is compile-out-able **only when nothing always-on (untagged, non-test)
imports its package** for ANY reason: lifecycle OR a borrowed helper. Always-on
code reaches it ONLY through build-tag-gated registration. A single direct
`import` from untagged code pins the package into every binary and defeats the
compile-out. `scripts/dev/dep_audit.py --check` (run by `make ze-verify`, target
`ze-tier-check`) fails on any such importer.
