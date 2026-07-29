# Inventory, spec status, doc validation, and consistency tools
#
# Quick reference:
#   make ze-verify-wiring-docs  Changed-file-aware wiring/doc/inventory gate
#   make ze-doc-test             All doc checks (drift + anchors + YANG/handler)
#   make ze-inventory            Plugin/YANG/RPC/test inventory
#   make ze-spec-status          Spec progress overview (+ closure advisory)
#   make ze-spec-citation-check  Spec citation freshness (dangling plan/spec refs)
#   make ze-validate-commands    YANG command vs handler cross-check
#
.PHONY: ze-spec-status ze-spec-status-json ze-spec-citation-check
.PHONY: ze-inventory ze-inventory-json ze-command-list ze-command-list-json
.PHONY: ze-validate-commands ze-validate-commands-json ze-command-ownership-check ze-command-ownership-check-json ze-cli-grammar-check ze-cli-grammar-check-json ze-doc-drift ze-doc-test ze-doc-index ze-doc-check-stale ze-rules-index ze-rules-index-check ze-rules-condensed ze-rules-condensed-check ze-rules-lint ze-discovery-index ze-discovery-index-check ze-learned-numbers-check ze-learned-numbers-fix ze-digest-check ze-consistency
.PHONY: ze-verify-wiring-docs ze-wiki-update ze-wiki-commands

ze-spec-status:
	@go run scripts/status/spec_status.go
	@echo ""
	@echo "── Closure advisory (non-blocking) ──"
	@python3 scripts/dev/spec-closure-check.py --list || true

ze-spec-status-json:
	@go run scripts/status/spec_status.go --json

# Spec citation freshness gate: a plan/spec-*.md that cites a sibling
# plan/spec-*.md absent on disk fails (unless the target is grandfathered in
# plan/.citation-baseline); a path:line citation whose backtick-quoted token
# drifted off that line WARNs (non-fatal). Runs on the verify path when a plan/
# file changes (scripts/dev/verify_wiring_docs.py routes it). Regenerate the
# baseline with `scripts/dev/spec-citation-check.py --write-baseline`.
ze-spec-citation-check:
	@python3 scripts/dev/spec-citation-check.py

ze-inventory:
	@$(GO_RUN) scripts/inventory/inventory.go

ze-inventory-json:
	@$(GO_RUN) scripts/inventory/inventory.go --json

ze-command-list:
	@$(GO_RUN) scripts/inventory/commands.go

ze-command-list-json:
	@$(GO_RUN) scripts/inventory/commands.go --json

ze-wiki-update: ze-wiki-commands
	@echo "Wiki updated"

ze-wiki-commands:
	@$(ZEBIN_ZE) help command --json | python3 scripts/dev/gen_wiki_commands.py > ../wiki/command-catalog.md
	@echo "  -> ../wiki/command-catalog.md"

ze-doc-drift:
	@$(GO_RUN) scripts/docvalid/doc_drift.go

ze-validate-commands:
	@$(GO_RUN) scripts/docvalid/commands.go

ze-validate-commands-json:
	@$(GO_RUN) scripts/docvalid/commands.go --json

ze-command-ownership-check:
	@$(GO_RUN) scripts/checks/command_ownership.go

ze-command-ownership-check-json:
	@$(GO_RUN) scripts/checks/command_ownership.go --json

# CLI grammar gate: every built-in command obeys the verb-first grammar rules
# (ai/rules/cli-grammar.md, R1-R8) and no .yang carries a --flag.
ze-cli-grammar-check:
	@$(GO_RUN) scripts/checks/cli_grammar.go

ze-cli-grammar-check-json:
	@$(GO_RUN) scripts/checks/cli_grammar.go --json


ze-verify-wiring-docs:
	@python3 scripts/dev/verify_wiring_docs.py --make "$(MAKE)"

