# Unit tests: Go test targets with race detector
#
# Quick reference:
#   make ze-unit-test          All packages, -race (~5 min)
#   make ze-test-bgp           BGP component group only (~1:30)
#   make ze-test-core           Core libraries only (~30s)
#   make ze-test-plugins       Plugins only (~40s)
#   make ze-test-config        Config only (~20s)
#   make ze-test-cli           CLI only (~10s)
#   make ze-test-rest          Everything else (~1:00)

.PHONY: ze-unit-test ze-installer-unit-test ze-unit-test-cover ze-unit-test-cached ze-unit-test-race-changed
.PHONY: ze-test-bgp ze-test-core ze-test-plugins ze-test-config ze-test-cli ze-test-rest

ze-unit-test ze-unit-test-cover ze-unit-test-cached ze-unit-test-race-changed ze-test-rest: ze-ensure-links

# Component groups for scoped testing (ze-test-<group>).
# "rest" = everything in ZE_PACKAGES not covered by a named group.
ZE_GROUP_BGP     = ./internal/component/bgp/...
ZE_GROUP_CORE    = ./internal/core/...
ZE_GROUP_PLUGINS = ./internal/plugins/...
ZE_GROUP_CONFIG  = ./internal/component/config/...
ZE_GROUP_CLI     = ./internal/component/cli/...
ZE_GROUP_REST    = $$(go list ./... | grep -v /cmd/ze-chaos \
	| grep -v '^codeberg.org/thomas-mangin/ze/internal/component/bgp' \
	| grep -v '^codeberg.org/thomas-mangin/ze/internal/core' \
	| grep -v '^codeberg.org/thomas-mangin/ze/internal/plugins' \
	| grep -v '^codeberg.org/thomas-mangin/ze/internal/component/config' \
	| grep -v '^codeberg.org/thomas-mangin/ze/internal/component/cli')

# Run ze unit tests with race detector (default-on features plus bare-core compile-out checks).
ze-unit-test: ze-installer-unit-test
	@echo "Running ze unit tests..."
	$(GO_TEST_RACE) $(ZE_PACKAGES)
	@echo "Unit tests: bare ze_core compile-out checks..."
	$(GO_TEST_CORE_RACE) ./cmd/ze/hub

# The installer initrd is built with `ze_installer`, so every _test.go guarded by
# that tag is invisible to the default test run: `go test` without it silently
# compiles a package with those files excluded. Four files sat in that state
# (tracked as `tag-orphan` in test/health/latest.json), including the rescue
# console's fatal-branch policy -- the code that decides whether a failed install
# opens a shell, opens nothing, or reboots. They compile and pass; they were just
# never asked to run.
#
# GOOS=linux because the files are also `//go:build linux`; this is a compile +
# run of the installer's own logic, not a QEMU boot.
ze-installer-unit-test:
	@echo "Unit tests: installer initrd (ze_installer tag)..."
	GOOS=linux go test -tags 'ze_core ze_installer' ./internal/install/...

# Run ze unit tests with coverage.
ze-unit-test-cover:
	@echo "Running ze unit tests with coverage..."
	$(GO_TEST_RACE) -coverprofile=coverage.out $(ZE_PACKAGES)
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Cacheable full pass: no -race, Go caches results by source hash.
# Instant when nothing changed, catches logic regressions everywhere.
ze-unit-test-cached:
	@echo "Unit tests: full pass (cacheable, no -race)..."
	$(GO_TEST) $(ZE_PACKAGES)
	@echo "Unit tests: bare ze_core compile-out checks..."
	$(GO_TEST_CORE) ./cmd/ze/hub

# Race pass: -race only on component groups with changed .go files.
# Unmapped packages are included individually, never as a full sweep.
ze-unit-test-race-changed:
	@groups=$$(scripts/dev/changed-groups.sh --pkgs 2>/dev/null); \
	if [ -z "$$groups" ]; then \
		echo "No changed .go files -- skipping -race pass"; \
	else \
		echo "Unit tests: -race on changed groups: $$groups"; \
		$(GO_TEST_RACE) $$groups; \
		echo "Unit tests: bare ze_core compile-out checks..."; \
		$(GO_TEST_CORE_RACE) ./cmd/ze/hub; \
	fi

# ─── Component-group unit tests ─────────────────────────────────────────────
# Each group covers one logical area. Use during development to test only
# what you're working on. All groups together = ze-unit-test.

ze-test-bgp:
	@echo "Unit tests: bgp group..."
	$(GO_TEST_RACE) $(ZE_GROUP_BGP)

ze-test-core:
	@echo "Unit tests: core group..."
	$(GO_TEST_RACE) $(ZE_GROUP_CORE)

ze-test-plugins:
	@echo "Unit tests: plugins group..."
	$(GO_TEST_RACE) $(ZE_GROUP_PLUGINS)

ze-test-config:
	@echo "Unit tests: config group..."
	$(GO_TEST_RACE) $(ZE_GROUP_CONFIG)

ze-test-cli:
	@echo "Unit tests: cli group..."
	$(GO_TEST_RACE) $(ZE_GROUP_CLI)

ze-test-rest:
	@echo "Unit tests: rest group (everything not in a named group)..."
	$(GO_TEST_RACE) $(ZE_GROUP_REST)
