---
kind: table
level:
stage:
---
| Command | What it does |
|---------|--------------|
| `./le commit create` | **Advisory.** `ste_problems` PRINTS findings for a commit's own `.md`, `.go`, or `.yang` files. It never refuses the commit. It runs on the files of that commit, so the prose it judges has one author |
| `./le ste check` | The same gate over every changed file in the working tree. Run it before you prepare a commit. About 2 seconds |
| `./le ste review` | The whole-tree report. Every finding with its `file:line`, its habit number, and the replacement to use |
| `./le ste review-changed` | The same report for changed files only |
| `./le ste check file <path> [file <path>...]` | Review named documents with the same native ratchet |
