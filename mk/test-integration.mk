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

.PHONY: ze-interop-test ze-ipsec-interop-test
.PHONY: ze-stress-test ze-stress-bird-test ze-stress-profile
.PHONY: ze-live-test ze-live-rpki-test
.PHONY: ze-integration-test ze-integration-iface-test ze-integration-fib-test ze-integration-firewall-test ze-integration-traffic-test
.PHONY: ze-release-check ze-deployment-vpp-test ze-deployment-l2tp-test ze-deployment-l2tp-ppp-test
.PHONY: ze-deployment-l2tp-ppp-docker-test ze-deployment-gokrazy-l2tp-ppp-test
.PHONY: ze-docker-evidence ze-deployment-preflight
.PHONY: ze-qemu-integration-test ze-qemu-l2tp-ppp-test ze-qemu-ldp-frr-test ze-install-qemu-test ze-install-iso-qemu-test ze-qemu-all-test

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

# ─── Live ───────────────────────────────────────────────────────────────────

ze-live-test: ze-live-rpki-test

ze-live-rpki-test:
	@echo "Running RPKI live test (requires Docker + internet)..."
	$(GO) test -v -tags live -timeout 180s -count=1 ./internal/component/bgp/plugins/rpki/... -run TestLive

# ─── Integration (network namespace) ────────────────────────────────────────

ze-integration-iface-test:
	@echo "Running iface integration tests (requires CAP_NET_ADMIN)..."
	$(GO) test -tags integration -count=1 -race -timeout 120s ./internal/component/iface/...

ze-integration-fib-test:
	@echo "Running FIB kernel integration tests (requires CAP_NET_ADMIN)..."
	$(GO) test -tags integration -count=1 -race -timeout 120s ./internal/plugins/fib/kernel/...

ze-integration-firewall-test:
	@echo "Running firewall nft integration tests (requires CAP_NET_ADMIN)..."
	$(GO) test -tags integration -count=1 -race -timeout 120s ./internal/plugins/firewall/nft/...

ze-integration-traffic-test:
	@echo "Running traffic-control netlink integration tests (requires CAP_NET_ADMIN)..."
	$(GO) test -tags integration -count=1 -race -timeout 120s ./internal/plugins/traffic/netlink/...

ze-integration-test: ze-integration-iface-test ze-integration-fib-test ze-integration-firewall-test ze-integration-traffic-test

# ─── Deployment evidence ────────────────────────────────────────────────────

