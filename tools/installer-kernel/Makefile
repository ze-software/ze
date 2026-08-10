# Build the ze installer kernel via the shared driver tools/kernel-builder/run.py.
#
#   make                                  # qemu builder, qemu profile, arm64 (default)
#   make BUILDER=docker                   # docker builder (faster when platform emulation is available)
#   make PROFILE=hardware                 # real hardware, headless (EFI, NIC drivers, NVMe)
#   make PROFILE=hardware-kms             # real hardware + Intel i915 KMS monitor output
#   make PROFILE=hardware ARCH=amd64      # hardware profile, x86_64
#   make ARCH=amd64                       # qemu profile, x86_64
#   make clean
#
# BUILDER selects the build environment (qemu: Alpine VM via QEMU; docker:
# Debian container). run.py owns the kernel version
# (internal/appliance/kernel.version), the arch->platform map, fragment
# resolution, and the container/VM invocation.
#
# PROFILE selects which driver set to merge on top of kernel.config. A profile
# is valid when both $(PROFILE).config and $(PROFILE).require exist; a config
# may declare one base fragment with "# ze-base: <profile>" and pull in shared
# fragments with "# ze-include: <name>".
#   qemu         - virtio only (small, fast build)
#   hardware     - virtio + EFI + real NIC/disk drivers (for bare metal / PXE)
#   hardware-kms - hardware + Intel i915 KMS (for monitor output on Alder Lake-N+)
# Produces build/kernel/Image (the installer kernel), build/kernel/config
# (resolved .config), build/kernel/kernel.version (provenance),
# build/kernel/.variant (arch-profile-version-builder consumed by
# `ze install ... iso`), and build/kernel/.request (the requested
# arch-profile-builder, read only by this Makefile).
#
# The build is incremental: a repeated make with the same ARCH, PROFILE and
# BUILDER and no newer source file does nothing.

ARCH ?= arm64
PROFILE ?= qemu
BUILDER ?= qemu
JOBS ?=
OUT := ../../build/kernel
BUILDER_DIR := ../kernel-builder
RUN := $(BUILDER_DIR)/run.py
VARIANT := $(OUT)/.variant
VERSION_FILE := ../../internal/appliance/kernel.version
REQUEST := $(OUT)/.request
# Every module run.py can import, found rather than listed: a hand-written list
# stops triggering a rebuild the day somebody adds a module to that directory.
BUILDER_SCRIPTS := $(wildcard $(BUILDER_DIR)/*.py)
PROFILE_DEPS = $(wildcard *.config) $(wildcard *.require)
COMMON_DEPS := $(wildcard $(BUILDER_DIR)/common/*.config) $(wildcard $(BUILDER_DIR)/common/*.require)

.PHONY: all clean FORCE

all: $(OUT)/Image

FORCE:

# ARCH, PROFILE and BUILDER select the build, and no FILE prerequisite can
# express a variable change. The recipe records the requested triple in
# $(REQUEST); the comparison below reads that record at parse time and adds the
# phony FORCE to the prerequisite list of $(OUT)/Image only when the request has
# changed. A repeated request adds nothing, so the target keeps its file
# prerequisites alone and `make -q` exits 0 in the steady state.
#
# The comparison reads a file and writes none, so `make -q` still changes
# nothing. It also never compares an mtime, so the answer does not depend on
# timestamp granularity. That matters here: GNU Make 3.81, the make Apple ships,
# resolves mtimes to one second. A stamp file used as an ordinary prerequisite
# therefore misses a profile switch made inside the same second as the build it
# follows.
#
# $(REQUEST) is a build-system stamp that only this Makefile reads. $(VARIANT)
# is the record of what was BUILT and is read by installerKernelBuildMatches in
# internal/appliance/cmd_iso.go. Two facts, so two files.
#
# `ze appliance kernel` also writes $(OUT)/Image, and it does not run this
# Makefile. It deletes $(REQUEST) on every installer-target run, so the next
# make here rebuilds rather than trust a record it did not write. The producer is
# invalidateInstallerKernelRequest in internal/appliance/cmd_kernel.go.
REQUESTED := $(ARCH)-$(PROFILE)-$(BUILDER)
RECORDED := $(shell cat $(REQUEST) 2>/dev/null)
ifneq ($(REQUESTED),$(RECORDED))
$(OUT)/Image: FORCE
endif

# $(MAKEFILE_LIST) is this file. The recipe below names the profile, the arch,
# the builder and the run.py argument list, so an edit to it changes the build
# as much as an edit to a config fragment does. The unconditional FORCE that
# used to sit on this line covered that; a file prerequisite list has to say it.
$(OUT)/Image: $(MAKEFILE_LIST) $(BUILDER_SCRIPTS) $(BUILDER_DIR)/Dockerfile kernel.config $(PROFILE_DEPS) $(COMMON_DEPS) $(VERSION_FILE)
	mkdir -p $(OUT)
	python3 $(RUN) \
		--target installer \
		--arch $(ARCH) \
		--profile $(PROFILE) \
		--builder $(BUILDER) \
		--src-dir tools/installer-kernel \
		--out-dir build/kernel \
		--builder-dir tools/kernel-builder \
		--common-dir tools/kernel-builder/common \
		--modules no \
		$(if $(JOBS),--jobs $(JOBS),)
	@ver=$$(sed -n 's/^version=//p' $(OUT)/kernel.version); echo "$(ARCH)-$(PROFILE)-$$ver-$(BUILDER)" > $(VARIANT)
	@echo "$(REQUESTED)" > $(REQUEST)
	@echo "Built: $(OUT)/Image (profile=$(PROFILE), arch=$(ARCH), builder=$(BUILDER))"

clean:
	rm -rf $(OUT)
