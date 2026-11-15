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
| Updated | 2026-11-15 |

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

The exact AI contribution paragraph and both exact maintainer alert templates are approved. Thomas selected the existing GitHub activity feed and authorized notifier/schedule installation on `claude`; these decisions need no renewed approval. Source and portable installation documentation are committed in `0320acf1ed73fe29efa2d2b6a1b6aefd8efc83aa`, following descriptor hardening in `27023312aeca32fb288cb2db78b294845f604124`. Independent source review found no remaining product issue in the reviewed delta. Installation is not complete.

Thomas chose to pull the committed work on `claude` himself before asking Codex to install it. The latest read-only preflight observed clean server `main` at `914b25449dd030fd98f2fd5b75a9229abe1e9d4e`, with server-only commits to preserve. This owner-performed integration resolves the hand-off route without granting an agent a branch-integration exception. Installation remains pending the owner's pull and a refreshed clean-checkout/revision check. Do not overwrite, stash, change branches or integrate on inferred permission.

The verified private bundle is retained at `/home/thomas/tmp/ze-github-notifier-01a1a3ac/notifier-install.bundle` and is refreshed for the complete committed hand-off. It carries committed inputs without public publication. The existing owning session record and installation hand-off retain its current advertised revision, digest, verification and removal condition; this spec does not pin a mutable transport file's historical digest. No notifier account, binary, unit, private configuration or schedule has been installed.

Canonical contribution HTML and Markdown have been regenerated through the native docs producer and checked. No site artifact was committed or deployed. Publication must preserve the requested source/artifact identities and obtain an explicit owner push instruction for the exact shared-branch ancestry; unrelated README changes remain excluded from this task's authorization.

The native session state owns the complete wording review, paired-identity preview, token-free destination identity, scoped test logs and publication ancestry snapshot. After private integration is permitted, finish the existing Linux installation and operational proof described in `docs/contributing/github-notifier.md`. Source and website publication remain withheld for Thomas's later step 3 decision. AC-9 live installation evidence, AC-6 deployment confirmation and final closure remain owned here. A source-review result must not be described as an installed notifier or completed publication.

## Implementation Evidence

| Scope | Current evidence | Qualification |
|-------|------------------|---------------|
| GitHub revision/run and local readiness reports | Registered-action, pagination, identity, state, bounds and machine-input tests pass; live CodeQL run 43726804487 report and preview succeed | API observations do not certify native test or release results. No remote machine execution was added. |
| Publication receipt and confirmation mechanics | Registered positive/negative tests cover source races, changed bytes, raw committed blobs, index/filter/replacement tricks, live mismatches and zero-network refusal | Actual deployment confirmation waits for an approved artifact commit and publication. |
| Notification activation and polling | Committed explicit initialization/delivery, private authority, workflow-filtered polling and durable receipts; 41 named notification bodies and six terminal-state subcases passed; scoped GitHub lint passed on host and Linux | These are local source proofs. No notifier is installed and no real message was sent. |
| Descriptor refusal discrimination | Native FIFO test passed clean, then failed in 10 seconds under the overlay removing only `O_NONBLOCK`, with its own blocked-reader diagnostic; no source was changed | Proves the receipt acquisition regression; not a whole-tree or installed-host certificate. |
| Approved destination preview | Real paired-identity preview succeeded with approval binding `c9f08a1bed9cc3cedd3b85143f89f4913ac79dbb82fcaca6d5594cbd940d76e0` | Preview-only, no send and no credential read. |
| Portable installation documentation | `docs/contributing/github-notifier.md` and versioned installer, shared leaf guards, isolated refusal proof and systemd units committed; all three shell syntax checks passed | Actual Linux leaf-guard execution, native build, installation, service poll and enabled schedule remain owed after private integration. |
| Full documentation check | Completed with exit 1 and 720 pre-existing published website/wiki catalog-drift findings; no notifier-guide match in the failure log | Not an all-documentation pass. Existing catalog publication responsibility remains with `plan/post-release/engineering/spec-publish-the-regenerated-command-catalog.md`. |
| Contribution policy and intake | Exact approved AI policy guard and canonical-link render guards pass; issue/PR sources prepared | Non-AI messaging and online publication remain subject to owner review. |
| Installed command paths | Both new UI carriers passed 80 actual named invocations under load and passed again in the live suite | The full UI suite also found 15 other failures and 20 skips; it is not a broad green certificate. |
| Canonical review site | Native docs-producer render and site check pass with 1118 claimed routes; both contribution HTML/Markdown pairs contain the policy link | Rendering grants no approval or publication authority. |

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

Boundary inputs: require a full 40-character commit ID and positive run/attempt IDs; test zero, negative/dash-leading and malformed values. Test the last configured API-page/response limit and the first rejected value; pagination exhaustion is incomplete, never a truncated pass. Interop is N-A: no routing protocol changes. Proposed functional carrier `test/ui/le-github-report.ci` exercises the registered command against a local API fixture; its author must read `ze-test` before creating it. All listed tests are planned and unrun.

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

Implementation and portable documentation are committed; the existing feed, exact alert templates and installation host are approved. Independent security/correctness and Go-style/test-discrimination review covered the initial integration source and subsequent fixes. Receipt FIFO acquisition, leaf/ancestor alias refusal, recovery scope, quiet-workflow liveness, installer leaves and regression cleanup findings were corrected. The current source-only delta has no remaining product finding; no operational CLEAN artifact or closure was recorded.

This is a blocked hand-back, not a reduced scope or closure. After Thomas pulls the private bundle, main must run the actual sourced leaf-guard proof on Linux, build and identify the clean reviewed native executable, install it outside the checkout, provision approved private authority, initialize each workflow explicitly, demonstrate a service-account poll and preserved receipts, and enable/check the fifteen-minute timer. Preserve any unresolved attempted delivery; no implicit reset or retry is permitted. Record observed real transitions, or qualify quiet-only evidence accurately. Final source-pinned review and closure remain owed after the operational work and owner-approved publication.

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
