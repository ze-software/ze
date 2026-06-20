# Gokrazy VM appliance and custom kernel builds
#
# Builds a bootable VM image with Ze baked in. The default architecture is
# amd64 for N100-class mini PCs and common hypervisors; set GOKRAZY_ARCH=arm64
# for a native Apple Silicon QEMU image.
# Everything is vendored: gok tool deps in vendor/github.com/gokrazy/,
# dependency pins in gokrazy/ze/builddir/*/go.mod.
# After cloning, run `make ze-gokrazy-deps` once to populate the Go module cache
# for the gokrazy system packages (kernel, init). After that, builds work offline.
#
# Requires: e2fsprogs (brew install e2fsprogs)
#           qemu (brew install qemu) -- for ze-gokrazy-run only
#
# The image contains:
#   - Linux kernel + gokrazy init (process supervisor, DHCP, NTP, web UI)
#   - Ze binary with all internal plugins compiled in
#   - Config template + SSH credentials + TLS cert in /perm/ze/database.zefs
#   - First boot: template merged with interface discovery into active config
#
# Ze web UI:      https://localhost:28080/ (ze login required)
# Gokrazy mgmt:   https://localhost:28080/gokrazy/ (ze login required)
# Ze SSH CLI:     ssh -p 2222 <user>@localhost
#
# Usage:
#   make ze-gokrazy-deps                    -- one-time: download gokrazy system packages
#   make ze-gokrazy USER=admin PASS=secret CERTNAME=router.local  -- first build (cert cached per name)
#   make ze-gokrazy USER=admin PASS=secret  -- first build (no cert caching without CERTNAME)
#   make ze-gokrazy GOKRAZY_ARCH=arm64 USER=admin PASS=secret  -- native Apple Silicon VM image
#   make ze-gokrazy                         -- rebuild: reuse existing credentials
#   make ze-gokrazy ZEFS=/path/to/db.zefs   -- rebuild: use external database
#   make ze-gokrazy-run                     -- boot image in QEMU

.PHONY: ze-gokrazy ze-gokrazy-deps ze-gokrazy-run ze-kernel ze-kernel-clean

GOKRAZY_INSTANCE   := ze
GOKRAZY_DIR        := gokrazy
GOKRAZY_ARCH       ?= amd64
GOKRAZY_IMG        := tmp/gokrazy/ze.img
GOKRAZY_IMG_SIZE   := 2147483648
GOKRAZY_PERM_OFF   := 1157627904
GOKRAZY_PERM_BLK   := 966639
GOKRAZY_PERM_4K    := 241660
GOKRAZY_PERM_SKIP  := 282624
E2FS               := /opt/homebrew/Cellar/e2fsprogs/1.47.4/sbin
GOKRAZY_QEMU_ACCEL ?= tcg
GOKRAZY_QEMU_AARCH64_BIOS ?= /opt/homebrew/share/qemu/edk2-aarch64-code.fd
GOKRAZY_QEMU_AARCH64_CPU ?= max

bin/gok:
	@echo "Building ze-gok from vendored source..."
	@mkdir -p bin
	go build -mod=vendor -o bin/gok ./cmd/ze-gok

GOMODCACHE_LOCAL := $(CURDIR)/$(GOKRAZY_DIR)/modcache

ze-gokrazy-deps: bin/gok
	@echo "Downloading gokrazy dependencies into $(GOKRAZY_DIR)/modcache/..."
	@for d in $$(find $(GOKRAZY_DIR)/$(GOKRAZY_INSTANCE)/builddir -name go.mod -exec dirname {} \;); do \
		echo "  $$d"; \
		(cd "$$d" && GOMODCACHE=$(GOMODCACHE_LOCAL) go mod download all) || exit 1; \
	done
	@echo "Done. Builds now work offline."

GOKRAZY_ZEFS     := tmp/gokrazy/init/database.zefs
GOKRAZY_CERT_DIR := tmp/gokrazy/certs/$(CERTNAME)
GOKRAZY_TEMPLATE ?= gokrazy/ze/ze.conf

