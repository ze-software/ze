---
kind: fence
level:
stage:
---
In the comment syntax the file itself uses: `//` in a `.go` test, `#` in a `.ci` or
`.et` scenario. Both are accepted on a `#` carrier, so the `# // test-relax:` already
in 315 scenarios keeps working.

```
// test-relax: <why this test/assertion no longer applies>
# test-relax: <why this test/assertion no longer applies>
```
