#!/usr/bin/env python3
"""Tests for subprogram registration and routing.

The property worth pinning is the one the whole layout exists for: dispatch and
a standalone run reach the SAME function with the SAME options. Everything else
here guards the registry against a subprogram that does not keep its side of
the contract.
"""

from __future__ import annotations

import argparse
import io
import sys
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path
from types import ModuleType
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le import main as dispatch
from le import registry
from le.application import lint, setup


def _standalone_setup(argv: list[str]) -> setup.Options:
    """The options a standalone `python3 -m le.application.setup` would build."""
    parser = argparse.ArgumentParser()
    setup.add_arguments(parser)
    return setup.options(parser.parse_args(argv))


def _standalone_lint(argv: list[str]) -> lint.Options:
    """The options a standalone `python3 -m le.application.lint` would build."""
    parser = argparse.ArgumentParser()
    lint.add_arguments(parser)
    return lint.options(parser.parse_args(argv))


def _dispatched(name: str, argv: list[str]) -> object:
    """The options `./le <name> ...` would build, by the dispatcher's own route."""
    parsed = dispatch.build_parser().parse_args([name, *argv])
    entry: registry.Entry = parsed.entry
    return entry.load().options(parsed)


class TestRegistry(unittest.TestCase):
    def test_every_entry_loads_and_is_a_subprogram(self) -> None:
        """A registered name that does not import is a broken `le --help`."""
        for entry in registry.REGISTRY:
            module = entry.load()
            for attribute in ('add_arguments', 'options', 'action', 'main'):
                assert hasattr(module, attribute), f'{entry.name} has no {attribute}'

    def test_names_are_unique(self) -> None:
        names = [entry.name for entry in registry.REGISTRY]
        assert len(names) == len(set(names))

    def test_a_module_missing_a_name_is_refused_by_name(self) -> None:
        """The error must say what is missing, not raise AttributeError later."""
        incomplete = ModuleType('incomplete')
        # setattr, not attribute assignment: a module object's attributes are
        # not known to a type checker, and naming one directly is an error
        # rather than a cast.
        setattr(incomplete, 'add_arguments', lambda parser: None)  # noqa: B010
        with (
            mock.patch.object(registry, 'import_module', return_value=incomplete),
            self.assertRaises(TypeError) as caught,
        ):
            registry.load('incomplete')
        message = str(caught.exception)
        assert 'options' in message
        assert 'action' in message
        assert 'main' in message

    def test_find_returns_none_for_an_unknown_name(self) -> None:
        assert registry.find('no-such-subprogram') is None
        assert registry.find('setup') is not None


class TestBothRoutesAgree(unittest.TestCase):
    """`./le setup --check` and `python3 -m le.application.setup --check` must
    build the same options.

    They share `add_arguments` and `options`, so this holds by construction.
    The test exists so that a future subprogram which parses its own argv in
    `main` is caught: that is the shape which lets the two routes drift.
    """

    def test_setup_flags_match(self) -> None:
        for argv in ([], ['--check'], ['--no-vendor'], ['--check', '--no-vendor']):
            assert _standalone_setup(argv) == _dispatched('setup', argv), argv

    def test_lint_flags_match(self) -> None:
        for argv in ([], ['--fix'], ['--types-only'], ['--lint-only'], ['--strict-only']):
            assert _standalone_lint(argv) == _dispatched('lint', argv), argv


class TestOptions(unittest.TestCase):
    def test_setup_defaults_to_installing_and_vendoring(self) -> None:
        assert _standalone_setup([]) == setup.Options(check=False, vendor=True)

    def test_setup_check_changes_nothing(self) -> None:
        assert _standalone_setup(['--check']).check is True

    def test_no_vendor_skips_the_go_mod_step(self) -> None:
        assert _standalone_setup(['--no-vendor']).vendor is False

    def test_lint_types_only_and_lint_only_are_exclusive(self) -> None:
        parser = argparse.ArgumentParser()
        lint.add_arguments(parser)
        # argparse writes its usage to stderr before exiting; capture both so
        # the failure this asserts does not look like one in the test output.
        with (
            redirect_stdout(io.StringIO()),
            redirect_stderr(io.StringIO()),
            self.assertRaises(SystemExit),
        ):
            parser.parse_args(['--types-only', '--lint-only'])


class TestDispatch(unittest.TestCase):
    def test_no_subprogram_prints_help_and_fails(self) -> None:
        buffer = io.StringIO()
        with redirect_stdout(buffer):
            code = dispatch.main([])
        assert code == 1
        assert 'subprogram' in buffer.getvalue()

    def test_dispatch_calls_action_not_main(self) -> None:
        """Dispatch must not re-enter the subprogram's argv parsing.

        Going through `main` would parse the same argv twice and give the two
        routes a chance to disagree, which is the whole reason `action` takes a
        typed value instead of a namespace.
        """
        with (
            mock.patch.object(setup, 'action', return_value=0) as action,
            mock.patch.object(setup, 'main') as standalone,
        ):
            assert dispatch.main(['setup', '--check']) == 0
        action.assert_called_once()
        standalone.assert_not_called()
        assert action.call_args.args[0] == setup.Options(check=True, vendor=True)


class TestLegacyCeiling(unittest.TestCase):
    def test_the_ceiling_is_read_from_pyproject(self) -> None:
        """The number lives beside the rules it counts, not in the code."""
        assert lint.legacy_ceiling() >= 0


if __name__ == '__main__':
    unittest.main()
