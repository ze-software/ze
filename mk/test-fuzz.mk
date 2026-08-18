# Fuzz tests: Go fuzzing targets
#
# Quick reference:
#   make ze-fuzz-test                        All targets, 10s each
#   make ze-fuzz-test-one FUZZ=FuzzName PKG=path  Single target, 30s default

.PHONY: ze-fuzz-test ze-fuzz-test-one

# Run ze fuzz tests (all targets, 10s each).
#
# The per-target enumeration is NO LONGER hand-maintained here: it lives in the
# generated, committed fragment mk/test-fuzz-targets.mk (the `ze-fuzz-test`
# recipe), produced by `make generate` (scripts/dev/fuzz-targets.py). The
# generator walks internal/ for `func Fuzz`, resolves each to its exact package
# path, and emits one anchored `-fuzz=^<Name>$` invocation per target, so ISIS,
# OSPF, and every future fuzzer are included by registration, not by editing this
# file. A stale fragment is caught by `make ze-fuzz-targets-check`.
#
# Constraints the generator encodes (both are Go fuzz errors otherwise):
#   * exact single-package paths (never ./.../...) -- avoids "matches more than
#     one package" where a tree has sibling packages (e.g. isis/{packet,yang}).
#   * `-fuzz=^<Name>$` anchoring -- avoids "matches more than one fuzz target"
#     where one target name is a prefix of another (FuzzParseVPN[AddPath]).
include mk/test-fuzz-targets.mk

# Run a single fuzz target for longer (usage: make ze-fuzz-test-one FUZZ=FuzzParseNLRIs PKG=./internal/component/bgp/wireu/... TIME=30s)
FUZZ ?= FuzzParseNLRIs
PKG  ?= ./internal/component/bgp/wireu/...
TIME ?= 30s

ze-fuzz-test-one:
	@scripts/dev/ze-run.sh ze-fuzz-test-one $(MAKE) --no-print-directory _ze-fuzz-test-one-impl

_ze-fuzz-test-one-impl:
	$(GO_TEST) -fuzz=$(FUZZ) -fuzztime=$(TIME) $(PKG)

# The `_<target>-impl` half of every admitted pair defined in this file.
# The public half calls the admission wrapper and this half holds the work;
# see the job-admission block above ZE_RUN_SLOTS in the Makefile.
# _ze-fuzz-test-impl is DEFINED in the generated mk/test-fuzz-targets.mk, the
# same split ze-fuzz-test itself has (declared above, defined there). Declared
# here because the generator emits recipes, not .PHONY lines.
.PHONY: _ze-fuzz-test-one-impl _ze-fuzz-test-impl
