# Guard enumerates what matters instead of subtracting what does not

A guard that lists the fields it cares about is correct on the day it is written.
It is wrong on the day somebody adds a field. The new field is not on the list,
so it compares equal. The guard then answers "nothing changed" about a thing
that changed, nothing goes red, and the caller drops a correctly computed value
one line later and reports success.

The shape that survives is the opposite one. Compare the WHOLE derived set, then
SUBTRACT the members one decision proved it can ignore. A field nobody
classified lands on the conservative side by construction, which is the
fail-closed direction `ai/rules/evidence.md` asks for. Distrust any equality
helper, diff predicate, or header comparison whose body reads as a column of
field names.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-13 | bgp-peer-settings-reload-ignored | bgp reload peer reconcile | `peerSettingsEqual` (`internal/component/bgp/reactor/reactor_api.go`) decided whether a peer changed on reload by comparing a hand-written list of about 15 of `PeerSettings`' roughly 50 fields. `ImportFilters`, `ExportFilters`, `RouteReflectorClient`, `ASOverride`, `RSClient`, `ClusterID`, `NextHopMode`, `MD5Key` and about 25 more were absent, and `StaticRoutes` was compared by length alone. A reload touching only those fields reported "functionally equivalent", so `reconcilePeersJournaled` put the peer in neither `toRemove` nor `toAdd` and dropped the freshly parsed settings. The datapath kept enforcing the old chain while `bgp peer` DISPLAYED the new one. The display reads the re-parsed config and the enforcement reads the running peer. `peerDiffCount` reported 0 changes. An operator tightening policy to block a prefix saw success and got the old permissive chain, which is the failure direction that looks like the safe one | fixed. Both replacement guards are SUBTRACTIONS from a derived set. `peerSettingsEqual` is `reflect.DeepEqual` over the whole struct minus `Capabilities`, which compare semantically by wire encoding. `peerSettingsSwapPlan` (`peer_settings_apply.go`) decides swap-or-restart as "not equal once one copier has neutralized the fields it delivers". A field added tomorrow therefore forces a restart until somebody classifies it on purpose. The plan also RETURNS that copier, so the set it judged deliverable is the set the apply site writes. `openHeaderEqual` (`peer_settings_negotiation.go`) got the same treatment over `message.Open`, and it is the sharper warning. Its hand-picked list was TOTAL over the struct when it was written, so a plain revert stays green and only a field escaping the list reproduces the failure. Proven by `TestPeerSettingsRestartRequiredIsFailClosed` over six unclassified fields, `TestOpenHeaderEqualCoversEveryOpenField`, and `test/reload/reload-import-policy-applies.ci`, which asserts the edited peer REJECTS the route while a control peer accepts it |
