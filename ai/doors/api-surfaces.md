# The external API surfaces door

<!-- door-base: internal/component/api internal/component/web internal/component/lg internal/component/gnmi internal/component/mcp internal/component/telemetry -->

Read `system.md` first. This door covers the interfaces that programs and
browsers outside Ze talk to. They are REST and gRPC (`internal/component/api`), the web UI
(`web`), the looking glass (`lg`), gNMI (`gnmi`), MCP (`mcp`) and the Prometheus
exporter (`telemetry`). Their wiring lives in `cmd/ze/hub` (`hub-engine.md`).

## Where it sits

- **They are thin adapters.** None of them owns a command or reads protocol
  state. The hub builds one `plugin.CommandDispatcher` per surface with
  `serverDispatcher` (`cmd/ze/hub/main_servers.go`), and every one of them calls
  the same `pluginserver.Dispatcher`:
  - REST and gRPC share one `api.APIEngine` (`cmd/ze/hub/api.go`
    `buildAPIEngine`). REST turns JSON parameters into command tokens
    (`api.BuildCommand`) before anything parses them (`ai/digests/api-ipc.md`).
  - The web UI and MCP get their own dispatchers (`cmd/ze/hub/main.go`, audit
    surfaces `Web` and `MCP`). The looking glass gets the web one
    (`cmd/ze/hub/service_lg.go` `buildLGService`) and never imports a RIB or
    peer package (`docs/architecture/web-interface.md`, "LG Data Access").
  - MCP builds its tools at `tools/list` time from the live command list
    (`internal/component/mcp` `CommandLister`), so a new command needs no MCP edit.
- **Two surfaces do not go through the command registry.** gNMI serves the
  YANG config tree (Get, Set, Subscribe) through a config session manager
  (`docs/architecture/api/architecture.md`, "gNMI Transport"). The exporter
  serves the metrics registry. Config edits from the web editor
  (`internal/component/web` `EditorManager`) and from the REST and gRPC config
  sessions go through the config editor, not through commands.
- **Every one can be compiled out**: `ze_rest`, `ze_grpc`, `ze_web`, `ze_lg`,
  `ze_gnmi`, `ze_mcp`, `ze_telemetry` (`feature-gates.txt`). The hub reaches them
  only through gated registration (`docs/architecture/plugin/feature-gates.md`).

## Who depends on it from outside

| Outside party | Contract | Breaks when |
|---------------|----------|-------------|
| Alice-LG and other birdwatcher clients | `docs/architecture/api/birdwatcher-compat.md`, which is normative: endpoints, envelope, snake_case keys (the one JSON exception, `web-interface.md`), and five listed divergences from upstream | A field, a count or an endpoint direction changes. Upstream publishes no schema, so that page is the only reference |
| gNMI collectors and tools (gnmic, Ansible, controllers) | gNMI Capabilities, Get, Set and Subscribe over the Ze YANG models, port 9339 by default (`api/architecture.md`, "gNMI Transport") | A YANG path or a key changes, because the client addresses config by path |
| REST clients | `/api/v1/...` routes, the generated OpenAPI at `/api/v1/openapi.json` (`internal/component/api/rest/server.go`), `docs/guide/api.md` | A route, a parameter or the response JSON changes |
| gRPC clients | `api/proto/ze.proto` (`ZeService`, `ZeConfigService`) | A message or a field number changes |
| MCP clients | The `2026-07-28` Streamable HTTP profile, the only one accepted (`docs/architecture/mcp/overview.md`), and tools derived from commands | A command's name, parameters or answer shape changes, because the tool changes with it |
| Prometheus, and dashboards built for Netdata | Metric names and labels (`docs/guide/monitoring.md`, `docs/plugin-development/metrics.md`). The OS metrics copy Netdata's naming, and no test compares them with a running Netdata | A metric or a label is renamed |
| Browser users | The web UI and its URL scheme (`web-interface.md`, "URL Scheme") | A page, a form or a route changes |

Each surface carries its own authentication and TLS. A listener that serves
without it is a security defect, not a default (`plan/journal/enabled-gate-discards-settings.md`).

## Boundaries

