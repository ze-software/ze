---
title: Protocol Implementation
when: implementing or changing a protocol, an external API, a wire format, or a backend that applies operator config
severity: blocking
related: rfc-compliance, completion, planning, go-standards, architecture, plugins
---
directives ## Directives
  read-the-rfc-cite-it-inline-and-never-approximate
rfc-summaries-before-design ## RFC Summaries Before Design
  read-every-summary-before-any-design-recommendation
  excuses-for-skipping-the-rfc-summary
self-documenting-code ## Self-Documenting Code
  when-an-inline-reference-is-required
  place-the-reference-block-at-the-top-of-the-file
  where-an-inline-reference-is-not-required
protocol-subpackage-skeleton-advisory ## Protocol Subpackage Skeleton (advisory)
  bfd-is-the-reference-layout
exact-or-reject ## Exact Or Reject
  what-to-reject-instead-of-approximating
  what-every-backend-must-reject-before-apply
  phrases-that-signal-a-silent-approximation
  design-it-properly-or-reject-it-in-the-verifier
  questions-that-find-a-silent-lossy-path
  fix-any-checklist-no-before-committing
related-rules ## Related Rules
  rules-that-cover-the-rest-of-a-protocol-change
