---
title: Running Commands
when: running any test, build, lint, or verification command from Bash, or writing a shell loop that forks or waits
severity: blocking
related: testing, platform-linux, git-safety
---
directives ## Directives
  run-commands-through-native-actions-and-never-poll
  heavy-jobs-are-admitted-by-native-actions-never-typed-raw
  bash-must-not-edit-a-governed-document
cgo-free-builds ## CGO-Free Builds
  ze-is-cgo-free
one-owner-runs-the-suites ## One Owner Runs The Suites
  rule
  never-attribute-a-red-taken-under-contention
  the-proof-a-fix-needs-is-not-a-suite
  known-failure-reruns-stay-focused-until-final-verification
  sample-tests-before-aggregate-suite
bare-go-test-lies-always-pass-the-feature-tags ## Bare `go test` Lies -- Always Pass The Feature Tags
  prefer-a-native-action-or-pass-the-tags-yourself
no-pipes-on-expensive-commands ## No Pipes On Expensive Commands
  never-pipe-an-expensive-command-read-the-log
  tee-is-the-one-allowed-pipe
write-ad-hoc-scratch-under-your-per-session-dir ## Write Ad-Hoc Scratch Under Your Per-Session Dir
  write-ad-hoc-scratch-under-this-session-s-private-directory
your-binaries-live-in-this-session-s-directory ## Your Binaries Live In This Session's Directory -- Ask For The Path
  never-hardcode-bin-ze-ask-for-the-path
never-launch-a-functional-suite-by-running-the-runner-binary ## Never Launch a Functional Suite By Running The Runner Binary
  running-the-runner-binary-gives-a-false-red
the-bash-hook-matches-your-command-text-including-search ## The Bash Hook Matches Your Command Text, Including Search Patterns
  scan-with-grep-tool-never-work-around-the-hook
no-fork-loops ## No Fork Loops
  the-ban-covers-every-bash-call
  batch-with-xargs-or-find-exec-when-a-loop-is-needed
lint-gate ## Lint Gate
  never-invoke-golangci-lint-directly
the-changed-set-selector ## Which Packages "Changed" Means
  read-the-stderr-line-before-you-trust-the-answer
