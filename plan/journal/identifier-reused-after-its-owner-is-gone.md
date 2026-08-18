# Identifier reused after its owner is gone

A cache or a map is invalidated correctly when the object is deleted, and then
refilled by a reader that resolves the same identifier again. The identifier is
one the kernel or a foreign system is free to hand to a different object, so the
refill answers for the NEW owner while the caller is still asking about the old
one. The invalidation is right; what is missing is any way for the refill to
know which owner the caller meant.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-18 | - | iface netlink monitor | `linkUpdateToEvent` (`internal/component/iface/cmd/monitor_netlink_linux.go`) drops the index from `ifNameCache` on `RTM_DELLINK`, which is correct on its own. `ifName` in the same file refills that entry on any later miss by asking the kernel through `netlink.LinkByIndex`, and its two call sites are the route path and the address path, which `streamNetlinkMonitor` starts as goroutines separate from the link goroutine that does the delete. A kernel interface index is reusable, so a route or address update still in flight for the deleted link resolves through a fresh lookup and caches the NEW link's name against the OLD event. A lookup that fails returns the empty string and caches nothing, so the wrong answer needs the index to have been reused, not merely freed | recorded, not fixed. The effect is the label on an emitted event and not a forwarding decision, and the next link update overwrites the entry, so it self-corrects within one message. The shape is what is worth counting: an invalidation on delete does not bound a cache any reader may refill from an identifier its owner does not control |
