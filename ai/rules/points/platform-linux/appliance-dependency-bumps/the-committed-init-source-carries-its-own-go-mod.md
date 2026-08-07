---
kind: note
level:
stage:
---
`gokrazy/modcache/.gitignore` ignores everything except the gokrazy init source
(`github.com/gokrazy/gokrazy@*/**`). That committed source includes upstream's own
`go.mod`, and GitHub's dependency graph scans **every** `go.mod` in the repo as a
manifest. When upstream's `go.mod` names a version with a later advisory, the alert
fires on that file even though the image never builds the vulnerable version (the
builddir modules pin the fix and MVS takes the max).
