# Spec: fixit-plugin-event-subscription

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/plugin/server/dispatch.go`, `internal/component/plugin/server/startup.go`, `internal/component/plugin/resolve.go` - subscription registration + event delivery
4. `internal/component/plugin/server/subscribe.go` - runtime subscribe parser (the working path)

## Task

Fix two related gaps in the plugin event pub/sub surface that make non-BGP
component events (IPsec/`vpn-ipsec`, and any future namespace) hard to observe
from an external plugin:

**Gap A -- startup subscriptions are namespace-locked to `bgp`.** A plugin's
`SetStartupSubscriptions(...)` (`pkg/plugin/sdk/sdk_callbacks.go` `SetStartupSubscriptions`)
carries an event list but no namespace; the engine registers it via
`registerSubscriptions`, which hardcodes the namespace to the single default
registered at startup. That default is `bgp` and only `bgp`
(`RegisterDefaultEventNamespace` is called exactly once, by the BGP component).
So a plugin cannot subscribe to `vpn-ipsec/sa-up` (or any non-bgp namespace) at
startup at all -- the subscription silently lands in the `bgp` namespace and
never matches.

**Gap B -- delivered events carry a bare payload with no namespace/name
envelope.** When the engine delivers an event to an external plugin, it sends
only the marshaled payload JSON. A subscriber that watches several event types
cannot tell from the wire which event arrived, because the payload has no
`namespace`/`name` fields of its own (e.g. the IPsec `SAEvent` carries peer
fields only).

**Why this matters (concrete):** implementing `spec-test-coverage-gaps` AC-2
(the `.ci` `expect=event` directive), the engine-step executor could not use a
single up-front subscription and match by name. The workaround was a per-step
*exclusive* subscription (subscribe -> wait for any delivery -> unsubscribe),
which only works because the *runtime* `request subscribe <ns> event <name>`
command DOES accept an explicit namespace (`ParseSubscription`). The startup
path and the delivery envelope are the gaps; the runtime command is the escape
hatch that proved the underlying delivery works.

**Gap C (found during design 2026-07-16, NOT in the original skeleton) -- the
`"*"` startup subscription is dead.** `"*"` is not a registered event type
anywhere, so `parseEventString` (`internal/component/plugin/server/dispatch.go:130`)
resolves it to `events.EventTypeUnknown` (0) and `Subscription.Matches`
(`internal/component/plugin/server/subscribe.go:70-72`) requires exact
`EventTypeID` equality. Every real event carries a non-zero ID, so
`SetStartupSubscriptions([]string{"*"}, ...)` -- used by
`internal/plugins/exabgp/main_sdk.go:71` and
`internal/plugins/exabgp/bridgeplugin/internal.go:93` -- registers a subscription
that can never match. This is the same choke point as Gap A (`registerSubscriptions`)
and cannot be fixed without touching it, so it is raised here rather than
silently left behind. ~~**Scope decision pending Thomas.**~~ -> RESOLVED (user Decision 2026-07-16, recorded immediately below): the expansion is approved into scope; Gap C is IN.

### Scope

- IN (Gap A): let a plugin express a namespace in its startup subscriptions so
  non-bgp events can be subscribed before the first delivery.
- IN (Gap B): give delivered events enough identity that a multi-subscription
  plugin can discriminate which (namespace, event) arrived, WITHOUT breaking
  existing raw-payload consumers.
- IN (Gap C): make `"*"` in a startup subscription expand to the concrete
  registered event types of the target namespace, reusing the precedent at
  `internal/component/plugin/server/event_monitor.go:267-296` (expand at
  registration time into one concrete `Subscription` per (namespace, eventType));
  do NOT add a wildcard branch to `Subscription.Matches` in the hot path.

  -> Decision (user, 2026-07-16), answering Gap C: **implement the expansion.**
  `"*"` means what it says and what both callers plainly intended
  (`internal/plugins/exabgp/main_sdk.go:71`, `bridgeplugin/internal.go:93`, each
  commented "Subscribe to all events" while receiving nothing). Rejecting it
  loudly was the alternative and was NOT chosen: it would make the failure visible
  but leave the two shipped entry points to be rewritten with explicit event lists,
  and the wildcard is the more useful surface.

  -> Constraint: expansion alone fixes this INSTANCE, not the CLASS. The root cause
  is recorded at Current Behavior: `parseEventString` (`dispatch.go:125-131`) is an
  acknowledged duplicate of `ParseSubscription`'s logic (`dispatch.go:124` says so)
  and the two have DRIFTED -- `ParseSubscription` validates the namespace
  (`subscribe.go:217`) and the event type (`subscribe.go:235`) and errors on an
  unknown one, while `parseEventString` validates nothing and silently yields
  `EventTypeUnknown`. Any future unregistered event string is silently dead the same
  way. Converging the two, or making an unknown type an error rather than 0, is the
  durable fix and is NOT in this spec's approved scope; raise it before assuming.
- OUT: the runtime `request subscribe` command (already works); the IPsec
  clear/re-establish bug (`spec-fixit-ipsec-clear-reestablish.md`); rearchitecting
  the event bus; changing the BGP reactor's own formatted delivery path
  (`internal/component/bgp/server/events.go`), which does not route through
  `payloadToJSON` at all.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` (:313 deliver-event, :371 subscribe-events, :400-412 batched delivery) - plugin RPC + event delivery contract
  -> Constraint: the delivered event is a JSON **string**, not an object: `deliver-event` carries `{"event":"<json>"}` and `deliver-batch` carries `{"events":["<json-1>","<json-2>"]}` (a JSON array of strings). Any Gap B identity that lives in a NEW sibling field of `DeliverEventInput` is invisible on the batch path, and `delivery.go` batches by default. Therefore the identity must ride INSIDE the event string, or the batch element type must change (breaking).
  -> Constraint: `pkg/plugin/` and `pkg/ze/` are the EXTERNAL plugin SDK -- a public API surface compiled against by out-of-tree plugins. Every change here is additive-only: new struct fields with `omitempty`, new `On*`/`Set*` methods, zero changes to existing signatures or existing JSON keys. An external plugin built against today's `pkg/plugin` MUST keep compiling and keep receiving byte-identical deliveries after this work.
- [ ] `ai/rules/plugin-design.md` ("SDK Is Generic", "Cross-Boundary Value Types") - plugin SDK / protocol change rules
  -> Constraint: an envelope change must not break the raw-payload consumers (exabgp bridge, flowspec-firewall, the 16 in-tree `SetStartupSubscriptions` callers).
  -> Constraint: "adding a callback = one `On*` method, zero changes to `sdk_dispatch.go` or the event loop". A Gap B opt-in that needs a new SDK handler must obey this.
- [ ] `internal/component/plugin/resolve.go` (:89-99 RegisterDefaultEventNamespace, :103-107 DefaultEventNamespace, :116-135 RegisterPluginEventTypes) - the single default namespace mechanism
  -> Decision: Gap A threads a per-subscription namespace through the startup RPC (additive field). It does NOT add per-component default namespaces: `RegisterDefaultEventNamespace` panics on a conflicting second registration (`resolve.go:95-97`), so "one default per component" is not expressible without redesigning that function, and a process-global default cannot disambiguate two components anyway.
  -> Constraint: `RegisterPluginEventTypes` (`resolve.go:121`) hardcodes the SAME default for plugin-DECLARED event types (`registry.Registration.EventTypes`). A plugin that declares a non-bgp event type today has it registered into `bgp`. This is the same root cause and the same one-line producer; see the reject-fence-observability collision note in Design Insights.

### RFC Summaries (MUST for protocol work)
- N/A - internal plugin protocol, no RFC.

