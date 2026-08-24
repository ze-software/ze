# Rules-system gates: the point corpus, its renders, and the discovery index
#
# Quick reference:
#   make ze-rules-condensed-update  Regenerate TRIGGERS.md and CORE.md from the points
#   make ze-rules-render-check      The rendered rules agree with their point files
#

.PHONY: ze-rules-index-update ze-rules-index-check ze-rules-condensed-update ze-rules-condensed-check ze-rules-points-roundtrip-check ze-rules-render-update ze-rules-render-check ze-rules-lint ze-discovery-index-update ze-discovery-index-check

# Depends on ze-rules-render-update for the reason ze-rules-condensed-update does, below:
# rules_index.py parses the RENDERED rules, so under `make -j` it must not race
# the generator that writes them.
ze-rules-index-update: ze-rules-render-update
	@python3 scripts/dev/rules_index.py

ze-rules-index-check:
	@python3 scripts/dev/rules_index.py --check

# Rule digest artifacts, both from one parse of ai/rules/*.md:
#   TRIGGERS.md   one routing line per rule (path, severity, **When:**)
#   CORE.md       the directives of the always-on rules, derived from the
#                 rung 1/2 ladder in ai/rules/rule-precedence.md
# Generated from the canonical rule format; a stale artifact fails ze-doc-verify.
#
# The ze-rules-render-update prerequisite is what ORDERS the two. Both digests parse
# the RENDERED rules, which ze-rules-render-update writes with a plain write_text, and
# GNU make honours prerequisite ORDER only when it runs serially: under
# `make -j ze-generated-files-update` the digest could be built from pre-render text or from a
# torn read. A dependency edge is the ordering make actually enforces, an
# order-only prerequisite (`|`) would not have (it orders a prerequisite against
# its own target, never two siblings of one target), and .NOTPARALLEL would pay
# for it by serialising every unrelated target in the file.
ze-rules-condensed-update: ze-rules-render-update
	@python3 scripts/dev/rules_condensed.py

ze-rules-condensed-check:
	@python3 scripts/dev/rules_condensed.py --check

# Rules-as-points round trip: split every ai/rules/*.md into per-point files in
# a scratch directory, render those files back, and compare bytes. A rendered
# rule is what every agent Reads, so a lossy split is silent instruction loss.
# This target is the gate on that: it exits non-zero naming any rule whose round
# trip is not byte-identical. Scratch only, it never writes ai/rules/points/.
#
# It runs inside ze-doc-verify, and the render check does NOT subsume it. They
# read the two directions of the same identity. `render --check` asks whether
# the rendered rule matches the points; the round trip asks whether the rendered
# rule can be split back into points at all. One blank line at the top of a
# point body satisfies the first and breaks the second, and the corpus is then
# permanently un-splittable with every other gate green.
ze-rules-points-roundtrip-check:
	@python3 scripts/dev/rules_points.py roundtrip

# Rules-as-points render: ai/rules/points/<rule>/<section>/ -> ai/rules/<rule>.md. The
# rendered rule is what every agent Reads, so this generator owns those files
# and an edit to one is refused by .claude/hooks/pretool-writeedit.py. Order
# matters inside ze-generated-files-update: ze-rules-condensed-update and ze-rules-index-update both parse the
# RENDERED rules, and each one declares this target as a PREREQUISITE so make
# enforces that under `-j` rather than the recipe order asserting it.
ze-rules-render-update:
	@python3 scripts/dev/rules_points.py render

ze-rules-render-check:
	@python3 scripts/dev/rules_points.py render --check

# Rule format lint: every ai/rules/*.md carries the required **When:** /
# **Severity:** metadata block (see ai/rules/rule-format.md), so tooling can
# parse triggers and severity instead of guessing. Runs inside ze-doc-verify.
ze-rules-lint:
	@python3 scripts/dev/rules_lint.py

# Generated discovery indexes: what each package does (PACKAGE-MAP), which files
# implement a design doc (DOCS-TO-CODE). Sourced from the tree; a stale
# index fails ze-doc-verify.
#
# `--check` exit codes: 0 = fresh, 3 = STALE (discovery_sources.STALE_EXIT),
# 1 = the generator itself failed. Every caller here only needs pass/fail, so
# plain `make` semantics and the `|| FAIL=1` form above are both correct. The
# distinction exists for commit_helper.py, which BLOCKS on 3 and must stay
# warn-only on 1 -- do not "simplify" a caller into one that cannot tell them
# apart, and do not reintroduce matching on the warning TEXT.
ze-discovery-index-update:
	@python3 scripts/dev/package_map.py
	@python3 scripts/dev/docs_to_code.py

ze-discovery-index-check:
	@python3 scripts/dev/package_map.py --check
