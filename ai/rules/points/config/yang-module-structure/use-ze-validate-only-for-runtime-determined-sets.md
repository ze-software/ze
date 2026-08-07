---
kind: directive
level: MUST NOT
stage:
---
- `ze:validate` is for **runtime-determined valid sets only**: registered address families, plugin names, IRR set references, or a union with a literal keyword (`nonzero-ipv4|literal-self`). It MUST NOT duplicate a constraint YANG native `pattern`/`range`/`enumeration` (or an existing `zt` typedef) already expresses. This is the stated contract in `ze-extensions.yang` on the `ze:validate` extension.
