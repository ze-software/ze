# Inventory, spec status, doc validation, and consistency tools
#
# Quick reference:
#   make ze-verify-wiring-docs  Changed-file-aware wiring/doc/inventory gate
#   make ze-doc-test             All doc checks (drift + anchors + YANG/handler)
#   make ze-ste-review           ASD-STE100 findings, with file:line and the fix
#   make ze-ste-check            ASD-STE100 gate: no habit grew vs HEAD
#   make ze-inventory            Plugin/YANG/RPC/test inventory
#   make ze-spec-status          Spec progress overview (+ closure advisory)
#   make ze-spec-citation-check  Spec citation freshness (dangling plan/spec refs)
#   make ze-validate-commands    YANG command vs handler cross-check
#
.PHONY: ze-spec-status ze-spec-status-json ze-spec-citation-check
.PHONY: ze-inventory ze-inventory-json ze-command-list ze-command-list-json
.PHONY: ze-validate-commands ze-validate-commands-json ze-command-ownership-check ze-command-ownership-check-json ze-cli-grammar-check ze-cli-grammar-check-json ze-config-claims-check ze-config-claims-check-json ze-doc-drift ze-doc-test ze-doc-index ze-doc-check-stale ze-rules-index ze-rules-index-check ze-rules-condensed ze-rules-condensed-check ze-rules-points-roundtrip ze-rules-render ze-rules-render-check ze-rules-gate-map ze-rules-payload ze-rules-router-report ze-rules-router-report-json ze-rules-lint ze-token-economy ze-discovery-index ze-discovery-index-check ze-learned-numbers-check ze-learned-numbers-fix ze-learned-normalise-check ze-learned-normalise-fix ze-learned-repath-check ze-learned-repath-apply ze-digest-check ze-learned-staleness ze-consistency
.PHONY: ze-verify-wiring-docs ze-wiki-update ze-wiki-commands
.PHONY: ze-ste-check ze-ste-review ze-ste-review-changed ze-ste-review-json

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
	echo "  -> Learned staleness (plan/learned/*.md cited paths and NNN citations resolve)..."; \
	python3 scripts/dev/learned_staleness.py --check || FAIL=1; \
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
# It is NOT wired into ze-doc-test. Several sessions share this checkout, so a
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

ze-doc-index:
	@python3 scripts/dev/code_to_docs.py

ze-doc-check-stale:
	@python3 scripts/dev/code_to_docs.py --check

# Depends on ze-rules-render for the reason ze-rules-condensed does, below:
# rules_index.py parses the RENDERED rules, so under `make -j` it must not race
# the generator that writes them.
ze-rules-index: ze-rules-render
	@python3 scripts/dev/rules_index.py

ze-rules-index-check:
	@python3 scripts/dev/rules_index.py --check

# Rule digest artifacts, both from one parse of ai/rules/*.md:
#   TRIGGERS.md   one routing line per rule (path, severity, **When:**)
#   CORE.md       the directives of the always-on rules, derived from the
#                 rung 1/2 ladder in ai/rules/rule-precedence.md
# Generated from the canonical rule format; a stale artifact fails ze-doc-test.
#
# The ze-rules-render prerequisite is what ORDERS the two. Both digests parse
# the RENDERED rules, which ze-rules-render writes with a plain write_text, and
# GNU make honours prerequisite ORDER only when it runs serially: under
# `make -j ze-regen` the digest could be built from pre-render text or from a
# torn read. A dependency edge is the ordering make actually enforces, an
# order-only prerequisite (`|`) would not have (it orders a prerequisite against
# its own target, never two siblings of one target), and .NOTPARALLEL would pay
# for it by serialising every unrelated target in the file.
ze-rules-condensed: ze-rules-render
	@python3 scripts/dev/rules_condensed.py

ze-rules-condensed-check:
	@python3 scripts/dev/rules_condensed.py --check

# Rules-as-points round trip: split every ai/rules/*.md into per-point files in
# a scratch directory, render those files back, and compare bytes. A rendered
# rule is what every agent Reads, so a lossy split is silent instruction loss.
# This target is the gate on that: it exits non-zero naming any rule whose round
# trip is not byte-identical. Scratch only, it never writes ai/rules/points/.
#
# It runs inside ze-doc-test, and the render check does NOT subsume it. They
# read the two directions of the same identity. `render --check` asks whether
# the rendered rule matches the points; the round trip asks whether the rendered
# rule can be split back into points at all. One blank line at the top of a
# point body satisfies the first and breaks the second, and the corpus is then
# permanently un-splittable with every other gate green.
ze-rules-points-roundtrip:
	@python3 scripts/dev/rules_points.py roundtrip

