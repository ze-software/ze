---
title: Testing
when: writing, changing, or deleting any test, and before writing implementation code for new behavior
severity: blocking
related: completion, platform-linux, rfc-compliance
---
directives ## Directives
  write-the-test-first-and-never-weaken-it
test-driven-development ## Test-Driven Development
  the-steps-of-the-tdd-cycle
  the-test-patterns-to-use
  test-both-edges-of-every-numeric-range
  the-coverage-target-for-each-code-type
  assert-the-ac-s-behavior-not-its-mechanism
  what-to-assert-for-each-kind-of-ac-wording
  quote-the-ac-and-check-a-stub-would-fail
  never-use-an-absence-as-proof-of-an-action
  add-a-test-for-anything-you-debugged
draft-a-functional-test-before-it-is-live-blocking ## Draft a Functional Test Before It Is Live (BLOCKING)
  never-draft-or-edit-a-ci-test-in-the-live-suite
  a-draft-is-promoted-or-deleted-never-left
  promote-early-because-the-checks-start-when-live
test-code-is-held-to-one-standard ## Test Code Is Held to One Standard
  test-code-must-run-and-must-be-correct-about-the-product
fix-code-not-tests ## Fix Code, Not Tests
  fix-the-code-when-a-test-fails-not-the-test
  never-change-test-data-to-make-a-test-pass
test-deletion-and-weakening ## Test Deletion and Weakening
  legitimate-and-illegitimate-reasons-to-delete-a-test
  the-weakenings-the-hook-detects
  the-one-weakening-the-hook-cannot-see
  add-a-test-never-repurpose-an-existing-one
  the-right-and-wrong-move-in-each-scenario
  why-a-rewrite-passes-the-structural-check
  find-a-semantic-replacement-in-review
  document-a-legitimate-relaxation-on-the-line
  the-token-leaves-an-audit-trail-you-must-honor
functional-test-gate ## Functional Test Gate
  every-user-facing-change-needs-a-functional-test
  the-test-and-directory-each-change-type-needs
  when-no-row-fits-write-one-if-unsure
  the-conditions-that-excuse-a-functional-test
  the-steps-of-a-mutation-verification
  guard-an-unobservable-behavior-with-a-unit-test
  a-fixture-must-encode-a-topology-that-can-exist
  the-excuses-for-skipping-a-functional-test
  how-this-section-relates-to-the-neighboring-rules
rfc-tagged-tests-blocking ## RFC-Tagged Tests (BLOCKING)
  never-edit-an-rfc-tagged-test-to-match-the-code
  what-to-do-in-each-tagged-test-situation
  where-an-rfc-tag-may-live-and-what-it-is-worth
  prefer-a-ci-binding-and-never-declare-a-tier
  test-relax-is-not-approval-to-change-a-tagged-test
  pin-every-requirement-with-a-positive-and-a-negative
back-fill-new-test-types-blocking ## Back-Fill New Test Types (BLOCKING)
  back-fill-a-new-test-type-onto-existing-code
  the-steps-of-a-back-fill
test-sensitivity-ratchets-blocking ## Test Sensitivity Ratchets (BLOCKING)
  a-test-that-re-implements-the-logic-it-names
  read-the-test-health-report-before-claiming-health
the-affected-population-is-not-the-edited-population ## The Affected Population Is Not the Edited Population
  re-check-the-tests-a-change-can-reach
  a-discrimination-proof-expires-when-the-environment-changes
  derive-the-reachable-set-from-the-graph-not-from-git-diff
  run-the-audit-that-exists-over-the-reachable-tagged-tests
  the-tests-you-write-for-a-change-are-green-by-construction
  a-shape-change-has-two-populations-old-name-and-new
no-throw-away-tests ## No Throw-Away Tests
  never-write-a-throw-away-test
  put-each-test-in-the-suite-that-runs-its-format
native-test-actions ## Native Test Actions
  read-the-platform-linux-rule-before-linux-code
  the-qemu-integration-action-and-when-it-is-required
  the-netns-actions-and-when-they-are-required
  prefer-a-knob-that-skips-the-privileged-work
  test-vpp-apply-through-the-fakeops-seam
  how-to-test-each-part-of-a-vpp-backend
  a-real-daemon-is-never-a-reason-to-skip-vpp-tests
  the-stages-of-a-native-verify-run
iteration-workflow-blocking ## Iteration Workflow (BLOCKING)
  change-one-file-test-it-then-scale
  start-with-the-narrowest-test-that-can-fail
  never-keep-a-numeric-test-id-past-the-turn
  name-a-real-slog-subsystem-in-a-ze-log-key
  the-escalation-ladder-and-where-to-rerun
  native-verify-is-the-final-gate-not-a-development-tool
  never-run-two-test-runs-at-once
  read-one-test-s-output-before-bulk-editing
timing-baseline ## Timing Baseline
  how-the-auto-timeout-is-computed
  slow-tests-are-flagged-against-the-baseline
test-tools ## Test Tools
  the-test-peer-and-the-test-runner
  update-the-discovery-paths-when-you-add-a-tool
native-go-tooling ## Native Go Tooling
  a-tool-with-no-native-action-never-runs
  the-native-runner-proves-its-population
  keep-first-party-tooling-in-native-go-packages
temporary-files ## Temporary Files
  use-project-tmp-for-scratch-files
  prefer-your-session-s-own-scratch-directory
debugging-failures ## Debugging Failures
  read-the-failure-index-before-any-rerun
  rerun-the-smallest-useful-scope-never-pipe-to-tail
editor-tests-et-format ## Editor Tests (.et format)
  what-et-tests-are-and-how-to-run-them
os-specific-tests ## OS-Specific Tests
  how-to-gate-a-test-by-os
reproducing-load-dependent-flaky-in-full-verify-failures ## Reproducing Load-Dependent (Flaky-in-Full-Verify) Failures
  pass-any-failure-to-catch-an-assertion-flake
  rebuild-before-trusting-a-no-build-verdict
  never-loop-the-full-suite-to-hunt-a-flake
reactor-concurrency-code-blocking ## Reactor Concurrency Code (BLOCKING)
  run-ze-race-reactor-when-you-touch-reactor-state
  the-verification-each-reactor-change-requires
  paste-the-race-reactor-output-as-evidence
compiled-observer-failures-blocking ## Compiled Observer Failures (BLOCKING)
  return-an-error-from-a-failing-observer
  the-fixture-runner-publishes-observer-errors
  the-bad-observer-form-and-its-replacement
  assert-on-production-log-lines-where-you-can
  which-assertion-pattern-to-use-when
  the-native-fixture-tests-own-the-failure-boundary
  the-sleep-count-may-only-go-down
ci-sleep-justification ## CI Sleep Justification
  the-reasons-a-sleep-must-carry-its-reason
  prefer-converting-the-sleep-to-a-deterministic-wait
  try-a-sync-primitive-before-you-write-a-sleep
  justify-every-sleep-in-a-ci-test
  the-specs-that-block-converting-these-sleeps
compiled-observer-api ## Compiled Observer API (`internal/test/fixture`)
  prefer-a-payload-barrier-for-route-server-tests
  prefer-a-payload-predicate-wait-over-a-sleep
mutation-testing ## Mutation Testing
  a-test-failing-to-test-is-worse-than-none
  mutate-a-file-you-own-never-a-shared-one
  state-whether-a-discrimination-re-run-was-real
pre-commit ## Pre-Commit
  native-verify-is-the-only-acceptable-pre-commit-check
