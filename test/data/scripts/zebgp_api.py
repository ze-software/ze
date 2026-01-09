#!/usr/bin/env python3
"""Shared library for ZeBGP API test scripts.

Provides buffered I/O for reliable communication with ZeBGP daemon.
Handles both text and JSON API response formats.

5-stage plugin registration protocol:
  - Stage 1: Declaration (declare rfc, encoding, family, conf, cmd)
  - Stage 2: Config Delivery (receive config lines)
  - Stage 3: Capability (capability bytes for OPEN)
  - Stage 4: Registry Sharing (receive registry commands)
  - Stage 5: Ready (ready signal)

Simple usage:
    from zebgp_api import ready, send, wait_for_shutdown

    ready()
    send('update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24')
    wait_for_shutdown()

Full protocol usage:
    from zebgp_api import API

    api = API()
    # Stage 1: Declare capabilities
    api.declare_rfc(4271)
    api.declare_encoding('text')
    api.declare_family('ipv4', 'unicast')
    api.declare_done()

    # Stage 2: Receive config
    config = api.wait_for_config()

    # Stage 3: Declare capabilities
    api.capability_done()

    # Stage 4: Receive registry
    registry = api.wait_for_registry()

    # Stage 5: Signal ready
    api.ready()

    # Normal operation
    api.send('update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24')
"""

from __future__ import annotations

import json
import os
import select
import signal
import sys
import time
from typing import Any


