---
kind: directive
level: MUST NOT
stage:
---
**An RFC sentence quoted in a comment MUST stay byte-exact, its own full stop inside the closing quote mark included, and it MUST NOT be edited to satisfy a linter (`ai/META-DECISIONS.md`).** A comment-punctuation lint rule that asks for a full stop after the quote is configured to exempt quoted text instead: the `godot` settings in `.golangci.yml` carry that exemption.
