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
.PHONY: ze-netns-test ze-netns-qemu-test
.PHONY: ze-release-check ze-deployment-vpp-test ze-deployment-vpp-iface-test ze-deployment-l2tp-test ze-deployment-l2tp-ppp-test
.PHONY: ze-deployment-l2tp-ppp-docker-test ze-deployment-gokrazy-l2tp-ppp-test
.PHONY: ze-deployment-pppoe-accel-docker-test
.PHONY: ze-docker-evidence ze-deployment-preflight
.PHONY: ze-qemu-integration-test ze-qemu-l2tp-ppp-test ze-qemu-pppoe-accel-test ze-qemu-ldp-frr-test ze-qemu-isis-frr-test ze-qemu-vrrp-keepalived-test ze-qemu-traffic-usage-test ze-vpp-hugepages-qemu-test ze-install-qemu-test ze-install-iso-qemu-test ze-install-scenarios-qemu-test ze-install-ventoy-qemu-test ze-qemu-all-test ze-qemu-needs-linux-test

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

ze-stress-test: bin/ze
	@echo "Running stress tests with ze-test peer injector (requires root + netns)..."
	@sudo ZE_BINARY=$(CURDIR)/bin/ze VERBOSE=$(VERBOSE) SESSION_TIMEOUT=$(SESSION_TIMEOUT) \
		python3 test/stress/run.py $(STRESS_SCENARIO)

ze-stress-bird-test:
	@echo "Running BIRD baseline stress test (requires root + bird2 + netns)..."
	@sudo VERBOSE=$(VERBOSE) SESSION_TIMEOUT=$(SESSION_TIMEOUT) \
		python3 test/stress/run.py 04-bulk-ipv4-bird

ze-stress-profile: bin/ze
	@echo "Running 1M profile stress test (requires root + netns)..."
	@sudo ZE_BINARY=$(CURDIR)/bin/ze ZE_PPROF=1 VERBOSE=$(VERBOSE) \
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
# rebuilds with `go build -o bin/ze` (internal/test/runner/runner.go:227), and
# file capabilities are an xattr on the inode, so a rebuild silently DISCARDS the
# setcap applied two lines above -- the daemon then runs without CAP_NET_ADMIN and
# every test in all four suites fails. It also removes the need for `go` on sudo's
# secure_path, which it is not on a default Debian/Ubuntu install. The target
# already declares bin/ze, bin/ze-stripped and bin/ze-test as prerequisites, so
# the binaries are current before setcap runs.
ZE_NETNS_SUITES ?= firewall policy ospf ospfv3
ZE_NETNS_CAPS   ?= cap_net_admin,cap_net_raw,cap_net_bind_service+ep
ze-netns-test: bin/ze bin/ze-stripped bin/ze-test
	@[ "$$(uname)" = "Linux" ] || { echo "ze-netns-test requires Linux (netns/nft); on macOS use ze-netns-qemu-test"; exit 1; }
	@command -v setcap >/dev/null 2>&1 || { echo "error: setcap not found (install libcap2-bin / libcap)"; exit 1; }
	@command -v nft    >/dev/null 2>&1 || { echo "error: nft not found (install nftables)"; exit 1; }
	@echo "Granting ambient caps to bin/ze + bin/ze-stripped ($(ZE_NETNS_CAPS))..."
	sudo setcap $(ZE_NETNS_CAPS) bin/ze
	sudo setcap $(ZE_NETNS_CAPS) bin/ze-stripped
	@before=$$(sudo nft list tables 2>/dev/null | sort); failed=0; \
	for suite in $(ZE_NETNS_SUITES); do \
		printf "\n=== netns suite: %s ===\n" "$$suite"; \
		sudo env PATH="$$PATH" ZE_TEST_NO_BUILD=1 ZE_TEST_NETNS=1 ZE_TEST_UID=$$(id -u) ZE_TEST_GID=$$(id -g) \
			bin/ze-test $$suite --all -p 1 || failed=$$((failed + 1)); \
	done; \
	after=$$(sudo nft list tables 2>/dev/null | sort); \
	sudo setcap -r bin/ze bin/ze-stripped 2>/dev/null || true; \
	if [ "$$before" != "$$after" ]; then \
		printf "\033[31mHOST-SAFETY FAILURE: host nft tables changed during netns run\033[0m\n"; \
		printf -- "--- before ---\n%s\n--- after ---\n%s\n" "$$before" "$$after"; \
		failed=$$((failed + 1)); \
	else \
		printf "\033[32mhost nft tables unchanged (host-safe)\033[0m\n"; \
	fi; \
	[ $$failed -eq 0 ] || { printf "\033[31mnetns run FAILED (%d issue(s))\033[0m\n" "$$failed"; exit 1; }; \
	printf "\033[32mnetns run OK\033[0m\n"

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
# binaries match the VM. ZE_QEMU_SKIP_SUITES (default: web,firewall) lets you
# drop suites: web needs agent-browser; firewall crashes the Alpine QEMU kernel
# on nft set-element-timeout operations.
# Cross-compiled binaries go to bin/ze-linux-<arch> so bin/ze stays the
# host-native binary. No need to run `make ze test` after QEMU testing.
QEMU_GOARCH := $(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')
ZE_QEMU_BIN := bin/ze-linux-$(QEMU_GOARCH)
ZE_QEMU_STRIPPED_BIN := bin/ze-stripped-linux-$(QEMU_GOARCH)
ZE_QEMU_TEST_BIN := bin/ze-test-linux-$(QEMU_GOARCH)
ZE_QEMU_SKIP_SUITES ?= web,firewall
ZE_QEMU_PARALLEL ?= 4

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

ze-qemu-all-test:
	@echo "Cross-compiling linux/$(QEMU_GOARCH) ze + ze-stripped + ze-test on host (CGO off)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_core zetest ze_distro ze_setup $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZE_QEMU_BIN) ./cmd/ze
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_core $(ZE_TAGS)' -o $(ZE_QEMU_STRIPPED_BIN) ./cmd/ze
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_test $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZE_QEMU_TEST_BIN) ./cmd/ze
	@echo "Running full test suite in QEMU Linux VM (host-compiled binaries; no in-VM ze/ze-test compile)..."
	python3 scripts/evidence/qemu-run.py \
		--packages "make coreutils nftables iproute2 iputils-ping kmod iptables" \
		--timeout 3600 \
		--run 'ZE_BIN="$(ZE_QEMU_BIN)" ZE_STRIPPED_BIN="$(ZE_QEMU_STRIPPED_BIN)" ZE_TEST_BIN="$(ZE_QEMU_TEST_BIN)" ZE_QEMU_SKIP_SUITES="$(ZE_QEMU_SKIP_SUITES)" ZE_QEMU_PARALLEL="$(ZE_QEMU_PARALLEL)" bash scripts/evidence/qemu-all-tests.sh'

