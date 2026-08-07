---
kind: note
level:
stage:
---
When a function needs a scratch buffer that the caller cannot provide (e.g.,
the function is called from many sites, or the scratch size varies), use
`sync.Pool`:
