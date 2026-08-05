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
#   make ze-gokrazy KERNEL_PKG=tmp/kernel/pkg  -- build against a custom kernel (see ze-kernel)
#   make ze-gokrazy-run                     -- boot image in QEMU
#
# Builds run from a COPY of gokrazy/ze under tmp/, prepared by ze-gok
# (internal/appliance/instance). No build step writes to a tracked path, so a
# custom-kernel build needs nothing reverted afterwards.

.PHONY: ze-gokrazy ze-gokrazy-deps ze-gokrazy-run ze-gokrazy-gosum-check ze-kernel ze-kernel-clean ze-host bin/gok

GOKRAZY_INSTANCE   := ze
GOKRAZY_DIR        := gokrazy
GOKRAZY_ARCH       ?= amd64
GOKRAZY_IMG        := tmp/gokrazy/ze.img
GOKRAZY_IMG_SIZE   := 2147483648
GOKRAZY_PERM_OFF   := 1157627904
GOKRAZY_PERM_BLK   := 966639
GOKRAZY_PERM_4K    := 241660
GOKRAZY_PERM_SKIP  := 282624
# e2fsprogs sbin dir holding mkfs.ext4 + debugfs. Linux ships them in /usr/sbin
# (or /sbin); macOS keg-only homebrew keeps them under the Cellar. Autodetect the
# first location that has BOTH tools; override with `make ... E2FS=/path`.
# Both, not just mkfs.ext4: the image build formats /perm with mkfs.ext4 and then
# injects credentials with debugfs, so a directory carrying only the first passes
# a one-tool probe and dies later. `make ... E2FS=` (explicitly empty) does not
# resume autodetect. Measured: ifndef tests whether the variable expands to
# something NON-EMPTY, so an empty override still enters this block and the probe
# RUNS; its result is then discarded, because a command-line assignment beats the
# makefile's `:=`. E2FS stays empty and the guard below rejects it. Pass a path,
# or leave E2FS unset.
ifndef E2FS
E2FS               := $(shell for d in /usr/sbin /sbin /usr/local/sbin $$(ls -d /opt/homebrew/Cellar/e2fsprogs/*/sbin 2>/dev/null); do [ -x "$$d/mkfs.ext4" ] && [ -x "$$d/debugfs" ] && { echo "$$d"; break; }; done)
endif
GOKRAZY_QEMU_ACCEL ?= tcg
GOKRAZY_QEMU_AARCH64_BIOS ?= /opt/homebrew/share/qemu/edk2-aarch64-code.fd
GOKRAZY_QEMU_AARCH64_CPU ?= max

# .PHONY on purpose: as a plain file target with no source prerequisites, a
# bin/gok built once was never rebuilt, so image builds silently ran a stale
# orchestrator (pre-preparer gok reached the network and skipped the prepared
# instance entirely). The vendored go build is cheap and cache-hot.
bin/gok:
	@echo "Building ze-gok from vendored source..."
	@mkdir -p bin
	go build -mod=vendor -o bin/gok ./cmd/ze-gok

GOMODCACHE_LOCAL := $(CURDIR)/$(GOKRAZY_DIR)/modcache

ze-gokrazy-deps: bin/gok
	@echo "Downloading gokrazy dependencies into $(GOKRAZY_DIR)/modcache/..."
	@for d in $$(find $(GOKRAZY_DIR)/$(GOKRAZY_INSTANCE)/builddir -name go.mod -exec dirname {} \;); do \
		echo "  $$d"; \
		(cd "$$d" && GOMODCACHE=$(GOMODCACHE_LOCAL) GOFLAGS=-modcacherw go mod download all) || exit 1; \
	done
	@echo "Done. Builds now work offline."

# Out-of-tree kernel package for a single build (see ze-kernel). Empty means the
# pinned github.com/rtr7/kernel. Never inferred from the filesystem: an implicit
# "use tmp/kernel/pkg if it exists" rule would silently give every later build a
# custom kernel.
KERNEL_PKG       ?=

GOKRAZY_ZEFS     := tmp/gokrazy/init/database.zefs
GOKRAZY_CERT_DIR := tmp/gokrazy/certs/$(CERTNAME)
GOKRAZY_TEMPLATE ?= gokrazy/ze/ze.conf

# Refuse to build an image whose packed modules disagree with the root module
# about what a given version contains. The tracked gokrazy/ze/builddir/**/go.sum
# files are read by no other build, so a drift there surfaces nowhere else.
ze-gokrazy-gosum-check:
	@python3 scripts/dev/gokrazy_gosum_check.py

ze-gokrazy: ze bin/gok ze-gokrazy-gosum-check
	@miss=""; for t in mkfs.ext4 debugfs; do { [ -n "$(E2FS)" ] && [ -x "$(E2FS)/$$t" ]; } || miss="$$miss $$t"; done; \
		[ -z "$$miss" ] || { echo "error: e2fsprogs tool(s) not found:$$miss (searched '$(E2FS)'). Install e2fsprogs (Debian/Ubuntu: apt install e2fsprogs; Fedora: dnf install e2fsprogs; macOS: brew install e2fsprogs), or pass the sbin directory that holds BOTH mkfs.ext4 and debugfs: make ze-gokrazy E2FS=/path/to/sbin"; exit 1; }
	@mkdir -p tmp/gokrazy/init
	@if [ -n "$(ZEFS)" ]; then \
		echo "--- Using external database: $(ZEFS) ---"; \
		cp "$(ZEFS)" $(GOKRAZY_ZEFS); \
	elif [ -n "$(USER)" ] && [ -n "$(PASS)" ]; then \
		: "Fresh-credentials build. --force moves any existing seed DB aside and"; \
		: "creates a clean one; the daemonRunning probe (init/main.go) now requires"; \
		: "ze's own SSH banner, so a host sshd on 0.0.0.0:22 no longer false-blocks"; \
		: "it (that false positive once forced a manual rm of the seed DB here)."; \
		: "--seed stops init baking this build host's interfaces into the active"; \
		: "config, which would shadow the file/template/ze.conf written below and"; \
		: "leave web/l2tp off; the appliance builds its config at first boot from"; \
		: "the template merged with on-device discovery."; \
		if [ -n "$(CERTNAME)" ] && [ -f $(GOKRAZY_CERT_DIR)/cert.pem ] && [ -f $(GOKRAZY_CERT_DIR)/key.pem ]; then \
			echo "--- Creating SSH credentials (reusing cached TLS certificate for $(CERTNAME)) ---"; \
			printf '%s\n' "$(USER)" "$(PASS)" "0.0.0.0" "22" "ze" | \
				env ze.config.dir=tmp/gokrazy/init $(ZEBIN_ZE) init --force --yes --seed 2>&1; \
			$(ZEBIN_ZE) data --path $(GOKRAZY_ZEFS) write meta/web/cert $(GOKRAZY_CERT_DIR)/cert.pem; \
			$(ZEBIN_ZE) data --path $(GOKRAZY_ZEFS) write meta/web/key $(GOKRAZY_CERT_DIR)/key.pem; \
		else \
			echo "--- Creating SSH credentials + TLS certificate ---"; \
			if [ -n "$(CERTNAME)" ]; then \
				printf '%s\n' "$(USER)" "$(PASS)" "0.0.0.0" "22" "ze" | \
					env ze.config.dir=tmp/gokrazy/init $(ZEBIN_ZE) init --force --yes --seed --web-cert-name $(CERTNAME) 2>&1; \
				mkdir -p $(GOKRAZY_CERT_DIR); \
				$(ZEBIN_ZE) data --path $(GOKRAZY_ZEFS) cat meta/web/cert > $(GOKRAZY_CERT_DIR)/cert.pem; \
				$(ZEBIN_ZE) data --path $(GOKRAZY_ZEFS) cat meta/web/key > $(GOKRAZY_CERT_DIR)/key.pem; \
				echo "cached TLS certificate for $(CERTNAME) in $(GOKRAZY_CERT_DIR)/"; \
			else \
				printf '%s\n' "$(USER)" "$(PASS)" "0.0.0.0" "22" "ze" | \
					env ze.config.dir=tmp/gokrazy/init $(ZEBIN_ZE) init --force --yes --seed --web-cert 0.0.0.0:8080 2>&1; \
			fi; \
		fi; \
		$(ZEBIN_ZE) data --path $(GOKRAZY_ZEFS) write file/template/ze.conf $(GOKRAZY_TEMPLATE); \
	elif [ ! -f $(GOKRAZY_ZEFS) ]; then \
		echo "error: no database found. First build requires credentials:"; \
		echo "  make ze-gokrazy USER=admin PASS=secret"; \
		exit 1; \
	else \
		echo "--- Reusing existing database ---"; \
	fi
	@echo "--- Building gokrazy image ---"
	@# ze-gok copies the instance under tmp/ before building, so the tracked
	@# gokrazy/ dir is never built from and never written to. KERNEL_PKG selects an
	@# out-of-tree kernel for THIS build only; leaving it unset uses the pin.
	@# `env` is required: a dotted name is not a valid shell identifier, so the
	@# `VAR=val cmd` prefix form would fail with "not a valid identifier".
	GOOS=linux GOARCH=$(GOKRAZY_ARCH) env 'ze.gok.kernel-package=$(KERNEL_PKG)' \
		bin/gok --parent_dir $(GOKRAZY_DIR) -i $(GOKRAZY_INSTANCE) overwrite \
		--full $(GOKRAZY_IMG) \
		--target_storage_bytes $(GOKRAZY_IMG_SIZE)
	@echo "--- Formatting /perm partition ---"
	$(E2FS)/mkfs.ext4 -q -F -O ^metadata_csum -E offset=$(GOKRAZY_PERM_OFF) $(GOKRAZY_IMG) $(GOKRAZY_PERM_BLK)
	@echo "--- Injecting credentials into /perm ---"
	@dd if=$(GOKRAZY_IMG) of=tmp/gokrazy/perm.img bs=4096 skip=$(GOKRAZY_PERM_SKIP) count=$(GOKRAZY_PERM_4K) 2>/dev/null
	@$(E2FS)/debugfs -w -R "mkdir ze" tmp/gokrazy/perm.img 2>/dev/null
	@$(E2FS)/debugfs -w -R "write tmp/gokrazy/init/database.zefs ze/database.zefs" tmp/gokrazy/perm.img 2>/dev/null
	@# Read the database back and compare it byte for byte. This is not belt and
	@# braces: debugfs EXITS 0 WHEN THE COMMAND FAILS and reports the failure only
	@# on stderr, which the two redirections above discard. Measured on e2fsprogs
	@# 1.47.0: a `write` into a directory that does not exist returns 0 with no
	@# output. Without this check an image whose /perm database was never written
	@# builds green and fails at boot, with nothing in the build output naming the
	@# cause.
	@#
	@# Dropping the `2>/dev/null` instead does not work: debugfs prints a version
	@# banner to stderr on every successful run, so the recipe would get noisier
	@# and still not fail.
	@#
	@# The comparison is on CONTENT rather than on `stat` output, so it does not
	@# depend on debugfs's field formatting and it catches a truncated write as
	@# well as an absent one.
	@$(E2FS)/debugfs -R "dump ze/database.zefs tmp/gokrazy/perm-readback.zefs" tmp/gokrazy/perm.img >/dev/null 2>&1; \
		cmp -s tmp/gokrazy/init/database.zefs tmp/gokrazy/perm-readback.zefs || { \
			echo "error: /perm credential injection failed: ze/database.zefs in the image does not match tmp/gokrazy/init/database.zefs."; \
			echo "       debugfs exits 0 on failure, so the write above cannot report this itself."; \
			echo "       The image would boot without its database. Refusing to publish it."; \
			rm -f tmp/gokrazy/perm-readback.zefs; \
			exit 1; }
	@rm -f tmp/gokrazy/perm-readback.zefs
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
# The runtime kernel is built out-of-tree (tmp/kernel/build via run.py, which
# reads internal/appliance/kernel.version), then assembled into an out-of-tree
# kernel PACKAGE (tmp/kernel/pkg: a copy of the pinned rtr7/kernel module with
# our vmlinuz/modules/DTBs/overlays).
#
# The package is selected PER BUILD: `make ze-gokrazy KERNEL_PKG=tmp/kernel/pkg`.
# ze-gok writes the `replace github.com/rtr7/kernel => <pkg>` into its prepared
# COPY of the instance, so no tracked file changes and there is nothing to
# revert; omitting KERNEL_PKG builds the pin. ze-kernel therefore writes no
# state, and ze-kernel-clean only removes tmp/kernel (plus a one-time migration
# for replaces left in the tracked go.mod by the pre-2026-07-23 flow). The pinned
# module cache is never mutated in place. gok resolves the kernel dir via
# `go list -mod=mod`, which honors the replace.
KERNEL_MODULE         := github.com/rtr7/kernel
KERNEL_ARCH           ?= $(GOKRAZY_ARCH)
KERNEL_BUILDER        ?= docker
KERNEL_MODULE_VERSION := $(shell cd gokrazy/ze/builddir/$(KERNEL_MODULE) 2>/dev/null && $(GO) list -m -f '{{.Version}}' $(KERNEL_MODULE) 2>/dev/null)
KERNEL_MODCACHE_DIR   := $(GOKRAZY_DIR)/modcache/$(KERNEL_MODULE)@$(KERNEL_MODULE_VERSION)
KERNEL_BUILD_DIR      := tmp/kernel/build
KERNEL_PKG_DIR        := tmp/kernel/pkg
KERNEL_BUILDDIR_GOMOD := gokrazy/ze/builddir/$(KERNEL_MODULE)/go.mod
# Legacy in-place-mutation backup from the pre-consolidation flow. The new flow
# never creates this; ze-kernel-clean restores from it once to migrate users
# whose modcache was overlaid by the old ze-kernel.
KERNEL_PINNED_BACKUP  := $(GOKRAZY_DIR)/modcache/.ze-pinned-kernel

# ze-host: the HOST ze binary that owns the cache KEY (Option C). Built with
# -tags ze_core,ze_setup and NO GOOS/GOARCH override so it runs on the build host; the
# target arch is passed as --arch, never applied to this build (CLAUDE.md "Binary naming
# convention"). Go stays the single source of truth for the key (kernelCacheVariantFor),
# so the make path can never drift from `ze appliance kernel`.
ze-host:
	@echo "--- Building host ze binary (ze-host: -tags ze_core,ze_setup, NO GOARCH override) ---"
	@$(GO) build -tags 'ze_core ze_setup' -o "$(CURDIR)/ze-host" ./cmd/ze

# ze-kernel routes the runtime kernel through the durable cache (~/.cache/ze, Option C):
# it asks ze-host for the arch+config-keyed cache dir, materializes from it on a HIT (no
# ~30-min rebuild), or builds via run.py then populates the cache on a MISS. The copy is
# staged in a sibling .copytree-* dir and swapped in with a rename -- the sanctioned
# equivalent of Go's atomic copyTree (internal/appliance/cache.go), so a concurrent reader
# on the shared cache key never sees a half-written tree. Plain `mv` (not `mv -T`, which
# macOS lacks) guarded by an existence check: if a concurrent populate already won the key,
# the loser discards its staging instead of nesting it inside the winner's tree.
# tmp/kernel/build stays a materialized VIEW, so `rm -rf tmp` costs only a copy.
ze-kernel: ze-host
	@case "$(KERNEL_ARCH)" in amd64|arm64) : ;; *) echo "error: unsupported KERNEL_ARCH=$(KERNEL_ARCH) (expected amd64 or arm64)"; exit 1 ;; esac
	@: "Dry-run guard: this recipe line embeds $$(MAKE), so GNU make executes it"; \
	: "even under -n. Without this, a dry run (or a make -n inspection test) would"; \
	: "run the real cache materialize/build/populate and, on a cold cache, fail the"; \
	: "magic check on a not-yet-built vmlinuz. The staging lines below (268+) still"; \
	: "print, so callers still see the out-of-tree assembly. The real (non -n) path"; \
	: "is byte-identical -- the guard is unreachable unless -n is set."; \
	case "$(firstword -$(MAKEFLAGS))" in *n*) echo "--- (dry-run) runtime kernel: materialize from durable cache on HIT, or build via run.py into $(KERNEL_BUILD_DIR) on MISS ---"; exit 0 ;; esac; \
	cache_dir="$$("$(CURDIR)/ze-host" appliance kernel --target runtime --arch $(KERNEL_ARCH) --print-cache-dir)"; \
	if [ -z "$$cache_dir" ]; then echo "error: could not resolve runtime kernel cache dir from ze-host"; exit 1; fi; \
	if [ -f "$$cache_dir/vmlinuz" ] && [ -d "$$cache_dir/lib/modules" ]; then \
		echo "--- Runtime kernel cache HIT: materializing from $$cache_dir (no ~30-min rebuild) ---"; \
		build_parent="$$(dirname "$(KERNEL_BUILD_DIR)")"; mkdir -p "$$build_parent"; \
		staging="$$(mktemp -d "$$build_parent/.copytree-XXXXXX")" && \
		cp -R "$$cache_dir/." "$$staging/" && \
		rm -rf "$(KERNEL_BUILD_DIR)" && \
		if [ -e "$(KERNEL_BUILD_DIR)" ]; then rm -rf "$$staging"; else mv "$$staging" "$(KERNEL_BUILD_DIR)"; rm -rf "$(KERNEL_BUILD_DIR)/$${staging##*/}"; fi; \
	else \
		echo "--- Runtime kernel cache MISS: building ($(KERNEL_ARCH), builder=$(KERNEL_BUILDER)) ---"; \
		: "Purge any stale build view first. The sub-make's target is the FILE"; \
		: "tmp/kernel/build/vmlinuz with no arch prerequisite, so a leftover view"; \
		: "from another arch would satisfy it, build NOTHING, and the populate"; \
		: "step below would poison this arch's durable cache key with the wrong"; \
		: "architecture -- permanently, since the HIT branch is existence-only."; \
		rm -rf "$(KERNEL_BUILD_DIR)"; \
		$(MAKE) -C gokrazy/kernel BUILDER=$(KERNEL_BUILDER) ARCH=$(KERNEL_ARCH); \
		: "Fail closed if what we are about to cache is not the requested"; \
		: "architecture: tmp/kernel/build is one shared unlocked path, so a"; \
		: "concurrent ze-kernel run with a different KERNEL_ARCH can rewrite it"; \
		: "between our sub-make and this populate, and an existence-only HIT"; \
		: "check would then serve the poisoned tree forever."; \
		python3 -c "import sys; magic={'amd64':(0x202,b'HdrS'),'arm64':(0x38,b'ARMd')}[sys.argv[2]]; data=open(sys.argv[1],'rb').read(magic[0]+4); sys.exit(0 if data[magic[0]:magic[0]+4]==magic[1] else 1)" "$(KERNEL_BUILD_DIR)/vmlinuz" "$(KERNEL_ARCH)" || { \
			echo "error: $(KERNEL_BUILD_DIR)/vmlinuz is missing or not a $(KERNEL_ARCH) kernel -- the build produced no usable image, or a concurrent ze-kernel run with a different KERNEL_ARCH clobbered the shared build dir; re-run: make ze-kernel KERNEL_ARCH=$(KERNEL_ARCH)"; exit 1; }; \
		echo "--- Populating durable cache: $$cache_dir ---"; \
		cache_parent="$$(dirname "$$cache_dir")"; mkdir -p "$$cache_parent"; \
		staging="$$(mktemp -d "$$cache_parent/.copytree-XXXXXX")" && \
		cp -R "$(KERNEL_BUILD_DIR)/." "$$staging/" && \
		rm -rf "$$cache_dir" && \
		if [ -e "$$cache_dir" ]; then rm -rf "$$staging"; else mv "$$staging" "$$cache_dir"; rm -rf "$$cache_dir/$${staging##*/}"; fi; \
		"$(CURDIR)/ze-host" appliance kernel --target runtime --arch $(KERNEL_ARCH) --evict-cache; \
	fi
	@echo "--- Staging test kernel to tmp/kernel/vmlinuz (QEMU evidence: ze-qemu-l2tp-ppp-test, ze-qemu-pppoe-accel-test) ---"
	@mkdir -p tmp/kernel
	@cp "$(KERNEL_BUILD_DIR)/vmlinuz" tmp/kernel/vmlinuz
	@echo "--- Assembling out-of-tree kernel package ($(KERNEL_PKG_DIR)) ---"
	@test -n "$(KERNEL_MODULE_VERSION)" || { echo "error: could not resolve pinned $(KERNEL_MODULE) version (run: make ze-gokrazy-deps)"; exit 1; }
	@test -d "$(KERNEL_MODCACHE_DIR)" || { echo "error: $(KERNEL_MODCACHE_DIR) not found (run: make ze-gokrazy-deps)"; exit 1; }
	@test -f "$(KERNEL_BUILD_DIR)/vmlinuz" || { echo "error: $(KERNEL_BUILD_DIR)/vmlinuz not found"; exit 1; }
	@test -d "$(KERNEL_BUILD_DIR)/lib/modules" || { echo "error: $(KERNEL_BUILD_DIR)/lib/modules not found"; exit 1; }
	@rm -rf "$(KERNEL_PKG_DIR)"
	@mkdir -p "$(KERNEL_PKG_DIR)"
	@cp -R "$(KERNEL_MODCACHE_DIR)/." "$(KERNEL_PKG_DIR)/"
	@chmod -R u+w "$(KERNEL_PKG_DIR)"
	@cp "$(KERNEL_BUILD_DIR)/vmlinuz" "$(KERNEL_PKG_DIR)/vmlinuz"
	@mkdir -p "$(KERNEL_PKG_DIR)/lib"
	@rm -rf "$(KERNEL_PKG_DIR)/lib/modules"
	@cp -R "$(KERNEL_BUILD_DIR)/lib/modules" "$(KERNEL_PKG_DIR)/lib/"
	@rm -f "$(KERNEL_PKG_DIR)"/*.dtb
	@cp "$(KERNEL_BUILD_DIR)"/*.dtb "$(KERNEL_PKG_DIR)"/ 2>/dev/null || true
	@if [ -d "$(KERNEL_BUILD_DIR)/overlays" ]; then \
		rm -rf "$(KERNEL_PKG_DIR)/overlays"; \
		cp -R "$(KERNEL_BUILD_DIR)/overlays" "$(KERNEL_PKG_DIR)/"; \
	fi
	@echo ""
	@module_version=""; for d in $(KERNEL_PKG_DIR)/lib/modules/*; do [ -d "$$d" ] || continue; module_version="$${d##*/}"; break; done; echo "Custom kernel: $$module_version (out-of-tree at $(KERNEL_PKG_DIR))"
	@echo "Next: make ze-gokrazy KERNEL_PKG=$(KERNEL_PKG_DIR) USER=... PASS=..."
	@echo "      (omit KERNEL_PKG to build against the pinned kernel; nothing to revert)"

