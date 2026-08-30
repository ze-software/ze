---
kind: directive
level: MUST
stage:
---
The YANG container nesting MUST mirror the corrected grammar. When the CLI path
changes from `show interface <name>` to `show interface name <name> detail`, the YANG
tree needs a `name` container under `interface`, then `detail` under that selector. A
filter form such as `show interface type <type>` needs a `type` container that
consumes the typed selector value.
