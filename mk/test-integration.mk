# Integration, interop, stress, live, and deployment tests
#
# These tests require external infrastructure: Docker, network namespaces,
# CAP_NET_ADMIN, QEMU, or internet access. They are never part of the
# default `make ze-verify` gate.
#
# Quick reference:
#   make ze-interop-test               FRR/BIRD interop (Docker)
#   make ze-ipsec-interop-test         strongSwan interop (Docker + privileged)
#   make ze-stress-test                BGP stress (Linux, root, netns)
#   make ze-integration-test           All netns integration tests (CAP_NET_ADMIN)
#   make ze-netns-plugin-test          Kernel-capability plugin .ci subset (Linux + sudo)
#   make ze-live-test                  Live tests (Docker + internet)
#   make ze-qemu-integration-test      Integration tests in QEMU VM (macOS-friendly)
#   make ze-deployment-preflight       Check deployment tooling availability
#   make ze-install-iso-qemu-test       Appliance ISO installer evidence (QEMU)
#   make ze-install-scenarios-qemu-test Installer failure-path/pin/rescue evidence (QEMU)
#   make ze-install-ventoy-qemu-test    Installer Ventoy ISO-on-FAT evidence (QEMU)

.PHONY: ze-interop-test ze-ipsec-interop-test
.PHONY: ze-stress-test ze-stress-bird-test ze-stress-profile ze-stress-web-test ze-stress-fleet-test
.PHONY: ze-live-test ze-live-rpki-test
.PHONY: ze-integration-test ze-integration-iface-test ze-integration-fib-test ze-integration-firewall-test ze-integration-traffic-test ze-integration-gtsm-test ze-integration-as112-test
.PHONY: ze-netns-test ze-netns-qemu-test ze-netns-plugin-test
.PHONY: ze-release-check ze-deployment-vpp-test ze-deployment-vpp-iface-test ze-deployment-l2tp-test ze-deployment-l2tp-ppp-test
.PHONY: ze-deployment-l2tp-ppp-docker-test ze-deployment-gokrazy-l2tp-ppp-test
.PHONY: ze-deployment-pppoe-accel-docker-test
.PHONY: ze-docker-evidence ze-deployment-preflight
.PHONY: ze-qemu-integration-test ze-qemu-l2tp-ppp-test ze-qemu-pppoe-accel-test ze-qemu-pppoe-test ze-qemu-ldp-frr-test ze-qemu-isis-frr-test ze-qemu-vrrp-keepalived-test ze-qemu-traffic-usage-test ze-vpp-hugepages-qemu-test ze-install-qemu-test ze-install-iso-qemu-test ze-install-scenarios-qemu-test ze-install-ventoy-qemu-test ze-qemu-all-test ze-qemu-needs-linux-test

# ─── Interop ────────────────────────────────────────────────────────────────

INTEROP_SCENARIO ?=

ze-interop-test:
	@echo "Running interop tests (requires Docker)..."
	@python3 test/interop/run.py $(INTEROP_SCENARIO)

IPSEC_INTEROP_SCENARIO ?=

ze-ipsec-interop-test:
	@echo "Running IPsec interop tests (requires Docker + privileged containers)..."
	python3 test/ipsec-interop/run.py $(IPSEC_INTEROP_SCENARIO)

# ─── Stress ─────────────────────────────────────────────────────────────────

STRESS_SCENARIO ?=

ze-stress-test: $(ZEBIN_ZE)
	@echo "Running stress tests with ze-test peer injector (requires root + netns)..."
	@sudo ZE_BINARY=$(CURDIR)/$(ZEBIN_ZE) VERBOSE=$(VERBOSE) SESSION_TIMEOUT=$(SESSION_TIMEOUT) \
		python3 test/stress/run.py $(STRESS_SCENARIO)

ze-stress-bird-test:
	@echo "Running BIRD baseline stress test (requires root + bird2 + netns)..."
	@sudo VERBOSE=$(VERBOSE) SESSION_TIMEOUT=$(SESSION_TIMEOUT) \
		python3 test/stress/run.py 04-bulk-ipv4-bird

ze-stress-profile: $(ZEBIN_ZE)
	@echo "Running 1M profile stress test (requires root + netns)..."
	@sudo ZE_BINARY=$(CURDIR)/$(ZEBIN_ZE) ZE_PPROF=1 VERBOSE=$(VERBOSE) \
		python3 test/stress/run.py 05-profile-1m

# Evidence-tier concurrency stress tests (build-tagged, out of ze-verify per R-6).
ze-stress-web-test:
	@echo "Running web concurrent-edit stress test (>=50 editor sessions, -race)..."
	CGO_ENABLED=1 $(GO) test -tags 'ze_core stress' -race -count=1 -timeout 300s ./internal/component/web/ -run TestWebConcurrentEditStress -v

ze-stress-fleet-test:
	@echo "Running fleet many-clients perf test (128 managed clients, real hub listener)..."
	$(GO) test -tags 'ze_core fleetperf' -count=1 -timeout 300s ./cmd/ze/hub/ -run TestFleetManyClientsPerf -v

# ─── Live ───────────────────────────────────────────────────────────────────

ze-live-test: ze-live-rpki-test

ze-live-rpki-test:
	@echo "Running RPKI live test (requires Docker + internet)..."
	$(GO) test -v -tags live -timeout 180s -count=1 ./internal/component/bgp/plugins/rpki/... -run TestLive

# ─── Integration (network namespace) ────────────────────────────────────────

ze-integration-iface-test:
	@echo "Running iface integration tests (requires CAP_NET_ADMIN)..."
	CGO_ENABLED=1 $(GO) test -tags integration -count=1 -race -timeout 120s ./internal/component/iface/...

ze-integration-fib-test:
	@echo "Running FIB kernel integration tests (requires CAP_NET_ADMIN)..."
	CGO_ENABLED=1 $(GO) test -tags integration -count=1 -race -timeout 120s ./internal/plugins/fib/kernel/...

ze-integration-firewall-test:
	@echo "Running firewall nft integration tests (requires CAP_NET_ADMIN)..."
	CGO_ENABLED=1 $(GO) test -tags integration -count=1 -race -timeout 120s ./internal/plugins/firewall/nft/...

ze-integration-traffic-test:
	@echo "Running traffic-control netlink integration tests (requires CAP_NET_ADMIN)..."
	CGO_ENABLED=1 $(GO) test -tags integration -count=1 -race -timeout 120s ./internal/plugins/traffic/netlink/...

ze-integration-gtsm-test:
	@echo "Running BGP GTSM / TTL-security socket-option integration tests (linux)..."
	CGO_ENABLED=1 $(GO) test -tags integration -count=1 -race -timeout 120s ./internal/core/network/... ./internal/component/bgp/reactor/...

ze-integration-as112-test:
	@echo "Running AS112 privileged-port-53 DNS-serving integration tests (requires CAP_NET_BIND_SERVICE/root)..."
	CGO_ENABLED=1 $(GO) test -tags integration -count=1 -race -timeout 60s ./internal/plugins/as112/...

ze-integration-test: ze-integration-iface-test ze-integration-fib-test ze-integration-firewall-test ze-integration-traffic-test ze-integration-gtsm-test ze-integration-as112-test

# ─── Per-test netns launch mode (Fix B, spec-netlink-ci-harness) ─────────────
# Run the netlink functional .ci suites (firewall/policy/ospf/ospfv3) HOST-SAFELY
# on a Linux host: each test gets its own throwaway network namespace and the ze
# daemon runs as a NORMAL user off a setcap'd binary (ambient CAP_NET_ADMIN), so
# the nft firewall / FIB it programs lives in the per-test netns and the host is
# never touched. The runner (root, via sudo) creates the netns (CAP_SYS_ADMIN)
# and drops ze via SysProcAttr.Credential; ZE_TEST_NETNS=1 arms the path.
#
# Requires Linux + sudo + setcap (libcap) + nft (nftables). Asserts the host
# `nft list tables` is byte-identical before and after (R-2 host-safety). The
# setcap'd binary refuses to program nft if it ever lands in the host netns
# (refuseHostNetnsFirewall), the belt to this braces. See docs/functional-tests.md.
#
# PATH is forwarded because sudo resets it to secure_path, which on a default
# Debian/Ubuntu install does NOT contain /usr/local/go/bin. Several ospf/ospfv3
# tests exec `go` themselves (a helper peer), so without this they fail with
# `start go: exec: "go": executable file not found in $PATH` -- 79 of 97 ospf
# tests, which reads like a protocol regression and is not one.
#
# ZE_TEST_NO_BUILD=1 is REQUIRED, not an optimisation. The runner's Build()
# rebuilds with `go build -o $(ZEBIN_ZE)` (internal/test/runner/runner.go:227), and
# file capabilities are an xattr on the inode, so a rebuild silently DISCARDS the
# setcap applied two lines above -- the daemon then runs without CAP_NET_ADMIN and
# every test in all four suites fails. It also removes the need for `go` on sudo's
# secure_path, which it is not on a default Debian/Ubuntu install. The target
# already declares $(ZEBIN_ZE), $(ZEBIN_STRIPPED) and $(ZEBIN_TEST) as prerequisites, so
# the binaries are current before setcap runs.
ZE_NETNS_SUITES ?= firewall policy ospf ospfv3
ZE_NETNS_CAPS   ?= cap_net_admin,cap_net_raw,cap_net_bind_service+ep
ze-netns-test: $(ZEBIN_ZE) $(ZEBIN_STRIPPED) $(ZEBIN_TEST)
	@[ "$$(uname)" = "Linux" ] || { echo "ze-netns-test requires Linux (netns/nft); on macOS use ze-netns-qemu-test"; exit 1; }
	@command -v setcap >/dev/null 2>&1 || { echo "error: setcap not found (install libcap2-bin / libcap)"; exit 1; }
	@command -v nft    >/dev/null 2>&1 || { echo "error: nft not found (install nftables)"; exit 1; }
	@echo "Granting ambient caps to $(ZEBIN_ZE) + $(ZEBIN_STRIPPED) ($(ZE_NETNS_CAPS))..."
	sudo setcap $(ZE_NETNS_CAPS) $(ZEBIN_ZE)
	sudo setcap $(ZE_NETNS_CAPS) $(ZEBIN_STRIPPED)
	@before=$$(sudo nft list tables 2>/dev/null | sort); failed=0; \
	for suite in $(ZE_NETNS_SUITES); do \
		printf "\n=== netns suite: %s ===\n" "$$suite"; \
		sudo env PATH="$$PATH" ZE_TEST_NO_BUILD=1 ZE_TEST_NETNS=1 ZE_TEST_UID=$$(id -u) ZE_TEST_GID=$$(id -g) \
			$(ZEBIN_TEST) $$suite --all -p 1 || failed=$$((failed + 1)); \
	done; \
	after=$$(sudo nft list tables 2>/dev/null | sort); \
	sudo setcap -r $(ZEBIN_ZE) $(ZEBIN_STRIPPED) 2>/dev/null || true; \
	: "give the shared port-lock dir back to the caller (see ZE_NETNS_PORT_LOCK_RESTORE below)"; \
	$(ZE_NETNS_PORT_LOCK_RESTORE); \
	if [ "$$before" != "$$after" ]; then \
		printf "\033[31mHOST-SAFETY FAILURE: host nft tables changed during netns run\033[0m\n"; \
		printf -- "--- before ---\n%s\n--- after ---\n%s\n" "$$before" "$$after"; \
		failed=$$((failed + 1)); \
	else \
		printf "\033[32mhost nft tables unchanged (host-safe)\033[0m\n"; \
	fi; \
	[ $$failed -eq 0 ] || { printf "\033[31mnetns run FAILED (%d issue(s))\033[0m\n" "$$failed"; exit 1; }; \
	printf "\033[32mnetns run OK\033[0m\n"

