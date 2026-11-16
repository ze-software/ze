# Spec: github-development-integration

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | tooling, docs |
| Priority | P2 |
| Release reason | Owner-requested development integration; this adds no first-release prerequisite or product-support claim. |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-16 |

## Task

Connect GitHub results, development-machine work, requested website publication, and Discord notifications through exact revision and artifact identities. Implementation is authorized; notification activation and publication retain the owner's review gates.

Task source, Thomas, 2026-11-03: "Can you please look at how we set up the GitHub online and see if there is things we should do to better integrate our development and make what we do clearer to end users, and automation we could do between the repository and the server. Do not change anything locally because the code is currently being edited on another machine."

Clarification, Thomas, 2026-11-03: "dev macine(s), discord and the web server."

Scope authorization, Thomas, 2026-11-15: "ok you can do all improvement but 5 - I wish to review any change to messaging myself"

Continuation, Thomas, 2026-11-15: "Okay, complete the work."

The latest AI-contribution paragraph reviewed in this chat is approved for application. Continue the integration work within the existing scope. Item 5 remains excluded. Public news retains its exact-text approval requirement, and Discord activation requires an identified destination. Publication must not silently include unrelated changes on the shared branch.

Continuation, Thomas: "Show me one. Finish two. Perform Four. Then I will consider three." This resumes alert activation and independent operational review while source and website publication remain withheld. Thomas subsequently approved both exact alert templates and selected the Linux development server `claude`, authorizing installation of the result-only notifier and schedule there. He also requires instructions for installing it elsewhere. Contribution/form wording remains available for review; the approved alert text is no longer a blocker. The notifier reads result metadata, never runs reported commits or tests, and retains durable private delivery state across restarts.

Item 5 is the GitHub front-door proposal: organization profile, repository description, README and roadmap presentation. It is excluded. Report templates, contribution instructions and notification text remain drafts until Thomas approves them. This spec adds no automatic release announcement.

Goals: G-1, show what GitHub actually tested for a revision; G-2, carry committed work between the two machines through the existing permitted routes; G-3, publish the exact requested and reviewed website artifact; G-4, prepare and, only after separate approval, deliver failure/recovery messages without repeated unchanged alerts.

The continuation also completes the existing contribution-guide and issue/PR-template work. The approved AI wording states a preference for capable recent models, never a model eligibility requirement. The canonical examples live in `CONTRIBUTING.md`; website contribution pages link that policy.

Machine integration adds identity and readiness reporting around the existing main-thread test workflow, not an automatic test runner. This task obeys the user's supplied AGENTS hard bans and creates no worktree or source copy, runs no remote test, and changes neither machine's policy. A report grants no permission to use a transport or execution context. An earlier native test result is not certified against current inputs unless an existing record actually binds them.

Existing-owner search: CI coverage and peer-harness specs concern different test mechanisms; `spec-site-rfc-recorded-data` owns RFC snapshot production, not deployment. `spec-publish-the-regenerated-command-catalog` retains its website/wiki drift and publication-responsibility obligation. This spec neither duplicates that obligation nor closes it. The browser-lab umbrella and release-distribution spec retain their separate product and release contracts.

## Blocker

The exact AI contribution paragraph, both maintainer alert templates and the existing GitHub activity feed are approved. Installation was recorded on `claude` at revision `66f29c9c16d53a850182f6145081c55ecdb3394d`. Failure run `43756347664`, attempt 1, has confirmed message `1571668976444903494`, and all six private receipts are ready and preserved. This installation evidence was not rechecked during laptop rate-fix validation. Earlier statements that no notifier was installed are superseded by this evidence.

The deployed unauthenticated poll exhausted the public source-IP quota. Its fifteen-minute schedule, active-run details/jobs, snapshot rechecks, initialization and manual checks all consumed requests. On 2026-11-15, the owner selected: "Keep unauthenticated access; reduce requests and poll less often." He also required: "Fix must be done on my laptop, not here." The selected fix retains unauthenticated reads, reduces requests and uses an hourly timer. It was implemented and validated only in the laptop primary checkout. No installed service, remote source, private authority or receipt is changed by this work. Server update requires a later owner instruction.

The selected destination remains `existing-github-activity-feed`, channel `1488613715006984342`, webhook identity `1488619854981238966`, binding `c9f08a1bed9cc3cedd3b85143f89f4913ac79dbb82fcaca6d5594cbd940d76e0`. No token belongs in this record. An earlier credential exposure was disclosed. Rotation remains open and was not performed during the laptop rate fix. Preserve the approved destination, templates and all six receipts through any later rotation or deployment.

The earlier private bundle at `/home/thomas/tmp/ze-github-notifier-01a1a3ac/notifier-install.bundle` is a historical handoff artifact. This task does not access it or establish its current contents. Its existing owning session record retains its purpose and removal condition. Do not treat that bundle or the installed binary as carrying this laptop rate fix.

Canonical contribution HTML and Markdown have been regenerated through the native docs producer and checked. No site artifact was committed or deployed. Publication must preserve the requested source/artifact identities and obtain an explicit owner push instruction for the exact shared-branch ancestry; unrelated README changes remain excluded from this task's authorization.