class API:
    """ExaBGP API client with buffered I/O.

    Uses os.read() with internal buffering to properly handle responses
    that may arrive in chunks or be split across multiple reads.
    """

    def __init__(self, stdin: int | None = None, stdout: int | None = None):
        """Initialize API client.

        Args:
            stdin: File descriptor to read from (default: sys.stdin)
            stdout: File object to write to (default: sys.stdout)
        """
        self._stdin_fd = stdin if stdin is not None else sys.stdin.fileno()
        self._stdout = stdout if stdout is not None else sys.stdout
        self._buffer = ''
        self._plugin_name = ''  # Set during registry sharing

        # Install SIGPIPE handler
        signal.signal(signal.SIGPIPE, signal.SIG_DFL)

    def flush(self, msg: str) -> None:
        """Write message to stdout and flush.

        Args:
            msg: Message to send (should include newline if needed)
        """
        self._stdout.write(msg)
        self._stdout.flush()

    def send(self, command: str) -> None:
        """Send a command to ExaBGP.

        Args:
            command: Command string (newline added automatically)
        """
        self.flush(f'{command}\n')

    def read_line(self, timeout: float = 0.1) -> str | None:
        """Read a complete line from stdin using buffered I/O.

        Reads data into internal buffer and returns complete lines.
        Handles data that arrives in chunks across multiple reads.

        Args:
            timeout: Seconds to wait for data (default: 0.1)

        Returns:
            Complete line (without newline) or None if no complete line available
        """
        # Check if we already have a complete line in buffer
        if '\n' in self._buffer:
            line, self._buffer = self._buffer.split('\n', 1)
            return line

        # Read more data if available
        try:
            ready, _, _ = select.select([self._stdin_fd], [], [], timeout)
            if ready:
                chunk = os.read(self._stdin_fd, 4096).decode('utf-8', errors='replace')
                if chunk:
                    self._buffer += chunk
        except (OSError, IOError):
            return None

        # Check again for complete line
        if '\n' in self._buffer:
            line, self._buffer = self._buffer.split('\n', 1)
            return line

        return None

    def parse_answer(self, line: str) -> str | None:
        """Parse answer type from response line.

        Handles both text and JSON formats:
        - Text: "done", "error", "shutdown"
        - JSON: {"answer": "done|error|shutdown", ...}

        Args:
            line: Response line to parse

        Returns:
            Answer type ('done', 'error', 'shutdown') or None if not an answer
        """
        if not line:
            return None

        if line.startswith('{'):
            # JSON format
            try:
                data = json.loads(line)
                return data.get('answer')
            except (json.JSONDecodeError, TypeError):
                return None
        else:
            # Text format - check if it's a known answer
            if line in ('done', 'error', 'shutdown'):
                return line
            return None

    def wait_for_ack(self, expected_count: int = 1, timeout: float = 2.0) -> bool:
        """Wait for ACK responses from ExaBGP.

        Polls stdin until all expected ACK messages are received.
        Uses buffered I/O to handle responses arriving in chunks.

        Args:
            expected_count: Number of ACK messages expected (default: 1)
            timeout: Total timeout in seconds (default: 2.0)

        Returns:
            True if all ACKs received successfully
            False if any command failed or timeout occurred

        Raises:
            SystemExit: If ExaBGP sends shutdown message
        """
        received = 0
        start_time = time.time()

        while received < expected_count:
            # Check timeout
            elapsed = time.time() - start_time
            if elapsed >= timeout:
                return False

            # Read a line (uses internal buffer)
            line = self.read_line(0.1)
            if line is None:
                continue

            # Parse the answer
            answer = self.parse_answer(line)
            if answer == 'done':
                received += 1
            elif answer == 'error':
                return False
            elif answer == 'shutdown':
                raise SystemExit(0)
            # Ignore other messages (could be BGP updates, data responses, etc.)

        return True

    def read_response(self, timeout: float = 2.0) -> dict | str | None:
        """Read and parse a complete response from ExaBGP.

        Collects lines until we get an 'answer' terminator (done/error/shutdown).
        Returns accumulated data along with the answer.

        Args:
            timeout: Maximum time to wait for response

        Returns:
            dict: {'data': [...], 'answer': 'done|error'} if data received
            dict: {'answer': 'done|error'} if only terminator received
            str: Raw text if non-JSON response
            None: Timeout with no data received

        Raises:
            SystemExit: If ExaBGP sends shutdown message
        """
        start_time = time.time()
        responses: list[Any] = []

        while True:
            # Check timeout
            elapsed = time.time() - start_time
            if elapsed >= timeout:
                break

            # Read a line
            line = self.read_line(0.1)
            if line is None:
                continue

            # Try to parse as JSON
            try:
                data = json.loads(line)
                # Check for terminator
                if isinstance(data, dict):
                    answer = data.get('answer')
                    if answer in ('done', 'error', 'shutdown'):
                        if answer == 'shutdown':
                            raise SystemExit(0)
                        if responses:
                            return {'data': responses, 'answer': answer}
                        return data
                # Accumulate non-terminator responses
                responses.append(data)
            except json.JSONDecodeError:
                # Not JSON - check for text terminators
                if line in ('done', 'error', 'shutdown'):
                    if line == 'shutdown':
                        raise SystemExit(0)
                    if responses:
                        return {'data': responses, 'answer': line}
                    return {'answer': line}
                # Return raw text
                return line

        # Timeout - return accumulated responses or None
        if responses:
            return {'data': responses, 'answer': 'timeout'}
        return None

    def send_and_wait(self, command: str, timeout: float = 2.0) -> bool:
        """Send command and wait for ACK.

        Convenience method combining send() and wait_for_ack().

        Args:
            command: Command to send
            timeout: Timeout for ACK

        Returns:
            True if command succeeded (got 'done')
            False if command failed or timed out
        """
        self.send(command)
        return self.wait_for_ack(expected_count=1, timeout=timeout)

    def wait_for_shutdown(self, timeout: float = 5.0) -> None:
        """Wait for shutdown signal from ExaBGP.

        Blocks until shutdown is received, parent dies, or timeout expires.

        Args:
            timeout: Maximum time to wait (default: 5.0 seconds)
        """
        start_time = time.time()
        try:
            while os.getppid() != 1 and time.time() - start_time < timeout:
                line = self.read_line(0.5)
                if line is not None:
                    answer = self.parse_answer(line)
                    if answer == 'shutdown' or 'shutdown' in line:
                        break
        except (IOError, OSError):
            pass

    # ==================================================================
    # Stage 1: Declaration Protocol
    # ==================================================================

    def declare_rfc(self, rfc_number: int) -> None:
        """Declare an RFC number (Stage 1).

        Used for human-readable feature tracking.

        Args:
            rfc_number: RFC number (e.g., 4271 for BGP-4)
        """
        self.send(f'declare rfc {rfc_number}')

    def declare_encoding(self, encoding: str) -> None:
        """Declare a supported encoding (Stage 1).

        Args:
            encoding: Encoding name (text, b64, or hex)
        """
        self.send(f'declare encoding {encoding}')

    def declare_family(self, afi: str, safi: str | None = None) -> None:
        """Declare an address family (Stage 1).

        Args:
            afi: Address Family Identifier (ipv4, ipv6, l2vpn, or 'all')
            safi: Subsequent AFI (unicast, multicast, etc.) - not needed for 'all'
        """
        if afi == 'all':
            self.send('declare family all')
        else:
            self.send(f'declare family {afi} {safi}')

    def declare_config(self, pattern: str) -> None:
        """Declare a config pattern hook (Stage 1).

        Pattern syntax:
          - '*' matches any single path element
          - '<name:regex>' is a named capture with validation regex

        Example: "peer * capability hostname <hostname:.*>"

        Args:
            pattern: Config pattern to match
        """
        self.send(f'declare conf {pattern}')

    def declare_command(self, command: str) -> None:
        """Declare a command handler (Stage 1).

        Args:
            command: Command name to register (e.g., "rib adjacent in show")
        """
        self.send(f'declare cmd {command}')

    def declare_done(self) -> None:
        """Signal Stage 1 declaration complete.

        After calling this, wait for config delivery (Stage 2).
        """
        self.send('declare done')

    # ==================================================================
    # Stage 2: Config Delivery
    # ==================================================================

    def wait_for_config(self, timeout: float = 5.0) -> list[dict]:
        """Wait for config delivery from ZeBGP (Stage 2).

        Reads configuration lines until "config done".

        Args:
            timeout: Maximum time to wait

        Returns:
            List of config entries, each with 'context', 'name', 'value' keys
        """
        configs = []
        start_time = time.time()

        while time.time() - start_time < timeout:
            line = self.read_line(0.1)
            if line is None:
                continue

            if line == 'config done':
                break

            # Parse: "config <context> <name> <value>"
            if line.startswith('config '):
                rest = line[len('config '):]
                # Split into parts - last is value, second-to-last is name, rest is context
                parts = rest.rsplit(' ', 2)
                if len(parts) >= 3:
                    configs.append({
                        'context': parts[0],
                        'name': parts[1],
                        'value': parts[2]
                    })

        return configs

    # ==================================================================
    # Stage 3: Capability Declaration
    # ==================================================================

    def declare_capability(self, code: int, payload: str, encoding: str = 'b64') -> None:
        """Declare a capability for OPEN messages (Stage 3).

        Args:
            code: Capability type code (e.g., 73 for hostname)
            payload: Encoded capability value
            encoding: Encoding of payload (b64, hex, or text)
        """
        self.send(f'capability {encoding} {code} {payload}')

    def capability_done(self) -> None:
        """Signal Stage 3 capability declaration complete.

        After calling this, wait for registry sharing (Stage 4).
        """
        self.send('capability done')

    # ==================================================================
    # Stage 4: Registry Sharing
    # ==================================================================

    def wait_for_registry(self, timeout: float = 5.0) -> dict:
        """Wait for registry sharing from ZeBGP (Stage 4).

        Reads registry lines until "registry done".

        Args:
            timeout: Maximum time to wait

        Returns:
            Dict with 'name' (plugin name) and 'commands' (list of registered commands)
        """
        result = {'name': '', 'commands': []}
        start_time = time.time()

        while time.time() - start_time < timeout:
            line = self.read_line(0.1)
            if line is None:
                continue

            if line == 'registry done':
                break

            # Parse: "registry name <name>" or "registry <plugin> <encoding> cmd <command>"
            if line.startswith('registry name '):
                result['name'] = line[len('registry name '):]
                self._plugin_name = result['name']
            elif line.startswith('registry '):
                # "registry <plugin> <encoding> cmd <command>"
                parts = line.split(' ', 4)
                if len(parts) >= 5 and parts[3] == 'cmd':
                    result['commands'].append({
                        'plugin': parts[1],
                        'encoding': parts[2],
                        'command': parts[4]
                    })

        return result

    @property
    def plugin_name(self) -> str:
        """Return the plugin name assigned during registry sharing."""
        return self._plugin_name

    # ==================================================================
    # Stage 5: Ready Signal
    # ==================================================================

    def ready(self) -> None:
        """Signal to ZeBGP that this API process is ready (Stage 5).

        This is the final stage of the startup protocol.
        """
        self.send('ready')

    # ==================================================================
    # Error Handling
    # ==================================================================

    def fail(self, message: str, encoding: str = 'text') -> None:
        """Signal startup failure to ZeBGP.

        Can be called at any stage to abort startup.

        Args:
            message: Error message
            encoding: Encoding of message (text, b64, or hex)
        """
        self.send(f'ready failed {encoding} {message}')