ze-deployment-vpp-test:
	@echo "Running real VPP daemon deployment test (requires Docker + privileged container)..."
	python3 scripts/evidence/effective-vpp.py

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
# binaries match the VM. ZE_QEMU_SKIP_SUITES (default: web, which needs
# agent-browser) lets you drop suites that cannot run headless in the VM.
# Cross-compiled binaries go to bin/ze-linux-<arch> so bin/ze stays the
# host-native binary. No need to run `make ze test` after QEMU testing.
QEMU_GOARCH := $(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')
ZE_QEMU_BIN := bin/ze-linux-$(QEMU_GOARCH)
ZE_QEMU_STRIPPED_BIN := bin/ze-stripped-linux-$(QEMU_GOARCH)
ZE_QEMU_TEST_BIN := bin/ze-test-linux-$(QEMU_GOARCH)
ZE_QEMU_SKIP_SUITES ?= web
ZE_QEMU_PARALLEL ?= 4

# The QEMU ze build deliberately OMITS ZE_LDFLAGS (the version/buildDate -X
# stamps). The .ci suites reuse this binary via ZE_TEST_NO_BUILD=1, and the
# native parse runner (cmd/ze-test buildZe) builds ze WITHOUT version ldflags,
# so test/parse/cli-show-version.ci asserts `ze show version` prints "ze dev".
# Stamping a real version here makes that test fail spuriously. Keep it unstamped.

ze-qemu-all-test:
	@echo "Cross-compiling linux/$(QEMU_GOARCH) ze + ze-stripped + ze-test on host (CGO off)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'zetest $(ZE_TAGS)' -o $(ZE_QEMU_BIN) ./cmd/ze
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'ze_stripped $(ZE_TAGS)' -o $(ZE_QEMU_STRIPPED_BIN) ./cmd/ze
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -o $(ZE_QEMU_TEST_BIN) ./cmd/ze-test
	@echo "Running full test suite in QEMU Linux VM (host-compiled binaries; no in-VM ze/ze-test compile)..."
	python3 scripts/evidence/qemu-run.py \
		--packages "make coreutils nftables iproute2 iputils-ping kmod iptables" \
		--timeout 3600 \
		--run 'ZE_BIN="$(ZE_QEMU_BIN)" ZE_STRIPPED_BIN="$(ZE_QEMU_STRIPPED_BIN)" ZE_TEST_BIN="$(ZE_QEMU_TEST_BIN)" ZE_QEMU_SKIP_SUITES="$(ZE_QEMU_SKIP_SUITES)" ZE_QEMU_PARALLEL="$(ZE_QEMU_PARALLEL)" bash scripts/evidence/qemu-all-tests.sh'

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
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'zetest $(ZE_TAGS)' -o $(ZE_QEMU_BIN) ./cmd/ze
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -o $(ZE_QEMU_TEST_BIN) ./cmd/ze-test
endif
	python3 scripts/evidence/qemu-run.py \
		--packages "make coreutils nftables iproute2 iputils-ping kmod iptables" \
		--timeout 1200 \
		--run 'ZE_TEST_NO_BUILD=1 ZE_BIN="$(ZE_QEMU_BIN)" $(RUN)'

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
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -tags 'zetest $(ZE_TAGS)' -o $(ZE_QEMU_BIN) ./cmd/ze
	CGO_ENABLED=0 GOOS=linux GOARCH=$(QEMU_GOARCH) $(GO) build -o $(ZE_QEMU_TEST_BIN) ./cmd/ze-test
endif
	python3 scripts/evidence/qemu-run.py \
		--packages "make coreutils nftables iproute2 iputils-ping kmod iptables" \
		--keep-alive

ze-qemu-integration-test:
	@echo "Running integration tests in QEMU Linux VM (requires qemu + internet for first run)..."
	python3 scripts/evidence/qemu-run.py \
		--packages "nftables iproute2 iputils-ping kmod iptables" \
		--run 'go test -tags integration -count=1 -timeout 120s ./cmd/ze/doctor ./internal/component/iface/... ./internal/component/config/system/... ./internal/core/routewatch/... ./internal/plugins/fib/kernel/... ./internal/plugins/firewall/nft/... ./internal/plugins/firewall/vpp/... ./internal/plugins/traffic/netlink/... ./internal/plugins/tftpserver/... ./internal/plugins/dhcpserver/...'

ze-qemu-ldp-frr-test:
	@echo "Running LDP interop test against FRR ldpd in QEMU Linux VM (installs frr)..."
	python3 scripts/evidence/qemu-run.py \
		--packages "frr iproute2 kmod" \
		--run 'go test -tags integration -count=1 -timeout 150s -run TestLDPInteropFRR ./internal/component/ldp/...'

ze-install-qemu-test:
	@echo "Running full installer-chain QEMU evidence (builds initrd + image, boots installer, verifies SSH login)..."
	@echo "Set ZE_INSTALL_KERNEL=/path/to/vmlinuz (IP_PNP_DHCP/VIRTIO_NET/VIRTIO_BLK/EXT4 built in); self-skips otherwise."
	@# macOS: point docker at colima's socket when DOCKER_HOST is unset, else the
	@# default context is down and busybox extraction (and the test) self-skips.
	@if [ "$$(uname)" = "Darwin" ] && [ -z "$$DOCKER_HOST" ] && [ -S "$$HOME/.colima/default/docker.sock" ]; then \
		DOCKER_HOST="unix://$$HOME/.colima/default/docker.sock" python3 scripts/evidence/effective-install-qemu.py; \
	else \
		python3 scripts/evidence/effective-install-qemu.py; \
	fi

ze-install-iso-qemu-test:
	@echo "Running appliance ISO installer QEMU evidence (builds initrd + image + ISO, boots ISO, verifies SSH login)..."
	@echo "Set ZE_INSTALL_KERNEL=/path/to/vmlinuz (IP_PNP_DHCP/VIRTIO_NET/VIRTIO_BLK/EXT4/ISO9660/SR built in); self-skips otherwise."
	@if [ "$$(uname)" = "Darwin" ] && [ -z "$$DOCKER_HOST" ] && [ -S "$$HOME/.colima/default/docker.sock" ]; then \
		DOCKER_HOST="unix://$$HOME/.colima/default/docker.sock" python3 scripts/evidence/effective-install-iso-qemu.py; \
	else \
		python3 scripts/evidence/effective-install-iso-qemu.py; \
	fi

ze-qemu-l2tp-ppp-test:
	@test -f tmp/kernel/vmlinuz || { echo "error: tmp/kernel/vmlinuz not found (run: make ze-kernel GOKRAZY_ARCH=arm64)"; exit 1; }
	@echo "Running L2TP PPP/NCP peer test in QEMU Linux VM with gokrazy kernel..."
	python3 scripts/evidence/qemu-run.py \
		--kernel tmp/kernel/vmlinuz \
		--packages "xl2tpd ppp iproute2 iputils-ping nftables kmod python3" \
		--run 'python3 scripts/evidence/effective-l2tp-ppp.py' \
		--timeout 600