The native session state owns the complete wording review, paired-identity preview, scoped test logs and publication ancestry snapshot. Laptop scoped validation, formatting and independent review of the rate delta are complete. The evidence covers the shared working tree, including concurrent CLI grammar edits, and is not a clean-commit certificate. Deployment of the rate fix and installed operational proof remain pending. Source and website publication remain withheld for Thomas's later step 3 decision. AC-6 deployment confirmation and final closure remain owned here.

## Implementation Evidence

| Scope | Current evidence | Qualification |
|-------|------------------|---------------|
| GitHub revision/run and local readiness reports | Registered-action, pagination, identity, state, bounds and machine-input tests pass; live CodeQL run 43726804487 report and preview succeed | API observations do not certify native test or release results. No remote machine execution was added. |
| Publication receipt and confirmation mechanics | Registered positive/negative tests cover source races, changed bytes, raw committed blobs, index/filter/replacement tricks, live mismatches and zero-network refusal | Actual deployment confirmation waits for an approved artifact commit and publication. |
| Notification activation and polling | Earlier local evidence: 41 named notification bodies and six terminal-state subcases passed, with scoped GitHub lint on host and Linux. The installed revision, confirmed failure message and six ready receipts are recorded above | Earlier checks and installation evidence are distinct from the laptop rate-fix results below. No installation changes were made during rate-fix validation. |
| Unauthenticated rate fix | Laptop changes select hourly polling, a 30-GET per-poll cap, validated active barriers before details/jobs and snapshot rechecks only for reachable completed candidates. Formatting is complete | Scoped tests, vet, lint and independent review passed as qualified below. Deployment, publication and credential rotation remain pending. |
| Full GitHub package race tests | Native `test unit pkg path ./internal/le/github/` exited 0 with the race detector. Supplemental log: `job-unit-pkg-545f962c.log` | Laptop shared-working-tree evidence, including concurrent CLI edits. Not a clean-commit or installed-fix certificate. |
| Final polling regressions | After the test-helper lint corrections, all 27 named `TestNotificationPoll` tests in `internal/le/github/notification_poll_test.go` and `notification_poll_rate_test.go` passed with `-race -tags ze_core`, including all 12 new regressions | This final run excludes the BGP reactor. It proves the selected polling tests, not a new minimal full-`le` personality or whole-tree build. |
| Scoped static validation | Registered native `exec go vet -tags ze_core ./internal/le/github` exited 0. `go lint run scope ./internal/le/github/...` reported zero issues on both host and Linux/integration passes | Three unreachable nil/nil returns and one redundant transport type declaration in the new fixture were corrected. Fatal assertions and test assertions were unchanged. |
| Independent rate-fix review | Review found a concurrent scan advance that could bypass the discovered active barrier, and a missing test for budget refusal during a snapshot GET. Both were fixed and the reviewer reported clean | `notificationPollScope.process` now retains the discovered barrier before its scanned shortcut. The controlled race and request-31 snapshot tests passed in the final polling run. No spec closure is claimed. |
| Descriptor refusal discrimination | Native FIFO test passed clean, then failed in 10 seconds under the overlay removing only `O_NONBLOCK`, with its own blocked-reader diagnostic; no source was changed | Proves the receipt acquisition regression; not a whole-tree or installed-host certificate. |
| Approved destination preview | Real paired-identity preview succeeded with approval binding `c9f08a1bed9cc3cedd3b85143f89f4913ac79dbb82fcaca6d5594cbd940d76e0` | Preview-only, no send and no credential read. |
| Portable installation documentation | Versioned installer, shared leaf guards, isolated refusal proof and systemd units were committed. The rate delta updates the runbook and shipped timer for hourly polling. `TestNotificationPollHourlyTimerKeepsPersistence` passed | The test checks the shipped calendar, persistence and jitter. The installed schedule was not changed or revalidated. |
| Full documentation check | The rate-fix `doc check all` run exited 1 with 389 published wiki/gh-pages command-catalog drift findings. No notifier-guide path was mentioned. An earlier integration run recorded 720 findings | Not an all-documentation pass. `plan/post-release/engineering/spec-publish-the-regenerated-command-catalog.md` owns the separate pending publication debt. No website/wiki render or publication is authorized here. |
| Contribution policy and intake | Exact approved AI policy guard and canonical-link render guards pass; issue/PR sources prepared | Non-AI messaging and online publication remain subject to owner review. |
| Installed command paths | Both new UI carriers passed 80 actual named invocations under load and passed again in the live suite | The full UI suite also found 15 other failures and 20 skips; it is not a broad green certificate. |
| Canonical review site | Native docs-producer render and site check pass with 1118 claimed routes; both contribution HTML/Markdown pairs contain the policy link | Rendering grants no approval or publication authority. |

