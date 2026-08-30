# Rule File Format

**When:** authoring or editing any rule: a point file under `ai/rules/points/`, its manifest, or a check's binding comment
**Severity:** blocking
**Related:** repo-maintenance

## Directives

**A rule is a DIRECTORY of points, and `ai/rules/<rule>.md` is GENERATED from it. You MUST edit the points; you MUST NOT edit the rendered file.** `writeRenderedRule` in `internal/le/hookruntime/writeedit.go` refuses the edit and names the canonical point directory. The layout, the manifest spine, the frontmatter fields, the header elements, the trigger requirements, the body budget, the renderer's refusals, the `// ze point:` binding and the generator order are all in `docs/contributing/rule-authoring.md`.

## Every directive states a level

- Every point whose `kind` is `directive` MUST state its obligation in a capitalised RFC 2119 keyword, and its `level:` MUST name the strongest TIER the body states: MAY, then SHOULD with SHOULD NOT, then MUST with MUST NOT. A directive whose weight a reader infers from tone is a directive two readers weigh differently.
- The lowercase spellings `must`, `shall`, `should` and `may` MUST NOT appear in a directive body, and a block that states no obligation is `kind: note` or `kind: table`. `writePointLanguage` (`internal/le/hookruntime/writeedit.go`) refuses the write and `./le rules lint` refuses the finished tree. The accepted keywords and how each maps to a level are in `docs/contributing/rule-authoring.md`.
