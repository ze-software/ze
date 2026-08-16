---
kind: note
level:
stage:
---
> **format-alloc is live.** It is `c_format_alloc` in
> `.claude/hooks/pretool-writeedit.py`. The guarded list is current, and comment
> lines are exempt like `sprintf-new`. Its incremental value over `sprintf-new`
> is the `strings.Join`, `Builder`, `NewReplacer`, and `ReplaceAll` bans.
> Covered by `scripts/dev/hook-fixture-check.py` (`format-alloc-*`).
