---
kind: directive
level: MUST
stage:
---
**You MUST lint through `make`, never by calling `golangci-lint` yourself. A direct call PANICS on a host whose Go is newer than the pinned linter, and the panic looks like a finding.**

```
file requires newer Go version go1.27 (application built with go1.26)
```

That is the environment, not your code. The `Makefile` exports `GOTOOLCHAIN` derived from `go.mod`, so every package compiles with the toolchain the pinned `golangci-lint` can read. A bare invocation inherits none of it, picks up the host toolchain, and produces export data the linter refuses.

**Four separate agents met this on 2026-08-23. Each one reported the panic as a possible finding, and only then found it was environmental.** A panic reads as a defect, and a linter panicking on the code you just wrote reads as a defect in that code. It is neither.

The same applies to any tool the `Makefile` configures through the environment. If a target exists, the target is the interface, and reaching past it into the underlying binary drops the configuration that makes it work.
