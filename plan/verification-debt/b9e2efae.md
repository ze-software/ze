# Verification debt -- commit session b9e2efae

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-22 | b9e2efae | qemu: name the cross-address SSH forward race | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-19T14:57:42Z) | open |
| 2026-10-22 | b9e2efae | qemu: name the cross-address SSH forward race | native structural checks (red) | arch tier/check and arch iface-resolution are red only in the latest recorded verify run; both pass on the current tree (le arch tier check: all OK; le arch iface-resolution: OK), and this commit touches neither surface | open |
| 2026-10-22 | b9e2efae | qemu: name the cross-address SSH forward race | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
