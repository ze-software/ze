---
kind: directive
level: MUST
stage:
---
- **You MUST read `docs/contributing/ze-go-style.md` at the START of EVERY session, before any code (owner directive, 2026-08-18).** `.claude/rules/session-start.md` step 2 carries it as a blocking checklist item.
- **The three-trigger gate this point used to state is WITHDRAWN.** It read the guide only before a Go design decision, a review, or an argument about how Ze code is written, and told you not to open it for an ordinary edit. It was set to save context and it cost more than it saved: a session can write Go all day, meet none of those three triggers, and never learn that Ze guards with early returns, splits a compound condition, and states an invariant positively. Measured 2026-08-18, on four `internal/` files in one session.
- That page carries the reasoning behind these rules. It states Ze's three design goals in their order: safety, performance, and developer experience.
- It covers control flow, limits, assertions in a language that has none, memory, errors, goroutines, and the shape of a function. It also covers names, comments, duplicated state, off-by-one errors, and the numbers.
- **The one line on that page with no rule file behind it: a peer MUST NOT be able to panic the daemon.** `panic("BUG:")` marks a state that only a Ze defect reaches. A malformed message from a socket is an operating error, and every parser returns an error for one.
- **When the guide and a rule file disagree, the rule file wins.** The guide explains. A rule file obliges.
- The guide is adapted from TigerStyle, the coding standard of TigerBeetle. Its last table names every deliberate divergence.