Rate-fix validation ran on the laptop. Canonical commands and the named test files above identify its scope; `tmp/session/2026-11-15-143107/scratch/job-github-notifier-rate-validation-56b36988.log` is a supplemental execution log, not the sole proof reference. The existing native tool `bin/le-github-notifier-rate-laptop-20261116/le` executed the registered checks after concurrent BGP edits temporarily blocked a fresh full launcher bootstrap. Its earlier native build passed. The final BGP-free package checks do not certify a new full launcher build. No `./le verify` command was run.

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md`: → Constraint: development tools register actions; the daemon acquires no development-only dependency.
- [ ] `ai/owner/machines.md`, `ai/META-DECISIONS.md`: → Constraint: git-only machine synchronization; scoped tests run in main; no agent starts `./le verify`; no permanent worker or test copy is added.
- [ ] `docs/contributing/running-commands.md`: → Decision: preserve native test grammar, main-thread job admission and session-owned logs.
- [ ] `docs/contributing/gh-pages.md`, `website/AI.md`: → Constraint: render on request into canonical `../gh-pages`; review precedes tests and publication; CI does not generate the site.
- [ ] `ai/skills/ze-weekly-update.md`: → Constraint: exact news chunks need approval, partial sends resume explicitly, and site/news source revisions remain fixed.
- [ ] `ai/rules/cli.md`, `ai/rules/repo-maintenance.md`, `ai/rules/spec-no-code.md`: → Decision: registered keyword/value actions, structured answers, discoverable diagnostics and no code snippets in this design.
- [ ] `plan/learned/DESIGN-HISTORY.md`, engineering-practice and testing sections: → Constraint: retain existing admission and evidence distinctions rather than add a competing scheduler or verdict.

Key insight: GitHub job conclusions, scoped development tests, full-verification records, site build records, owner approval and successful publication answer different questions. None substitutes for another.

## Current Behavior

**Source files read:**
- [ ] `internal/le/job/job.go` (`Admission.RunContext`), `registry.go` (`scanAndClaim`, `shares`): admission and qualified job results.
- [ ] `internal/le/verify/engine/artifacts.go` (`writeRunArtifacts`, `writeFailureIndex`): engine records, which current CI shards do not emit.
- [ ] `internal/le/site/build.go` (`Build`), `actions.go` (`runCheck`), `producer.go` (`writeProducerRecord`): rendering, artifact checks and route ownership.
- [ ] `internal/le/weekly/poster.go` (`publish`), `send.go` (`send`, `sendOne`, `archive`): preview, delivery and exact sent archive.
- [ ] `internal/le/setup/actions.go`, `.github/workflows/verify.yml`, `.github/workflows/preview.yml`: setup mode declarations, native verification loop and retained preview artifacts.

Preserve native admission, test selection, interrupted/unreached/carried evidence, site rendering and checking, exact news approval, bounded transport retries and explicit send resumption. Preserve the branch-based Pages deployment and all existing owner-only publish gates.

Change: GitHub has no common revision report through `le`; current verify shards run native actions individually and do not emit the engine failure index. Site build returns a source digest, while its producer record holds artifact path and route claims only. Neither record binds owner approval to the final bytes. Existing machine commands require manual coordination; this spec documents and reports that path without installing a worker.

### Unauthenticated polling rate correction

The existing public reader in `internal/le/github/client.go` `reach.get` bounds each operation at 200 requests. Polling in `notification_poll.go` shares one reader across workflow discovery, snapshot rechecks, detailed runs and paginated jobs. The shipped timer previously ran every fifteen minutes. This fix lowers only the poll's request limit to 30 and changes its shipped calendar to hourly, retaining persistence and 20 seconds of jitter. The separate report limit stays 200. No credential, new service or private-state migration is introduced.

Feed entries retain the existing identity, state, workflow, branch and trust checks. Discovery selects trusted entries oldest first within each workflow. A workflow needs a first-page snapshot recheck only if its oldest unscanned selected entry is completed. If that entry is active, processing reports the barrier without detail/job reads or receipt writes. Later completed entries cannot overtake it. All needed snapshot rechecks still finish before any processing or sender construction. Completed candidates retain detailed identity validation, full job/step evidence, approval and durable delivery checks. A detail response that became active stops before jobs.

With one page per feed, six quiet or oldest-active feeds use exactly six GitHub GETs per poll. The hourly schedule makes that nominally six per hour, compared with 24 quiet listing reads per hour previously. The old active path added snapshots, details and jobs. Completed work adds one snapshot recheck per affected workflow, one detail per candidate and every required job page, all inside the same 30-request budget. Request 30 is allowed; request 31 is refused before HTTP. Passing laptop regressions confirm these per-poll request counts and boundaries. The hourly savings remain nominal, not observed production savings or a guarantee about the shared source-IP quota. Manual invocations, timer catch-up and other public clients share that quota.

Budget exhaustion is incomplete. Discovery or recheck failure changes no receipt. Processing exhaustion retains all earlier confirmed deliveries and scan progress, without advancing the incomplete candidate or its suffix. A later poll resumes from retained history. If discovery and rechecks alone consume the budget, repeated polls can make no progress. That is a visible catch-up limit requiring an explicit operator decision, not permission to discard unscanned history or relax snapshot protection.

## Existing Mechanisms

| Mechanism | Where | Current behavior | Disposition |
|-----------|-------|------------------|-------------|
| Job admission | `internal/le/job/job.go` `Admission.RunContext`; `registry.go` `scanAndClaim`, `shares` | Per-checkout capacity, matching-input attachment, cancellation and result | Reuse unchanged; no second scheduler. |
| Machine transport | `ai/owner/machines.md`, both direction sections | Git over SSH; tests via SSH or Rex in the destination main checkout | Document and report; no permanent runner, copying or remote file edits. |
| Verification record | `internal/le/verify/engine/artifacts.go` `writeRunArtifacts` | Commit-qualified stages, interrupted/unreached states and carried history | Preserve; do not synthesize this record from GitHub conclusions. |
| GitHub verification | `.github/workflows/verify.yml`, native-stage loop | Six job conclusions and per-job logs; stages run separately | Read API job results; add no full verifier invocation. |
| Site artifact | `internal/le/site/build.go` `Build`; `producer.go` `writeProducerRecord` | Requested rendering, digest and route ownership | Extend check output with a final-byte publication receipt. |
| Artifact check | `internal/le/site/actions.go` `runCheck` | Source-only, missing-artifact/mirror, figure and coverage checks | Extend the structured answer, retain every existing finding. |
| Weekly publisher | `internal/le/weekly/poster.go` `publish`; `send.go` `sendOne`, `archive` | Preview, confirmation, bounded retry, partial-send handling and exact archive | Reuse for user news; maintainer alerts have separate reviewed templates. |
| Bounded public API reader | `internal/le/github/client.go` `start`, `reach.get` | One request/byte/time budget spans paginated reads | Extend the existing operation limit with a poll-specific value of 30. Preserve the independent report limit of 200. |
| Workflow polling and private receipts | `internal/le/github/notification_poll.go` `discover`, `process`, `readPreview`; `notification_receipt.go` | Validated feeds, pre-send snapshot checks, full terminal evidence, independent scan and confirmed delivery history | Avoid requests for unreachable active evidence. Retain all trust, snapshot, terminal and durable delivery checks. |

## Data Flow

### Entry Point

New development-tool grammar is proposed, not implemented: `./le github report revision <full-sha>` reads all relevant GitHub runs for one revision; `./le github report run <run-id>` reads one run with its jobs. Exactly one selector is required. Rendering uses the common pipe layer, including JSON, YAML and table. The new area registers through `leroot.Register` and `RegisterActions`, as existing `job` does.

Machine work keeps existing entry points: git synchronization, `./le job report` and a named native scoped test such as `./le test unit pkg path <package> run <test-regex>`. Website work keeps `site build`, producer-specific review rendering, `site check`, and the existing `le commit` create/push route. No new remote-execution CLI or automatic commit/push action is proposed.

### Transformation Path

1. The report resolves a full revision once, queries paginated runs/jobs for `ze-software/ze`, and returns their canonical API identities, head SHA, attempt, workflow/job/step names, states and links. A job is not a whole-tree certificate. Missing, cancelled, skipped, pending, failed and unreadable results stay distinct. Unexpected API values are unknown, never success.
2. The main session selects the scoped test and target machine under the current machine policy. It commits first, checks the destination is clean, and uses the permitted git pull route. It verifies the resulting HEAD equals the requested SHA before the test. A mismatch or dirty destination stops, without stash, reset, branch switching or a source copy. The main thread reads destination job capacity and runs one CPU-intensive job at a time. Local dirty-input evidence stays separate from committed-revision evidence; exact source inputs and changes during a run qualify its result.
3. On an owner request, render into canonical `../gh-pages` and let Thomas review there. After approval, run the existing site checks. Extend their structured answer with source HEAD, source digest, final artifact digest and check result. A receipt is mechanical evidence only; it never grants approval. Keep it in the main checkout's owning site record, outside the served tree. Before publication, recheck the final digest, commit only that artifact through `le commit`, then push only on the owner's instruction. Confirm the Pages deployment SHA and live changed routes/assets. A recovery is a new normal commit of the prior approved artifact, never a destructive git operation.
4. A new manually dispatched, read-only notification-preview workflow selects a run ID, reads trusted GitHub job results and renders candidate messages as a retained artifact. It checks out or executes no reported revision. Until Thomas approves the exact template and destination, it sends nothing. A later approved activation reads completed trusted default-branch workflow results, serializes delivery, compares the last confirmed delivered state, and sends only failure/recovery transitions. Ambiguous delivery stops for reconciliation; no blind retry claims success.

### Boundaries Crossed

| Boundary | How | Verification |
|----------|-----|--------------|
| Tool to GitHub | Fixed public-repository REST reads, pagination, bounded response/time | API fixtures for multiple pages, rate limits, malformed data and wrong SHA/run identity. |
| Development machine to peer machine | Existing git/SSH/Rex routes only | Recorded command plan plus fake transport tests; real feature tests remain main-thread work under machine policy. |
| Source checkout to website artifact | Existing renderer and final-byte receipt | Changed-byte negative test after review/check. |
| Artifact commit to public Pages | Existing branch deployment and read-only confirmation | Wrong deployment SHA and failed route/asset probes remain failures. |
| Trusted workflow to Discord | Preview first; approved channel-scoped secret only after activation | Recorder transport tests, transition state and ambiguous-send cases. |

### Changed-Wire Census

**Search evidence:** Scope: `internal/le/verify/engine`, `internal/le/site`, `internal/le/weekly`, and `.github/workflows`. Terms: `artifactIndex`, `writeRunArtifacts`, `writeProducerRecord`, `readProducerRecord`, `publish`, `sendOne`, `archive`, `upload-artifact`, and `STAGE_LIST`. Positive controls: the current preview workflow uploads an artifact, and the verify workflow lists and runs native stages. The source reads found the engine index writer, site claim writer/reader, and weekly preview/send/archive consumers described below. No daemon, plugin, SDK or routing-protocol wire changes.

| Wire | Role | Carrier | Source evidence | Disposition |
|------|------|---------|-----------------|-------------|
| GitHub REST run/job data | writer | GitHub Actions API, outside repository | Current API responses contain run/job identities, head SHA, attempt, status and conclusion. | Read-only external source; never execute its payload. |
| GitHub REST run/job data | reader | Proposed `internal/le/github/report.go` | Existing verify jobs supply API verdicts; no engine index is produced by the loop. | New bounded reader; API values stay distinct. |
| Engine artifact JSON | writer/reader | `artifacts.go` `writeFailureIndex`, `readEarlierIndex` | Current commit, stage and carried-result fields were read. | Unchanged; not forged or imported as a CI certificate. |
| Site producer record | writer/reader | `producer.go` `writeProducerRecord`, `readProducerRecord` | Record contains artifact path and route claims. | Preserve schema; add a separate final-byte receipt to the owning site record. |
| Site publication receipt | writer | Proposed extension of site check answer | `BuildReport.SourceDigest` exists separately from route claims. | New explicit source/final-byte/check identity; not approval. |
| Site publication receipt | reader | Publication preflight and deployment confirmation, proposed site-owned code | Current `runCheck` checks artifact content but does not bind approval. | Reject changed bytes, wrong source or deployment SHA. |
| Discord message | writer/reader | Proposed preview/alert workflow and Discord webhook; existing weekly `publish`/`sendOne`/`archive` remains separate | Existing publisher produces exact chunks and records successful completion. | Reviewed maintainer template; preserve news path and credential boundary. |

### Integration Points

The new report area registers through the existing development action tables. Machine plans reuse `Admission.RunContext` and the documented SSH/Rex routes. Site check integration retains `producerRecordPath` as the artifact-keyed namespace owner; its new publication sidecar is `tmp/site/publication-<artifact-key>.json`, not a second producer-claim record or a file in `../gh-pages`. Weekly message sending retains `publish`, `sendOne` and `archive`.

### Architectural Verification

| Check | Holds? | Evidence or planned proof |
|-------|--------|---------------------------|
| No bypassed layers | Yes, by design | Native test, site and commit routes remain entry points. |
| No unintended coupling | Yes, by design | All new code is development tooling; daemon imports stay unchanged. |
| No duplicated functionality | Yes, by design | API report reads real verdicts; machine transport and weekly publishing are reused. |
| Zero-copy | N-A | Bounded metadata and rendered messages are not routing hot paths. |
| Outbound registration | Yes, by design | Proposed GitHub area uses existing action/shape registration. |
| Inbound registration | Yes, by design | Search found no existing GitHub tool area; one blank import discovers the new owner. No core switch or feature-name list changes. |

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | The two documented machine routes remain usable | `ai/owner/machines.md` | Coordination stops | Check connection only as part of an authorized feature test | Not probed; no remote service changes authorized by this design. |
| A-2 | Pages continues serving `gh-pages` | Live read-only Pages settings during review | Confirmation needs a different deployment identity | Read settings before publication | Confirmed for review; recheck at use. |
| A-3 | A maintainer channel and template can be approved | Current owner request reserves messaging review | Alert delivery remains preview-only | Thomas approves exact template and destination | Existing GitHub activity feed selected and identity verified; exact template review remains pending. |
| A-4 | Public Actions reads suffice initially | Repository is public | Rate-limited results become unknown | API failure fixtures and actual read-only report | Confirmed: native report and preview read run 43726804487 without credentials; bounded failure fixtures pass. |

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | A different or dirty tree is reported as tested | HEAD/input mismatch | Stop or label working-tree evidence; never copy a certificate. |
| R-2 | API pagination hides failed jobs | Incomplete page chain | Read every page; partial API results remain incomplete. |
| R-3 | Reviewed site bytes change before publication | Digest mismatch | Require fresh review/check; publish nothing. |
| R-4 | Alert state loses or duplicates delivery | Missing receipt or ambiguous response | Serialize; record confirmed state; stop ambiguous delivery for reconciliation. |
| R-5 | Untrusted workflow gains credentials | Wrong workflow/ref/repository | Fixed trusted source and least-privilege environment; never execute candidate code. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if wrong? | Misstated evidence, stale website bytes or duplicate messages; no router runtime path changes. |
| Recovery | Disable the new notifier; correct reports forward; republish the prior approved artifact through a new ordinary commit. No reset, force push or automatic message deletion. |
| Shared owners | CI safeguards are separate work in this session; RFC site recording and command-catalog publication remain owned by their existing specs. |

## Wiring Test

| Entry Point | → | Feature path, proposed | Test |
|-------------|---|------------------------|------|
| `le github report revision` | → | Registered action to bounded API report | `TestGitHubReportRevisionReachesAPI` |
| Existing scoped-test handoff | → | Main-thread transport plan to native test and qualified result | `TestMachineHandoffRunsOnlyMatchingCleanRevision` |
| `le site check` publication receipt | → | Existing check to final-byte digest, preflight and deployment identity | `TestSitePublicationChecksReviewedBytesAndDeployment` |
| Notification preview workflow | → | Selected trusted run to retained candidate message | `TestNotificationPreviewNeverSends` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Revision has several paginated runs and shards | Report includes every API result with SHA, attempt, job state and links | Pending/skipped/cancelled results remain visible; they are not passing tests. |
| AC-2 | Wrong SHA, unknown status or API failure | Report states the unresolved identity/data and returns a nonzero result | Previously known results may be shown only with their own identity and incomplete qualification. |
| AC-3 | Requested machine holds the matching clean revision and has capacity | The existing main-thread scoped-test workflow and registered machine report provide command/result evidence plus actual HEAD, input, machine and capacity facts; inability to bind an earlier result to those inputs is explicit | The report starts no test or remote work. The other architecture owes no duplicate run. |
| AC-4 | Destination is dirty, mismatched or unreachable | Preflight reports the exact stop reason and executes no test or destructive git command | Existing unrelated work stays intact. |
| AC-5 | Owner requests site generation and approves final pages | Canonical artifact passes existing checks and gains an exact source/final-byte receipt | Local review renders stay unpublished until an explicit push instruction. |
| AC-6 | Approved publication reaches Pages | Confirmation matches the artifact commit and verifies changed live routes/assets | Unchanged routes may retain their existing publication identity. |
| AC-7 | Bytes change or deployment/live probe differs | Publication/confirmation returns failure with the mismatched identity | Previous deployment remains identifiable for normal-commit recovery. |
| AC-8 | Preview requested before messaging approval | Preview artifact contains rendered candidate messages, selected run identity and proposed destination; its delivery state is preview-only | Existing approved weekly/manual notifications remain unchanged. Preview delivery stays inactive until the exact template and destination are approved. |
| AC-9 | Approved notifier sees failure, repeated failure, then recovery | Sends one failure transition and one recovery with exact GitHub links | Unchanged failures produce no extra message. |
| AC-10 | Transport result is ambiguous or template changes | Delivery report names the attempted event, destination and unresolved result; a changed template produces a fresh identified preview for approval | Confirmed earlier deliveries remain recorded; ambiguity does not authorize a retry or a success claim. |
| AC-11 | Contributor reads the guide or website contribution pages | The approved AI paragraph is present verbatim in the canonical guide, with a nonmandatory model preference; website contribution pages link the canonical examples | Item 5's profile, README, tagline and roadmap wording remain outside this task. |
| AC-12 | Contributor opens an issue or prepares a PR | Structured bug, migration and documentation forms and the PR template request relevant evidence and preserve the private security route and existing contribution obligations | Blank issue reports remain permitted; questions can use the existing Discord link. |
| AC-13 | Six quiet/active feeds, completed events around an active barrier, or public request-budget exhaustion | Six single-page quiet/active feeds use exactly six GitHub GETs and preserve receipts. Reachable terminal failures and later recoveries still deliver with complete evidence. One poll permits 30 GitHub GETs and refuses the next as incomplete | Active, malformed, raced or incomplete evidence cannot advance history. Completed events behind an active barrier remain unscanned. Prior confirmed deliveries and all unscanned history survive a budget refusal. Other clients can still exhaust the shared quota. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Planned file | Validates |
|------|--------------|-----------|
| `TestGitHubReportRevisionReachesAPI`, `TestGitHubReportRunReachesAPI` | `internal/le/github/report_test.go` | Real registered action, selectors and all pages; positive output is required. |
| `TestGitHubReportRefusesWrongIdentityAndIncompleteAPI` | same | AC-2, malformed/unknown states, rate limit, missing page and bounded responses. |
| `TestMachineHandoffRunsOnlyMatchingCleanRevision` | `internal/le/github/machine_test.go` | Approved existing-command plan plus clean/mismatched/dirty/unreachable cases, using fake transports. |
| `TestSitePublicationChecksReviewedBytesAndDeployment` | `internal/le/site/publication_test.go` | Matching artifact/deployment, then changed bytes, wrong source and failed live asset. |
| `TestNotificationPreviewNeverSends`, `TestNotificationTransitionsAndAmbiguousDelivery` | `internal/le/github/notification_test.go` | Preview reaches rendering; approved send, repeated failure, recovery and ambiguous-send stop. |
| `TestNotificationWorkflowReadsTrustedResultsOnly` | `internal/le/workflowcheck/github_notification_test.go` | No candidate checkout/execution, fixed trust source, bounded permissions and template approval. |
| `TestNotificationPollSixFeedsUseOnlyDiscovery`, `TestNotificationPollCompletedBehindActiveCostsOnlyDiscovery`, `TestNotificationPollDiscoveredActiveBarrierSurvivesConcurrentScan` | `internal/le/github/notification_poll_rate_test.go` | Exactly six quiet/active GETs, unchanged reopened receipts, no details or rechecks for unreachable completions. A concurrent receipt advance cannot remove the discovered barrier. A fresh poll can progress after completion. |
| `TestNotificationPollFailureBeforeActiveThenRecovery`, `TestNotificationPollMalformedActiveRefusesDiscovery`, `TestNotificationPollCompletedDetailBecomesActive` | same | Reachable failure and later recovery, full trust/state refusal, and detail race before jobs. |
| `TestNotificationPollRequestBoundaryRetainsHistory`, `TestNotificationPollDiscoveryBudgetLeavesAllReceipts`, `TestNotificationPollSnapshotBudgetLeavesAllReceipts`, `TestNotificationPollSnapshotRequest31LeavesAllReceipts` | same | Request 30 versus 31 across details, paginated jobs, discovery and rechecks. Preserve confirmed and unscanned history, then resume. Request 31 can be the recheck itself, with zero processing or mutation. |
| `TestNotificationPollBudgetDoesNotChangeReportBudget`, `TestNotificationPollHourlyTimerKeepsPersistence` | same | Independent report still allows 200 reads. Shipped calendar is hourly with persistence and existing jitter. |

Boundary inputs: require a full 40-character commit ID and positive run/attempt IDs; test zero, negative/dash-leading and malformed values. Test the last configured API-page/response limit and the first rejected value; pagination exhaustion is incomplete, never a truncated pass. Interop is N-A: no routing protocol changes. Proposed functional carrier `test/ui/le-github-report.ci` exercises the registered command against a local API fixture; its author must read `ze-test` before creating it. Implementation Evidence records executed coverage, including all 27 polling tests and the 12 new rate-fix regressions. Planned coverage without a recorded execution result remains unverified.

## Files to Modify

- `internal/le/register.go`: discover the new registered development-tool owner.
- `internal/le/site/actions.go`, `internal/le/site/producer.go`: expose final-byte receipt/check integration without changing route claims or running checks during build.
- `docs/contributing/running-commands.md`, `docs/contributing/gh-pages.md`, `docs/architecture/testing/ci-workflows.md`: document evidence, requested publication and preview boundaries after wording approval.
- `ai/INDEX.md`, `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md`: expose the owner, commands and governing page.

## Files to Create

- `internal/le/github/register.go`, `actions.go`, `report.go`, `report_test.go`: proposed registered API report owner and bounded reader.
- `internal/le/github/machine.go`, `machine_test.go`: proposed pure machine-command planning/result qualification; no installed worker or new remote-execution action.
- `internal/le/github/notification.go`, `notification_test.go`: proposed preview/transition/receipt logic with a fakeable sender.
- `internal/le/site/publication.go`, `publication_test.go`: proposed exact-byte publication receipt and deployment confirmation.
- `.github/workflows/notification-preview.yml`, `internal/le/workflowcheck/github_notification_test.go`: preview-only workflow and its policy proof.
- `test/ui/le-github-report.ci`, `docs/contributing/github-integration.md`: functional entry-point proof and the source-anchored integration contract, with wording approval before activation.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema, native constraints, custom validators | N-A | Development tooling only; no daemon config or RPC. |
| CLI commands and keyword grammar | Yes | Proposed `github` action table; selector before SHA/run ID; no positional free-form shell input. |
| Editor autocomplete | N-A | No config editor leaf. Help/completion derive from registered development actions. |
| Functional new-command proof | Yes | Planned UI carrier reaches report action and local API fixture. |
| Pipe completeness | Yes | Register answer shape and use common JSON/YAML/table and declared row operators. |
| Env registration | N-A initially | Public API reads need no new local credential variable; workflow token/approved webhook secret stay workflow-scoped. |
| Doctor runtime-dependency and problem codes | Yes | Integration owner must register readiness diagnostics for GitHub API/rate-limit failure, missing approved notification destination and unavailable authorized transport. Reuse existing machine/test prerequisites; no daemon probing of private developer services. |
| Prometheus metrics | N-A | Reports and delivery receipts are development records, not daemon telemetry. |
| BGP family surface | N-A | No SAFI, capability, attribute or protocol behavior. |

### Documentation Update Checklist

| # | Question | Applies? | File / reason |
|---|----------|----------|---------------|
| 1 | Product feature declaration | N-A | No Ze capability or support-level change. |
| 2 | Config syntax | N-A | No configuration change. |
| 3 | CLI reference | Yes | Development commands in `docs/contributing/github-integration.md` and running-commands; daemon command reference is unaffected. |
| 4 | API/RPC | N-A | Read-only external REST client; no daemon API. |
| 5 | Plugin | N-A | No runtime plugin. |
| 6 | User guide | Yes | Contributor integration page; wording waits for review. |
| 7 | Wire format | N-A | No Ze protocol wire changes; external metadata and receipt contract are in this spec. |
| 8 | SDK/process protocol | N-A | No external plugin contract. |
| 9 | RFC behavior/proof | N-A | No RFC coverage claim. |
| 10 | Test infrastructure | Yes | CI-workflows and integration page describe read-only reporting and preview policy. |
| 11 | Daemon comparison | N-A | No feature-parity change. |
| 12 | Architecture | Yes | New source-anchored integration contract; preserve website/AI and core development boundaries. |
| 13 | Route metadata | N-A | No routing metadata. |
| 14 | Prometheus | N-A | No metric added. |
| 15 | Registered inventory | Yes | New development area/action discovery in `ai/INDEX.md`; no daemon plugin inventory. |
| 16 | Existing source anchors | Yes | Re-read site `Design:` headers and derive anchors with the native spec citation action before implementation; site pages named above are required owners. |
| 17 | Existing examples | Yes | Verify machine, site and weekly command examples against current action tables; preserve approved public wording. |

## Implementation Steps

1. Wiring: register report selectors and answer shapes; write the failing entry-point tests and local API fixture before the reader.
2. Read/report: implement paginated identity-qualified GitHub results, then document the existing owner-started machine plan and its fake-transport negatives. Publish no local result as a commit certificate when source inputs differ.
3. Requested website: add exact-byte receipt/check and deployment confirmation around existing rendering, review, commit and Pages paths. No CI generation or private review server.
4. Notifications: deliver the preview-only workflow and recorder tests. Review the exact template and destination with Thomas before a separate activation edit; retain weekly exact-text approval.

### Mechanical, Review and Delivery Checks

| Check | Evidence required |
|-------|-------------------|
| Discovery | Index and code/doc maps resolve the new owner and every action. |
| Drift prevention | Registered action tables, authoritative API job states and one final artifact digest; no copied suite or feature inventory. |
| Correctness | Each AC has a positive path and its near-miss negative; skipped/unknown/shard results never become full passes. |
| Authorization | Machine commands obey current owner policy; approved notifier activation uses private operator authority; Thomas performs the private source pull before installation; site receipt grants no push approval. |
| Credential security | No candidate execution with a webhook; least-privilege trusted workflow; secrets stay out of reports, logs and site bytes. |
| Retention | CI preview reports use 14-day retention. Keep current publication receipt and prior recovery identity in the existing owning record; no accumulating dated archives. |
| Delivery | Main-thread scoped tests run once in main after job-capacity review; inspect named test output. No `./le verify` action is owed. |
| Failure routing | Wrong AC returns to design; implementation mismatch is fixed in its phase; three failed fixes return to Thomas with evidence. |

## Key Design Decisions

| Decision | Alternatives considered | Reason |
|----------|-------------------------|--------|
| Owner-started existing machine commands | Permanent self-hosted runner | Shared active checkouts and current access rules require deliberate scoped work; a service adds policy and unattended execution. |
| Read API job verdicts first | Reconstruct engine certificates from logs | API results exist now; current native shards do not emit engine records. |
| Requested local site artifact | CI site generation | Preserves the explicit owner rule and canonical review path. |
| Preview-first maintainer alerts | Activate messages while preparing templates | Thomas reserved messaging approval; unchanged failures must stay quiet. |

## Known Limitations and Review Gate

Earlier implementation and portable documentation were committed; the existing feed, exact alert templates and installation host are approved. Independent review covered that earlier source and its receipt/installer corrections. Installation and confirmed failure delivery were recorded, followed by public-quota exhaustion. The laptop rate delta now has passing scoped race tests, vet, zero-issue host/Linux lint, completed formatting and clean independent review after the race and snapshot-test corrections. The whole documentation check still fails on the separately owned published catalog drift. These are shared-working-tree results, including foreign CLI edits, not a clean-commit or installed-fix certificate.

The spec stays open. Concurrent CLI grammar work must be preserved independently of the rate-fix scope. No server update is authorized by this laptop work. After a later owner instruction, deploy the reviewed revision and hourly unit while preserving all existing activation files, approved destination/templates and six receipts. Do not initialize those receipts again. Confirm actual API costs, preserved confirmed delivery and the installed schedule. Credential rotation, installed operational proof, publication and spec closure remain open.

AC-6 remains deliberately pending Thomas's separate step 3 decision. No source/site push or live Pages confirmation is authorized by this hand-back. There are no delegated journal rows or transferred obligations. Item 5 remains excluded, and public news retains its separate exact-text approval process.

## Critical Review Checklist

| Surface | What to verify |
|---------|----------------|
| GitHub report | Registered entry points read every required page; revision, attempt and workflow/job identity remain explicit; unknown or incomplete results cannot become success. |
| Machine evidence | The actual tested inputs, command and machine qualify the result. Existing transport and current owner rules govern execution; no permanent worker or arbitrary remote shell action is introduced. |
| Website publication | The checked artifact, source identity and final digest match the reviewed receipt. Deployment confirmation checks the exact commit and live changed routes/assets. Site generation stays on request. |
| Discord | Preview has a positive output and sends nothing. Approved delivery binds template and destination, suppresses repeated unchanged failures, reports recovery once, and refuses ambiguous retries. |
| Credentials | Untrusted workflow payloads execute no code and acquire no webhook secret. Logs and receipts contain no credentials. |
| CLI and source ownership | Commands register through the existing action and rendering systems; every library feature has a production caller and entry-point test. |
| Tests and documentation | Positive and near-miss negative cases cover the complete accepted behavior; the phase updates its owning pages without weakening existing proofs. |

## Checklist

- [ ] Design and exact messaging boundaries approved before implementation.
- [ ] Tests written for every AC and registered entry point.
- [ ] Tests FAIL for the intended missing behavior, recorded by the main thread.
- [ ] Tests PASS with positive output and the identity/authorization negatives.
- [ ] Functional `.ci` entry-point proof passes from the main checkout.
- [ ] Integration and documentation applicability rows checked against the implemented change.
- [ ] Independent review and closure evidence complete before the two-commit closure.
