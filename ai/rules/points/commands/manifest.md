---
title: Running Commands
when: running any test, build, lint, or verification command from Bash, or writing a shell loop that forks or waits
severity: blocking
---
directives ## Directives
  run-commands-through-native-actions-and-never-poll
  heavy-jobs-are-admitted-by-native-actions-never-typed-raw
  bash-must-not-edit-a-governed-document
  never-hardcode-bin-ze-ask-for-the-path
  ze-is-cgo-free
no-pipes-on-expensive-commands ## Pipes
  never-pipe-an-expensive-command-read-the-log
write-ad-hoc-scratch-under-your-per-session-dir ## Scratch
  write-ad-hoc-scratch-under-this-session-s-private-directory
