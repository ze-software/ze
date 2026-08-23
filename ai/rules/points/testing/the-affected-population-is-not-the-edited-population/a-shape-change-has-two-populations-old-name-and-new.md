---
kind: directive
level: MUST
stage:
rationale: plan/journal/green-that-could-not-have-been-red.md
---
**When a payload SHAPE changes, you MUST search for the NEW name as well as the old one.** Searching what you REMOVE finds code that stops working. It cannot find code that breaks because of what you ADD. An added key CAN land in a branch that already reads it for a different producer. That branch then handles the new payload wrongly and quietly. Nothing prompts this search, because the new name is yours and feels safe.

**For the OLD name, a hit is not yet a consumer: you MUST establish WHICH producer emits the key it matched.** One key name can have several producers, and only some of them are yours. A key name is not a producer.

Measured on 2026-08-23, converting `show bgp rib` from `{adj-rib-in: {peer: [routes]}}` to one envelope under `routes`. 120 `.ci` files mentioned the old keys and 6 parsed the payload. Five were consumers of the changed command. The sixth, `med-removal-before-decision.ci`, parsed `adj-rib-in` from `show bgp adj-rib-in`, a different plugin whose handler still returns that shape (`rib_commands.go`, `internal/component/bgp/plugins/adj_rib_in`). Converting it broke a passing test. No search over the old key can say which of the six is which.

The new-name half cost more. `extractRoutes` (`internal/component/lg/handler_ui.go`) held a branch reading `routes` for a different, already-flat producer, ABOVE the grouped-shape branch. Once `show bgp rib` answered `routes`, that branch became the path every RIB answer took and returned the rows untouched, so no row carried `peer-address` and the attributes stayed wrapped. The looking-glass graph drew nothing. **No search for the old key reaches that file, and neither does a search for consumers of that command, because it consumes a generic payload.**

**The audit is owed AGAIN for the fix, and this is the half a reader will miss.** The natural reading of the rule above is "audit the consumers before you change the shape". But a repair to a shape change is itself a shape change. It lands in a function whose branches each carry a prior contract, so it earns its own pass over both populations.

Same function, same hour. The first repair to `extractRoutes` normalized every element and dropped what it could not. That broke `TestExtractRoutes/prefixes fallback`. For some producers `prefixes` is a list of bare prefix STRINGS, and that branch had always passed its elements through untouched. The final version states each branch's prior contract instead. Flat branches pass through what they cannot normalize. The grouped branch still skips non-records. It was found only because an existing test sat in the package.

**It stayed quiet because the colliding branch did not fail.** It returned a valid empty result, and `No routes found` reads as a true answer about an empty RIB. A branch that accepts what it cannot handle and answers plausibly is the shape this file collects. Refusing would have been loud and correct.

**The pre-push gate caught it, and the focused tests did not.** Six red fixtures across the looking-glass and MCP suites, none in the package that changed. A focused run covers the code you edited, and a shape change is defined by who READS it.
