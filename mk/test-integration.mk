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

.PHONY: ze-interop-test ze-ipsec-interop-test
.PHONY: ze-stress-test ze-stress-bird-test ze-stress-profile
.PHONY: ze-live-test ze-live-rpki-test
.PHONY: ze-integration-test ze-integration-iface-test ze-integration-fib-test ze-integration-firewall-test ze-integration-traffic-test
.PHONY: ze-release-check ze-deployment-vpp-test ze-deployment-l2tp-test ze-deployment-l2tp-ppp-test
.PHONY: ze-deployment-l2tp-ppp-docker-test ze-deployment-gokrazy-l2tp-ppp-test
.PHONY: ze-docker-evidence ze-deployment-preflight
.PHONY: ze-qemu-integration-test ze-qemu-l2tp-ppp-test ze-qemu-ldp-frr-test ze-install-qemu-test

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

ze-qemu-l2tp-ppp-test:
	@test -f tmp/kernel/vmlinuz || { echo "error: tmp/kernel/vmlinuz not found (run: make ze-kernel GOKRAZY_ARCH=arm64)"; exit 1; }
	@echo "Running L2TP PPP/NCP peer test in QEMU Linux VM with gokrazy kernel..."
	python3 scripts/evidence/qemu-run.py \
		--kernel tmp/kernel/vmlinuz \
		--packages "xl2tpd ppp iproute2 iputils-ping nftables kmod python3" \
		--run 'python3 scripts/evidence/effective-l2tp-ppp.py' \
		--timeout 600
