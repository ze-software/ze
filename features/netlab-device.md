# netlab device

## Meta

| Field | Value |
|-------|-------|
| Name | netlab device |
| Page | docs/guide/netlab.md |
| Kind | packaging |
| Scope | complete |
| Level | experimental |
| Components | contrib/netlab, internal/le/test/netlab |
| Real-path tests | test/plugin/netlab-lab-profile.ci |
| Docs | docs/guide/netlab.md |
| Doc review | 2026-10-07: the row itself records that netlab up and validate are unrun; contrib/netlab and the render-check action exist |
| Defect review | 2026-10-07: no open spec or journal row found naming netlab |
| Extra criteria | supported: a live netlab up and netlab validate run = none yet |

## Description

Ze runs as a netlab daemon device under containerlab. `contrib/netlab/` mirrors what netlab installs: the daemon definition, the Jinja2 templates, a reference topology, and the committed render. The templates render the whole running configuration into `/etc/ze/ze.conf`. Declared modules are bgp, ospf, isis, bfd, and routing. Ze reloads on SIGHUP, and netlab sends none, so `ze.yml` declares `initial.reload: false`. A configuration change in a running lab needs a manual signal or a node restart. `netlab validate` reads `ze cli -c "show ... \| json compact"` through `docker exec`. `./le test netlab render-check` renders the templates with a real netlab and compares the result against the golden files. `test/plugin/netlab-lab-profile.ci` starts a daemon from one of them. A live lab was not started, so `netlab up` and `netlab validate` are unrun. The declared features are rendered and parsed, never proven against netlab's integration tests. No LLDP. <!-- source: contrib/netlab/ze.yml -- daemon_config, features --> <!-- source: contrib/netlab/ze/ze.j2 -- running configuration template --> <!-- source: internal/le/test/netlab/actions.go -- Actions -->
