# Test weakenings this commit accepts

The BFD two-link prestarts moved from `*-prestart.sh` files in the scenario
directory to Go constants indexed by link-local owner. The removed case tested a
refusal for a state the new type cannot hold, so nothing it guarded is left
unguarded.

| Test | Reason |
|------|--------|
| TestPrestartRunsBeforeTheDaemon | The "no wrapper for the container" case is gone. It checked that a `<peer>-prestart.sh` naming a container with no link-local start wrapper was refused. Prestarts are now an `ownerPrestarts` array indexed by `zeLinkLocalOwner` / `frrLinkLocalOwner`, so a prestart for a container with no wrapper cannot be written at all. The other refusal (an owner whose placeholder no scenario file carries) is kept and now calls `renderPrestarts` directly. |
