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
# Serial (-p 1): firewall and policy are ZE_NETNS_SUITES members
# (mk/test-integration.mk) -- they are written for the per-test network namespace
# that `make ze-netns-test` gives them (ZE_TEST_NETNS=1, itself -p 1). The QEMU
# run has no netns isolation, so every test here shares ONE root namespace and
# concurrent tests collide on global kernel objects:
#   - the single `inet ze_pr` nft table (every policy test writes the same table,
#     and each daemon deletes it on shutdown -- policy 1/3/4/6 read another
#     test's rules or find the table already gone),
#   - the global ip-rule table (marks.go fwmarkBase makes the mark deterministic,
#     so policy 2 and 3 build the identical `mark 0x50000 table 100` rule and the
#     loser gets EEXIST),
#   - the global nft input hook. nftables evaluates EVERY base chain registered
#     at a hook and a drop in any one of them is final, so one test's chain
#     silently filters another test's traffic. firewall 1/2/10/12/13 install
#     `policy drop` input chains that accept only their own narrow match, and
#     flush-crash/flush-persist deliberately LEAVE such a chain in the kernel
#     (that persistence IS their assertion). Together they dropped firewall 4's
#     loopback `ze cli` -> SSH:2222 connection for its full 30s retry window, no
#     matter what firewall 4's own table accepted. The two flush tests now reap
#     their table on exit, so serial order is enough; do not rely on 004 sorting
#     before them.
#   - the fixed SSH port 2222 (firewall 4).
# Same reason reload/managed/traffic are serial below. Serializing restores the
# one-daemon-at-a-time assumption these suites are written against.
fsuite firewall "$ZE_TEST_BIN" firewall --all -p 1
fsuite policy "$ZE_TEST_BIN" policy --all -p 1
fsuite install "$ZE_TEST_BIN" install --all -p "$PARALLEL"
fsuite appliance "$ZE_TEST_BIN" appliance --all -p "$PARALLEL"
# IGP/MPLS suites: gated natively too, but test/ospf and test/ospfv3 contain
# option=needs-linux tests that only execute here (spec-test-coverage-gaps AC-5;
# the "full pass" claim in ai/rules/qemu-testing.md needs every gating suite).
fsuite ldp "$ZE_TEST_BIN" ldp --all -p "$PARALLEL"
fsuite rsvpte "$ZE_TEST_BIN" rsvpte --all -p "$PARALLEL"
fsuite isis "$ZE_TEST_BIN" isis --all -p "$PARALLEL"
fsuite ospf "$ZE_TEST_BIN" ospf --all -p "$PARALLEL"
fsuite ospfv3 "$ZE_TEST_BIN" ospfv3 --all -p "$PARALLEL"
# VRRP: the config/doctor tests are offline and gated natively, but every test
# that boots a daemon is needs-linux and can ONLY execute here -- the iface
# plugin fails its Config stage on darwin ("interface management not supported
# on darwin"), which cascades into vrrp failing at stage Init, so no VRRP
# runtime surface exists on the dev machine. Without this line those tests would
# skip natively and never run anywhere.
fsuite vrrp "$ZE_TEST_BIN" vrrp --all -p "$PARALLEL"
# Offline wire-decode suites (cheap, gated natively as well).
fsuite l2tp-wire "$ZE_TEST_BIN" l2tp-wire --all -p "$PARALLEL"
fsuite isis-wire "$ZE_TEST_BIN" isis-wire --all -p "$PARALLEL"
fsuite ospf-wire "$ZE_TEST_BIN" ospf-wire --all -p "$PARALLEL"
# Serial (-p 1): the needs-linux qdisc tests mutate shared kernel qdisc state on
# eth0, so parallel runs would race. Runs as root in the QEMU VM (CAP_NET_ADMIN),
# where option=needs-linux tests assert real `tc qdisc show` output.
fsuite traffic "$ZE_TEST_BIN" traffic --all -p 1