ze-gokrazy: ze bin/gok
	@test -f $(E2FS)/mkfs.ext4 || { echo "error: e2fsprogs not found (brew install e2fsprogs)"; exit 1; }
	@mkdir -p tmp/gokrazy/init
	@if [ -n "$(ZEFS)" ]; then \
		echo "--- Using external database: $(ZEFS) ---"; \
		cp "$(ZEFS)" $(GOKRAZY_ZEFS); \
	elif [ -n "$(USER)" ] && [ -n "$(PASS)" ]; then \
		if [ -n "$(CERTNAME)" ] && [ -f $(GOKRAZY_CERT_DIR)/cert.pem ] && [ -f $(GOKRAZY_CERT_DIR)/key.pem ]; then \
			echo "--- Creating SSH credentials (reusing cached TLS certificate for $(CERTNAME)) ---"; \
			printf '%s\n' "$(USER)" "$(PASS)" "0.0.0.0" "22" "ze" | \
				env ze.config.dir=tmp/gokrazy/init bin/ze init --force --yes 2>&1; \
			bin/ze data --path $(GOKRAZY_ZEFS) write meta/web/cert $(GOKRAZY_CERT_DIR)/cert.pem; \
			bin/ze data --path $(GOKRAZY_ZEFS) write meta/web/key $(GOKRAZY_CERT_DIR)/key.pem; \
		else \
			echo "--- Creating SSH credentials + TLS certificate ---"; \
			if [ -n "$(CERTNAME)" ]; then \
				printf '%s\n' "$(USER)" "$(PASS)" "0.0.0.0" "22" "ze" | \
					env ze.config.dir=tmp/gokrazy/init bin/ze init --force --yes --web-cert-name $(CERTNAME) 2>&1; \
				mkdir -p $(GOKRAZY_CERT_DIR); \
				bin/ze data --path $(GOKRAZY_ZEFS) cat meta/web/cert > $(GOKRAZY_CERT_DIR)/cert.pem; \
				bin/ze data --path $(GOKRAZY_ZEFS) cat meta/web/key > $(GOKRAZY_CERT_DIR)/key.pem; \
				echo "cached TLS certificate for $(CERTNAME) in $(GOKRAZY_CERT_DIR)/"; \
			else \
				printf '%s\n' "$(USER)" "$(PASS)" "0.0.0.0" "22" "ze" | \
					env ze.config.dir=tmp/gokrazy/init bin/ze init --force --yes --web-cert 0.0.0.0:8080 2>&1; \
			fi; \
		fi; \
		bin/ze data --path $(GOKRAZY_ZEFS) write file/template/ze.conf $(GOKRAZY_TEMPLATE); \
	elif [ ! -f $(GOKRAZY_ZEFS) ]; then \
		echo "error: no database found. First build requires credentials:"; \
		echo "  make ze-gokrazy USER=admin PASS=secret"; \
		exit 1; \
	else \
		echo "--- Reusing existing database ---"; \
	fi
	@echo "--- Building gokrazy image ---"
	GOOS=linux GOARCH=$(GOKRAZY_ARCH) bin/gok --parent_dir $(GOKRAZY_DIR) -i $(GOKRAZY_INSTANCE) overwrite \
		--full $(GOKRAZY_IMG) \
		--target_storage_bytes $(GOKRAZY_IMG_SIZE)
	@echo "--- Formatting /perm partition ---"
	$(E2FS)/mkfs.ext4 -q -F -O ^metadata_csum -E offset=$(GOKRAZY_PERM_OFF) $(GOKRAZY_IMG) $(GOKRAZY_PERM_BLK)
	@echo "--- Injecting credentials into /perm ---"
	@dd if=$(GOKRAZY_IMG) of=tmp/gokrazy/perm.img bs=4096 skip=$(GOKRAZY_PERM_SKIP) count=$(GOKRAZY_PERM_4K) 2>/dev/null
	@$(E2FS)/debugfs -w -R "mkdir ze" tmp/gokrazy/perm.img 2>/dev/null
	@$(E2FS)/debugfs -w -R "write tmp/gokrazy/init/database.zefs ze/database.zefs" tmp/gokrazy/perm.img 2>/dev/null
	@dd if=tmp/gokrazy/perm.img of=$(GOKRAZY_IMG) bs=4096 seek=$(GOKRAZY_PERM_SKIP) conv=notrunc 2>/dev/null
	@rm -f tmp/gokrazy/perm.img
	@echo ""
	@echo "Image ready: $(GOKRAZY_IMG)"
	@echo "Run: make ze-gokrazy-run"

