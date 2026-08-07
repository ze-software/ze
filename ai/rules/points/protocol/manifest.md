---
title: Protocol Implementation
when: implementing or changing a protocol, an external API, a wire format, or a backend that applies operator config
severity: blocking
related: rfc-compliance, completion, planning, go-standards, architecture, plugins
---
directives ## Directives
  read-the-rfc-cite-it-inline-and-never-approximate
rfc-summaries-before-design ## RFC Summaries Before Design
  mechanical-rule
  read-every-summary-before-any-design-recommendation
  banned-reasoning
  excuses-for-skipping-the-rfc-summary
self-documenting-code ## Self-Documenting Code
  rules
  when-an-inline-reference-is-required
  format
  place-the-reference-block-at-the-top-of-the-file
  not-required
  where-an-inline-reference-is-not-required
protocol-subpackage-skeleton-advisory ## Protocol Subpackage Skeleton (advisory)
  the-skeleton
  the-skeleton-applies-once-a-protocol-grows-subpackages
  the-modules-of-the-protocol-skeleton
  bfd-is-the-reference-layout
  how-existing-protocols-map-probe-2026-07-08
  how-each-protocol-maps-to-the-skeleton
  the-advisory-report
  the-skeleton-report-is-advisory-and-never-a-gate
exact-or-reject ## Exact Or Reject
  in-practice
  what-to-reject-instead-of-approximating
  checklist-for-every-backend
  what-every-backend-must-reject-before-apply
  banned-phrases-in-code-comments
  phrases-that-signal-a-silent-approximation
  design-it-properly-or-reject-it-in-the-verifier
  mechanical-check
  run-this-checklist-before-marking-a-backend-done
  questions-that-find-a-silent-lossy-path
  fix-any-checklist-no-before-committing
related-rules ## Related Rules
  rules-that-cover-the-rest-of-a-protocol-change
rationale ## Rationale
  pointer-to-the-exact-or-reject-rationale
  why-rfc-summaries-beat-memory
  why-trained-rfc-knowledge-is-unreliable
  why-the-inline-reference-is-mandatory
  why-a-session-cannot-remember-the-upstream-spec
examples ## Examples
  an-inline-spec-reference-block