# ─── Kernel-capability subset of the plugin suite (netns launch mode) ───────
# Six test/plugin tests need real kernel capabilities and are therefore NOT
# provable by `make ze-plugin-test` on an unprivileged host:
#
#   show-l2tp-history / show-l2tp-sessions / show-l2tp-session-detail /
#   teardown-session / teardown-session-all
#       Each establishes an L2TP session. `ze.l2tp.skip-kernel-probe=true`
#       bypasses only the modprobe at Start (internal/component/l2tp/
#       subsystem.go), NOT the kernel data plane: whenever resolveGenlFamily
#       succeeds -- i.e. on any host where l2tp_netlink is loaded --
#       newSubsystemKernelWorker builds a real worker, ICCN sets
#       kernelSetupNeeded, and the genl tunnel create needs CAP_NET_ADMIN.
#       Without it the create returns EPERM, handleKernelError tears the
#       session down, and the observer reports "session never established".
#       The kernel then also needs a PPPoL2TP socket and /dev/ppp, which is
#       mode 0600 root:root, hence cap_dac_override.
#   show-system-kernel-log
#       Reads /dev/kmsg, which a host with kernel.dmesg_restrict=1 refuses
#       without CAP_SYSLOG.
#
# These do NOT carry option=needs-linux:caps=..., deliberately. That marker
# skips on non-Linux too, and all six PASS on macOS and on any Linux host
# whose kernel has no l2tp genl family -- there the kernel worker is never
# built and the tests exercise the control plane alone. Marking them would
# delete real coverage to hide a host-specific requirement
# (ai/rules/testing.md), so the requirement gets a RUNNER instead.
#
# Host safety comes from the same per-test netns launch mode as ze-netns-test
# above (ZE_TEST_NETNS=1): each test runs in a throwaway namespace and ze runs
# as a NORMAL user off a setcap'd binary, so the kernel L2TP tunnel and the
# pppN interface it creates live and die inside that namespace. Run privileged
# in the HOST namespace instead and a real `ppp0` plus a real kernel L2TP
# tunnel appear on the operator's machine (verified 2026-07-28), which is
# exactly what this mode exists to prevent. The recipe asserts `ip l2tp show`
# and `ip -br link` are byte-identical before and after, the same shape as the
# nft assertion in ze-netns-test.
#
# The setcap lands on the THROWAWAY isolated binary set ($(ZE_ALT_BIN), built
# by the same $(ZE_ALT_BUILD) every functional target uses), never on the dev
# bin/ze, and the whole directory is removed by the trap on exit -- so no
# capability-bearing binary survives the run even if it fails. ZE_TEST_NO_BUILD=1
# is REQUIRED: file capabilities are an inode xattr, so a rebuild mid-run would
# silently discard them (same trap as ze-netns-test).
#
# The trap removes the directory under sudo FIRST, then again as the caller.
# The runner runs as root here and derives ze's config dir from the binary's
# parent (internal/core/paths/paths.go isBinDir), so it creates a root-owned
# etc/ze inside the throwaway root; the plain user-level rm then removed bin/
# and failed on etc/, leaving a root-owned directory behind on every run.
#
# Fails LOUDLY when the privilege is unavailable: no Linux, no sudo, or no
# setcap is an error exit, never a silent skip (ai/rules/evidence.md).
#
# ZE_NETNS_PORT_LOCK_RESTORE undoes the one piece of state a root runner leaves
# OUTSIDE the repo. The port allocator locks each candidate port with a file in
# $TMPDIR/ze-test-port-locks (internal/test/runner/ports.go), a directory shared
# by every runner on the machine; run as root it creates root-owned lock files
# there, and the NEXT unprivileged `make ze-plugin-test` then dies on
# "allocate ports: open port lock 3926: permission denied" -- a failure with no
# visible connection to the privileged run that caused it.
ZE_NETNS_PORT_LOCK_RESTORE = sudo chown -R $$(id -u):$$(id -g) "$${TMPDIR:-/tmp}/ze-test-port-locks" 2>/dev/null || true
ZE_NETNS_PLUGIN_CAPS ?= cap_net_admin,cap_net_raw,cap_net_bind_service,cap_dac_override,cap_syslog+ep
# Only show-system-kernel-log remains here. The five L2TP tests this target was
# built for (show-l2tp-history, show-l2tp-session-detail, show-l2tp-sessions,
# teardown-session, teardown-session-all) now set
# ze.l2tp.disable-kernel-dataplane, so no kernel worker is built, nothing is
# programmed into the kernel, and they pass in a plain unprivileged
# `make ze-plugin-test` -- verified 3x 5/5. Running them HERE would prove nothing
# extra: the knob disables the data plane whether or not the caps are present,
# and the netns vehicle is ~5x slower (20.1s against their 20s budget, i.e. an
# outright timeout) for a run that exercises the same control-plane path.
#
# show-system-kernel-log cannot be freed the same way: it reads /dev/kmsg, which
# is crw------- root root and gated by kernel.dmesg_restrict, so it needs
# cap_syslog + cap_dac_override rather than a knob that skips work.
#
# Adding a test back here is right whenever it genuinely needs a capability;
# adding one that merely CAN run here is not.
ZE_NETNS_PLUGIN_TESTS ?= show-system-kernel-log
ze-netns-plugin-test:
	@[ "$$(uname)" = "Linux" ] || { echo "error: ze-netns-plugin-test requires Linux (netns + kernel L2TP); there is no macOS equivalent"; exit 1; }
	@command -v setcap >/dev/null 2>&1 || { echo "error: setcap not found (install libcap2-bin / libcap)"; exit 1; }
	@sudo -n true >/dev/null 2>&1 || { echo "error: passwordless sudo required (the runner needs CAP_SYS_ADMIN to create the per-test netns)"; exit 1; }
	@trap 'sudo rm -rf $(ZE_ALT_DIR) 2>/dev/null; $(ZE_ALT_TRAP); $(ZE_NETNS_PORT_LOCK_RESTORE)' EXIT; $(ZE_ALT_BUILD) \
	printf 'Granting %s to the throwaway %s...\n' '$(ZE_NETNS_PLUGIN_CAPS)' '$(ZE_ALT_BIN)/ze'; \
	sudo setcap $(ZE_NETNS_PLUGIN_CAPS) $(ZE_ALT_BIN)/ze || exit 1; \
	before_l2tp=$$(sudo ip l2tp show tunnel 2>/dev/null; sudo ip l2tp show session 2>/dev/null); \
	before_link=$$(ip -br link 2>/dev/null | sort); \
	failed=0; \
	sudo env PATH="$$PATH" ZE_TEST_NO_BUILD=1 ZE_TEST_NETNS=1 ZE_TEST_UID=$$(id -u) ZE_TEST_GID=$$(id -g) \
		ZE_BIN=$(ZE_ALT_BIN)/ze ZE_TEST_BIN=$(ZE_ALT_BIN)/ze-test \
		$(SUITE_RUN) $(ZE_ALT_BIN)/ze-test bgp plugin -p 1 $(ZE_NETNS_PLUGIN_TESTS) || failed=1; \
	sudo setcap -r $(ZE_ALT_BIN)/ze 2>/dev/null || true; \
	after_l2tp=$$(sudo ip l2tp show tunnel 2>/dev/null; sudo ip l2tp show session 2>/dev/null); \
	after_link=$$(ip -br link 2>/dev/null | sort); \
	if [ "$$before_l2tp" != "$$after_l2tp" ] || [ "$$before_link" != "$$after_link" ]; then \
		printf "\033[31mHOST-SAFETY FAILURE: host L2TP or link state changed during the run\033[0m\n"; \
		printf -- "--- l2tp before ---\n%s\n--- l2tp after ---\n%s\n" "$$before_l2tp" "$$after_l2tp"; \
		printf -- "--- links before ---\n%s\n--- links after ---\n%s\n" "$$before_link" "$$after_link"; \
		failed=1; \
	else \
		printf "\033[32mhost L2TP tunnels/sessions and links unchanged (host-safe)\033[0m\n"; \
	fi; \
	[ $$failed -eq 0 ] || { printf "\033[31mze-netns-plugin-test FAILED\033[0m\n"; exit 1; }; \
	printf "\033[32mze-netns-plugin-test OK (%s)\033[0m\n" '$(words $(ZE_NETNS_PLUGIN_TESTS)) tests'

# ─── Deployment evidence ────────────────────────────────────────────────────

ze-deployment-vpp-test:
	@echo "Running real VPP daemon deployment test (requires Docker + privileged container)..."
	python3 scripts/evidence/effective-vpp.py

ze-deployment-vpp-iface-test:
	@echo "Running real VPP interface-feature deployment test (tunnels/mirror/wireguard/LCP; requires Docker + privileged container)..."
	python3 scripts/evidence/effective-vpp-iface.py

ze-release-check:
	@echo "Running clean release-candidate verification (requires Docker + clean worktree)..."
	bash scripts/evidence/effective-verify.sh

ze-deployment-l2tp-test:
	@echo "Running external L2TP peer deployment test (requires Docker + privileged container)..."
	python3 scripts/evidence/effective-l2tp-peer.py

ze-deployment-l2tp-ppp-test:
	@echo "Running full L2TP PPP/NCP peer deployment test (requires Linux root + xl2tpd + pppd + ping + PPPoL2TP kernel support)..."
	python3 scripts/evidence/effective-l2tp-ppp.py

ze-deployment-l2tp-ppp-docker-test:
	@echo "Running L2TP PPP/NCP peer-isolated Docker lab (requires Docker host PPPoL2TP kernel support)..."
	python3 test/l2tp-interop/run.py

ze-deployment-pppoe-accel-docker-test:
	@echo "Running PPPoE client-vs-accel-ppp Docker lab (requires Docker host PPPoE kernel support)..."
	python3 test/pppoe-interop/run.py

