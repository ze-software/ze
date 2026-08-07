---
kind: fence
level:
stage:
---
```
[ ] Namespace is urn:ze:<component>:<kind> (kind is a colon segment)
[ ] Prefix is short, unquoted, no hyphens; zt/ze not reused
[ ] Module has a revision and a description; no stray organization
[ ] Every IP/prefix/ASN/port/community leaf uses the zt typedef, not a copy
[ ] ze:validate used only for runtime sets, never to duplicate a pattern/range
[ ] Dimensioned leaf: integer + `units <full-word>` + protocol-sane `default`; no unit in the name
[ ] Endpoint: uses zt:listener (bind) or zt:endpoint (target); no combined host:port string
[ ] BFD integration references a bfd profile; auth references the shared key-chains
[ ] Cross-protocol concept matches its siblings (grep OSPF/IS-IS/BGP first)
[ ] Toggles are positive `enabled` booleans; no type empty, no enable/disable enum
[ ] 4-space indent; compact leaves only for type(+default/description)
```
