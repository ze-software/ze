# Closed spec survives on disk

A closure commit removes the spec from git, and the file stays in the working
tree as an untracked copy. The backlog then shows a closed spec as open.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-12 | ipsec-dataplane-inspection | `./le commit create ... remove <path>`, the generated commit script | Commit B removed `plan/immediate/spec-ipsec-dataplane-inspection.md` from git, and the file stayed on disk as an untracked copy byte-identical to the one commit A preserved. The script commits from a private index, so the removal is recorded in that index and the working-tree file is never unlinked. `./le spec status` and every backlog reader still count the spec as in-progress. The same residue was recorded for `spec-ipv6cp-accepts-and-proposes-a-zero-interface-identifier` in the 2026-09-09 handover, so this is the second occurrence | not fixed, recorded. Deleting the leftover needs the owner's word (`ai/rules/never-destroy-work.md`) and the content is preserved in the closure commit, so nothing is lost by leaving it. For whoever takes the mechanism: the removal block could unlink the path after the commit succeeds, which is the one moment the script knows the content is safe in git |
