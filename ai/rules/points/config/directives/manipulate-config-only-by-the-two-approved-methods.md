---
kind: directive
level: MUST
stage:
---
**Config content MUST be manipulated by one of two methods only: a parsed YANG tree when a loaded tree is in memory, or `set` command lines when building or merging config text.** Raw text surgery, a custom merge function that parses config syntax outside the config system, and any manipulation that infers structure from text patterns MUST NOT be used. An unknown key MUST fail at every level with the closest valid key suggested, and MUST NOT be ignored.
