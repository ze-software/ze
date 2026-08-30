---
title: RFC Compliance (every protocol, not just BGP)
when: writing, changing, reviewing, or testing ANY protocol-implementing code, for ANY RFC Ze implements
severity: blocking
---
directives ## Directives
  hold-every-protocol-to-its-own-rfcs
  never-trade-conformance-for-convenience
  treat-conformance-as-non-negotiable
  record-an-authorised-deviation-in-the-journal
  what-to-do-in-each-conformance-situation
  read-the-rfc-text-before-claiming-conformance
  quote-the-rfc-text-before-reporting-a-violation
  an-obsoleted-predecessor-is-not-evidence
implement-full-compliance-ask-thomas-only-before-doing-less ## Implement Full Compliance. Ask Thomas Only Before Doing LESS (owner directive, 2026-07-27, clarified 2026-08-01)
  implement-full-compliance-when-it-is-reachable
  asking-is-required-only-when-you-are-about-to-do-less
  full-compliance-on-the-table-means-implement-not-ask
  when-to-stop-and-ask-thomas
  how-to-ask-thomas-about-a-compliance-shortfall
  treat-every-earlier-non-compliance-answer-as-void
  where-void-answers-hide-and-what-to-do
  raise-a-void-answer-the-moment-you-find-one
rfc-summaries-rfc-short ## RFC Summaries (`rfc/short/`)
  keep-ze-specifics-out-of-an-rfc-summary
  mark-every-requirement-of-a-superseded-summary-with-its-successor
extraction-completeness-blocking-when-enrolling-a-summary ## Extraction Completeness (BLOCKING when enrolling a summary)
  walk-the-rfc-text-before-enrolling-a-summary
  record-the-walk-as-a-sign-off-artifact
  grandfather-pre-gate-summaries-by-scope-not-allowlist
  verify-the-requirement-text-matches-the-rfc
what-keeps-rfc-testing-valid-the-eight-ratchets ## What Keeps RFC Testing Valid (the eight ratchets)
  what-the-gate-owes-and-where-it-is-described
  what-the-ratchets-miss-an-in-place-weakening
before-implementing-bgp-features ## Before Implementing BGP Features
  read-the-rfc-and-the-exabgp-reference-first
  which-source-wins-rfc-then-exabgp
wire-format-documentation-mandatory ## Wire Format Documentation (MANDATORY)
  never-modify-protocol-code-without-documenting-wire-format
rfc-must-comments-blocking ## RFC MUST Comments (BLOCKING)
  comment-every-enforced-must-with-its-rfc-section
  what-an-rfc-comment-must-document
may-clauses ## MAY Clauses
  ask-the-user-how-to-handle-a-may-clause
common-rfcs ## Common RFCs
  the-rfc-and-code-location-for-each-bgp-feature
