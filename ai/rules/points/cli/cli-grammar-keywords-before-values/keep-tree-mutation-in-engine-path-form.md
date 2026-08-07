---
kind: note
level: MUST
stage:
---
If a change means "create/delete/change a node in the YANG config tree",
the command surface MUST stay in engine path form. Do not mirror that
operation as an RPC command just to make the grammar look regular.
