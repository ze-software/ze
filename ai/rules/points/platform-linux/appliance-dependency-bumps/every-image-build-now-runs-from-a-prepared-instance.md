---
kind: note
level:
stage:
---
That route is closed. **Every** image build now runs from a prepared copy of the
instance under the project `tmp/`, carrying the full `builddir` with its
filesystem-path replaces rewritten to absolute paths
(`internal/appliance/instance`). Both entry points go through it: `ze appliance
build` via `resolveBuildParentDir`, and `ze-gok` via `cmd/ze-gok`, which rewrites
`--parent_dir` before gok sees it. Preparation fails closed when the builddir is
missing or empty, rather than letting gok synthesize modules.
