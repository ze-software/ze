# Mutation testing: gomu integration (advisory, not gating)
#
# Quick reference:
#   make ze-mutation-test       All non-excluded packages (slow on full repo)
#   make ze-mutation-changed    Only changed files (incremental, fast)
#   make ze-mutation-report     Full run with HTML report
#
# gomu is vendored in tools.go and invoked via go run. No install needed.
#
# gomu has no --tags or package-path support. It scans the entire
# module from the working directory. Files with custom build tags
# (ze_test, ze_chaos, ze_perf, ze_analyze) and other non-mutatable
# paths are excluded via .gomuignore.

.PHONY: ze-mutation-test ze-mutation-changed ze-mutation-report

GOMU_WORKERS   ?= 2
GOMU_TIMEOUT   ?= 120
GOMU_THRESHOLD ?= 0

GOMU_PROCS := $(shell n=$$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4); echo $$(( n / 2 )))
GOMU = nice -n 15 env GOMAXPROCS=$(GOMU_PROCS) go run github.com/sivchari/gomu/cmd/gomu

# Full mutation test with JSON output
ze-mutation-test:
	@set -o pipefail; \
	echo "Mutation testing: all packages..."; \
	mkdir -p tmp; \
	$(GOMU) run \
		--workers $(GOMU_WORKERS) \
		--timeout $(GOMU_TIMEOUT) \
		--threshold $(GOMU_THRESHOLD) \
		--output json \
		--incremental=false \
		--fail-on-gate=false \
		2>&1 | tee tmp/mutation.log; \
	if [ -f mutation-report.json ]; then mv mutation-report.json tmp/mutation-report.json; fi; \
	python3 scripts/dev/mutation_history.py || true; \
	python3 scripts/dev/testing_health.py --record || true; \
	echo "Report: tmp/mutation-report.json"

# Incremental mutation test on changed files only.
# The changed-pkgs.sh pre-check avoids invoking gomu when nothing changed;
# gomu's own --incremental does a separate git-diff pass internally.
ze-mutation-changed:
	@set -o pipefail; \
	pkgs=$$(scripts/dev/changed-pkgs.sh 2>/dev/null); \
	if [ -z "$$pkgs" ]; then \
		echo "No changed .go files -- skipping mutation testing"; \
		exit 0; \
	fi; \
	echo "Mutation testing: changed files (incremental)..."; \
	mkdir -p tmp; \
	$(GOMU) run \
		--workers $(GOMU_WORKERS) \
		--timeout $(GOMU_TIMEOUT) \
		--threshold $(GOMU_THRESHOLD) \
		--output json \
		--incremental \
		--base-branch=main \
		--fail-on-gate=false \
		2>&1 | tee tmp/mutation.log; \
	if [ -f mutation-report.json ]; then mv mutation-report.json tmp/mutation-report.json; fi; \
	python3 scripts/dev/mutation_history.py || true; \
	python3 scripts/dev/testing_health.py --record || true; \
	echo "Report: tmp/mutation-report.json"

# Run mutation test on one or more packages, output surviving mutants as JSON.
# Usage:
#   make ze-mutation-pkg PKG=./internal/core/textbuf/         (single package)
#   make ze-mutation-pkg PKG=./internal/core/...              (all under core/)
#   make ze-mutation-pkg PKG="./internal/core/textbuf/ ./internal/core/netutil/"  (list)
ze-mutation-pkg:
	@set -o pipefail; \
	if [ -z "$(PKG)" ]; then \
		echo "Usage: make ze-mutation-pkg PKG=./internal/core/textbuf/"; \
		echo "       make ze-mutation-pkg PKG=./internal/core/..."; \
		exit 1; \
	fi; \
	mkdir -p tmp; \
	rm -f tmp/mutation-report-*.json; \
	pkgs=""; \
	for p in $(PKG); do \
		case "$$p" in \
		*...) pkgs="$$pkgs $$(go list $$p 2>/dev/null | sed 's|^github.com/ze-software/ze/|./|')";; \
		*) pkgs="$$pkgs $$p";; \
		esac; \
	done; \
	echo "Mutation testing:$$pkgs"; \
	for pkg in $$pkgs; do \
		echo ""; \
		echo "Processing $$pkg..."; \
		$(GOMU) run \
			--workers $(GOMU_WORKERS) \
			--timeout $(GOMU_TIMEOUT) \
			--threshold 0 \
			--output json \
			--incremental=false \
			--fail-on-gate=false \
			$$pkg 2>&1; \
		if [ -f mutation-report.json ]; then \
			slug=$$(echo "$$pkg" | sed 's|^\./||; s|/$$||; s|/|-|g'); \
			mv mutation-report.json "tmp/mutation-report-$$slug.json"; \
		fi; \
	done; \
	echo ""; \
	python3 scripts/dev/mutation_combine.py; \
	python3 scripts/dev/mutation_history.py || true; \
	python3 scripts/dev/testing_health.py --record || true; \
	echo "Report: tmp/mutation-report.json"

# Full mutation test with HTML report
ze-mutation-report:
	@set -o pipefail; \
	echo "Mutation testing: generating HTML report..."; \
	mkdir -p tmp; \
	$(GOMU) run \
		--workers $(GOMU_WORKERS) \
		--timeout $(GOMU_TIMEOUT) \
		--threshold $(GOMU_THRESHOLD) \
		--output html \
		--incremental=false \
		--fail-on-gate=false \
		2>&1 | tee tmp/mutation-html.log; \
	if [ -f mutation-report.html ]; then mv mutation-report.html tmp/mutation-report.html; fi; \
	echo "Report: tmp/mutation-report.html"
