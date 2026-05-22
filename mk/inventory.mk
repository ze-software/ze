# Inventory, spec status, doc validation, and consistency tools
#
# Quick reference:
#   make ze-doc-test             All doc checks (drift + YANG/handler)
#   make ze-inventory            Plugin/YANG/RPC/test inventory
#   make ze-spec-status          Spec progress overview
#   make ze-validate-commands    YANG command vs handler cross-check

.PHONY: ze-spec-status ze-spec-status-json ze-learned-counter
.PHONY: ze-inventory ze-inventory-json ze-command-list ze-command-list-json
.PHONY: ze-validate-commands ze-validate-commands-json ze-doc-drift ze-doc-test ze-consistency

ze-spec-status:
	@go run scripts/status/spec_status.go

ze-spec-status-json:
	@go run scripts/status/spec_status.go --json

ze-learned-counter:
	@n=$$(ls plan/learned/[0-9]*.md 2>/dev/null | sed 's/.*\///' | grep -oE '^[0-9]+' | sort -rn | head -1); \
	echo $$(( $${n:-0} + 1 )) > plan/learned/.counter; \
	echo "plan/learned/.counter set to $$(cat plan/learned/.counter)"

ze-inventory:
	@go run scripts/inventory/inventory.go

ze-inventory-json:
	@go run scripts/inventory/inventory.go --json

ze-command-list:
	@go run scripts/inventory/commands.go

ze-command-list-json:
	@go run scripts/inventory/commands.go --json

ze-doc-drift:
	@go run scripts/docvalid/doc_drift.go

ze-validate-commands:
	@go run scripts/docvalid/commands.go

ze-validate-commands-json:
	@go run scripts/docvalid/commands.go --json

ze-doc-test:
	@echo "Running documentation tests..."
	@FAIL=0; \
	echo ""; \
	echo "  -> Documentation drift (docs claims vs registry, Makefile, filesystem)..."; \
	go run scripts/docvalid/doc_drift.go || FAIL=1; \
	echo ""; \
	echo "  -> YANG/handler contract (validate-commands)..."; \
	go run scripts/docvalid/commands.go || FAIL=1; \
	echo ""; \
	if [ $$FAIL -ne 0 ]; then \
		echo "Documentation tests FAILED -- see output above."; \
		echo "See docs/contributing/documentation-testing.md for how to fix."; \
		exit 1; \
	fi; \
	echo "Documentation tests PASSED"

ze-consistency:
	@echo "Running consistency checks..."
	@go run scripts/lint/consistency.go .
