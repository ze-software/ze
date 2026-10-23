# Closed spec survives on disk

A closure commit removes the spec from git, and the file stays in the working
tree as an untracked copy. The backlog then shows a closed spec as open.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-12 | ipsec-dataplane-inspection | `./le commit create ... remove <path>`, the generated commit script | Commit B removed `plan/immediate/spec-ipsec-dataplane-inspection.md` from git and the file stayed on disk untracked: the private-index script never unlinked it, so backlog readers still counted the spec. Second occurrence | fixed: `d70bbec7fc` ("a removal deletes the working-tree copy it committed"); `renderWorkingTreeRemoval` (`internal/le/commit/script.go`) unlinks each removed path after the commit lands, when the staged entry matches HEAD's |