**Key insights:**
- There is exactly ONE namespace-resolution producer for every plugin-RPC subscription: `dispatch.go:148`. Startup (`startup.go:611`) and the runtime `subscribe-events` RPC share it. The runtime TEXT command (`request subscribe <ns> event <type>`) does NOT: it has its own parser (`ParseSubscription`, `subscribe.go:183-256`) that accepts and validates an explicit namespace. The text path is the working escape hatch; the RPC path is the gap.
- `parseEventString` (`dispatch.go:125-131`) is an acknowledged duplicate of `ParseSubscription`'s logic ("This mirrors the text protocol's ParseSubscription logic", `dispatch.go:124`). The two have already drifted: `ParseSubscription` validates namespace (`subscribe.go:217`) and event type (`subscribe.go:235`) and errors on unknown; `parseEventString` validates nothing and silently yields `EventTypeUnknown`. Gap C is a direct consequence of that drift.
- BGP events never reach `payloadToJSON`. The reactor formats them itself via `formatMessageForSubscription` (`internal/component/bgp/server/events.go:393-408`) and delivers through `proc.Deliver` directly (`events.go:218`). `deliverEvent`/`payloadToJSON` (`dispatch.go:199-305`) serves only the `emit-event` RPC and `EmitEngineEvent`. This is why bgp events look self-describing and `vpn-ipsec/sa-up` does not: two different delivery formatters, only one of which was ever given an identity contract.
- An INTERNAL plugin can already observe any namespace: it calls `eb.Subscribe("interface", "addr-added", ...)` on the in-process event bus (`internal/plugins/flowspec-firewall/engine.go:207-208`). An EXTERNAL plugin has no such escape hatch, which is exactly why the RPC namespace gap only hurts external plugins.

## Current Behavior (MANDATORY)

**Source files read (2026-07-10 spec author; re-verified line-by-line 2026-07-16
design session -- the 2026-07-10 line numbers for `pkg/plugin/rpc/types.go` and
`payloadToJSON` had drifted and are corrected below):**

- [ ] `internal/component/plugin/server/dispatch.go` (:135-166 `registerSubscriptions`) - THE namespace choke point. `input.Format`/`input.Encoding` are applied to the PROCESS, not the subscription (`proc.SetFormat` :139, `proc.SetEncoding` :142). The namespace is not read from the input at all: `namespace := plugin.DefaultEventNamespace()` (:148), warn when empty (:150-152), `nsID := events.LookupNamespaceID(namespace)` (:153) -- resolved ONCE, then reused for every entry in `input.Events` (:154-165). No per-event namespace exists.
  -> Constraint: Format/Encoding are per-PROCESS state, not per-subscription. Any Gap B opt-in expressed on `SubscribeEventsInput` inherits that scope: the last subscribe RPC wins for the whole process. This is existing behavior, not a regression, but it forecloses "envelope this subscription only".
- [ ] `internal/component/plugin/server/dispatch.go` (:125-131 `parseEventString`) - splits `"update direction sent"` into (eventType, direction); anything else falls through to `events.LookupEventTypeID(event)` with `DirBoth` (:130). No validation, no error return, no wildcard handling. An unknown string yields `EventTypeUnknown` and a silently dead subscription.
- [ ] `internal/component/plugin/resolve.go` (:79-107) - `defaultEventNamespace` is ONE process-global string (:80-82). `RegisterDefaultEventNamespace` (:89-99) panics on empty (:90-92) and on a CONFLICTING second registration (:95-97), so it structurally cannot hold two namespaces. `DefaultEventNamespace()` (:103-107) returns "" when unregistered.
- [ ] `internal/component/bgp/plugin/register.go:64` - `zeplugin.RegisterDefaultEventNamespace(bgpevents.Namespace)`. Verified by grep: this is the ONLY caller in the tree. The default is therefore always exactly `"bgp"` in any build that loads the BGP component, and `""` in one that does not.
- [ ] `internal/component/plugin/resolve.go` (:116-135 `RegisterPluginEventTypes`) - the SECOND consumer of the same global default (:121). Plugin-declared `Registration.EventTypes` are registered into `bgp` and nowhere else; the in-code comment concedes it ("If a future plugin needs another namespace, EventTypes would need namespace info", :119-120). Only one plugin declares EventTypes today (`internal/component/bgp/plugins/rpki_decorator/register.go:19`, `"update-rpki"`), and it IS a bgp event, so the latent bug has never fired.
- [ ] `internal/component/plugin/server/startup.go` (:605-614 `engineStartupSink.onReady`) - `input.Subscribe != nil && s.subscriptions != nil` -> `s.registerSubscriptions(proc, input.Subscribe)` (:610-611). Confirmed: the startup path adds NO namespace handling of its own; it inherits the `dispatch.go:148` default verbatim. Registration happens before the Ready->Running barrier (:608-609), which is the property to preserve.
- [ ] `pkg/plugin/rpc/types.go` (:488-494 `SubscribeEventsInput`) - fields `Events`/`Peers`/`Format`/`Encoding`, all `omitempty`. NO namespace field. **(corrected from :480-486)**
- [ ] `pkg/plugin/rpc/types.go` (:496-503 `ReadyInput`) - `Subscribe *SubscribeEventsInput` is a SINGLE pointer, not a slice, plus `Transport string`. A plugin can therefore express exactly one startup subscription block -> one Format, one Encoding, and (post-fix) one namespace. Multi-namespace startup subscription needs either a per-event namespace or an additive slice field.
- [ ] `pkg/plugin/rpc/types.go` (:246-249 `DeliverEventInput`) - `Event string` and nothing else. The delivered event is a bare JSON string.
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` (:594-602 `SetStartupSubscriptions`) - signature `(events, peers []string, format string)`; builds `&rpc.SubscribeEventsInput{Events, Peers, Format}` (:597-601). No namespace parameter, no namespace field to set. `SetEncoding` (:608-615) mutates the same struct.
- [ ] `pkg/plugin/sdk/sdk.go` (:356-365) - Stage 5 copies `p.startupSubscription` verbatim into `readyInput.Subscribe` (:359-361). The SDK performs NO expansion, NO validation, and NO namespace defaulting on the way out.
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` (:141-169 `OnEvent`) - the consumer contract. `EventHandler` is `func(event string) error`. The `deliver-event` callback unmarshals `{"event": string}` (:146-154); the `deliver-batch` callback unmarshals each element with `json.Unmarshal(raw, &eventStr)` (:160-168) -- i.e. every batch element MUST be a JSON string. Making an element an object breaks every existing external plugin at run time.
- [ ] `internal/component/plugin/server/subscribe.go` (:183-256 `ParseSubscription`) - the RUNTIME text path. Accepts `[peer <sel>|plugin <name>] [<ns>] event <type> [direction ...]`, defaults ns to `bgp` only when a peer filter is present and the next token is `event` (:214-215), otherwise validates against `events.IsValidNamespace` (:217) and validates the event type against the namespace (:235-237). This is the working precedent and the one that already does what Gap A needs.
- [ ] `internal/component/plugin/server/subscribe.go` (:66-82 `Subscription.Matches`) - exact `NamespaceID` equality (:67-69) then exact `EventTypeID` equality (:70-72). No wildcard branch exists on either field. This is the producer that makes Gap C fatal.
- [ ] `internal/core/events/ids.go` (:76-80 `LookupNamespaceID`, :84-88 `LookupEventTypeID`) - plain map reads returning the zero value (`NamespaceUnknown`/`EventTypeUnknown`, both `0`) for anything unregistered. No error, no ok-flag: an unknown namespace or event string is INDISTINGUISHABLE from a valid one at this layer.
- [ ] `internal/core/events/events.go` (:167-202 `RegisterNamespace`, :209-234 `RegisterEventType`) - IDs are assigned sequentially from 1 (:56, :60), so no real registered event can ever have ID 0.
- [ ] `internal/component/plugin/server/dispatch.go` (:199-275 `deliverEvent`, :286-305 `payloadToJSON`) - **(corrected from :245-283/:286-304)**. `deliverEvent` validates the emit against the registry (`events.IsValidEvent`, :205), resolves IDs (:215-217), gets matching procs (:249), then marshals ONCE for all of them (:257) and delivers the same string to every proc (:269). `payloadToJSON` (:286-305): nil -> `"null"` (:293), `string` passthrough (:294-296), `json.RawMessage` passthrough (:297-299), else `json.Marshal` (:300-303). No namespace, no event name, no envelope is ever added.
  -> Constraint: `deliverEvent` marshals one string for ALL subscribers (:257). A per-process envelope choice means the payload must be rendered per distinct encoding, not once. The precedent for that is the BGP path's `formatCache` (`internal/component/bgp/server/events.go:196-203`), which memoizes one rendering per distinct format+encoding key.