# Rules-as-points render: ai/rules/points/<rule>/<section>/ -> ai/rules/<rule>.md. The
# rendered rule is what every agent Reads, so this generator owns those files
# and an edit to one is refused by .claude/hooks/pretool-writeedit.py. Order
# matters inside ze-regen: ze-rules-condensed and ze-rules-index both parse the
# RENDERED rules, and each one declares this target as a PREREQUISITE so make
# enforces that under `-j` rather than the recipe order asserting it.
ze-rules-render:
	@python3 scripts/dev/rules_points.py render

ze-rules-render-check:
	@python3 scripts/dev/rules_points.py render --check

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
ze-rules-gate-map:
	@python3 scripts/dev/rules_points.py coverage

# What a session actually loads: ai/INSTRUCTIONS.md + TRIGGERS.md + CORE.md,
# measured against the token budget and against the digest it replaces.
ze-rules-payload:
	@python3 scripts/dev/rules_condensed.py --payload

# Trigger-routing coverage: over every past task description in plan/ (learned
# summaries' Context, open specs' Task), which rules the trigger index would
# surface, and which BLOCKING rules no task surfaces at all. The second set is
# what the always-on core exists to protect, and the generator derives the core
# from it -- so a rule listed here has already been made eager.
ze-rules-router-report:
	@python3 scripts/dev/rules_router.py

ze-rules-router-report-json:
	@python3 scripts/dev/rules_router.py --json

# Where this repository's agent sessions spend their tokens: API calls, the
# context carried at each one, the context-size histogram, and a capped-context
# counterfactual. Reads the machine-local Claude Code transcript store
# (~/.claude/projects/<slug>/), so a checkout with no transcripts reports that
# and exits 0. Token counts only, never money. Override the ceiling with
# `make ze-token-economy ZE_CONTEXT_CAP=150000`.
# Scope to one session with `make ze-token-economy ZE_SESSION=<id-prefix>`. The
# startup-context comparison between two agent types is only valid inside one
# session: the always-on preamble changes size between them, and it is the
# largest term in that number.
ZE_CONTEXT_CAP ?= 200000
ZE_SESSION ?=
ze-token-economy:
	@python3 scripts/dev/token_economy.py --cap $(ZE_CONTEXT_CAP) $(if $(ZE_SESSION),--session $(ZE_SESSION))

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

# Learned-summary section headings: every plan/learned/NNN-*.md spells its
# sections the way `ai/rules/planning.md` names them, so a reader and a tool
# find `## Files` under one name. `--check` reports, `--fix` rewrites in place.
# Not folded into ze-doc-test: heading drift is not a correctness defect, and a
# heading gate that reddens a colleague's in-flight summary gets switched off.
ze-learned-normalise-check:
	@python3 scripts/dev/learned_normalise.py --check

ze-learned-normalise-fix:
	@python3 scripts/dev/learned_normalise.py --fix

# Dead `## Files` paths that name code which MOVED rather than code which went
# away. It repoints such a citation at the successor path, and leaves alone any
# path with several plausible successors or none: a citation rewritten to the
# wrong file reads as true, while a dead one stays visible to
# ze-learned-staleness. This is the repair tool for that gate's findings.
ze-learned-repath-check:
	@python3 scripts/dev/learned_repath.py --check

ze-learned-repath-apply:
	@python3 scripts/dev/learned_repath.py --apply

# Digest anchor validity: every `file:line` reference in ai/digests/*.md resolves
# to a real file and an in-range line. The digests are hand-maintained, so this
# catches the anchors rotting when code moves. Runs inside ze-doc-test.
ze-digest-check:
	@python3 scripts/dev/digest_check.py

# Learned-summary decay: every path a plan/learned/NNN-*.md cites in its
# `## Files` section still exists, and every `plan/learned/NNN` citation names a
# summary that was not retired. The summaries are hand-written and never
# regenerated, so their references rot silently as code moves and old summaries
# are deleted. Dead references are compared against the shrink-only baseline
# plan/.learned-staleness-baseline (the plan/.citation-baseline idiom): more
# than the baseline fails, fewer rewrites it. Runs inside ze-doc-test.
ze-learned-staleness:
	@python3 scripts/dev/learned_staleness.py

ze-consistency:
	@echo "Running consistency checks..."
	@go run scripts/lint/consistency.go .
