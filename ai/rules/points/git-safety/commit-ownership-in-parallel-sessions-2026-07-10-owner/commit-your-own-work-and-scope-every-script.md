---
kind: directive
level: MUST
stage:
---
**When several sessions work the same tree, each session MUST commit the features it is in charge of implementing. You MUST NOT leave your own finished work uncommitted for another session to sweep or strand.**
**You MUST scope every commit script to your own files, one `file <path>` keyword per file, and MUST verify `git diff --cached --name-only` shows nothing foreign before running it.**
