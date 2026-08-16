# ─── Cadence targets: what to run daily, weekly and monthly ─────────────────
#
# The repository has more than forty check, census and audit targets. `make
# ze-verify` runs 27 of them and the nightly workflows run a dozen more, which
# leaves a set that is in NEITHER and is therefore run by nobody. These three
# targets exist so the owner runs one command instead of remembering that set.
#
# They are deliberately NOT stages of ze-verify: `TestStagesForModeMatchesGolden`
# (scripts/status/verify_run_test.go) locks that stage list against a golden, and
# ze-verify is a merge gate whose job is to be fast and decisive. A cadence run
# is the opposite. It surfaces things, and most of what it surfaces is a census
# that has no verdict to give.
#
# So a member is one of two kinds, and the distinction is the whole design:
#
#   gate  a real check with a verdict. A non-zero exit fails the cadence run.
#   note  a census or a report. It is printed and never fails the run, because
#         `journal.py report()` and its siblings exit 0 whatever they find.
#
# Mixing the two under one exit code is what makes an aggregate meaningless:
# either the censuses drag it red every day until it is ignored, or the gates
# are swallowed. The summary table is the product; the exit code covers gates.

.PHONY: ze-daily ze-weekly ze-monthly

# The runner both targets share. `$(1)` is the banner.
#
#   run_check <gate|note> <name> <command...>
#
# Modelled on run_suite (mk/test-functional.mk): one shell, one function, a
# failure list accumulated in shell variables rather than a file, so nothing is
# left behind in a shared checkout when the run is interrupted.
define ZE_CADENCE_RUN
	@set -u; \
	failed=""; noted=""; ran=0; \
	run_check() { \
		kind="$$1"; name="$$2"; shift 2; \
		ran=$$((ran + 1)); \
		printf "\n\033[1m── %s\033[0m (%s)\n" "$$name" "$$kind"; \
		if "$$@"; then :; else \
			if [ "$$kind" = gate ]; then \
				failed="$${failed:+$$failed }$$name"; \
			else \
				noted="$${noted:+$$noted }$$name"; \
			fi; \
		fi; \
	}; \
	printf "\033[1m%s\033[0m\n" "$(1)"; \
	$(2) \
	printf "\n\033[1m── summary\033[0m\n"; \
	printf "  %d member(s) ran\n" "$$ran"; \
	if [ -n "$$noted" ]; then printf "  \033[33mnote reported: %s\033[0m\n" "$$noted"; fi; \
	if [ -n "$$failed" ]; then \
		printf "  \033[31m\033[1mgate FAILED: %s\033[0m\n" "$$failed"; \
		exit 1; \
	fi; \
	printf "  \033[32mevery gate passed\033[0m\n"
endef

# ─── Daily ──────────────────────────────────────────────────────────────────
#
# Seconds. No Docker, no QEMU, no network, and it MUST NOT take the repo-wide
# verify lock, so it never blocks on somebody else's ze-verify.
#
# ze-ste-check is a `note` and MUST stay one. CLAUDE.md is explicit that the
# writing rule "is a GUIDELINE, not a law and not a gate. The checker reports and
# lets the work through." It also measures a changed file against HEAD, so in a
# checkout five sessions share it reports on their half-finished prose as readily
# as on yours -- and a gate that reds on somebody else's work gets switched off,
# which is the reasoning test/relax-ceiling.txt already records for the census.
#
# ze-validate is here because it is the cheapest unrun gate in the repository:
# ze-verify runs ze-validate-tree, which passes `--changed-file ''`, and both
# check_cross_package_wiring and check_cli_handler_coverage (scripts/dev/validate.py)
# return empty before reading anything when the changed-file list is empty. So
# those two checks are in the tree, are wired to a target, and run nowhere.
ze-daily:
	$(call ZE_CADENCE_RUN,ze-daily -- run this one every morning,\
		run_check gate ze-validate $(MAKE) --no-print-directory ze-validate; \
		run_check note ze-ste-check $(MAKE) --no-print-directory ze-ste-check; \
		run_check note ze-spec-status $(MAKE) --no-print-directory ze-spec-status; \
		run_check note ze-journal $(MAKE) --no-print-directory ze-journal; \
		run_check note ze-setup-probe $(MAKE) --no-print-directory ze-setup CHECK=1; \
	)

# ─── Weekly ─────────────────────────────────────────────────────────────────
#
# Minutes, still local. ze-chaos-verify takes the SAME repo-wide lock as
# ze-verify (scripts/dev/verify-lock.sh, mk/test-chaos.mk), so do not start this
# beside a verify: it will block rather than fail, which looks like a hang.
#
# ze-race-reactor is here because ai/rules/testing.md requires it for every
# reactor concurrency change and nothing enforces that. ze-consistency is here
# because its own Go test only ever drives t.TempDir() fixtures, never the
# repository, so the live tree is unchecked until someone runs the target.
ze-weekly:
	$(call ZE_CADENCE_RUN,ze-weekly -- takes the verify lock; do not run beside a verify,\
		run_check gate ze-consistency $(MAKE) --no-print-directory ze-consistency; \
		run_check gate ze-race-reactor $(MAKE) --no-print-directory ze-race-reactor; \
		run_check gate ze-chaos-verify $(MAKE) --no-print-directory ze-chaos-verify; \
		run_check note ze-rules-router-report $(MAKE) --no-print-directory ze-rules-router-report; \
		run_check note ze-rfc-extraction-status $(MAKE) --no-print-directory ze-rfc-extraction-status; \
		run_check note ze-check-vendor-web-updates $(MAKE) --no-print-directory ze-check-vendor-web-updates; \
		run_check note ze-test-health-record $(MAKE) --no-print-directory ze-test-health-record; \
	)

# ─── Monthly ────────────────────────────────────────────────────────────────
#
# Needs Docker, QEMU, root or a long build, so every member is a `note`: on a
# machine without the infrastructure the member reports and the run continues.
# A gate here would only teach the owner that ze-monthly always fails.
#
# The preflight probes run FIRST and answer what this machine can do, which is
# the one thing worth knowing before the slow members start.
#
# ze-qemu-integration-test earns its place: evidence-nightly.yml excludes it
# because hosted runners have no reliable KVM, so the gate ai/rules/platform-linux.md
# mandates for linux-only code runs on no machine but this one.
ze-monthly:
	$(call ZE_CADENCE_RUN,ze-monthly -- needs Docker/QEMU/root; members report rather than gate,\
		run_check note ze-deployment-preflight $(MAKE) --no-print-directory ze-deployment-preflight; \
		run_check note ze-qemu-integration-test $(MAKE) --no-print-directory ze-qemu-integration-test; \
		run_check note ze-perf-gate $(MAKE) --no-print-directory ze-perf-gate; \
		run_check note ze-mutation-changed $(MAKE) --no-print-directory ze-mutation-changed; \
		run_check note ze-ste-review $(MAKE) --no-print-directory ze-ste-review; \
	)
