# CLI contract gates -- MOVED to `le`.
#
# The gates themselves now live in scripts/le/application/check_cli.py. This
# file is a shim: each target forwards to `le` so that every existing caller
# keeps working -- CI, ze-doc-verify, ze-precommit-verify, and the several
# thousand `make ze-...` references across docs, rules and plan/.
#
# AGENTS AND HUMANS: use `le` directly. It is the source of truth.
#
#   ./le check-cli                          every check in this area
#   ./le check-cli --list                   what each gate is for
#   ./le check-cli ze-cli-grammar-check     one gate
#   ./le check-cli ze-cli-grammar-check --json
#
# Nothing here carries logic. A change to what a gate DOES belongs in the
# Python module; a change here can only break the forwarding. When the last
# caller stops spelling these as Make targets, this file is deleted.

.PHONY: ze-docs-pipe-operators-update ze-command-contract-check ze-command-contract-check-json ze-command-ownership-check ze-command-ownership-check-json ze-cli-grammar-check ze-cli-grammar-check-json ze-config-claims-check ze-config-claims-check-json

ze-command-contract-check:
	@$(CURDIR)/le check-cli ze-command-contract-check

ze-command-contract-check-json:
	@$(CURDIR)/le check-cli ze-command-contract-check --json

ze-command-ownership-check:
	@$(CURDIR)/le check-cli ze-command-ownership-check

ze-command-ownership-check-json:
	@$(CURDIR)/le check-cli ze-command-ownership-check --json

ze-cli-grammar-check:
	@$(CURDIR)/le check-cli ze-cli-grammar-check

ze-cli-grammar-check-json:
	@$(CURDIR)/le check-cli ze-cli-grammar-check --json

ze-config-claims-check:
	@$(CURDIR)/le check-cli ze-config-claims-check

ze-config-claims-check-json:
	@$(CURDIR)/le check-cli ze-config-claims-check --json

ze-docs-pipe-operators-update: ## Regenerate the published pipe operator table from the operator catalog
	@$(CURDIR)/le check-cli ze-docs-pipe-operators-update
