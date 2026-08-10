# Build the Ze runtime kernel package artifacts for gokrazy.
#
# Produces build/vmlinuz, build/config, build/lib/modules/, arm64 DTBs, and
# build/.request (the requested arch-profile-builder, read only by this
# Makefile), via the shared driver tools/kernel-builder/run.py (docker or qemu).
# This Makefile only selects the runtime config fragments and build environment;
# run.py owns the kernel version (internal/appliance/kernel.version), the
# arch->platform map, fragment resolution, and the container/VM invocation.
#
# The build is incremental: a repeated make with the same ARCH and BUILDER and no
# newer source file does nothing.
#
# `ze appliance kernel --target runtime` also writes build/vmlinuz, and it does
# not run this Makefile. It deletes build/.request first, so the next make here
# rebuilds rather than trust a record it did not write. The producer is
# invalidateKernelBuildRecords in internal/appliance/cmd_kernel.go.

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
REQUEST := $(OUT)/.request
PATCH_DEPS := $(wildcard patches/*.patch)
COMMON_DEPS := $(wildcard $(BUILDER_DIR)/common/*.config) $(wildcard $(BUILDER_DIR)/common/*.require)
# Every module run.py can import, found rather than listed: a hand-written list
# stops triggering a rebuild the day somebody adds a module to that directory.
BUILDER_SCRIPTS := $(wildcard $(BUILDER_DIR)/*.py)

.PHONY: all clean FORCE

all: $(OUT)/vmlinuz

FORCE:

# ARCH and BUILDER select the build, and no FILE prerequisite can express a
# variable change: an amd64 vmlinuz and an arm64 one live at the same path. The
# recipe records the requested triple in $(REQUEST); the comparison below reads
# that record at parse time and adds the phony FORCE to the prerequisite list of
# $(OUT)/vmlinuz only when the request has changed. A repeated request adds
# nothing, so the target keeps its file prerequisites alone and `make -q` exits 0
# in the steady state.
#
# The comparison reads a file and writes none, so `make -q` still changes
# nothing. It also never compares an mtime, so the answer does not depend on
# timestamp granularity. That matters here: GNU Make 3.81, the make Apple ships,
# resolves mtimes to one second, so a stamp file used as an ordinary prerequisite
# misses an arch switch made inside the same second as the build it follows.
#
# Same mechanism, same file name, as tools/installer-kernel/Makefile.
REQUESTED := $(ARCH)-$(PROFILE)-$(BUILDER)
RECORDED := $(shell cat $(REQUEST) 2>/dev/null)
ifneq ($(REQUESTED),$(RECORDED))
$(OUT)/vmlinuz: FORCE
endif

# $(MAKEFILE_LIST) is this file. The recipe below names the profile, the arch,
# the builder and the run.py argument list, so an edit to it changes the build as
# much as an edit to a config fragment does.
$(OUT)/vmlinuz: $(MAKEFILE_LIST) kernel.config kernel.require runtime.config runtime.require patches/series $(PATCH_DEPS) $(BUILDER_SCRIPTS) $(BUILDER_DIR)/Dockerfile $(COMMON_DEPS)
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
	@echo "$(REQUESTED)" > $(REQUEST)

clean:
	rm -rf $(OUT)
