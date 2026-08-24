# Vendored renderer fonts

These are the only fonts the renderer image holds. Its Dockerfile copies the
three directories below and then deletes every font a Debian package installed,
so the browser demo's typography is decided by bytes this repository carries.

Demo artifacts are committed and inspected before they are pushed (D-1 of
`spec-website-asciinema-terminal-demos`), so a renderer whose fonts
float with the distribution churns the poster on each re-render.

Three families are needed, and the third is the one that is easy to miss:

| Family | Who selects it |
|--------|----------------|
| Fira Code | `--font-mono` in `internal/component/web/assets/style.css` |
| Noto Sans | `--font-ui` in the same file |
| Liberation | nobody names it. The web interface's form controls set no family, so chromium draws them in its default face, Arial, and fontconfig's `30-metric-aliases.conf` resolves Arial to Liberation Sans |

Measured in both images with Chrome DevTools `CSS.getPlatformFontsForNode`: an
`<input>` is Liberation Sans at 185x21 px where Liberation is present and Noto
Sans at 206x24 px where it is not. That is the whole of the "form fields 2 to
6 px lower" the browser demo showed when the renderer left the VHS base, which
had Liberation installed. The text that names Fira Code and Noto Sans measured
identically in both.

## License

All three families are under the SIL Open Font License, Version 1.1. The license
text is beside the files it covers, as the OFL requires of a redistribution:

| Family | Copyright | License file |
|--------|-----------|--------------|
| Fira Code | 2014 The Fira Code Project Authors, Reserved Font Name "Fira Code" | `OFL-FiraCode.txt` |
| Liberation | 2010 Google Corporation and 2012 Red Hat, Inc., Reserved Font Names "Arimo", "Tinos", "Cousine" and "Liberation" | `OFL-Liberation.txt` |
| Noto Sans | 2022 The Noto Project Authors | `OFL-NotoSans.txt` |

The files are unmodified copies of the upstream releases. No family is
renamed, so no Reserved Font Name is used.

## Files

| Family | Version | Release |
|--------|---------|---------|
| Fira Code | 6.002 | `https://github.com/tonsky/FiraCode/releases/download/6.2/Fira_Code_v6.2.zip`, directory `ttf/` |
| Liberation | 2.1.5 | `https://github.com/liberationfonts/liberation-fonts/files/7261482/liberation-fonts-ttf-2.1.5.tar.gz` |
| Noto Sans | 2.015 | `https://github.com/notofonts/latin-greek-cyrillic/releases/download/NotoSans-v2.015/NotoSans-v2.015.zip`, directory `NotoSans/hinted/ttf/` |

| File | sha256 |
|------|--------|
| `fira-code/FiraCode-Bold.ttf` | `41f6554e845e2f5b70adad3950122334b866aac436793b7742ade600067701be` |
| `fira-code/FiraCode-Light.ttf` | `c146c9a7a61914f9f5a47d24c199c50c8f143f5710b93efd3a3953af50816443` |
| `fira-code/FiraCode-Medium.ttf` | `97091f90623661fb4f7979c10d188f30f4806d8ce326b0bc8d1acc79dcc20d8f` |
| `fira-code/FiraCode-Regular.ttf` | `5992ab9640e2df491b2f609467b1de60e8bc39b2c28db184342a0592d98f6117` |
| `fira-code/FiraCode-Retina.ttf` | `4fe2df1cea543281e8ec0fa512d1b493eacb859cf62bc7a84886daa89268b3f3` |
| `fira-code/FiraCode-SemiBold.ttf` | `500c74eec6249b06d49aef922dd3e8fc754c70c3b3f7791cd7b1a09ca9a26140` |
| `liberation/LiberationMono-Bold.ttf` | `bd62a0672d0b9b6710b01df434c80ad54fa5f0835207eb7b17b7a761463067bb` |
| `liberation/LiberationMono-BoldItalic.ttf` | `79451f3c09fe25116098853b7a2ca6e2436220ccc11af022979adbcf195be130` |
| `liberation/LiberationMono-Italic.ttf` | `605c01c711b44480a7508d349dfbf3264e81fa43d69e61cfa7d10b86e764c4d1` |
| `liberation/LiberationMono-Regular.ttf` | `f2b83c763e8afd21709333370bed4774337fae82267937e2b5aea7e2fbd922c1` |
| `liberation/LiberationSans-Bold.ttf` | `788abee4c806d660e8aee46689dd8540cd4bb98da03dcc9d171ce3efd99a9173` |
| `liberation/LiberationSans-BoldItalic.ttf` | `698da70fc191cc5f33ad4d6d3fe830fe4624b898ea2e3169955928b7c491f1ee` |
| `liberation/LiberationSans-Italic.ttf` | `e5bae5c4cde31f22142753855f4f8fb86da6ff39955ed3c0a11248b0d16948b0` |
| `liberation/LiberationSans-Regular.ttf` | `76d04c18ea243f426b7de1f3ad208e927008f961dc5945e5aad352d0dfde8ee8` |
| `liberation/LiberationSerif-Bold.ttf` | `d754ba427cfe0bca54ae052384baa8f842da5bd6550ad4da024ac441e7a7d5ce` |
| `liberation/LiberationSerif-BoldItalic.ttf` | `f17db8af71e24d2066b587546021d4f0b296be389512b658dec3c09affeb11a7` |
| `liberation/LiberationSerif-Italic.ttf` | `0e3dea9f8d613e006ccfa62201f33e265d19167bd0907725c3e145368b04fc2e` |
| `liberation/LiberationSerif-Regular.ttf` | `058ea80864aef09a23f45cbec2bb5400bc3dfbdea01c3f10538a21fcb497fb74` |
| `noto/NotoSans-Bold.ttf` | `1df075a380fc7cb898acf64c1f7b3b4dd780de3caa860178bf929de35817a913` |
| `noto/NotoSans-BoldItalic.ttf` | `1b602a9d6353be42c91df097a4857b69fa2696f26703d7a33b54a15d87c2622c` |
| `noto/NotoSans-Italic.ttf` | `467e3f89eeca4108bb8710a2b9e0cf2281ac56d5b0609211a83776d0505eecb5` |
| `noto/NotoSans-Regular.ttf` | `478c558ea716033cd60c03438f628dfa75694dcf6b5f6d505a2f05fd2b4f3823` |

The three directory names are the ones the earlier VHS-based renderer used:
`/usr/share/fonts/fira-code`, `/usr/share/fonts/liberation` and
`/usr/share/fonts/noto`. That image took the same builds from Alpine's
`font-fira-code`, `font-liberation` and `font-noto` packages, and all 22 files
above are byte-identical to the ones it carried.

Only the faces the earlier renderer carried are vendored. Noto Sans ships more
than 200 files upstream, of which the four static Latin faces are what the web
interface can select.

## Refresh

Download the release, take the directory named in the table, and check every
digest above changes together with the version:

```
curl -sSL -o firacode.zip https://github.com/tonsky/FiraCode/releases/download/6.2/Fira_Code_v6.2.zip
unzip -j firacode.zip 'ttf/*.ttf' -d fira-code
```

The Fira Code release carries no license file; take `LICENSE` from the same tag
at `https://raw.githubusercontent.com/tonsky/FiraCode/6.2/LICENSE`. The Noto
Sans release carries `OFL.txt` at its root, and the Liberation tarball carries
`LICENSE`.

A new release changes the browser demo's poster, so it also changes the
renderer image. Move the image tag in the same commit, in both places that pin
it: `TERMINAL_DEMO_IMAGE` in `mk/build-terminal-demo.mk` and `renderer.image`
in `demos/terminal/manifest.json`.
