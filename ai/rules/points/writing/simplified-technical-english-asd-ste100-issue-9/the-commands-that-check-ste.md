---
kind: table
level:
stage:
---
| Command | What it does |
|---------|--------------|
| `scripts/dev/commit_helper.py create` | **Advisory.** `ste_problems` PRINTS findings for a commit's own `.md`, `.go`, or `.yang` files. It never refuses the commit. It runs on the files of that commit, so the prose it judges has one author |
| `make ze-ste-check` | The same gate over every changed file in the working tree. Run it before you prepare a commit. About 2 seconds |
| `make ze-ste-review` | The whole-tree report. Every finding with its `file:line`, its habit number, and the replacement to use |
| `make ze-ste-review-changed` | The same report for changed files only |
| `python3 scripts/dev/ste_check.py <file>...` | Review named documents |
| `git log -1 --format=%B \| python3 scripts/dev/ste_check.py -` | Review a commit message or a PR body before you send it |
