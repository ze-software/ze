# GitHub evidence and development handoff

Use the registered reporting command to inspect one revision, one workflow run,
or the current local checkout:

```sh
./le github report revision <full-commit-id>
./le github report run <positive-run-id>
./le github report machine
./le github report machine revision <full-commit-id>
```

Choose revision or run for GitHub evidence. Machine observations can optionally
name the revision the caller intends to test. The common pipe layer renders
the same structured answer as JSON, YAML or a table:

```sh
./le github report run <positive-run-id> '|' json
./le github report machine '|' yaml
```

A workflow conclusion describes GitHub's own result. The `complete` field
means every requested API page, identity and state was understood. A complete
report can contain failed jobs, pending work or skipped steps. Native test
logs and the verification engine's records retain their separate meaning.
<!-- source: internal/le/github/actions.go -- Actions, Answer -->

## Revision and run evidence

The reader uses public, read-only HTTPS requests for `ze-software/ze`.
It acquires no GitHub or Discord credentials. A full 40-character commit ID
qualifies a revision query; a positive run ID qualifies a run query.

Each run preserves its workflow ID, path, source branch, source repository,
commit, attempt and canonical link. Fork pull requests retain their fork source
repository while the hosting workflow remains Ze's. Jobs are read from the
selected run's attempt-specific endpoint, then checked against its run ID,
attempt and commit. Step numbers and verdicts remain separate fields.

Pagination uses explicit page numbers at the fixed API origin. The bounds are
100 entries per page, 10 pages per collection, 200 requests and 32 MiB of body
data per operation, 4 MiB per response and a 45-second overall deadline. HTTP
redirects are refused. Pagination links cannot supply another origin and are
never used as request destinations.

Missing arrays, inconsistent counts, repeated identities, unknown states and
unreadable pages make the report incomplete and return a nonzero result.
Previously observed rows remain qualified by their own identities. Rate limits
report the HTTP status and quota headers; response bodies are excluded from
errors. The exact external state vocabulary lives in `api-states.json`.
<!-- source: internal/le/github/client.go -- NewClient, reach.get, checkLinks -->
<!-- source: internal/le/github/report.go -- Client.reportRevision, Client.reportRun, reach.jobs -->

## Current local machine observations

The machine report reads the local checkout's HEAD, branch, dirty state and
native input fingerprint, together with the existing job report's capacity,
process liveness and memory reading. Source fingerprints bracket the observation
so a concurrent change qualifies the result as changed input. A dirty tree,
linked worktree, non-main branch,
revision mismatch, unreadable measurement or unavailable capacity returns a
nonzero result with the observations that were available.

`ready` describes the observation's committed-input handoff conditions. It
reserves no job slot and proves no previous run. `execution` remains
`not-started`. The main thread still records the actual native command, named
test result and source-input context when it executes a scoped test.
<!-- source: internal/le/github/machine.go -- observeMachine, localMachineSources -->

Machine coordination continues through the existing main-thread scoped-test
workflow. This integration adds no remote executor, source copy or permission
to use a worktree. Consult the current instructions and
`ai/owner/machines.md` before a machine handoff; the report does not resolve a
policy conflict or grant transport authorization.

## Website deployment identity

The site owner shares the same bounded API reader. It reads the newest
`github-pages` deployment and that deployment's newest status. The deployment
must identify `gh-pages`, a full commit ID and a positive deployment/status ID.
The published origin must be `https://ze-software.net/`.

A known unsuccessful status remains visible; an older successful deployment
never replaces it. Site confirmation compares the returned identity with the
reviewed artifact commit and performs its own live route and asset checks.
This read-only API method performs no publication.
<!-- source: internal/le/github/pages.go -- Client.LatestPagesDeployment -->

## Maintainer notification preview

Render the two candidate messages for an identified workflow run:

```sh
./le github preview run <positive-run-id> destination pending-owner-approval '|' json
```

