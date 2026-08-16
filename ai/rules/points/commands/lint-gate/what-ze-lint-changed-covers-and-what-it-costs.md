---
kind: note
level:
stage:
---
This lints all packages with uncommitted Go changes, TWICE: once for the host
build, then again under `GOOS=linux` with the `integration` build tag. The second
pass is the only thing that reads a `//go:build integration` file, and on a
non-Linux host it is the only thing that reads a `//go:build linux` file. Takes
3-10 seconds once both caches are warm; the first run after a checkout pays a
cold `GOOS=linux` analysis, which is minutes.
