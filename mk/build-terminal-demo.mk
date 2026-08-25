# Reproducible website terminal demonstrations -- the render drivers MOVED to `le`.
#
# The four whole-manifest drivers now live in
# scripts/le/application/build_terminal_demo.py, which carries the reasons this
# file used to hold in comments. Each target below is a shim: it forwards to
# `le` so that every existing caller keeps working -- release preparation, CI,
# and the references across docs and plan/.
#
# AGENTS AND HUMANS: use `le` directly. It is the source of truth.
#
#   ./le build-terminal-demo                                  every check
#   ./le build-terminal-demo --list                           what each gate is for
#   ./le build-terminal-demo --write                          re-render every demo
#   ./le build-terminal-demo ze-terminal-demo-check-all       one gate
#
# The forwarding targets carry no logic. A change to what a gate DOES belongs in
# the Python module; a change here can only break the forwarding.
#
# WHAT STAYED, AND WHY. Four targets still hold their recipes here, because a
# Gate is one argv run without a shell and these are not:
#
#   ze-terminal-demo-image-build       a `command -v docker` guard that prints
#                                      its own error, then a docker build
#   ze-terminal-demo-binaries-build    a mkdir, then two cross builds whose
#                                      -ldflags is the Makefile's ZE_LDFLAGS
#   ze-terminal-demo-render            a `test -n "$(DEMO)"` guard over a
#                                      caller-supplied demo id
#   ze-terminal-demo-release-render-all, ze-release-assets-update,
#   ze-release-assets-check            aliases with no recipe at all; the
#                                      prerequisite IS the target
#
# The prerequisite edges below are what `make -j` enforces, and they cannot move
# into Python: a render needs the image and the binaries first.

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

# The host needs docker, python3, and ffmpeg. ffmpeg is read only by the ONE
# browser demo, whose video render.py rescales and whose poster it resizes; a
# terminal demo records an asciicast and needs none of it. Until 2026-08-24 a
# `ze-terminal-demo-tools-install` target installed ffmpeg beside VHS and ttyd.
# The recorder opens its own PTY now, so that script was deleted with VHS and
# ffmpeg is the operator's to install.
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
	@$(CURDIR)/le build-terminal-demo ze-terminal-demo-render-all

ze-terminal-demo-validation-check-all: ze-terminal-demo-image-build ze-terminal-demo-binaries-build
	@$(CURDIR)/le build-terminal-demo ze-terminal-demo-validation-check-all

ze-terminal-demo-check-all:
	@$(CURDIR)/le build-terminal-demo ze-terminal-demo-check-all

# Release preparation calls this aggregate target before tagging. It rebuilds
# the released binaries and every website recording from the checked-in tapes.
ze-terminal-demo-release-render-all: ze-terminal-demo-render-all

ze-terminal-demo-release-check-all:
	@$(CURDIR)/le build-terminal-demo ze-terminal-demo-release-check-all

ze-release-assets-update: ze-terminal-demo-release-render-all

ze-release-assets-check: ze-terminal-demo-release-check-all
