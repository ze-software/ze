# Release evidence: full test matrix for release-readiness proof
#
# These targets compose every existing test suite into a single gate.
# ze-precommit-verify stays the fast pre-commit gate (4-10 min). ze-release-evidence-verify runs
# everything: interop, chaos, fuzz, perf regression, QEMU, deployment.
#
# Quick reference:
#   make ze-release-evidence-preflight  Check required tooling
#   make ze-release-evidence-verify            Full release evidence matrix
#   make ze-perf-evidence-update-check                   Perf bench + regression check
#
# Skip categories:
#   ZE_RELEASE_SKIP=interop,perf make ze-release-evidence-verify

.PHONY: ze-release-evidence-verify ze-release-evidence-preflight ze-functional-extra-test ze-perf-evidence-update-check

ZE_RELEASE_SKIP ?=

ifeq ($(GOKRAZY_ARCH),arm64)
ZE_RELEASE_QEMU_BIN ?= qemu-system-aarch64
else
ZE_RELEASE_QEMU_BIN ?= qemu-system-x86_64
endif

# ─── Preflight ─────────────────────────────────────────────────────────────

ze-release-evidence-preflight:
	@missing=0; advisory=0; \
	if command -v docker >/dev/null 2>&1; then \
		echo "ok: docker (interop, ipsec-interop, l2tp-interop, pppoe-interop, perf, vpp-deployment, live)"; \
	else \
		echo "MISSING: docker (required for interop, perf, deployment, live categories)"; \
		missing=1; \
	fi; \
	if command -v $(ZE_RELEASE_QEMU_BIN) >/dev/null 2>&1; then \
		echo "ok: $(ZE_RELEASE_QEMU_BIN) (qemu category)"; \
	else \
		echo "advisory: $(ZE_RELEASE_QEMU_BIN) not found (qemu category will be skipped)"; \
		advisory=1; \
	fi; \
	if [ $$advisory -gt 0 ]; then \
		printf "\n%d advisory: some categories will be skipped\n" $$advisory; \
	fi; \
	if [ $$missing -gt 0 ]; then \
		printf "\n%d required tool(s) missing\n" $$missing; \
		exit 1; \
	fi; \
	echo ""; echo "Preflight passed"

# ─── Perf gate ─────────────────────────────────────────────────────────────

ze-perf-evidence-update-check: ze-perf-build
	@echo "Running perf benchmarks (ze DUT only)..."
	@$(MAKE) --no-print-directory ze-perf-bench PERF_DUT=ze
	@echo "Appending result to history..."
	@mkdir -p test/perf/history
	@python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(json.dumps(d))" \
		test/perf/results/ze.json >> test/perf/history/ze.ndjson
	@echo "Checking for regressions..."
	@$(ZEBIN_PERF) track --check test/perf/history/ze.ndjson

# ─── Extra functional evidence ──────────────────────────────────────────────

