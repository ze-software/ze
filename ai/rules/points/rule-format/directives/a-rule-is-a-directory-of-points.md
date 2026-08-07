---
kind: note
level:
stage:
---
A rule is a DIRECTORY of points, and `ai/rules/<rule>.md` is generated from it. An agent reads the rendered file and gets the same bytes it always got. An author edits the points. An edit to the rendered file is refused by `c_rendered_rules` in `.claude/hooks/pretool-writeedit.py`, which names the point directory to edit instead.
