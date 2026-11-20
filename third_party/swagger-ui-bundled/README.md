# Licences of the libraries bundled into swagger-ui

This directory holds the licence texts of the libraries compiled into the
swagger-ui bundle (`swagger-ui-bundle.js`) and stylesheet (`swagger-ui.css`)
that the REST API explorer serves from `third_party/web/swagger-ui/`. Each
`<package>.LICENSE` is the package's own licence file, read from
`https://unpkg.com/<package>@<version>/`, byte for byte.

Nothing here is served. The files sit outside `third_party/web/` because every
file there is synced to the asset consumers and served beside the assets.
`./le deps credits update` copies these texts into the credits the `ze`
binary embeds, and `ze thanks licence` prints them.

The versions are taken from swagger-ui's `package-lock.json` at tag v5.32.2,
the development tree's lockfile. Where a banner in
`swagger-ui-bundle.js.LICENSE.txt` names a version, it agrees. The lockfile
has not been checked byte for byte against the bundle. Each package's version
is declared once, in its `works` entry of `website/data/dependencies.json`.
