---
kind: table
level:
stage:
---
| What you find | What it means |
|---------------|---------------|
| `github.com/ze-software/ze@v0.0.0-<date>-<hash>` | ze was fetched from the proxy. The builddir replaces ze with the working tree, so a build that reaches the proxy for ze did not read the builddir, and it compiled a *pushed commit* rather than your tree |
| A version of a builddir-pinned module that is not the pinned one | `gok` fell back to `go get` and took whatever upstream had. For `github.com/rtr7/kernel` that is the appliance's **kernel** |
