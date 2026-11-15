# Install maintainer result notifications

The notifier reads public GitHub Actions metadata for `ze-software/ze`. It sends
approved failure and recovery messages to one identified Discord destination.
It never checks out or executes the reported revision, starts tests, or publishes
source or website changes. Public news retains its separate approval process.

The Linux installation uses one short-lived systemd service every fifteen minutes.
The reviewed executable lives outside the development checkout; private receipt
files survive each invocation and upgrades. GitHub/API uncertainty stops the
affected work. An uncertain Discord POST stops that workflow until an operator
reconciles its attempted event and channel; it is never retried automatically.

## Build the exact reviewed revision

Use a Linux host with systemd, Go at the version pinned by `go.mod`, the existing
Ze build prerequisites, and the reviewed source already in its primary `main`
checkout. Run scoped validation there through the normal main-thread procedure.
Carry commits between development machines through the permitted private Git
route; installing this notifier grants no GitHub push permission. Do not use a
worktree, source export, or copied development tree. Do not cross-compile `le`.

Confirm HEAD is the reviewed full SHA and compiler inputs are clean before and
after building. Resolve any conflicting active work before installation. From
that main checkout, choose a unique local build name and run:

```sh
env -u GOOS -u GOARCH -u ZE_LE_BUILD_NAME GOFLAGS=-trimpath GOWORK=off GOENV=off \
  ./le --name github-notifier-install help github
```

The native launcher selects `ze_le`, the feature manifest and patched standard
library, sets `CGO_ENABLED=0`, and builds `bin/le-github-notifier-install/le`.
`le build host-driver` builds a different personality and cannot replace it.
The final executable filename must remain `le`. Inspect `go version -m` and
record the source SHA and binary digest; the installer refuses a mismatched
revision, dirty VCS record, or missing Linux/static-build metadata.
<!-- source: le -- build_le -->

Install with the Go tool on sudo's PATH. Substitute the reviewed full SHA:

```sh
sudo env PATH="$PATH" sh contrib/github-notifier/install.sh \
  --binary "$PWD/bin/le-github-notifier-install/le" --revision <full-sha>
```

The installer creates the dedicated `ze-github-notifier` account, installs a
root-owned binary under `/opt/ze-github-notifier/releases/<full-sha>/le`, records
its digest in `build.txt`, and selects it through `/opt/ze-github-notifier/current`.
It installs the service/timer and leaves the timer stopped. Existing private
configuration and receipts are preserved. A repeated install of the same SHA
requires identical binary bytes.

Before enabling an installation, run the production leaf-guard proof on Linux
using an existing task-owned scratch directory:

```sh
sudo sh contrib/github-notifier/test-leaf-guards.sh --scratch <absolute-task-scratch>
```

It exercises the same guards sourced by the installer against real isolated
root-owned controls, matching user-owned/writable executable copies, executable
aliases, and build-record aliases. Both named PASS lines are required. It never
touches installed units, accounts or private state, and removes its own fixtures.

## Provision private authority

Keep `/etc/ze-github-notifier` and `/var/lib/ze-github-notifier` owned by the
service account with mode `0700`; every authority/credential file is mode `0600`
and owned by that account. Runtime systemd protection makes configuration and
the executable read-only; only `/var/lib/ze-github-notifier` is writable.

Review the exact current templates and token-free destination identities using
the installed binary from outside the checkout:

```sh
cd /
env -u ZE_REPO_ROOT -u ZE_LE_BUILD_NAME /opt/ze-github-notifier/current/le \
  github preview run <trusted-run-id> destination <label> \
  channel <channel-id> webhook <webhook-id> '|' json
```

The preview sends nothing. Obtain owner approval of these exact template bytes
and destination identities, then copy its `approval-digest` into each workflow
activation file. A digest alone does not grant approval. A different destination
or changed template requires fresh review. The current templates are:

```text
{workflow} failed at {revision} (attempt {attempt}).
Run and logs: {run_url}
```

```text
{workflow} recovered at {revision} (attempt {attempt}) after its delivered failure.
Run and evidence: {run_url}
```

Provision the webhook URL directly into a private credential file, using a
private editor or your secret-management installation route. Do not put it in
command arguments, shell history, Git, unit files, reports, or receipts. The
file contains the URL only, with no trailing newline. The destination's current
webhook/channel identity is checked again before each message POST.

After private provisioning, set ownership/mode on the named files, for example:

```sh
sudo chown ze-github-notifier:ze-github-notifier /etc/ze-github-notifier/webhook
sudo chmod 0600 /etc/ze-github-notifier/webhook
sudo install -d -o ze-github-notifier -g ze-github-notifier -m 0700 \
  /var/lib/ze-github-notifier/verify
```

Create a separate existing `0700` state directory for each workflow, such as
`/var/lib/ze-github-notifier/verify`. Provision each workflow's private activation
JSON, for example `/etc/ze-github-notifier/verify.json`:

```json
{
  "version": 1,
  "destination": "maintainers",
  "channel-id": "<approved-channel-id>",
  "webhook-id": "<approved-webhook-id>",
  "failure-template": "{workflow} failed at {revision} (attempt {attempt}).\nRun and logs: {run_url}",
  "recovery-template": "{workflow} recovered at {revision} (attempt {attempt}) after its delivered failure.\nRun and evidence: {run_url}",
  "approval-digest": "<approved-preview-digest>",
  "credential-file": "/etc/ze-github-notifier/webhook",
  "state-directory": "/var/lib/ze-github-notifier/verify"
}
```

Use the same approved destination/template values in the preview and file.
Paths must be absolute, clean paths; files/directories must not be symlink
aliases. The actual webhook credential stays separate from this JSON.
After editing each activation file and the polling file below, give it the same
service-account ownership and `0600` mode as the credential file. Do not enable
the timer until every selected workflow has been initialized successfully.

Initialize each workflow explicitly using one trusted run from that workflow:

```sh
cd /
sudo -u ze-github-notifier env -u ZE_REPO_ROOT -u ZE_LE_BUILD_NAME \
  /opt/ze-github-notifier/current/le github initialize run <verify-run-id> \
  config /etc/ze-github-notifier/verify.json '|' json
```

Initialization creates an empty receipt without sending a message or replacing
any file. Repeat for the selected workflows using separate files/state
directories and the appropriate run IDs. The reviewed set comprises
`verify.yml`, `evidence-nightly.yml`, `qemu-nightly.yml`, `perf-nightly.yml`,
`govulncheck.yml`, and `codeql.yml`. Only Ze `main` runs from `push`, `schedule`,
and `workflow_dispatch` events qualify. Select the desired subset explicitly.
<!-- source: internal/le/github/notification.go -- notificationTrust -->

Provision `/etc/ze-github-notifier/poll.json` with the inclusive earliest run ID
to inspect and 1–6 explicit activation-file paths:

```json
{
  "version": 1,
  "start-run-id": 43726804487,
  "activation-configs": ["/etc/ze-github-notifier/verify.json"]
}
```

The number above illustrates the field shape; choose the operational boundary
you reviewed. It applies only until each workflow has its own scan progress.
Choose a recent boundary whose workflow history fits the bounded feeds.
There is no automatic bootstrap, implicit reset, or newest-success fallback.
Run one poll before enabling the timer; this can send approved transitions:

```sh
cd /
sudo -u ze-github-notifier env -u ZE_REPO_ROOT -u ZE_LE_BUILD_NAME \
  /opt/ze-github-notifier/current/le github poll \
  config /etc/ze-github-notifier/poll.json '|' json
sudo systemctl enable --now ze-github-notifier.timer
```

Keep the working directory and installed executable outside checkout ancestry,
and clear `ZE_REPO_ROOT`/`ZE_LE_BUILD_NAME`. Otherwise common `le` startup can
discover a development checkout and run its cache/toolchain preparation. The
supplied service clears those variables and runs in its persistent state root.
<!-- source: internal/le/le/path/lepath.go -- Root -->
<!-- source: internal/le/register.go -- ensureCacheLink, activatePatchedGOROOT -->

## Check operation and recover

```sh
sudo systemctl status ze-github-notifier.timer ze-github-notifier.service
sudo systemctl list-timers ze-github-notifier.timer
sudo journalctl -u ze-github-notifier.service -n 30 --no-pager
```

A successful invocation returns structured results. Missing/corrupt/private
authority errors occur before listing. Discovery reads a bounded `main` run
suffix from each initialized workflow's own feed. Quiet workflows therefore do
not turn unrelated repository activity into a backlog. Every feed is checked
before delivery starts; a feed with new candidate events has its first page
checked again before sending. Polling refuses a
changed snapshot, duplicate identity, unknown state, or page/request/byte/time
exhaustion. Discovery stops at its cursor boundary or after reading the complete
workflow feed; reaching the page bound before either condition refuses polling.
All API work shares the existing 45-second,
200-request and 32 MiB operation limits, ten pages per listing and 100 entries
per page. HTTP redirects are refused. There is no API credential.

