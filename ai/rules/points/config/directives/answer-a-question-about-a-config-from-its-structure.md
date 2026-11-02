---
kind: directive
level: MUST
stage:
---
**Code that asks a question of a config file, Ze's or a peer daemon's, MUST answer it from the file's parsed structure and MUST NOT match its text; `le` tooling, interop labs and test harnesses are included.** A text match answers a question about spelling: it misses the same setting written as a nested block, and it fires on a comment or on the same key in another section. Read a Ze config through the config parser, and a foreign format (`strongswan.conf`) by its section path. The reader's test MUST place the setting nested, in another section, and in a comment.
