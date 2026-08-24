"""The default-on feature tags, DERIVED from feature-gates.txt.

`feature-gates.txt` is the single source of truth for compile-out-able
features, and the Makefile reads it the same way:

    Makefile:110  ZE_FEATURES := $(shell awk '$$1 ~ /^ze_/ {print $$1}' \
                                    $(CURDIR)/feature-gates.txt | sort -u)

A per-script literal is a second record of one fact, and the one nothing
compares drifts. That is not a hypothetical here; it has happened twice:

  - `ze_bgp` became a gate, and `effective-vpp.py` went on building a ze with
    no BGP. Its fib case died on "unknown top-level keyword: bgp" and no VPP
    backend was linked. That script grew a local derivation and the other nine
    kept their literals.
  - `ze_l2tp` became a gate in b7bfbac23 on 2026-07-24 and no evidence script
    was updated. `effective-l2tp-ppp.py` builds the daemon
    `ai/rules/platform-linux.md` names as the proof that L2TP works against
    real kernel modules, so that proof was silently unavailable for a month:
    the daemon has no L2TP and dies on "unknown top-level keyword: l2tp"
    before it reaches a kernel feature.

So the derivation lives HERE, once, and every evidence script that builds a
daemon imports it. `scripts/evidence/homebrew.py` and `alpine_iso.py` are the
same pattern: a sibling module, imported by name.

NOT every build wants these. A host driver -- the `ze-host` that runs
`ze appliance ...` on the build machine -- is built `ze_core ze_setup` with no
feature tags, which is what `mk/build-gokrazy.mk:282` does. Feature gates
select daemon features; they have nothing to say about the setup surface.
"""

from __future__ import annotations

from pathlib import Path

FEATURE_GATES = "feature-gates.txt"


def feature_tags(root: Path) -> list[str]:
    """Every `ze_*` gate named in feature-gates.txt, sorted and deduplicated.

    Mirrors the Makefile's awk exactly: a line's FIRST field, when it starts
    with `ze_`. Comment lines start with `#`, so their first field never does.
    """
    tags: set[str] = set()
    text = (root / FEATURE_GATES).read_text(encoding="utf-8")
    for line in text.splitlines():
        fields = line.split()
        if fields and fields[0].startswith("ze_"):
            tags.add(fields[0])
    return sorted(tags)


def daemon_build_tags(root: Path, base: str = "ze_core ze_distro") -> str:
    """The `-tags` string for a daemon build: the base plus every feature gate.

    Space-separated, which is what the Makefile passes and what `go build`
    documents. Commas work too, but one spelling across the tree is one fewer
    thing to check when a build comes out missing a feature.
    """
    return " ".join([base, *feature_tags(root)])
