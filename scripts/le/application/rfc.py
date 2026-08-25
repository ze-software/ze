"""RFC conformance gates: the coverage ratchets, the requirement ledger, the audit seals.

Ported from the root Makefile.

    ./le rfc                                  every check in the area
    ./le rfc --list                           what each one is for
    ./le rfc ze-rfc-check                     one of them
    ./le rfc --write                          the two generators
    ./le rfc ze-rfc-extraction-status --json

**`ze-rfc-check` was two commands and is two gates here.** The recipe ran the
gate's own fixtures first and the live tree second, and a `Gate` holds one
command. The order is the point rather than the packaging: a coverage gate that
has not proven itself against its fixtures is not evidence about the tree it
judges. Declaration order keeps the pair in that order for a bare `./le rfc`,
and the Make target names both gates on two lines, the way
`ze-discovery-index-update` does in mk/check-rules.mk.

**`ze-rfc-extraction-status` carries `--json` as a JSON flag rather than in its
argv, and the Make target passes it.** The gate is always JSON -- that envelope
is its only consumer, and mk/schedule-cadence.mk parses what it prints. The
flag route is the one that suppresses the `==>` banner and the run summary, so
stdout stays a single JSON document.

One target stayed in the root Makefile: `ze-rfc-extraction-create` takes
`STEM=<rfc-stem>` and guards on it with a shell conditional, and neither the
Make variable nor the guard survives a fixed argv.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence

from le import gateapp
from le.devtools.gate import Gate, GateSet

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']


def _rfc(*args: str) -> tuple[str, ...]:
    return ('python3', 'scripts/dev/rfc_requirements.py', *args)


GATES = GateSet(
    area='rfc',
    gates=(
        Gate(
            name='ze-rfc-selftest',
            argv=_rfc('--selftest'),
            why='the gate proves itself against its own fixtures before it judges the live tree',
        ),
        Gate(
            name='ze-rfc-check',
            argv=_rfc('--check'),
            why=(
                'every MUST-level requirement of an ENROLLED RFC (rfc/enrolled.txt) is bound'
                ' to a positive AND a negative test by an `RFC requirement: <ID> <polarity>`'
                ' tag, or carries a reasoned annotation. Requirement text lives in'
                ' rfc/short/*.md; the test links are derived from the tags, never'
                ' hand-written (ai/rules/evidence.md)'
            ),
        ),
        Gate(
            name='ze-rfc-extraction-status',
            argv=_rfc('--extraction-status'),
            json_flag='--json',
            why=(
                "the machine-readable extraction counts the umbrella's drain quota consumes"
                ' (plan/spec-rfcgate-0-umbrella.md, "Where the counter lives"): signed and'
                ' enrolled counts, the per-register split, and the unsigned backlog. Always'
                " JSON -- that envelope is the mode's only consumer"
            ),
        ),
        Gate(
            name='ze-rfc-index-update',
            argv=_rfc('--write'),
            why=(
                'regenerate the RFC requirement ledger (requirement -> enforcing tests): the'
                ' index ai/RFC-REQUIREMENTS.md, and one file per RFC stem under'
                " rfc/requirements/ holding that RFC's rows. It also deletes a shard whose"
                ' stem no longer renders'
            ),
            writes=True,
        ),
        Gate(
            name='ze-rfc-reseal',
            argv=_rfc('--reseal'),
            why=(
                'rewrite the file-level fingerprints of the audit verdicts a mechanical edit'
                ' staled (plan/spec-rfcgate-3-audit-teeth.md): the tagged unit is'
                ' byte-identical and only the file around it moved, so nothing was re-judged'
                ' and no human should be asked to re-read. Deliberately its OWN gate (owner'
                ' ruling 2026-07-29) -- folding it into ze-rfc-check, a check that writes'
                ' cannot be trusted to report, or into ze-rfc-index-update, which runs'
                ' routinely for reasons unrelated to any audit, would automate the blind'
                ' re-stamp reflex the spec exists to remove. A verdict whose unit, cited'
                ' producer code, or requirement text MOVED is refused and stays stale: that'
                ' one needs /ze-rfc-audit <rfc>, then ze-rfc-index-update'
            ),
            writes=True,
        ),
    ),
)


def add_arguments(parser: argparse.ArgumentParser) -> None:
    gateapp.add_arguments(parser, GATES)


def options(namespace: argparse.Namespace) -> gateapp.Options:
    return gateapp.options(namespace)


def action(opts: gateapp.Options) -> int:
    return gateapp.action(opts, GATES)


def main(argv: Sequence[str] | None = None) -> int:
    return gateapp.main(argv, GATES, __doc__, run=action)


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
