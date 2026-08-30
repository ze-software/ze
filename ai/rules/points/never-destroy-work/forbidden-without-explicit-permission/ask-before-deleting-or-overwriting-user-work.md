---
kind: directive
level: MUST NOT
stage:
---
**Each operation below MUST NOT be performed without explicit permission:**
- `rm <path>` on any user-visible file, meaning any path that is not already untracked trash. Ask before deleting it, and never leave the file behind as a workaround when deletion is the correct fix.
- `git restore <path>` on a modified working-tree file, and `git reset --hard` or `git clean -f` anywhere. `ai/rules/git-safety.md` forbids these outright.
- Overwriting an existing file with content that drops user edits. Read the current file, then merge or ask.
- Truncating or overwriting a log file, session state, or a `tmp/` artifact the user might be inspecting. Leave it.
