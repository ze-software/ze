#!/usr/bin/env bash
# Run the full ze test suite inside the QEMU Linux VM.
#
# Invoked by `make ze-qemu-all-test` via scripts/evidence/qemu-run.py, which has
# already: booted Alpine, 9p-mounted the repo at /workspace, loaded the ppp/l2tp/
# nft kernel modules, and installed the Go toolchain (GOCACHE/GOMODCACHE live on
# the 9p mount so they persist between runs).
#
# Design choice (host-compile, run-in-VM):
#   ze and ze-test are cross-compiled on the host (fast, lots of RAM) and shared
#   into the VM via 9p. ZE_TEST_NO_BUILD=1 makes both the functional runner and
#   the bgp suites' buildZe() reuse those binaries instead of recompiling the
#   whole tree over the slow 9p mount. Go unit/integration tests still compile in
#   the VM (incremental; cache persisted on 9p), with -race OFF because the Alpine
#   image ships no C compiler and the race detector needs CGO. Race coverage comes
#   from the native macOS / Linux-CI unit runs; the VM's unique value is the
#   //go:build linux code paths (netlink, nft, fib, procfs, ...) and the
#   integration-tagged tests, which never run on a macOS dev box.
#
# Parallelism: the project's `make ze-functional-test` tunes per-suite -p for a
# fast host/CI (parse/decode default to -p 20, encode/plugin to -p 8). An 8-vCPU
# HVF VM oversubscribes at those values -- the heavier suites (plugin) spawn ze +
# external plugin processes per test and time out waiting for CPU. So we run every
# suite at ZE_QEMU_PARALLEL (default 4) here instead of via the make target;
# reload/managed stay serial (-p 1) because they mutate shared config state.
#
# Phases (each reports pass/fail; any failure -> overall non-zero exit):
#   1. functional gating suites   (bin/ze-test; `web` skipped -> needs agent-browser)
#   2. unit tests                 (go test ./..., no -race, cacheable)
#   3. integration tests          (-tags integration; linux-only netlink/nft/fib/...)
#
# Tunables (env): ZE_QEMU_SKIP_SUITES (comma list, default "web"),
#                 ZE_QEMU_PARALLEL (default 4), ZE_QEMU_SUITE_TIMEOUT (default 900s).
set -u

cd /workspace || { echo "error: /workspace not mounted"; exit 1; }

export ZE_TEST_NO_BUILD=1
SKIP_SUITES="${ZE_QEMU_SKIP_SUITES:-web}"
PARALLEL="${ZE_QEMU_PARALLEL:-4}"
SUITE_TIMEOUT="${ZE_QEMU_SUITE_TIMEOUT:-900s}"

for bin in bin/ze bin/ze-test; do
	if [ ! -x "$bin" ]; then
		echo "error: $bin missing or not executable -- cross-compile it on the host first" >&2
		echo "       (make ze-qemu-all-test does this automatically)" >&2
		exit 1
	fi
done

green() { printf '\033[32m%s\033[0m\n' "$1"; }
red() { printf '\033[31m%s\033[0m\n' "$1"; }
banner() { printf '\n========================= %s =========================\n' "$1"; }

rc=0
failed=""

# fsuite NAME CMD... : run one functional suite under a wall-clock cap, honouring
# ZE_QEMU_SKIP_SUITES. timeout runs the suite in its own process group and kills
# the whole group on expiry so a stuck ze/plugin child cannot wedge the run.
fsuite() {
	name="$1"
	shift
	case ",$SKIP_SUITES," in
	*",$name,"*)
		printf '\n--- suite %s: SKIPPED (ZE_QEMU_SKIP_SUITES) ---\n' "$name"
		return 0
		;;
	esac
	printf '\n--- suite %s (-p as listed) ---\n' "$name"
	if timeout --kill-after=15s "$SUITE_TIMEOUT" "$@"; then
		green "suite PASS: $name"
	else
		red "suite FAIL: $name (exit $?)"
		failed="${failed:+$failed, }functional/$name"
		rc=1
	fi
}

# run_check NAME CMD... : run a non-suite phase (unit, integration).
run_check() {
	name="$1"
	shift
	banner "PHASE: $name"
	if "$@"; then
		green "PHASE PASS: $name"
	else
		red "PHASE FAIL: $name (exit $?)"
		failed="${failed:+$failed, }$name"
		rc=1
	fi
}

# 1. Functional gating suites at VM-appropriate parallelism.
banner "PHASE: functional suites (-p $PARALLEL, skip: $SKIP_SUITES)"
fsuite encode bin/ze-test bgp encode --all -p "$PARALLEL"
fsuite plugin bin/ze-test bgp plugin --all -p "$PARALLEL"
fsuite parse bin/ze-test bgp parse --all -p "$PARALLEL"
fsuite decode bin/ze-test bgp decode --all -p "$PARALLEL"
fsuite reload bin/ze-test bgp reload --all -p 1
fsuite ui bin/ze-test ui --all -p "$PARALLEL"
fsuite editor bin/ze-test editor
fsuite managed bin/ze-test managed --all -p 1
fsuite l2tp bin/ze-test l2tp --all -p "$PARALLEL"
fsuite firewall bin/ze-test firewall --all -p "$PARALLEL"
fsuite policy bin/ze-test policy --all -p "$PARALLEL"
fsuite install bin/ze-test install --all -p "$PARALLEL"

# 2. Unit tests: full pass, no -race, cacheable. Picks up the //go:build linux
#    test files that never compile on macOS.
run_check "unit tests (no -race, cacheable)" \
	make --no-print-directory ze-unit-test-cached

# 3. Integration tests: linux-only, netlink/nft/fib/socket. Same package set as
#    `make ze-qemu-integration-test`, no -race (no C compiler in the VM).
run_check "integration tests (-tags integration)" \
	go test -tags integration -count=1 -timeout 120s \
	./cmd/ze/doctor \
	./internal/component/iface/... \
	./internal/component/config/system/... \
	./internal/core/routewatch/... \
	./internal/plugins/fib/kernel/... \
	./internal/plugins/firewall/nft/... \
	./internal/plugins/firewall/vpp/... \
	./internal/plugins/traffic/netlink/... \
	./internal/plugins/tftpserver/... \
	./internal/plugins/dhcpserver/...

banner "SUMMARY"
if [ "$rc" -eq 0 ]; then
	green "ALL PHASES PASSED"
else
	red "FAILED: $failed"
fi
exit "$rc"
