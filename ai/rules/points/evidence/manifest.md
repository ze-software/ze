---
title: Evidence and Guards
when: stating what code does, acting on recorded claims, writing or reviewing a guard, or writing any string that enumerates data a registry already holds
severity: blocking
related: writing, planning, protocol
---
directives ## Directives
  state-only-what-the-source-says-and-read-the-producer
  make-every-guard-fail-closed-or-speak
  derive-every-string-from-the-canonical-registry
no-fabrication ## No Fabrication
  say-so-when-the-source-does-not-answer
  verify-runtime-behavior-against-the-producing-code
  name-the-authority-before-reconciling-two-statements
  read-the-decision-record-before-asserting-intent
  a-self-consistent-story-is-a-hypothesis-not-a-finding
  a-pending-result-is-not-a-result
  one-instance-is-not-a-population
  the-named-example-is-the-worst-sample
  paste-the-command-output-never-a-prose-summary-of-it
  verify-by-reading-cite-by-file-and-symbol
  put-a-load-bearing-line-number-in-a-fenced-block
  cite-a-foreign-project-by-file-and-symbol
  a-pinned-tag-does-not-make-a-line-number-safe
  cite-a-line-number-only-when-a-generator-maintains-it
  derive-a-location-or-leave-it-out
  keep-multiplicity-when-a-symbol-key-replaces-a-location
  name-the-symbol-before-removing-a-location
  a-pasted-line-number-goes-stale-at-the-next-edit
  answer-only-from-text-you-can-cite
  read-the-producing-code-before-claiming-behavior
  name-the-keystone-fact-and-read-its-producer
  banned-inference-patterns
  investigate-source-in-session-before-writing-a-spec
claims-about-project-state ## Claims About the State of the Project
  rule
  five-claims-that-decay
  verify-the-frame-before-you-work-inside-it
  re-verify-before-escalating-to-the-owner
  establish-reachability-before-stating-impact
  foreign-system-semantics
  never-label-an-assumption-measured
  correct-the-document-where-the-stale-claim-lived
  a-peer-correction-is-a-claim-not-evidence
  prove-a-search-can-find-before-you-report-a-zero
  close-every-record-of-a-defect-you-fix
records-you-author ## Records You Author
  rule
  read-the-parser-before-writing-the-field
  a-decorated-value-is-a-different-value
  trace-every-constraint-you-put-in-a-brief
  an-unchecked-brief-becomes-invisible-scope
  derive-readiness-from-dependencies-not-status
  a-status-field-is-an-estimate-its-dependencies-are-the-fact
fail-closed-guards ## Fail-Closed Guards
  what-counts-as-a-guard
  put-the-guard-at-the-writer-not-the-producer
  a-guard-is-real-only-on-the-path-that-carries-the-traffic
  what-fail-closed-requires
  never-let-a-zero-value-look-like-a-valid-answer
  read-maps-with-the-comma-ok-form
  check-for-empty-as-well-as-missing
  drive-the-guard-from-its-entry-point
  test-the-shape-that-must-be-rejected
  never-treat-a-doc-or-comment-as-safety-evidence
  more-instances-of-the-fail-open-shape
derive-never-hardcode ## Derive, Never Hardcode
  pull-each-surface-from-its-registry
  add-a-list-accessor-instead-of-pasting-the-list
  return-typed-values-never-rendered-text
  treat-the-registry-as-the-single-truth
  grep-for-duplicate-lists-before-committing
  cases-where-a-hardcoded-list-is-allowed