ze-doc-test:
	@echo "Running documentation tests..."
	@FAIL=0; \
	echo ""; \
	echo "  -> Documentation drift (docs claims vs registry, Makefile, filesystem)..."; \
	$(GO_RUN) scripts/docvalid/doc_drift.go || FAIL=1; \
	echo ""; \
	echo "  -> YANG/handler contract (validate-commands)..."; \
	$(GO_RUN) scripts/docvalid/commands.go || FAIL=1; \
	echo ""; \
	echo "  -> Source anchors (docs source references exist)..."; \
	python3 scripts/dev/code_to_docs.py --check || FAIL=1; \
	echo ""; \
	echo "  -> Rules index (ai/rules/INDEX.md fresh, every rule has a summary)..."; \
	python3 scripts/dev/rules_index.py --check || FAIL=1; \
	echo ""; \
	echo "  -> Rule format (every ai/rules/*.md has the When/Severity block)..."; \
	python3 scripts/dev/rules_lint.py --quiet || FAIL=1; \
	echo ""; \
	echo "  -> Rules digest (ai/rules/CONDENSED.md fresh)..."; \
	python3 scripts/dev/rules_condensed.py --check || FAIL=1; \
	echo ""; \
	echo "  -> Discovery indexes (package map, docs-to-code, learned index fresh)..."; \
	python3 scripts/dev/package_map.py --check || FAIL=1; \
	python3 scripts/dev/docs_to_code.py --check || FAIL=1; \
	python3 scripts/dev/learned_index.py --check || FAIL=1; \
	python3 scripts/dev/rfc_requirements.py --check-fresh || FAIL=1; \
	echo ""; \
	echo "  -> Learned numbering (no duplicate NNN, H1 matches filename)..."; \
	python3 scripts/dev/learned_numbers.py --check || FAIL=1; \
	echo ""; \
	echo "  -> Digest anchors (ai/digests/*.md file:line references resolve)..."; \
	python3 scripts/dev/digest_check.py --check || FAIL=1; \
	echo ""; \
	if [ $$FAIL -ne 0 ]; then \
		echo "Documentation tests FAILED -- see output above."; \
		echo "See docs/contributing/documentation-testing.md for how to fix."; \
		exit 1; \
	fi; \
	echo "Documentation tests PASSED"

ze-doc-index:
	@python3 scripts/dev/code_to_docs.py

ze-doc-check-stale:
	@python3 scripts/dev/code_to_docs.py --check

ze-rules-index:
	@python3 scripts/dev/rules_index.py

ze-rules-index-check:
	@python3 scripts/dev/rules_index.py --check

# Condensed rule digest (ai/rules/CONDENSED.md): the actionable core of every
# rule, eager-loaded into every session via `@ai/rules/CONDENSED.md` in
# ai/INSTRUCTIONS.md. Generated from the canonical rule format; a stale digest
# fails ze-doc-test.
ze-rules-condensed:
	@python3 scripts/dev/rules_condensed.py

ze-rules-condensed-check:
	@python3 scripts/dev/rules_condensed.py --check

# Rule format lint: every ai/rules/*.md carries the required **When:** /
# **Severity:** metadata block (see ai/rules/rule-format.md), so tooling can
# parse triggers and severity instead of guessing. Runs inside ze-doc-test.
ze-rules-lint:
	@python3 scripts/dev/rules_lint.py

# Generated discovery indexes: what each package does (PACKAGE-MAP), which files
# implement a design doc (DOCS-TO-CODE), and every learned summary by number
# (LEARNED-FULL-INDEX). Sourced from the tree; a stale index fails ze-doc-test.
#
# `--check` exit codes: 0 = fresh, 3 = STALE (discovery_sources.STALE_EXIT),
# 1 = the generator itself failed. Every caller here only needs pass/fail, so
# plain `make` semantics and the `|| FAIL=1` form above are both correct. The
# distinction exists for commit_helper.py, which BLOCKS on 3 and must stay
# warn-only on 1 -- do not "simplify" a caller into one that cannot tell them
# apart, and do not reintroduce matching on the warning TEXT.
ze-discovery-index:
	@python3 scripts/dev/package_map.py
	@python3 scripts/dev/docs_to_code.py
	@python3 scripts/dev/learned_index.py

ze-discovery-index-check:
	@python3 scripts/dev/package_map.py --check
	@python3 scripts/dev/docs_to_code.py --check
	@python3 scripts/dev/learned_index.py --check
	@python3 scripts/dev/learned_numbers.py --check

# Learned-summary numbering: no two plan/learned/NNN-*.md share a number and
# each H1 number matches its filename. Duplicates are not caught by learned-next
# (it allocates max(existing prefixes)+1 against the local tree only), so two
# branches collide and only merging reveals it. `--fix` renumbers the colliding
# summaries and rewrites their references.
ze-learned-numbers-check:
	@python3 scripts/dev/learned_numbers.py --check

ze-learned-numbers-fix:
	@python3 scripts/dev/learned_numbers.py --fix

# Digest anchor validity: every `file:line` reference in ai/digests/*.md resolves
# to a real file and an in-range line. The digests are hand-maintained, so this
# catches the anchors rotting when code moves. Runs inside ze-doc-test.
ze-digest-check:
	@python3 scripts/dev/digest_check.py

ze-consistency:
	@echo "Running consistency checks..."
	@go run scripts/lint/consistency.go .
