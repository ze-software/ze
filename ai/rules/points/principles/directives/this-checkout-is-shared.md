---
kind: directive
level: MUST NOT
stage:
---
**Several sessions work this checkout at once, so a red, an unrelated file, a staged file or an artifact you did not produce MUST NOT be treated as yours to fix, wait for, or carry.** A fully green tree is unreachable by construction. Judge your own change by the evidence your own change produced, name what you could not see, and leave other sessions' unrelated files alone; when a file you edited compiles with concurrent edits from another session, commit the whole changed file and acknowledge them in the commit body (`ai/rules/git-safety.md`).
