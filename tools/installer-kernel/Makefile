# Build the ze installer kernel.
#
#   make                                  # qemu builder, qemu profile, arm64 (default)
#   make BUILDER=docker                   # docker builder (faster cross-arch via Rosetta 2)
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
LINUX_VERSION ?= 7.0.11
PROFILE ?= qemu
BUILDER ?= qemu
JOBS ?=
IMG := ze-installer-kernel-builder
OUT := build

PROFILE_CONFIG := $(PROFILE).config

.PHONY: all clean

all: $(OUT)/Image

PROFILE_DEPS = $(PROFILE_CONFIG)
ifeq ($(PROFILE),hardware-kms)
PROFILE_DEPS = hardware.config hardware-kms.config
endif

ifeq ($(BUILDER),docker)

$(OUT)/Image: Dockerfile build.sh kernel.config $(PROFILE_DEPS)
	@if [ ! -f "$(PROFILE_CONFIG)" ]; then \
		echo "error: unknown PROFILE=$(PROFILE) (expected qemu, hardware, or hardware-kms)" >&2; \
		exit 1; \
	fi
	docker build -t $(IMG) .
	mkdir -p $(OUT)
	docker run --rm \
		-e LINUX_VERSION=$(LINUX_VERSION) \
		-e ARCH=$(ARCH) \
		-e PROFILE=$(PROFILE) \
		$(if $(JOBS),-e JOBS=$(JOBS),) \
		-v "$(CURDIR):/src:ro" \
		-v "$(CURDIR)/$(OUT):/out" \
		$(IMG) sh /src/build.sh
	@echo "Built: $(OUT)/Image (profile=$(PROFILE), arch=$(ARCH), builder=docker)"

else

$(OUT)/Image: qemu-build.py build.sh kernel.config $(PROFILE_DEPS)
	@if [ ! -f "$(PROFILE_CONFIG)" ]; then \
		echo "error: unknown PROFILE=$(PROFILE) (expected qemu, hardware, or hardware-kms)" >&2; \
		exit 1; \
	fi
	mkdir -p $(OUT)
	python3 qemu-build.py \
		--arch $(ARCH) \
		--profile $(PROFILE) \
		--version $(LINUX_VERSION) \
		$(if $(JOBS),--jobs $(JOBS),)
	@echo "Built: $(OUT)/Image (profile=$(PROFILE), arch=$(ARCH), builder=qemu)"

endif

clean:
	rm -rf $(OUT)
