# Build the ze installer kernel in a container.
#
#   make                              # qemu profile, arm64 (default)
#   make PROFILE=hardware             # real hardware (EFI, NIC drivers, NVMe)
#   make PROFILE=hardware ARCH=amd64  # hardware profile, x86_64
#   make ARCH=amd64                   # qemu profile, x86_64
#   make LINUX_VERSION=6.12.9         # pin a different kernel
#   make clean
#
# PROFILE selects which driver set to merge on top of kernel.config:
#   qemu     - virtio only (small, fast build)
#   hardware - virtio + EFI + real NIC/disk drivers (for bare metal / PXE)
#
# Produces build/Image (the installer kernel) and build/config (resolved
# .config). The caller chooses the architecture via ARCH before invoking
# `ze install appliance iso`, or passes an explicit `--kernel` path when
# keeping multiple kernels side by side.

ARCH ?= arm64
LINUX_VERSION ?= 7.0.11
PROFILE ?= qemu
JOBS ?=
IMG := ze-installer-kernel-builder
OUT := build

PROFILE_CONFIG := $(PROFILE).config

.PHONY: all clean

all: $(OUT)/Image

$(OUT)/Image: Dockerfile build.sh kernel.config $(PROFILE_CONFIG)
	@if [ ! -f "$(PROFILE_CONFIG)" ]; then \
		echo "error: unknown PROFILE=$(PROFILE) (expected qemu or hardware)" >&2; \
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
	@echo "Built: $(OUT)/Image (profile=$(PROFILE), arch=$(ARCH))"

clean:
	rm -rf $(OUT)
