---
name: No git reset - use restore or just commit
description: git reset is blocked by project hooks. Don't try it. If unstaging is also blocked, just commit what's staged.
type: feedback
---

Never use `git reset HEAD` to unstage files - it's blocked by project hooks.
Use `git restore --staged` if unstaging is needed.
If that's also denied, accept that staged files will be included in the commit and proceed.

**Why:** Project enforces git safety rules. `git reset` is a destructive command that's blocked.
**How to apply:** When needing to commit only specific files while other files are staged, try `git restore --staged` first. If denied, just commit everything together.
