---
kind: directive
level: MUST
stage:
---
When a change creates, deletes, or alters a node in the YANG config tree, the command
surface MUST stay in engine path form. That operation MUST NOT be mirrored as an RPC
command to make the grammar look regular.
