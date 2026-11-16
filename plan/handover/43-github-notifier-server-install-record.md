# Record: the GitHub notifier installed and upgraded on the server

Carried on 2026-11-16 from two commits that existed only in the server
checkout: `4b244eb556` (2026-11-15) and `fe3492ced9` (2026-11-16). Both added
text to `plan/post-release/engineering/spec-github-development-integration.md`.
That spec was closed and deleted in `70ddaad4c1` before they left the server,
so they could no longer be applied. The text each one added is kept below,
unchanged, so the record survives the reset of the server checkout.

"The spec" in the text below is that closed spec. Where the text says the spec
stays open, it describes the day it was written.

## Added by 4b244eb556: installed notifier and operational evidence

Operational authorization, Thomas, 2026-11-15:

> Follow docs/contributing/github-notifier.md and the owning spec
> plan/post-release/engineering/spec-github-development-integration.md.
> I authorize the native build, dedicated account, versioned executable,
> private configuration and credentials, systemd service, and 15-minute timer.
>
> Both unchanged failure/recovery templates are approved for:
> destination: existing-github-activity-feed
> channel: 1488613715006984342
> webhook: 1488619854981238966
> approval-digest: c9f08a1bed9cc3cedd3b85143f89f4913ac79dbb82fcaca6d5594cbd940d76e0
>
> Reuse the existing webhook configured in GitHub hook 603786411.
> Keep credentials out of arguments, output, and Git.
> Configure verify, evidence-nightly, qemu-nightly, perf-nightly,
> govulncheck, and CodeQL.
>
> Check the clean source revision and machine capacity. Run the actual
> Linux installer-guard proof, validate the units, initialize separate
> workflow histories, demonstrate polling and persistent delivery history,
> then enable and verify the timer. Preserve existing state and other work.
>
> Report the installed revision, binary digest, service status, and next
> scheduled run. Do not push, publish the website/news, or close the spec
> while publication remains pending.

During provisioning, an incorrect component selection in the main session's hook-identity script printed the webhook token in tool output. The main session promptly disclosed the exposure. Subsequent credential handling stayed in memory and private files. Rotation remains owed through an owner-approved route that preserves or replaces the approved destination binding deliberately. The exposed token is not recorded here or in Git. Installation evidence does not clear this credential incident.

### Installed Notifier Evidence

Observed on `claude`, 2026-11-15. The main thread checked machine capacity before the native build and preserved the existing checkout and other work.

| Scope | Observed evidence | Qualification |
|-------|-------------------|---------------|
| Source and executable | Clean revision `66f29c9c16d53a850182f6145081c55ecdb3394d`. Installed executable `/opt/ze-github-notifier/releases/66f29c9c16d53a850182f6145081c55ecdb3394d/le`. SHA-256 `248e0eaa8863cf1a41ba5d067ff74c25418a3caf20b4b825a6830dd41d0e6d2c`. | Native Go 1.27.4, Linux/amd64, `CGO_ENABLED=0`, trimpath, `ze_le`, and embedded `vcs.modified=false`. The `current` link selects this release. |
| Installer guards and units | Actual sourced Linux leaf-guard proof produced both required named PASS lines. Installer completed. `systemd-analyze verify` returned exit 0. | Dedicated `ze-github-notifier` UID 994, GID 985. No source copy or worktree was used. |
| Private authority | Exact preview approval matched the owner's digest. Existing GitHub hook 603786411 supplied the approved webhook credential. | Private configuration is under `/etc/ze-github-notifier`, with separate histories under `/var/lib/ze-github-notifier`. Observed directories were 0700 and files 0600, owned by UID 994. Credential exposure is recorded separately above. |
| Explicit initialization | All six initializations succeeded: verify `43756347664`, evidence-nightly `43707532066`, qemu-nightly `43713038523`, perf-nightly `43709239479`, govulncheck `43716289643`, CodeQL `43770146422`. | Each has its own activation file and receipt directory. No prior histories existed. Initialization created empty histories and sent nothing. |
| Catch-up boundary | Inclusive `start-run-id` is `43756347635`, a run from `2026-11-15T20:08:24Z`. | The operator deliberately selected a recent bounded catch-up within the public API quota. Older nightly runs are outside this fresh boundary. |
| Live failure delivery | Service-account poll delivered verify run `43756347664`, attempt 1, with confirmed Discord message ID `1571668976444903494`. Run: https://github.com/ze-software/ze/actions/runs/43756347664. | Verify receipt remains ready, with seen/scan run `43756347664`, attempt 1, and the confirmed failure message identity. No recovery was observed. |
| Persistent histories | CodeQL quietly scanned through `43770146422`. Four nightly receipts remain empty and ready because their latest runs precede the boundary. | A separate actual systemd service invocation at `2026-11-15T23:31:53Z` sent no repeat. All six receipt SHA-256 values were unchanged across that process. |
| Poll and service result | Both service-account poll and systemd invocation returned exit 1 only for active verify run `43766021591` and CodeQL run `43774639826`. | Service state is `failed`, result `exit-code`, status 1. Active evidence blocks those workflows without erasing delivery history. This is partial polling evidence, not a successful poll. |
| Timer | `systemctl enable --now` completed. Timer is enabled and active, waiting, with fifteen-minute calendar, persistence, and 20-second jitter. | Next run observed at `2026-11-15T23:45:10Z`. The timer remains able to reread the active GitHub results. |

