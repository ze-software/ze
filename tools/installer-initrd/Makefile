# Build system for ze installer initrd.
#
# Produces a cpio-format initrd containing a minimal busybox environment
# and the ze installer init script. The initrd is designed to be PXE-booted
# alongside a Linux kernel.
#
# Prerequisites:
#   - busybox-static (or busybox with static linking)
#   - cpio
#   - gzip
#
# Usage:
#   make                    # build initrd.img.gz
#   make BUSYBOX=/path/to/busybox  # custom busybox path
#   make clean              # remove build artifacts
#   make test               # run unit tests

BUSYBOX ?= $(shell command -v busybox 2>/dev/null)
OUTDIR := build
ROOTFS := $(OUTDIR)/rootfs
INITRD := $(OUTDIR)/initrd.img.gz

.PHONY: all clean test check-busybox

all: $(INITRD)

check-busybox:
	@if [ -z "$(BUSYBOX)" ]; then \
		echo "ERROR: busybox not found. Install busybox-static or set BUSYBOX=/path/to/busybox"; \
		exit 1; \
	fi
	@if ! file "$(BUSYBOX)" | grep -q 'statically linked\|static-pie'; then \
		echo "WARNING: $(BUSYBOX) may not be statically linked. The initrd may not boot."; \
	fi

$(INITRD): check-busybox init
	rm -rf $(ROOTFS)
	mkdir -p $(ROOTFS)/bin $(ROOTFS)/sbin $(ROOTFS)/proc $(ROOTFS)/sys
	mkdir -p $(ROOTFS)/dev $(ROOTFS)/mnt/perm $(ROOTFS)/tmp
	cp "$(BUSYBOX)" $(ROOTFS)/bin/busybox
	chmod 755 $(ROOTFS)/bin/busybox
	for cmd in sh cat echo mount umount mkdir sleep wget dd sync \
		reboot blockdev basename; do \
		ln -sf busybox $(ROOTFS)/bin/$$cmd; \
	done
	cp init $(ROOTFS)/init
	chmod 755 $(ROOTFS)/init
	cd $(ROOTFS) && find . | cpio -o -H newc 2>/dev/null | gzip -9 > ../initrd.img.gz
	@echo "Built: $(INITRD)"
	@echo "Size: $$(du -h $(INITRD) | cut -f1)"

clean:
	rm -rf $(OUTDIR)

test:
	@echo "Running installer initrd tests..."
	@sh test/test-cmdline-parse.sh
	@sh test/test-disk-detect.sh
	@sh test/test-download.sh
	@echo "All tests passed."
