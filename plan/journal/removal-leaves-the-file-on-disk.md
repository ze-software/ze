# Removal leaves the file on disk

A command removes a path from the tree it commits and does not remove it from
the working tree. The commit is right, `git log` is right, and the file is still
there, now untracked. Every gate that answers a question by asking the
FILESYSTEM then reads the old tree, and the one that reads git reads the new
one, so the two disagree with nothing saying which is current.

The tell is a gate that goes green over a removal it should have noticed, and a
`git status` carrying a `??` line for a path the last commit deleted.

`/ze-close` names the shape from the other side: `find_dangling` resolves a
citation with a filesystem test, "which sees an untracked file, so the working
tree reads green while the tree your commit produces is red". The same test that
hides a dangling citation hides the removal that created it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-06 | ze-test-dns-stub | `./le commit create ... remove <path>`, through `renderCommitBlock` and `renderSharedIndexRepair` (`internal/le/commit/script.go`) | The generated script removes the path with `git update-index --force-remove` against the private index and again against the shared one. Neither touches the working tree, and nothing in `prepare.go` does either. So closure commit B removed `plan/spec-ze-test-dns-stub.md` from git and left an identical untracked copy on disk. `./le spec citation` then answered `OK (322 specs)` over a tree that holds 321, because it resolves a spec by asking whether the file exists. The count moved to 321 only after the file was deleted by hand | Not fixed at the source. This closure removed its own residue with `rm` after confirming the on-disk bytes matched what commit A recorded, which is where the content survives. The source repair is two lines in `renderSharedIndexRepair`, after the commit has read the blob out of the private index, and it belongs to a deliberate pass rather than to a closing session: every commit in this repository flows through that script, the golden-text tests in `internal/le/commit` assert what it renders, and a wrong edit stops every other session committing |
