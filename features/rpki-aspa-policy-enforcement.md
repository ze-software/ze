# RPKI ASPA Policy Enforcement

## Meta

| Field | Value |
|-------|-------|
| Name | RPKI ASPA Policy Enforcement |
| Kind | protocol |
| Scope | complete |
| Level | stub-backed |
| Components | internal/component/bgp/plugins/rpki |
| Real-path tests | test/plugin/rpki-aspa-valid.ci, test/plugin/rpki-aspa-invalid.ci, test/plugin/rpki-aspa-unknown.ci, test/plugin/rpki-aspa-disabled.ci, test/plugin/rpki-aspa-policy-reject.ci, test/plugin/rpki-aspa-policy-logonly.ci, test/plugin/rpki-aspa-policy-unknown-reject.ci |
| RFCs | draft-ietf-sidrops-aspa-verification |
| Docs | docs/guide/rpki.md |
| Stub evidence | test/plugin/rpki-aspa-valid.ci, test/plugin/rpki-aspa-invalid.ci, test/plugin/rpki-aspa-unknown.ci, test/plugin/rpki-aspa-disabled.ci, test/plugin/rpki-aspa-policy-reject.ci, test/plugin/rpki-aspa-policy-logonly.ci, test/plugin/rpki-aspa-policy-unknown-reject.ci |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; reject log-only accept match parseActionLeaf in internal/component/bgp/plugins/rpki/rpki_config.go |
| Defect review | 2026-10-07: plan/immediate/spec-rpki-invalid-accepted-and-state-policy.md open |
| Extra criteria | supported: ASPA records served by a real RTR v2 cache = test/interop/scenarios/rtr-stayrtr-aspa |

## Description

ASPA path verification (draft-ietf-sidrops-aspa-verification-28) with configurable policy enforcement: `rpki/aspa/action/invalid` supports `reject`, `log-only`, `accept`. ASPA records use the RTR v2 format from draft-ietf-sidrops-8210bis-27. <!-- source: internal/component/bgp/plugins/rpki/rpki_config.go -- parseRPKIConfig -->
