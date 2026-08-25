# Verify helpers -- MOVED to `le`.
#
# The helpers themselves now live in scripts/le/application/helper_verify.py.
# This file is a shim: each target forwards to `le` so that every existing
# caller keeps working -- the rules, the hooks, and the `make ze-...`
# references across docs and plan/.
#
# AGENTS AND HUMANS: use `le` directly. It is the source of truth.
#
#   ./le helper-verify --list                 what each one is for
#   ./le helper-verify ze-working-tree-check  changed paths, grouped by area
#   ./le helper-verify ze-verify-worktree     the gate, against a COMMIT
#
# MAX_AREAS, COMMIT and KEEP moved with the targets and are read from the
# environment there. A variable set on the make command line reaches the
# recipe's environment, so `make ze-verify-worktree COMMIT=<rev> KEEP=1` still
# works and so does the same assignment in front of `./le`.
#
# Nothing here carries logic. A change to what a helper DOES belongs in the
# Python module; a change here can only break the forwarding.

.PHONY: ze-working-tree-check ze-verify-worktree

ze-working-tree-check:
	@$(CURDIR)/le helper-verify ze-working-tree-check

ze-verify-worktree:
	@$(CURDIR)/le helper-verify ze-verify-worktree
