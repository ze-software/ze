# Repository reports: inventory, spec status, token economy, and the rules readouts
#
# Quick reference:
#   make ze-inventory         Plugin/YANG/RPC/test inventory
#   make ze-spec-status       Spec progress overview (+ closure advisory)
#   make ze-token-economy-report  Per-session context cost, read from the transcripts
#

.PHONY: ze-spec-status ze-spec-status-json ze-inventory ze-inventory-json ze-command-list ze-command-list-json ze-rules-gate-map-report ze-rules-payload-report ze-rules-router-report ze-rules-router-report-json ze-token-economy-report ze-journal-report

ze-spec-status:
	@go run scripts/status/spec_status.go
	@echo ""
	@echo "── Closure advisory (non-blocking) ──"
	@python3 scripts/dev/spec-closure-check.py --list || true

ze-spec-status-json:
	@go run scripts/status/spec_status.go --json

ze-inventory:
	@$(GO_RUN) scripts/inventory/inventory.go

ze-inventory-json:
	@$(GO_RUN) scripts/inventory/inventory.go --json

ze-command-list:
	@$(GO_RUN) scripts/inventory/commands.go

ze-command-list-json:
	@$(GO_RUN) scripts/inventory/commands.go --json

# Rules-as-points gate map: which rule point each hook check enforces. The
# `# ze point: <rule>/<section>/<slug>` comments in the three PreToolUse dispatchers are
# joined against the point files, and three sets come out. Gated and ungated are
# MEASUREMENTS and exit 0: an ungated point is a rule no machine enforces yet,
# which is the number this target exists to publish. Dangling FAILS: a check
# naming a point that does not exist is what a reworded rule looks like, and
# before the id was a path nothing could see it.
#
# Three more sets fail here, and each one is a route by which an instruction or
# its gate leaves with every other target green: a check that named a point at
# HEAD and declares `none` now, a rule holding fewer points than HEAD with no
# row in ai/rules/points/RETIRED.md, and a `rationale`/`excepted-by` naming
# nothing. All three read git HEAD, so this target needs a repository.
ze-rules-gate-map-report:
	@python3 scripts/dev/rules_points.py coverage

# What a session actually loads: ai/INSTRUCTIONS.md + TRIGGERS.md + CORE.md,
# measured against the token budget and against the digest it replaces.
ze-rules-payload-report:
	@python3 scripts/dev/rules_condensed.py --payload

# Trigger-routing coverage: over every task description in plan/ (each open
# spec's Task section), which rules the trigger index would
# surface, and which BLOCKING rules no task surfaces at all. The second set is
# what the always-on core exists to protect, and the generator derives the core
# from it -- so a rule listed here has already been made eager.
ze-rules-router-report:
	@python3 scripts/dev/rules_router.py

ze-rules-router-report-json:
	@python3 scripts/dev/rules_router.py --json

ZE_CONTEXT_CAP ?= 200000
ZE_SESSION ?=

# Where this repository's agent sessions spend their tokens: API calls, the
# context carried at each one, the context-size histogram, and a capped-context
# counterfactual. Reads the machine-local Claude Code transcript store
# (~/.claude/projects/<slug>/), so a checkout with no transcripts reports that
# and exits 0. Token counts only, never money. Override the ceiling with
# `make ze-token-economy-report ZE_CONTEXT_CAP=150000`.
# Scope to one session with `make ze-token-economy-report ZE_SESSION=<id-prefix>`. The
# startup-context comparison between two agent types is only valid inside one
# session: the always-on preamble changes size between them, and it is the
# largest term in that number.
ze-token-economy-report:
	@python3 scripts/dev/token_economy.py --cap $(ZE_CONTEXT_CAP) $(if $(ZE_SESSION),--session $(ZE_SESSION))

# Problem journal detector: print every problem class with 2+ occurrences,
# its row count, and the span between first and last date.  When every class
# has 1 row it prints nothing and exits 0.
ze-journal-report:
	@python3 scripts/dev/journal.py
