# Documentation gates -- MOVED to `le`.
#
# The gates themselves now live in scripts/le/application/check_docs.py. This
# file is a shim: each target forwards to `le` so that every existing caller
# keeps working -- CI, ze-doc-verify, ze-precommit-verify, and the several
# thousand `make ze-...` references across docs, rules and plan/.
#
# AGENTS AND HUMANS: use `le` directly. It is the source of truth.
#
#   ./le check-docs                       every check in this area
#   ./le check-docs --list                what each gate is for
#   ./le check-docs ze-digest-check       one gate
#   ./le check-docs ze-ste-review --json
#
# THREE TARGETS DID NOT MOVE, and their recipes are still here, at the bottom:
# ze-doc-verify is a twelve-step sequence over one FAIL flag ending in a
# conditional exit, ze-wiki-commands-update is a pipeline into a redirect
# outside the checkout, and ze-wiki-update is an alias whose only content is a
# prerequisite edge. A gate is one command; those three are shell programs.
#
# Nothing else here carries logic. A change to what a gate DOES belongs in the
# Python module; a change here can only break the forwarding. When the last
# caller stops spelling these as Make targets, the forwarding half is deleted.

.PHONY: ze-spec-citation-check ze-wiki-update ze-wiki-commands-update ze-doc-drift-check ze-doc-wiring-check ze-doc-verify ze-ste-check ze-ste-review ze-ste-review-changed ze-ste-review-json ze-doc-index-update ze-doc-index-check ze-digest-check ze-consistency-check

ze-spec-citation-check:
	@$(CURDIR)/le check-docs ze-spec-citation-check

ze-doc-drift-check:
	@$(CURDIR)/le check-docs ze-doc-drift-check

ze-doc-wiring-check:
	@$(CURDIR)/le check-docs ze-doc-wiring-check

ze-doc-index-check:
	@$(CURDIR)/le check-docs ze-doc-index-check

ze-doc-index-update:
	@$(CURDIR)/le check-docs ze-doc-index-update

ze-digest-check:
	@$(CURDIR)/le check-docs ze-digest-check

ze-consistency-check:
	@$(CURDIR)/le check-docs ze-consistency-check

ze-ste-check:
	@$(CURDIR)/le check-docs ze-ste-check

ze-ste-review:
	@$(CURDIR)/le check-docs ze-ste-review

ze-ste-review-changed:
	@$(CURDIR)/le check-docs ze-ste-review-changed

ze-ste-review-json:
	@$(CURDIR)/le check-docs ze-ste-review --json

# --- The three that stayed -------------------------------------------------

# The prerequisite edge is the ordering `make -j` actually enforces. Keep it.
ze-wiki-update: ze-wiki-commands-update
	@echo "Wiki updated"

ze-wiki-commands-update:
	@$(ZEBIN_ZE) help command --json | python3 scripts/dev/gen_wiki_commands.py > ../wiki/command-catalog.md
	@echo "  -> ../wiki/command-catalog.md"

ze-doc-verify:
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
	echo "  -> Rules render (ai/rules/<rule>.md matches ai/rules/points/)..."; \
	python3 scripts/dev/rules_points.py render --check || FAIL=1; \
	echo ""; \
	echo "  -> Rules round trip (split every rendered rule, render it back, compare bytes)..."; \
	python3 scripts/dev/rules_points.py roundtrip || FAIL=1; \
	echo ""; \
	echo "  -> Rules gate map (no hook check names a point that does not exist)..."; \
	python3 scripts/dev/rules_points.py coverage --quiet || FAIL=1; \
	echo ""; \
	echo "  -> Rules index (ai/rules/INDEX.md fresh, every rule has a summary)..."; \
	python3 scripts/dev/rules_index.py --check || FAIL=1; \
	echo ""; \
	echo "  -> Rule format (every ai/rules/*.md has the When/Severity block)..."; \
	python3 scripts/dev/rules_lint.py --quiet || FAIL=1; \
	echo ""; \
	echo "  -> Rules digest (ai/rules/TRIGGERS.md + CORE.md fresh)..."; \
	python3 scripts/dev/rules_condensed.py --check || FAIL=1; \
	echo ""; \
	echo "  -> Discovery indexes (package map, docs-to-code fresh)..."; \
	python3 scripts/dev/package_map.py --check || FAIL=1; \
	python3 scripts/dev/docs_to_code.py --check || FAIL=1; \
	python3 scripts/dev/rfc_requirements.py --check-fresh || FAIL=1; \
	echo ""; \
	echo "  -> Problem journal (classes with 2+ rows)..."; \
	python3 scripts/dev/journal.py || FAIL=1; \
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
