---
kind: fence
level:
stage:
---
```
old_name="bgp-gr"  # what you are renaming away from
new_name="bgp.gr"  # what you are renaming to
# Show every place that still mentions the old name
grep -rn "$old_name" internal/ pkg/ cmd/ test/ docs/ plan/ .claude/ 2>/dev/null
```