ze-deployment-gokrazy-l2tp-ppp-test:
	@echo "Running gokrazy appliance L2TP PPP/NCP deployment test (requires Linux root + QEMU + xl2tpd + pppd + PPPoL2TP support)..."
	python3 scripts/evidence/effective-gokrazy-l2tp-ppp.py

EVIDENCE_SCRIPT ?=
EVIDENCE_PACKAGES ?=
ze-docker-evidence:
	@test -n "$(EVIDENCE_SCRIPT)" || { echo "error: set EVIDENCE_SCRIPT=scripts/evidence/foo.py"; exit 1; }
	python3 scripts/evidence/docker-run.py $(EVIDENCE_SCRIPT) $(EVIDENCE_PACKAGES)

ze-deployment-preflight:
	@missing=0; \
	if command -v target-runner >/dev/null 2>&1; then echo "ok: target-runner"; else echo "missing: target-runner (target environment release evidence)"; missing=1; fi; \
	if command -v docker >/dev/null 2>&1; then echo "ok: docker for clean ze-verify substitute"; else echo "missing: docker (local clean ze-verify substitute)"; missing=1; fi; \
	if command -v vpp >/dev/null 2>&1 && command -v vppctl >/dev/null 2>&1; then echo "ok: vpp + vppctl"; elif command -v docker >/dev/null 2>&1; then echo "ok: docker for real VPP daemon evidence"; else echo "missing: vpp/vppctl or docker (real VPP daemon evidence)"; missing=1; fi; \
	if command -v docker >/dev/null 2>&1; then echo "ok: docker for xl2tpd L2TP control-session evidence"; else echo "missing: docker (L2TP control-session evidence)"; missing=1; fi; \
	case "$(GOKRAZY_ARCH)" in amd64) qemu_bin=qemu-system-x86_64 ;; arm64) qemu_bin=qemu-system-aarch64 ;; *) qemu_bin=qemu-system-x86_64 ;; esac; \
	if command -v $$qemu_bin >/dev/null 2>&1; then echo "ok: $$qemu_bin for gokrazy appliance evidence (GOKRAZY_ARCH=$(GOKRAZY_ARCH))"; else echo "missing: $$qemu_bin (gokrazy appliance evidence, GOKRAZY_ARCH=$(GOKRAZY_ARCH))"; missing=1; fi; \
	if [ -e /dev/ppp ] && { [ -e /proc/net/pppol2tp ] || [ -d /sys/module/l2tp_ppp ] || [ -d /sys/module/pppol2tp ]; } && command -v ip >/dev/null 2>&1 && command -v ping >/dev/null 2>&1 && command -v xl2tpd >/dev/null 2>&1 && command -v pppd >/dev/null 2>&1; then echo "ok: xl2tpd + pppd + ping + /dev/ppp + iproute2 + PPPoL2TP kernel support for full PPP/NCP peer evidence"; else echo "missing: xl2tpd + pppd + ping + /dev/ppp + iproute2 + PPPoL2TP kernel support (full PPP/NCP peer evidence)"; missing=1; fi; \
	exit $$missing

# ─── QEMU ───────────────────────────────────────────────────────────────────

