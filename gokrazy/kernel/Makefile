# Build the Ze runtime kernel package artifacts for gokrazy.
#
# Produces build/vmlinuz, build/config, build/lib/modules/, and arm64 DTBs.
# The shared builder lives in ../../tools/kernel-builder; this Makefile only
# selects the runtime config fragments and build environment.

ARCH ?= amd64
KVER ?= 7.0.11
PROFILE := runtime
BUILDER ?= docker
JOBS ?=
IMG := ze-runtime-kernel-builder
REPO_ROOT := $(shell git -C $(dir $(lastword $(MAKEFILE_LIST))) rev-parse --show-toplevel)
BUILDER_DIR := $(REPO_ROOT)/tools/kernel-builder
# Build output lives under the repo scratch tree (tmp/ is gitignored, so the
# ~400MB artifacts never pollute git status). OUT is absolute because make
# targets run with CWD=gokrazy/kernel and Docker needs a real bind-mount path;
# OUT_REL is the repo-relative form qemu-build.py requires (it rejects abs/..).
OUT     := $(REPO_ROOT)/tmp/kernel/build
OUT_REL := tmp/kernel/build
PATCH_DEPS := $(wildcard patches/*.patch)

ifeq ($(ARCH),amd64)
DOCKER_PLATFORM := linux/amd64
else ifeq ($(ARCH),x86_64)
DOCKER_PLATFORM := linux/amd64
else ifeq ($(ARCH),arm64)
DOCKER_PLATFORM := linux/arm64
else
$(error unsupported ARCH=$(ARCH) (expected amd64 or arm64))
endif

.PHONY: all clean

all: $(OUT)/vmlinuz

ifeq ($(BUILDER),docker)
$(OUT)/vmlinuz: kernel.config runtime.config patches/series $(PATCH_DEPS) $(BUILDER_DIR)/build.sh $(BUILDER_DIR)/Dockerfile
else ifeq ($(BUILDER),qemu)
$(OUT)/vmlinuz: kernel.config runtime.config patches/series $(PATCH_DEPS) $(BUILDER_DIR)/build.sh $(BUILDER_DIR)/qemu-build.py
else
$(OUT)/vmlinuz: kernel.config runtime.config patches/series $(PATCH_DEPS) $(BUILDER_DIR)/build.sh
endif
ifeq ($(BUILDER),docker)
	docker build --platform $(DOCKER_PLATFORM) -t $(IMG) $(BUILDER_DIR)
	mkdir -p $(OUT)
	docker run --rm --platform $(DOCKER_PLATFORM) \
		-e LINUX_VERSION=$(KVER) \
		-e ARCH=$(ARCH) \
		-e PROFILE=$(PROFILE) \
		-e SRC_DIR=/src \
		-e OUT_DIR=/out \
		-e MODULES=yes \
		-e PATCHES_DIR=/src/patches \
		$(if $(JOBS),-e JOBS=$(JOBS),) \
		-v "$(BUILDER_DIR):/builder:ro" \
		-v "$(CURDIR):/src:ro" \
		-v "$(OUT):/out" \
		$(IMG) sh /builder/build.sh
else ifeq ($(BUILDER),qemu)
	mkdir -p $(OUT)
	python3 $(BUILDER_DIR)/qemu-build.py \
		--arch $(ARCH) \
		--profile $(PROFILE) \
		--version $(KVER) \
		--src-dir gokrazy/kernel \
		--out-dir $(OUT_REL) \
		--builder-dir tools/kernel-builder \
		--modules yes \
		--patches-dir gokrazy/kernel/patches \
		$(if $(JOBS),--jobs $(JOBS),)
else
	$(error unsupported BUILDER=$(BUILDER) (expected docker or qemu))
endif
	@test -f $(OUT)/vmlinuz
	@test -d $(OUT)/lib/modules

clean:
	rm -rf $(OUT)
