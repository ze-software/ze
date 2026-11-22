# Handover Documents

Live home for handovers: a handoff that must survive beyond the chat is written
here as `YYYY-MM-DD-<slug>.md`, dated the day it is written, so two sessions
writing at once never take the same number (the `NN-` files are older
handovers, kept under their names), and
removed through `./le commit create ... remove <path>` once every item in it has
landed (`ai/rules/completion.md`).

**This file exists so the directory does.** Git does not track empty
directories, so when the last handover is closed the directory disappears from
the tree, and every reference to `plan/handover/` in `ai/rules/planning.md` and
`ai/rules/CONDENSED.md` becomes a broken path -- `./le doc check links` (inside
`./le doc wiring`, a deterministic structural gate) then fails, and
the retired `scripts/dev/commit_helper.py` (current producer: `internal/le/commit/prepare.go`) refuses every commit in the repo until it is
fixed. That is what happened on 2026-07-27 after the 11 open handovers were
closed: 6 broken references, all to a directory whose absence was deliberate.

Deleting this README to "clean up" an empty directory reopens exactly that
hole. An empty `plan/handover/` is the normal, healthy state -- it means no
work is mid-handover -- and it should stay representable without breaking the
gate.

Do not add a handover here just to have one. The rule is that a handover is
written when work must cross a session boundary, not on a schedule.
