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
#   1. functional gating suites   ($ZE_TEST_BIN; `web` skipped -> needs agent-browser)
#   2. unit tests                 (go test ./..., no -race, cacheable)
#   3. integration tests          (-tags integration; linux-only netlink/nft/fib/...)
#
# Tunables (env): ZE_QEMU_SKIP_SUITES (comma list, default "web,firewall"),
#                 ZE_QEMU_PARALLEL (default 4), ZE_QEMU_SUITE_TIMEOUT (default 900s).
set -u

cd /workspace || { echo "error: /workspace not mounted"; exit 1; }

export ZE_TEST_NO_BUILD=1
export ZE_QEMU=1
SKIP_SUITES="${ZE_QEMU_SKIP_SUITES:-web,firewall}"
PARALLEL="${ZE_QEMU_PARALLEL:-4}"
SUITE_TIMEOUT="${ZE_QEMU_SUITE_TIMEOUT:-900s}"

# Resolve arch-suffixed binary names. The Makefile cross-compiles to
# bin/ze-linux-<arch>, bin/ze-stripped-linux-<arch>, and
# bin/ze-test-linux-<arch> so bin/ze stays the host-native binary. Put stable
# names in a VM-local directory because some UI tests execute `ze-stripped`
# through PATH.
ZE_BIN="${ZE_BIN:-bin/ze}"
ZE_STRIPPED_BIN="${ZE_STRIPPED_BIN:-bin/ze-stripped}"
ZE_TEST_BIN="${ZE_TEST_BIN:-bin/ze-test}"

workspace_path() {
	case "$1" in
	/*) printf '%s\n' "$1" ;;
	*) printf '/workspace/%s\n' "$1" ;;
	esac
}

for bin in "$ZE_BIN" "$ZE_STRIPPED_BIN" "$ZE_TEST_BIN"; do
	resolved="$(workspace_path "$bin")"
	if [ ! -x "$resolved" ]; then
		echo "error: $bin missing or not executable -- cross-compile it on the host first" >&2
		echo "       (make ze-qemu-all-test does this automatically)" >&2
		exit 1
	fi
done

QEMU_BIN_DIR="/tmp/ze-qemu-bin"
mkdir -p "$QEMU_BIN_DIR"
ln -sf "$(workspace_path "$ZE_BIN")" "$QEMU_BIN_DIR/ze"
ln -sf "$(workspace_path "$ZE_STRIPPED_BIN")" "$QEMU_BIN_DIR/ze-stripped"
ln -sf "$(workspace_path "$ZE_TEST_BIN")" "$QEMU_BIN_DIR/ze-test"
ZE_BIN="$QEMU_BIN_DIR/ze"
ZE_TEST_BIN="$QEMU_BIN_DIR/ze-test"
export ZE_BIN
export PATH="$QEMU_BIN_DIR:$PATH"

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
fsuite encode "$ZE_TEST_BIN" bgp encode --all -p "$PARALLEL"
fsuite plugin "$ZE_TEST_BIN" bgp plugin --all -p "$PARALLEL"
fsuite parse "$ZE_TEST_BIN" bgp parse --all -p "$PARALLEL"
fsuite decode "$ZE_TEST_BIN" bgp decode --all -p "$PARALLEL"
fsuite reload "$ZE_TEST_BIN" bgp reload --all -p 1
fsuite ui "$ZE_TEST_BIN" ui --all -p "$PARALLEL"
fsuite editor "$ZE_TEST_BIN" editor
fsuite managed "$ZE_TEST_BIN" managed --all -p 1
fsuite l2tp "$ZE_TEST_BIN" l2tp --all -p "$PARALLEL"
fsuite firewall "$ZE_TEST_BIN" firewall --all -p "$PARALLEL"
fsuite policy "$ZE_TEST_BIN" policy --all -p "$PARALLEL"
fsuite install "$ZE_TEST_BIN" install --all -p "$PARALLEL"

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
