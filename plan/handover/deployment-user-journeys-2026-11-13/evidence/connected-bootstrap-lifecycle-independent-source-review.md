# Connected bootstrap and graceful lifecycle: independent source preparation

Reviewed November 13, 2026 by retained independent deployment reviewer.
Disposition: source CLEAR for this bounded frozen delta. This is not a formal
Review Gate, runtime pass, spec acceptance or closure artifact.

## Frozen producers

All paths are under `internal/plugins/connected/`:

| Path | SHA-256 |
| --- | --- |
| connected.go | dc5e20bd4143ad4664ca711694dcd058a3cde8d8a6216c96c1164edfacb69171 |
| eventbus.go | cc0f5a12a3c8cf7898fccaebf16e3c2c2a6688da00beb4cc8a2f12ae91531189 |
| register.go | b65978ef5a34fa4dfad13a41ce9f65a7859f849135ad85d1cdc71f1486014a76 |
| locrib.go | 8a40cbcf07581f30fce7a8aed8b7f0234dd3af151934f9ae41fbcfb8581c9fea |
| lifecycle.go | b6998ce73909a899ebcab21c5e0b2664b155d300ac5c839d2cbe67633b002bf9 |
| bootstrap_test.go | 7a4a589dbba461e1218d0fce5e56004a3cb74504e79eff2995a88c8e309e5045 |

Read all scoped hunks, reached producer bodies and the whole new lifecycle and
bootstrap regression files through runtime/security, tests/wiring/edges and
style/docs lenses. Read the narrow connected startup, internal stop and graceful
removal paragraphs of `docs/guide/redistribution.md`; other foreign document
hunks receive no credit from this review. Applicable system/plugin-infra and
forwarding door contracts informed the review.

## Producer checks

Startup registers primary interface address subscriptions and secondary
redistribution replay subscriptions before SDK Run. Envelope delivery retains
the event namespace; OnStarted asks the active owner for inventory without
waiting for a provider that is absent. Connected publishes through the SDK in
both transports rather than depending on a process-local bus. Marshaling owns
all bytes before the pooled route batch is released.

Read the reached SDK callback registration, Run, direct callback handling,
socket dispatch and EmitEvent producers. Read actual server emitEvent,
deliverEvent, decodeTypedPayload and payloadToJSON: string JSON is decoded to
the registered connected RouteChangeBatch for engine subscribers and forwarded
without a second string encoding to process subscribers. Neither a fixture-only
payload interpretation nor a pooled-pointer escape is required.

Complete address additions/removals, replay and retirement take exclusive
emitMu. Retirement therefore follows admitted publications; later callbacks
observe retired and cannot revive a stopped source. It marks retired only after
all redistributable prefix removals succeed. A failed response may follow a
delivered withdrawal, so compensation re-announces the entire retained set;
all restoration errors are joined into the refusal. Inventory is retained and
retired remains false. Persistent transport failure is not claimed to guarantee
restoration; the guide accurately states attempts, errors and retry behavior.

SDK callback errors refuse Bye and keep its direct/socket loop alive. A successful
Bye terminates Run; its direct bridge shutdown rejects new delivery and joins
admitted handlers before the deferred local withdrawal reads the inventory.
Read DirectBridge CloseCallbacks, admission, StopDelivery, WaitDelivery and the
two delivery entry points. Local cleanup removes only connected protocol,
instance zero paths, preserving unrelated source paths. Forked path cleanup
remains the engine's disconnect ownership.

## Regression assessment and evidence boundaries

Read all bootstrap tests and their net.Pipe/DirectBridge fixture. Warm startup,
missing owner, duplicate addresses, last covering address, ReplayID and internal
Loc-RIB cleanup have positive controls. New graceful lifecycle parents execute
each of direct and socket transports: exact owned redistribution withdrawals
and unrelated-source survival; admission blocked across an in-flight publication
and retirement; failed second withdrawal, complete compensation, live event
acceptance and successful retry; and joined restoration failure plus subsequent
recovery. Barriers assert ordering rather than using elapsed time as success.

Read ROOT's existing `job-connected-bootstrap-full-race-coverage-3c29d6c0.log`:
the prior three bootstrap parents and existing package tests passed, with 87.5%
statement coverage reported. That receipt predates the four new graceful
lifecycle parents and does not prove them. Read actual semantic RED receipt
`connected-clean-bye-redistribution-red.log`: both transports had no connected
withdrawals after clean stop. ROOT still owes the new four-parent GREEN and any
required current package/runtime checks.

Generic crash-held redistribution state and the separately diagnosed external
orchestrator local-bus defect are outside this frozen implementation; this
review does not claim them repaired. ROOT owns their separate spec/journal
disposition before formal deployment closure.

Retained immutable deployment population remains 431 paths through 5797d548acf7.
This uncommitted bounded preparation must be matched against the eventual
committed delta before admission. No tests, builds, formatters, tracked edits,
subagents, formal CLEAN or closure were performed by this reviewer.