# Convenience functions for simple scripts that don't need the class

_api: API | None = None


def _get_api() -> API:
    """Get or create singleton API instance."""
    global _api
    if _api is None:
        _api = API()
    return _api


def ready() -> None:
    """Signal to ZeBGP that this API process is ready.

    MUST be called at the start of every API script before sending commands.
    ZeBGP waits for all API processes to signal ready before starting BGP peers.

    This function performs the minimal 5-stage protocol:
    - Stage 1: declare done (no declarations)
    - Stage 2: wait for config
    - Stage 3: capability done (no capabilities)
    - Stage 4: wait for registry
    - Stage 5: ready

    For full protocol support, use the API class directly with the
    declare methods.
    """
    api = _get_api()
    # Stage 1: Empty declaration
    api.declare_done()
    # Stage 2: Receive config (discard)
    api.wait_for_config(timeout=5.0)
    # Stage 3: No capabilities
    api.capability_done()
    # Stage 4: Receive registry (discard)
    api.wait_for_registry(timeout=5.0)
    # Stage 5: Ready
    api.ready()


def flush(msg: str) -> None:
    """Write message to stdout and flush."""
    _get_api().flush(msg)


def send(command: str) -> None:
    """Send command to ExaBGP."""
    _get_api().send(command)


def wait_for_ack(expected_count: int = 1, timeout: float = 2.0) -> bool:
    """Wait for ACK responses from ExaBGP."""
    return _get_api().wait_for_ack(expected_count, timeout)


