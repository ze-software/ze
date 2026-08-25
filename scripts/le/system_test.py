#!/usr/bin/env python3
"""Tests for machine state that is not a binary.

Ported from scripts/dev/dev_setup_test.py when `le setup` replaced that script.

The loopback addresses are the substantial half. A BGP session needs a
different address at each end -- RFC 4271 Section 5.1.3 forbids a peer its own
address as NEXT_HOP -- and IPv6 has one loopback address per host. Adding a
second one needs root, so it happens in setup rather than in the test runner.
"""

from __future__ import annotations

import io
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le.devtools import system

# A path that is not there, for the "this kernel has no such knob" branches.
ABSENT = Path('/nonexistent/le-test/not-a-real-node')


def _holding(text: str) -> Path:
    """A real file holding `text`.

    A real path rather than a mock: `userns_state` reads it, and a mock that
    answers `read_text` would pass with the `exists` guard deleted.
    """
    path = Path(tempfile.mkdtemp()) / 'knob'
    path.write_text(text)
    return path


class TestLoopbackAddresses(unittest.TestCase):
    def test_ipv6_is_unique_local_on_every_platform(self) -> None:
        """fd00::/8 is RFC 4193 unique-local: never globally routable, so a
        fixture that leaks a packet toward it cannot reach a real destination.

        A documentation prefix (2001:db8::/32) is globally scoped and would not
        carry that property.
        """
        assert system.LOOPBACK_IPV6.startswith('fd')
        for on_darwin in (True, False):
            with mock.patch.object(system, 'on_darwin', return_value=on_darwin):
                assert system.LOOPBACK_IPV6 in system.loopback_addresses()

    def test_ipv4_aliases_are_darwin_only(self) -> None:
        """Linux routes all of 127.0.0.0/8 to lo, so an alias there is work with
        no effect. macOS binds only 127.0.0.1 until each alias is added.
        """
        with mock.patch.object(system, 'on_darwin', return_value=True):
            addresses = system.loopback_addresses()
            assert '127.0.0.2' in addresses
            assert '127.0.0.5' in addresses
        with mock.patch.object(system, 'on_darwin', return_value=False):
            assert system.loopback_addresses() == [system.LOOPBACK_IPV6]

    def test_the_add_command_matches_the_platform(self) -> None:
        with mock.patch.object(system, 'on_darwin', return_value=True):
            assert system.loopback_add_argv('fd00::2') == [
                'ifconfig',
                'lo0',
                'inet6',
                'fd00::2/128',
                'alias',
            ]
            assert system.loopback_add_argv('127.0.0.2') == [
                'ifconfig',
                'lo0',
                'alias',
                '127.0.0.2',
            ]
        with mock.patch.object(system, 'on_darwin', return_value=False):
            # /128 keeps it a host address, so no route toward the rest of
            # fd00::/8 is created.
            assert system.loopback_add_argv('fd00::2') == [
                'ip',
                '-6',
                'addr',
                'add',
                'fd00::2/128',
                'dev',
                'lo',
            ]

    def test_the_probe_answers_on_a_bind(self) -> None:
        """A bind, not a scan of the interface list: an IPv6 address is listed
        while duplicate-address detection still refuses it, and a bind is what
        every fixture needs to succeed.

        ::1 is on every host; the documentation prefix (RFC 3849) is on none.
        """
        assert system.loopback_bindable('::1')
        assert not system.loopback_bindable('2001:db8::1')

    def test_only_missing_addresses_are_added(self) -> None:
        """Idempotence is structural: a configured host passes an empty list, so
        a re-run of setup runs no command at all.
        """
        with mock.patch.object(system, 'loopback_bindable', return_value=True):
            assert system.missing_loopback() == []

        with (
            mock.patch.object(system, 'run_privileged') as privileged,
            mock.patch.object(system, 'missing_loopback', return_value=[]),
            redirect_stdout(io.StringIO()),
        ):
            assert system.apply_loopback([])
            privileged.assert_not_called()

    def test_each_missing_address_reaches_run_privileged(self) -> None:
        with (
            mock.patch.object(system, 'on_darwin', return_value=False),
            mock.patch.object(system, 'run_privileged', return_value=(True, '')) as privileged,
            mock.patch.object(system, 'missing_loopback', return_value=[]),
            redirect_stdout(io.StringIO()),
        ):
            assert system.apply_loopback(['fd00::2'])
        assert privileged.call_args.args[0] == [
            'ip',
            '-6',
            'addr',
            'add',
            'fd00::2/128',
            'dev',
            'lo',
        ]

    def test_an_address_that_did_not_appear_is_a_failure(self) -> None:
        """The command can exit 0 and leave the address unusable. The bind probe
        after the fact is what decides, not the exit code.
        """
        with (
            mock.patch.object(system, 'run_privileged', return_value=(True, '')),
            mock.patch.object(system, 'missing_loopback', return_value=['fd00::2']),
            redirect_stdout(io.StringIO()),
        ):
            assert not system.apply_loopback(['fd00::2'])

    def test_no_route_to_root_says_what_to_run(self) -> None:
        buffer = io.StringIO()
        with (
            mock.patch.object(system, 'on_darwin', return_value=True),
            mock.patch.object(system, 'run_privileged', return_value=(False, 'no route')),
            redirect_stdout(buffer),
        ):
            assert not system.apply_loopback(['fd00::2'])
            system.print_loopback_fix(['fd00::2'])
        assert 'sudo ifconfig lo0 inet6 fd00::2/128 alias' in buffer.getvalue()