# 2. Unit tests: full pass, no -race, cacheable. Picks up the //go:build linux
#    test files that never compile on macOS.
#
# GOCACHE/GOMODCACHE are passed as make COMMAND-LINE variables, not inherited from
# the environment qemu-run.py exports. Makefile:17 does `export GOCACHE :=
# $(CURDIR)/cache/go-cache`, and a makefile assignment beats an environment
# variable, so the exported value was silently discarded for every make target.
# $(CURDIR)/cache is a symlink to the HOST's ~/.cache/ze, which does not exist in
# the VM, so go died with `failed to initialize build cache at
# /workspace/cache/go-cache: mkdir /workspace/cache: file exists` (the dangling
# symlink is the "file"). `go list ./...` failed the same way, so the package list
# collapsed to the root module and the phase reported the misleading `build
# constraints exclude all Go files in /workspace`. A command-line assignment DOES
# override the makefile and propagates to sub-makes via MAKEFLAGS.
#
# This is why the phase-3 integration tests below compiled fine while this phase
# could not: they invoke `go test` directly and never pass through the Makefile.
run_check "unit tests (no -race, cacheable)" \
	make --no-print-directory GOCACHE="$GOCACHE" GOMODCACHE="$GOMODCACHE" ze-unit-test-cached

# 3. Integration tests: linux-only, netlink/nft/fib/socket. Same package set as
#    `make ze-qemu-integration-test`; IS-IS transport is added when present.
integration_pkgs=(
	# The doctor's integration test is internal/component/doctor
	# (checks_integration_linux_test.go, TestCheckMachineIDIntegration). This
	# entry read ./cmd/ze/doctor, a directory that does not exist -- `ze doctor`
	# dispatches from cmd/ze, it has no package of its own -- so every run
	# reported `FAIL ./cmd/ze/doctor [setup failed]: stat /workspace/cmd/ze/doctor:
	# directory not found` and the check never executed. Unlike the optional
	# transport entries below it carried no `[ -d ]` guard, so the typo failed the
	# whole phase instead of being skipped.
	./internal/component/doctor/...
	./internal/component/host/...
	./internal/component/iface/...
	./internal/component/config/system/...
	./internal/core/routewatch/...
	./internal/core/network/...
	./internal/component/bgp/reactor/...
	./internal/plugins/fib/kernel/...
	./internal/plugins/firewall/nft/...
	./internal/plugins/firewall/vpp/...
	./internal/plugins/traffic/netlink/...
	./internal/plugins/tftpserver/...
	./internal/plugins/dhcpserver/...
)
if [ -d ./internal/plugins/isis/transport ]; then
	integration_pkgs+=(./internal/plugins/isis/transport/...)
fi
if [ -d ./internal/plugins/ospf/transport ]; then
	integration_pkgs+=(./internal/plugins/ospf/transport/...)
fi
if [ -d ./internal/plugins/ospf/v3/transport ]; then
	integration_pkgs+=(./internal/plugins/ospf/v3/transport/...)
fi
# VRRP transport (spec-vrrp-4): raw IP proto 112 sockets, the 224.0.0.18 /
# ff02::12 multicast joins, and the GTSM TTL=255 checks all need a real kernel
# and CAP_NET_RAW, so these tests exist only under -tags integration on linux
# (internal/plugins/vrrp/transport/transport_integration_linux_test.go).
# The macvlan side (spec-vrrp-3) needs no entry here: that code lives in
# internal/component/iface/macvlan.go, already covered by iface/... above.
if [ -d ./internal/plugins/vrrp/transport ]; then
	integration_pkgs+=(./internal/plugins/vrrp/transport/...)
fi
# IS-IS adjacency integration test (spec-isis-5): two engines reach Up over a
# real veth pair. The root isis package carries the integration-tagged test.
if [ -d ./internal/plugins/isis ]; then
	integration_pkgs+=(./internal/plugins/isis)
fi
# Fail loudly on a package path that does not exist. `go test` reports a missing
# directory as `FAIL <pkg> [setup failed]` buried among real results, which is how
# ./cmd/ze/doctor sat here broken while the phase looked merely "red as usual".
# A path typo is a bug in THIS file, not a test failure, so say so in those terms.
missing_pkgs=""
for pkg in "${integration_pkgs[@]}"; do
	if [ ! -d "${pkg%/...}" ]; then
		missing_pkgs="$missing_pkgs $pkg"
	fi
done
if [ -n "$missing_pkgs" ]; then
	red "integration_pkgs names path(s) that do not exist:$missing_pkgs"
	echo "       Fix the list in scripts/evidence/qemu-all-tests.sh; this is a script bug, not a test failure." >&2
	exit 1
fi
run_check "integration tests (-tags integration)" \
	go test -tags integration -count=1 -timeout 120s "${integration_pkgs[@]}"

banner "SUMMARY"
if [ "$rc" -eq 0 ]; then
	green "ALL PHASES PASSED"
else
	red "FAILED: $failed"
fi
exit "$rc"
