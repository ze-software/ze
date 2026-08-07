---
kind: note
level:
stage:
---
`<build-tag>` then `<gated-package>`. Tags are `ze_<feature>` by convention.
The generator gates both `<pkg>` and `<pkg>/yang` when those packages are
discovered. The direct package covers RPC/registration side effects, and the
YANG package covers config or command schema. **Every other consumer
DERIVES from this file**. Do NOT hand-edit a parallel list:
