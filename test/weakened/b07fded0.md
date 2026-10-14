| Test | Reason |
|---|---|
| configureAttributeDefaults | Replaced, not a weakening: the absentBase global it stored into is deleted, because each modifyDef now carries its delivery's base (R-3, one snapshot). configuredBase parses the same block through the same parseAttributeDefaults and each test hands the result to its definition, so every assertion still runs against a configured base. |
| storeAbsentBase | Replaced, not a weakening: it set and restored the deleted absentBase global. The empty-base case now passes map[string]uint32{} to currentForArithmetic, and the no-base case uses a definition no delivery built (nil absentBase), which is the state the pre-configure window it modelled maps to. |
