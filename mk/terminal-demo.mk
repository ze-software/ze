# Reproducible website terminal demonstrations.
#
# Every render uses the pinned VHS container on Docker's native Linux
# architecture, so it also works on Apple Silicon without requiring emulation.
# The container has no external network and only receives the capabilities
# required by the Linux network-namespace traceroute lab.

TERMINAL_DEMO_GOARCH ?= $(shell $(GO) env GOARCH)
TERMINAL_DEMO_IMAGE := ze-terminal-demos:vhs-0.11.0-playwright-1.55.0
TERMINAL_DEMO_RELEASE ?= $(ZE_VERSION)
TERMINAL_DEMO_BIN_DIR := $(CURDIR)/tmp/terminal-demos/bin
TERMINAL_DEMO_OUTPUT ?= $(CURDIR)/../gh-pages/assets/demos
TERMINAL_DEMO_TAGS := ze_core ze_distro $(ZE_FEATURES) $(ZE_TAGS)

.PHONY: ze-terminal-demo ze-terminal-demos ze-terminal-demos-check
.PHONY: ze-terminal-demo-image ze-terminal-demo-binaries
.PHONY: ze-terminal-demos-validate
.PHONY: ze-terminal-demos-release ze-terminal-demos-release-check
.PHONY: ze-terminal-demo-tools
.PHONY: ze-release-assets ze-release-assets-check

ze-terminal-demo-tools:
	@demos/terminal/install-vhs.sh

ze-terminal-demo-image:
	@command -v docker >/dev/null || { echo "error: docker is required to render terminal demos"; exit 1; }
	docker build \
		-f demos/terminal/Dockerfile \
		-t $(TERMINAL_DEMO_IMAGE) \
		demos/terminal

ze-terminal-demo-binaries:
	@mkdir -p $(TERMINAL_DEMO_BIN_DIR)
	GOOS=linux GOARCH=$(TERMINAL_DEMO_GOARCH) CGO_ENABLED=0 \
		$(GO) build -tags '$(TERMINAL_DEMO_TAGS)' -ldflags "$(ZE_LDFLAGS)" \
		-o $(TERMINAL_DEMO_BIN_DIR)/ze ./cmd/ze
	GOOS=linux GOARCH=$(TERMINAL_DEMO_GOARCH) CGO_ENABLED=0 \
		$(GO) build -tags ze_test \
		-o $(TERMINAL_DEMO_BIN_DIR)/ze-test ./cmd/ze

ze-terminal-demo: ze-terminal-demo-image ze-terminal-demo-binaries
	@test -n "$(DEMO)" || { echo "error: pass DEMO=<id> (see demos/terminal/manifest.json)"; exit 1; }
	@ZE_TERMINAL_DEMO_OUTPUT="$(TERMINAL_DEMO_OUTPUT)" python3 demos/terminal/render.py --demo "$(DEMO)" --release "$(TERMINAL_DEMO_RELEASE)"

ze-terminal-demos: ze-terminal-demo-image ze-terminal-demo-binaries
	@ZE_TERMINAL_DEMO_OUTPUT="$(TERMINAL_DEMO_OUTPUT)" python3 demos/terminal/render.py --all --release "$(TERMINAL_DEMO_RELEASE)"

ze-terminal-demos-validate: ze-terminal-demo-image ze-terminal-demo-binaries
	@ZE_TERMINAL_DEMO_OUTPUT="$(TERMINAL_DEMO_OUTPUT)" python3 demos/terminal/render.py --all --validate

ze-terminal-demos-check:
	@ZE_TERMINAL_DEMO_OUTPUT="$(TERMINAL_DEMO_OUTPUT)" python3 demos/terminal/render.py --all --check

# Release preparation calls this aggregate target before tagging. It rebuilds
# the released binaries and every website recording from the checked-in tapes.
ze-terminal-demos-release: ze-terminal-demos

ze-terminal-demos-release-check:
	@ZE_TERMINAL_DEMO_OUTPUT="$(TERMINAL_DEMO_OUTPUT)" python3 demos/terminal/render.py --all --release "$(TERMINAL_DEMO_RELEASE)" --check

ze-release-assets: ze-terminal-demos-release

ze-release-assets-check: ze-terminal-demos-release-check
