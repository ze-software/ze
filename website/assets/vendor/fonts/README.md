# Vendored web fonts

The site serves Poppins and Lato from this directory. Nothing is fetched from
`fonts.googleapis.com` or `fonts.gstatic.com` (D-2 of
`spec-website-asciinema-terminal-demos`).

## License

Both families are under the SIL Open Font License, Version 1.1. The license
text is beside the files it covers, as the OFL requires of a redistribution:

| Family | Copyright | License file |
|--------|-----------|--------------|
| Lato | 2010-2014 tyPoland Lukasz Dziedzic, Reserved Font Name "Lato" | `OFL-Lato.txt` |
| Poppins | 2020 The Poppins Project Authors | `OFL-Poppins.txt` |

The OFL permits redistribution with or without modification. These files are
unmodified copies of the subsets Google Fonts serves. Neither family is
renamed, so no Reserved Font Name is used.

## Files

| File | Holds |
|------|-------|
| `fonts.css` | The faces every page uses: Poppins 400, 700, 800 and Lato 400, 700 |
| `poppins-600.css` | Poppins 600, linked by the talk decks under `talks/` alone |
| `<family>-<weight>-<subset>.woff2` | One face, one weight, one unicode range |

Two subsets are vendored per weight, `latin` and `latin-ext`, with the
`unicode-range` Google publishes for each. A browser downloads a subset only
when the page holds a character in its range. The `devanagari` subset is not
vendored: it is 118 KB of the 216 KB Google serves for these weights, and no
page in this site holds a Devanagari character.

## Digests

Google Fonts publishes no digest to check a download against, so these record
which bytes this site serves rather than attesting to an upstream release. They
are what a refresh compares against: a file whose digest is unchanged is the
same face, and one that moved is a new release to review.

| File | sha256 |
|------|--------|
| `lato-400-latin-ext.woff2` | `8b9fc9737043f88c1a9a7195c27a239bd329cc33d928ffb67736c61ae7a1dbbd` |
| `lato-400-latin.woff2` | `918b7dc3e2e2d015c16ce08b57bcb64d2253bafc1707658f361e72865498e537` |
| `lato-700-latin-ext.woff2` | `3a5797f440bc67b59830cad81e59a71011e35cb45ed7f747b61a69b0a9b0d6ae` |
| `lato-700-latin.woff2` | `c447dd7677b419db7b21dbdfc6277c7816a913ffda76fd2e52702df538de0e49` |
| `poppins-400-latin-ext.woff2` | `0b1fcab42c18b69bcfe9ce4799fcbff5af1621c53ffcfdc4723c6f5ec4ee3ffb` |
| `poppins-400-latin.woff2` | `7d93459d86585bfcdbb7e0376056226adb25821ee54b96236fe2123e9560929f` |
| `poppins-600-latin-ext.woff2` | `bb1f2d582e7fba586ab70c91ef062d3becaf78b887654953863521b73665d171` |
| `poppins-600-latin.woff2` | `f4e80d9dfd374d02989b87a27b5ed4cb78fbb177c27f1478e9a8b0afb7513149` |
| `poppins-700-latin-ext.woff2` | `ccfd87f69ef00d811da3d06488cec4e79ec99d289cfbcbe4be42031cecae775a` |
| `poppins-700-latin.woff2` | `9338e65fc077355c7a87ae0d64cc101e23b9bf8ad78ae65f0f319c857311b526` |
| `poppins-800-latin-ext.woff2` | `a72eccfa6cfa9c26b4004e98dbf8592ea0fb3704b2fc749e8bd60a996e2e5a7c` |
| `poppins-800-latin.woff2` | `60bf0aba6526436f3930c58c12047687fbb6bff4dd180cce4613458ed3439ea2` |

## Refresh

The `.woff2` files came from the URLs in the stylesheet Google Fonts returns
for the site's own font request. To take a newer release:

```
curl -A "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36" \
  "https://fonts.googleapis.com/css2?family=Poppins:wght@400;700;800&family=Lato:wght@400;700&display=swap"
```

The user agent decides the format: this one gets `woff2`. Each `@font-face` in
the answer carries the `src` URL to download and the `unicode-range` to copy.
Poppins is at `v24` and Lato at `v25` in the URLs these files came from
(2026-08-24). Take `OFL.txt` for each family from
`https://github.com/google/fonts/tree/main/ofl/<family>`.

`internal/le/site.TestVendoredFontsAreSelfHosted` checks that every
`url()` in these stylesheets names a local file and every face keeps
`font-display: swap`.