- [ ] `internal/component/bgp/server/events.go` (:168-256 `onMessageReceived`, :393-408 `formatMessageForSubscription`) - the BGP delivery path. `GetMatching(bgpNS(), ...)` (:184) with the namespace HARDCODED to bgp, formats via the format package per format+encoding key (:201), and calls `proc.Deliver` itself (:218). It never touches `deliverEvent` or `payloadToJSON`. All 6 other bgp emit sites (:279, :530, :608, :647, :728, :806) do the same.
- [ ] `internal/component/plugin/process/delivery.go` (:42-47 `EventDelivery`, :66-83 `Deliver`, :104-118 `deliveryLoop`) - `Output string` is the pre-formatted payload; `deliveryLoop` drains into a batch and sends via `deliver-batch`. Confirms the batch-of-strings constraint end to end.
- [ ] `internal/component/plugin/server/event_monitor.go` (:267-296) - the wildcard PRECEDENT. The CLI event monitor expands "all types" into one concrete `Subscription` per (namespace, eventType) at registration time (:280-295), using `allEventTypes()` (:267). Gap C should reuse this shape, not invent a wildcard match branch in the hot path.
- [ ] `internal/component/cmd/subscribe/subscribe.go` (:29-62 `handleSubscribe`) - the only other production `SubscriptionManager.Add` caller (:53); it goes through `ParseSubscription` (:30) and so already honors an explicit namespace.
- [ ] `internal/component/ike/engine/events.go` (:6 `Namespace = "vpn-ipsec"`, :28-32) - the motivating namespace. `SAUp = events.Register[*SAEvent](Namespace, "sa-up")`; `SAEvent` (:9-15) carries `peer-name`/`initiator-spi`/`responder-spi`/`remote-address`/`auth-method` and NO type discriminator. Confirms the Gap B claim concretely: a subscriber receiving this JSON cannot tell `sa-up` from `sa-down` -- both are `*SAEvent` (:28-29) and the payloads are structurally identical.
- [ ] `internal/test/runner/engine_steps.go` (:384-409 `EngineStepExpectEvent`) - the workaround this spec exists to remove, with the gap named in its own comment (:385-387): "Delivered events carry the BARE payload JSON -- no namespace or name envelope exists on the wire". Subscribes (:393-394), waits for ANY delivery in the window (:401), unsubscribes (:402-403).
- [ ] `internal/component/plugin/server/deliver_subscriber_test.go` (spec-test-coverage-gaps) - proves `EmitEngineEvent` delivers to a `SubscriptionManager` subscriber with `delivered==1` in namespace `test-deliver-ns` (:41-48), so the delivery leg itself works for a non-bgp namespace when the subscription is registered directly. The gap is purely in how a PLUGIN asks for that subscription.

**Consumer survey (validates A-3; all 16 in-tree `SetStartupSubscriptions` callers, grep 2026-07-16):**

| Caller | Events | Format | Namespace intent |
|--------|--------|--------|------------------|
| `internal/component/bgp/plugins/rib/rib.go:631` | update sent/received, state, refresh | `full` | bgp |
| `internal/component/bgp/plugins/adj_rib_in/rib.go:178` | update received, state | `full` | bgp |
| `internal/component/bgp/plugins/rpki/rpki.go:229` | update received | `full` | bgp |
| `internal/component/bgp/plugins/rpki_decorator/decorator.go:81` | (list) | - | bgp |
| `internal/component/bgp/plugins/gr/gr.go:197` | (list) | - | bgp |
| `internal/component/bgp/plugins/rr/rr.go:160` | (list) | - | bgp |
| `internal/component/bgp/plugins/rs/server.go:306` | (list) | - | bgp |
| `internal/component/bgp/plugins/bmp/bmp.go:217` | (list) | - | bgp |
| `internal/component/bgp/plugins/persist/server.go:167` | (list) | - | bgp |
| `internal/component/bgp/plugins/watchdog/watchdog.go:103` | state | `""` | bgp |
| `internal/component/bgp/plugins/redistribute_egress/register.go:83` | state | `""` | bgp |
| `internal/plugins/flowspec-firewall/engine.go:195` | update received, state | `parsed` | bgp (+ reaches `interface` namespace via the IN-PROCESS event bus, `engine.go:207-208`, not via subscription) |
| `internal/plugins/exabgp/main_sdk.go:71` | `*` | `""` | bgp -- **DEAD (Gap C)** |
| `internal/plugins/exabgp/bridgeplugin/internal.go:93` | `*` | `""` | bgp -- **DEAD (Gap C)** |
| `internal/test/cli/cmd_text_plugin.go:41` | update | `""` | bgp |

-> Decision: A-3 is CONFIRMED with a correction. Every in-tree caller wants the `bgp` namespace, and every one of them passes a bare-payload-consuming `OnEvent(func(string) error)`. An additive namespace field with an empty default is invisible to all 16. But two of them (`*`) are already broken today, so "preserve existing behavior" for those two means preserving a no-op.

**Behavior to preserve:**
- The runtime `request subscribe <ns> event <name>` command and its parser.
- Existing raw-payload consumers: the exabgp bridge and any plugin that parses `SetStartupSubscriptions([]string{"*"}, ...)` deliveries as bare payloads (`internal/plugins/exabgp/`, `internal/plugins/flowspec-firewall/engine.go`).
- BGP-namespace startup subscriptions (the common case) must keep working unchanged.
- Lazy-marshal-once delivery performance (`payloadToJSON` marshals only when an external subscriber exists).
- The `pkg/plugin` / `pkg/ze` public API: existing exported signatures and existing JSON keys are frozen. Out-of-tree plugins compiled against today's SDK must keep compiling and keep receiving byte-identical deliveries.
- `deliver-batch` elements stay JSON strings (`sdk_callbacks.go:160-168`).
- Startup subscriptions stay registered before the Ready->Running barrier (`startup.go:608-609`).

**Behavior to change:**
- Gap A: a startup subscription can name a non-bgp namespace.
- Gap B: delivered events are discriminable by (namespace, event) without breaking raw-payload consumers. The exact mechanism (envelope opt-in, sidecar field, or per-subscription encoding) is a design decision; see Key Design Decisions.
- Gap C (~~proposed~~ **approved 2026-07-16**): `"*"` expands to the namespace's registered event types (rejecting loudly was the alternative and was NOT chosen). Silent no-op is not an option once the code is touched.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Startup: `ze-plugin-engine:ready` with `ReadyInput.Subscribe` (SDK `SetStartupSubscriptions`)
- Runtime: `request subscribe <ns> event <name>` dispatch-command
- Delivery: engine `EmitEngineEvent(namespace, eventType, payload)` -> `deliverEvent` -> per-process `Deliver`

