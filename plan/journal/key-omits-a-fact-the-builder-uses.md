# Key omits a fact the builder uses

Work is shared between consumers by grouping them on a key, and the key is a
hand-written list of the facts that make two consumers equivalent. The producer
that builds the shared result takes MORE facts than the key names. Two consumers
that differ only in an unlisted fact then hash together, one result is built,
and every member receives it. The one whose fact was ignored receives another
consumer's bytes, decided by iteration order.

Nothing goes red: the result is well formed, the group is populated, and the
consumer that happens to build first is correct. Only the second consumer is
wrong, and only against a configuration nobody wrote a test for.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-06 | bgp-local-as-options | `announceBuildKey` (`internal/component/bgp/reactor/reactor_api_batch.go`) and `buildBatchAnnounceUpdate` beside it | The key carried a bare `localAS uint32` where the builder takes the whole local-as prepend, so two peers on one `local-as` differing only by `local-options` hashed together. `AnnounceNLRIBatch` builds one UPDATE per group and sends it to every member, so an operator with one customer mid-migration under `replace-as` and one without sent whichever AS_PATH built first to BOTH, decided by Go map iteration order. Not a distinction that failed to appear: one peer receiving another peer's AS_PATH. The shape is the point rather than this field. `git log -S` on each key field shows every one arrived in its OWN commit, and three of the four arrived as a FIX rather than with the feature: `nextHop` in `ec3ad9c766`, `addPath` in `a9f7f93843`, `propagatePrefixSID` in `4054ed854c`, and only `rsClient` landed with its feature in `ddf04953a5`. So the key is a central enumeration of "every per-peer fact that changes the built bytes", maintained by remembering, and the majority of its fields were added after the defect shipped | the local-as field is fixed in the announce-rail commit. The CLASS is not, and the class is what earns the work: `ai/rules/principles.md` bans exactly this shape, "a new feature MUST register itself and be discovered; it MUST NOT require an edit to a switch, a case, a factory, a field list, or any other central enumeration". The key currently covers every builder argument (`nextHop`, `isIBGP`, `rsClient`, `asn4`, `addPath`, `prepend`, `propagatePrefixSID`) plus `extended`, so the tree is correct TODAY and the next per-peer wire decision is the exposure. The structural repair is to make the key BE the builder's argument set rather than a parallel list of it: one struct passed to `buildBatchAnnounceUpdate` and used as the map key, so a new field cannot be added to one without the compiler demanding it in the other. Commissioned 2026-09-06 |
