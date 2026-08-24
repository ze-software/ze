# Documentation gates: drift, wiring, anchors, indexes, and Simplified Technical English
#
# Quick reference:
#   make ze-doc-verify        All doc checks (drift + anchors + YANG/handler)
#   make ze-doc-wiring-check  Changed-file-aware wiring/doc/inventory gate
#   make ze-ste-review        ASD-STE100 findings, with file:line and the fix
#   make ze-ste-check         ASD-STE100 gate: no habit grew vs HEAD
#

.PHONY: ze-spec-citation-check ze-wiki-update ze-wiki-commands-update ze-doc-drift-check ze-doc-wiring-check ze-doc-verify ze-ste-check ze-ste-review ze-ste-review-changed ze-ste-review-json ze-doc-index-update ze-doc-index-check ze-digest-check ze-consistency-check

# Spec citation freshness gate: a plan/spec-*.md that cites a sibling
# plan/spec-*.md absent on disk fails (unless the target is grandfathered in
# plan/.citation-baseline); a path:line citation whose backtick-quoted token
# drifted off that line WARNs (non-fatal). Runs on the verify path when a plan/
# file changes (scripts/dev/verify_wiring_docs.py routes it). Regenerate the
# baseline with `scripts/dev/spec-citation-check.py --write-baseline`.
ze-spec-citation-check:
	@python3 scripts/dev/spec-citation-check.py

ze-wiki-update: ze-wiki-commands-update
	@echo "Wiki updated"

ze-wiki-commands-update:
	@$(ZEBIN_ZE) help command --json | python3 scripts/dev/gen_wiki_commands.py > ../wiki/command-catalog.md
	@echo "  -> ../wiki/command-catalog.md"

ze-doc-drift-check:
	@$(GO_RUN) scripts/docvalid/doc_drift.go

ze-doc-wiring-check:
	@python3 scripts/dev/verify_wiring_docs.py --make "$(MAKE)"

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

# Simplified Technical English (ASD-STE100 Issue 9) -- rule one of the repository
# (ai/rules/writing.md). ze-ste-check counts the six banned
# habits in each file the working tree changed, against that file's own HEAD
# version, and fails when a habit grew. HEAD is the baseline, so legacy prose
# stays until someone rewrites it, no baseline file exists to re-bless, and the
# one way to green is to fix the prose. About 2 seconds.
#
# It is NOT wired into ze-doc-verify. Several sessions share this checkout, so a
# tree-wide run reports a sibling session's in-flight sentences and nobody can
# tell whose they are. The BLOCKING gate lives in commit_helper.py
# (ste_problems), scoped to the files of ONE commit, which is the only place
# where prose has a single author.
ze-ste-check:
	@python3 scripts/dev/ste_check.py --check

ze-ste-review:
	@python3 scripts/dev/ste_check.py

ze-ste-review-changed:
	@python3 scripts/dev/ste_check.py --changed

ze-ste-review-json:
	@python3 scripts/dev/ste_check.py --json

ze-doc-index-update:
	@python3 scripts/dev/code_to_docs.py

ze-doc-index-check:
	@python3 scripts/dev/code_to_docs.py --check

# Digest anchor validity: every `file:line` reference in ai/digests/*.md resolves
# to a real file and an in-range line. The digests are hand-maintained, so this
# catches the anchors rotting when code moves. Runs inside ze-doc-verify.
ze-digest-check:
	@python3 scripts/dev/digest_check.py

ze-consistency-check:
	@echo "Running consistency checks..."
	@go run scripts/lint/consistency.go .