## Added by fe3492ced9: the hourly notifier upgrade

Where this commit rewrote a paragraph of the spec, the rewritten paragraph is
shown.

The deployed unauthenticated poll exhausted the public source-IP quota. Its fifteen-minute schedule, active-run details/jobs, snapshot rechecks, initialization and manual checks all consumed requests. On 2026-11-15, the owner selected: "Keep unauthenticated access; reduce requests and poll less often." He also required: "Fix must be done on my laptop, not here." The selected fix retains unauthenticated reads, reduces requests and uses an hourly timer. Source changes and scoped validation were completed in the laptop primary checkout. The later instruction, "in that case, pull the code from my laptop, recompile and make sure we update machine here", authorized the server upgrade recorded below.

The earlier private bundle at `/home/thomas/tmp/ze-github-notifier-01a1a3ac/notifier-install.bundle` is a historical handoff artifact. This task does not access it or establish its current contents. Its existing owning session record retains its purpose and removal condition. That bundle and the original installed release do not carry the laptop rate fix.

The native session state owns the complete wording review, paired-identity preview, scoped test logs and publication ancestry snapshot. Laptop scoped validation, formatting and independent review of the rate delta are complete. Those test results cover the shared working tree, including concurrent CLI grammar edits, and are not a clean-commit certificate. The server upgrade has separate clean-build and installed-poll evidence below. Source and website publication remain withheld for Thomas's later step 3 decision. AC-6 deployment confirmation and final closure remain owned here.

### Hourly Upgrade, 2026-11-16

The owner authorized the private laptop pull and native server upgrade. The laptop rate fix is commit `2a500e0bfc5cc7ec4c1b2c3d3406a47bded4e78c`. A private Git bundle carried laptop main through `79f11edaef810814652d4425b3a291126fb7fb64`. The normal rebase pull preserved the server's existing commits and both installation records. No source or website push occurred.

| Scope | Observed evidence | Qualification |
|-------|-------------------|---------------|
| Build readiness | Checkout clean before and after the named native build. Capacity report: three free job slots and 23.4 GiB available memory. | Build admitted through the existing job registry in the primary checkout. No source copy, worktree, or cross-compilation. |
| Installed executable | Revision `0a992d411555e06568aeb7a3e6027a0fc616d29b`; SHA-256 `d47697d64e23de4d3af2443491fcdb76226af3f02e4a368abd8e9381d73c1408`. The `current` link selects this version under `/opt/ze-github-notifier/releases/`. | Native Go 1.27.4, Linux/amd64, `CGO_ENABLED=0`, trimpath, `ze_le`, and `vcs.modified=false`. The previous `66f29c9c16d53a850182f6145081c55ecdb3394d` release remains available for forward repair. |
| Installer and units | Both named production Linux leaf-guard proofs passed. Installer completed; `systemd-analyze verify` returned exit 0. | Existing dedicated account and private directories retained. |
| Private preservation | All eight authority files and six receipts were byte-identical across installation and the new installed poll. All receipts remained ready. | Snapshots taken under all six existing receipt locks. Private files retained service ownership and mode 0600. No initialization, reset, credential change, or history replacement. |
| Installed service poll | Systemd invocation `e2dd4ec3cec54cd2a3aa7106b021663c`, `2026-11-16T01:39:21Z`–`01:39:33Z`. No API rate-limit refusal or repeated delivery. Verify failure message `1571668976444903494` retained. | Service exited 1 solely because verify run `43766021591`, attempt 1, remained active. State is `failed`, result `exit-code`; this is incomplete catch-up, not a successful poll. CodeQL cursor was already `43774639826` before this upgrade. |
| Enabled schedule | Timer enabled and active, waiting; hourly calendar, persistence, and 20-second jitter. Next run observed at `2026-11-16T02:00:16Z`. | Nominal six quiet-feed GETs per hour and the shared 30-request bound have scoped fixture proof. This installed invocation did not measure its network request count. Other clients still share the unauthenticated source-IP quota. |

The spec stays open. The later owner-authorized server upgrade installed the laptop rate fix and hourly unit. Approved destination/templates, all activation files, and six existing histories were preserved. Installed polling remains incomplete while the earlier verify run is active; request counts were not measured in production. Credential rotation, publication, and spec closure remain open. Preserve other sessions' concurrent work and never initialize the existing receipts again.