The command reads public GitHub evidence and returns `delivery-state:
preview-only`, the selected run, a proposed destination label, both rendered
messages and an approval digest. It writes no receipt and sends no message.
The destination label alone supplies no Discord identity. To bind a proposed
destination, supply both token-free identities:

```sh
./le github preview run <positive-run-id> destination <label> channel <channel-id> webhook <webhook-id> '|' json
```

Both identities are required together. A preview computes a comparison digest
but records no owner approval. No preview acquires a credential or creates state.
<!-- source: internal/le/github/notification_actions.go -- notificationPreviewAction, answerNotificationPreview -->

The manually dispatched `notification-preview` workflow uses default-branch
code, with the selected run ID passed as quoted data. It has read-only GitHub
permissions and no Discord credential. Its JSON artifact expires after 14 days.
The evidence reader accepts only Ze's own `main` runs from `push`, `schedule`
or `workflow_dispatch` events for the reviewed verification, evidence, QEMU,
performance, vulnerability and CodeQL workflow paths. Workflow ID, attempt,
full commit ID and canonical run link remain part of the evidence.
<!-- source: .github/workflows/notification-preview.yml -- preview -->
<!-- source: internal/le/github/notification.go -- notificationTrust -->

Thomas approved both templates in `candidateNotificationTemplate` for automatic
delivery to the selected existing GitHub activity feed. Approval binds the
exact failure and recovery template bytes, destination label, channel identity
and webhook identity. Changing any bound value requires fresh owner approval.
Public news retains its separate exact-text approval procedure.
<!-- source: internal/le/github/notification.go -- candidateNotificationTemplate, buildNotificationPreview -->
<!-- source: internal/le/github/notification_delivery.go -- checkNotificationApproval -->

## Private notification configuration

The operator provisions one private configuration file per workflow. This
file is trusted operator authority, not cryptographic proof of a human's
identity. Neither initialization nor delivery issues approval. A CLI argument,
API response or computed preview digest cannot create approval by itself.
<!-- source: internal/le/github/notification_activation.go -- loadNotificationActivation -->

Use an existing directory owned by the account running the notifier, with
permissions `0700`. Keep the configuration and separate credential files at
`0600`, owned by that same account. Both files must be regular files, with no
symbolic link at the file or containing directory. Paths must be absolute and
clean. Configuration reads are limited to 64 KiB, with unknown keys and
trailing data refused.
<!-- source: internal/le/github/notification_activation.go -- readNotificationPrivateFile, notificationPrivateOwner, notificationPrivatePath -->

After exact text and destination approval, copy the identity preview's digest
into `approval-digest`. Use these configuration fields:

```json
{
  "version": 1,
  "destination": "<approved-label>",
  "channel-id": "<approved-channel-id>",
  "webhook-id": "<approved-webhook-id>",
  "failure-template": "<exact-approved-failure-template>",
  "recovery-template": "<exact-approved-recovery-template>",
  "approval-digest": "<digest-from-the-approved-identity-preview>",
  "credential-file": "/absolute/private/directory/webhook",
  "state-directory": "/absolute/private/directory/workflow-receipts"
}
```

The credential file contains only the exact Discord webhook URL, without a
newline. It is limited to 2048 bytes. Credentials never belong in command
arguments, configuration JSON, output or receipt files. Each workflow needs
its own existing private state directory. The receipt binds the workflow ID
and path, so another workflow cannot use its history.
<!-- source: internal/le/github/notification_activation.go -- notificationActivationFile, loadNotificationActivation, notificationActivation.deliver -->
<!-- source: internal/le/github/notification_delivery.go -- deliverNotification -->

## Explicit initialization and delivery

Initialize each workflow once, selecting an identified trusted run. Then
deliver one identified run at a time:

```sh
./le github receipt run <positive-run-id> config /absolute/private/directory/config.json
./le github delivery run <positive-run-id> config /absolute/private/directory/config.json '|' json
```

