---
kind: note
level:
stage:
---
`scripts/dev/github_workflows_test.go` pins the workflow set: that the nightly is
scheduled-only, runs fuzz AND integration by make-target name, is advisory, does
not smuggle in the QEMU target, that `verify.yml` stays a fast push/pull_request
gate, that every `make <target>` any workflow names exists, and that no
`.woodpecker` pipeline remains.