ze-gokrazy-run:
	@test -f $(GOKRAZY_IMG) || { echo "error: $(GOKRAZY_IMG) not found (run: make ze-gokrazy USER=admin PASS=secret)"; exit 1; }
	@case "$(GOKRAZY_ARCH)" in \
		amd64) command -v qemu-system-x86_64 >/dev/null || { echo "error: qemu-system-x86_64 not found (brew install qemu)"; exit 1; } ;; \
		arm64) command -v qemu-system-aarch64 >/dev/null || { echo "error: qemu-system-aarch64 not found (brew install qemu)"; exit 1; }; test -f $(GOKRAZY_QEMU_AARCH64_BIOS) || { echo "error: $(GOKRAZY_QEMU_AARCH64_BIOS) not found"; exit 1; } ;; \
		*) echo "error: unsupported GOKRAZY_ARCH=$(GOKRAZY_ARCH) (expected amd64 or arm64)"; exit 1 ;; \
	esac
	@echo "Booting Ze gokrazy appliance..."
	@echo "  Ze web:      https://localhost:28080/"
	@echo "  Gokrazy:     https://localhost:28080/gokrazy/"
	@echo "  Ze SSH:      ssh -p 2222 <user>@localhost"
	@echo "  Quit:        Ctrl-A X"
	@echo ""
	@case "$(GOKRAZY_ARCH)" in \
		amd64) \
			qemu-system-x86_64 \
				-machine accel=$(GOKRAZY_QEMU_ACCEL) \
				-smp 2 -m 512 \
				-drive file=$(GOKRAZY_IMG),format=raw \
				-nographic -serial mon:stdio \
				-nic user,model=e1000,hostfwd=tcp::28080-:8080,hostfwd=tcp::2222-:22 ;; \
		arm64) \
			qemu-system-aarch64 \
				-machine virt,highmem=off,accel=$(GOKRAZY_QEMU_ACCEL) \
				-cpu $(GOKRAZY_QEMU_AARCH64_CPU) \
				-smp 2 -m 512 \
				-bios $(GOKRAZY_QEMU_AARCH64_BIOS) \
				-drive file=$(GOKRAZY_IMG),format=raw \
				-nographic -serial mon:stdio \
				-netdev user,id=net0,hostfwd=tcp::28080-:8080,hostfwd=tcp::2222-:22 \
				-device e1000,netdev=net0 ;; \
	esac

# ---------------------------------------------------------------------------
# Custom kernel build (overrides the rtr7/kernel pin used by ze-gokrazy)
# ---------------------------------------------------------------------------
KVER                  ?= 7.0.11
KERNEL_MODULE         := github.com/rtr7/kernel
KERNEL_ARCH           ?= $(GOKRAZY_ARCH)
KERNEL_BUILDER        ?= docker
KERNEL_MODULE_VERSION := $(shell cd gokrazy/ze/builddir/$(KERNEL_MODULE) 2>/dev/null && $(GO) list -m -f '{{.Version}}' $(KERNEL_MODULE) 2>/dev/null)
KERNEL_MODCACHE_DIR   := $(GOKRAZY_DIR)/modcache/$(KERNEL_MODULE)@$(KERNEL_MODULE_VERSION)
KERNEL_PINNED_BACKUP  := $(GOKRAZY_DIR)/modcache/.ze-pinned-kernel
KERNEL_BUILD_DIR      := tmp/kernel/build

