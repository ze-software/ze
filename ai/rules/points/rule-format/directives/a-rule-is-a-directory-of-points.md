---
kind: directive
level: MUST NOT
stage:
---
**A rule is a DIRECTORY of points, and `ai/rules/<rule>.md` is GENERATED from it. You MUST edit the points; you MUST NOT edit the rendered file.** `writeRenderedRule` in `internal/le/hookruntime/writeedit.go` refuses the edit and names the canonical point directory. The layout, the frontmatter fields, the renderer's refusals and the generator order are in `docs/contributing/rule-authoring.md`.
