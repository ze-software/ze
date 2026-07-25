# Chaos tests: fault-injection simulation via ze-chaos
#
# Quick reference:
#   make ze-chaos-test              All chaos tests (unit + functional + integration + web)
#   make ze-chaos-verify            Lint + all chaos tests
#   make ze-chaos-functional-test   In-process chaos simulation
#   make ze-chaos-integration-test  End-to-end: Ze + chaos peers (.ci tests)
#   make ze-chaos-web-test          Chaos web dashboard HTTP checks

.PHONY: ze-chaos-lint ze-chaos-unit-test ze-chaos-functional-test ze-chaos-integration-test ze-chaos-web-test ze-chaos-test ze-chaos-verify
.PHONY: _ze-chaos-verify-impl

CHAOS_PACKAGES = ./internal/chaos/...

# Chaos simulation parameters. Seed is random by default (printed for reproduction).
# Override: make ze-chaos-functional-test CHAOS_SEED=12345 CHAOS_DURATION=60s CHAOS_PEERS=8
CHAOS_SEED     ?= 0
CHAOS_DURATION ?= 30s
CHAOS_PEERS    ?= 4
CHAOS_ROUTES   ?= 10

ze-chaos-lint:
	@echo "Running chaos linter..."
	@golangci-lint run $(CHAOS_PACKAGES)

ze-chaos-unit-test:
	@echo "Running chaos unit tests..."
	$(GO_TEST_RACE) $(CHAOS_PACKAGES)

ze-chaos-functional-test: $(ZEBIN_CHAOS)
	@$(ZEBIN_CHAOS) --in-process --duration $(CHAOS_DURATION) \
		--peers $(CHAOS_PEERS) --routes $(CHAOS_ROUTES) \
		--seed $(CHAOS_SEED) --quiet

ze-chaos-integration-test: $(ZEBIN_TEST)
	@$(ZEBIN_TEST) bgp chaos --all -t 40s

ze-chaos-web-test: $(ZEBIN_TEST)
	@$(ZEBIN_TEST) bgp chaos-web --all

ze-chaos-test: ze-chaos-unit-test ze-chaos-functional-test ze-chaos-integration-test ze-chaos-web-test
	@echo "All chaos tests passed"

# Wrapped in the shared verify lock (see ze-verify) because chaos tests
# run $(ZEBIN_ZE) instances that would contend with a concurrent ze-verify.
ze-chaos-verify:
	@scripts/dev/verify-lock.sh ze-chaos-verify $(MAKE) --no-print-directory _ze-chaos-verify-impl

_ze-chaos-verify-impl: ze-chaos-lint ze-chaos-unit-test ze-chaos-functional-test ze-chaos-integration-test ze-chaos-web-test
	@echo "Chaos verification passed"