ze-functional-extra-test: $(ZEBIN_TEST)
	@failed=0; failed_names=""; total=0; \
	run_extra() { \
		suite="$$1"; shift; \
		total=$$((total + 1)); \
		"$$@" && printf "\033[32mPASS  %s\033[0m\n" "$$suite" || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }$$suite"; printf "\033[31mFAIL  %s\033[0m\n" "$$suite"; }; \
	}; \
	run_extra static $(MAKE) --no-print-directory ze-functional-static-test; \
	run_extra traffic $(MAKE) --no-print-directory ze-functional-traffic-test; \
	run_extra vpp $(MAKE) --no-print-directory ze-functional-vpp-test; \
	run_extra l2tp-wire $(MAKE) --no-print-directory ze-functional-l2tp-wire-test; \
	if [ $$failed -gt 0 ]; then \
		printf "\n\033[31mFAIL  %d of %d extra functional suites failed: %s\033[0m\n" $$failed $$total "$$failed_names"; \
		exit 1; \
	fi; \
	printf "\n\033[32mPASS  all $$total extra functional suites\033[0m\n"

# ─── Release evidence ─────────────────────────────────────────────────────

ze-release-evidence-verify: ze-release-evidence-preflight $(ZEBIN_ZE) $(ZEBIN_TEST)
	@failed=0; failed_names=""; skipped_names=""; total=0; \
	has_docker=false; has_qemu=false; \
	if command -v docker >/dev/null 2>&1; then has_docker=true; fi; \
	if command -v $(ZE_RELEASE_QEMU_BIN) >/dev/null 2>&1; then has_qemu=true; fi; \
	skip_category() { \
		cat="$$1"; reason="$$2"; \
		skipped_names="$${skipped_names:+$$skipped_names }$$cat"; \
		printf "\033[33mSKIP  %s (%s)\033[0m\n" "$$cat" "$$reason"; \
	}; \
	run_category() { \
		cat="$$1"; shift; \
		if echo ",$(ZE_RELEASE_SKIP)," | grep -q ",$$cat,"; then \
			skip_category "$$cat" "ZE_RELEASE_SKIP"; \
			return 0; \
		fi; \
		total=$$((total + 1)); \
		printf "\n════════════════════════════════════════\n"; \
		printf "CATEGORY: %s\n" "$$cat"; \
		printf "════════════════════════════════════════\n"; \
		"$$@" && printf "\033[32mPASS  %s\033[0m\n" "$$cat" || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }$$cat"; printf "\033[31mFAIL  %s\033[0m\n" "$$cat"; }; \
	}; \
	run_if_docker() { \
		cat="$$1"; shift; \
		if echo ",$(ZE_RELEASE_SKIP)," | grep -q ",$$cat,"; then \
			skip_category "$$cat" "ZE_RELEASE_SKIP"; \
			return 0; \
		fi; \
		if [ "$$has_docker" = false ]; then \
			skip_category "$$cat" "docker unavailable"; \
			return 0; \
		fi; \
		total=$$((total + 1)); \
		printf "\n════════════════════════════════════════\n"; \
		printf "CATEGORY: %s\n" "$$cat"; \
		printf "════════════════════════════════════════\n"; \
		"$$@" && printf "\033[32mPASS  %s\033[0m\n" "$$cat" || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }$$cat"; printf "\033[31mFAIL  %s\033[0m\n" "$$cat"; }; \
	}; \
	run_if_qemu() { \
		cat="$$1"; shift; \
		if echo ",$(ZE_RELEASE_SKIP)," | grep -q ",$$cat,"; then \
			skip_category "$$cat" "ZE_RELEASE_SKIP"; \
			return 0; \
		fi; \
		if [ "$$has_qemu" = false ]; then \
			skip_category "$$cat" "$(ZE_RELEASE_QEMU_BIN) unavailable"; \
			return 0; \
		fi; \
		total=$$((total + 1)); \
		printf "\n════════════════════════════════════════\n"; \
		printf "CATEGORY: %s\n" "$$cat"; \
		printf "════════════════════════════════════════\n"; \
		"$$@" && printf "\033[32mPASS  %s\033[0m\n" "$$cat" || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }$$cat"; printf "\033[31mFAIL  %s\033[0m\n" "$$cat"; }; \
	}; \
	run_advisory() { \
		cat="$$1"; shift; \
		if echo ",$(ZE_RELEASE_SKIP)," | grep -q ",$$cat,"; then \
			skip_category "$$cat" "ZE_RELEASE_SKIP"; \
			return 0; \
		fi; \
		printf "\n════════════════════════════════════════\n"; \
		printf "CATEGORY: %s (advisory, non-gating)\n" "$$cat"; \
		printf "════════════════════════════════════════\n"; \
		"$$@" || printf "\033[33mADVISORY  %s reported issues (does not gate the release)\033[0m\n" "$$cat"; \
	}; \
	run_category verify $(MAKE) --no-print-directory ze-precommit-verify; \
	run_category chaos $(MAKE) --no-print-directory ze-chaos-test; \
	run_category fuzz $(MAKE) --no-print-directory ze-fuzz-test; \
	run_advisory mutation $(MAKE) --no-print-directory ze-mutation-test-changed; \
	run_if_docker interop $(MAKE) --no-print-directory ze-interop-test; \
	run_if_docker ipsec-interop $(MAKE) --no-print-directory ze-interop-ipsec-test; \
	run_if_docker l2tp-interop $(MAKE) --no-print-directory ze-deployment-docker-l2tp-ppp-test; \
	run_if_docker pppoe-interop $(MAKE) --no-print-directory ze-deployment-docker-pppoe-accel-test; \
	run_category functional-extra $(MAKE) --no-print-directory ze-functional-extra-test; \
	run_if_docker perf $(MAKE) --no-print-directory ze-perf-evidence-update-check; \
	run_if_qemu qemu $(MAKE) --no-print-directory ze-qemu-integration-test; \
	run_if_docker vpp-deployment $(MAKE) --no-print-directory ze-deployment-vpp-test; \
	run_if_docker live $(MAKE) --no-print-directory ze-live-test; \
	printf "\n════════════════════════════════════════\n"; \
	printf "RELEASE EVIDENCE SUMMARY\n"; \
	printf "════════════════════════════════════════\n"; \
	if [ -n "$$skipped_names" ]; then \
		printf "\033[33mSKIPPED: %s\033[0m\n" "$$skipped_names"; \
	fi; \
	if [ $$failed -gt 0 ]; then \
		printf "\033[31mFAIL  %d of %d categories failed: %s\033[0m\n" $$failed $$total "$$failed_names"; \
		printf "\n\033[33mTo run failed categories individually:\033[0m\n"; \
		for cat in $$failed_names; do \
			case "$$cat" in \
				verify) printf "  make ze-precommit-verify\n" ;; \
				chaos) printf "  make ze-chaos-test\n" ;; \
				fuzz) printf "  make ze-fuzz-test\n" ;; \
				interop) printf "  make ze-interop-test\n" ;; \
				ipsec-interop) printf "  make ze-interop-ipsec-test\n" ;; \
				l2tp-interop) printf "  make ze-deployment-docker-l2tp-ppp-test\n" ;; \
				pppoe-interop) printf "  make ze-deployment-docker-pppoe-accel-test\n" ;; \
				functional-extra) printf "  make ze-functional-extra-test\n" ;; \
				perf) printf "  make ze-perf-evidence-update-check\n" ;; \
				qemu) printf "  make ze-qemu-integration-test\n" ;; \
				vpp-deployment) printf "  make ze-deployment-vpp-test\n" ;; \
				live) printf "  make ze-live-test\n" ;; \
			esac; \
		done; \
		printf "\n"; \
		exit 1; \
	else \
		printf "\033[32mPASS  all %d categories\033[0m\n\n" $$total; \
	fi
