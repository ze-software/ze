# Fuzz tests -- MOVED to `le`.
#
# The fuzzing itself now lives in scripts/le/application/fuzz.py. This file is
# a shim: each target forwards so every existing caller keeps working.
#
# AGENTS AND HUMANS: use `le` directly. It is the source of truth.
#
#   ./le fuzz                        every target, 10s each
#   ./le fuzz --list                 what would run, and where
#   ./le fuzz --name FuzzParseOpen   one target, 30s
#
# THE GENERATED FRAGMENT IS GONE, and that is the point of the move.
# mk/test-fuzz-targets.mk was a committed file holding one recipe line per
# `func Fuzz`, written by scripts/dev/fuzz-targets.py, kept honest by
# ze-fuzz-targets-check, and listed in the regen-check's git-diff guard. Four
# mechanisms served one fact: which packages declare a fuzz target. Make cannot
# read that at run time, so the fact had to be frozen into a file and then
# policed. `le fuzz` walks internal/ when it runs, so there is no artifact to
# commit, nothing to go stale, and no check needed to notice that it has. The
# generator, the check target, its .PHONY entry, its place in
# ze-generated-files-check, its regen-check path and its wiring-docs mapping
# were all removed with it.
#
# THE ADMISSION WRAPPER STAYS HERE, and it cannot move. scripts/dev/ze-run.sh
# re-enters make to take a job slot, so a fuzz run does not fight every other
# session on a shared box. That is a Make-level concern about Make's own
# concurrency, and `le` is not the right owner for it.

.PHONY: ze-fuzz-test ze-fuzz-test-one _ze-fuzz-test-impl _ze-fuzz-test-one-impl

ze-fuzz-test:
	@scripts/dev/ze-run.sh ze-fuzz-test $(MAKE) --no-print-directory _ze-fuzz-test-impl

_ze-fuzz-test-impl:
	@$(CURDIR)/le fuzz

# Run a single fuzz target for longer.
# Usage: make ze-fuzz-test-one FUZZ=FuzzParseNLRIs PKG=./internal/component/bgp/wireu TIME=30s
FUZZ ?= FuzzParseNLRIs
PKG  ?= ./internal/component/bgp/wireu/...
TIME ?= 30s

ze-fuzz-test-one:
	@scripts/dev/ze-run.sh ze-fuzz-test-one $(MAKE) --no-print-directory _ze-fuzz-test-one-impl

_ze-fuzz-test-one-impl:
	@$(CURDIR)/le fuzz --name $(FUZZ) --time $(TIME) $(if $(PKG),--package $(PKG))