### Transformation Path
1. `SetStartupSubscriptions` (`sdk_callbacks.go:594`) -> `p.startupSubscription` (`sdk.go:359-361`) -> `ready` RPC -> `onReady` (`startup.go:605`) -> `registerSubscriptions` (`dispatch.go:135`) -> namespace resolved from the process-global default (`dispatch.go:148`) -- **the Gap A choke point, one line** -> `parseEventString` (`dispatch.go:155`) -- **the Gap C choke point** -> `SubscriptionManager.Add` (`dispatch.go:164`)
2. Emit -> `deliverEvent` (`dispatch.go:199`) -> `GetMatching(nsID, etID, ...)` (`dispatch.go:249`) -> `payloadToJSON` once for all procs (`dispatch.go:257`) -- **the Gap B choke point** -> `proc.Deliver` (`dispatch.go:269`)
3. `deliveryLoop` (`process/delivery.go:108`) -> `deliver-batch` (array of JSON strings) -> SDK batch callback (`sdk_callbacks.go:155-169`) -> `OnEvent(string)`
4. (Parallel, untouched) BGP: `onMessageReceived` (`bgp/server/events.go:168`) -> `GetMatching(bgpNS(), ...)` (:184) -> `formatMessageForSubscription` (:201/:393) -> `proc.Deliver` (:218). Never enters step 2.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| plugin -> engine (startup) | ready RPC Subscribe field (`sdk.go:359-361` -> `startup.go:610-611`) | [ ] verified 2026-07-16: producer and consumer both read, no namespace on either side |
| plugin -> engine (runtime RPC) | `subscribe-events` -> same `registerSubscriptions` (`dispatch.go:135`) | [ ] verified 2026-07-16: shares the Gap A choke point with startup |
| plugin -> engine (runtime text) | `request subscribe <ns> event <t>` -> `ParseSubscription` (`subscribe.go:183`) -> `Add` (`cmd/subscribe/subscribe.go:53`) | [ ] verified 2026-07-16: DOES carry a namespace; the working path |
| engine -> plugin (delivery) | `deliver-event` `{"event":string}` / `deliver-batch` `{"events":[string]}` -> `OnEvent(func(string) error)` | [ ] verified 2026-07-16: element type is a JSON string on both paths |

