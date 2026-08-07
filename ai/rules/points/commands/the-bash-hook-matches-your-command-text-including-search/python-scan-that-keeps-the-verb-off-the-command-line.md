---
kind: fence
level:
stage:
---
```
python3 - <<'PY'
import glob, re
broad = re.compile(r"add\s+(-A|--all|\.)|commit\s+-a")
for s in glob.glob("tmp/commit-*.sh"):
    if broad.search(open(s, errors="replace").read()):
        print("BROAD-STAGE", s)
PY
```
