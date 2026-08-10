---
kind: directive
level: MUST
stage:
---
- **The manifest owns the whole spine.** Its body carries one section line, `<dir-slug> ## The Heading Verbatim`, then that section's point slugs indented by two spaces. The renderer emits the heading and those bodies in that order, joined by one blank line. A reorder is a manifest edit, never a rename.
- **The path is the id, and it is always `<rule>/<section>/<slug>`.** A `##` heading is the section DIRECTORY; a `###` or `####` heading stays a point inside it, so the depth is fixed at two. There is no content hash and no numeric prefix. A reworded instruction is a one-file diff, `git log` on the point file dates it, and every gate bound to that point keeps working.
- **MUST add a point in two edits: write the file in its section directory, then list its slug under that section in the manifest.** A point on disk that no manifest lists is a hard error, and so is a `*.md` sitting outside every section, so the file alone changes nothing.
- **A Write over a point that already exists is REFUSED.** `c_point_overwrite` in `.claude/hooks/pretool-writeedit.py` blocks it and names both routes: MAY edit the point that is there, or pick a slug no file in that directory uses. An Edit, and a Write to a free slug, stay permitted.
- **A body is verbatim.** The renderer concatenates bodies and rewrites nothing inside one, which is what makes `make ze-rules-points-roundtrip` a proof rather than a comparison.
