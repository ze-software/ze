# Published RFC conformance ledger

## Meta

| Field | Value |
|-------|-------|
| Name | Published RFC conformance ledger |
| Page | docs/features/rfc-status.md |
| Kind | dev-tool |
| Scope | complete |
| Level | experimental |
| Components | internal/le/site, internal/le/rfc |
| Real-path tests | test/ui/le-rfc-answers.ci, test/ui/le-rfc-partial-proof.ci, internal/le/site/rfcdetail_test.go::TestABuildPublishesTheRequirementLedger, internal/le/site/rfcdetail_test.go::TestAStemPageStatesItsEnrolmentAndItsPublicStatus |
| Docs | docs/features/rfc-status.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; one page per summary is writeRFCDetailPage and the ledger is collectRequirementLedger in internal/le/site |
| Defect review | 2026-10-07: journal claim-outlives-the-evidence-it-cites rows taken as open |
| Extra criteria | supported: the published tier reflects an executed run rather than the presence of a test = internal/le/site/rfcledger_executed_run_test.go |

## Description

Every requirement this repository extracted from an RFC is published, one page per summary under `/quality/rfc-compliance/<stem>/`. Each page carries the requirement text, its level and section, the tests bound to it and the carrier that runs them, the audit verdict a reader recorded and how current it is, the recorded break each tagged unit was seen to fail under, and the extraction sign-off that decided which sentences became requirements. The disclosure is full: a gated MUST with no test, a `weak`, `wrong` or `unimplemented` verdict, a stale or shifted verdict, a tagged unit with no discrimination record, and a `no-break` record are each named under their own requirement id rather than folded into a count. <!-- source: internal/le/site/rfcledger.go -- collectRequirementLedger --> <!-- source: internal/le/site/rfcdetail.go -- writeRFCDetailPage -->
