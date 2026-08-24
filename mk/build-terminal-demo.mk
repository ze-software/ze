# Reproducible website terminal demonstrations.
#
# Every render uses the pinned renderer container on Docker's native Linux
# architecture, so it also works on Apple Silicon without requiring emulation.
# The container has no external network and only receives the capabilities
# required by the Linux network-namespace traceroute lab.
#
# The host needs docker, python3, and ffmpeg. ffmpeg is read only by the ONE
# browser demo, whose video render.py rescales and whose poster it resizes; a
# terminal demo records an asciicast and needs none of it. Until 2026-08-24 a
# `ze-terminal-demo-tools-install` target installed ffmpeg beside VHS and ttyd.
# The recorder opens its own PTY now, so that script was deleted with VHS and
# ffmpeg is the operator's to install.

TERMINAL_DEMO_GOARCH ?= $(shell $(GO) env GOARCH)
TERMINAL_DEMO_IMAGE := ze-terminal-demo-render-all:debian-13-playwright-1.55.0-firacode-6.002-notosans-2.015-liberation-2.1.5
TERMINAL_DEMO_RELEASE ?= $(ZE_VERSION)
TERMINAL_DEMO_BIN_DIR := $(CURDIR)/tmp/terminal-demos/bin
TERMINAL_DEMO_OUTPUT ?= $(CURDIR)/../gh-pages/assets/demos
TERMINAL_DEMO_TAGS := ze_core ze_distro $(ZE_FEATURES) $(ZE_TAGS)

.PHONY: ze-terminal-demo-render ze-terminal-demo-render-all ze-terminal-demo-check-all
.PHONY: ze-terminal-demo-image-build ze-terminal-demo-binaries-build
.PHONY: ze-terminal-demo-validation-check-all
.PHONY: ze-terminal-demo-release-render-all ze-terminal-demo-release-check-all
.PHONY: ze-release-assets-update ze-release-assets-check

ze-terminal-demo-image-build:
	@command -v docker >/dev/null || { echo "error: docker is required to render terminal demos"; exit 1; }
	docker build \
		-f demos/terminal/Dockerfile \
		-t $(TERMINAL_DEMO_IMAGE) \
		demos/terminal

ze-terminal-demo-binaries-build:
	@mkdir -p $(TERMINAL_DEMO_BIN_DIR)
	GOOS=linux GOARCH=$(TERMINAL_DEMO_GOARCH) CGO_ENABLED=0 \
		$(GO) build -tags '$(TERMINAL_DEMO_TAGS)' -ldflags "$(ZE_LDFLAGS)" \
		-o $(TERMINAL_DEMO_BIN_DIR)/ze ./cmd/ze
	GOOS=linux GOARCH=$(TERMINAL_DEMO_GOARCH) CGO_ENABLED=0 \
		$(GO) build -tags ze_test \
		-o $(TERMINAL_DEMO_BIN_DIR)/ze-test ./cmd/ze

ze-terminal-demo-render: ze-terminal-demo-image-build ze-terminal-demo-binaries-build
	@test -n "$(DEMO)" || { echo "error: pass DEMO=<id> (see demos/terminal/manifest.json)"; exit 1; }
	@ZE_TERMINAL_DEMO_OUTPUT="$(TERMINAL_DEMO_OUTPUT)" python3 demos/terminal/render.py --demo "$(DEMO)" --release "$(TERMINAL_DEMO_RELEASE)"

ze-terminal-demo-render-all: ze-terminal-demo-image-build ze-terminal-demo-binaries-build
	@ZE_TERMINAL_DEMO_OUTPUT="$(TERMINAL_DEMO_OUTPUT)" python3 demos/terminal/render.py --all --release "$(TERMINAL_DEMO_RELEASE)"

ze-terminal-demo-validation-check-all: ze-terminal-demo-image-build ze-terminal-demo-binaries-build
	@ZE_TERMINAL_DEMO_OUTPUT="$(TERMINAL_DEMO_OUTPUT)" python3 demos/terminal/render.py --all --validate

ze-terminal-demo-check-all:
	@ZE_TERMINAL_DEMO_OUTPUT="$(TERMINAL_DEMO_OUTPUT)" python3 demos/terminal/render.py --all --check

# Release preparation calls this aggregate target before tagging. It rebuilds
# the released binaries and every website recording from the checked-in tapes.
ze-terminal-demo-release-render-all: ze-terminal-demo-render-all

ze-terminal-demo-release-check-all:
	@ZE_TERMINAL_DEMO_OUTPUT="$(TERMINAL_DEMO_OUTPUT)" python3 demos/terminal/render.py --all --release "$(TERMINAL_DEMO_RELEASE)" --check

ze-release-assets-update: ze-terminal-demo-release-render-all

ze-release-assets-check: ze-terminal-demo-release-check-all
