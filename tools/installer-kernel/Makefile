# Build the ze installer kernel in a container.
#
#   make                         # build build/Image for the selected ARCH (default: arm64)
#   make ARCH=amd64             # cross-target x86_64 kernel
#   make LINUX_VERSION=6.12.9   # pin a different kernel
#   make clean
#
# Produces build/Image (the installer kernel) and build/config (resolved
# .config). The caller chooses the architecture via ARCH before invoking
# `ze install appliance iso`, or passes an explicit `--kernel` path when
# keeping multiple kernels side by side.

ARCH ?= arm64
LINUX_VERSION ?= 6.6.58
JOBS ?=
IMG := ze-installer-kernel-builder
OUT := build

.PHONY: all clean

all: $(OUT)/Image

$(OUT)/Image: Dockerfile build.sh kernel.config
	docker build -t $(IMG) .
	mkdir -p $(OUT)
	docker run --rm \
		-e LINUX_VERSION=$(LINUX_VERSION) \
		-e ARCH=$(ARCH) \
		$(if $(JOBS),-e JOBS=$(JOBS),) \
		-v "$(CURDIR):/src:ro" \
		-v "$(CURDIR)/$(OUT):/out" \
		$(IMG) sh /src/build.sh
	@echo "Built: $(OUT)/Image"

clean:
	rm -rf $(OUT)
