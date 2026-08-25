# Repository reports -- MOVED to `le`.
#
# The reports themselves now live in scripts/le/application/report_inventory.py.
# This file is a shim: each target forwards to `le` so that every existing
# caller keeps working -- CI, the hooks, and the several thousand `make ze-...`
# references across docs, rules and plan/.
#
# AGENTS AND HUMANS: use `le` directly. It is the source of truth.
#
#   ./le report-inventory                  every report in this area
#   ./le report-inventory --list           what each one is for
#   ./le report-inventory ze-inventory     one report
#   ./le report-inventory ze-inventory --json
#
# TWO TARGETS DID NOT MOVE, and their recipes are still here, at the bottom.
# ze-spec-status is a sequence of two programs and two headings where the
# second program's failure is deliberately swallowed, which a gate cannot
# express; ze-spec-status-json is its JSON twin, and splitting the pair would
# leave a lone `-json` gate whose other half is a Make recipe.
#
# ZE_CONTEXT_CAP and ZE_SESSION moved with ze-token-economy-report and are read
# from the environment there. A variable set on the make command line reaches
# the recipe's environment, so `make ze-token-economy-report
# ZE_CONTEXT_CAP=150000` still works and so does the same assignment in front
# of `./le`.
#
# Nothing else here carries logic. A change to what a report DOES belongs in
# the Python module; a change here can only break the forwarding.

.PHONY: ze-spec-status ze-spec-status-json ze-inventory ze-inventory-json ze-command-list ze-command-list-json ze-rules-gate-map-report ze-rules-payload-report ze-rules-router-report ze-rules-router-report-json ze-token-economy-report ze-journal-report

ze-inventory:
	@$(CURDIR)/le report-inventory ze-inventory

ze-inventory-json:
	@$(CURDIR)/le report-inventory ze-inventory --json

ze-command-list:
	@$(CURDIR)/le report-inventory ze-command-list

ze-command-list-json:
	@$(CURDIR)/le report-inventory ze-command-list --json

ze-rules-gate-map-report:
	@$(CURDIR)/le report-inventory ze-rules-gate-map-report

ze-rules-payload-report:
	@$(CURDIR)/le report-inventory ze-rules-payload-report

ze-rules-router-report:
	@$(CURDIR)/le report-inventory ze-rules-router-report

ze-rules-router-report-json:
	@$(CURDIR)/le report-inventory ze-rules-router-report --json

ze-token-economy-report:
	@$(CURDIR)/le report-inventory ze-token-economy-report

ze-journal-report:
	@$(CURDIR)/le report-inventory ze-journal-report

# --- The two that stayed ---------------------------------------------------

ze-spec-status:
	@go run scripts/status/spec_status.go
	@echo ""
	@echo "── Closure advisory (non-blocking) ──"
	@python3 scripts/dev/spec-closure-check.py --list || true

ze-spec-status-json:
	@go run scripts/status/spec_status.go --json
