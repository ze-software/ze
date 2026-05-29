# Build the ze installer kernel in a container.
#
#   make                         # build build/Image for the host arch
#   make ARCH=x86_64             # cross-target (needs an x86_64 docker arch)
#   make LINUX_VERSION=6.12.9    # pin a different kernel
#   make clean
#
# The build runs natively for the docker daemon's architecture, so run it on a
# host whose docker arch matches ARCH (arm64 on Apple Silicon / colima).
# Produces build/Image (the installer kernel) and build/config (resolved .config).
# Point the QEMU install test at it: ZE_INSTALL_KERNEL=$(pwd)/build/Image.

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
