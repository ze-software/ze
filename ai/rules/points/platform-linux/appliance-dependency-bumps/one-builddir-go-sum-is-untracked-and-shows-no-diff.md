---
kind: note
level:
stage:
---
On step 4, one of the eight go.sum files is **untracked**:
`gokrazy/ze/builddir/github.com/ze-software/ze/go.sum` is gitignored (see
`.gitignore`), because that module is only `replace ze => <repo root>` and every
line of its sum is already in the root `go.sum`. Regenerate it like the rest;
expect no diff. The other seven are tracked locks and DO show a diff.
