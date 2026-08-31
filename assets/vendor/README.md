# Vendored browser assets

Third-party files served from this site. Nothing here is fetched at build time
or at page load: the site is self-hosted, so a reader's browser talks to no CDN
and to no third-party host.

`internal/le/site.IsSourceOnly` keeps this file out of the published
artifact. Every other file in this directory is staged verbatim because
`assets/vendor` is a public asset directory.

## asciinema-player

| Field | Value |
|-------|-------|
| Version | 3.17.0 |
| License | Apache-2.0 (`asciinema-player.LICENSE`) |
| Upstream | `https://registry.npmjs.org/asciinema-player/-/asciinema-player-3.17.0.tgz` |
| Tarball sha512 | `25b8cd2660364cb21e60d68e1236be9154b35da74aa2aa48cd55667c118d8f60e676d13fbc4c410a7904f0d604729690703bd4620f0bb6dd2ead3f347eb0029b` |
| Tarball sha256 | `d03ef67fc2f32a7aa5177c0b64650c7c8fd234ef2d3e4b34886632bb3dd58c5c` |
| `asciinema-player.min.js` sha256 | `a13c37632e1b5c49fe9128417b9319a9b5bc64cb457dd5ae52cbba8a3aceb880` |
| `asciinema-player.css` sha256 | `f619fe17597043564f03b2c6918b3daf890ee8b912fb408542fba11afade4fdb` |

Both files come from `dist/bundle/` of that tarball, unmodified.

### What was checked before the files were committed

- The downloaded tarball's sha512 equals the digest the npm registry publishes
  for 3.17.0, and its sha1 equals the registry's `shasum`.
- npm serves a SLSA v1 provenance attestation for the same sha512. It names the
  build as a GitHub Actions workflow, `.github/workflows/release.yml`, run from <!-- doc-links: ignore (a path in the asciinema-player repository, not in this one) -->
  `https://github.com/asciinema/asciinema-player` at `refs/tags/v3.17.0`.
- The minified bundle contains no external host. The only absolute URLs are the
  SVG namespace and `http://localhost/`, which is the base a relative recording
  URL resolves against when `location` is absent.
- The stylesheet contains no `url()` and no `@import`, so it downloads no font
  and no image.
- The bundle carries its terminal emulator as a WebAssembly module embedded as
  base64 in the script itself, decoded in the page. Nothing is fetched for it.
- `fetch` is called on the recording URL the page supplies, and on a URL only
  the `benchmark` driver names. `WebSocket` is used only by the `websocket`
  driver. The site passes a plain `.cast` path, so neither driver is selected.
- `innerHTML` is written twice, once with a template string from the bundle and
  once with a fixed SVG path chosen by a `switch`. Neither takes recording data.
- The recordings are produced by `internal/le/terminaldemo` from tapes in this
  repository, then checked against the digest in the artifact manifest before
  a page can embed one. The player is the only code that reads a cast.

### Upgrading

Download the tarball, check its digest against the registry and its provenance
attestation, copy `dist/bundle/asciinema-player.min.js` and
`dist/bundle/asciinema-player.css` here, update this table, and re-read the list
above against the new bundle.