- **Never add a command in an adapter.** Register it with its owner
  (`commands-cli.md`). The adapter reaches it through the dispatcher.
- **Never import a protocol package to read state.** Ask the dispatcher. The LG
  and the web UI hold no RIB or peer import (`ai/digests/web.md`).
- **Never copy a fact the YANG model or the registry already holds**: a field
  description, a parameter list, a command name (`ai/rules/principles.md`).
- **Markup is `.templ`, with no inline script or style** (`ai/rules/architecture.md`).
- **Always-on code never imports a gated surface** (`feature-gates.md`, "The one
  invariant"). `cmd/ze/hub` reaches each one through a registration seam such as
  `gnmi_infra.go`.

## Read before code

| Read | It answers |
|------|-----------|
| `docs/architecture/api/architecture.md` | REST and gRPC auth flow, identity per transport, the gNMI transport |
| `docs/architecture/api/birdwatcher-compat.md` | The LG JSON contract and its tests |
| `docs/architecture/web-interface.md`, `web-components.md`, `web-workbench-pages.md` | Web routes, auth, editor, workbench, LG wiring |
| `docs/architecture/mcp/overview.md` | The MCP profile, auth and tool derivation |
| `docs/guide/monitoring.md`, `docs/plugin-development/metrics.md` | The exporter and the metric naming rules |
| `ai/digests/api-ipc.md`, `ai/digests/web.md`, `ai/digests/mcp.md`, `ai/digests/observation-telemetry.md` | The traced request flow of each surface |
| `ai/patterns/web-endpoint.md` | The files a new page or endpoint needs |
| `ai/rules/cli.md`, `ai/rules/config.md`, `ai/rules/user-facing-errors.md` | Answer data, listener config, errors on every interface |

## Traps

- **An `enabled` gate that drops the security settings.** LG TLS, the gNMI
  token, the API TLS and the web address were each read only when config said
  `enabled`. As a result, an env-started listener ran with no auth or on the wrong address
  (`plan/learned/RECURRING-PATTERNS.md`, "An `enabled` gate on config extraction
  discards the service's security settings").
- **The LG reshapes engine output and gets it wrong.** `primary` was read from a
  key no RIB row carries, and peer routes were counted in both directions
  (`plan/journal/constant-reported-as-measured-state.md`,
  `plan/journal/counter-counts-the-wrong-packets.md`, both 2026-10-08).
- **A fixture that is not the engine's shape.** The LG test fake answered with
  strings where the producer writes numbers, and tests stayed green
  (`plan/journal/fixture-encodes-an-impossible-state.md`,
  and `birdwatcher-compat.md`, "9. Testing").
- **MCP builds arguments in a form the dispatcher refuses.** An anchored peer
  selector went out in keyword form
  (`plan/journal/guard-demands-what-the-model-cannot-supply.md`, 2026-09-15), and
  one arm of `groupCommands` dropped `takesSelector`
  (`plan/journal/helper-bypassed-by-an-open-coded-copy.md`, 2026-08-31).
- **Help text written twice.** `page_l2tp.go` hand-wrote field descriptions
  beside the YANG leaves (`helper-bypassed-by-an-open-coded-copy.md`, 2026-08-31).
- **A golden captured from a binary built without the feature tags**
  (`plan/journal/feature-test-missing-build-tag.md`, 2026-09-13).
- **A hand-written published number.** The guide stated a fixed Netdata metric
  count that no test counts
  (`plan/journal/published-value-drifts-from-the-behavior-it-describes.md`, 2026-10-07).
- **REST spells some commands itself.** The convenience routes in
  `rest/server.go` send fixed strings (`show bgp`, `show version`,
  `request reload`), so a command rename must find them.

## Gates

| Command | It judges |
|---------|-----------|
| `./le arch tier check` | No always-on import of a gated surface |
| `./le cli dispatch check` | Every command string the tree sends still routes |
| `./le web assets check`, `./le web htmx check` | Per-page asset sets, and htmx 4 findings |
| `./le doc check templ-output` | Generated templ output matches its sources |
| `./le test functional web` | The browser suite (`.wb`) |
| `./le test functional plugin` | Includes the LG `.ci` tests that drive a real `bgp-rib` over HTTP |