class TestCheckModeTouchesNothing(unittest.TestCase):
    """`--check` probes. It must not change the machine, ever.

    The old suite proved this for the loopback path by patching
    `run_privileged` and asserting zero calls. The port lost it: the only
    remaining check greps stdout for install strings, which proves what was
    PRINTED and not what was RUN. A probe that silently added an address or
    edited a sysctl would pass that.

    `run_privileged` is the one door to root for all three system fixes
    (`le/devtools/system.py`), so asserting it is never called covers every
    one of them at once.
    """

    def _visit_under_check(self, visit: str) -> None:
        from le.application import setup

        opts = setup.Options(check=True)
        with (
            mock.patch.object(system, 'run_privileged') as privileged,
            mock.patch.object(system, 'missing_loopback', return_value=['fd00::2']),
            mock.patch.object(system, 'userns_state', return_value=system.Userns.RESTRICTED),
            mock.patch.object(system, 'kvm_state', return_value=system.Kvm.NO_GROUP),
            redirect_stdout(io.StringIO()),
        ):
            getattr(setup, visit)(opts)
        privileged.assert_not_called()

    def test_check_mode_adds_no_loopback_address(self) -> None:
        self._visit_under_check('_visit_loopback')

    def test_check_mode_writes_no_sysctl(self) -> None:
        self._visit_under_check('_visit_userns')

    def test_check_mode_changes_no_group(self) -> None:
        self._visit_under_check('_visit_kvm')

    def test_each_one_still_reports_the_problem_it_found(self) -> None:
        """Touching nothing must not become saying nothing."""
        from le.application import setup
        from le.console import State

        opts = setup.Options(check=True)
        with (
            mock.patch.object(system, 'missing_loopback', return_value=['fd00::2']),
            mock.patch.object(system, 'userns_state', return_value=system.Userns.RESTRICTED),
            mock.patch.object(system, 'kvm_state', return_value=system.Kvm.NO_GROUP),
            redirect_stdout(io.StringIO()),
        ):
            for visit in ('_visit_loopback', '_visit_userns', '_visit_kvm'):
                outcome = getattr(setup, visit)(opts)
                assert outcome.state is State.MISSING, visit
                assert outcome.state.blocking, visit


class TestUserns(unittest.TestCase):
    """Ubuntu 23.10+ blocks the user-namespace sandbox Chrome relies on.

    The agent-browser web functional tests then cannot launch Chrome ("No
    usable sandbox!").
    """

    def test_absent_knob_is_not_applicable(self) -> None:
        """A non-AppArmor host has nothing to fix, which is not a failure."""
        with mock.patch.object(system, 'USERNS_PROC', ABSENT):
            assert system.userns_state() is system.Userns.NA

    def test_a_set_knob_is_restricted(self) -> None:
        with mock.patch.object(system, 'USERNS_PROC', _holding('1\n')):
            assert system.userns_state() is system.Userns.RESTRICTED

    def test_a_cleared_knob_is_ok(self) -> None:
        with mock.patch.object(system, 'USERNS_PROC', _holding('0\n')):
            assert system.userns_state() is system.Userns.OK

    def test_the_fix_writes_a_drop_in_and_applies_it_live(self) -> None:
        """One without the other leaves the machine right now or right after
        the next boot, not both.
        """
        commands = system.userns_commands()
        assert commands[0][0] == ['tee', system.USERNS_CONF]
        assert commands[0][1] == b'kernel.apparmor_restrict_unprivileged_userns = 0\n'
        assert commands[1][0] == ['sysctl', '-w', f'{system.USERNS_SYSCTL}=0']

    def test_the_shown_line_carries_a_sudo_placeholder(self) -> None:
        """The argv is a `tee` with no content, which is not what a human types."""
        shown = system.userns_commands()[0][2]
        assert shown is not None
        assert '{sudo}' in shown
        assert 'echo' in shown

    def test_a_failed_step_stops_and_reports(self) -> None:
        buffer = io.StringIO()
        with (
            mock.patch.object(system, 'run_privileged', return_value=(False, 'denied')) as ran,
            redirect_stdout(buffer),
        ):
            assert not system.apply_userns()
        assert len(ran.call_args_list) == 1
        assert 'denied' in buffer.getvalue()


class TestKvm(unittest.TestCase):
    """QEMU does not fall back when /dev/kvm is unreadable.

    It dies with "Could not access KVM kernel module: Permission denied" and
    the caller reports a timeout.
    """

    def test_no_device_is_not_applicable(self) -> None:
        """No hardware virt, or a VM without nested virt. QEMU uses tcg."""
        with mock.patch.object(system, 'KVM_DEV', ABSENT):
            assert system.kvm_state() is system.Kvm.NA

    def test_in_the_group_but_not_yet_usable_is_pending_login(self) -> None:
        """The distinction that decides the message.

        After `usermod -aG` the group database says yes while every
        already-running process still says no. Telling those two apart is the
        difference between "run this command" and "log back in".
        """
        with (
            mock.patch.object(system, 'KVM_DEV', _holding('')),
            mock.patch('le.devtools.system.os.access', return_value=False),
            mock.patch.object(system, 'in_kvm_group', return_value=True),
        ):
            assert system.kvm_state() is system.Kvm.PENDING_LOGIN

    def test_not_in_the_group_is_no_group(self) -> None:
        with (
            mock.patch.object(system, 'KVM_DEV', _holding('')),
            mock.patch('le.devtools.system.os.access', return_value=False),
            mock.patch.object(system, 'in_kvm_group', return_value=False),
        ):
            assert system.kvm_state() is system.Kvm.NO_GROUP

    def test_a_missing_group_in_the_database_is_not_an_error(self) -> None:
        """A host with no kvm group at all reaches this."""
        with mock.patch('le.devtools.system.grp.getgrnam', side_effect=KeyError('kvm')):
            assert not system.in_kvm_group()


if __name__ == '__main__':
    unittest.main()
