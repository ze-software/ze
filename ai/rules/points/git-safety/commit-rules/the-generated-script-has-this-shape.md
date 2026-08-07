---
kind: note
level:
stage:
---
The generated script has this shape:
```bash
#!/bin/bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# Commit a: type: subject line
# Lesson: not needed - hook fix, no novel pattern
git add -- \
  file1.go \
  file2.go \
  file3_test.go
git commit -F tmp/commit-msg-<SESSION>-a.txt
```
