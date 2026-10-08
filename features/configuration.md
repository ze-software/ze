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
| Defect review | 2026-10-07: open: plan/immediate/spec-config-verify-rejects-unknown-keys.md, spec-config-leaf-consumption-gate.md, and the plan/pre-release/spec-config-yang-*.md set |

## Description

YANG-modeled config with prefix limits, update groups, session resilience, duplicate list-key rejection, side-effect-free in-process plugin verifiers for static/API/CLI validation, and transactional commits using active/candidate/rollback pointers. Live external plugin `OnConfigVerify` callbacks run only in daemon reload/commit transactions.
