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
# Produces build/Image (the installer kernel), build/config (resolved .config),
# build/kernel.version (provenance), and build/.variant (arch-profile-version-builder
# consumed by `ze install ... iso`).

ARCH ?= arm64
PROFILE ?= qemu
BUILDER ?= qemu
JOBS ?=
OUT := ../../build/kernel
BUILDER_DIR := ../kernel-builder
RUN := $(BUILDER_DIR)/run.py
VARIANT := $(OUT)/.variant
PROFILE_DEPS = $(wildcard *.config) $(wildcard *.require)
COMMON_DEPS := $(wildcard $(BUILDER_DIR)/common/*.config) $(wildcard $(BUILDER_DIR)/common/*.require)

.PHONY: all clean FORCE

all: $(OUT)/Image

FORCE:

$(OUT)/Image: $(RUN) $(BUILDER_DIR)/build.py $(BUILDER_DIR)/ksource.py $(BUILDER_DIR)/qemu-build.py $(BUILDER_DIR)/Dockerfile kernel.config $(PROFILE_DEPS) $(COMMON_DEPS) FORCE
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
	@echo "Built: $(OUT)/Image (profile=$(PROFILE), arch=$(ARCH), builder=$(BUILDER))"

clean:
	rm -rf $(OUT)
