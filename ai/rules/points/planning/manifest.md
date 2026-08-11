---
title: Specs and Phases
when: before implementing any non-trivial feature, and whenever a spec phase starts, resumes, or closes
severity: blocking
related: completion, quality
---
directives ## Directives
  complete-before-implementing-any-non-trivial-feature
  supervise-delegate-and-review-independently
spec-work-runs-in-subagents-the-main-thread-supervises ## Spec Work Runs in Subagents; the Main Thread Supervises
  the-main-thread-launches-verifies-and-gates-each-phase
  announce-the-fan-out-before-you-spawn-it
  where-each-spec-phase-runs
  launch-independent-phases-in-one-message
  brief-every-subagent-with-its-spec-phase-and-rules
  verify-a-subagent-s-report-against-source
  keep-anything-the-user-must-answer-in-the-main-thread
  delegation-never-dilutes-the-independence-of-review
  delegation-does-not-override-phase-to-model-boundaries
  supervise-thinly-and-keep-exploration-out-of-main
  hand-off-when-the-main-thread-context-grows-too-large
  spawn-one-agent-per-implementation-phase
  report-an-oversized-package-never-trim-its-scope
  banned-reasoning-delegation
  banned-delegation-reasoning-and-the-reality
  enforcement-delegation
  spawning-an-agent-here-needs-no-permission
^work-phases ## Work Phases
  work-is-classified-by-phase-not-by-convenience
  what-each-work-phase-covers
  implementation-carries-no-model-requirement
  run-every-review-on-opus-5
  the-boundary-that-matters-most-is-independence-not-model
  review-is-independent-of-the-author
  what-to-do-at-each-phase-boundary
  critical-review-still-governs-over-this-section
  two-session-handoff
  split-implementation-and-closure-over-two-sessions
  which-session-does-what-in-a-two-session-handoff
  subagents
  never-downgrade-a-subagent-s-model
  banned-reasoning-model-phases
  banned-model-phase-reasoning-and-the-reality
  enforcement-model-phases
  how-the-model-phase-gates-work-and-where-they-stop
spec-selection ## Spec Selection
  one-spec-at-a-time-per-session
plan-file-location ## Plan File Location
  prefer-a-spec-over-a-plan-file
creating-a-spec-blocking ## Creating a Spec (BLOCKING)
  always-start-a-spec-from-plan-template-md
  keep-the-design-and-closure-templates-apart
  leave-placeholders-only-while-the-spec-is-a-skeleton
  name-make-ze-verify-as-the-spec-s-only-gate
pre-implementation ## Pre-Implementation
  the-research-and-design-phase-checklist
implementation-plan-format ## Implementation Plan Format
  present-the-implementation-plan-before-writing-code
  wait-for-user-approval-before-editing-any-file
spec-rules ## Spec Rules
  how-to-write-and-edit-a-spec
risks-assumptions-blocking-for-new-specs ## Risks & Assumptions (BLOCKING for new specs)
  write-gate-concerns-into-the-risks-and-assumptions-tables
  what-the-assumptions-and-risks-tables-capture
  rules
  give-every-assumption-a-validation-method
spec-sets ## Spec Sets
  use-a-shared-prefix-for-a-related-set-of-specs
  the-spec-set-naming-patterns
  how-a-spec-set-is-numbered-and-cross-referenced
spec-metadata-blocking ## Spec Metadata (BLOCKING)
  give-every-spec-a-metadata-table-under-its-title
  the-metadata-fields-and-their-values
  when-to-update-blocking
  transition-status-at-the-start-of-a-phase
  update-spec-status-at-each-transition
  status-vocabulary
  what-each-spec-status-means
  viewing-status
  how-to-view-the-spec-inventory
pre-spec-verification ## Pre-Spec Verification
  the-pre-spec-verification-checklist
retroactive-specs ## Retroactive Specs
  close-a-retroactive-spec-with-its-own-code
completion-checklist ## Completion Checklist
  after-all-tests-pass-complete-in-order
  completion-checklist-items-in-order
review-gate-blocking ## Review Gate (BLOCKING)
  run-the-review-and-loop-until-only-notes-remain
  every-review-round-is-smaller-and-the-loop-must-end
critical-review-is-the-central-deliverable ## Critical Review Is the Central Deliverable
  why-review-is-the-highest-leverage-step
  the-one-load-bearing-rule
  run-the-review-in-a-different-context-than-the-author
  your-own-reasoning-about-your-code-is-not-a-review
  what-a-real-review-pass-is
  what-makes-a-review-pass-real
  bounding-the-loop
  how-each-review-round-is-scoped-and-when-it-ends
  a-finding-in-the-record-is-not-a-finding-in-the-product
  a-defect-in-test-only-code-is-not-a-finding-in-the-product
  state-the-review-effort-before-you-spend-it
  announce-the-pass-count-and-lenses-before-spawning
  enforcement-critical-review-structural-a-hook-not-discipline
  the-commit-gate-that-needs-a-clean-review-artifact
  what-the-review-hook-can-and-cannot-prove
  banned-rationalizations
  stop-and-spawn-reviewers-when-you-think-one-of-these
  banned-review-rationalizations-and-the-reality
  scope
  review-every-closure-and-every-substantive-change
