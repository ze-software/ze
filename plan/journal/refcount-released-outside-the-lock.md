# Refcount released outside the lock that guards the map

The decrement that decides an object is free happens outside the lock that
guards the map the object is reachable through. The atomic makes the COUNT
safe and leaves the DECISION unsafe: another goroutine takes the lock, finds
the entry still present, and acquires it in the window before the releaser
deletes it. One caller then holds a live object whose shared parts the other
caller is releasing.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-18 | - | bgp rib route store | `releaseRoute` (`internal/component/bgp/rib/store.go`) calls `route.Release()` BEFORE it takes `rs.routesMu`. `Release` (`internal/component/bgp/rib/route.go`) does the atomic decrement and returns true at zero, and only then does `releaseRoute` lock, delete the index from `rs.routes`, unlock, and release the interned attributes and the interned NLRI. `internRoute` holds `rs.routesMu` across its whole body and does `existing.Acquire()` on any entry it finds, so an intern arriving in that window takes the count 0 to 1 and hands its caller a route that is already condemned. The releaser then deletes the map entry under that caller and releases the interned attributes and NLRI the returned route still points at. What the count reads at the moment of the decrement is not what it reads when the map is edited, and nothing carries the decision across the gap | recorded, not fixed, and the race is the smaller half of what is here. `RouteStore` has NO production user: `NewRouteStore`, `internRoute` and `releaseRoute` are each reached only from `store_test.go`, so the whole intern-and-release pair is unwired and the defect cannot fire today. The lock repair itself is small and its shape is constrained: take `routesMu`, decrement inside it, delete, unlock, and release the interned attributes and NLRI only AFTER the unlock, because `getOrCreateAttrStore` takes `rs.mu` while `internRoute` already takes `rs.mu` before `routesMu`, so holding both the other way round inverts the order. Releasing after the unlock is safe because `AttributeStore.Release` (`internal/component/bgp/store/attribute.go`) carries its own lock and refcount. The decision that comes first is whether the store is wired or deleted, which is not a defect fix |
