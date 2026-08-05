# The Birdwatcher-compatible Looking Glass API

## Status of this document

This is a specification of the JSON API Ze's looking glass serves for Alice-LG
and other birdwatcher clients. It is normative for Ze: an implementation change
that contradicts a MUST here is a defect in the implementation or in this
document, and one of the two is wrong.

It exists because there is no other reference. Upstream birdwatcher publishes no
schema for its own responses, and its README defers to "an API defined by Barry
O'Donovan's birds-eye". Every statement below about UPSTREAM behavior was
established by reading `alice-lg/birdwatcher`'s source and names the function it
came from; nothing here is quoted from upstream documentation, because none
exists.

## 1. Conventions

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT,
RECOMMENDED, MAY and OPTIONAL in this document are to be interpreted as described
in RFC 2119.

"Server" means Ze's looking glass. "Client" means Alice-LG or any other consumer
of this API. "Upstream" means `alice-lg/birdwatcher`.

## 2. Transport and framing

The server MUST serve every endpoint over HTTP GET. It MUST NOT require any
other method, and it MUST NOT define side effects for any endpoint: this surface
is read-only.

The server MUST set `Content-Type: application/json`.

The server MUST answer a request under `/api/looking-glass/` that matches no
endpoint with a JSON error body, and MUST NOT fall through to an HTML error
page. A client parsing JSON must never receive markup.
<!-- source: internal/component/lg/server.go -- route registration -->

Field names MUST be `snake_case`. This is the one surface where Ze abandons its
own `kebab-case` convention, because the client vocabulary is birdwatcher's.
Translation between the two is the responsibility of the `transform*` functions.
<!-- source: internal/component/lg/handler_api.go -- transformProtocols -->

## 3. Endpoints

The server MUST expose the following. All are relative to
`/api/looking-glass/`.

| Endpoint | Payload key | Returns |
|----------|-------------|---------|
| `status` | `status` | Daemon status and version |
| `protocols/bgp` | `protocols` | Every BGP session as a protocol object |
| `protocols/short` | `protocols` | The same set, reduced |
| `protocols/bmp` | `protocols` | BMP-monitored peers as protocol objects |
| `routes/protocol/{name}` | `routes` | Routes learned from one session |
| `routes/peer/{peer}` | `routes` | Routes learned from one peer address |
| `routes/table/{family}` | `routes` | Routes in one address family |
| `routes/filtered/{name}` | `routes` | Routes an import policy rejected |
| `routes/export/{name}` | `routes` | Routes exported to a session |
| `routes/noexport/{name}` | `routes` | Routes withheld from a session |
| `routes/count/protocol/{name}` | `routes_count` | A count only |
| `routes/prefix` | `routes` | Routes matching a prefix query |
| `routes/search` | `routes` | Routes matching a free query |
| `routes/bmp/{name}` | `routes` | Routes seen for a BMP-monitored peer |

A `{name}` or `{peer}` path segment MUST be validated before use. The server
MUST answer an invalid one with 400 and MUST NOT pass it to the engine.

Endpoints returning routes MAY accept `limit` and `offset` query parameters. When
a client sends a `limit` above the server's maximum, the server MUST answer 400
and MUST NOT silently clamp: a client must never believe it received a full page
when it did not.
<!-- source: internal/component/lg/handler_api.go -- parsePagination -->

## 4. Response envelope

Every response MUST be a JSON object carrying an `api` member and exactly one
payload member.
<!-- source: internal/component/lg/handler_api.go -- apiEnvelope -->

```json
{
  "api": {
    "Version": "Ze Looking Glass",
    "result_from_cache": false
  },
  "protocols": {}
}
```

| Member | Type | Requirement |
|--------|------|-------------|
| `api.Version` | string | MUST be present. The capitalized spelling is REQUIRED, because it is birdwatcher's. Ze reports a product name; a client MUST NOT parse it as a semantic version |
| `api.result_from_cache` | bool | MUST be present. Ze MUST report `false`: it answers from live state and operates no response cache |

A response to a routes endpoint MUST carry `routes_count` beside the envelope,
giving the number of route objects in `routes`.

A client MUST ignore members it does not recognize. The server MAY add members in
a later revision, and MUST NOT treat that as a breaking change.

## 5. Protocol object

`protocols/bgp` and `protocols/short` MUST return an object keyed by protocol
name, whose values are protocol objects.
<!-- source: internal/component/lg/handler_api.go -- transformProtocols -->

| Member | Type | Requirement |
|--------|------|-------------|
| `bird_protocol` | string | MUST equal the key this object is stored under |
| `state` | string | MUST be present |
| `state_changed` | string | RFC 3339 timestamp of the last transition. MUST be present, MAY be empty |
| `neighbor_address` | string | MUST be present |
| `neighbor_as` | number | MUST be present |
| `description` | string | MUST be present, MAY be empty |
| `last_error` | string | MUST be present, and MUST be empty when there is no error |
| `table` | string | MUST be `master` for a BGP session and `bmp` for a BMP-monitored peer |
| `uptime` | number | Seconds. MUST be present |
| `routes_received` | number | See Section 7 |
| `routes_imported` | number | See Section 7 |
| `routes_exported` | number | See Section 7 |
| `routes_filtered` | number | See Section 7 |
| `routes_counts_available` | bool | See Section 7. This member is Ze's own and MUST NOT be expected from upstream |
| `routes` | object | MUST carry `imported`, `filtered`, `exported` and `preferred`, mirroring the four flat counts |

