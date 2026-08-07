---
kind: table
level:
stage:
---
| The renderer refuses | Why |
|----------------------|-----|
| A point file the manifest does not list | The manifest is the reading order, so an unlisted point would vanish from the rule with nothing going red |
| A `*.md` sitting directly in a rule directory | It is outside every section, so it has no `<rule>/<section>/<slug>` id and nothing renders it |
| A section directory the manifest does not list, or a section listing no point | The same silent drop, one level up: a whole section leaves the rule |
| A manifest slug or section with no file or directory on disk | Half a rule is worse than a failed render |
| The same slug twice in one section, or the same section twice | The second entry would emit the same body a second time |
| A slug carrying a path separator, a leading dot, or a parent reference | A manifest MUST NOT make the renderer read outside its own rule directory |
