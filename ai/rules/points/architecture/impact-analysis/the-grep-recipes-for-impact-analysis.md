---
kind: fence
level:
stage:
---
```bash
# Who calls this function?
grep -rn "FunctionName" internal/ cmd/ --include="*.go" | grep -v "_test.go"

# Who reads this YANG path?
grep -rn "path/to/leaf" internal/ --include="*.go"

# Who references this registered name?
grep -rn "plugin-name" internal/ pkg/ cmd/ test/ docs/ plan/ .claude/

# Who imports this package?
grep -rn "github.com/ze-software/ze/internal/component/foo" internal/ cmd/ --include="*.go"
```
