---
kind: note
level:
stage:
---
Each `CommandDecl` a plugin sends at stage 1 carries three optional fields:
`shape`, `columns` and `address-fields`. They say what the command's ANSWER
holds, so the CLI publishes the operators the command supports and refuses the
others by name before dispatch. An absent field is an undeclared field. A plugin
that sends none keeps the behavior it had before the fields existed.
