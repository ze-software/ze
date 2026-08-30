---
kind: directive
level: MUST NOT
stage:
---
**A rule is a DIRECTORY of points, and `ai/rules/<rule>.md` is GENERATED from it. You MUST edit the points; you MUST NOT edit the rendered file.** `writeRenderedRule` in `internal/le/hookruntime/writeedit.go` refuses the edit and names the canonical point directory. The layout, the manifest spine, the frontmatter fields, the header elements, the trigger requirements, the body budget, the renderer's refusals, the `// ze point:` binding and the generator order are all in `docs/contributing/rule-authoring.md`.
