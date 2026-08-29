---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `steProblems` (`internal/le/commit`) | `writing.md` | `./le commit create` with any `.md`, `.go`, or `.yang` in the commit | Calls the native `internal/le/ste` ratchet over the commit's files and prints the six ASD-STE100 habits that grew against HEAD. Advisory: STE is a guideline, so this never refuses a commit. |
| `./le ste check` | `writing.md` | on demand, before you prepare a commit | The same comparison over every changed file in the working tree. It prints the file, the habit, and only the new findings. Surfaces are Markdown in `docs/`, `ai/`, `plan/`, and the root, prose comments in `.go`, and `description` strings in `.yang`. Renames follow `-M`, so a moved legacy document does not report its inherited content as new. Deliberately NOT in `./le doc check verify`. Whole-tree report: `./le ste review`. |
