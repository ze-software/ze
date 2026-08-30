---
title: Rule File Format
when: authoring or editing any rule: a point file under `ai/rules/points/`, its manifest, or a check's binding comment
severity: blocking
related: repo-maintenance
---
directives ## Directives
  a-rule-is-a-directory-of-points
  the-manifest-owns-the-spine-and-the-path-is-the-id
  open-every-rule-with-a-title-and-metadata-block
  required-elements-of-a-rule-header
  keep-a-procedure-out-of-an-always-on-rule
every-directive-states-a-level ## Every directive states a level
  every-directive-states-its-rfc-2119-level
the-trigger-is-a-routing-key ## The trigger is a routing key
  name-a-situation-in-the-when-line-not-a-summary
  what-a-trigger-line-must-do
  give-a-reference-rule-a-trigger-too
  score-a-candidate-trigger-before-splitting-a-rule
the-body-has-a-budget-too ## The body has a budget too
  what-keeps-a-rule-body-short
  move-reference-material-out-of-an-oversized-rule
  mark-a-severity-note-on-a-line-about-another-artifact
  author-a-rule-so-its-directives-reach-the-digest
binding-a-check-to-a-point ## Binding a check to a point
  declare-the-bound-point-in-a-comment-above-the-def
