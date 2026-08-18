---
title: Repository Maintenance
when: adding or changing a feature, tool, gate, hook, runtime dependency, or generated file, looking up which check enforces a rule, or reporting development friction
severity: blocking
related: rule-format, writing, evidence, testing
---
directives ## Directives
  update-the-discovery-path-in-the-same-work
discovery-updates ## Discovery Updates
  trigger
  when-this-rule-applies
  changes-that-need-a-discovery-update
  private-refactors-that-set-a-pattern-still-apply
  required-discovery-artifacts
  update-every-row-that-applies
  which-document-each-change-obliges-you-to-update
  never-add-a-page-no-navigation-path-reaches
  mechanical-checklist
  answer-these-before-implementation-is-complete
  the-discovery-questions-and-where-each-answer-goes
  current-discovery-surfaces
  use-these-before-inventing-a-new-mechanism
  the-discovery-surface-that-answers-each-need
  add-the-missing-discovery-link-before-claiming-done
doctor-checks ## Doctor Checks
  the-rule
  the-dependency-owner-must-register-the-doctor-check
  where-doctor-checks-belong-and-who-owns-them
  the-doctor-check-each-new-dependency-needs
  diagnostic-code-convention
  name-and-register-every-diagnostic-code
  mechanical-check
  verify-the-check-is-registered-and-explainable
  where-to-register-checks
  the-registration-mechanism-for-each-owner
  keep-an-unowned-check-in-the-doctor-package
  test-requirement
  the-tests-a-new-doctor-check-must-carry
  what-each-doctor-test-type-proves-and-where-it-lives
  linux-only-checks-need-linux-tests-and-qemu
canonical-sources-and-sync-direction ## Canonical Sources and Sync Direction
  sync-flows
  sync-generated-files-from-their-canonical-source
  rule-placement
  keep-shared-rules-in-ai-rules-and-render-them
  what-the-session-payload-artifacts-hold
  change-the-ladder-not-the-core-md-membership
  mechanical-check
  edit-the-canonical-source-not-the-generated-file
  drift-detection
  git-diff-cannot-show-drift-in-the-mirrors
  banned-actions
  banned-edits-and-the-canonical-source-to-use
hook-to-rule-mapping ## Hook-to-Rule Mapping
  what-this-mapping-table-answers
  architecture-checks-live-in-three-python-dispatchers
  shell-hooks-were-folded-into-one-dispatcher-per-trigger
  what-each-dispatcher-runs-on-and-contains
  the-hooks-that-stayed-standalone
  edit-the-dispatcher-function-then-check-parity
  reads-never-block-and-some-write-a-freshness-marker
  keep-the-session-id-resolvers-in-agreement
  pretooluse-checks-block-before-the-tool-runs
  lsp-gate-block-until-lsp-sh-standalone
  what-the-lsp-gate-blocks-and-until-when
  bash-pretool-bash-py
  the-bash-checks-and-what-each-one-blocks
  why-the-commit-time-gates-moved-out-of-the-hook
  golangci-lint-run-also-runs-standalone-on-bash-git-commit
  write-edit-pretool-writeedit-py
  the-write-edit-checks-and-what-each-one-blocks
  why-format-alloc-is-live-and-what-it-adds
  agent-skill-task-agent-pretool-agent-skill-py
  the-task-agent-checks-and-what-each-one-blocks
  posttooluse-checks-run-after-the-tool-completes
  the-posttooluse-checks-and-what-each-one-does
  why-validate-spec-sh-stayed-standalone
  ze-verify-runs-the-wiring-and-doc-drift-gate
  changed-file-gates-inside-ze-verify-wiring-docs
  these-gates-are-make-targets-scoped-to-changed-files
  the-wiring-docs-gates-and-what-each-one-blocks
  ze-verify-runs-the-tree-wide-half-of-ze-validate
  the-validate-checks-and-which-half-the-gate-runs
  why-two-validate-checks-stay-out-of-the-gate
  prose-gate-asd-ste100
  the-prose-gate-compares-each-file-with-its-own-head
  the-prose-gate-checks-and-what-each-one-does
  commit-time-gates-scripts-dev-commit-helper-py
  the-commit-gates-run-at-script-creation-time
  the-commit-time-gates-and-what-each-one-does
  hook-tests-make-ze-hook-test
  what-each-hook-test-runner-covers
  session-lifecycle-hooks
  stdout-reaches-the-model-stderr-costs-no-context
  the-session-lifecycle-hooks-and-what-each-one-does
  pre-flight-checklist-by-file-type
  the-checklist-checks-run-inside-the-dispatchers
  any-go-file-under-internal
  what-fires-on-a-go-file-under-internal
  test-files-test-go-ci
  what-fires-on-a-test-file
  spec-files-plan-spec-md
  what-fires-on-a-spec-file
  python-files-py
  what-fires-on-a-python-file
  commits
  what-fires-on-a-commit
gate-population ## Gate Population
  rule
  state-what-the-gate-cannot-see
  anchor-a-structural-read-on-a-marker-not-a-position
  compile-the-tests-before-you-call-it-committable
ze-project-knowledge ## Ze Project Knowledge
  project-knowledge-not-in-other-rules
  project-facts-that-no-other-rule-carries
  mistake-log
  how-to-write-a-mistake-log-entry
  the-recurring-mistakes-and-their-corrections
own-mistakes ## Your Own Mistakes
  rule
  state-the-root-cause-never-the-instance
  an-example-narrows-the-rule-to-itself
  route-the-half-a-rule-cannot-reach-to-a-spec
friction-reporting ## Friction Reporting
  report-immediately-when
  the-friction-categories-worth-reporting
  format
  the-friction-report-template
  timing
  file-friction-early-chat-is-not-filing
  do-not-report
  what-does-not-count-as-friction
