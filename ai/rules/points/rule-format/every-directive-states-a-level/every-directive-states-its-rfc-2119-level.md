---
kind: directive
level: MUST
stage:
---
- Every point whose `kind` is `directive` MUST state its obligation in RFC 2119 language, and its `level:` MUST name the strongest TIER the body states. A directive whose weight a reader infers from tone is a directive two readers weigh differently.
- The tiers are MAY, then SHOULD with SHOULD NOT, then MUST with MUST NOT. RFC 2119 ranks obligation by STRENGTH and does not rank MUST against MUST NOT, so a point stating both MAY declare whichever its central clause carries. Ordering the two would force a point whose central clause is a prohibition to declare MUST, and the prohibition would go unrecorded.
- Use MUST and MUST NOT for an obligation. Use SHOULD and SHOULD NOT for a strong default that a reader MAY depart from with a stated reason. Use MAY for a permission. The linter accepts SHALL, SHALL NOT, REQUIRED, RECOMMENDED, NOT RECOMMENDED and OPTIONAL. It maps each keyword to its level, so `level:` has one spelling per level.
- The lowercase spellings `must`, `shall`, `should` and `may` MUST NOT appear in a directive body. They read as the obligation word and carry none of its force, and `ai/rules/writing.md` bans the hedging spelling outright. Capitalise the keyword, or rewrite the sentence so it carries no modal.
- A block that states no obligation is `kind: note` or `kind: table`, never `kind: directive`. The gate is scoped to directives on purpose: a two-column lookup gains a word and no obligation from being made to say MUST.
- Text inside a code span, fenced block, or Markdown blockquote is quoted, never stated. Neither gate reads it. Quoted text keeps its own spelling.
- `writePointLanguage` in `internal/le/hookruntime/writeedit.go` refuses the write, and `./le rules lint` refuses the finished tree. A Write carries the whole point, so a missing keyword is refused there. An Edit or MultiEdit carries fragments, so the hook refuses only lowercase modals in those fragments.
