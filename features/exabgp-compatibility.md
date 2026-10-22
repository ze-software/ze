# ExaBGP Compatibility

## Meta

| Field | Value |
|-------|-------|
| Name | ExaBGP Compatibility |
| Page | docs/features/exabgp-compatibility.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/exabgp, internal/plugins/exabgp |
| Real-path tests | test/parse/cli-exabgp-migrate.ci, test/parse/cli-exabgp-migrate-env.ci, test/plugin/cli-exabgp-help.ci, test/plugin/exabgp-bridge-internal.ci, test/plugin/exabgp-bridge-sdk.ci, test/plugin/exabgp-bridge-bare-form-reaches-the-wire.ci, test/plugin/exabgp-bridge-shutdown-internal.ci, test/plugin/exabgp-bridge-shutdown-external.ci |
| Docs | docs/features/exabgp-compatibility.md |
| Doc review | 2026-10-22: ExaBGP migration is `ze exabgp migrate` (internal/plugins/exabgp/main.go), behind the ze_exabgp gate with the bridge (cmd/ze/dispatch_exabgp.go builds only under ze_exabgp); `ze config migrate` (internal/component/config/cli/cmd_migrate.go) converts Ze's own older config forms and does not read ExaBGP configs |
| Defect review | 2026-10-07: journal check-cannot-see-the-change-it-looks-for, declared-format-contradicts-payload and announced-state-never-replayed rows naming the bridge taken as open |
| Extra criteria | supported: migrate round-trips real ExaBGP configurations against a live ExaBGP = test/plugin/exabgp-migrate-live-roundtrip.ci |

## Description

Automatic config migration and plugin bridge. The bridge plugin and the `ze exabgp` command, which carries `ze exabgp migrate`, are compile-out-able with the `ze_exabgp` build tag (default-on in `ZE_FEATURES`). <!-- source: feature-gates.txt -- ze_exabgp -->
