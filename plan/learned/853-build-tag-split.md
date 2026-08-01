# 853 -- Build Tag Split

## Context

Ze used a single negative build tag (`ze_stripped`) to produce two binary variants: the default build included everything, and `ze_stripped` excluded install/appliance/self-update features for the gokrazy appliance image. This inverted the safety model: new code was included in all binaries by default unless someone remembered to gate it behind `!ze_stripped`. It also conflated three unrelated concerns (on-device install, appliance building, PXE provisioning) into one binary.

## Decisions

- Chose three positive tags (`ze_distro`, `ze_appliance`, `ze_setup`) over a single `ze_device`/`ze_setup` pair, because appliance and provisioning are independent concerns that operators may want separately.
- Chose same `cmd/ze/` entry point with build tags over separate `cmd/ze-setup/` and `cmd/ze-appliance/` main packages, because tags are Go's native mechanism for this and avoid code duplication.
- Chose no-tag default as stripped/minimal over requiring a strip tag, because it matches gokrazy (no config change needed) and new code is excluded by default until explicitly tagged.
- Chose `ze_distro` for the self-update backend gate over a separate `ze_selfupdate` tag, because self-update is a property of the on-device binary, not an independent feature.
- Chose deleting redundant stripped package files (`doc_stripped.go`, `register_stripped.go`) over converting them, because their non-stripped counterparts (`doc.go`, `register.go`) have no build constraints and always compile.
- Did not need conditional install/uninstall root handler registration: the dispatch packages are only imported transitively when a subcommand plugin is imported via a tag file. No-tag build naturally has no install command.

## Consequences

- The gokrazy config now uses empty `GoBuildTags: []` instead of `["ze_stripped"]`.
- `make build` produces four binaries: `ze` (ze_distro), `ze-appliance` (ze_appliance), `ze-setup` (ze_setup), `ze-stripped` (no tags). Dev builds get all features via `ze_distro,ze_appliance,ze_setup`.
- New feature code must explicitly choose which tag(s) to compile under. Without a tag, it's excluded from all non-dev binaries.
- `go build ./cmd/ze` (no tags) on a developer workstation now produces a stripped binary, not a full one. Developers must use `make build` or explicit tags for full features.
- Files named `*_linux.go` get an implicit GOOS=linux build constraint from Go. The ze_distro tag file is named `setup_features_distro.go` to avoid this.

## Gotchas

- Go treats `_linux.go` filename suffix as an implicit `GOOS=linux` constraint. The initial `setup_features_linux.go` silently excluded the file on macOS, producing a binary with no install commands. Renamed to `setup_features_distro.go`.
- Build tag tests that assert negative presence ("appliance NOT registered") fail when all tags are combined. Used compound constraints (`ze_distro && !ze_appliance && !ze_setup`) so each test only runs in its intended single-tag scenario.
- The `ze_stripped` string appeared in learned summaries, docs/features.md, source anchors, CODE-TO-DOCS.md, and file names (not just build constraints). All required updating for AC-10 (zero grep matches).
- Files renamed from `*_stripped*` to `*_minimal*` to eliminate `ze_stripped` substring from filenames that appear in documentation references.

## Files

- Created: `cmd/ze/setup_features_distro.go` (ze_distro), `cmd/ze/setup_features_setup.go` (ze_setup), `cmd/ze/setup_features_appliance.go` (ze_appliance)
- Created: `cmd/ze/build_tag_distro_test.go`, `cmd/ze/build_tag_appliance_test.go`, `cmd/ze/build_tag_setup_test.go`, `cmd/ze/build_tag_full_test.go`
- Deleted: `cmd/ze/setup_features_full.go`, `cmd/ze/setup_features_stripped.go`, `cmd/ze/appliance_import.go`
- Deleted: `internal/component/cmd/update/doc_stripped.go`, `internal/component/cmd/update/yang/register_stripped.go`
- Renamed: `backend_ze_stripped.go` -> `backend_ze_minimal.go`, `backend_ze_stripped_test.go` -> `backend_ze_minimal_test.go`, `firmware_stripped_test.go` -> `firmware_minimal_test.go`, `register_stripped_test.go` -> `register_minimal_test.go`
- Modified: `cmd/ze/setup_features_stripped_test.go` (removed build tag)
- Modified: `internal/component/config/system/backend_ze_distro.go`, `selfupdate.go`, `selfupdate_test.go`, `backend_test.go` (changed `!ze_stripped` to `ze_distro`)
- Modified: `internal/component/config/system/backend_ze_appliance.go` (changed `ze_stripped` to `!ze_distro`, updated messages)
- Modified: `internal/component/config/system/backend_ze_appliance_test.go` (changed tag + assertions)
- Modified: `internal/component/cmd/update/firmware_minimal_test.go`, `schema/register_minimal_test.go` (changed tags + assertions)
- Modified: `internal/appliance/cmd_build_test.go` (updated gokrazy config assertion)
- Modified: `Makefile` (added bin/ze-appliance, bin/ze-setup targets; bin/ze uses ze_distro; bin/ze-stripped uses no tags)
- Modified: `gokrazy/ze/config.json` (GoBuildTags: [])
- Modified: `mk/test-integration.mk` (removed ze_stripped from QEMU build)
- Modified: `docs/features.md` (updated build tag references and source anchors)
- Modified: `plan/learned/850-appliance-command-plugin.md`, `plan/learned/834-stripped-build-and-iso-coverage.md` (updated references)
- Modified: `ai/CODE-TO-DOCS.md` (updated file mapping)
