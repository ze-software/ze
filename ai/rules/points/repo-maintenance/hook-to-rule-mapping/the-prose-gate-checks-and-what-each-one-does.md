---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `ste_problems` (`commit_helper.py`) | `writing.md` | `commit_helper.py create` with any `.md`, `.go`, or `.yang` in the commit | Runs `ste_check.py --check` over the FILES OF THAT COMMIT and PRINTS the six banned ASD-STE100 habits that grew against HEAD. Advisory: STE is a guideline, so this never refuses a commit. Commit scope is deliberate: several sessions share this checkout, so a tree-wide prose gate judges a colleague's in-flight sentences and gets switched off. BLOCKING. |
| `make ze-ste-check` | `writing.md` | on demand, before you prepare a commit | The same comparison over every changed file in the working tree. It prints the file, the habit, and only the new findings. Surfaces are Markdown in `docs/`, `ai/`, `plan/`, and the root, prose comments in `.go`, and `description` strings in `.yang`. Renames follow `-M`, so a moved legacy document does not report its inherited content as new. Deliberately NOT in `ze-doc-test`. Whole-tree report: `make ze-ste-review`. |
