---
title: Finishing Work
when: before claiming any work done, complete, or ready to commit, and whenever a defect, a red test, or a missing behavior blocks that claim
severity: blocking
related: planning, testing, interop-and-goal-validation, writing, evidence, rule-precedence
---
directives ## Directives
  this-rule-is-at-the-level-of-git-safety
  never-claim-done-while-an-ac-is-unimplemented
  fix-a-defect-that-blocks-your-goal
  fix-the-problem-not-only-the-files-you-opened
  spec-a-found-problem-close-then-ask
  interoperability-and-correctness-are-never-optional
  recording-a-problem-is-not-addressing-it-fix-the-root-cause
  write-the-diagnosis-before-silencing-a-symptom
  implement-the-missing-behavior-at-the-source
  every-exported-symbol-needs-a-daemon-caller
  prove-every-feature-works-integrated
  audit-the-spec-line-by-line-before-marking-it-done
  never-ask-permission-before-finishing-the-work
  the-answer-to-an-exemption-argument-is-always-no
the-problem ## The Problem
  the-false-completion-claim
the-rule ## The Rule
  deferred-never-means-done
what-done-requires ## What "Done" Requires
  the-preconditions-for-saying-done
  the-requirements-for-done
  say-what-remains-when-a-requirement-fails
banned-phrases-when-work-remains ## Banned Phrases When Work Remains
  phrases-that-hide-unfinished-work
what-to-do-instead ## What To Do Instead
  when-you-genuinely-cannot-complete-an-item
  say-you-cannot-finish-and-keep-the-spec-open
  never-present-deferred-work-as-finished
scope-reduction-requires-explicit-user-approval ## Scope Reduction Requires Explicit User Approval
  never-reduce-scope-without-the-user
  when-an-acceptance-criterion-turns-out-harder
  stop-and-ask-when-an-ac-is-harder-than-expected
  never-decide-an-ac-is-out-of-scope-yourself
  documentation-is-not-a-substitute-for-implementation
  documenting-a-gap-is-scope-reduction
  excuses-that-do-not-resolve-a-deliverable
  never-present-a-documented-gap-as-a-closed-gap
on-violation ## On Violation
  stop-immediately-on-violation
recording-is-not-fixing-owner-directive-2026-07-23 ## Recording is not fixing (owner directive, 2026-07-23)
  you-own-the-defect-you-walked-into
  fix-these-instead-of-recording-them
  the-one-exception-a-mechanism-you-could-not-determine
  a-reproducible-failure-has-no-recording-path
  never-record-a-hypothesis-as-a-finding
verification-debt-is-not-defect-debt ## Verification debt is not defect debt
  an-unrun-gate-is-recordable-a-defect-is-not
  which-is-which
  commit-locally-pay-at-the-push
the-failure-this-rule-exists-to-stop ## The failure this rule exists to stop
  the-blocked-deliverable-pattern
  the-moves-that-follow-a-blocked-deliverable
  pre-existing-does-not-put-a-bug-out-of-scope
the-distinction-from-legitimate-deferral ## The distinction from legitimate deferral
  deferral-is-for-separable-work-not-for-a-blocker
  the-question-that-separates-deferral-from-parking
  when-a-deferral-is-legitimate
  when-unsure-you-are-on-the-fix-it-side
  spec-the-defect-you-own-close-then-ask
banned-moves ## Banned moves
  moves-that-reduce-scope-invisibly
when-you-genuinely-cannot-finish ## When you genuinely cannot finish
  never-hide-a-blocker
  state-the-goal-is-not-met-and-ask-which-way-to-fix-it
verification-of-the-goal ## Verification of the goal
  the-goal-is-met-when-the-real-user-visible-path-works
load-is-never-an-explanation-it-is-the-bug ## Load is never an explanation. It is the bug.
  a-load-sensitive-test-is-a-broken-test
  conclusions-that-are-banned-everywhere
  what-each-load-excuse-really-means
  load-explained-is-diagnosed-not-non-deterministic
making-a-test-load-proof ## Making a test load-proof
  wait-for-the-condition-never-for-a-duration
  how-to-replace-each-timing-assumption
  raising-a-timeout-is-not-a-fix
  a-real-wait-often-exposes-a-genuine-race
  the-stress-tools-diagnose-they-never-absolve
recording ## Recording
  a-shard-is-a-running-log-not-a-destination
  ask-what-did-i-fix-before-writing-a-record
  what-never-earns-a-shard
  the-gate-that-refuses-a-load-excuse-in-a-shard
  a-row-that-is-not-committed-does-not-exist