ze-kernel-clean:
	@$(MAKE) -C gokrazy/kernel clean
	@# Migration only. Builds no longer write a replace into the tracked go.mod
	@# (the kernel package is passed per build via KERNEL_PKG), so this clears a
	@# replace left behind by a pre-2026-07-23 ze-kernel run.
	@if grep -q 'replace $(KERNEL_MODULE) ' $(KERNEL_BUILDDIR_GOMOD) 2>/dev/null; then \
		echo "--- Dropping a stale out-of-tree kernel replace from the tracked go.mod ---"; \
		$(GO) mod edit -dropreplace=$(KERNEL_MODULE) $(KERNEL_BUILDDIR_GOMOD); \
	fi
	@if [ -d "$(KERNEL_PINNED_BACKUP)" ]; then \
		echo "--- Migrating: restoring pinned modcache from legacy .ze-pinned-kernel and removing it ---"; \
		rm -rf "$(KERNEL_MODCACHE_DIR)"; \
		cp -R "$(KERNEL_PINNED_BACKUP)" "$(KERNEL_MODCACHE_DIR)"; \
		rm -rf "$(KERNEL_PINNED_BACKUP)"; \
	fi
	@rm -rf tmp/kernel
	@echo "ze-gokrazy will now use the pinned rtr7/kernel."
