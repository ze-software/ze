---
kind: directive
level: MUST
stage:
---
**When the compound is genuine, MUST exempt it -- MUST NOT split it.** `CheckSiblings` fires on
the mere existence of a sibling matching the LEFT segment, so it cannot tell a real
namespace from two names that share a word by accident (test 3). When test 2 wins -- the
token is one indivisible protocol / LSA / object name -- MUST list the full command path in
`treeNamespaceExempt` (`internal/le/cligrammar/cligrammar.go`) with a one-line reason. It is the
tree-side counterpart of `rootNamespaceExempt`, is counted and printed (`Tree
namespace-exempt`), and leaves every unlisted collision blocking. MUST reach for it only when
splitting would state something false about the object model; `show ospf database
router-information` is the worked case (RFC 7770's RI LSA is an *Opaque* LSA, so filing
it under the `router` sibling -- the Type 1 Router-LSA -- would be wrong). Note the check
only ever looks left, so the same shared-word situation on the right (`summary` /
`asbr-summary`, `external` / `nssa-external`) is never flagged at all.
