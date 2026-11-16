# Spec: github-notifier-credential-rotation

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling, docs |
| Priority | P0 |
| Release reason | Private maintainer-notifier credential exposure; no shipped NOS behavior or first-release dependency. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-16 |

## Task

Rotation of the earlier exposed Discord webhook credential remains an open owned operational follow-up. This obligation survives closure of `spec-github-development-integration`; the owner's publication handoff is not an attestation that he promised credential rotation. Closure records no credential read, rotation or revocation. This spec is its open owner, not evidence of remediation or permission for an agent to access the credential or change the server.

Existing-owner search covered the declared work folders for notifier, webhook, credential exposure and rotation. The integration spec was the only owner of this particular exposure; the release-signing and NetBox token specs concern other credentials.

Preserve the approved destination label `existing-github-activity-feed`, channel `1488613715006984342`, webhook identity `1488619854981238966`, exact approved templates, and all six workflow receipt histories. Preserve the confirmed failure message `1571668976444903494`. No token, webhook URL or private file content belongs in Git, logs or arguments.

## Required Reading

- `ai/doors/system.md` and `ai/doors/le-tooling.md`: keep private operator authority outside source and do not execute reported commits.
- `docs/contributing/github-notifier.md`, private provisioning, recovery and migration sections: stop scheduling before a state-preserving credential change; never initialize transferred history.
- `docs/contributing/github-integration.md`, approval and receipt sections: the approval digest binds exact templates and destination identities, not the token.

## Current Behavior

Source read: `internal/le/github/notification_activation.go` `loadNotificationActivation` and `readNotificationPrivateFile` load private authority and a separate bounded credential; `notification_discord.go` `newDiscordNotificationSender` checks the fixed origin and approved webhook identity. `notification_delivery.go` `checkNotificationApproval` and `deliverNotification` preserve exact approval and durable history.

Parent-supplied server evidence reports eight unchanged authority files, six ready receipts and six locks after the 2026-11-16 upgrade. That proves preservation across that upgrade, not credential rotation. The exposed credential's revocation remains unverified.

## Existing Mechanisms

| Mechanism | Where | Current behavior | Disposition |
|-----------|-------|------------------|-------------|
| Private credential boundary | `notification_activation.go` `readNotificationPrivateFile` | Owned 0600 regular file in an owned 0700 directory; bounded reads | Reuse; never copy the secret into a tracked record |
| Exact approval | `notification.go` `notificationTemplateBinding` | Binds templates and destination identities | Preserve; changed identities need fresh owner approval |
| Durable deduplication | `notification_delivery.go` `deliverNotification`; `notification_store.go` `withLock` | Durable intent, one send, confirmed history or reconciliation stop | Preserve all workflow histories; never reset or reinitialize |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Owner performs credential rotation | Record token-free confirmation that the exposed credential was revoked and its replacement is privately provisioned; no secret read by the closing agent |
| AC-2 | Rotation changes only the secret | Approved destination/template bytes and all six histories survive; retain the confirmed message identity and private permissions |
| AC-3 | Rotation changes webhook or channel identity | Obtain fresh exact approval before using the new identity and explicitly reconcile the preserved histories; do not initialize a replacement ledger |
| AC-4 | Owner resumes the notifier | Record one bounded operational result and scheduling state, with ambiguous delivery reconciled before any retry; report any active-run or quota refusal truthfully |

## Work Boundary

This is pending owner operational work. Do not start remote work, publish, push, rotate, read the webhook credential or change service state under the integration closure's authority. No implementation or test run occurred when this owner record was created.
