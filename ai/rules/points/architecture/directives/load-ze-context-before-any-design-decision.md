---
kind: directive
level: MUST
stage:
---
**Before any design decision (communication mechanism, naming, package placement, platform backend, lifecycle), load the relevant context below. Trained instincts about "how software works" are wrong here: ze has opinions.**
**Complete the "Before Writing Code" checklist before writing any code, tests, or documentation.**
**Before any spec: READ source, document current behavior, preserve by default.**
**Before modifying a file, check what else needs to change. Changes to certain file types have predictable ripple effects.**
**Trace full data flow before writing or reviewing specs.**
**Where a Go package lives under `internal/` is decided by dependency direction, not by size or age. Three tiers, two mechanical axes. New code MUST land in the correct tier; an engine in the wrong tier fails `./le verify worktree`.**
**Persist runtime state through the managed zefs store, never as a loose file.**
**Ze differs from typical Go projects in specific, load-bearing ways. An AI trained on standard Go patterns will default to the wrong approach unless it reads the divergence tables below. Each entry names the standard approach, the Ze approach, the rule that governs it, and a one-line reason.**