def read_response(timeout: float = 2.0) -> dict | str | None:
    """Read complete response from ExaBGP."""
    return _get_api().read_response(timeout)


def send_and_wait(command: str, timeout: float = 2.0) -> bool:
    """Send command and wait for ACK."""
    return _get_api().send_and_wait(command, timeout)


def wait_for_shutdown(timeout: float = 5.0) -> None:
    """Wait for shutdown signal."""
    _get_api().wait_for_shutdown(timeout)


# New protocol convenience functions

def declare_rfc(rfc_number: int) -> None:
    """Declare an RFC number (Stage 1)."""
    _get_api().declare_rfc(rfc_number)


def declare_encoding(encoding: str) -> None:
    """Declare a supported encoding (Stage 1)."""
    _get_api().declare_encoding(encoding)


def declare_family(afi: str, safi: str | None = None) -> None:
    """Declare an address family (Stage 1)."""
    _get_api().declare_family(afi, safi)


def declare_config(pattern: str) -> None:
    """Declare a config pattern hook (Stage 1)."""
    _get_api().declare_config(pattern)


def declare_command(command: str) -> None:
    """Declare a command handler (Stage 1)."""
    _get_api().declare_command(command)


def declare_done() -> None:
    """Signal Stage 1 declaration complete."""
    _get_api().declare_done()


def wait_for_config(timeout: float = 5.0) -> list[dict]:
    """Wait for config delivery from ZeBGP (Stage 2)."""
    return _get_api().wait_for_config(timeout)


def declare_capability(code: int, payload: str, encoding: str = 'b64') -> None:
    """Declare a capability for OPEN messages (Stage 3)."""
    _get_api().declare_capability(code, payload, encoding)


def capability_done() -> None:
    """Signal Stage 3 capability declaration complete."""
    _get_api().capability_done()


def wait_for_registry(timeout: float = 5.0) -> dict:
    """Wait for registry sharing from ZeBGP (Stage 4)."""
    return _get_api().wait_for_registry(timeout)


def fail(message: str, encoding: str = 'text') -> None:
    """Signal startup failure to ZeBGP."""
    _get_api().fail(message, encoding)