length-is-not-evidence ## Length is not evidence
  a-record-earns-its-length-from-what-a-future-reader-must-do
  where-the-length-budgets-live
anti-rationalization ## Anti-Rationalization
  the-answer-is-always-no
  tdd
  answers-to-tdd-excuses
  test-failures
  answers-to-test-failure-excuses
  every-test-failure-gets-fixed
  fix-an-unrelated-failure-in-its-own-commit
  excuses-that-leave-a-failure-in-place
  completion-rationalizations
  answers-to-completion-excuses
  the-three-fix-rule
  stop-after-three-failed-fixes-and-ask
  posture
  no-performative-agreement-just-fix-it
diagnosis-before-fix ## Diagnosis Before Fix
  the-diagnosis-write-all-five-before-any-edit
  what-a-diagnosis-must-state
  implement-the-source-fix-only-after-the-diagnosis
  when-a-check-or-validation-rejects-you
  never-ask-how-do-i-get-past-this
  the-three-way-question
  incomplete-check-data-is-where-workarounds-hide
  altitude
  fix-where-the-problem-is-not-where-it-shows-up
  trigger-words-stop-and-write-the-diagnosis
  the-word-just-is-the-tell
no-workarounds-for-missing-behavior ## No Workarounds For Missing Behavior
  a-workaround-is-evidence-of-incomplete-work
  when-tempted-to-work-around-a-problem
  the-steps-that-replace-a-workaround
  banned-fixes
  fixes-that-are-banned-as-workarounds
  allowed-exception
  a-workaround-is-allowed-only-when-it-is-the-deliverable
  verification-of-the-user-visible-goal
  verify-through-the-intended-entry-point
wiring-completeness ## Wiring Completeness
  the-check-behind-the-done-requirements
  the-principle-wire-first-feature-second
  wiring-is-the-first-implementation-step
  where-wiring-is-checked-in-each-phase
  checking-wiring-at-completion-means-gates-failed
  mechanical-check-mandatory-before-claiming-done
  the-changed-file-wiring-gate-inside-ze-verify
  what-the-wiring-gate-checks
  how-to-review-one-new-exported-symbol
  the-dead-symbol-grep
  a-symbol-with-only-test-callers-is-dead-code
  grep-every-consumer-of-multi-consumer-data
  common-violations
  wiring-excuses-and-why-each-is-wrong
  where-to-check
  where-new-code-must-be-called-from
feature-integration-completeness ## Feature Integration Completeness
  prove-integration-not-isolation
  the-test-each-feature-type-requires
  the-delete-the-code-self-check
  functional-ci-test-blocking
  give-every-user-facing-feature-a-ci-test
  where-each-feature-type-ci-test-lives
  a-unit-test-never-substitutes-for-a-ci-test
  never-defer-the-ci-test-that-proves-the-entry-point
  wiring-tests-blocking-never-deferrable
  a-wiring-test-proves-the-feature-is-reachable
  excuses-for-shipping-an-unwired-feature
  an-unwritable-wiring-test-means-blocked-not-done
  every-spec-carries-a-wiring-test-table
  production-path-verification-blocking
  grep-every-implementation-before-changing-a-handler
  how-to-find-the-implementation-a-consumer-calls
  one-implementation-found-is-not-proof-there-s-only-one
implementation-audit ## Implementation Audit
  pointer-to-the-audit-rationale
  when-to-run-the-audit
  run-the-audit-before-a-journal-row-or-a-done-claim
  process
  how-to-run-the-implementation-audit
  approval-required
  what-each-audit-status-requires
  cannot-mark-done-until
  the-audit-completion-checklist
  evidence-standards
  what-counts-as-evidence-for-each-claim
  ac-evidence-verification-blocking
  quote-the-ac-then-name-the-asserting-test
  pre-commit-verification-blocking
  re-verify-every-item-after-the-audit
  how-to-verify-each-closure-table
  fill-an-evidence-row-in-every-table
  what-is-not-acceptable-as-evidence
  red-flags
  signs-the-implementation-is-incomplete
don-t-ask-do ## Don't Ask, Do
  finish-the-task-then-report-what-was-done
  the-exception-ambiguous-scope-or-a-destructive-action
  when-asking-is-mandatory
  the-standing-exceptions-to-do-not-ask
  enforcement
  this-rule-is-hook-enforced-breaking-it-costs-a-blocked-stop
  how-the-stop-hook-scans-for-banned-phrases
rationale ## Rationale
  why-diagnosis-before-fix-exists
  what-fix-do-not-record-cost-to-learn
