---
kind: directive
level: MUST
stage:
---
- **You MUST read `docs/contributing/ze-style.md` before a Go design decision, before a review of Go code, and before an argument about how Ze code is written.** For an ordinary edit, this rule and the rules it names carry every mechanical obligation, so do not open the guide.
- That page carries the reasoning behind these rules. It states Ze's three design goals in their order: safety, performance, and developer experience.
- It covers control flow, limits, assertions in a language that has none, memory, errors, goroutines, and the shape of a function. It also covers names, comments, duplicated state, off-by-one errors, and the numbers.
- **The one line on that page with no rule file behind it: a peer MUST NOT be able to panic the daemon.** `panic("BUG:")` marks a state that only a Ze defect reaches. A malformed message from a socket is an operating error, and every parser returns an error for one.
- **When the guide and a rule file disagree, the rule file wins.** The guide explains. A rule file obliges.
- The guide is adapted from TigerStyle, the coding standard of TigerBeetle. Its last table names every deliberate divergence.
