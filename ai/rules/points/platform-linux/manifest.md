---
title: Linux, QEMU and the Appliance
when: writing Linux-only code, changing the installer initrd, or bumping and booting an appliance dependency
severity: blocking
related: completion, testing, git-safety
---
directives ## Directives
  test-every-linux-only-path-inside-qemu
when-qemu-tests-are-required ## When QEMU Tests Are Required
  the-qemu-test-each-linux-only-change-requires
linux-only-functional-ci-tests-run-via-qemu-never-natively ## Linux-only functional (`.ci`) tests run via QEMU, never natively
  mark-a-kernel-touching-ci-test-needs-linux
  the-needs-linux-marker
  where-the-needs-linux-option-is-implemented
  how-needs-linux-behaves-on-each-host
  one-vm-runs-them-all-from-two-entry-points
  choose-between-the-tight-loop-and-the-full-pass
  qemu-discovers-needs-linux-tests-with-no-wiring
  both-functional-targets-boot-zes-runtime-kernel
  decision-rule
  which-option-marker-each-ci-test-needs
  declare-the-capability-never-hide-the-test-with-skip-os
  why-linux-alone-is-not-enough-declare-net-admin
  a-caps-typo-fails-to-parse-on-every-host
  know-that-a-caps-test-moves-to-the-nightly-run
  never-use-skip-os-in-place-of-needs-linux
how-to-write-a-qemu-integration-test ## How to Write a QEMU Integration Test
  use-integration-linux-for-anything-that-touches-the-kernel
  never-require-physical-hardware
  skip-not-fail-when-a-prerequisite-is-absent
  add-your-package-to-the-native-qemu-inventory
  a-dataplane-counter-owes-an-egress-and-a-positive-control
interop-labs-and-docker-based-tests-need-a-qemu-runner-too ## Interop Labs and Docker-Based Tests Need a QEMU Runner Too
  ship-a-qemu-path-beside-every-docker-interop-lab
  the-pattern-do-every-step-in-the-same-change
  the-steps-that-build-a-qemu-runner-for-a-lab
  the-docker-and-qemu-targets-of-each-lab
  add-the-row-and-ship-both-targets-together
  use-a-custom-kernel-only-when-a-config-is-missing
what-the-qemu-vm-provides ## What the QEMU VM Provides
  the-vm-is-an-alpine-live-system
  the-facilities-the-vm-provides
what-the-qemu-vm-does-not-provide ## What the QEMU VM Does NOT Provide
  what-is-missing-and-what-to-use-instead
running-qemu-tests ## Running QEMU Tests
  the-command-that-runs-every-integration-package
  what-the-first-run-downloads-and-caches
  install-qemu-on-macos-for-hvf-acceleration
  join-the-kvm-group-on-linux-or-qemu-will-not-run
reference-implementations ## Reference Implementations
  reference-files-to-copy
what-actually-runs-these-suites ## What actually RUNS these suites
  why-this-section-names-the-gate
  ci-runs-on-github-actions-not-codeberg
  where-each-suite-runs-and-whether-it-blocks
  notes-on-the-nightly-row
  the-cron-lives-in-the-workflow-and-expires-when-idle
  why-the-integration-suite-runs-on-github-and-not-codeberg
  never-assume-ci-runs-the-qemu-integration-suite
  every-qemu-and-interop-target-needs-a-caller
  what-pins-the-workflow-set
common-mistakes ## Common Mistakes
  common-mistakes-and-their-fixes
initrd-prefer-procfs-sysfs-over-external-commands ## Initrd: Prefer Procfs/Sysfs Over External Commands
  read-this-before-changing-the-installer-initrd
  detect-state-through-proc-and-sys-never-exec-command
  procfs-decision-table
  the-procfs-source-for-each-need
  in-process-replacements-no-external-client
  the-shell-operations-that-are-now-in-process-go
  use-in-process-go-in-place-of-each-external-tool
  isolate-an-unavoidable-syscall-in-a-linux-helper
appliance-dependency-bumps ## Appliance Dependency Bumps
  a-modcache-dependabot-alert-is-a-stale-manifest
  why-the-alert-fires
  gok-has-no-vendor-support-so-a-modcache-is-checked-in
  the-committed-init-source-carries-its-own-go-mod
  never-vendor-the-modcache-or-hand-edit-a-go-sum
  the-fix-bump-the-vendored-init
  the-runbook-that-bumps-the-vendored-init
  one-builddir-go-sum-is-untracked-and-shows-no-diff
  use-a-proof-that-actually-boots-an-image
  which-boot-proof-asserts-what
  never-accept-a-skip-as-boot-evidence
  git-safety-for-the-re-vendor
  stage-the-re-vendor-through-the-commit-helper
  cache-permissions
  download-into-the-modcache-with-modcacherw
  which-tools-already-set-modcacherw
  module-cache-hygiene-what-may-accumulate-and-what-must-never
  the-modcache-is-never-garbage-collected
  the-growth-that-is-expected
  the-growth-that-is-a-defect
  what-each-unexpected-module-version-means
  the-window-where-every-pin-was-discarded
  every-image-build-now-runs-from-a-prepared-instance
  the-tests-that-gate-instance-preparation
  never-rm-rf-the-modcache-delete-named-versions
  do-not-just-dismiss-the-alert
  bump-the-pin-instead-of-dismissing-the-alert
  proactive-review-cadence-builddir-pins
  why-dependabot-is-off-for-the-builddir-and-modcache
  review-the-builddir-pins-every-release-cycle
  what-each-pin-review-must-do
  the-pins-move-only-through-the-runbook-never-through-a-bot
  gplv2-source-offer-sign-off-rtr7-kernel-unresolved-flag-only
  the-image-ships-a-gplv2-kernel-and-owes-its-source
  record-a-source-offer-sign-off-before-distributing
  root-module-pseudo-version-pins-no-upstream-tags
  why-the-root-pseudo-version-pins-are-not-a-defect
  the-root-deps-that-publish-no-semver-tag
  keep-the-pseudo-versions-until-upstream-cuts-a-tag
