---
kind: fence
level:
stage:
---
```bash
for f in test/plugin/*.ci; do grep -n 'pattern' "$f"; done       # 400 forks
for f in *.go; do grep -l 'Foo' "$f" | xargs sed -n '1p'; done  # 800 forks
```
