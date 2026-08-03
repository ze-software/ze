# RS Env Promote: worker-queue-size to YANG

Promoted `ze.rs.chan.size` env var to a YANG config leaf at
`bgp/route-server/worker-queue-size` with env var override
`ze.bgp.route-server.worker-queue-size`.

## Pattern: Env-to-YANG promotion for BGP plugin config

1. Create `<plugin>/yang/ze-<name>-conf.yang` augmenting `/bgp:bgp`
2. Run `yang_glue.go` + `plugin_imports.go` to generate embed/register and update `all/all.go`
3. Add `ConfigRoots`, `Features`, `YANG` to plugin registration
4. Add `OnConfigure` handler that reads the JSON config tree and applies values
5. Env var takes precedence: check `env.Get(key) != ""` before applying YANG value
6. Rename env var to mirror YANG path per `config.md`

## Key decisions

- **OnConfigure timing:** fires at Stage 2 of `p.Run()`, before events (Stage 4+).
  Workers are lazy-created on first Dispatch, so SetChanSize reaches them before any
  worker channel is allocated.
- **SetChanSize under wp.mu:** safe because `cfg.chanSize` is only read in `Dispatch`
  under the same lock.
- **No backward compatibility:** old env var removed entirely per user instruction.

## Review catch

Initial implementation used `ze.rs.worker-queue-size` which violates the
`config.md` hierarchy rule (env var path must mirror YANG tree path).
Corrected to `ze.bgp.route-server.worker-queue-size` during review.

## Files

None recorded.
