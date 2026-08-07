---
kind: note
level:
stage:
---
Both were live between 2026-07-18 and 2026-07-22: the derived (hugepage) parent
handed `gok` an instance with no `builddir`, so every pin was discarded
(against `vendor/github.com/gokrazy/tools/packer/gotool.go` `getPkg`/`getIncomplete`).
