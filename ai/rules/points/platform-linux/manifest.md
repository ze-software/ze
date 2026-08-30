---
title: Linux, QEMU and the Appliance
when: writing Linux-only code, changing the installer initrd, or bumping and booting an appliance dependency
severity: blocking
related: completion, testing, git-safety
---
directives ## Directives
  test-every-linux-only-path-inside-qemu
linux-only-functional-ci-tests-run-via-qemu-never-natively ## Linux-only functional (`.ci`) tests run via QEMU, never natively
  mark-a-kernel-touching-ci-test-needs-linux
  never-use-skip-os-in-place-of-needs-linux
  know-that-a-caps-test-moves-to-the-nightly-run
  choose-between-the-tight-loop-and-the-full-pass
  both-functional-targets-boot-zes-runtime-kernel
how-to-write-a-qemu-integration-test ## How to Write a QEMU Integration Test
  use-integration-linux-for-anything-that-touches-the-kernel
  never-require-physical-hardware
  skip-not-fail-when-a-prerequisite-is-absent
  add-your-package-to-the-native-qemu-inventory
  a-dataplane-counter-owes-an-egress-and-a-positive-control
interop-labs-and-docker-based-tests-need-a-qemu-runner-too ## Interop Labs and Docker-Based Tests Need a QEMU Runner Too
  ship-a-qemu-path-beside-every-docker-interop-lab
  the-steps-that-build-a-qemu-runner-for-a-lab
  add-the-row-and-ship-both-targets-together
  use-a-custom-kernel-only-when-a-config-is-missing
what-actually-runs-these-suites ## What Actually Runs These Suites
  never-assume-ci-runs-the-qemu-integration-suite
  every-qemu-and-interop-target-needs-a-caller
initrd-prefer-procfs-sysfs-over-external-commands ## Initrd: Prefer Procfs/Sysfs Over External Commands
  use-in-process-go-in-place-of-each-external-tool
  isolate-an-unavoidable-syscall-in-a-linux-helper
appliance-dependency-bumps ## Appliance Dependency Bumps
  bump-the-pin-instead-of-dismissing-the-alert
  never-vendor-the-modcache-or-hand-edit-a-go-sum
  the-runbook-that-bumps-the-vendored-init
  never-accept-a-skip-as-boot-evidence
  stage-the-re-vendor-through-the-commit-helper
  download-into-the-modcache-with-modcacherw
  the-growth-that-is-a-defect
  never-rm-rf-the-modcache-delete-named-versions
  review-the-builddir-pins-every-release-cycle
  what-each-pin-review-must-do
  the-pins-move-only-through-the-runbook-never-through-a-bot
  record-a-source-offer-sign-off-before-distributing
  keep-the-pseudo-versions-until-upstream-cuts-a-tag
