# Rationale: File Modularity

## Problem

Large files with multiple concerns force Claude to load thousands of lines into context when only one concern is relevant. A 5439-line `reactor.go` with 5 concerns means loading 4000+ irrelevant lines every time any one concern needs attention. This wastes context window and degrades response quality.

## Why One Concern Per File

- **Context efficiency**: Claude reads files in ~2000-line chunks. A 500-line single-concern file means loading exactly what's needed.
- **Navigation**: File names become a table of contents. `reactor_announce.go` tells you where route announcement logic lives without grepping.
- **Parallel work**: Different concerns in different files can be worked on independently across sessions.
- **Review**: Smaller files are easier to review — each file can be understood in isolation.

## Why Not Always Split

Some files are large but single-concern. A pool implementation with interleaved Intern/Compaction/Index operations can't be split without duplicating critical internal state access. A capability registry where the dispatcher references every type is coherent despite size. Splitting these would scatter a single logical unit across files with no benefit.

## Go-Specific Context

Go compiles all files in a package as a single unit. There is zero semantic difference between one 5000-line file and five 1000-line files in the same package. Splitting is purely about human (and AI) readability — the compiler doesn't care.

## Prior Art

`docs/learned/221-file-splitting.md` — first round split 4 files (bgp.go, model.go + their tests). Validated the approach: all tests pass, auto-linter handles imports, no behavioral changes.
