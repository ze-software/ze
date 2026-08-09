# AS112 and BGP: layering, and the two advisory checks

AS112 announces two covering prefixes only while its name server answers. The
health signal drives a BGP watchdog group. Two components must cooperate, and
neither is allowed to know the other.

<!-- source: internal/plugins/as112/redistribute.go -- generic named redistribute source -->
<!-- source: internal/component/doctor/checks_as112_coordination.go -- the two advisory checks -->

## The layering rule

**The as112 plugin never reads BGP config, and BGP never holds AS112
knowledge.** as112 emits a generic named redistribute source. BGP imports
`as112` exactly as it imports `static`. Delete the plugin directory and every
AS112 feature goes with it.

## RFC requirements this wiring carries

| Requirement | Obligation |
|-------------|-----------|
| RFC 7534 Section 3.3 | The AS112 service prefix is not advertised while anycast addresses are unconfigured or DNS software is not running. Ze enforces this with the watchdog serving-state gate |
| RFC 7534 Section 3.4 | Outbound BGP advertisement is restricted by a prefix filter permitting only the AS112 service prefixes, and by an AS_PATH filter matching only locally-originated routes. Ze enforces this with the community and the AS_PATH origin-match handles |
| RFC 3765 | NOPEER is a well-known community. An operator selects NO_EXPORT or NOPEER per deployment |
| RFC 6996 Section 5 | Private-use ASN ranges. The origin-uncoordinated check reads its boundaries from this section |

Full checklists: `rfc/short/rfc7534.md`, `rfc/short/rfc7535.md`,
`rfc/short/rfc6996.md`.

## Why the checks live in `doctor`

Neither the as112 plugin nor the bgp component may own a check that reads the
other's config. Both checks therefore live in `internal/component/doctor`, which
reads the whole `config.Tree` generically and imports neither package. This is
the "dependency with no narrower owner" bucket in `ai/rules/repo-maintenance.md`,
and it follows the existing `checkConfigReferences` and `checkBGPMD5` pattern.

| Check code | Warns when |
|------------|-----------|
| `doctor-as112-watchdog-missing-withdraw` | a BGP `update` block announces an AS112 covering prefix with no `watchdog { withdraw true }` marker |
| `doctor-as112-global-origin-uncoordinated` | `asn.local 112` plus `replace-as` targets an eBGP session whose remote ASN is outside the RFC 6996 private-use ranges |

Codes are registered in `internal/core/diagnostic/codes.go`.
<!-- source: internal/core/diagnostic/codes.go -- diagnostic code registration -->

## Two traps this feature exposed

**A well-known community table maintained twice drifts.** The runtime
`update text` grammar carried 3 of about 15 registered names, so a route
configured with `community [ nopeer ]` failed to re-parse on watchdog replay and
was dropped in silence. `attribute.ParseCommunity` is now the single parser for
both the config-time path and the runtime replay path.
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- parseCommunityText -->

**An environment variable set for one subprocess leaks into the parent that
reads the same name.** Setting `ZE_CONFIG_DIR` on the daemon process so a probe
subprocess could inherit it also moved the daemon's own config storage. Bake the
variable into the probe command string instead.
