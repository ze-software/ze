---
kind: note
level:
stage:
---
`tmp/` is shared by every concurrent session in this checkout (it is keyed
per-checkout, not per-session -- `scripts/dev/ensure-links.py`). A fixed name at
the `tmp/` root -- `tmp/out.log`, `tmp/stdout`, `tmp/gotest.log` -- collides with
a sibling session writing the same name, and is never cleaned when your session
ends.

**A file at the `tmp/` root is REFUSED, on both surfaces that create one**:
`check_scratch_path` on a Bash redirect or `tee`, `c_scratch_path_we` on Write
and Edit. Both call `.claude/hooks/lib/scratch_path.py`, so the two surfaces
answer alike. A path carrying a directory component passes, which covers
`tmp/session/<YYYY-MM-DD>-<sid>/` and every producer's own folder. The root
names that are session-keyed or shared by design pass too:
`ze-precommit-verify*`, `.ze-verify*`, `commit-*`, `commit-msg-*`, `delete-*`,
`mutation*`, `test-timings*`.
