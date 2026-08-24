# CLI contract gates: YANG command vs handler, ownership, grammar, and config claims
#
# Quick reference:
#   make ze-command-contract-check  YANG command vs handler cross-check
#   make ze-cli-grammar-check       Keyword-before-value grammar over every command
#

.PHONY: ze-docs-pipe-operators-update ze-command-contract-check ze-command-contract-check-json ze-command-ownership-check ze-command-ownership-check-json ze-cli-grammar-check ze-cli-grammar-check-json ze-config-claims-check ze-config-claims-check-json

ze-docs-pipe-operators-update: ## Regenerate the published pipe operator table from the operator catalog
	@$(GO_RUN) scripts/docvalid/doc_drift.go --write-generated

ze-command-contract-check:
	@$(GO_RUN) scripts/docvalid/commands.go

ze-command-contract-check-json:
	@$(GO_RUN) scripts/docvalid/commands.go --json

ze-command-ownership-check:
	@$(GO_RUN) scripts/checks/command_ownership.go

ze-command-ownership-check-json:
	@$(GO_RUN) scripts/checks/command_ownership.go --json

# CLI grammar gate: every built-in command obeys the verb-first grammar rules
# (ai/rules/cli.md, R1-R8) and no .yang carries a --flag.
ze-cli-grammar-check:
	@$(GO_RUN) scripts/checks/cli_grammar.go

ze-cli-grammar-check-json:
	@$(GO_RUN) scripts/checks/cli_grammar.go --json

# Config claim completeness gate (spec-improve-7): every config subtree an
# operator can write is delivered to a plugin config root, a hub handler path,
# or a recorded exception; and every declared config root names a real schema
# node. GO_RUN carries the full feature tag set, which this needs: a reduced set
# compiles modules out and shrinks the surface the gate can see.
ze-config-claims-check:
	@$(GO_RUN) scripts/checks/config_claims.go

ze-config-claims-check-json:
	@$(GO_RUN) scripts/checks/config_claims.go --json