# Full test suite in one QEMU Linux VM, host-compile / run-in-VM:
#   - ze, ze-stripped, and ze-test are cross-compiled on the host (CGO off) to
#     arch-suffixed names (bin/ze-linux-<arch>, bin/ze-stripped-linux-<arch>,
#     bin/ze-test-linux-<arch>) and shared into the VM over 9p; ZE_BIN,
#     ZE_STRIPPED_BIN, and ZE_TEST_BIN tell the runner where to find them, and
#     ZE_TEST_NO_BUILD=1 skips recompilation on the slow 9p mount.
#   - unit + integration Go tests still compile in the VM (incremental, cache on
#     9p), no -race (Alpine has no C compiler; race coverage comes from native /
#     Linux-CI unit runs). The VM's unique value: //go:build linux paths and the
#     integration-tagged netlink/nft/fib/socket tests.
# GOARCH is derived from the host (Apple Silicon -> arm64, Intel -> amd64) so the
# binaries match the VM. ZE_QEMU_SKIP_SUITES (default: web) lets you drop
# suites: web needs agent-browser.
#
# firewall left that default on 2026-08-07. It was there because the suite
# "crashes the Alpine QEMU kernel on nft set-element-timeout operations", and
# both functional targets now boot ze's own runtime kernel instead of stock
# Alpine (see ze-qemu-kernel-guard below). Measured on 7.1.4: `nft add set ...
# flags timeout` then `nft add element ... timeout 5s` both succeed, the element
# reads back with its expiry, and the VM survives a following `nft flush
# ruleset`. The stock Alpine 6.12.13-0-virt kernel is the one that cannot take
# it, and no target runs on it any more.
# Cross-compiled binaries take the name ze-linux-<arch> so $(ZEBIN_ZE) stays the
# host-native binary. No need to run `make ze test` after QEMU testing.
QEMU_GOARCH := $(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')
# $(ZE_BIN_DIR) (mk/session.mk) is bin/ off-session and this session's own
# directory under an AI session, so two sessions cross-compiling for the VM at
# once cannot overwrite each other's DUT binaries mid-run. Every consumer below
# goes through these three variables, so the directory reaches the 9p share and
# the in-VM ZE_BIN/ZE_TEST_BIN without any literal name needing to know about it.
ZE_QEMU_BIN := $(ZE_BIN_DIR)/ze-linux-$(QEMU_GOARCH)
ZE_QEMU_STRIPPED_BIN := $(ZE_BIN_DIR)/ze-stripped-linux-$(QEMU_GOARCH)
ZE_QEMU_TEST_BIN := $(ZE_BIN_DIR)/ze-test-linux-$(QEMU_GOARCH)
# Exported so helper scripts print and use the paths this run actually built,
# rather than re-deriving an unsuffixed literal (scripts/evidence/qemu-run.py's
# copy-paste hint, scripts/evidence/netns_qemu.py's exec).
export ZE_QEMU_BIN ZE_QEMU_STRIPPED_BIN ZE_QEMU_TEST_BIN
ZE_QEMU_SKIP_SUITES ?= web
ZE_QEMU_PARALLEL ?= 4

# ─── The runtime kernel both functional QEMU targets boot ───────────────────
#
# Staged by `make ze-kernel` (mk/gokrazy.mk), which materializes it from the
# durable arch+config-keyed cache under ~/.cache/ze in seconds on a hit, and
# builds only on a miss.
ZE_QEMU_KERNEL := tmp/kernel/vmlinuz

# Refuse to boot anything but THIS host's architecture and THIS tree's kernel
# config, and never fall back to stock Alpine.
#
# `test -f $(ZE_QEMU_KERNEL)` alone, which is what every kernel-consuming target
# used to do, is not
# enough: GOKRAZY_ARCH defaults to amd64 (mk/gokrazy.mk), tmp/kernel/vmlinuz is
# not keyed by architecture, and QEMU_GOARCH follows uname. So a bare
# `make ze-kernel` on an Apple Silicon host stages an amd64 vmlinuz, an
# existence-only guard accepts it, and the VM dies during boot with no line that
# names the architecture.
#
# The architecture is not re-derived here. ze-host owns the cache key
# (kernelCacheVariantFor, internal/appliance/cache.go), which already hashes the
# architecture AND every resolved config fragment. Comparing the staged kernel
# against the key's own cache entry therefore answers both questions at once --
# right architecture, and current config -- and adds no second copy of the
# bzImage/Image magic numbers to keep in step (ai/rules/evidence.md).
#
# Fail closed. A silent fall back to the stock Alpine kernel would restore the
# nft crash quietly, which is the one outcome worse than an error message.
#
# EVERY target that uses this guard MUST declare `: ze-host`. The first command
# below execs $(CURDIR)/ze-host, which a clean checkout does not have. Without
# the prerequisite the guard still fails closed, but it fails with
# `sh: ze-host: not found` and then reports the CACHE branch's message, whose
# hint (`make ze-kernel`) does not fix the actual problem. That is a guard that
# denies while naming the wrong cause, and it is what
# TestQemuTargetsGuardTheStagedKernel now checks for every user of the guard
# rather than for a hand-written list of two.
define ze-qemu-kernel-guard
@hint="run: make ze-kernel KERNEL_ARCH=$(QEMU_GOARCH)"; \
	cache_dir="$$("$(CURDIR)/ze-host" appliance kernel --target runtime --arch $(QEMU_GOARCH) --print-cache-dir)"; \
	test -f "$(ZE_QEMU_KERNEL)" || { echo "error: $(ZE_QEMU_KERNEL) not found -- this target boots ze's runtime kernel and never stock Alpine ($$hint)"; exit 1; }; \
	{ test -n "$$cache_dir" && test -f "$$cache_dir/vmlinuz"; } || { echo "error: no $(QEMU_GOARCH) runtime kernel in the durable cache ($$hint)"; exit 1; }; \
	cmp -s "$(ZE_QEMU_KERNEL)" "$$cache_dir/vmlinuz" || { echo "error: $(ZE_QEMU_KERNEL) is not this tree's $(QEMU_GOARCH) runtime kernel -- wrong architecture, or a kernel config fragment changed after it was staged ($$hint)"; exit 1; }; \
	echo "Runtime kernel: $(ZE_QEMU_KERNEL) ($(QEMU_GOARCH), matches $$cache_dir)"
endef

# The QEMU ze build deliberately OMITS ZE_LDFLAGS (the version/buildDate -X
# stamps). The .ci suites reuse this binary via ZE_TEST_NO_BUILD=1, and the
# native parse runner (cmd/ze-test buildZe) builds ze WITHOUT version ldflags,
# so test/parse/cli-show-version.ci asserts `ze show version` prints "ze dev".
# Stamping a real version here makes that test fail spuriously. Keep it unstamped.
#
# The tag set MUST match internal/test/runner TestBuildTags (runner.go:50):
# zetest + ze_core + ze_distro + ze_setup + every default-on feature tag. It
# omitted ze_setup and $(ZE_FEATURES) until 2026-07-23, so the VM ran a daemon
# with NO ssh, web, bgp, isis, ospf, vrrp, ldp, rsvpte, gnmi or telemetry -- not
# the binary `make ze` ships. That surfaced as unrelated-looking failures: a
# config using `system { authentication { user ... } }` was rejected with
# "unknown field in authentication: user", because that leaf is declared in
# internal/component/ssh/yang/ze-ssh-conf.yang and ze_ssh gates that package
# (feature-gates.txt:35).
#
# ZE_QEMU_DUT_TAGS captures that exact set ONCE, and every QEMU target that
# builds $(ZE_QEMU_BIN) uses it -- so no target can hand-write a narrower set
# and drift (it drifted three times). zetest without ze_setup $(ZE_FEATURES)
# pulls in fakeddos, whose YANG imports ze-ddos-detect-conf (ze_ddos), and then
# every config load dies "no such module: ze-ddos-detect-conf".
ZE_QEMU_DUT_TAGS := ze_core zetest ze_distro ze_setup $(ZE_FEATURES) $(ZE_TAGS)
# The stripped DUT is deliberately the MINIMAL build -- it is what
# test/ui/ze-stripped-surface.ci asserts against, so it must NOT pick up
# $(ZE_FEATURES). That is the one build here that does not derive from
# ZE_QEMU_DUT_TAGS, and the reason is this line rather than an oversight.
ZE_QEMU_STRIPPED_TAGS := ze_core $(ZE_TAGS)
ZE_QEMU_TEST_TAGS := ze_test $(ZE_FEATURES) $(ZE_TAGS)

# Every QEMU target cross-compiles the SAME three binaries. Spelling that out
# per target let ze-qemu-debug and ze-qemu-shell build only two of them while
# ze-qemu-debug's PATH shim still symlinked the third: bin/ze-stripped-linux-*
# was never produced, so /tmp/ze-qemu-bin/ze-stripped dangled and any suite that
# execs it (test/ui, via ZE_TEST_DEPS_STRIPPED) could not be debugged with the
# one target whose entire job is reproducing a failure. Same drift class as the
# tag sets above: the fix is one definition, used by all five call sites, so a
# sixth target cannot build a partial set.
define ze-qemu-crossbuild
@echo "Cross-compiling linux/$(QEMU_GOARCH) ze + ze-stripped + ze-test on host (CGO off)..."
@mkdir -p $(ZE_BIN_DIR)
CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags '$(ZE_QEMU_DUT_TAGS)' -o $(ZE_QEMU_BIN) ./cmd/ze
CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags '$(ZE_QEMU_STRIPPED_TAGS)' -o $(ZE_QEMU_STRIPPED_BIN) ./cmd/ze
CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags '$(ZE_QEMU_TEST_TAGS)' -o $(ZE_QEMU_TEST_BIN) ./cmd/ze
endef

ze-qemu-all-test: ze-host
	$(ze-qemu-kernel-guard)
	$(ze-qemu-crossbuild)
	@echo "Running full test suite in QEMU Linux VM (host-compiled binaries; no in-VM ze/ze-test compile)..."
	# e2fsprogs: same reason as ze-qemu-needs-linux-test below -- this target runs
	# the same qemu-all-tests.sh, so its unit phase needs debugfs/mkfs.ext4 too.
	python3 scripts/evidence/qemu-run.py \
		--kernel $(ZE_QEMU_KERNEL) \
		--packages "make coreutils nftables iproute2 iputils-ping kmod iptables e2fsprogs e2fsprogs-extra" \
		--timeout 3600 \
		--run 'ZE_BIN="$(ZE_QEMU_BIN)" ZE_STRIPPED_BIN="$(ZE_QEMU_STRIPPED_BIN)" ZE_TEST_BIN="$(ZE_QEMU_TEST_BIN)" ZE_QEMU_SKIP_SUITES="$(ZE_QEMU_SKIP_SUITES)" ZE_QEMU_PARALLEL="$(ZE_QEMU_PARALLEL)" bash scripts/evidence/qemu-all-tests.sh'

# Tight loop: run ONLY the Linux-only functional tests (option=needs-linux) in a
# single QEMU Linux VM. These tests SKIP natively on darwin (so `make ze-verify`
# stays green and fast) and are validated here instead. ZE_QEMU_LINUX_ONLY=1
# flips the runner to skip every test that is NOT marked needs-linux, so the VM
# spends its time only on the Linux-only surface -- one VM boot, all the
# Linux-only tests, never one VM per test. See ai/rules/platform-linux.md.
#
# web is skipped (browser-driven, not a kernel-feature surface); every other
# suite runs so a needs-linux test in any of them (plugin, firewall, l2tp, ...)
# is exercised.
ze-qemu-needs-linux-test: ze-host
	$(ze-qemu-kernel-guard)
	$(ze-qemu-crossbuild)
	@echo "Running ONLY option=needs-linux tests in QEMU Linux VM (ZE_QEMU_LINUX_ONLY=1)..."
	# 5400s. 1800s was sized when the unit phase died on startup (GOCACHE pointed
	# through a host-only symlink, see qemu-all-tests.sh) and so cost seconds.
	# With that repaired the phase compiles and runs the whole tree in the VM,
	# which alone exceeded the old budget: the run was killed mid-unit-phase and
	# the integration phase never executed. It was then raised to 3600s -- and
	# the first real scheduled execution (GitHub run 30249183064, 2026-07-27)
	# MEASURED 3269s end to end on an ubuntu-latest runner WITH working KVM:
	#
	#   08:17:12 qemu-run.py starts    08:17:24 VM bootstrapped (9.8s boot)
	#   08:17:44 functional suites     08:31:03 in-VM unit phase (GOMAXPROCS=2)
	#   09:11:41 done
	#
	# 91% of the budget, with the ~40-minute in-VM unit phase dominating. A margin
	# that thin means any growth in the tree -- or one run without KVM, where TCG
	# costs several times more -- trips the cap, and a wall-clock kill presents as
	# an opaque QEMU timeout rather than as a test result. The cap exists to catch
	# a HANG, so it must sit well clear of a healthy run: 5400s is ~1.65x the
	# measured time and still inside the workflow's own 120-minute job timeout.
	#
	# e2fsprogs + e2fsprogs-extra supply mkfs.ext4/e2fsck and debugfs. Alpine splits
	# debugfs into the -extra package, and resolveE2FSDir (internal/appliance/
	# cmd_build.go:45-66) requires mkfs.ext4 AND debugfs in the SAME directory, so
	# e2fsprogs alone left e2fsDir empty and every tool read as absent -- injectZeFS
	# logged "e2fsck not found" and "debugfs write silently failed" with e2fsprogs
	# demonstrably installed, and four tests failed on that rather than on ze.
	python3 scripts/evidence/qemu-run.py \
		--kernel $(ZE_QEMU_KERNEL) \
		--packages "make coreutils nftables iproute2 iputils-ping kmod iptables e2fsprogs e2fsprogs-extra" \
		--timeout 5400 \
		--run 'ZE_QEMU_LINUX_ONLY=1 ZE_BIN="$(ZE_QEMU_BIN)" ZE_STRIPPED_BIN="$(ZE_QEMU_STRIPPED_BIN)" ZE_TEST_BIN="$(ZE_QEMU_TEST_BIN)" ZE_QEMU_SKIP_SUITES="web" ZE_QEMU_PARALLEL="$(ZE_QEMU_PARALLEL)" bash scripts/evidence/qemu-all-tests.sh'

# Debug specific functional tests in the QEMU VM with verbose output.
#
# Unlike ze-qemu-all-test (all-or-nothing, non-verbose), this runs ONE arbitrary
# command in the VM and streams its output, so a failing .ci test can be re-run
# with -v to see the expect-vs-got diff. The command runs from /workspace with
# ZE_TEST_NO_BUILD=1 set (the runner reuses the cross-compiled binaries instead
# of recompiling on the slow 9p mount). Indices come from the ze-qemu-all-test
# summary line, e.g. "failed 2 [264, 310]".
#
# Usage:
#   make ze-qemu-debug RUN='bin/ze-test-linux-arm64 bgp parse 264 310 -v'
#   make ze-qemu-debug RUN='bin/ze-test-linux-arm64 bgp plugin 79 -v'
#   make ze-qemu-debug NOBUILD=1 RUN='bin/ze-test-linux-arm64 bgp parse 264 -v'
#
# By default it cross-compiles linux/$(QEMU_GOARCH) ze + ze-test from the current
# working tree (debug the code you are editing). NOBUILD=1 skips the compile and
# reuses whatever linux binaries already sit in bin/ (a prior cross-compile, or a
# restored set) so you can debug a specific build without rebuilding.
#
# The /tmp/ze-qemu-bin PATH shim mirrors qemu-all-tests.sh:68-76 and is REQUIRED
# for parity, not a convenience. ZE_BIN/ZE_TEST_BIN only tell the RUNNER which
# binaries to launch; a test whose config asks the daemon to spawn a helper by
# bare name -- `plugin { external ddos-detect { run "ze-test plugin-external
# ddos-detect" } }` in test/plugin/ddos-detect-external-warns.ci -- resolves that
# name through the daemon's PATH. Without the shim the helper is simply not found,
# the daemon emits nothing, and the runner reports the maximally unhelpful
# "timeout ... server likely failed to start or crashed". That test then FAILS
# here while PASSING in the real ze-qemu-needs-linux-test run, which is the worst
# possible signal from a tool whose entire job is reproducing a failure.
# RUN reaches the guard through the environment, not through the recipe text.
# `test -n "$(RUN)"` looked equivalent and was not: make pastes RUN in literally,
# so a RUN carrying its own double quotes -- `RUN='sh -c "go test ..."'`, the
# form needed to run anything but a bare ze-test invocation -- closed the guard's
# quotes early, test saw four arguments, and the target printed its usage line as
# though RUN had been empty. Silent, and it looks like the caller's mistake.
ze-qemu-debug: export ZE_QEMU_DEBUG_RUN = $(RUN)
ze-qemu-debug:
	@test -n "$$ZE_QEMU_DEBUG_RUN" || { echo 'usage: make ze-qemu-debug RUN='"'"'$(ZE_QEMU_TEST_BIN) bgp <suite> <N...> -v'"'"; exit 2; }
ifneq ($(NOBUILD),1)
	$(ze-qemu-crossbuild)
endif
	python3 scripts/evidence/qemu-run.py \
		--packages "make coreutils nftables iproute2 iputils-ping kmod iptables" \
		--timeout 1200 \
		--run 'mkdir -p /tmp/ze-qemu-bin && ln -sf /workspace/$(ZE_QEMU_BIN) /tmp/ze-qemu-bin/ze && ln -sf /workspace/$(ZE_QEMU_STRIPPED_BIN) /tmp/ze-qemu-bin/ze-stripped && ln -sf /workspace/$(ZE_QEMU_TEST_BIN) /tmp/ze-qemu-bin/ze-test && export PATH=/tmp/ze-qemu-bin:$$PATH && ZE_TEST_NO_BUILD=1 ZE_QEMU=1 ZE_BIN="$(ZE_QEMU_BIN)" ZE_TEST_BIN="$(ZE_QEMU_TEST_BIN)" $(RUN)'

# Boot a QEMU VM and keep it alive for interactive failure investigation.
#
# Runs the same VM setup as ze-qemu-debug (9p mount, packages, Go toolchain),
# writes the Go/workspace env to /etc/profile.d/ze.sh, then idles and prints the
# ssh command to use. SSH in repeatedly to run ONE .ci test at a time and inspect
# dmesg / nft / ip state between runs -- the way to diagnose a suite that crashes
# the VM (e.g. firewall) or a flake that only appears under specific kernel state.
# It blocks, so run it in the background; stop the process to power off the VM.
# Cross-compiles linux binaries first unless NOBUILD=1.
ze-qemu-shell:
ifneq ($(NOBUILD),1)
	$(ze-qemu-crossbuild)
endif
	python3 scripts/evidence/qemu-run.py \
		--packages "make coreutils nftables iproute2 iputils-ping kmod iptables" \
		--keep-alive

# Package list is DERIVED from `//go:build integration && linux` tags so a new
# linux-only package cannot be silently omitted (ai/rules/platform-linux.md).
# Exclusions: ldp runs in ze-qemu-ldp-frr-test (needs FRR in the VM).
# firewall/vpp is added explicitly: its fakeOps tests are linux-tagged but not
# integration-tagged, and still need a linux GOOS to compile.
ZE_QEMU_INTEGRATION_PKGS = $(shell grep -rl --include='*.go' '^//go:build integration && linux' internal/ cmd/ 2>/dev/null | sed 's|/[^/]*$$||' | sort -u | grep -v '^internal/plugins/ldp$$' | sed 's|^|./|')

# The installer initrd suite (`//go:build linux && ze_installer`) is EXECUTED
# here, not by `make ze-installer-unit-test`. That target can only run these for
# real on a Linux host; on darwin it degrades to a `go vet` compile-check
# because a cross-compiled linux test binary cannot exec (see mk/test-unit.mk).
# The VM is the Linux host that closes that gap, so the tag-guarded rescue /
# console / bootstrap logic is genuinely run on every platform's evidence path.
# Separate invocation because it needs its own tag set, which the
# integration-tagged packages do not share.
ze-qemu-integration-test:
	@echo "Running integration tests in QEMU Linux VM (requires qemu + internet for first run)..."
	python3 scripts/evidence/qemu-run.py \
		--packages "nftables iproute2 iputils-ping kmod iptables" \
		--run 'go test -tags integration -count=1 -timeout 120s $(ZE_QEMU_INTEGRATION_PKGS) ./internal/plugins/firewall/vpp/... && go test -tags "ze_core ze_installer" -count=1 -timeout 120s ./internal/install/...'

# AC-7: exercise the per-test netns launch mode (Fix B) end-to-end under QEMU on a
# real Linux kernel (macOS has no netns/nft). Cross-compiles ze/ze-stripped/ze-test,
# boots the Alpine VM, setcaps ze, and runs a host-safe firewall subset under
# ZE_TEST_NETNS (see scripts/evidence/netns_qemu.py), asserting the host firewall is
# untouched. The R-2 host-netns guard unit test (refuseHostNetnsFirewall) is covered
# separately by ze-qemu-integration-test (it is in the nft integration package).
ze-netns-qemu-test:
	# This is the functional-test DUT daemon: zetest pulls in the test-only
	# plugins (internal/test/plugins/all) the .ci suites need -- it is NOT a
	# production build (the real $(ZEBIN_ZE) has neither zetest nor ze_test). It builds
	# with the SAME tag set as the sibling QEMU targets above (257/279) and as
	# internal/test/runner TestBuildTags: ze_core zetest ze_distro ze_setup plus
	# $(ZE_FEATURES) (the default-on feature gates from feature-gates.txt). The
	# full feature set is REQUIRED, not a convenience: a test-only plugin's YANG
	# can import a feature-gated module (fakeddos/yang imports ze-ddos-detect-conf,
	# owned by ze_ddos), and a hand-picked minimal build then fails EVERY config
	# load with "no such module: ze-ddos-detect-conf". $(ZE_FEATURES) also carries
	# ze_ssh (for 004-cli-show's SSH path), so no feature needs listing by hand.
	$(ze-qemu-crossbuild)
	@echo "Running netns launch-mode evidence in QEMU Linux VM (host-safe firewall subset)..."
	python3 scripts/evidence/qemu-run.py \
		--packages "nftables iproute2 python3 libcap kmod iptables iputils-ping" \
		--timeout 1200 \
		--run 'QEMU_GOARCH=$(QEMU_GOARCH) ZE_QEMU_BIN="$(ZE_QEMU_BIN)" ZE_QEMU_STRIPPED_BIN="$(ZE_QEMU_STRIPPED_BIN)" ZE_QEMU_TEST_BIN="$(ZE_QEMU_TEST_BIN)" python3 scripts/evidence/netns_qemu.py'

# The PPPoE access concentrator's own functional suite (test/pppoe/). It needs
# two things together that no existing target supplies at once:
#
#   - the per-test netns launch mode, because each test asks the runner for a
#     veth PAIR (option=netns-link:peer=) and creating veth-bng in a developer's
#     real host namespace is the one thing that mode exists to prevent, and
#   - ze's runtime kernel, because handlePADR opens an AF_PPPOX/PX_PROTO_OE
#     socket BEFORE it sends PADS (internal/component/l2tp/pppoe/server.go) and
#     the stock Alpine kernel carries no CONFIG_PPPOE, so every PADR would die on
#     "kernel socket failed" and no test could pass.
#
# ze-netns-qemu-test gives the first on the stock kernel; this gives both. It
# reuses that target's in-VM driver and selects the suite with
# ZE_NETNS_QEMU_SUITES, so the setcap / uid-drop / host-safety machinery has one
# implementation rather than two.
ze-qemu-pppoe-test: ze-host
	$(ze-qemu-kernel-guard)
	$(ze-qemu-crossbuild)
	@echo "Running the PPPoE .ci suite under the netns launch mode in QEMU (runtime kernel)..."
	python3 scripts/evidence/qemu-run.py \
		--kernel $(ZE_QEMU_KERNEL) \
		--packages "nftables iproute2 python3 libcap kmod" \
		--timeout 1200 \
		--run 'QEMU_GOARCH=$(QEMU_GOARCH) ZE_NETNS_QEMU_SUITES=pppoe ZE_QEMU_BIN="$(ZE_QEMU_BIN)" ZE_QEMU_STRIPPED_BIN="$(ZE_QEMU_STRIPPED_BIN)" ZE_QEMU_TEST_BIN="$(ZE_QEMU_TEST_BIN)" python3 scripts/evidence/netns_qemu.py'

ze-qemu-ldp-frr-test:
	@echo "Running LDP interop test against FRR ldpd in QEMU Linux VM (installs frr)..."
	python3 scripts/evidence/qemu-run.py \
		--packages "frr iproute2 kmod" \
		--run 'go test -tags integration -count=1 -timeout 150s -run TestLDPInteropFRR ./internal/plugins/ldp/...'

ze-qemu-isis-frr-test:
	@echo "Running IS-IS interop test against FRR isisd in QEMU Linux VM (installs frr)..."
	python3 scripts/evidence/qemu-run.py \
		--packages "frr iproute2 kmod tcpdump" \
		--run 'go test -tags integration -count=1 -timeout 180s -run TestISISInteropFRR ./internal/plugins/isis/...'

ze-install-qemu-test:
	@echo "Running full installer-chain QEMU evidence (builds Go initrd + image, boots installer, verifies SSH login)..."
	@echo "Set ZE_INSTALL_KERNEL=/path/to/vmlinuz (IP_PNP_DHCP/VIRTIO_NET/VIRTIO_BLK/EXT4 built in); self-skips otherwise."
	@# macOS: point docker at colima's socket when DOCKER_HOST is unset, else the
	@# default context is down and the image build self-skips.
	@if [ "$$(uname)" = "Darwin" ] && [ -z "$$DOCKER_HOST" ] && [ -S "$$HOME/.colima/default/docker.sock" ]; then \
		DOCKER_HOST="unix://$$HOME/.colima/default/docker.sock" python3 scripts/evidence/effective-install-qemu.py; \
	else \
		python3 scripts/evidence/effective-install-qemu.py; \
	fi

ze-vpp-hugepages-qemu-test:
	@echo "Running VPP hugepage boot-reservation QEMU evidence (builds an appliance with image.hugepages, boots it, asserts show host kernel + show host memory over the Ze CLI)..."
	@echo "Self-skips when qemu / sshpass / e2fsprogs / go are absent. On Linux needs the kvm group (make ze-setup checks it as kvm-access)."
	python3 scripts/evidence/effective-vpp-hugepages-qemu.py

ze-install-iso-qemu-test:
	@echo "Running appliance ISO installer QEMU evidence (builds initrd + image + ISO, boots ISO, verifies SSH login)..."
	@echo "Set ZE_INSTALL_KERNEL=/path/to/vmlinuz (IP_PNP_DHCP/VIRTIO_NET/VIRTIO_BLK/EXT4/ISO9660/SR built in); self-skips otherwise."
	@if [ "$$(uname)" = "Darwin" ] && [ -z "$$DOCKER_HOST" ] && [ -S "$$HOME/.colima/default/docker.sock" ]; then \
		DOCKER_HOST="unix://$$HOME/.colima/default/docker.sock" python3 scripts/evidence/effective-install-iso-qemu.py; \
	else \
		python3 scripts/evidence/effective-install-iso-qemu.py; \
	fi

ze-install-scenarios-qemu-test:
	@echo "Running installer failure-path/pin/rescue QEMU evidence (R-6 fault, ze.mac pin, rescue console)..."
	@echo "Set ZE_INSTALL_KERNEL=/path/to/vmlinuz (IP_PNP_DHCP/VIRTIO_NET/VIRTIO_BLK/EXT4 built in); self-skips otherwise."
	@if [ "$$(uname)" = "Darwin" ] && [ -z "$$DOCKER_HOST" ] && [ -S "$$HOME/.colima/default/docker.sock" ]; then \
		DOCKER_HOST="unix://$$HOME/.colima/default/docker.sock" python3 scripts/evidence/effective-install-scenarios-qemu.py; \
	else \
		python3 scripts/evidence/effective-install-scenarios-qemu.py; \
	fi

ze-install-ventoy-qemu-test:
	@echo "Running installer Ventoy ISO-on-FAT QEMU evidence (ISO as a file on a FAT data disk)..."
	@echo "Needs grub-mkstandalone + xorriso + mtools; set ZE_INSTALL_KERNEL (ISO9660/VFAT/BLK_DEV_LOOP built in); self-skips otherwise."
	@if [ "$$(uname)" = "Darwin" ] && [ -z "$$DOCKER_HOST" ] && [ -S "$$HOME/.colima/default/docker.sock" ]; then \
		DOCKER_HOST="unix://$$HOME/.colima/default/docker.sock" python3 scripts/evidence/effective-install-ventoy-qemu.py; \
	else \
		python3 scripts/evidence/effective-install-ventoy-qemu.py; \
	fi

ze-qemu-l2tp-ppp-test: ze-host
	$(ze-qemu-kernel-guard)
	@echo "Running L2TP PPP/NCP peer test in QEMU Linux VM with gokrazy kernel..."
	python3 scripts/evidence/qemu-run.py \
		--kernel tmp/kernel/vmlinuz \
		--packages "xl2tpd ppp iproute2 iputils-ping nftables kmod python3" \
		--run 'python3 scripts/evidence/effective-l2tp-ppp.py' \
		--timeout 600

# QEMU sibling of test/pppoe-interop/ (the Docker lab). Runs Ze's PPPoE client
# against a real accel-ppp AC in two netns inside the runtime kernel (which has
# CONFIG_PPPOE built in). accel-ppp is installed from Alpine community.
# `make ze-kernel` builds and stages the kernel to tmp/kernel/vmlinuz, and
# rebuilds when runtime.config changes so CONFIG_PPPOE is picked up.
ze-qemu-pppoe-accel-test: ze-host
	$(ze-qemu-kernel-guard)
	@echo "Running PPPoE client-vs-accel-ppp test in QEMU Linux VM with runtime kernel..."
	python3 scripts/evidence/qemu-run.py \
		--kernel tmp/kernel/vmlinuz \
		--packages "accel-ppp ppp iproute2 iputils-ping kmod python3" \
		--run 'python3 scripts/evidence/effective-pppoe-accel.py' \
		--timeout 600

# VRRP interop: ze's VRRP vs a real keepalived on one L2 segment (spec-vrrp-6).
# Three netns (ze / keepalived / observer) bridged in a fourth, so the observer
# sees flooded multicast and can prove the VIP moves at L2, not just in a log.
#
# No --kernel here, unlike the l2tp/pppoe labs above: those need CONFIG_PPPOE /
# CONFIG_PPPOL2TP, which the stock Alpine kernel lacks. VRRP needs only macvlan,
# bridge and veth, and the stock Alpine 6.12.13-0-virt kernel has all three
# (probed 2026-07-15: dummy/macvlan-bridge-mode/bridge/veth/netns all create
# cleanly). Staying on the stock kernel keeps this target runnable without a
# ~30-minute `make ze-kernel` build first, matching the isis-frr/ldp-frr labs.
# keepalived comes from Alpine community (v2.3.1, built with VRRP + VRRP_VMAC).
ze-qemu-vrrp-keepalived-test:
	@echo "Running VRRP-vs-keepalived interop test in QEMU Linux VM (installs keepalived)..."
	python3 scripts/evidence/qemu-run.py \
		--packages "keepalived tcpdump iproute2 iputils-ping kmod python3" \
		--run 'python3 scripts/evidence/effective-vrrp-keepalived.py' \
		--timeout 900

# Exercises the traffic-usage eBPF TCX programs against ze's own runtime kernel
# (built from runtime.config with CONFIG_BPF_SYSCALL/CONFIG_BPF_JIT/CONFIG_VETH).
# Loads the pure-Go programs, attaches them to a veth pair, injects frames via
# AF_PACKET, asserts the maps, and scrapes /metrics. Validates the kernel
# additions end-to-end, not just on the stock Alpine kernel.
ze-qemu-traffic-usage-test: ze-host
	$(ze-qemu-kernel-guard)
	@echo "Running traffic-usage eBPF TCX test in QEMU Linux VM with the runtime kernel..."
	python3 scripts/evidence/qemu-run.py \
		--kernel tmp/kernel/vmlinuz \
		--packages "iproute2 kmod" \
		--run 'go test -tags integration -count=1 -timeout 180s -run "TestProgram_|TestAttachTCX_CountsTraffic|TestMetricsScrape" ./internal/plugins/trafficusage/...' \
		--timeout 600