The server MUST emit both the flat counts and the nested `routes` object, because
clients differ in which they read.

## 6. Route object

Every routes endpoint MUST return an array of route objects under `routes`.
<!-- source: internal/component/lg/handler_api.go -- transformRoutes -->

| Member | Type | Requirement |
|--------|------|-------------|
| `network` | string | The prefix. MUST be present |
| `gateway` | string | Next hop. MUST be present |
| `metric` | number | MULTI_EXIT_DISC. MUST be present |
| `interface` | string | MUST be present. Ze MUST report the empty string: it does not attribute a route to an interface on this surface |
| `from_protocol` | string | The learning session. The server MUST substitute the peer address when one is known |
| `age` | number | Seconds. MUST be present |
| `learnt_from` | string | Peer address. MUST be present |
| `primary` | bool | Whether this is the best path. MUST be present |
| `bgp` | object | MUST be present, as below |

### 6.1 The `bgp` member

| Member | Type | Attribute |
|--------|------|-----------|
| `origin` | string | ORIGIN |
| `as_path` | array | AS_PATH |
| `next_hop` | string | NEXT_HOP |
| `local_pref` | number | LOCAL_PREF |
| `med` | number | MULTI_EXIT_DISC |
| `communities` | array | COMMUNITY, as `[asn, value]` pairs |
| `large_communities` | array | LARGE_COMMUNITY, as `[global, local1, local2]` triples |
| `ext_communities` | array | EXTENDED_COMMUNITY |

The server MUST emit `communities` and `large_communities` in the shapes above.
A client MUST NOT assume a flat list.

## 7. Route counts and their availability

This section is the one place Ze knowingly diverges from upstream, and the
divergence is deliberate.

### 7.1 What upstream does

Upstream OMITS a count member entirely when BIRD reports the count as
unavailable. Its parser returns before writing anything when BIRD prints `---`:

```go
func setChangeCount(name string, value string, res Parsed) {
	if value == "---" { // field not available for protocol
		return
	}
	res[name] = parseInt(value)
}
```

Established by reading `alice-lg/birdwatcher`, `bird/parser.go`,
`setChangeCount`. Absence, upstream, means "no such number for this protocol".

### 7.2 What Ze does, and why

Ze MUST emit all four count members unconditionally, including when it has no
source for them, in which case each MUST be `0`.

This contradicts Section 7.1 on purpose. Owner decision, 2026-08-05: this is a
compatibility surface, Alice-LG's reaction to an absent count could not be tested
at the time, and a fabricated zero was judged the lesser risk against a client
that might render blank or error.

Because that zero is not a measurement, the server MUST also emit
`routes_counts_available`:

- It MUST be `true` only when the counts were produced from a real source.
- It MUST be `false` when the counts are placeholders.
- A client that distinguishes "no routes" from "unknown" MUST consult it, and
  MUST NOT infer either from a count of `0` alone.

<!-- source: internal/component/lg/handler_api.go -- routeCountsAvailable -->

The producer already draws this distinction and the server MUST preserve it:
`fetchRibRouteCounts` omits the count keys when the `bgp-rib` plugin is not
loaded, recording that they are "never faked to 0".
<!-- source: internal/component/cmd/peer/summary.go -- fetchRibRouteCounts -->

For a BMP-monitored peer the server MUST report `routes_counts_available` as
`false`: no source is consulted for those four members at all.
<!-- source: internal/component/lg/handler_api.go -- transformBMPProtocols -->

### 7.3 Known equalities and absences

`routes_imported` MUST be expected to equal `routes_received`. Both are the
Adj-RIB-In size, because Ze drops rejected routes before storage and keeps no
separate pre-policy count. A client MUST NOT infer a policy drop from the
difference, which is always zero. BIRD reports received greater than or equal to
imported, so this is a real behavioral difference and not a naming one.
<!-- source: internal/component/cmd/peer/summary.go -- mergeRibRouteCounts -->

`routes_filtered` MUST be expected to be `0`. Ze retains no filtered routes: the
reject gate drops the route rather than storing it, so nothing can produce the
number. `routes/filtered/{name}` MUST return an empty array for the same reason,
and MUST do so with a success status rather than an error, because an empty
result is the honest answer.

Adding a filtered-route store is `plan/spec-bgp-filtered-route-storage.md`. Until
it lands, this section is the contract.

## 8. Divergence summary

A client written against upstream birdwatcher and pointed at Ze will observe
exactly these differences.

| # | Divergence | Deliberate |
|---|-----------|-----------|
| 1 | Unavailable counts are `0` rather than absent, with `routes_counts_available` carrying the truth | Yes, Section 7.2 |
| 2 | `routes_imported` always equals `routes_received` | Yes, Section 7.3 |
| 3 | `routes_filtered` is always `0` and `routes/filtered/{name}` is always empty | Yes, Section 7.3 |
| 4 | `interface` is always empty | Yes, Section 6 |
| 5 | `api.result_from_cache` is always `false` | Yes, Section 4 |

## 9. Testing

`internal/component/lg/handler_api_test.go` feeds `transformProtocols` the
payload the engine actually produces rather than a hand-built map. That test
exists because an earlier version read `ze["peers"]` while the engine returns
`{"summary":{"peers":[...]}}`, so every peer was dropped and Alice-LG saw no
sessions at all. A test built from a hand-written fixture would not have caught
it, which is why this one MUST keep using the real shape.