The fifteen-minute default leaves room under the public API quota: six quiet
feeds normally need 24 listing requests per hour; if every feed has candidate
events, the first-page rechecks raise that baseline to 48, before detailed run
and job reads. Busy installations can still meet rate limits. Such a refusal
preserves state and remains visible in the journal; reduce polling frequency
when needed. No token or quota workaround is installed automatically.

Within each workflow, earlier active/incomplete evidence blocks later events;
other initialized workflows may proceed. Skipped, cancelled and complete
nonactionable results advance only the scan cursor. They cannot erase confirmed
failure history or establish recovery. Failure/timed-out evidence without
sufficient jobs stops that workflow. Original observed-run ordering still
prevents older runs or attempts reversing newer delivered state.

The receipt's optional `scan-run-id`/`scan-attempt` are independent of its
delivery observations. A single file lock covers durable intent, the one POST,
confirmation and scan publication. A crash after confirmed delivery but before
scan publication is quiet on restart because delivery history already identifies
the event. An in-flight or blocked receipt permits no further send.
<!-- source: internal/le/github/notification_poll.go -- loadNotificationPoll, discover, process -->
<!-- source: internal/le/github/notification_receipt.go -- notificationInFlightReceipt.persist -->

For an API/rate-limit problem, inspect the journal's bounded status/quota facts;
the next timer invocation rereads evidence. Keep the cursor/history intact. A
large backlog exceeding the page bound requires an explicit operator catch-up
decision; moving `start-run-id` must not discard required unscanned history.

For an ambiguous POST, stop the timer and inspect the private attempted run,
attempt, kind and destination. Check that exact event in the approved channel.
Preserve the receipt while deciding whether Discord accepted it. There is no
automatic reconciliation/reset command: prepare an explicit evidence-backed
repair for review, preserving confirmed earlier history; do not delete the
receipt or reinitialize it. Resolve the cause before reenabling the timer.

## Move an existing installation

Keep one active host for each workflow and approved destination. The receipt
lock coordinates processes on one host. Independent receipt files on two hosts
can each authorize the same message.
<!-- source: internal/le/github/notification_store.go -- fileNotificationStore.withLock -->

1. Disable the old host's timer and stop its service using the commands below.
   Confirm that both remain inactive before transferring files. Reconcile any
   in-flight or blocked receipt before the move.
2. Install the reviewed binary and units on the new host with its timer stopped.
   Transfer the current private activation files, polling file, credential and
   every workflow receipt through an approved private route. Transfer one
   consistent set from the stopped old service, preserving all delivery and
   scan history. Keep the same approved template bytes and destination identities.
3. Assign the transferred files to the new host's `ze-github-notifier` UID and
   group. Set private directories to `0700` and private files to `0600`.
   Account UIDs can differ between hosts. Validate the transferred receipts
   instead of running `initialize`. A fresh ledger for the same workflow and
   destination can repeat earlier messages.
4. Confirm that the old host remains stopped and disabled. Run one poll on the
   new host and inspect its result. Enable the new timer only after that poll
   succeeds. Keep the old timer disabled after the move.
<!-- source: internal/le/github/notification_activation.go -- loadNotificationActivation, notificationActivation.initialize -->
<!-- source: internal/le/github/notification_poll.go -- loadNotificationPoll, notificationPollReady -->
<!-- source: contrib/github-notifier/ze-github-notifier.service -- User, WorkingDirectory -->

Record the active host in the existing installation record. Keep the old private
files only until the new host is verified and every attempted delivery is
resolved. Then remove superseded files through the deliberate removal procedure.

## Upgrade, disable, and remove

Stop the timer and service, build and validate the next reviewed commit natively
in the primary main checkout, then rerun the installer with that binary/SHA.
Keep every private receipt and existing approved activation file. Recheck the
binary digest, run one poll as the service account, inspect its result, and
reenable the timer. A template/destination change needs fresh exact approval
and a deliberate history migration; it is not a routine binary upgrade.

Keep the current executable and the immediately previous reviewed executable
only while needed for forward repair. Record their SHA/digest in the owning
installation record; remove superseded versions after checking active processes
and references. A version switch must preserve state and compatible receipt
meaning; never fall back to an older receipt file.

To disable delivery:

```sh
sudo systemctl disable --now ze-github-notifier.timer
sudo systemctl stop ze-github-notifier.service
```

For removal, disable and stop first, then remove the two installed units and run
`sudo systemctl daemon-reload`. Remove `/opt/ze-github-notifier` only after
checking that no process/reference uses its binaries. Keep the private
configuration/state and locked service account until every attempted delivery
is resolved and the operator authorizes their disposal. Remove those files and
the account deliberately afterward; uninstall must not silently erase evidence.
