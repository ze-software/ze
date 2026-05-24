# Release evidence: full test matrix for release-readiness proof
#
# These targets compose every existing test suite into a single gate.
# ze-verify stays fast (~2 min) for pre-commit. ze-release-evidence runs
# everything: interop, chaos, fuzz, perf regression, QEMU, deployment.
#
# Quick reference:
#   make ze-release-evidence-preflight  Check required tooling
#   make ze-release-evidence            Full release evidence matrix
#   make ze-perf-gate                   Perf bench + regression check
#
# Skip categories:
#   ZE_RELEASE_SKIP=interop,perf make ze-release-evidence

.PHONY: ze-release-evidence ze-release-evidence-preflight ze-perf-gate

ZE_RELEASE_SKIP ?=

# ─── Preflight ─────────────────────────────────────────────────────────────

ze-release-evidence-preflight:
	@missing=0; advisory=0; \
	if command -v docker >/dev/null 2>&1; then \
		echo "ok: docker (interop, ipsec-interop, l2tp-interop, perf, vpp-deployment, live)"; \
	else \
		echo "MISSING: docker (required for interop, perf, deployment, live categories)"; \
		missing=1; \
	fi; \
	case "$(GOKRAZY_ARCH)" in amd64) qemu_bin=qemu-system-x86_64 ;; arm64) qemu_bin=qemu-system-aarch64 ;; *) qemu_bin=qemu-system-x86_64 ;; esac; \
	if command -v $$qemu_bin >/dev/null 2>&1; then \
		echo "ok: $$qemu_bin (qemu category)"; \
	else \
		echo "advisory: $$qemu_bin not found (qemu category will be skipped)"; \
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

ze-perf-gate: ze-perf
	@echo "Running perf benchmarks (ze DUT only)..."
	@python3 test/perf/run.py --build --test ze
	@echo "Appending result to history..."
	@mkdir -p test/perf/history
	@python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(json.dumps(d))" \
		test/perf/results/ze.json >> test/perf/history/ze.ndjson
	@echo "Checking for regressions..."
	@bin/ze-perf track --check test/perf/history/ze.ndjson

# ─── Release evidence ─────────────────────────────────────────────────────

ze-release-evidence: bin/ze bin/ze-test
	@failed=0; failed_names=""; skipped_names=""; total=0; \
	has_docker=false; has_qemu=false; \
	if command -v docker >/dev/null 2>&1; then has_docker=true; fi; \
	case "$(GOKRAZY_ARCH)" in amd64) qemu_bin=qemu-system-x86_64 ;; arm64) qemu_bin=qemu-system-aarch64 ;; *) qemu_bin=qemu-system-x86_64 ;; esac; \
	if command -v $$qemu_bin >/dev/null 2>&1; then has_qemu=true; fi; \
	run_category() { \
		cat="$$1"; shift; \
		if echo ",$(ZE_RELEASE_SKIP)," | grep -q ",$$cat,"; then \
			skipped_names="$${skipped_names:+$$skipped_names }$$cat"; \
			return 0; \
		fi; \
		total=$$((total + 1)); \
		printf "\n════════════════════════════════════════\n"; \
		printf "CATEGORY: %s\n" "$$cat"; \
		printf "════════════════════════════════════════\n"; \
		"$$@" || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }$$cat"; }; \
	}; \
	run_if_docker() { \
		cat="$$1"; shift; \
		if echo ",$(ZE_RELEASE_SKIP)," | grep -q ",$$cat,"; then \
			skipped_names="$${skipped_names:+$$skipped_names }$$cat"; \
			return 0; \
		fi; \
		if [ "$$has_docker" = false ]; then \
			skipped_names="$${skipped_names:+$$skipped_names }$$cat"; \
			return 0; \
		fi; \
		total=$$((total + 1)); \
		printf "\n════════════════════════════════════════\n"; \
		printf "CATEGORY: %s\n" "$$cat"; \
		printf "════════════════════════════════════════\n"; \
		"$$@" || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }$$cat"; }; \
	}; \
	run_if_qemu() { \
		cat="$$1"; shift; \
		if echo ",$(ZE_RELEASE_SKIP)," | grep -q ",$$cat,"; then \
			skipped_names="$${skipped_names:+$$skipped_names }$$cat"; \
			return 0; \
		fi; \
		if [ "$$has_qemu" = false ]; then \
			skipped_names="$${skipped_names:+$$skipped_names }$$cat"; \
			return 0; \
		fi; \
		total=$$((total + 1)); \
		printf "\n════════════════════════════════════════\n"; \
		printf "CATEGORY: %s\n" "$$cat"; \
		printf "════════════════════════════════════════\n"; \
		"$$@" || { failed=$$((failed + 1)); failed_names="$${failed_names:+$$failed_names }$$cat"; }; \
	}; \
	run_category verify $(MAKE) --no-print-directory ze-verify; \
	run_category chaos $(MAKE) --no-print-directory ze-chaos-test; \
	run_category fuzz $(MAKE) --no-print-directory ze-fuzz-test; \
	run_if_docker interop $(MAKE) --no-print-directory ze-interop-test; \
	run_if_docker ipsec-interop $(MAKE) --no-print-directory ze-ipsec-interop-test; \
	run_if_docker l2tp-interop $(MAKE) --no-print-directory ze-deployment-l2tp-ppp-docker-test; \
	run_category static $(MAKE) --no-print-directory ze-static-test; \
	run_category traffic $(MAKE) --no-print-directory ze-traffic-test; \
	run_category vpp $(MAKE) --no-print-directory ze-vpp-test; \
	run_category l2tp-wire $(MAKE) --no-print-directory ze-l2tp-wire-test; \
	run_if_docker perf $(MAKE) --no-print-directory ze-perf-gate; \
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
				verify) printf "  make ze-verify\n" ;; \
				chaos) printf "  make ze-chaos-test\n" ;; \
				fuzz) printf "  make ze-fuzz-test\n" ;; \
				interop) printf "  make ze-interop-test\n" ;; \
				ipsec-interop) printf "  make ze-ipsec-interop-test\n" ;; \
				l2tp-interop) printf "  make ze-deployment-l2tp-ppp-docker-test\n" ;; \
				static) printf "  make ze-static-test\n" ;; \
				traffic) printf "  make ze-traffic-test\n" ;; \
				vpp) printf "  make ze-vpp-test\n" ;; \
				l2tp-wire) printf "  make ze-l2tp-wire-test\n" ;; \
				perf) printf "  make ze-perf-gate\n" ;; \
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
