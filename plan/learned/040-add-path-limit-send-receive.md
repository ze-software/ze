# Learned: a PATHS-LIMIT is enforced on receive, and the RIB picks what is sent

`spec-add-path-limit-send-receive` replaced the scalar ADD-PATH `limit N` with
`limit { receive N; }` at the add-path and family levels, refused the old
spelling with the replacement line, migrated ExaBGP's per-family `limit` to the
receive leaf, enforced Ze's advertised limit on received paths, and made the
send side under a peer's PATHS-LIMIT send the paths the bgp-rib election ranks
first, withdrawing a demoted path before announcing its replacement. The
reference pages are `docs/guide/add-path.md` ("Inbound Enforcement", "Best paths
under the peer's limit"), `docs/architecture/update-building.md` (the limited
election and its lock order), `docs/architecture/plugin/rib-storage-design.md`
(the `limit` argument of the recovery election) and
`rfc/short/draft-abraitis-idr-addpath-paths-limit.md`.

## What outlives the spec

1. **The reactor never ranks paths; it asks the RIB.** A limited election is a
   data extension of the source-DOWN recovery election
   (`ribevents.RecoveryRequest.Limit`), run on the destination's recovery
   worker for contested prefixes only. A path the RIB does not report (an API
   route) keeps its slot. Without a RIB, arrival order stands, counted and
   warned once per family.
2. **One admission type serves both directions.** Send bound and receive
   ceiling share the per-prefix state, section filter, body rewrite and
   rollback journal (`pathsLimitAdmission`); receive adds only a one-entry
   refused set, which doubles as the warn-once marker.
3. **Code that runs inside an adj-RIB-Out write reaches the table through the
   write, never through its mutex.** `beginAdjOut` holds the table mutex until
   `release`; taking it again parked the writer forever with `writeMu` held,
   and the only symptoms were a hung shutdown notification and a functional
   test timing out after eight minutes. Both sides of that window now say MUST.
4. **One predicate decides whether a family can carry a limit.**
   `pathsLimitEnforceable` answers advertise and enforce alike; a receive
   limit Ze could not enforce is refused at load rather than advertised.
5. **A BIRD scenario feeding MEDs to another eBGP peer needs `allow bgp_med`.**
   BIRD strips MED on eBGP export unless the export filter sets it, and the
   checker read every path as MED-less. The wait reported only its last probe,
   which the deadline cut short, so the failure read as an FRR hang until
   `interoplab.Wait` kept the last completed probe's error beside it.
6. **An interop scenario for API-originated routes cannot prove an election.**
   Those routes bypass the RIB, so best-path proof needed its own scenario
   (`bgp-paths-limit-best-path-frr`) fed from BIRD through the RIB.
