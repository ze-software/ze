---
title: Interop Testing and Goal Validation
when: implementing or changing protocol behavior, and when validating that a spec's stated goals are met
severity: blocking
---
directives ## Directives
  prove-interop-and-prove-the-goal
interop-testing-protocol-features ## Interop Testing (protocol features)
  prove-protocol-behavior-against-another-implementation
  required-interop-test-by-protocol
  test-infrastructure-and-target-per-protocol
  what-must-be-tested
  the-interop-assertion-owed-by-each-feature-type
  when-interop-tests-are-not-required
  conditions-that-need-no-interop-test
  interop-scenario-structure
  a-scenario-directory-is-named-never-numbered
  the-config-files-every-interop-scenario-carries
  the-native-checker-every-scenario-must-register
prove-the-test-discriminates-blocking ## Prove the test discriminates (BLOCKING)
  a-test-is-evidence-only-if-it-can-fail
  revert-the-change-and-confirm-the-test-goes-red
  why-a-regression-test-never-had-a-red-phase
  vacuity-traps-and-how-to-spot-each-one
  say-so-when-the-peer-cannot-discriminate-the-change
goal-validation-all-features ## Goal Validation (all features)
  why-passing-tests-do-not-prove-the-goal
  the-goal-validation-rule
  answer-this-for-every-spec-goal-before-claiming-done
  name-the-evidence-that-proves-the-goal
  the-evidence-each-goal-type-requires
  what-goal-validation-excludes
  claims-that-are-not-goal-validation
  where-goal-validation-goes-in-the-spec
  where-the-goal-validation-table-lives-and-who-fills-it
mechanical-check ## Mechanical Check
  interop-check-protocol-specs
  commands-to-find-an-existing-interop-scenario
  create-the-scenario-when-none-matches
  goal-validation-check-all-specs
  what-the-goal-validation-table-must-contain
relationship-to-other-rules ## Relationship to Other Rules
  what-each-neighboring-rule-covers-and-what-this-one-adds