spec-closure-blocking ## Spec Closure (BLOCKING)
  a-spec-is-not-done-until-it-is-deleted-from-plan
  the-spec-closure-lifecycle
  why-closure-needs-two-commits
  produce-both-closure-commits-from-one-script
  why-commit-a-preserves-the-spec-in-history
  repoint-every-reference-to-the-spec-not-only-design
  each-gate-reads-a-different-reference-kind
  what-each-reference-gate-reads
  what-a-design-only-grep-cost-twice-in-one-day
  re-read-what-a-bulk-repoint-produced
  baseline-a-citation-only-when-it-is-a-historical-record
  resolve-rows-pointing-at-the-spec-during-closure
  why-closure-must-resolve-rows-that-name-the-spec
  never-resolve-a-row-to-a-record-that-omits-the-item
  never-git-rm-f-an-uncommitted-spec
  banned-closure-shortcuts-and-why
  closure-enforcement-automated
  why-three-gates-enforce-closure
  what-fires-each-closure-gate
  how-to-list-the-unclosed-spec-backlog
spec-preservation ## Spec Preservation
  where-the-rationale-lives
  what-to-discard-when-writing-the-row
  delete-the-spec-only-after-the-journal-row-is-written
  commit-the-spec-before-deletion
  transform-spec-scaffolding-into-knowledge
verify-specs-against-code-blocking ## Verify Specs Against Code (BLOCKING)
  grep-the-code-before-reporting-spec-progress
deferred-work-blocking ## Deferred Work (BLOCKING)
  where-the-full-deferral-process-lives
  verify-every-deferral-before-marking-a-spec-done
deferral-tracking ## Deferral Tracking
  record-and-home-every-scope-reduction
  why-the-homing-gate-warns-instead-of-blocking
  central-log
  where-deferrals-live-one-shard-per-source
  which-shard-holds-a-row
  the-shard-file-for-each-row-source
  write-your-row-into-the-shard-for-its-source
  delete-a-shard-at-closure-only-when-every-row-is-done
  read-the-closing-spec-s-own-shard-as-a-source
  keep-a-shard-that-still-holds-a-live-row
  a-live-row-beats-deletion-at-closure
  never-bulk-delete-orphaned-shards
  the-gate-that-blocks-removing-a-live-bearing-shard
  the-last-closer-deletes-an-all-terminal-orphan-shard
  give-an-orphaned-live-row-a-real-destination
  when-to-record
  what-triggers-a-deferral-record
  table-format
  the-deferral-table-header-row
  what-each-deferral-row-column-holds
  status-vocabulary-the-gate-reads-this
  where-the-terminal-status-set-is-defined
  the-deferral-status-vocabulary
  keep-a-homed-deferral-row-live-until-the-work-lands
  never-close-a-row-early-to-quiet-the-warning
  any-status-outside-the-terminal-set-is-live
  the-blind-spot-a-terminal-status-creates
  keep-this-table-and-the-terminal-status-set-in-step
  rules-deferrals
  the-deferral-recording-rules
  do-not-use-the-slack-the-gate-allows
  never-home-a-deferral-at-plan-known-failures
  choosing-the-destination-spec-blocking
  give-every-deferral-a-destination-spec-immediately
  how-to-choose-a-destination-spec-in-order
  prefer-an-existing-spec-over-a-new-deferral-file
  record-a-live-row-whichever-route-homes-the-work
  deferral-spec-naming-blocking
  how-a-deferral-holder-spec-is-named
  the-deferral-spec-filename-pattern
  the-parts-of-a-deferral-spec-filename
  rules-for-naming-a-deferral-holder-spec
  creating-the-deferral-spec
  the-steps-that-create-a-deferral-spec
  keep-a-deferral-skeleton-small
  how-the-unassigned-deferral-gate-behaves
  verify-before-deferring-blocking
  grep-before-claiming-infrastructure-is-missing
  what-is-not-a-deferral
  what-does-not-count-as-a-deferral
  resolving-deferrals
  close-a-row-when-the-work-lands-not-when-it-is-filed
  how-to-close-a-deferral-row
  filing-work-in-a-spec-does-not-close-the-row
executive-summary-report ## Executive Summary Report
  present-the-executive-summary-when-work-is-complete
  treat-the-sections-as-a-checklist-not-a-quota
  the-executive-summary-template
  the-purpose-of-each-executive-summary-section
documentation-update-checklist-blocking ## Documentation Update Checklist (BLOCKING)
  where-the-canonical-documentation-checklist-lives
  a-new-page-or-a-changed-claim-owes-an-independent-reader
writing-journal-rows ## Writing Journal Rows
  write-a-row-only-when-the-work-taught-something
  what-each-journal-row-cell-holds
  the-quality-test-every-journal-row-must-pass
session-handoff ## Session Handoff
  why-a-handoff-leads-with-its-rationale
  when-user-asks-how-to-continue
  start-a-handoff-with-its-rationale-then-the-edits
  what-a-handoff-includes-and-excludes
  the-rationale-is-a-verification-checkpoint-not-an-essay
  template-handoff
  the-handoff-template
  handover-documents-plan-handover
  write-a-handoff-to-plan-handover-when-it-must-persist
  rules-for-naming-and-placing-a-handover-file
  rules-handoff
  rules-for-the-edits-in-a-handoff
rationale ## Rationale
  why-the-main-thread-delegates-spec-work
