# Build the ze installer kernel.
#
#   make                                  # qemu builder, qemu profile, arm64 (default)
#   make BUILDER=docker                   # docker builder (faster when platform emulation is available)
#   make PROFILE=hardware                 # real hardware, headless (EFI, NIC drivers, NVMe)
#   make PROFILE=hardware-kms             # real hardware + Intel i915 KMS monitor output
#   make PROFILE=hardware ARCH=amd64      # hardware profile, x86_64
#   make ARCH=amd64                       # qemu profile, x86_64
#   make LINUX_VERSION=6.12.9             # pin a different kernel
#   make clean
#
# BUILDER selects the build environment:
#   qemu   (default) - Alpine VM via QEMU; requires: qemu, python3, curl
#   docker           - Debian container via Docker; requires: docker
#
# PROFILE selects which driver set to merge on top of kernel.config:
#   qemu         - virtio only (small, fast build)
#   hardware     - virtio + EFI + real NIC/disk drivers (for bare metal / PXE)
#   hardware-kms - hardware + Intel i915 KMS (for monitor output on Alder Lake-N+)
#
# Produces build/Image (the installer kernel) and build/config (resolved
# .config). The caller chooses the architecture via ARCH before invoking
# `ze install appliance iso`, or passes an explicit `--kernel` path when
# keeping multiple kernels side by side.

ARCH ?= arm64
# Single source of truth for the kernel version: internal/appliance/kernel.version
# (also embedded into `ze appliance kernel`). Bump the kernel by editing that file.
LINUX_VERSION ?= $(shell cat $(dir $(lastword $(MAKEFILE_LIST)))../../internal/appliance/kernel.version)
PROFILE ?= qemu
BUILDER ?= qemu
JOBS ?=
IMG := ze-installer-kernel-builder
OUT := build
BUILDER_DIR := ../kernel-builder
PROFILE_CONFIG := $(PROFILE).config
VARIANT := $(OUT)/.variant

ifeq ($(ARCH),amd64)
DOCKER_PLATFORM := linux/amd64
else ifeq ($(ARCH),x86_64)
DOCKER_PLATFORM := linux/amd64
else ifeq ($(ARCH),arm64)
DOCKER_PLATFORM := linux/arm64
else
$(error unsupported ARCH=$(ARCH) (expected amd64 or arm64))
endif

.PHONY: all clean FORCE

all: $(OUT)/Image

PROFILE_DEPS = $(PROFILE_CONFIG)
ifeq ($(PROFILE),hardware-kms)
PROFILE_DEPS = hardware.config hardware-kms.config
endif

$(VARIANT): FORCE
	@mkdir -p $(OUT)
	@current="$(ARCH)-$(PROFILE)-$(LINUX_VERSION)-$(BUILDER)"; \
	if [ ! -f $(VARIANT) ] || [ "$$(cat $(VARIANT))" != "$$current" ]; then \
		echo "$$current" > $(VARIANT); \
	fi

FORCE:

ifeq ($(BUILDER),docker)

$(OUT)/Image: $(BUILDER_DIR)/Dockerfile $(BUILDER_DIR)/build.sh kernel.config $(PROFILE_DEPS) $(VARIANT)
	@if [ ! -f "$(PROFILE_CONFIG)" ]; then \
		echo "error: unknown PROFILE=$(PROFILE) (expected qemu, hardware, or hardware-kms)" >&2; \
		exit 1; \
	fi
	docker build --platform $(DOCKER_PLATFORM) -t $(IMG) $(BUILDER_DIR)
	mkdir -p $(OUT)
	docker run --rm --platform $(DOCKER_PLATFORM) \
		-e LINUX_VERSION=$(LINUX_VERSION) \
		-e ARCH=$(ARCH) \
		-e PROFILE=$(PROFILE) \
		-e SRC_DIR=/src \
		-e OUT_DIR=/out \
		$(if $(JOBS),-e JOBS=$(JOBS),) \
		-v "$(abspath $(BUILDER_DIR)):/builder:ro" \
		-v "$(CURDIR):/src:ro" \
		-v "$(CURDIR)/$(OUT):/out" \
		$(IMG) sh /builder/build.sh
	@echo "Built: $(OUT)/Image (profile=$(PROFILE), arch=$(ARCH), builder=docker)"

else ifeq ($(BUILDER),qemu)

$(OUT)/Image: $(BUILDER_DIR)/qemu-build.py $(BUILDER_DIR)/build.sh kernel.config $(PROFILE_DEPS) $(VARIANT)
	@if [ ! -f "$(PROFILE_CONFIG)" ]; then \
		echo "error: unknown PROFILE=$(PROFILE) (expected qemu, hardware, or hardware-kms)" >&2; \
		exit 1; \
	fi
	mkdir -p $(OUT)
	python3 $(BUILDER_DIR)/qemu-build.py \
		--arch $(ARCH) \
		--profile $(PROFILE) \
		--version $(LINUX_VERSION) \
		--src-dir tools/installer-kernel \
		--out-dir tools/installer-kernel/$(OUT) \
		--builder-dir tools/kernel-builder \
		$(if $(JOBS),--jobs $(JOBS),)
	@echo "Built: $(OUT)/Image (profile=$(PROFILE), arch=$(ARCH), builder=qemu)"

else
$(error unsupported BUILDER=$(BUILDER) (expected docker or qemu))
endif

clean:
	rm -rf $(OUT)
