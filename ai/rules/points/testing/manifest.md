---
title: Testing
when: writing, changing, or deleting any test, and before writing implementation code for new behavior
severity: blocking
related: completion, platform-linux, rfc-compliance
---
directives ## Directives
  where-the-rationale-and-template-live
  write-the-test-first-and-never-weaken-it
test-driven-development ## Test-Driven Development
  the-tdd-cycle
  the-steps-of-the-tdd-cycle
  rfc-enforcing-tests
  tag-an-rfc-enforcing-test-with-its-requirement-id
  the-rfc-requirement-tag-format
  how-to-write-and-place-the-rfc-tag
  test-patterns
  the-test-patterns-to-use
  boundary-testing-mandatory
  test-both-edges-of-every-numeric-range
  worked-boundary-values-for-common-ranges
  coverage
  the-coverage-target-for-each-code-type
  ac-linked-tests-blocking
  assert-the-ac-s-behavior-not-its-mechanism
  what-to-assert-for-each-kind-of-ac-wording
  quote-the-ac-and-check-a-stub-would-fail
  never-use-an-absence-as-proof-of-an-action
  test-first-rules
  add-a-test-for-anything-you-debugged
draft-a-functional-test-before-it-is-live-blocking ## Draft a Functional Test Before It Is Live (BLOCKING)
  never-draft-or-edit-a-ci-test-in-the-live-suite
  the-commands-of-the-draft-workflow
  the-incubator-is-gitignored-and-gated-by-nothing
  a-draft-is-promoted-or-deleted-never-left
  promote-early-because-the-checks-start-when-live
test-code-is-held-to-one-standard ## Test Code Is Held to One Standard
  test-code-must-run-and-must-be-correct-about-the-product
fix-code-not-tests ## Fix Code, Not Tests
  fix-the-code-when-a-test-fails-not-the-test
  never-change-test-data-to-make-a-test-pass
test-deletion-and-weakening ## Test Deletion and Weakening
  legitimate-and-illegitimate-reasons-to-delete-a-test
  what-the-test-weakening-hook-blocks
  the-hook-blocks-these-edits-to-a-test-file
  the-weakenings-the-hook-detects
  the-one-weakening-the-hook-cannot-see
  test-rewrite-as-replacement-blocking
  add-a-test-never-repurpose-an-existing-one
  the-right-and-wrong-move-in-each-scenario
  why-a-rewrite-passes-the-structural-check
  find-a-semantic-replacement-in-review
  the-auditable-escape-hatch
  document-a-legitimate-relaxation-on-the-line
  the-test-relax-token-format
  the-token-leaves-an-audit-trail-you-must-honor
functional-test-gate ## Functional Test Gate
  the-functional-test-rule
  every-user-facing-change-needs-a-functional-test
  required-test-type-by-change
  the-test-and-directory-each-change-type-needs
  when-no-row-fits-write-one-if-unsure
  when-unit-tests-alone-are-sufficient
  when-a-unit-test-alone-is-acceptable
  the-conditions-that-excuse-a-functional-test
  otherwise-both-kinds-of-test-are-required
  mechanical-check-mandatory-before-claiming-done
  check-every-user-facing-change-in-the-diff
  the-command-that-finds-the-covering-test
  a-missing-functional-test-is-a-blocker
  mutation-verify-that-the-test-gates
  a-test-that-exists-is-not-a-test-that-gates
  mutation-verify-every-new-or-changed-guard-test
  the-steps-of-a-mutation-verification
  no-tool-catches-a-functional-false-pass
  guard-an-unobservable-behavior-with-a-unit-test
  a-fixture-must-encode-a-topology-that-can-exist
  common-violations
  the-excuses-for-skipping-a-functional-test
  relationship-to-other-rules
  how-this-section-relates-to-the-neighboring-rules