Initialization creates an empty receipt and sends nothing. It refuses to
replace any existing receipt, including corrupt or unresolved state. Delivery
opens existing state and attempts at most one message. It never initializes,
resets or retries state implicitly. Both commands validate approval before
reading workflow evidence. The credential is read only for delivery.
<!-- source: internal/le/github/notification_activation_actions.go -- notificationActivationActions, answerNotificationActivation -->
<!-- source: internal/le/github/notification_activation.go -- notificationActivation.initialize, notificationActivation.deliver -->

An initialization or delivery refusal carries the registered diagnostic
`github-notification-unavailable`, with the delivery outcome and the command
that explains the required private files. Missing operator authority, transport
or receipt readiness cannot produce a successful status. Credential contents
are excluded from the diagnostic and report.
<!-- source: internal/le/github/notification_activation_actions.go -- notificationActivationError, notificationActivationOperatorError -->
<!-- source: internal/le/github/register.go -- init -->

For polling behavior and the portable Linux systemd installation, see
[Install maintainer result notifications](github-notifier.md).
<!-- source: internal/le/github/notification_poll.go -- notificationPollActions -->

## Delivery history and recovery

The delivery library records a confirmed failure before it can qualify a later
result as recovery. The same failure signature remains quiet across later runs;
a changed failure can produce another transition. Recovery requires a complete,
successful run with the same job names and associated step numbers/names, and
every job and recorded step
successful. A job with no step evidence cannot establish recovery. Active,
incomplete, skipped and unknown evidence cannot establish recovery. Older runs
and already processed attempts cannot reverse a later delivered state.
<!-- source: internal/le/github/notification.go -- notificationResult -->
<!-- source: internal/le/github/notification_delivery.go -- notificationTransition -->

Each workflow's file store belongs in an existing private directory. The caller
must explicitly initialise an empty approved receipt once. Missing or corrupt
state fails closed, and initialisation refuses to replace an existing file.
An exclusive OS lock covers reading, delivery and result persistence. Atomic,
synced file publication makes the attempted event durable before the sender
runs. A crash, lost acknowledgement or failed confirmation write leaves an
unresolved receipt that prevents further delivery until it is reconciled.
The ledger boundary decodes validated private ready, in-flight and blocked
states. Only ready state can prepare an attempt; only successful intent
persistence produces the capability that calls and resolves the sender.
<!-- source: internal/le/github/notification_store.go -- newFileNotificationStore, initialize, withLock, notificationFileLedger.publish -->
<!-- source: internal/le/github/notification_delivery.go -- deliverNotification -->
<!-- source: internal/le/github/notification_receipt.go -- decodeNotificationReceipt, notificationInFlightReceipt.persist, notificationPersistedAttempt.resolve -->

The Discord sender checks the webhook's current channel before its
single message POST and requests `wait=true` to obtain a message identity.
It disables mentions and redirects, bounds request time and response bytes,
and excludes webhook URLs and response bodies from its errors. An uncertain
POST is never retried automatically. Webhook credentials belong only in the
private credential file; previews and receipt files
contain identity fields without the credential URL or token.
<!-- source: internal/le/github/notification_discord.go -- newDiscordNotificationSender, Send, notificationDiscordResponse -->

If delivery becomes unresolved, stop scheduling that workflow and preserve its
receipt. Compare its attempted run, attempt and message kind with the actual
Discord channel before any further delivery. A missing acknowledgment can mean
the message was posted. There is no reset or retry command that can settle
that uncertainty. Reinitializing or replacing the receipt would discard the
history that prevents duplicate messages. Reconcile the observed outcome and
review any receipt correction before restarting the schedule.
<!-- source: internal/le/github/notification_receipt.go -- decodeNotificationReceipt, notificationPersistedAttempt.resolve -->
<!-- source: internal/le/github/notification_store.go -- initialize, withLock -->
