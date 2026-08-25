# Rules-system gates -- MOVED to `le`.
#
# The gates themselves now live in scripts/le/application/check_rules.py. This
# file is a shim: each target forwards to `le` so that every existing caller
# keeps working -- CI, ze-doc-verify, ze-precommit-verify, and the several
# thousand `make ze-...` references across docs, rules and plan/.
#
# AGENTS AND HUMANS: use `le` directly. It is the source of truth.
#
#   ./le check-rules                        every check in this area
#   ./le check-rules --write                every generator, in the one correct order
#   ./le check-rules --list                 what each gate is for
#   ./le check-rules ze-rules-render-check  one gate
#
# THE GENERATOR ORDER MOVED WITH THE GATES, and it is no longer expressible
# here. `rules_index.py` and `rules_condensed.py` both parse the RENDERED
# rules, which `rules_points.py render` writes. Make enforced that with a
# prerequisite edge, because `make -j` honours prerequisite order and not
# recipe order: without it a digest could be built from pre-render text or from
# a torn read. The edges below are kept for exactly that reason, so a
# `make -j ze-generated-files-update` still orders correctly. `le check-rules
# --write` walks WRITE_ORDER instead, which is a list rather than a graph.
#
# Nothing here carries logic. A change to what a gate DOES belongs in the
# Python module; a change here can only break the forwarding. When the last
# caller stops spelling these as Make targets, this file is deleted.

.PHONY: ze-rules-index-update ze-rules-index-check ze-rules-condensed-update ze-rules-condensed-check ze-rules-points-roundtrip-check ze-rules-render-update ze-rules-render-check ze-rules-lint ze-discovery-index-update ze-discovery-index-check

ze-rules-render-check:
	@$(CURDIR)/le check-rules ze-rules-render-check

ze-rules-points-roundtrip-check:
	@$(CURDIR)/le check-rules ze-rules-points-roundtrip-check

ze-rules-lint:
	@$(CURDIR)/le check-rules ze-rules-lint

ze-rules-index-check:
	@$(CURDIR)/le check-rules ze-rules-index-check

ze-rules-condensed-check:
	@$(CURDIR)/le check-rules ze-rules-condensed-check

ze-discovery-index-check:
	@$(CURDIR)/le check-rules ze-discovery-index-check

ze-rules-render-update:
	@$(CURDIR)/le check-rules ze-rules-render-update

# The prerequisite edge is the ordering `make -j` actually enforces. Keep it.
ze-rules-index-update: ze-rules-render-update
	@$(CURDIR)/le check-rules ze-rules-index-update

ze-rules-condensed-update: ze-rules-render-update
	@$(CURDIR)/le check-rules ze-rules-condensed-update

ze-discovery-index-update:
	@$(CURDIR)/le check-rules ze-discovery-index-update
	@$(CURDIR)/le check-rules ze-docs-to-code-update