rfc-tagged-tests-blocking ## RFC-Tagged Tests (BLOCKING)
  never-edit-an-rfc-tagged-test-to-match-the-code
  what-to-do-in-each-tagged-test-situation
  where-an-rfc-tag-may-live-and-what-it-is-worth
  what-each-carrier-earns-in-the-ledger
  prefer-a-ci-binding-and-never-declare-a-tier
  test-relax-is-not-approval-to-change-a-tagged-test
  pin-every-requirement-with-a-positive-and-a-negative
back-fill-new-test-types-blocking ## Back-Fill New Test Types (BLOCKING)
  back-fill-a-new-test-type-onto-existing-code
  what-the-same-work-must-include
  the-steps-of-a-back-fill
test-sensitivity-ratchets-blocking ## Test Sensitivity Ratchets (BLOCKING)
  a-test-that-cannot-fail-reads-as-coverage
  a-test-that-re-implements-the-logic-it-names
  the-ratchet-that-counts-them-and-only-goes-down
  what-each-detector-fires-on-and-how-to-fix-it
  why-benchmarks-and-fuzz-targets-are-exempt
  read-the-test-health-report-before-claiming-health
  which-target-enforces-what
  why-the-report-is-published-and-not-byte-gated
no-throw-away-tests ## No Throw-Away Tests
  never-write-a-throw-away-test
  where-each-kind-of-test-lives-and-its-format
  put-each-test-in-the-suite-that-runs-its-format
make-targets ## Make Targets
  component-group-unit-tests
  test-one-area-during-development-not-every-package
  the-component-group-targets-and-their-cost
  use-the-group-that-matches-your-change
  verification-targets
  what-each-verification-target-runs
  contended-run-verdicts
  what-a-contended-run-verdict-means
  how-to-read-contended-failures
  linux-only-tests-qemu
  read-the-platform-linux-rule-before-linux-code
  the-qemu-integration-target-and-when-it-is-required
  capability-requiring-ci-tests-linux-host-per-test-netns
  the-netns-targets-and-when-they-are-required
  how-the-netns-targets-get-their-privilege
  prefer-a-knob-that-skips-the-privileged-work
  test-vpp-apply-through-the-fakeops-seam
  vpp-backend-testing-is-mandatory-blocking
  every-vpp-backend-ships-with-functional-tests
  how-to-test-each-part-of-a-vpp-backend
  a-real-daemon-is-never-a-reason-to-skip-vpp-tests
  two-pass-verification-how-ze-verify-works
  why-ze-verify-runs-two-passes
  the-stages-of-a-ze-verify-run
  how-long-a-full-verify-actually-takes
iteration-workflow-blocking ## Iteration Workflow (BLOCKING)
  change-one-file-test-it-then-scale
  start-with-the-narrowest-test-that-can-fail
  run-the-associated-test-first-then-widen
  the-steps-of-one-iteration
  targeted-test-commands-for-development
  the-command-and-cost-for-each-test-scope
  never-keep-a-numeric-test-id-past-the-turn
  when-to-use-an-id-and-when-to-use-a-pattern
  a-positional-name-selector-is-as-stable-as-pattern
  always-spell-pattern-in-full-never-p
  name-a-real-slog-subsystem-in-a-ze-log-key
  the-escalation-ladder-and-where-to-rerun
  ze-verify-is-the-final-gate-not-a-development-tool
  never-run-two-test-runs-at-once
  read-one-test-s-output-before-bulk-editing
individual-commands ## Individual Commands
  the-raw-go-test-commands
timing-baseline ## Timing Baseline
  where-the-per-test-timing-baseline-is-kept
  how-the-auto-timeout-is-computed
  slow-tests-are-flagged-against-the-baseline
test-tools ## Test Tools
  the-test-peer-and-the-test-runner
  update-the-discovery-paths-when-you-add-a-tool
testing-python-tooling-scripts ## Testing Python Tooling (scripts/)
  a-python-test-nothing-invokes-never-runs
  the-wired-conventions-for-python-tests
  both-conventions-run-inside-go-test
  never-add-a-python-test-outside-a-covered-directory
temporary-files ## Temporary Files
  use-project-tmp-for-scratch-files
  prefer-your-session-s-own-scratch-directory
  the-test-runner-already-roots-its-scratch-there