ze-kernel:
	@case "$(KERNEL_ARCH)" in amd64|arm64) : ;; *) echo "error: unsupported KERNEL_ARCH=$(KERNEL_ARCH) (expected amd64 or arm64)"; exit 1 ;; esac
	@echo "--- Building runtime kernel ($(KVER), $(KERNEL_ARCH), builder=$(KERNEL_BUILDER)) ---"
	@$(MAKE) -C gokrazy/kernel BUILDER=$(KERNEL_BUILDER) ARCH=$(KERNEL_ARCH) KVER=$(KVER)
	@echo "--- Staging test kernel to tmp/kernel/vmlinuz (QEMU evidence: ze-qemu-l2tp-ppp-test, ze-qemu-pppoe-accel-test) ---"
	@mkdir -p tmp/kernel
	@cp "$(KERNEL_BUILD_DIR)/vmlinuz" tmp/kernel/vmlinuz
	@echo "--- Installing custom kernel into gokrazy module cache ---"
	@test -n "$(KERNEL_MODULE_VERSION)" || { echo "error: could not resolve pinned $(KERNEL_MODULE) version"; exit 1; }
	@test -d "$(KERNEL_MODCACHE_DIR)" || { echo "error: $(KERNEL_MODCACHE_DIR) not found (run: make ze-gokrazy-deps)"; exit 1; }
	@test -f "$(KERNEL_BUILD_DIR)/vmlinuz" || { echo "error: $(KERNEL_BUILD_DIR)/vmlinuz not found"; exit 1; }
	@test -d "$(KERNEL_BUILD_DIR)/lib/modules" || { echo "error: $(KERNEL_BUILD_DIR)/lib/modules not found"; exit 1; }
	@if [ ! -d "$(KERNEL_PINNED_BACKUP)" ]; then \
		echo "--- Backing up pinned kernel module cache ---"; \
		cp -R "$(KERNEL_MODCACHE_DIR)" "$(KERNEL_PINNED_BACKUP)"; \
	fi
	@cp "$(KERNEL_BUILD_DIR)/vmlinuz" "$(KERNEL_MODCACHE_DIR)/vmlinuz"
	@mkdir -p "$(KERNEL_MODCACHE_DIR)/lib"
	@rm -rf "$(KERNEL_MODCACHE_DIR)/lib/modules"
	@cp -R "$(KERNEL_BUILD_DIR)/lib/modules" "$(KERNEL_MODCACHE_DIR)/lib/"
	@rm -f "$(KERNEL_MODCACHE_DIR)"/*.dtb
	@cp "$(KERNEL_BUILD_DIR)"/*.dtb "$(KERNEL_MODCACHE_DIR)"/ 2>/dev/null || true
	@if [ -d "$(KERNEL_BUILD_DIR)/overlays" ]; then \
		rm -rf "$(KERNEL_MODCACHE_DIR)/overlays"; \
		cp -R "$(KERNEL_BUILD_DIR)/overlays" "$(KERNEL_MODCACHE_DIR)/"; \
	fi
	@echo ""
	@module_version=""; for d in $(KERNEL_BUILD_DIR)/lib/modules/*; do [ -d "$$d" ] || continue; module_version="$${d##*/}"; break; done; echo "Custom kernel: $$module_version"
	@echo "Next: make ze-gokrazy USER=... PASS=..."

ze-kernel-clean:
	@$(MAKE) -C gokrazy/kernel clean
	@if [ -d "$(KERNEL_PINNED_BACKUP)" ]; then \
		echo "--- Restoring pinned kernel module cache ---"; \
		rm -rf "$(KERNEL_MODCACHE_DIR)"; \
		cp -R "$(KERNEL_PINNED_BACKUP)" "$(KERNEL_MODCACHE_DIR)"; \
		rm -rf "$(KERNEL_PINNED_BACKUP)"; \
	else \
		echo "warning: no pinned kernel backup found; run make ze-gokrazy-deps to refresh the module cache if needed"; \
	fi
	@rm -rf tmp/kernel
	@echo "ze-gokrazy will now use the pinned rtr7/kernel."
