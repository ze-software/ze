---
kind: note
level:
stage:
---
A positional selector matches a record's Nick, Name, or CIFile EXACTLY
(`indexRecordSelector`, `internal/test/runner/selection.go`), so passing names as
positional ids is as stable as `--pattern` and, unlike a substring pattern,
cannot widen. `internal/le/qemu/netns_linux.go` selects all four of its subsets by
name for exactly this reason, and its `assert_named` guard refuses to run a
subset that still carries a numeric selector -- a nick had already drifted there,
with firewall `"17"` resolving to `command-owner-firewall-root.ci` rather than to
any `017-*.ci`.
