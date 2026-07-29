# Deferrals: mcp2026-0-umbrella

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-28 | spec-mcp2026-0-umbrella | Surface live Ze state (event bus transitions: protocol session up/down, OSPF neighbour and interface state, VRRP state change) to MCP clients, by modelling it as `ze://` resources and pushing `notifications/resources/updated` over `subscriptions/listen` | Owner question 2026-07-28 ("should `subscriptions/listen` be used to inform the client of event bus messages?"). It is a new feature, not a conformance obligation, and the umbrella is a conformance cutover. Umbrella A-4 is confirmed: no server-side MUST obliges Ze to implement `subscriptions/listen`, and Ze has nothing to advertise on it today. Umbrella A-7 records the scope boundary. The design is already fixed by the umbrella's Key Design Decision, so the destination spec starts from a settled approach rather than an open question | `plan/spec-mcp2026-5-state-resources.md` | deferred |
