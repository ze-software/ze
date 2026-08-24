# Verify helpers: what the working tree holds, and the throwaway-worktree gate run
#
# Quick reference:
#   make ze-working-tree-check  Changed paths, grouped by area
#   make ze-verify-worktree     Run the gate against a COMMIT, not this tree
#

.PHONY: ze-working-tree-check ze-verify-worktree

# How wide the uncommitted tree is, grouped by area. Advisory: it reports and
# exits 0, because only a person can say whether two areas are one logical
# change. The failure it exists to surface is several FINISHED chunks held in
# one tree, which a checkout destroys and which every later chunk must be
# diffed around (ai/rules/git-safety.md, "Commit Granularity").
# Pass MAX_AREAS=N to make it a gate.
ze-working-tree-check:
	@python3 scripts/dev/working_tree_check.py $(if $(MAX_AREAS),--max-areas $(MAX_AREAS),)

# Run the pre-commit gate against a COMMIT, in a throwaway worktree, so the
# working tree stays free and no mid-run edit can invalidate the result.
# The gate is 25-53 minutes on this hardware (measured 2026-08-21), which is
# why verifying in place pushes people to batch commits.
# COMMIT=<rev> picks the commit (default HEAD), KEEP=1 leaves the worktree.
# See ai/rules/git-safety.md, "Verify a Commit, Not the Working Tree".
ze-verify-worktree:
	@python3 scripts/dev/verify_worktree.py \
		$(if $(COMMIT),--commit $(COMMIT),) $(if $(KEEP),--keep,)