### Integration Points
- `internal/component/plugin/server/` subscription manager, dispatch, startup sink
- `pkg/plugin/rpc/types.go` SubscribeEventsInput (Gap A field)
- `pkg/plugin/sdk/` startup subscription API (Gap A surface)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - namespace resolution is registry-driven, not a hardcoded default (this fix removes a hardcoded default)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Startup subscriptions are namespace-locked to bgp | dispatch.go:148 + resolve.go single default + only bgp registers it | Gap A is narrower | grep RegisterDefaultEventNamespace callers | **confirmed** 2026-07-16 (grep: sole caller is `internal/component/bgp/plugin/register.go:64`; `resolve.go:95-97` panics on a conflicting second registration, so a second default is not even expressible) |
| A-2 | Delivered events carry no namespace/name envelope | payloadToJSON dispatch.go:286-305 | Gap B does not exist | read a delivered payload for a typed event | **confirmed** 2026-07-16 (`payloadToJSON` :294-303 is passthrough/marshal only; `DeliverEventInput` types.go:246-249 is `{Event string}`; `ike/engine/events.go:9-15` `SAEvent` has no type field and is shared by `sa-up` AND `sa-down` :28-29, so the two are indistinguishable on the wire) |
| A-3 | Existing consumers rely on the bare-payload shape | exabgp bridge, flowspec-firewall use `*`/parsed formats | an envelope breaks them | grep SetStartupSubscriptions consumers, read their OnEvent | **confirmed with a correction** 2026-07-16 (Consumer survey table above: 16 callers, all bgp, all `OnEvent(func(string) error)` bare-payload. Correction: the two `*` callers consume nothing today because their subscription is dead -- see A-5) |
| A-4 | The runtime subscribe path is the correct namespace precedent to mirror | ParseSubscription subscribe.go:209-222 | design a new namespace mechanism | reuse LookupNamespaceID validation | **confirmed** 2026-07-16 (`subscribe.go:213-222` accepts ns, `:217` validates via `IsValidNamespace`, `:235` validates the event type against it) |
| A-5 | `"*"` in a startup subscription is a silent no-op (Gap C) | `parseEventString` dispatch.go:130 -> `LookupEventTypeID("*")`; `Matches` subscribe.go:70-72 exact-ID equality | Gap C does not exist; exabgp is fine | grep every `RegisterNamespace`/`RegisterEventType` caller for a `"*"` event type; confirm no wildcard branch in `Matches` | **confirmed** 2026-07-16 (no registration of `"*"` anywhere: `RegisterNamespace` callers grepped -- bgp registers a fixed list, `register.go:52-59`; `RegisterEventType` callers are only `resolve.go:129` and `config_tx_bridge.go:107-125`. `LookupEventTypeID` ids.go:84-88 returns 0 for it; IDs start at 1, events.go:60; `Matches` has no wildcard branch) |
| A-6 | No test covers event DELIVERY through a `*` startup subscription | grep test/ for exabgp event assertions | Gap C would already be red in CI, contradicting A-5 | read the exabgp test fixtures | **confirmed** 2026-07-16 (`test/exabgp/process/` is a config-MIGRATION fixture: input.conf/expected.conf only. `test/plugin/exabgp-bridge-sdk.ci` exercises plugin->engine only: the script announces a route and the test asserts the resulting BGP UPDATE hex on the wire; it never asserts an event reaching the script. Nothing contradicts A-5.) |
| A-7 | An envelope cannot ride as a sibling JSON field of the delivered event | `deliver-batch` element type | a cheaper Gap B fix exists (sidecar field on DeliverEventInput) | read the SDK batch callback | **confirmed** 2026-07-16 (`sdk_callbacks.go:160-168` unmarshals every batch element into a `string`; `process-protocol.md:411` shows `{"events":["<json-event-1>",...]}`. A sidecar on `DeliverEventInput` would only reach the single-event path, which is not the default path. Identity must ride INSIDE the string.) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An envelope change breaks the exabgp bridge / existing subscribers | exabgp or flowspec `.ci` red | make the envelope opt-in per process (`Envelope bool`, default false); keep bare-payload the default. Survives design: the opt-in is never set by any of the 16 existing callers. |
| R-2 | Per-subscription namespace re-marshals on the hot path, hurting BGP throughput | perf regression in stress runs | **Downgraded during design.** BGP events never enter `deliverEvent`/`payloadToJSON` (they use `bgp/server/events.go:184-218` + `formatMessageForSubscription`). The Gap B change cannot touch the BGP hot path as long as the envelope is confined to `deliverEvent`. Residual risk only if the envelope is made uniform across both formatters (see D-4). |
| R-3 | Namespace threading touches the SDK ABI and breaks external plugins built against pkg/plugin | external plugin build breaks | additive field + NEW method; `SetStartupSubscriptions` keeps its exact signature and its exact wire output (empty namespace = default = bgp). No existing symbol changes. |
| R-4 | Fixing Gap C turns two shipped no-op subscriptions into live firehoses | exabgp bridge suddenly floods its script; `test/plugin/exabgp-bridge-sdk.ci` timing changes | This is the POINT of the fix, but it is a behavior change on a shipped path, not a no-op cleanup. Gate on Thomas's scope decision; if approved, the `.ci` must assert an event actually reaches the script (A-6 says nothing does today). -> Scope decision MADE (approved 2026-07-16): the `.ci` event-arrival assertion is REQUIRED, not conditional. |
| R-5 | Unifying `parseEventString` onto `ParseSubscription` makes previously-silent subscriptions hard errors | any plugin passing an unregistered event string now fails startup | Deliberate (fail-on-unknown is the ze rule), but it converts Gap C from "dead" to "fatal" for exabgp unless `*` expansion lands in the same change. Do not unify the parsers without deciding Gap C first. |
| R-6 | The reject-fence-observability spec lands a plugin-subscribable event type in `Registration.EventTypes` and it silently registers into the `bgp` namespace (`resolve.go:121`) | a `reload-processed` event shows up under `bgp/` in `show` output or monitor listings | Same root cause, different producer line. Coordinate: either that spec's event stays in an already-bgp-shaped namespace, or this spec's namespace threading must cover `RegisterPluginEventTypes` too. **Needs a joint decision; not resolvable unilaterally.** |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin declares a startup subscription in a non-bgp namespace | -> | `registerSubscriptions` (`dispatch.go:135`) registers it in that namespace | `TestStartupSubscriptionHonorsNamespace` (unit) |
| Engine emits a non-bgp event; subscriber discriminates it | -> | opt-in envelope in `payloadToJSON`/`deliverEvent` | `TestDeliveredEventIsDiscriminable` (unit) |
| A plugin subscribes to `vpn-ipsec/sa-up` at startup and receives it | -> | end-to-end startup subscribe + deliver | `test/plugin/plugin-startup-subscribe-namespace.ci` |
| The new SDK method is reachable from an out-of-tree-shaped plugin | -> | `SetStartupSubscriptionsIn` -> `ReadyInput.Subscribe.Namespace` -> engine | `.ci` above uses the SDK method, not a hand-built `Subscription` (a unit test that calls `SubscriptionManager.Add` directly proves nothing about wiring) |
| Gap C (~~if approved~~ **approved 2026-07-16**): `*` reaches the exabgp script | -> | wildcard expansion at registration | extend `test/plugin/exabgp-bridge-sdk.ci` or a sibling: assert an event ARRIVES at the script (nothing asserts this today, A-6) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin sets a startup subscription naming namespace `vpn-ipsec`, event `sa-up` | The engine registers it in `vpn-ipsec`; a `vpn-ipsec/sa-up` emit is delivered to that plugin |
| AC-2 | Plugin subscribed to two event types with `Envelope` opted in receives one of them | The delivered string carries `namespace` + `event` + `payload`; the plugin routes on those without parsing peer-specific payload fields. Specifically `vpn-ipsec/sa-up` vs `vpn-ipsec/sa-down` are distinguishable, which is impossible today (both are `*SAEvent`, `ike/engine/events.go:28-29`) |
| AC-3 | Existing bgp-namespace startup subscription (no explicit namespace, no envelope) | Unchanged: still lands in `bgp`, still delivers, and the delivered bytes are IDENTICAL to pre-change |
| AC-4 | exabgp bridge / flowspec-firewall existing subscriptions | Unchanged delivery; their `.ci`/interop tests stay green (R-1) |
| AC-5 | BGP stress path | No throughput regression from the change (R-2) |
| AC-6 | A startup subscription names an UNREGISTERED namespace | The engine logs a warn naming the plugin and the namespace and skips the subscription. It does NOT resolve to `NamespaceUnknown` and register a silently-dead subscription (the Gap C failure mode, A-5) |
| AC-7 | An out-of-tree plugin built against the PREVIOUS `pkg/plugin` | Still compiles against the new SDK (no changed signature) and still receives byte-identical deliveries (no changed default). Demonstrated by: no diff to any existing exported signature, and AC-3's byte-identity assertion |
| AC-8 (Gap C, ~~IF APPROVED~~ **APPROVED 2026-07-16**) | `SetStartupSubscriptions([]string{"*"}, ...)` in namespace `bgp` | Expands to one subscription per registered `bgp` event type; an emitted `update` reaches the plugin. Today it reaches nothing (A-5, A-6) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An external plugin observes IPsec sa-up at startup | SetStartupSubscriptions(ns=vpn-ipsec) -> ready RPC -> register -> emit -> deliver -> OnEvent | `test/plugin/plugin-startup-subscribe-namespace.ci` |
| 2 | A multi-event plugin routes by event identity | deliver-event with discriminable identity -> plugin handler switch | `TestDeliveredEventIsDiscriminable` + `.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStartupSubscriptionHonorsNamespace` | `internal/component/plugin/server/*_test.go` | AC-1 Gap A. Model on `deliver_subscriber_test.go` (already proves a non-bgp namespace delivers when the subscription is registered by hand); the new test must go through `registerSubscriptions` instead | |
| `TestDeliveredEventIsDiscriminable` | `internal/component/plugin/server/*_test.go` | AC-2 Gap B. Two events sharing one payload type (`sa-up`/`sa-down` shape) so the test FAILS on any payload-sniffing implementation | |
| `TestBgpStartupSubscriptionUnchanged` | `internal/component/plugin/server/*_test.go` | AC-3 regression guard. Assert delivered BYTES, not just delivered count -- a count-only assertion would pass an accidental always-on envelope (mistake-log pattern: count-only assertions) | |
| `TestStartupSubscriptionUnknownNamespaceWarnsAndSkips` | `internal/component/plugin/server/*_test.go` | AC-6 | |
| `TestSubscribeEventsInputNamespaceOmittedFromJSON` | `pkg/plugin/rpc/*_test.go` | AC-7 wire compat: marshaling a namespace-less `SubscribeEventsInput` produces byte-identical JSON to pre-change (`omitempty` actually omits) | |
| `TestStartupWildcardExpandsToNamespaceEvents` (Gap C, ~~if approved~~ **approved 2026-07-16**) | `internal/component/plugin/server/*_test.go` | AC-8 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric input) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-startup-subscribe-namespace.ci` | `test/plugin/` | plugin subscribes to a non-bgp namespace at startup and receives the event | |
| `plugin-event-discriminable.ci` | `test/plugin/` | multi-subscription plugin routes by event identity | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - internal plugin protocol, no external peer daemon; existing exabgp/bgp `.ci` cover the regression surface | - | - | - | |

## Files to Modify
- `internal/component/plugin/server/dispatch.go` - `registerSubscriptions:148` namespace resolution (Gap A), `parseEventString:125-131` (Gap C, ~~if approved~~ **approved 2026-07-16**), `deliverEvent:257` + `payloadToJSON:286` delivery identity (Gap B). **NOTE: as of 2026-07-16 a concurrent session is actively editing `internal/component/plugin/server/` for `spec-fixit-reject-fence-observability` (reload generation counter, possibly a new plugin-subscribable event type). Re-read this directory before implementing; the line numbers above WILL drift.**
- `internal/component/plugin/server/startup.go` - `onReady:610-611`. Likely NO change needed: it forwards `input.Subscribe` wholesale, so the namespace rides along for free once the field exists. Verify at implementation; if unchanged, remove from this list rather than touching it.
- `internal/component/plugin/server/subscribe.go` - only if D-6 (parser unification) is approved.
- `internal/component/plugin/process/process.go` - per-process `Envelope` flag (Gap B), alongside the existing `SetFormat`/`SetEncoding` state.
- `pkg/plugin/rpc/types.go` - `SubscribeEventsInput` (:488-494): additive `Namespace` (Gap A) + `Envelope` (Gap B) fields. **PUBLIC SDK -- additive only.**
- `pkg/plugin/sdk/sdk_callbacks.go` - new `SetStartupSubscriptionsIn` (:594 area, D-2) + envelope opt-in method. **PUBLIC SDK -- no existing signature changes.**
- `internal/component/plugin/resolve.go` - only if the default-namespace mechanism changes (D-1 says it does not) OR if the R-6 coordination decision extends namespace threading to `RegisterPluginEventTypes:121`.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | N/A | no config surface |
| CLI commands/flags | N/A | runtime subscribe unchanged |
| Functional test for new RPC/API | Yes | `test/plugin/plugin-startup-subscribe-namespace.ci` |
| Plugin SDK/protocol changed | Yes | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| Doctor check | N/A | no new runtime dependency |
| Prometheus counters | N/A | no new observable product state |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` (subscribe-events namespace) |
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 12 | Internal architecture changed? | [ ] | `ai/digests/` plugin event-flow digest |
| 15 | Registered event type / send type changed? | [ ] | `docs/plugin-overview.md`, `docs/features/plugins.md` |

## Files to Create
- `test/plugin/plugin-startup-subscribe-namespace.ci`, `test/plugin/plugin-event-discriminable.ci`
- `internal/component/plugin/server/<name>_test.go` (unit tests above)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, validate A-3 (consumer survey) first |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 13. /ze-review gate | Review Gate section |
| 14. Present + close | Executive Summary; two-commit closure |

