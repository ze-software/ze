# Connected post-retirement fences: independent source review

November 13, 2026. Source CLEAR for the whole new
`internal/plugins/connected/lifecycle_test.go`; APPROVE all four exact private
guard-removal overlays as proposed supplementary discrimination. No execution,
tracked authoring, formal CLEAN or closure by this reviewer.

Test SHA-256:
`f7b19daaaa0be1f5fb82b4638da8539f2d05ca1b43d5e1791bfe73c651b496a7`.

Read the complete new test and reached current handleAddrAdded,
handleAddrRemoved, reemitAll and retire producers through runtime/security,
tests/wiring/edges and style/docs lenses. Diffed each private replacement Go
file against its current source: only its first-line MUTATION-APPLIED marker
and the named method's `if o.retired` return block differ. Read every overlay
JSON: each maps exactly one production file to that private copy. No working
tree production mutant exists from these overlays.

All four operations start with two useful connected Adds whose protocol,
ReplayID, prefix and action are checked; both covering-address sets contain
one address. The test deep-clones the inventory before retirement, requires
retired true, and checks exactly one incremental Remove for each prefix. It
then invokes one operation synchronously. Exact publication count prevents
post-retirement announcements/withdrawals, and a separate deep comparison
protects inventory that deferred local cleanup still owns. Blanket publication
suppression would fail the initial Add and Remove controls.

| Overlay | Exact guard removed | Expected semantic failure | JSON SHA-256 |
| --- | --- | --- | --- |
| lateAdd | handleAddrAdded | An extra Add for 198.51.100.0/24 and changed inventory | 8d777f0a0607902dbf375f7ece81a48f1115bf74c3b6c8f4e6f4d4892beea2d5 |
| lateRemove | handleAddrRemoved | An extra Remove for 192.0.2.0/24 and lost inventory | 611401e1e9801d90e94f62ca9965bc8d0a2c5f36c4ea368d74b02e87282fe7df |
| replay | reemitAll | Extra live-prefix Adds with ReplayID 73 | 048ed9975f8ee2f97ed9e388b798baea3d860ddf6805fc74f24e22708ed2eefd |
| secondretire | retire | Repeated Removes for both retained prefixes | 551224e927aa1731dc8c182546f0bebde8f9999c06b2f79684ab8b5390a9596c |

Private replacements are under `scratch/connected-retirement-mutations/`:

- lateAdd.go: af0ab80213be14b07c4b288aa47770dd9d4b7c3fa010692f35e58b89a052c792
- lateRemove.go: 724a1e2ac65a9c5c78de746384b6dd3497d83a0512bb70eafffa48976b51006e
- replay.go: 23ba788390426a566288040c41c3d99f730e49688cdd10c9b5778fbb6ab975e0
- secondretire.go: f210b0018d470b7ed747892e4153156c22b9e97ee0f68c72d5dca7beba98244f

This deterministic test supplements the real-runner shutdown barriers: SDK
shutdown can reject a late delivery before the observer's own guards execute.
It does not claim another transport test or Loc-RIB cleanup proof. No existing
real-runner or production body is changed by this addition.

Read ROOT's actual baseline receipt
`job-connected-retirement-fences-baseline-58ea4496.log`: the parent and all
four named children pass. Read prior full package receipt
`job-connected-graceful-final-race-coverage-522bd10c.log`: all seven real-runner
parents and existing package controls pass; statement coverage is 86.7%.
ROOT owns the four actual guard-removal REDs and subsequent production GREEN;
no result is inferred from approved overlay text. Retained immutable 431-path
deployment review context remains unchanged until later delta reconciliation.