# Tight loop: run ONLY the Linux-only functional tests (option=needs-linux) in a
# single QEMU Linux VM. These tests SKIP natively on darwin (so `make ze-verify`
# stays green and fast) and are validated here instead. ZE_QEMU_LINUX_ONLY=1
# flips the runner to skip every test that is NOT marked needs-linux, so the VM
# spends its time only on the Linux-only surface -- one VM boot, all the
# Linux-only tests, never one VM per test. See ai/rules/qemu-testing.md.
#
# web is skipped (browser-driven, not a kernel-feature surface); every other
# suite runs so a needs-linux test in any of them (plugin, firewall, l2tp, ...)
# is exercised.
ze-qemu-needs-linux-test:
	@echo "Cross-compiling linux/$(QEMU_GOARCH) ze + ze-stripped + ze-test on host (CGO off)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_core zetest ze_distro ze_setup $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZE_QEMU_BIN) ./cmd/ze
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_core $(ZE_TAGS)' -o $(ZE_QEMU_STRIPPED_BIN) ./cmd/ze
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_test $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZE_QEMU_TEST_BIN) ./cmd/ze
	@echo "Running ONLY option=needs-linux tests in QEMU Linux VM (ZE_QEMU_LINUX_ONLY=1)..."
	python3 scripts/evidence/qemu-run.py \
		--packages "make coreutils nftables iproute2 iputils-ping kmod iptables" \
		--timeout 1800 \
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
ze-qemu-debug:
	@test -n "$(RUN)" || { echo 'usage: make ze-qemu-debug RUN='"'"'$(ZE_QEMU_TEST_BIN) bgp <suite> <N...> -v'"'"; exit 2; }
ifneq ($(NOBUILD),1)
	@echo "Cross-compiling linux/$(QEMU_GOARCH) ze + ze-test on host (CGO off)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_core zetest ze_distro $(ZE_TAGS)' -o $(ZE_QEMU_BIN) ./cmd/ze
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_test $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZE_QEMU_TEST_BIN) ./cmd/ze
endif
	python3 scripts/evidence/qemu-run.py \
		--packages "make coreutils nftables iproute2 iputils-ping kmod iptables" \
		--timeout 1200 \
		--run 'ZE_TEST_NO_BUILD=1 ZE_QEMU=1 ZE_BIN="$(ZE_QEMU_BIN)" $(RUN)'

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
	@echo "Cross-compiling linux/$(QEMU_GOARCH) ze + ze-test on host (CGO off)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_core zetest ze_distro $(ZE_TAGS)' -o $(ZE_QEMU_BIN) ./cmd/ze
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_test $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZE_QEMU_TEST_BIN) ./cmd/ze
endif
	python3 scripts/evidence/qemu-run.py \
		--packages "make coreutils nftables iproute2 iputils-ping kmod iptables" \
		--keep-alive

