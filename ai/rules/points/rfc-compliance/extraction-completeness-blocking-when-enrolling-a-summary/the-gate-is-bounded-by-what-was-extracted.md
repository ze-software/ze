---
kind: note
level:
stage:
---
`make ze-rfc-check` verifies that every requirement **listed** in a summary is
covered. It cannot know about an obligation nobody wrote down. A green gate is
bounded by what was extracted, so a missing extraction is invisible to it and to
any audit that only re-checks classifications.
