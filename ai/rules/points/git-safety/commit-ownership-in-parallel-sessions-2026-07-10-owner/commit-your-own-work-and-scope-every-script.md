---
kind: note
level: MUST
stage:
---
When several sessions work the same tree, each session MUST commit the
features it is in charge of implementing -- never leave your own finished
work uncommitted for another session to sweep or strand. Scope every commit
script to your own files (explicit `--file` lists; verify
`git diff --cached --name-only` shows nothing foreign before running it).
