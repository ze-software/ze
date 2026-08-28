---
kind: note
level:
stage:
---
A rule is a DIRECTORY of points, and `ai/rules/<rule>.md` is generated from it. An agent reads the rendered file and gets the same bytes it always got. An author edits the points. `writeRenderedRule` in `internal/le/hookruntime/writeedit.go` refuses an edit to the rendered file and names the canonical point directory.
