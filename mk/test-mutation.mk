# Mutation testing: gomu integration (advisory, not gating)
#
# Quick reference:
#   make ze-mutation-test       All non-excluded packages (slow on full repo)
#   make ze-mutation-changed    Only changed files (incremental, fast)
#   make ze-mutation-report     Full run with HTML report
#
# Install: go install github.com/sivchari/gomu/cmd/gomu@latest
#
# gomu has no --tags or package-path support. It scans the entire
# module from the working directory. Files with custom build tags
# (ze_test, ze_chaos, ze_perf, ze_analyze) and other non-mutatable
# paths are excluded via .gomuignore.

.PHONY: ze-mutation-test ze-mutation-changed ze-mutation-report

GOMU_WORKERS   ?= $(GO_TEST_PROCS)
GOMU_TIMEOUT   ?= 120
GOMU_THRESHOLD ?= 0

# Full mutation test with JSON output
ze-mutation-test:
	@set -o pipefail; \
	if ! command -v gomu >/dev/null 2>&1; then \
		echo "gomu not installed (advisory -- not blocking)."; \
		echo "Install: go install github.com/sivchari/gomu/cmd/gomu@latest"; \
		exit 0; \
	fi; \
	echo "Mutation testing: all packages..."; \
	mkdir -p tmp; \
	gomu run \
		--workers $(GOMU_WORKERS) \
		--timeout $(GOMU_TIMEOUT) \
		--threshold $(GOMU_THRESHOLD) \
		--output json \
		--fail-on-gate=false \
		2>&1 | tee tmp/mutation.log; \
	if [ -f mutation-report.json ]; then mv mutation-report.json tmp/mutation-report.json; fi; \
	echo "Report: tmp/mutation-report.json"

# Incremental mutation test on changed files only.
# The changed-pkgs.sh pre-check avoids invoking gomu when nothing changed;
# gomu's own --incremental does a separate git-diff pass internally.
ze-mutation-changed:
	@set -o pipefail; \
	if ! command -v gomu >/dev/null 2>&1; then \
		echo "gomu not installed (advisory -- not blocking)."; \
		echo "Install: go install github.com/sivchari/gomu/cmd/gomu@latest"; \
		exit 0; \
	fi; \
	pkgs=$$(scripts/dev/changed-pkgs.sh 2>/dev/null); \
	if [ -z "$$pkgs" ]; then \
		echo "No changed .go files -- skipping mutation testing"; \
		exit 0; \
	fi; \
	echo "Mutation testing: changed files (incremental)..."; \
	mkdir -p tmp; \
	gomu run \
		--workers $(GOMU_WORKERS) \
		--timeout $(GOMU_TIMEOUT) \
		--threshold $(GOMU_THRESHOLD) \
		--output json \
		--incremental \
		--base-branch=main \
		--fail-on-gate=false \
		2>&1 | tee tmp/mutation.log; \
	if [ -f mutation-report.json ]; then mv mutation-report.json tmp/mutation-report.json; fi; \
	echo "Report: tmp/mutation-report.json"

# Full mutation test with HTML report
ze-mutation-report:
	@set -o pipefail; \
	if ! command -v gomu >/dev/null 2>&1; then \
		echo "gomu not installed (advisory -- not blocking)."; \
		echo "Install: go install github.com/sivchari/gomu/cmd/gomu@latest"; \
		exit 0; \
	fi; \
	echo "Mutation testing: generating HTML report..."; \
	mkdir -p tmp; \
	gomu run \
		--workers $(GOMU_WORKERS) \
		--timeout $(GOMU_TIMEOUT) \
		--threshold $(GOMU_THRESHOLD) \
		--output html \
		--fail-on-gate=false \
		2>&1 | tee tmp/mutation-html.log; \
	if [ -f mutation-report.html ]; then mv mutation-report.html tmp/mutation-report.html; fi; \
	echo "Report: tmp/mutation-report.html"
