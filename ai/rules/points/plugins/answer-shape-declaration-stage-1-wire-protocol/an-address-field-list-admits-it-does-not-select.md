---
kind: note
level:
stage:
---
**A declared address-field list is an ADMISSION gate, not a selector.** It
decides whether `| resolve` and `| origin` run at all. It does not decide what
they decorate: `resolveJSON` and `originJSON` walk every key of the answer and
decorate each string that parses as an address. So a plugin that declares one
address field gets decoration on every address in the answer. A plugin that
declares none gets both operators refused by name.
<!-- source: internal/component/command/pipe_resolve.go -- resolveJSON -->
<!-- source: internal/component/command/pipe_origin.go -- originJSON -->
