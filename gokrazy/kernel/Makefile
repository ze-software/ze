# Build the Ze runtime kernel package artifacts for gokrazy.
#
# Produces build/vmlinuz, build/config, build/lib/modules/, and arm64 DTBs via
# the shared driver tools/kernel-builder/run.py (docker or qemu). This Makefile
# only selects the runtime config fragments and build environment; run.py owns
# the kernel version (internal/appliance/kernel.version), the arch->platform
# map, fragment resolution, and the container/VM invocation.

ARCH ?= amd64
PROFILE := runtime
BUILDER ?= docker
JOBS ?=
REPO_ROOT := $(shell git -C $(dir $(lastword $(MAKEFILE_LIST))) rev-parse --show-toplevel)
BUILDER_DIR := $(REPO_ROOT)/tools/kernel-builder
RUN := $(BUILDER_DIR)/run.py
# Build output lives under the repo scratch tree (tmp/ is gitignored, so the
# ~400MB artifacts never pollute git status). OUT is absolute for the test
# assertions; OUT_REL is the repo-relative form run.py requires.
OUT     := $(REPO_ROOT)/tmp/kernel/build
OUT_REL := tmp/kernel/build
PATCH_DEPS := $(wildcard patches/*.patch)
COMMON_DEPS := $(wildcard $(BUILDER_DIR)/common/*.config) $(wildcard $(BUILDER_DIR)/common/*.require)

.PHONY: all clean

all: $(OUT)/vmlinuz

$(OUT)/vmlinuz: kernel.config kernel.require runtime.config runtime.require patches/series $(PATCH_DEPS) $(RUN) $(BUILDER_DIR)/build.py $(BUILDER_DIR)/ksource.py $(BUILDER_DIR)/qemu-build.py $(BUILDER_DIR)/Dockerfile $(COMMON_DEPS)
	mkdir -p $(OUT)
	python3 $(RUN) \
		--target runtime \
		--arch $(ARCH) \
		--profile $(PROFILE) \
		--builder $(BUILDER) \
		--src-dir gokrazy/kernel \
		--out-dir $(OUT_REL) \
		--builder-dir tools/kernel-builder \
		--common-dir tools/kernel-builder/common \
		--patches-dir gokrazy/kernel/patches \
		--modules yes \
		$(if $(JOBS),--jobs $(JOBS),)
	@test -f $(OUT)/vmlinuz
	@test -d $(OUT)/lib/modules

clean:
	rm -rf $(OUT)
