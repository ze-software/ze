---
kind: directive
level:
stage:
---
- **Wire -> RIB:** TCP -> message parse -> UPDATE (WireUpdate, lazy iterator) -> attribute extraction -> pool dedup -> RIB entry (NLRI -> attr refs)
- **API -> Wire:** command parse -> attribute building -> WireUpdate -> PackContext -> wire bytes
- **Plugin <-> Engine:** event -> JSON encode -> write stdin -> plugin processes -> write stdout command -> engine parses -> execute
