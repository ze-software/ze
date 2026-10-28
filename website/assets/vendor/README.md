# Vendored browser assets

Third-party files served from this site. Nothing here is fetched at build time
or at page load: the site is self-hosted, so a reader's browser talks to no CDN
and to no third-party host.

`internal/le/site.isSourceOnly` keeps this file out of the published
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
| Upstream `asciinema-player.min.js` sha256 | `a13c37632e1b5c49fe9128417b9319a9b5bc64cb457dd5ae52cbba8a3aceb880` |
| Patched `asciinema-player.min.js` sha256 | `a9a460ee15f1b76d821a71cf594aa6e20cbe8299caf99ef7d4c14243d9042cb4` |
| `asciinema-player.css` sha256 | `f619fe17597043564f03b2c6918b3daf890ee8b912fb408542fba11afade4fdb` |

The CSS is unmodified. The JavaScript is generated from the readable
`dist/bundle/asciinema-player.js` in that tarball with the local patch below.

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
- The recordings are produced by `internal/le/site/terminaldemo` from tapes in this
  repository, then checked against the digest in the artifact manifest before
  a page can embed one. The player is the only code that reads a cast.

### Recorded-poster state repair

Upstream 3.17.0 corresponds to revision
`8ba9c54008ed1f25d2914abe31ea118dbf7da2e4`. Its recording driver's
[`clearPoster`](https://github.com/asciinema/asciinema-player/blob/v3.17.0/src/driver/recording.js)
resets the terminal after a recorded-time poster but retains the recording's
event position. A forward seek or frame step then paints only incremental
changes onto an empty terminal. The
[`asciinema-player-poster.patch`](../../tools/asciinema-player-poster.patch)
changes that function in the readable distribution: only text posters reset
the terminal. Recorded-time posters retain their reconstructed terminal state.
Playback still seeks to its configured start. The patch also adds a
`Modified by Ze` notice beside the upstream license header; its `@license`
marker keeps the notice in the published minified file.

The patch applies to the verified readable bundle, whose SHA-256 is
`4d63cfb88859f158d1eef5d8a5550861abcb7ec195fb5d684f35f6446a29fe11`.
Its own SHA-256 is
`a404c35311683e9b77faa3ba621c39fba864e1ed1b70e14bac26993b1a1fc3ee`.
Generation uses Terser 5.42.0, the version pinned by upstream's
`v3.17.0` package lock, with upstream's `format.comments: /@license/`
option. It preserves the existing compiled WebAssembly and needs no Rust build.
Node v22.23.4 and npm 10.9.9 produced the patched bundle. Minifying the pristine
readable bundle with these options and a final newline reproduced upstream's
minified SHA-256 exactly. No build dependency is added to the website.

From the repository root, reproduce the artifact in session scratch:

```sh
repo="$PWD"
work="$(./le session scratch ensure)/asciinema-poster-build"
mkdir -p "$work"
cd "$work"
curl --fail --location https://registry.npmjs.org/asciinema-player/-/asciinema-player-3.17.0.tgz --output asciinema-player-3.17.0.tgz
printf '%s  %s\n' d03ef67fc2f32a7aa5177c0b64650c7c8fd234ef2d3e4b34886632bb3dd58c5c asciinema-player-3.17.0.tgz | sha256sum --check
tar -xzf asciinema-player-3.17.0.tgz
npm install --prefix . --ignore-scripts --no-audit --no-fund --save-exact terser@5.42.0
patch --directory package --strip 1 < "$repo/website/tools/asciinema-player-poster.patch"
node --input-type=module -e 'import fs from "node:fs"; import { minify } from "terser"; const result = await minify(fs.readFileSync("package/dist/bundle/asciinema-player.js", "utf8"), {format: {comments: /@license/}}); fs.writeFileSync(process.argv[1], result.code + "\n");' "$repo/website/assets/vendor/asciinema-player.min.js"
sha256sum "$repo/website/assets/vendor/asciinema-player.min.js"
```

Before publishing, exercise forward seek and frame step before Play, full
replay from zero, and the management chapter's configured start in a browser.

### Upgrading

Download the tarball and check its digest against the registry and its
provenance attestation. Check whether upstream has fixed `clearPoster`; retain
the local patch only while it is needed. Regenerate the JavaScript from the
readable bundle, copy its CSS here, update this table and generation instructions,
and re-read the checks above against the new bundle.
