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
  pointer-to-the-derive-not-hardcode-rationale
no-fabrication ## No Fabrication
  rule
  say-so-when-the-source-does-not-answer
  behavioral-claims-and-recommendations
  verify-runtime-behavior-against-the-producing-code
  read-the-decision-record-before-asserting-intent
  a-self-consistent-story-is-a-hypothesis-not-a-finding
  paste-the-command-output-never-a-prose-summary-of-it
  the-bgp-reconnect-flap-non-problem
  citation
  verification-and-citation-are-separate-decisions
  verify-by-reading-cite-by-file-and-symbol
  put-a-load-bearing-line-number-in-a-fenced-block
  cite-a-foreign-project-by-file-and-symbol
  a-pinned-tag-does-not-make-a-line-number-safe
  cite-a-line-number-only-when-a-generator-maintains-it
  derive-a-location-or-leave-it-out
  keep-multiplicity-when-a-symbol-key-replaces-a-location
  name-the-symbol-before-removing-a-location
  a-pasted-line-number-goes-stale-at-the-next-edit
  mechanical-check
  before-answering-a-factual-question-about-file-content
  answer-only-from-text-you-can-cite
  read-the-producing-code-before-claiming-behavior
  name-the-keystone-fact-and-read-its-producer
  banned
  banned-inference-patterns
  mechanical-backstop
  investigate-source-in-session-before-writing-a-spec
claims-about-project-state ## Claims About the State of the Project
  rule
  five-claims-that-decay
  a-stale-frame-costs-more-than-a-wrong-fact
  verify-the-frame-before-you-work-inside-it
  re-verify-before-escalating-to-the-owner
  establish-reachability-before-stating-impact
  foreign-system-semantics
  never-label-an-assumption-measured
  correct-the-document-where-the-stale-claim-lived
fail-closed-guards ## Fail-Closed Guards
  rule
  what-counts-as-a-guard
  what-fail-closed-requires
  the-zero-value-trap
  never-let-a-zero-value-look-like-a-valid-answer
  read-maps-with-the-comma-ok-form
  check-for-empty-as-well-as-missing
  test-corollary
  drive-the-guard-from-its-entry-point
  why-a-green-test-on-an-uncalled-guard-misleads
  test-the-shape-that-must-be-rejected
  evidence-corollary
  never-treat-a-doc-or-comment-as-safety-evidence
  why-safety-claims-carry-the-highest-cost
  a-spot-check-can-pass-where-the-guard-fails
  worked-example
  the-authz-empty-profile-privilege-escalation
  more-instances-of-the-fail-open-shape
derive-never-hardcode ## Derive, Never Hardcode
  rule
  pull-each-surface-from-its-registry
  add-a-list-accessor-instead-of-pasting-the-list
  structured-data-not-pre-formatted-strings
  let-the-display-layer-own-rendering
  return-typed-values-never-rendered-text
  treat-the-registry-as-the-single-truth
  mechanical-check
  grep-for-duplicate-lists-before-committing
  when-a-hardcoded-list-is-ok
  cases-where-a-hardcoded-list-is-allowed
  otherwise-derive
