# Fuzz tests: Go fuzzing targets
#
# Quick reference:
#   make ze-fuzz-test                        All targets, 10s each
#   make ze-fuzz-one FUZZ=FuzzName PKG=path  Single target, 30s default

.PHONY: ze-fuzz-test ze-fuzz-one

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

# Run a single fuzz target for longer (usage: make ze-fuzz-one FUZZ=FuzzParseNLRIs PKG=./internal/component/bgp/wireu/... TIME=30s)
FUZZ ?= FuzzParseNLRIs
PKG  ?= ./internal/component/bgp/wireu/...
TIME ?= 30s

ze-fuzz-one:
	$(GO_TEST) -fuzz=$(FUZZ) -fuzztime=$(TIME) $(PKG)
