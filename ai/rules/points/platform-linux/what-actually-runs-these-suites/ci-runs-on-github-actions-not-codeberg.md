---
kind: note
level:
stage:
---
Validation runs on **GitHub Actions** (`.github/workflows/`), not Codeberg. The
repo is pushed to both codeberg.org and github.com/ze-software/ze; CI moved to
GitHub because running heavy nightly sweeps on Codeberg's donated shared runners
is inconsiderate of a free service, and because GitHub's `ubuntu-latest` grants
the root / `CAP_NET_ADMIN` the integration suite needs, which the shared
Woodpecker instance could not.