### Implementation Phases
0. **Phase: re-read `internal/component/plugin/server/`** - `spec-fixit-reject-fence-observability` was editing this package concurrently with this design (2026-07-16). Confirm `registerSubscriptions`, `deliverEvent`, and `payloadToJSON` still look as described in Current Behavior, and check whether that spec landed a plugin-subscribable event type (R-6). Do not start before this.
1. ~~**Phase: Consumer survey (validate A-3)**~~ -- **DONE during design 2026-07-16.** All 16 `SetStartupSubscriptions` callers enumerated in the Current Behavior Consumer survey table; A-3 confirmed with the `*` correction. Do not repeat it; go straight to phase 2.
2. **Phase: Gap A** - additive `Namespace` field (D-1) + additive SDK method (D-2), defaulting to today's behavior; unregistered namespace warns and skips (AC-6); unit + `.ci`. Assert byte-identity for the default path (AC-3/AC-7) in the SAME phase, not at the end.
3. **Phase: Gap B** - opt-in envelope (D-4) with per-choice memoized rendering (D-5), preserving bare-payload default and lazy-marshal; unit + `.ci`.
4. **Phase: Gap C** - ~~ONLY if Thomas approved it into scope.~~ **APPROVED into scope 2026-07-16; this phase is REQUIRED.** Wildcard expansion at registration time (D-7), then optionally the parser unification (D-6). Requires a `.ci` that proves an event reaches the exabgp script, since A-6 confirmed none exists.
5. **Phase: regression + perf** - bgp startup path byte-unchanged; exabgp/flowspec green; stress path unregressed (R-2 is structurally low: BGP never enters `deliverEvent`).
6. **Full verification + closure.**

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Backward compat | bgp startup subscriptions + raw-payload consumers unchanged (AC-3/AC-4) |
| Registration over hardcoding | namespace comes from the registry, not a single hardcoded default |
| Performance | lazy-marshal-once preserved; envelope only when requested (R-2) |
| Plugin ABI | SDK/RPC change is additive with a legacy default (R-3) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| non-bgp startup subscribe works | `go test -run TestStartupSubscriptionHonorsNamespace ./internal/component/plugin/server/` |
| delivered event discriminable | `go test -run TestDeliveredEventIsDiscriminable ./internal/component/plugin/server/` |
| exabgp unregressed | `bin/ze-test exabgp --all` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | namespace strings validated against registered namespaces (LookupNamespaceID) |
| Information leakage | envelope must not expose more than the payload already did |

### Failure Routing
| Failure | Route To |
|---------|----------|
| exabgp/flowspec test red | envelope not additive -> back to DESIGN (make it opt-in) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->

**2026-07-16 design session:**

- **Gaps A and B share one root cause, and it is one line.** `dispatch.go:148`
  (`namespace := plugin.DefaultEventNamespace()`) is the single producer of the
  namespace for every plugin-RPC subscription, startup and runtime alike. The
  skeleton framed A and B as "two related gaps"; A is really "the RPC has no
  namespace field so the resolver has nothing to resolve", and B is "the
  formatter on that same path was never given an identity contract". Both live
  in `deliverEvent`'s file, ~120 lines apart.

- **There are TWO delivery formatters, and only the neglected one is broken.**
  BGP events are formatted by `formatMessageForSubscription`
  (`bgp/server/events.go:393`) and delivered directly (`events.go:218`); they
  never touch `payloadToJSON`. Everything else (emit-event RPC, EmitEngineEvent
  -> IPsec, config transactions, iface, vpp, ...) goes through `payloadToJSON`
  (`dispatch.go:286`), which adds no identity. That asymmetry is why "events are
  self-describing" felt true (it is, for bgp) while `vpn-ipsec/sa-up` is not.
  Any claim of the form "delivered events carry X" must name WHICH formatter.

- **The IPsec case is the sharpest possible proof of Gap B.** `SAUp` and `SADown`
  are both `events.Register[*SAEvent]` (`ike/engine/events.go:28-29`) over a
  payload with no discriminator field (`:9-15`). Two different events, one
  identical JSON shape. No amount of payload sniffing can separate them. This is
  not a "nice to have envelope", it is an information-theoretic gap.

- **Gap C: `"*"` has never worked.** Two shipped entry points
  (`exabgp/main_sdk.go:71`, `exabgp/bridgeplugin/internal.go:93`) subscribe with
  `"*"`, which resolves to `EventTypeUnknown` (0) and can never match an
  exact-equality check against IDs that start at 1. No test catches it (A-6):
  `test/exabgp/process/` is a config-migration fixture and
  `test/plugin/exabgp-bridge-sdk.ci` only exercises plugin->engine (the script
  announces, the test asserts UPDATE hex on the wire). The comment at
  `main_sdk.go:70` says "Subscribe to all events using text encoding"; the code
  subscribes to nothing. **This was found by reading the producer
  (`LookupEventTypeID` -> `Matches`) rather than trusting the caller's comment
  -- exactly the `ai/rules/no-fabrication.md` mechanical check.**

- **`parseEventString` vs `ParseSubscription` is a live duplication with drift.**
  `dispatch.go:124` says it "mirrors" `ParseSubscription`. It does not: the text
  parser validates namespace and event type and returns errors
  (`subscribe.go:217`, `:235`); the RPC parser validates nothing and silently
  yields a dead ID. Gap C is the direct consequence. The end state is one
  parser, but unification cannot land before `*` has a defined meaning (R-5, D-6).

- **The batch path forecloses the obvious Gap B fix.** The instinctive design
  (add `namespace`/`event` next to `event` in `DeliverEventInput`) reaches only
  the single-event RPC. `deliver-batch` carries a JSON array of STRINGS
  (`process-protocol.md:411`, `sdk_callbacks.go:160-168`) and batching is the
  default path (`process/delivery.go:108-118`). Identity must ride inside the
  string, or every existing external plugin breaks at run time. A-7.

- **Internal plugins have an escape hatch external plugins do not.**
  `flowspec-firewall` subscribes to the `interface` namespace by calling
  `eb.Subscribe("interface", "addr-added", ...)` on the in-process event bus
  (`engine.go:207-208`) while using `SetStartupSubscriptions` only for bgp. That
  is why this gap has stayed invisible: everything that needed a non-bgp
  namespace so far happened to be in-process. External plugins have no bus.

