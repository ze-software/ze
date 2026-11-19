# Configuration

## Meta

| Field | Value |
|-------|-------|
| Name | Configuration |
| Page | docs/features/configuration.md |
| Kind | daemon |
| Scope | partial |
| Scope gaps | live external plugin OnConfigVerify runs only inside daemon reload and commit transactions, YANG types bits and identityref and instance-identifier and leafref (plan/pre-release/spec-config-yang-type-*.md), YANG deviation and if-feature and submodules (plan/pre-release/spec-config-yang-*.md) |
| Level | experimental |
| Components | internal/component/config |
| Real-path tests | test/reload/commit-transactional.ci, test/reload/commit-verify-reject.ci, test/reload/config-reload-invalid-validator.ci, test/parse/config-startup-refuses-invalid-validator.ci, test/parse/config-secret-roundtrip.ci |
| Docs | docs/features/configuration.md |
| Doc review | 2026-10-07: row prose matches the audit of internal/component/config (transactional active/candidate/rollback pointers); no sentence found false |
| Defect review | 2026-10-07: open: plan/release/fixes/spec-config-verify-rejects-unknown-keys.md, spec-config-leaf-consumption-gate.md, and the plan/pre-release/spec-config-yang-*.md set |

## Description

YANG-modeled config with prefix limits, update groups, session resilience, duplicate list-key rejection, side-effect-free in-process plugin verifiers for static/API/CLI validation, and transactional commits using active/candidate/rollback pointers. Live external plugin `OnConfigVerify` callbacks run only in daemon reload/commit transactions.

Read-only commands under `show config supplied` inspect bounded caller-held
bytes through the same dump, validation, formatting, graph and comparison
producers as file inputs. They do not stage or publish configuration.

The same registered surface provides current-format conversion, diagnostic
repair plans and masked, unexpanded source-subtree display. File counterparts
confine daemon-host paths; supplied forms receive caller bytes. Environment
validation uses the registered type and recorded-range authority shared with
doctor, while list/history adapters share their inventory producers.
<!-- source: internal/component/config/cli/config_supplied_tools.go -- suppliedMigrate, suppliedFixPlan, sourceBytesAnswer -->
<!-- source: internal/component/config/cli/config_environment.go -- environmentValidationAnswer -->
<!-- source: internal/component/config/cli/config_inventory.go -- configListRows, configHistoryRows -->

Detached set, activation, deactivation and complete replacement return text
through shared Editor operations without publishing into the live candidate.
Change statements and assigned values stay inside the masked input envelope;
replacement is checked by the shared transition validator, whose advisories
remain visible in serialized and preview results. Local file tools
choose their own file/stdout publication after the same editing operation.
<!-- source: internal/component/config/cli/config_detached.go -- detachedMutationAnswer, applyConfigSet, applyConfigActivation, detachedReplaceAnswer -->
<!-- source: internal/component/config/cli/config_supplied.go -- suppliedHandler -->
<!-- source: internal/component/config/cli/register.go -- registerSuppliedCommands -->

Pending validation selects the saved draft through one local/daemon reader and
confines the actual draft path on remote calls. History declares its file
selector and shares the same revision rows with the local adapter.
<!-- source: internal/component/config/cli/config_validation_source.go -- readValidationSource -->
<!-- source: internal/component/config/cli/config_data.go -- dataHistory, dataValidate -->

Stored configuration rename preserves its complete version, pointer and saved
editor family. The local adapter shares the storage transaction and sends a
busy store's operation through its owning daemon, with a matching-name
acknowledgment. The daemon refuses active running and managed-client identities.
<!-- source: internal/component/config/storage/rename_config.go -- RenameConfig -->
<!-- source: internal/component/config/storage/cli/data_rpc_rename.go -- handleDataConfigRename -->
<!-- source: internal/component/config/cli/config_rename_daemon.go -- renameConfigThroughDaemon -->

Stored rollback selects the existing history order and publishes a new active
version through the storage owner, retaining the previous active version.
The local adapter routes busy stores to their owning daemon and requires a
complete matching acknowledgment. The running source uses normal stage/reload
publication. Loose files retain direct offline restoration.
<!-- source: internal/component/config/storage/rollback_config.go -- RollbackConfig -->
<!-- source: internal/component/config/storage/cli/data_rpc_rollback.go -- handleDataConfigRollback, rollbackDaemonConfig -->
<!-- source: internal/component/config/cli/cmd_rollback.go -- cmdRollbackThrough, rollbackStoredConfig -->
<!-- source: internal/component/config/cli/config_rollback_daemon.go -- rollbackConfigThroughDaemon -->
