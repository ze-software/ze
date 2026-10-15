# Reviewed claims: session ef77fab0

Each row records a docs claim a reader compared with a change that preserves
behavior. The rules and the check that reads this table are in
`docs/contributing/documentation-testing.md`.

| Doc | Source | Symbol | Commit | Reason |
|-----|--------|--------|--------|--------|
| docs/guide/route-injection.md | pkg/plugin/sdk/sdk.go | Plugin | 23104ccc4b | adds the configApplyRegistered field behind the config-verify refusal; the claim, that a Go plugin injects a route with p.UpdateRoute, is unaffected and UpdateRoute is unchanged |
