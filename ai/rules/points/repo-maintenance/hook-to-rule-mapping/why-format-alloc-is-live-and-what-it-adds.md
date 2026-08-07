---
kind: note
level:
stage:
---
> **format-alloc is now live** (enabled 2026-07-09, spec-followup-hooks). It is
> `c_format_alloc` in `.claude/hooks/pretool-writeedit.py`. The retired shell
> version used bash-4 `declare -A`, which the macOS bash
> 3.2 shebang could not run, so it exited 0 and never enforced anything. The
> guarded list is now current (`bgp/attribute/text.go` removed with the attribute
> package in `3e66070f8`; `bgp/format/json.go` added) and comment lines are exempt
> like `sprintf-new`. Its incremental value over `sprintf-new` (which already bans
> `fmt.Sprintf`/`Fprintf` + `strconv.Format*` everywhere) is the `strings.Join`/
> `Builder`/`NewReplacer`/`ReplaceAll` bans. Covered by
> `scripts/dev/hook-fixture-check.py` (`format-alloc-*`).
