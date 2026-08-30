---
kind: directive
level: MUST
stage:
---
- **A gated package that lives INSIDE another gate's package tree and imports it is a DEPENDENT piece, and its present build-tag test MUST carry the same compound constraint as the generated group file.** Otherwise the test runs in a build combination that cannot exist. The generator derives the constraint from the package path, so the manifest line stays the ordinary `<tag> <pkg>` with no new column. `ze_bmp` inside `ze_bgp` is the worked example, in `docs/architecture/plugin/feature-gates.md`.