debugging-failures ## Debugging Failures
  read-the-failure-index-before-any-rerun
  where-the-failure-index-is-written
  rerun-the-smallest-useful-scope-never-pipe-to-tail
editor-tests-et-format ## Editor Tests (.et format)
  what-et-tests-are-and-how-to-run-them
  et-directives
  every-et-directive-and-what-it-does
  the-named-keys-an-et-test-can-press
  expectations
  every-et-expectation-and-what-it-checks
  when-to-use-et-vs-ci-vs-go-tests
  which-format-suits-which-test-need
  the-editor-test-directory-layout
  the-concerns-editor-tests-are-grouped-by
os-specific-tests ## OS-Specific Tests
  how-to-gate-a-test-by-os
  a-darwin-fail-from-an-unsupported-stub-is-a-setup-bug
common-flaky-test-causes ## Common Flaky Test Causes
  the-root-cause-behind-each-flake-symptom
  the-flake-shapes-to-check-first
reproducing-load-dependent-flaky-in-full-verify-failures ## Reproducing Load-Dependent (Flaky-in-Full-Verify) Failures
  why-a-load-dependent-failure-hides-in-a-single-suite
  use-the-stress-reproducer-not-the-full-suite
  what-the-stress-reproducer-does
  stress-reproducer-command-examples
  how-suite-and-test-selectors-are-split
  pass-any-failure-to-catch-an-assertion-flake
  what-the-reproducer-captures-and-what-it-exits
  rebuild-before-trusting-a-no-build-verdict
  rules-stress-reproduction
  never-loop-the-full-suite-to-hunt-a-flake
reactor-concurrency-code-blocking ## Reactor Concurrency Code (BLOCKING)
  run-ze-race-reactor-when-you-touch-reactor-state
  the-verification-each-reactor-change-requires
  paste-the-race-reactor-output-as-evidence
observer-exit-antipattern-in-ci-tests-blocking ## Observer-Exit Antipattern in `.ci` Tests (BLOCKING)
  never-signal-failure-with-sys-exit-in-an-observer
  use-runtime-fail-to-signal-an-observer-failure
  the-bad-observer-form-and-its-replacement
  assert-on-production-log-lines-where-you-can
  which-assertion-pattern-to-use-when
  fail-a-ci-observer-with-runtime-fail-not-sys-exit
  the-sleep-count-may-only-go-down
  every-tolerated-sleep-still-needs-a-justification
ci-sleep-justification ## CI Sleep Justification
  why-an-unexplained-sleep-is-capped-too
  the-reasons-a-sleep-must-carry-its-reason
  prefer-converting-the-sleep-to-a-deterministic-wait
  what-counts-as-justified
  the-comment-must-name-the-kind-of-sleep
  what-the-comment-must-say-for-each-kind-of-sleep
  where-to-put-the-comment-and-how-to-indent-it
  enforcement
  justify-every-sleep-in-a-ci-test
  related-ci-sleep
  the-specs-that-block-converting-these-sleeps
python-observer-api-test-scripts-ze-api-py ## Python Observer API (`test/scripts/ze_api.py`)
  what-an-embedded-python-plugin-can-import
  every-ze-api-function-and-what-it-does
  prefer-run-rs-observer-for-a-route-server-ci
  prefer-a-payload-predicate-wait-over-a-sleep
  the-full-protocol-form-of-the-api-class
  where-the-ze-api-source-and-examples-live
  the-declarative-form-of-a-ci-engine-step
mutation-testing ## Mutation Testing
  what-mutation-testing-does-and-that-it-is-advisory
  gomu-is-vendored-and-needs-no-install
  the-mutation-testing-targets
  the-environment-variables-that-tune-gomu
  what-is-excluded-because-gomu-has-no-tag-support
  mutate-a-file-you-own-never-a-shared-one
pre-commit ## Pre-Commit
  where-the-full-pre-commit-workflow-lives
  ze-verify-is-the-only-acceptable-pre-commit-check
