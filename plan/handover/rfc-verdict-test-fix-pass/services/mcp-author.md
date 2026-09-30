# services child, package internal/component/mcp: author handoff (2026-09-29)

Stems rfc8414, rfc9728. RFC9728-3.1-3 is blocked (spec-mcp-protected-resource-metadata-path) and untouched.
RFC9728-7.1-2 tags only cmd/ze/hub (not this package): left to the cmd/ze/hub author.

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|------------|-----------------------------------|-----------------|------------------|-------|
| RFC8414-2-1 | tests | NEW `TestRFC8414IssuerIsHTTPSWithoutQueryOrFragment` (as_metadata_rfc8414_test.go): + https issuer, no query/fragment, fetched and issuer returned; - http issuer, `?tenant=a`, bare `?`, `#frag` each refused with zero metadata and no request reaching the AS; document without issuer refused. Old units in as_metadata_test.go keep their tags | + revert fetchASMetadata; - revert asMetadataURL | enforced | |
| RFC8414-4-1 | row + tests | Row widened to lead-in + step 1 ("Remove any JSON-applied escaping ..."), same shape as RFC9728-6-3 (correction paragraph). NEW `TestRFC8414MetadataIssuerUnescapedBeforeCompare`: + metadata issuer with escaped solidi and b written as backslash-u-0062 matches configured https://host/ab; - configured issuer spelled as the raw JSON text is refused. JWT-path units keep their 4-1 tags | + and - revert fetchASMetadata | enforced | steps 2/3 stay on 4-2/4-3 |
| RFC8414-3.3-1 (narrowing) | row split | NEW row RFC8414-6.1-4 [MUST] quoting the §6.1 confidentiality sentence; extraction 6.1:4 remapped from 3.3-1 to 6.1-4; 3.3-1 keeps its §3 quote and tags. NEW `TestRFC8414MetadataFetchUsesConfidentialIntegritySuite`: + negotiated suite is in tls.CipherSuites and not InsecureCipherSuites; - cleartext server behind an https issuer and an RC4-only TLS server each give zero metadata, neither handler reached | + revert fetchASMetadata; - revert oauthHTTPClient (both re-recorded after a comment edit) | 6.1-4 new, needs a first verdict (enforced expected) | RC4 refusal rests on Go's default client suites; claim words it as the suite Go lists insecure |
| RFC9728-2-2 | tests | NEW `TestRFC9728AuthorizationServersAreIssuerIdentifiers`: + authorization_servers is a JSON array of strings holding exactly the validated issuer, https, no query/fragment; - http, query and fragment authorization-server values refused by NewStreamable so no document lists them | + revert writeResourceMetadata; - revert asMetadataURL | enforced | row is [MAY]; negative is a genuine refusal path (R1 b) |
| RFC9728-3-2 | tests (query clause) + BLOCKED (trailing slash) | NEW `TestRFC9728WellKnownInsertedBeforeQuery`: + `https://mcp.example/mcp?tenant=a` at `/.well-known/oauth-protected-resource/mcp?tenant=a`, query-only `https://mcp.example?tenant=a` at `/.well-known/oauth-protected-resource?tenant=a`, advertised URL and resource match; - query dropped, suffix after path, bare suffix, suffix after query each serve no document | + revert resourceMetadataURL; - revert resourceMetadataPath | weak until D1 of spec-mcp-protected-resource-metadata-path lands | `https://host/mcp/` is published from `/mcp` (TrimRight in resourceOriginAndPath): same defect as 3.1-3. Recommend adding 3-2 to that spec's AC-4 |
| RFC9728-3.1-2 | row (retired, R2) | Row deleted; `Retired 2026-09-29` paragraph in rfc/corrections/rfc9728.md; extraction 3.1:1 now `excluded` / `binds-another-role`; extraction re-signed 2026-09-29 with resign-reason; audit entry and its 2 discrimination records removed; tags deleted from `TestRFC9728MetadataQueriedWithGET` (approval D-15 given; test stays untagged) | records removed | leaves the audit | "Support remaining" line in the summary updated |
| RFC9728-3.3-1 | BLOCKED | existing tests fine; the only gap is `https://mcp.example/mcp/` (resource returned differs from identifier the URL was formed from) = D1 of spec-mcp-protected-resource-metadata-path | none | weak until D1 | Recommend adding 3.3-1 to that spec's AC-4 |
| RFC9728-5.1-1 | tests + annotation | NEW `TestRFC9728ChallengeNamesMetadataURL`: + 401 carries `WWW-Authenticate: Bearer` with resource_metadata EXACTLY the metadata URL of the metadata resource (override differs from audience), and GET there returns that resource's document. Row gets `{single-polarity: positive; ...}` (permission row, no negative tag at HEAD) | + revert resourceMetadataURL | enforced | |

## Verified
- `go test -race -count=1 ./internal/component/mcp/` under `./le job run`: ok (19.4s). gofmt clean. Post-edit lint hook clean.
- `./le rfc check`: for rfc8414/rfc9728 only STALE-verdict lines remain (2-1, 4-1, 3-2, 2-2, 5.1-1), which the judge re-stamps. The earlier extraction-resign finding is fixed.
- OWED (main thread): `./le go lint run`, `./le verify worktree`.

## Notes for the main thread
- rfc/audit/rfc8414.json and rfc/audit/rfc9728.json also carry a `reaudit_history` array and re-hashed `tests` maps from a `./le rfc reseal` I did not run (someone else ran it while I worked). My only audit edit is removing the RFC9728-3.1-2 entry.
- The approval for mcp.TestRFC9728MetadataQueriedWithGET was recorded by `./le rfc approve` (its store is outside the tree I grep).

## Files changed
- internal/component/mcp/as_metadata_rfc8414_test.go
- internal/component/mcp/metadata_rfc9728_test.go
- rfc/short/rfc8414.md
- rfc/short/rfc9728.md
- rfc/corrections/rfc8414.md (new)
- rfc/corrections/rfc9728.md
- rfc/extraction/rfc8414.json
- rfc/extraction/rfc9728.json
- rfc/audit/rfc9728.json (RFC9728-3.1-2 entry removed)
- rfc/discrimination/rfc8414.json
- rfc/discrimination/rfc9728.json
