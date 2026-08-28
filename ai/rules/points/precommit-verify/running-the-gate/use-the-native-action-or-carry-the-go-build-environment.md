---
kind: directive
level: MUST
stage:
---
**You MUST use the owning native action, or carry its Go build environment yourself.**
`internal/le/gotoolchain` derives the pinned toolchain, repository build cache,
module cache, feature tags, timeout, and process ceiling used by `./le test-unit`
and verification. A bare `go test` uses the user's defaults and can compile a
different feature population. If no native action owns the focused run, pass
the required tags, `GOCACHE`, `GOMODCACHE`, `CGO_ENABLED`, and race setting
explicitly. Race-built test executables never ship or serve as release evidence
(`ai/rules/commands.md`, "Bare `go test` Lies").
