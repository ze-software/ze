---
title: Testing
when: writing, changing, or deleting any test, and before writing implementation code for new behavior
severity: blocking
related: completion, platform-linux, rfc-compliance
---
directives ## Directives
  write-the-test-first-and-never-weaken-it
  draft-a-ci-before-it-goes-live
fix-code-not-tests ## Fix Code, Not Tests
  fix-the-code-when-a-test-fails-not-the-test
  the-weakening-the-hook-cannot-see
  write-the-weakened-row-before-the-edit
the-affected-population-is-not-the-edited-population ## The Affected Population Is Not the Edited Population
  the-tests-you-write-for-a-change-are-green-by-construction
mutation-testing ## Proving a Test Discriminates
  a-cached-verdict-can-hide-an-exec-reached-mutation
  verify-the-mutation-applied-before-trusting-the-run
rfc-tagged-tests-blocking ## RFC-Tagged Tests
  never-edit-an-rfc-tagged-test-to-match-the-code
  pin-every-requirement-with-a-positive-and-a-negative
  reindex-after-moving-a-tagged-test
iteration-workflow-blocking ## Iteration Workflow
  never-keep-a-numeric-test-id-past-the-turn
  name-a-real-slog-subsystem-in-a-ze-log-key
  pass-any-failure-to-catch-an-assertion-flake
  rebuild-before-trusting-a-no-build-verdict
ci-sleep-justification ## CI Sleep Justification
  justify-every-sleep-in-a-ci-test
temporary-files ## Temporary Files
  use-project-tmp-for-scratch-files
native-test-actions ## Native Test Actions
  return-an-error-from-a-failing-observer
  run-the-focused-test-through-a-native-action
  the-action-or-page-that-owns-each-test-question
