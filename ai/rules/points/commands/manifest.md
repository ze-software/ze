---
title: Running Commands
when: running any test, build, lint, or verification command from Bash, or writing a shell loop that forks or waits
severity: blocking
related: testing, platform-linux, git-safety
---
directives ## Directives
  run-commands-through-make-and-never-poll
bare-go-test-lies-always-pass-the-feature-tags ## Bare `go test` Lies -- Always Pass The Feature Tags
  bare-go-test-omits-the-feature-build-tags
  prefer-a-make-target-or-pass-the-tags-yourself
  go-test-invocation-carrying-the-feature-tags
  a-git-archive-scratch-tree-has-the-same-trap
  phantom-reds-have-cost-real-debugging-time
  how-to-recognize-a-missing-tags-failure
no-pipes-on-expensive-commands ## No Pipes On Expensive Commands
  never-pipe-an-expensive-command-read-the-log
  tee-is-the-one-allowed-pipe
  where-verify-writes-its-log-and-how-to-read-it
write-ad-hoc-scratch-under-your-per-session-dir ## Write Ad-Hoc Scratch Under Your Per-Session Dir
  a-fixed-name-in-tmp-collides-between-sessions
  write-ad-hoc-scratch-under-this-session-s-private-directory
  session-scratch-sh-gives-you-a-private-directory
  nothing-is-deleted-automatically-and-what-stays-put
your-binaries-live-in-this-session-s-directory ## Your Binaries Live In This Session's Directory -- Ask For The Path
  every-binary-is-built-in-this-session-s-directory
  never-hardcode-bin-ze-ask-for-the-path
  use-make-ze-path-to-get-the-binary
  why-the-binaries-live-with-the-session
  the-session-store-is-seeded-on-the-first-build
  why-test-binaries-live-in-a-private-bin
never-launch-a-functional-suite-by-running-the-runner-binary ## Never Launch a Functional Suite By Running The Runner Binary
  running-the-runner-binary-gives-a-false-red
  the-make-target-builds-a-zetest-tagged-pair
  the-same-trap-as-bare-go-test-one-layer-out
  how-to-run-a-suite-one-test-or-a-vm-test
  the-runner-s-failure-hints-repeat-the-bad-launch
the-bash-hook-matches-your-command-text-including-search ## The Bash Hook Matches Your Command Text, Including Search Patterns
  the-hook-matches-the-command-string-not-intent
  a-grep-whose-pattern-spells-a-banned-verb
  scan-with-python-never-work-around-the-hook
  python-scan-that-keeps-the-verb-off-the-command-line
  why-the-hook-is-coarse-on-purpose
no-fork-loops ## No Fork Loops
  bad
  per-file-fork-loops-that-cost-hundreds-of-forks
  good
  one-recursive-grep-instead-of-a-loop
  when-a-loop-is-unavoidable
  batch-with-xargs-or-find-exec-when-a-loop-is-needed
  find-exec-batching-many-files-into-one-call
  scope
  the-ban-covers-every-bash-call-and-script
no-poll-loops ## No Poll Loops
  what-to-do-for-each-kind-of-wait
lint-gate ## Lint Gate
  the-problem
  the-per-edit-hook-only-sees-changed-lines
  the-rule
  run-the-lint-gate-before-claiming-go-work-done
  make-ze-lint-changed
  what-ze-lint-changed-covers-and-what-it-costs
  fix-every-lint-issue-before-claiming-done
  when-to-run
  lint-moments-and-the-action-each-one-needs
  what-it-catches-that-per-edit-hooks-miss
  cross-file-defects-only-package-lint-finds
rationale ## Rationale
  fork-cost
  measured-fork-cost-on-macos
  poll-cost
  abandoned-poll-loops-made-the-suites-flaky
  the-harm-is-the-wake-and-its-lifetime