- **R-6 / COLLISION with `spec-fixit-reject-fence-observability` (Status: ready,
  being implemented concurrently).** That spec considers adding "an event type in
  the plugin `Registration.EventTypes` for reload-processed / external-plugin-exited"
  (its Implementation Steps, item ~77). `Registration.EventTypes` is registered by
  `RegisterPluginEventTypes` (`resolve.go:116-135`), which reads the SAME global
  default at `:121` and whose own comment concedes the limitation at `:119-120`.
  A `reload-processed` event declared that way lands in the `bgp` namespace --
  wrong on its face for a plugin-host lifecycle event, and invisible to anyone
  subscribing to a sane namespace. The two specs therefore touch the same
  hardcoded-namespace bug from two producers (`dispatch.go:148` for
  subscriptions, `resolve.go:121` for declarations). **Not resolved here.**
  Options for Thomas: (a) that spec uses the counter, not the event type (its own
  Implementation Steps already prefer the counter: "prefer it unless the
  external-plugin-exit cases need an event") -- collision evaporates; (b) that
  spec ships the event under `bgp` and this spec migrates it later (debt);
  (c) this spec's namespace threading is extended to `RegisterPluginEventTypes`
  and that spec depends on it (serializes the two).
  **Update, same session, later:** the concurrent implementation has landed
  `internal/component/plugin/server/reload_generation.go` (untracked at the time of
  writing) and it declares NO event type, no namespace, no `Emit` -- i.e. it took
  option (a), the counter. **The collision has not materialized.** R-6 stays on the
  books as a latent bug of `resolve.go:121` (the next plugin to declare a non-bgp
  `EventTypes` entry hits it silently), not as a live conflict between the two specs.
  Re-check at Phase 0 before implementing.

  -> AUTONOMOUS DEFAULT (2026-07-17), resolving the "Options for Thomas" list
  above: **option (a).** `spec-fixit-reject-fence-observability` uses the
  reload-generation COUNTER, not a new plugin-subscribable event type, so this
  spec's namespace threading is NOT extended to `RegisterPluginEventTypes` and the
  two specs have no ordering dependency. Rationale: (a) is the smallest,
  self-contained choice AND has already materialized -- re-verified 2026-07-17,
  `internal/component/plugin/server/reload_generation.go` declares no
  `events.Register`, no namespace, and no `Emit`. The collision does not exist; R-6
  remains only a latent `resolve.go:121` bug for the next plugin that declares a
  non-bgp `EventTypes` entry (OUT of this spec's scope). Thomas: override if that
  sibling spec later ships the event type after all.

## Key Design Decisions

**SDK compatibility is the governing constraint.** `pkg/plugin/` and `pkg/ze/` are
the external plugin SDK: a public API surface that out-of-tree plugins compile
against and whose JSON is a wire contract with binaries this repo does not build.
Every decision below is shaped by "additive only": new `omitempty` fields, new
methods, new opt-in flags. No existing exported signature changes, no existing
JSON key changes meaning, no existing default delivery byte changes.

| ID | Decision | Alternatives Considered | Rationale |
|----|----------|------------------------|-----------|
| D-1 | Gap A: add `Namespace string \`json:"namespace,omitempty"\`` to `rpc.SubscribeEventsInput`. `registerSubscriptions` uses it when non-empty, else falls back to `plugin.DefaultEventNamespace()` -- the exact current behavior. Validate a non-empty namespace with `events.IsValidNamespace` and log+skip on unknown (do not silently resolve to `NamespaceUnknown`). | (a) Per-component default namespaces via `RegisterDefaultEventNamespace`: rejected, `resolve.go:95-97` panics on a conflicting registration and a process-global cannot disambiguate two components. (b) Qualify each event string as `<ns>/<event>`: rejected as the primary mechanism, it invents a third grammar next to `parseEventString` and `ParseSubscription`. | The field mirrors the working text path (`ParseSubscription`, A-4). Empty = today's behavior, so all 16 existing callers and every out-of-tree plugin are byte-identical on the wire (R-3). One namespace per subscribe block matches the existing per-process scope of `Format`/`Encoding`. |
| D-2 | Gap A SDK surface: ADD `SetStartupSubscriptionsIn(namespace string, events, peers []string, format string)` (name pending Thomas). Keep `SetStartupSubscriptions` untouched, delegating to it with `namespace == ""`. | Change `SetStartupSubscriptions` to take a namespace: rejected outright, it breaks every out-of-tree plugin at compile time. Variadic option args: rejected, not a pattern used elsewhere in this SDK. | "Adding a callback = one method, zero changes to the event loop" (`ai/rules/plugin-design.md` SDK Is Generic). Additive method = zero compat impact. -> AUTONOMOUS DEFAULT (2026-07-17), resolving "name pending Thomas": use `SetStartupSubscriptionsIn` (this cell's own proposal). Rationale: it mirrors the frozen `SetStartupSubscriptions` with an `In`(namespace) suffix, is purely additive, and changes no existing signature; the rejected alternatives (mutate the existing signature; variadic options) are worse on compat. Thomas: override the name before implementation if you prefer another. |
| D-3 | Multi-namespace startup: OUT of scope for the first pass. `ReadyInput.Subscribe` stays a single `*SubscribeEventsInput` (one namespace per plugin at startup). A plugin needing two namespaces at startup uses one block + the runtime text path, or we add `Subscriptions []SubscribeEventsInput` later (additive). | Make `ReadyInput.Subscribe` a slice now: rejected, changes an existing JSON key's type (breaking). Add the slice field now: deferred, no in-tree consumer needs it yet and `ai/rules/design-principles.md` abstracts at 2+ use cases. | AC-1 (`vpn-ipsec/sa-up`) needs exactly one namespace. Recorded in Known Limitations so the next session does not rediscover it. |
| D-4 | Gap B: the identity rides INSIDE the delivered string, as an opt-in envelope: `{"namespace":"vpn-ipsec","event":"sa-up","payload":<bare payload JSON>}`. Opt-in via a new `Envelope bool \`json:"envelope,omitempty"\`` on `SubscribeEventsInput` -> per-process flag (same scope as `Format`/`Encoding`, `dispatch.go:139-142`) -> a new SDK `OnEventEnveloped`-style handler or documented parsing on the existing `OnEvent`. Confined to `deliverEvent`; the BGP formatter path is untouched. | (a) Sidecar fields on `DeliverEventInput`: **rejected, broken by A-7** -- `deliver-batch` elements are JSON strings and batching is the default path, so a sidecar would only reach the single-event path. (b) A new `Encoding: "envelope"` value: rejected, `Encoding` is also consumed by the BGP formatter (`bgp/server/events.go:201`, `formatMessageForSubscription:412`), so an unknown value there would silently mean "not text" and produce inconsistent behavior across the two formatters. (c) Envelope always-on: rejected, breaks all 16 consumers (R-1). | A separate boolean keeps the two formatters' concerns separate. Wire shape is unchanged (still a string in `event` / in the `events` array), so old plugins and the batch path need no protocol change at all. |
| D-5 | Gap B perf: `deliverEvent` currently marshals once for ALL procs (`dispatch.go:257`). With a per-process opt-in, render at most twice (bare + enveloped), memoized per distinct choice, and build the enveloped form ONLY if some matching proc opted in. | Marshal per proc: rejected, O(procs) on an emit path. | Mirrors the existing `formatCache` precedent (`bgp/server/events.go:196-203`). Preserves lazy-marshal-once for the default case: zero opt-in subscribers = byte-identical work to today. |
| D-6 | Do NOT unify `parseEventString` onto `ParseSubscription` in this spec unless Gap C is approved in the same pass (R-5). | Unify now for one parser: attractive (`dispatch.go:124` admits the duplication) but it makes `"*"` a hard error and breaks exabgp startup. | Ordering constraint, not a rejection: unification is the right end state, it just cannot land before `*` has a meaning. |
| D-7 | Gap C (~~IF approved~~ **APPROVED 2026-07-16**): `"*"` expands at registration time into one `Subscription` per registered event type of the target namespace, reusing the shape of `event_monitor.go:267-296`. No wildcard branch is added to `Subscription.Matches`. | A wildcard `EventType` sentinel checked in `Matches`: rejected, puts a branch on the per-event matching path for a startup-time concern. | Registration-time expansion keeps the hot path exactly as it is (`ai/rules/buffer-first.md` spirit) and reuses a precedent already in this package. |

## Known Limitations
- Gap B is an opt-in envelope, not a per-subscription encoding (D-4). Rationale recorded there: `Encoding` is shared with the BGP formatter and a new value would mean different things on the two paths.
- **One namespace per plugin per startup** (D-3). `ReadyInput.Subscribe` is a single pointer (`types.go:501`); making it a slice would change an existing key's type. A plugin needing startup subscriptions in two namespaces must add a `Subscriptions []SubscribeEventsInput` field (additive) first. No in-tree consumer needs it today.
- **`Format`/`Encoding`/`Envelope` are per-PROCESS, not per-subscription** (`dispatch.go:139-142`). Pre-existing; this spec inherits it rather than fixing it. Consequence: a plugin cannot envelope one subscription and not another, and two subscribe RPCs with conflicting flags = last writer wins.
- **The envelope covers the `deliverEvent` path only.** BGP-namespace events keep their formatter-shaped output (`bgp/server/events.go:393`) and are unaffected by the envelope flag. This is deliberate (R-2: it keeps the BGP hot path untouched) and acceptable because BGP's formatted output already carries a type. It does mean a plugin subscribed across both bgp and non-bgp namespaces sees two shapes. Uniform enveloping across both formatters is a separate, larger change.
- ~~Gap C's scope (`*` expansion vs explicit rejection vs leave broken) is UNRESOLVED pending Thomas.~~ -> RESOLVED (user 2026-07-16): `*` expansion approved into scope (see Task > Gap C Decision and D-7). No longer an open limitation.

## RFC Documentation
- N/A (internal plugin protocol).

## Implementation Summary
### What Was Implemented
- **Gap A** — `rpc.SubscribeEventsInput.Namespace` (additive, omitempty). `registerSubscriptions` resolves it via new `resolveSubscriptionNamespace`: explicit namespace validated with `events.IsValidNamespace` (unknown -> warn + skip whole block, no dead sub); empty -> `plugin.DefaultEventNamespace()` (legacy). SDK `SetStartupSubscriptionsIn(namespace, events, peers, format)` added; `SetStartupSubscriptions` untouched.
- **Gap B** — `rpc.SubscribeEventsInput.Envelope` (additive, omitempty) -> per-process `process.Envelope()`/`SetEnvelope()` (`atomic.Bool`). `deliverEvent` renders the bare payload once (unchanged) and, only when a matching proc opted in, builds `rpc.EventEnvelope{namespace,event,payload}` at most once via `buildEventEnvelope`. SDK `SetEnvelope(true)` + `rpc.ParseEventEnvelope` for the consumer.
- **Gap C** — `"*"` in a startup subscription expands at registration time into one sub per event type of the namespace using the server-package `allEventTypes()` (filters the bgp `"sent"` pseudo type); no wildcard branch added to `Subscription.Matches`.
- **Files:** `internal/component/plugin/server/dispatch.go`, `internal/component/plugin/process/process.go`, `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_callbacks.go`. Tests: `internal/component/plugin/server/startup_subscription_test.go`, `pkg/plugin/rpc/event_envelope_test.go`, `pkg/plugin/sdk/startup_subscriptions_in_test.go`.
### Bugs Found/Fixed
- Review ISSUE: rejected (unknown-namespace) subscribe block mutated per-process delivery state before the reject — reordered so reject has zero side effects.
### Documentation Updates
- `docs/architecture/api/process-protocol.md` (Subscription Namespace + Enveloped Delivery + SubscribeEventsInput field table).
- `ai/rules/plugin-design.md` (external-plugin startup subscription namespace/envelope note).
### Deviations from Plan
- **Functional `.ci` fixtures NOT authored/run** (`plugin-startup-subscribe-namespace.ci`, `plugin-event-discriminable.ci`, exabgp Gap C extension). This was a parked background job forbidden from running functional/QEMU suites; every AC's MECHANISM is proven by scoped unit tests through the real production entry points, but the end-to-end SDK-fork path (spec Wiring rows 3–5, AC-8 "reaches exabgp script") remains to be authored + run in a live env before spec closure. See `tmp/drain-fixit-plugin-event-subscription.md`. Spec is NOT closed.
- Multi-namespace startup (D-3) intentionally out of first pass.

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
### Files from Plan
| File | Status | Notes |
|------|--------|-------|
### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| non-bgp startup subscription works | functional | `plugin-startup-subscribe-namespace.ci` |
| events discriminable | functional + unit | `plugin-event-discriminable.ci`, `TestDeliveredEventIsDiscriminable` |
| no regression | functional | exabgp + bgp plugin suites green |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial) — independent general-purpose subagent over the 7 code+test files
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Fail-closed skip ran AFTER SetFormat/SetEncoding/SetEnvelope, so a rejected (unknown-namespace) block still reconfigured per-process delivery state | `dispatch.go` registerSubscriptions | fixed: resolveSubscriptionNamespace + `if !ok { return }` moved above the Set* calls; new test `TestUnknownNamespaceBlockHasNoSideEffects` |
| 2 | NOTE | SDK→engine end-to-end wiring (`SetStartupSubscriptionsIn`/`SetEnvelope` -> ready RPC -> registerSubscriptions) unproven; the spec's `.ci` files were not authored | pkg/plugin/sdk, test/plugin | acknowledged: PARKED. Mechanism is compositionally proven by unit tests through the real entry points; the fork/ready-RPC transport needs a live env (see drain recipe). Recorded as remaining work, spec NOT closed. |
| 3 | NOTE | Envelope path is stricter than bare path for a non-JSON string-passthrough payload (would fail the whole emit); untested for "null"/passthrough | `dispatch.go` buildEventEnvelope | addressed: added `TestEnvelopeWithNilSignalPayload` (null passthrough). Non-JSON-string producers already violate the documented payloadToJSON contract. |
| 4 | NOTE | Mixed `["*", "<explicit>"]` registers duplicate subs but GetMatching breaks after first match per proc — safe | subscribe.go GetMatching | acknowledged, no action (no double delivery) |

### Fixes applied
- Reordered `registerSubscriptions` so an unknown-namespace block is rejected with zero side effects (no per-process format/encoding/envelope mutation). Added `TestUnknownNamespaceBlockHasNoSideEffects`.
- Added `TestEnvelopeWithNilSignalPayload` covering the signal (nil -> "null") payload through the envelope.

### Run 2+ (re-runs until clean)
Re-verified after fix: `go test` + `golangci-lint` on all 4 packages clean; the ISSUE's producer reordered and guarded by a test. No new findings.

### Final status
- [x] Independent review shows 0 BLOCKER, 0 ISSUE (the one ISSUE fixed and re-verified)
- [x] All NOTEs recorded above (wiring NOTE = parked `.ci`, documented in drain recipe)
- Review artifact: `tmp/review/fixit-plugin-event-subscription-<SID>.md` (verdict=clean, 7 files) via `review_gate.py record`

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `pkg/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

## Notes
- **Design session 2026-07-16.** Status skeleton -> design. Current Behavior
  re-verified line by line (two line ranges from the skeleton had drifted and are
  corrected in place). A-1..A-4 validated; A-5..A-7 added and validated; R-4..R-6
  added. Gap C discovered and raised for a scope decision. Key Design Decisions
  D-1..D-7 recorded. ~~**NOT ready for implementation: needs Thomas on Gap C scope,
  on the D-2 method name, and on the R-6 collision with
  `spec-fixit-reject-fence-observability`.**~~
- **READINESS RESOLUTION (2026-07-17).** Status design -> ready. All three blockers
  named in the struck "NOT ready" sentence above are resolved:
  - **Gap C scope** -- APPROVED into scope by Thomas 2026-07-16 (Task > Gap C
    "Decision (user, 2026-07-16)": implement the expansion). Consequently every
    "IF APPROVED"/"if approved" hedge elsewhere in this spec now reads APPROVED and
    its work is REQUIRED, not optional: the Behavior-to-change Gap C bullet, the R-4
    mitigation cell, the Wiring Test "Gap C" row, AC-8, the TDD
    `TestStartupWildcardExpandsToNamespaceEvents` row, the Files-to-Modify
    `parseEventString` note, Implementation Phase 4, and D-7. A `.ci` proving an
    event actually reaches the exabgp script is mandatory (A-6: none exists today).
  - **D-2 method name** -- AUTONOMOUS DEFAULT (2026-07-17): `SetStartupSubscriptionsIn`
    (the name D-2 itself proposes; recorded at D-2). Override before implementation
    if another name is preferred.
  - **R-6 collision** -- AUTONOMOUS DEFAULT (2026-07-17): option (a); resolved and
    recorded at R-6 in Design Insights. `spec-fixit-reject-fence-observability` uses
    the reload-generation counter (verified: `reload_generation.go` declares no
    event type, namespace, or `Emit`), so there is no cross-spec dependency.
  Phase 0's "re-read `internal/component/plugin/server/`" still stands: the line
  numbers in Current Behavior may have drifted under concurrent edits, so the
  implementer re-confirms them before touching code (they verified substantively as
  of 2026-07-17).
- No file under `internal/component/plugin/server/` was touched by this design
  session: a concurrent session was implementing `spec-fixit-reject-fence-observability`
  in that package. Reading only.
- Authored 2026-07-10 from the `spec-test-coverage-gaps` AC-2 engine-step
  implementation (see that spec's Design Insights: the executor's exclusive-window
  subscription workaround and the `deliver_subscriber_test.go` delivery proof).
  Skeleton = captured intent with verified `file:line` evidence; moves to `design`
  when picked up.
