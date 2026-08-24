# Appliance installer: ISO and PXE boot
#
# Full build from a JSON config file:
#   make ze-iso-build-full CONFIG=prod.json SSH_PASSWORD='...'
#
# Rebuild (appliance already initialized):
#   make ze-iso-build NAME=prod
#
# PXE boot (optional, after ISO build):
#   make ze-pxe-build NAME=prod
#
# The CONFIG file drives everything: arch, kernel profile, credentials,
# networking. See docs/guide/appliance.md for the schema. Only
# SSH_PASSWORD is passed separately (env var, not stored in the file).
#
# Variables:
#   CONFIG             Path to appliance JSON config file
#   NAME               Appliance name (default: stem of CONFIG, or "prod")
#   SSH_PASSWORD       SSH password (required for init)
#   APPLIANCE_BUILDER  docker or qemu (default: docker)
#   PXE_DIR            PXE/TFTP root (default: build/pxe)
#   IPXE_DIR           iPXE source checkout (default: build/ipxe)

.PHONY: ze-iso-build-full ze-iso-initialize ze-iso-build ze-iso-check ze-pxe-build ze-installer-build

CONFIG            ?=
APPLIANCE_BUILDER ?= docker
PXE_DIR           ?= build/pxe
IPXE_DIR          ?= build/ipxe

# Derive NAME from CONFIG filename when CONFIG is set, otherwise default to prod.
ifdef CONFIG
NAME ?= $(basename $(notdir $(CONFIG)))
else
NAME ?= prod
endif

APPLIANCE_DIR := $(HOME)/.config/ze/appliances/$(NAME)

# --- Full build: init + ISO from a config file --------------------------------

ze-iso-build-full: $(ZEBIN_SETUP)
	@test -n "$(CONFIG)" || { echo "error: CONFIG required"; echo "  make ze-iso-build-full CONFIG=mybox.json SSH_PASSWORD='...'"; exit 1; }
	@test -f "$(CONFIG)" || { echo "error: $(CONFIG) not found"; exit 1; }
	@test -n "$(SSH_PASSWORD)" || { echo "error: SSH_PASSWORD required"; echo "  make ze-iso-build-full CONFIG=$(CONFIG) SSH_PASSWORD='...'"; exit 1; }
	@rm -rf $(APPLIANCE_DIR)
	@echo "=== Initializing appliance $(NAME) from $(CONFIG) ==="
	env ze.appliance.ssh.password='$(SSH_PASSWORD)' \
		$(ZEBIN_SETUP) appliance init --config $(CONFIG) $(NAME)
	@echo ""
	@echo "=== Building installer kernel ($(APPLIANCE_BUILDER)) ==="
	$(ZEBIN_SETUP) appliance kernel --builder $(APPLIANCE_BUILDER) $(NAME)
	@echo ""
	@echo "=== Building installer initrd ==="
	$(ZEBIN_SETUP) appliance initrd
	@echo ""
	@echo "=== Building appliance disk image ==="
	$(ZEBIN_SETUP) appliance build $(NAME)
	@echo ""
	@echo "=== Building installer ISO ==="
	$(ZEBIN_SETUP) appliance iso $(NAME)
	@echo ""
	@iso=$$(ls -1t $(APPLIANCE_DIR)/*.iso 2>/dev/null | head -1); \
	if [ -n "$$iso" ]; then \
		echo "ISO ready: $$iso"; \
		echo ""; \
		echo "Copy off the box:"; \
		echo "  scp $$(hostname):$$iso /tmp/ze.iso"; \
		echo ""; \
		echo "Or build PXE boot:"; \
		echo "  make ze-pxe-build NAME=$(NAME)"; \
	fi

# --- Individual steps ---------------------------------------------------------

ze-iso-initialize: $(ZEBIN_SETUP)
	@test -n "$(CONFIG)" || { echo "error: CONFIG required"; echo "  make ze-iso-initialize CONFIG=mybox.json SSH_PASSWORD='...'"; exit 1; }
	@test -f "$(CONFIG)" || { echo "error: $(CONFIG) not found"; exit 1; }
	@test -n "$(SSH_PASSWORD)" || { echo "error: SSH_PASSWORD required"; exit 1; }
	env ze.appliance.ssh.password='$(SSH_PASSWORD)' \
		$(ZEBIN_SETUP) appliance init --config $(CONFIG) $(NAME)

ze-iso-check: $(ZEBIN_SETUP)
	@$(ZEBIN_SETUP) appliance iso --check

ze-iso-build: $(ZEBIN_SETUP)
	@echo "--- Building installer kernel ($(APPLIANCE_BUILDER)) ---"
	$(ZEBIN_SETUP) appliance kernel --builder $(APPLIANCE_BUILDER) $(NAME)
	@echo "--- Building installer initrd ---"
	$(ZEBIN_SETUP) appliance initrd
	@echo "--- Building appliance disk image ---"
	$(ZEBIN_SETUP) appliance build $(NAME)
	@echo "--- Building installer ISO ---"
	$(ZEBIN_SETUP) appliance iso $(NAME)
	@echo ""
	@iso=$$(ls -1t $(APPLIANCE_DIR)/*.iso 2>/dev/null | head -1); \
	if [ -n "$$iso" ]; then \
		echo "ISO ready: $$iso"; \
	fi

ze-installer-build:
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -tags ze_installer -ldflags "$(ZE_LDFLAGS)" -o bin/ze-installer-amd64 ./cmd/ze-installer
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -tags ze_installer -ldflags "$(ZE_LDFLAGS)" -o bin/ze-installer-arm64 ./cmd/ze-installer

ze-pxe-build: $(ZEBIN_SETUP)
	@test -d "$(APPLIANCE_DIR)" || { echo "error: appliance $(NAME) not found; run ze-iso-build-full or ze-iso-build first"; exit 1; }
	@echo "--- Setting up PXE boot ---"
	mkdir -p $(PXE_DIR)/tftp $(PXE_DIR)/boot
	cp build/kernel/Image $(PXE_DIR)/boot/vmlinuz
	cp build/initrd/initrd.img.gz $(PXE_DIR)/boot/
	@img=$$(ls -1t $(APPLIANCE_DIR)/*.img 2>/dev/null | head -1 | xargs basename 2>/dev/null); \
	test -n "$$img" || { echo "error: no .img found in $(APPLIANCE_DIR)"; exit 1; }; \
	echo "--- Building iPXE binaries ---"; \
	test -d "$(IPXE_DIR)/.git" || git clone https://github.com/ipxe/ipxe.git $(IPXE_DIR); \
	printf '#!ipxe\nkernel http://$${next-server}/install/boot/vmlinuz ze.server=$${next-server} ze.image=%s ze.mac=$${mac} ip=dhcp panic=-1 console=ttyS0,115200n8 console=tty0\ninitrd http://$${next-server}/install/boot/initrd.img.gz\nboot\n' "$$img" > tmp/ze-install.ipxe; \
	$(MAKE) -C $(IPXE_DIR)/src bin/ipxe.pxe EMBED=$(CURDIR)/tmp/ze-install.ipxe; \
	$(MAKE) -C $(IPXE_DIR)/src bin-x86_64-efi/ipxe.efi EMBED=$(CURDIR)/tmp/ze-install.ipxe; \
	cp $(IPXE_DIR)/src/bin/ipxe.pxe $(PXE_DIR)/tftp/ipxe.pxe; \
	cp $(IPXE_DIR)/src/bin-x86_64-efi/ipxe.efi $(PXE_DIR)/tftp/ipxe.efi; \
	echo ""; \
	echo "PXE ready: $(PXE_DIR)/tftp/"