# Package list is DERIVED from `//go:build integration && linux` tags so a new
# linux-only package cannot be silently omitted (ai/rules/qemu-testing.md).
# Exclusions: ldp runs in ze-qemu-ldp-frr-test (needs FRR in the VM).
# firewall/vpp is added explicitly: its fakeOps tests are linux-tagged but not
# integration-tagged, and still need a linux GOOS to compile.
ZE_QEMU_INTEGRATION_PKGS = $(shell grep -rl --include='*.go' '^//go:build integration && linux' internal/ cmd/ 2>/dev/null | sed 's|/[^/]*$$||' | sort -u | grep -v '^internal/plugins/ldp$$' | sed 's|^|./|')

ze-qemu-integration-test:
	@echo "Running integration tests in QEMU Linux VM (requires qemu + internet for first run)..."
	python3 scripts/evidence/qemu-run.py \
		--packages "nftables iproute2 iputils-ping kmod iptables" \
		--run 'go test -tags integration -count=1 -timeout 120s $(ZE_QEMU_INTEGRATION_PKGS) ./internal/plugins/firewall/vpp/...'

# AC-7: exercise the per-test netns launch mode (Fix B) end-to-end under QEMU on a
# real Linux kernel (macOS has no netns/nft). Cross-compiles ze/ze-stripped/ze-test,
# boots the Alpine VM, setcaps ze, and runs a host-safe firewall subset under
# ZE_TEST_NETNS (see scripts/evidence/netns_qemu.py), asserting the host firewall is
# untouched. The R-2 host-netns guard unit test (refuseHostNetnsFirewall) is covered
# separately by ze-qemu-integration-test (it is in the nft integration package).
ze-netns-qemu-test:
	@echo "Cross-compiling linux/$(QEMU_GOARCH) ze + ze-stripped + ze-test on host (CGO off)..."
	@mkdir -p bin
	# This is the functional-test DUT daemon: zetest pulls in the test-only
	# plugins (internal/test/plugins/all) the .ci suites need -- it is NOT a
	# production build (the real bin/ze has neither zetest nor ze_test). ze_ssh
	# is added for 004-cli-show: it compiles in the SSH component's `system
	# authentication` / `environment ssh` config schema + the SSH CLI server
	# (//go:build ze_ssh in all_ze_ssh.go). In the real bin/ze, ze_ssh is a
	# default feature-gate, so 004's SSH path is present there too; `ze init` is
	# already in ze_core (no ze_setup needed). Harmless for the other firewall
	# suites -- the SSH server only starts when a config declares environment ssh.
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_core zetest ze_distro ze_ssh $(ZE_TAGS)' -o $(ZE_QEMU_BIN) ./cmd/ze
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_core $(ZE_TAGS)' -o $(ZE_QEMU_STRIPPED_BIN) ./cmd/ze
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_test $(ZE_FEATURES) $(ZE_TAGS)' -o $(ZE_QEMU_TEST_BIN) ./cmd/ze
	@echo "Running netns launch-mode evidence in QEMU Linux VM (host-safe firewall subset)..."
	python3 scripts/evidence/qemu-run.py \
		--packages "nftables iproute2 python3 libcap kmod iptables iputils-ping" \
		--timeout 1200 \
		--run 'QEMU_GOARCH=$(QEMU_GOARCH) python3 scripts/evidence/netns_qemu.py'

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

ze-qemu-l2tp-ppp-test:
	@test -f tmp/kernel/vmlinuz || { echo "error: tmp/kernel/vmlinuz not found (run: make ze-kernel GOKRAZY_ARCH=arm64)"; exit 1; }
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
ze-qemu-pppoe-accel-test:
	@test -f tmp/kernel/vmlinuz || { echo "error: tmp/kernel/vmlinuz not found (run: make ze-kernel GOKRAZY_ARCH=arm64)"; exit 1; }
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
ze-qemu-traffic-usage-test:
	@test -f tmp/kernel/vmlinuz || { echo "error: tmp/kernel/vmlinuz not found (run: make ze-kernel GOKRAZY_ARCH=arm64)"; exit 1; }
	@echo "Running traffic-usage eBPF TCX test in QEMU Linux VM with the runtime kernel..."
	python3 scripts/evidence/qemu-run.py \
		--kernel tmp/kernel/vmlinuz \
		--packages "iproute2 kmod" \
		--run 'go test -tags integration -count=1 -timeout 180s -run "TestProgram_|TestAttachTCX_CountsTraffic|TestMetricsScrape" ./internal/plugins/trafficusage/...' \
		--timeout 600
