---
kind: directive
level: MUST NOT
stage:
---
**`hookStop` (`internal/le/hookruntime/lifecycle.go`) reads your last message and refuses the stop on any phrase below. These are the words the gate matches, case-insensitively, and you MUST NOT end a turn on one.**

| Scanned | Phrases |
|---------|---------|
| Always | `let me know if you`, `would you like me to`, `feel free to`, `if you'd like me to`, `if you want me to`, `happy to help`, `I can <verb> ... if you`, `I'll stop here`, `I'll pause here`, `that's all for now`, `I'll leave ... to you`, `should I proceed/continue/go ahead`, `do you want me to`, `want me to`, `want me to ... or`, `shall I proceed/continue/go ahead/start/keep`, `before I proceed`, `ready for me to`, `or leave/skip/ignore it`, `or should I`, `or something else` |
| Only while a claimed spec is `in-progress` | `what would you like`, `what do you want to do`, `what's next`, `what next` |

**A blocked Stop is NOT an instruction to do the work you just offered.** Answer one question: who asked for it? The user did, so finish it and do not ask again. You thought of it, so DROP it, and MUST NOT start it, size it, or offer it a second time.
**A phrase inside backticks or a closed fence is QUOTED and does not block, and the list is not exhaustive, so a green Stop is no proof you followed this rule.**
