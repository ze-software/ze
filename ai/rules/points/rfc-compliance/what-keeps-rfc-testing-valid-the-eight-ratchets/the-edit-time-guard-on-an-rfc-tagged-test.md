---
kind: note
level:
stage:
---
At edit time the `rfc-tagged-test` guard (`_rfc_tagged_change_err`) blocks a behavior
change to any test carrying an `RFC requirement:` tag, and separately blocks REMOVING the
tag. Removal is checked first and on its own: a tag is a comment, so a behavior comparison
waves its deletion through, after which the test is unguarded and a self-written row in
`test/weakened.md` alone buys any later weakening. Scope is the enclosing test function, not the edited hunk (a tag
sits on the doc comment, so a hunk-scoped guard misses exactly the edit it exists to stop)
and not the whole file (which blocked 331 of 3220 untagged helper functions).
