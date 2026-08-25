"""What a subprogram is, and how `le` finds one.

Ze's own architecture is a small core plus registration: components and plugins
declare themselves and the core discovers them through a registry rather than
importing each one by name (`ai/rules/architecture.md`). This is that pattern
for the build tool, so adding an area means adding one module and one line,
never editing a dispatch table that grows a case per feature.

A subprogram is any module carrying the three names in `Subprogram` below. The
protocol is structural, so a module satisfies it by having them -- there is no
base class to inherit and nothing to register with by hand.
"""

from __future__ import annotations

import argparse
from collections.abc import Sequence
from dataclasses import dataclass
from importlib import import_module
from typing import Protocol, runtime_checkable

__all__ = ['REGISTRY', 'Entry', 'Subprogram', 'load']


@runtime_checkable
class Subprogram(Protocol):
    """The shape every module under `le.application` has.

    `add_arguments` declares the flags. Both routes into the subprogram call
    it, so `./le setup --check` and `python3 -m le.application.setup --check`
    parse the same flags with the same defaults and the same help text, and a
    flag added once is added to both.

    `options` turns the parsed namespace into a typed, frozen value. It is the
    one place argparse's `Any` becomes a real type, so everything past it is
    checked. Both routes call it.

    `action` is the work, and it takes only that typed value. Dispatch calls it
    directly, so nothing between the argv and the work is written twice.

    `main` is a standalone shell: parse, build options, call `action`. It holds
    no logic of its own and dispatch never calls it -- doing so would parse the
    same argv twice and give the two routes a chance to disagree.
    """

    def add_arguments(self, parser: argparse.ArgumentParser) -> None: ...

    def options(self, namespace: argparse.Namespace) -> object: ...

    def action(self, options: object) -> int: ...

    def main(self, argv: Sequence[str] | None = None) -> int: ...


@dataclass(frozen=True)
class Entry:
    """One subprogram, named for the command line and located by import path.

    The module is NOT imported when the registry is built. `le --help` lists
    every subprogram, and importing all of them to print a one-line summary
    would make the cheapest command pay for the most expensive area's imports.
    `load` does it when a command is actually chosen.
    """

    name: str
    module: str
    help: str

    def load(self) -> Subprogram:
        """Import this subprogram's module and return it."""
        return load(self.module)


def load(module: str) -> Subprogram:
    """Import `module` and check it is a subprogram before it is used.

    The check is here rather than at the call site so that a module missing
    `action` or `add_arguments` fails with a sentence naming what it lacks,
    instead of an AttributeError from inside the dispatcher.
    """
    loaded = import_module(module)
    required = ('add_arguments', 'options', 'action', 'main')
    missing = [name for name in required if not hasattr(loaded, name)]
    if missing:
        raise TypeError(f'{module} is not a subprogram: it has no {", ".join(missing)}')
    return loaded


# Every subprogram, in the order `le --help` lists them. One line per area,
# mirroring one mk/*.mk file, and the list is the only place an area is named.
REGISTRY: tuple[Entry, ...] = (
    Entry(
        name='setup',
        module='le.application.setup',
        help='install and verify every tool a Ze dev or test workflow needs',
    ),
    Entry(
        name='lint',
        module='le.application.lint',
        help='lint and type-check the Python half of the tree',
    ),
    Entry(
        name='check-cli',
        module='le.application.check_cli',
        help='CLI contract gates: command/handler, ownership, grammar, config claims',
    ),
    Entry(
        name='check-rules',
        module='le.application.check_rules',
        help='rules-system gates: the point corpus, its renders, the discovery indexes',
    ),
)


def find(name: str) -> Entry | None:
    """The registry entry called `name`, or None."""
    for entry in REGISTRY:
        if entry.name == name:
            return entry
    return None
