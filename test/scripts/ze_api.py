#!/usr/bin/env python3
"""Shared library for ZeBGP API test scripts.

Provides YANG RPC communication with ZeBGP daemon over socket pair FDs.
Uses newline-delimited #id verb [json] framing over Unix socket pairs:
  - Socket A (FD from ZE_ENGINE_FD, default 3): plugin -> engine RPCs
  - Socket B (FD from ZE_CALLBACK_FD, default 4): engine -> plugin callbacks

5-stage plugin registration protocol (YANG RPC):
  - Stage 1: declare-registration (plugin -> engine, Socket A)
  - Stage 2: configure (engine -> plugin, Socket B)
  - Stage 3: declare-capabilities (plugin -> engine, Socket A)
  - Stage 4: share-registry (engine -> plugin, Socket B)
  - Stage 5: ready (plugin -> engine, Socket A)

Simple usage:
    from ze_api import ready, send, wait_for_shutdown

    ready()
    send('peer * update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24')
    wait_for_shutdown()

Full protocol usage:
    from ze_api import API

    api = API()
    # Stage 1: Declare capabilities
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
    api.send('peer * update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24')
"""

from __future__ import annotations

import array
import json
import os
import select
import signal
import socket
import sys
from typing import Any


class API:
    """ZeBGP API client using YANG RPC over socket pair FDs.

    Communicates with the engine via two Unix sockets:
      - Socket A (engine_fd): plugin -> engine RPCs (registration, routes, subscribe)
      - Socket B (callback_fd): engine -> plugin callbacks (config, events, bye)

    Messages are newline-delimited lines: #<id> <verb> [<json-payload>]
    """

    def __init__(self, engine_fd: int | None = None, callback_fd: int | None = None):
        """Initialize API client.

        Args:
            engine_fd: File descriptor for Socket A (default: ZE_ENGINE_FD env or 3)
            callback_fd: File descriptor for Socket B (default: ZE_CALLBACK_FD env or 4)
        """
        if engine_fd is None:
            engine_fd = int(os.environ.get('ZE_ENGINE_FD', '3'))
        if callback_fd is None:
            callback_fd = int(os.environ.get('ZE_CALLBACK_FD', '4'))

        self._engine_fd = engine_fd
        self._callback_fd = callback_fd
        self._engine_buf = b''
        self._callback_buf = b''
        self._req_id = 0
        self._shutdown = False

        # Accumulated declarations for Stage 1
        self._families: list[dict[str, str]] = []
        self._commands: list[dict[str, str]] = []
        self._wants_config: list[str] = []

        # Accumulated connection handlers for Stage 1
        self._connection_handlers: list[dict[str, Any]] = []

        # Accumulated capabilities for Stage 3
        self._capabilities: list[dict[str, Any]] = []

        # Accumulated subscription for Stage 5 (ready RPC)
        self._subscription: dict[str, Any] | None = None

        # Plugin name from registry sharing
        self._plugin_name = ''

        # Pending events from deliver-batch (returned one per read_line call)
        self._pending_events: list[str] = []

        # Install SIGPIPE handler
        signal.signal(signal.SIGPIPE, signal.SIG_DFL)

    # ==================================================================
    # Low-level newline-framed line transport
    # ==================================================================

    def _format_line(self, req_id: int, verb: str, payload: dict | None = None) -> bytes:
        """Format #<id> <verb> [<json-payload>] newline-terminated line."""
        if payload is not None:
            json_str = json.dumps(payload, separators=(',', ':'))
            return f'#{req_id} {verb} {json_str}\n'.encode('utf-8')
        return f'#{req_id} {verb}\n'.encode('utf-8')

    def _parse_line(self, line: str) -> tuple[int, str, dict | None]:
        """Parse #<id> <verb> [<json-payload>] from a raw line.

        Returns:
            Tuple of (request_id, verb, payload_dict_or_None)
        """
        if not line.startswith('#'):
            raise RuntimeError(f'line missing # prefix: {line[:80]}')
        rest = line[1:]
        id_str, _, body = rest.partition(' ')
        req_id = int(id_str)
        verb, _, payload_str = body.partition(' ')
        payload = json.loads(payload_str) if payload_str else None
        return req_id, verb, payload

    def _send_rpc(self, fd: int, req_id: int, method: str, params: dict | None = None) -> None:
        """Send a newline-terminated RPC line: #<id> <method> [<json-params>]."""
        line = self._format_line(req_id, method, params)
        os.write(fd, line)

    def _read_line(self, fd: int, buf_attr: str, timeout: float | None = None) -> str | None:
        """Read a newline-terminated line from fd.

        Args:
            fd: File descriptor to read from
            buf_attr: Name of the buffer attribute ('_engine_buf' or '_callback_buf')
            timeout: Seconds to wait (None = block forever)

        Returns:
            Raw line (without newline), or None on timeout/EOF
        """
        buf = getattr(self, buf_attr)

        while True:
            # Check buffer for complete line
            nl_pos = buf.find(b'\n')
            if nl_pos >= 0:
                line_bytes = buf[:nl_pos]
                setattr(self, buf_attr, buf[nl_pos + 1:])
                return line_bytes.decode('utf-8')

            # Wait for data
            if timeout is not None:
                ready_fds, _, _ = select.select([fd], [], [], timeout)
                if not ready_fds:
                    return None
            else:
                # Block until data available
                select.select([fd], [], [])

            try:
                chunk = os.read(fd, 65536)
            except OSError:
                return None
            if not chunk:
                return None
            buf += chunk
            setattr(self, buf_attr, buf)

    # ==================================================================
    # RPC helpers
    # ==================================================================

    def _next_id(self) -> int:
        """Generate next request ID."""
        self._req_id += 1
        return self._req_id

    def _call_engine(self, method: str, params: Any = None) -> dict | None:
        """Send RPC to engine on Socket A and wait for response.

        Sends: #<id> <method> [<json-params>]
        Expects: #<id> ok [<json>] or #<id> error [<json>]

        Args:
            method: RPC method name (e.g., 'ze-plugin-engine:declare-registration')
            params: Optional parameters dict

        Returns:
            Response payload dict, or None

        Raises:
            RuntimeError: If the engine returns an error response
        """
        req_id = self._next_id()
        self._send_rpc(self._engine_fd, req_id, method, params)

        line = self._read_line(self._engine_fd, '_engine_buf')
        if line is None:
            raise RuntimeError(f'no response for {method}')

        resp_id, verb, payload = self._parse_line(line)
        if verb == 'error':
            msg = ''
            if payload:
                msg = payload.get('message', str(payload))
            raise RuntimeError(f'RPC error from {method}: {msg}')
        # Wrap payload in {"result": ...} envelope for backward compatibility
        # with test scripts that access resp.get('result', {}).
        return {'result': payload}

    def _respond_ok(self, req_id: int) -> None:
        """Send OK response on Socket B: #<id> ok"""
        line = self._format_line(req_id, 'ok')
        os.write(self._callback_fd, line)

    def _serve_one(self, expected_method: str, timeout: float = 10.0) -> dict | None:
        """Read one RPC request from Socket B, verify method, respond OK.

        Reads: #<id> <method> [<json-params>]
        Sends: #<id> ok

        Args:
            expected_method: Expected RPC method name
            timeout: Seconds to wait

        Returns:
            The params from the request, or None

        Raises:
            RuntimeError: If unexpected method received
        """
        line = self._read_line(self._callback_fd, '_callback_buf', timeout=timeout)
        if line is None:
            raise RuntimeError(f'timeout waiting for {expected_method}')

        req_id, method, params = self._parse_line(line)
        if method != expected_method:
            raise RuntimeError(f'expected {expected_method}, got {method}')

        self._respond_ok(req_id)
        return params

    # ==================================================================
    # Stage 1: Declaration Protocol
    # ==================================================================

    def declare_rfc(self, rfc_number: int) -> None:
        """Declare an RFC number (Stage 1). Informational only."""
        # RFC declarations are informational in text protocol.
        # YANG RPC doesn't have an RFC field, so this is a no-op.
        pass

    def declare_encoding(self, encoding: str) -> None:
        """Declare a supported encoding (Stage 1). Informational only."""
        # Encoding declarations are informational in text protocol.
        # YANG RPC doesn't have an encoding field, so this is a no-op.
        pass

    def declare_family(self, afi: str, safi: str | None = None, mode: str = 'both') -> None:
        """Declare an address family (Stage 1).

        Args:
            afi: Address Family Identifier (ipv4, ipv6, l2vpn, or 'all')
            safi: Subsequent AFI (unicast, multicast, flow, etc.)
            mode: 'encode', 'decode', or 'both'
        """
        if afi == 'all':
            # 'all' is not valid in YANG RPC, skip (engine handles all families)
            return
        name = f'{afi}/{safi}' if safi else afi
        self._families.append({'name': name, 'mode': mode})

    def declare_config(self, pattern: str) -> None:
        """Declare a config pattern hook (Stage 1).

        Args:
            pattern: Config root pattern
        """
        self._wants_config.append(pattern)

    def declare_command(self, command: str) -> None:
        """Declare a command handler (Stage 1).

        Args:
            command: Command name to register
        """
        self._commands.append({'name': command})

    def declare_connection_handler(self, handler_type: str = 'listen',
                                   port: int = 0, address: str = '') -> None:
        """Declare a connection handler for listen socket handoff (Stage 1).

        The engine will create a listen socket on the specified port and
        send the fd via SCM_RIGHTS on Socket B after Stage 1.
        Call receive_listener() after declare_done() to receive the fd.

        Args:
            handler_type: Handler type ('listen' for Mode A)
            port: TCP port to listen on (1-65535)
            address: Bind address (empty = all interfaces)
        """
        self._connection_handlers.append({
            'type': handler_type,
            'port': port,
            'address': address,
        })

    def declare_done(self) -> None:
        """Signal Stage 1 declaration complete.

        Sends ze-plugin-engine:declare-registration RPC with all
        accumulated declarations.
        """
        params: dict[str, Any] = {}
        if self._families:
            params['families'] = self._families
        if self._commands:
            params['commands'] = self._commands
        if self._wants_config:
            params['wants-config'] = self._wants_config
        if self._connection_handlers:
            params['connection-handlers'] = self._connection_handlers

        self._call_engine('ze-plugin-engine:declare-registration', params)

    def receive_listener(self) -> socket.socket:
        """Receive a listen socket fd from the engine via SCM_RIGHTS on Socket B.

        Must be called after declare_done() and before wait_for_config().
        The engine sends one fd per connection-handler declared in Stage 1.

        Returns:
            A socket object wrapping the received listen socket fd.
        """
        # Create a socket object from the raw callback fd for recvmsg.
        # socket.fromfd() dups the fd -- we must close this socket after use.
        sock = socket.fromfd(self._callback_fd, socket.AF_UNIX, socket.SOCK_STREAM)
        try:
            # Read 1 framing byte + ancillary data (SCM_RIGHTS carrying one fd).
            fds = array.array('i')
            msg, ancdata, _flags, _addr = sock.recvmsg(
                1, socket.CMSG_SPACE(fds.itemsize))
            if not msg:
                raise RuntimeError('no data received (connection closed?)')
            for cmsg_level, cmsg_type, cmsg_data in ancdata:
                if (cmsg_level == socket.SOL_SOCKET and
                        cmsg_type == socket.SCM_RIGHTS):
                    received_fds = array.array('i', cmsg_data)
                    if not received_fds:
                        continue
                    fd = received_fds[0]
                    # Close any extra fds.
                    for extra_fd in received_fds[1:]:
                        os.close(extra_fd)
                    # Detect socket family from the fd via getsockname.
                    # The engine may create IPv4 or IPv6 listeners depending
                    # on the address configured in the connection-handler.
                    probe = socket.fromfd(fd, socket.AF_INET, socket.SOCK_STREAM)
                    try:
                        addr = probe.getsockname()
                        # IPv6 getsockname returns 4-tuple, IPv4 returns 2-tuple
                        family = socket.AF_INET6 if len(addr) == 4 else socket.AF_INET
                    except OSError:
                        family = socket.AF_INET
                    finally:
                        probe.close()
                    # Create a listener socket with the detected family.
                    # fromfd() dups the fd, so close the original.
                    listener = socket.fromfd(fd, family, socket.SOCK_STREAM)
                    os.close(fd)
                    return listener
            raise RuntimeError('no fd in control message')
        finally:
            # Close the dup'd socket -- the original _callback_fd is unaffected.
            sock.close()

    # ==================================================================
    # Stage 2: Config Delivery
    # ==================================================================

    def wait_for_config(self, timeout: float = 10.0) -> list[dict]:
        """Wait for config delivery from ZeBGP (Stage 2).

        Reads ze-plugin-callback:configure RPC from Socket B.

        Args:
            timeout: Maximum time to wait

        Returns:
            List of config sections, each with 'root' and 'data' keys
        """
        params = self._serve_one('ze-plugin-callback:configure', timeout=timeout)
        if params is None:
            return []
        sections = params.get('sections', []) or []
        # Convert to compatible format
        configs = []
        for section in sections:
            configs.append({
                'context': section.get('root', ''),
                'name': '',
                'value': section.get('data', ''),
                'root': section.get('root', ''),
                'data': section.get('data', ''),
            })
        return configs

    # ==================================================================
    # Stage 3: Capability Declaration
    # ==================================================================

    def declare_capability(self, code: int, payload: str, encoding: str = 'b64') -> None:
        """Declare a capability for OPEN messages (Stage 3).

        Args:
            code: Capability type code
            payload: Encoded capability value
            encoding: Encoding of payload (b64, hex, or text)
        """
        self._capabilities.append({
            'code': code,
            'encoding': encoding,
            'payload': payload,
        })

    def capability_done(self) -> None:
        """Signal Stage 3 capability declaration complete.

        Sends ze-plugin-engine:declare-capabilities RPC.
        """
        params = {'capabilities': self._capabilities}
        self._call_engine('ze-plugin-engine:declare-capabilities', params)

    # ==================================================================
    # Stage 4: Registry Sharing
    # ==================================================================

    def wait_for_registry(self, timeout: float = 10.0) -> dict:
        """Wait for registry sharing from ZeBGP (Stage 4).

        Reads ze-plugin-callback:share-registry RPC from Socket B.

        Args:
            timeout: Maximum time to wait

        Returns:
            Dict with 'name' (empty in YANG RPC) and 'commands' list
        """
        params = self._serve_one('ze-plugin-callback:share-registry', timeout=timeout)
        commands = []
        if params:
            for cmd in (params.get('commands', []) or []):
                commands.append({
                    'plugin': cmd.get('plugin', ''),
                    'encoding': cmd.get('encoding', ''),
                    'command': cmd.get('name', ''),
                    'name': cmd.get('name', ''),
                })
        return {'name': self._plugin_name, 'commands': commands}

    @property
    def plugin_name(self) -> str:
        """Return the plugin name assigned during registry sharing."""
        return self._plugin_name

    # ==================================================================
    # Stage 5: Ready Signal
    # ==================================================================

    def ready(self) -> None:
        """Signal to ZeBGP that this API process is ready (Stage 5).

        Sends ze-plugin-engine:ready RPC, including any accumulated
        subscriptions to avoid the race condition between ready and
        event delivery.
        """
        params: dict[str, Any] = {}
        if self._subscription is not None:
            params['subscribe'] = self._subscription
        self._call_engine('ze-plugin-engine:ready', params)

    # ==================================================================
    # Runtime: Send commands
    # ==================================================================

    def flush(self, msg: str) -> None:
        """Write message (strip trailing newline, route to appropriate RPC).

        Args:
            msg: Message to send (trailing newline stripped)
        """
        msg = msg.rstrip('\n')
        if msg:
            self.send(msg)

    def send(self, command: str) -> None:
        """Send a command to ZeBGP.

        Routes to the appropriate RPC based on command prefix:
        - 'peer ...' -> ze-plugin-engine:update-route
        - 'subscribe ...' -> accumulates for ready RPC

        Args:
            command: Command string
        """
        command = command.strip()
        if not command:
            return

        if command.startswith('subscribe '):
            self._handle_subscribe_command(command)
        elif command.startswith('peer ') or command.startswith('update '):
            self._send_update_route(command)
        else:
            # Unknown command type -- try as update-route
            self._send_update_route(command)

    def _handle_subscribe_command(self, command: str) -> None:
        """Parse subscribe text command and accumulate for ready RPC.

        Format: 'subscribe event <event1> [<event2> ...]'
        """
        parts = command.split()
        # Skip 'subscribe' and optional 'event' prefix
        events = []
        i = 1
        while i < len(parts):
            if parts[i] == 'event':
                i += 1
                continue
            events.append(parts[i])
            i += 1

        if events:
            self._subscription = {
                'events': events,
                'peers': [],
                'format': '',
            }

    def subscribe(self, events: list[str], peers: list[str] | None = None, fmt: str = '') -> None:
        """Set event subscriptions for the ready RPC.

        Must be called before ready(). Subscriptions are included
        atomically in the ready RPC to avoid race conditions.

        Args:
            events: Event types ('update', 'open', 'state', etc.)
            peers: Optional peer filter (empty = all)
            fmt: Wire format ('hex', 'parsed', etc.)
        """
        self._subscription = {
            'events': events,
            'peers': peers or [],
            'format': fmt,
        }

    def _send_update_route(self, command: str) -> None:
        """Send ze-plugin-engine:update-route RPC.

        Args:
            command: Full command string (e.g., 'peer * update text ...')
        """
        # Extract peer selector from 'peer <selector> <rest>'
        peer_selector = '*'
        cmd = command
        if command.startswith('peer '):
            rest = command[len('peer '):]
            # Find the next space-separated token as peer selector
            parts = rest.split(' ', 1)
            peer_selector = parts[0]
            cmd = parts[1] if len(parts) > 1 else ''

        params = {
            'peer-selector': peer_selector,
            'command': cmd,
        }
        self._call_engine('ze-plugin-engine:update-route', params)

    # ==================================================================
    # Runtime: Read events / responses
    # ==================================================================

    def read_line(self, timeout: float = 0.1) -> str | None:
        """Read the next event from Socket B.

        Handles incoming RPC callbacks:
        - deliver-event: extracts event JSON string, responds OK, returns event
        - bye: responds OK, marks shutdown, returns None
        - other methods: responds OK, skips

        Args:
            timeout: Seconds to wait for data

        Returns:
            Event JSON string, or None if no event available
        """
        if self._shutdown:
            return None

        # Return buffered events from a previous deliver-batch first.
        if self._pending_events:
            return self._pending_events.pop(0)

        raw = self._read_line(self._callback_fd, '_callback_buf', timeout=timeout)
        if raw is None:
            return None

        req_id, method, params = self._parse_line(raw)

        if method == 'ze-plugin-callback:deliver-event':
            self._respond_ok(req_id)
            if params:
                return params.get('event', '')
            return ''

        if method == 'ze-plugin-callback:deliver-batch':
            self._respond_ok(req_id)
            if params:
                events = params.get('events', [])
                # Convert each event object to a JSON string for the plugin.
                for event in events:
                    self._pending_events.append(
                        json.dumps(event, separators=(',', ':')) if isinstance(event, dict) else str(event)
                    )
            if self._pending_events:
                return self._pending_events.pop(0)
            return ''

        if method == 'ze-plugin-callback:bye':
            self._respond_ok(req_id)
            self._shutdown = True
            return None

        # Other callbacks -- respond OK and skip
        self._respond_ok(req_id)
        return self.read_line(timeout)

    def parse_answer(self, line: str) -> str | None:
        """Parse answer type from response line.

        In YANG RPC, responses are handled per-RPC. This is kept for
        compatibility but mainly returns None.

        Args:
            line: Response line to parse

        Returns:
            Answer type or None
        """
        if not line:
            return None
        if line.startswith('{'):
            try:
                data = json.loads(line)
                return data.get('answer')
            except (json.JSONDecodeError, TypeError):
                return None
        if line in ('done', 'error', 'shutdown'):
            return line
        return None

    def wait_for_ack(self, expected_count: int = 1, timeout: float = 2.0) -> bool:
        """Wait for route delivery after send().

        In YANG RPC, send() gets an RPC response synchronously, but that only
        means "command dispatched" -- NOT "route delivered to peer". The BGP
        session may still be establishing (OPENSENT/OPENCONFIRM) when the RPC
        returns, and queued routes are flushed asynchronously on ESTABLISHED.

        This delay ensures routes have time to be delivered before the plugin
        proceeds with further commands (e.g., teardown, withdraw) that depend
        on the routes having reached the peer.

        Args:
            expected_count: Number of routes sent (scales the delay)
            timeout: Timeout in seconds (unused, kept for API compat)

        Returns:
            True (always succeeds)
        """
        import time
        # Allow time for BGP session establishment + route delivery.
        # Session establishes in ~1ms typically, but under load it may take longer.
        delay = 0.2 * max(1, expected_count)
        time.sleep(delay)
        return True

    def read_response(self, timeout: float = 2.0) -> dict | str | None:
        """Read and parse a complete response.

        Args:
            timeout: Maximum time to wait

        Returns:
            Event data dict, or None on timeout
        """
        line = self.read_line(timeout)
        if line is None:
            return None
        try:
            return json.loads(line)
        except (json.JSONDecodeError, TypeError):
            return line

    def send_and_wait(self, command: str, timeout: float = 2.0) -> bool:
        """Send command and wait for ACK.

        In YANG RPC, send() is synchronous, so this is equivalent to send().

        Args:
            command: Command to send
            timeout: Timeout for ACK (ignored)

        Returns:
            True if command succeeded
        """
        try:
            self.send(command)
            return True
        except RuntimeError:
            return False

    def wait_for_shutdown(self, timeout: float = 5.0) -> None:
        """Wait for shutdown signal from ZeBGP.

        Processes incoming RPC callbacks on Socket B until bye is received,
        parent dies, or timeout expires.

        Args:
            timeout: Maximum time to wait
        """
        import time
        start_time = time.time()
        try:
            while not self._shutdown and os.getppid() != 1:
                elapsed = time.time() - start_time
                if elapsed >= timeout:
                    break
                remaining = timeout - elapsed
                wait = min(0.5, remaining)
                # Process any pending callbacks (events, bye)
                self.read_line(timeout=wait)
        except (IOError, OSError):
            pass

    # ==================================================================
    # Error Handling
    # ==================================================================

    def fail(self, message: str, encoding: str = 'text') -> None:
        """Signal startup failure to ZeBGP.

        In YANG RPC, errors are signaled via RPC error responses.
        This raises an exception to abort the plugin.

        Args:
            message: Error message
            encoding: Encoding of message (ignored in YANG RPC)
        """
        raise RuntimeError(f'plugin startup failed: {message}')


# ==================================================================
# Convenience functions for simple scripts
# ==================================================================

_api: API | None = None


def _get_api() -> API:
    """Get or create singleton API instance."""
    global _api
    if _api is None:
        _api = API()
    return _api


def ready() -> None:
    """Signal to ZeBGP that this API process is ready.

    Performs the minimal 5-stage protocol:
    - Stage 1: declare done (no declarations)
    - Stage 2: wait for config
    - Stage 3: capability done (no capabilities)
    - Stage 4: wait for registry
    - Stage 5: ready
    """
    api = _get_api()
    # Stage 1: Empty declaration
    api.declare_done()
    # Stage 2: Receive config (discard)
    api.wait_for_config(timeout=10.0)
    # Stage 3: No capabilities
    api.capability_done()
    # Stage 4: Receive registry (discard)
    api.wait_for_registry(timeout=10.0)
    # Stage 5: Ready
    api.ready()


def flush(msg: str) -> None:
    """Write message (strip newline, route to RPC)."""
    _get_api().flush(msg)


def send(command: str) -> None:
    """Send command to ZeBGP."""
    _get_api().send(command)


def wait_for_ack(expected_count: int = 1, timeout: float = 2.0) -> bool:
    """Wait for ACK responses (no-op in YANG RPC)."""
    return _get_api().wait_for_ack(expected_count, timeout)


def read_response(timeout: float = 2.0) -> dict | str | None:
    """Read complete response from ZeBGP."""
    return _get_api().read_response(timeout)


def send_and_wait(command: str, timeout: float = 2.0) -> bool:
    """Send command and wait for ACK."""
    return _get_api().send_and_wait(command, timeout)


def wait_for_shutdown(timeout: float = 5.0) -> None:
    """Wait for shutdown signal."""
    _get_api().wait_for_shutdown(timeout)


# Stage protocol convenience functions

def declare_rfc(rfc_number: int) -> None:
    """Declare an RFC number (Stage 1)."""
    _get_api().declare_rfc(rfc_number)


def declare_encoding(encoding: str) -> None:
    """Declare a supported encoding (Stage 1)."""
    _get_api().declare_encoding(encoding)


def declare_family(afi: str, safi: str | None = None, mode: str = 'both') -> None:
    """Declare an address family (Stage 1)."""
    _get_api().declare_family(afi, safi, mode)


def declare_config(pattern: str) -> None:
    """Declare a config pattern hook (Stage 1)."""
    _get_api().declare_config(pattern)


def declare_command(command: str) -> None:
    """Declare a command handler (Stage 1)."""
    _get_api().declare_command(command)


def declare_connection_handler(handler_type: str = 'listen',
                                port: int = 0, address: str = '') -> None:
    """Declare a connection handler for listen socket handoff (Stage 1)."""
    _get_api().declare_connection_handler(handler_type, port, address)


def receive_listener() -> socket.socket:
    """Receive a listen socket fd from the engine via SCM_RIGHTS."""
    return _get_api().receive_listener()


def declare_done() -> None:
    """Signal Stage 1 declaration complete."""
    _get_api().declare_done()


def wait_for_config(timeout: float = 10.0) -> list[dict]:
    """Wait for config delivery from ZeBGP (Stage 2)."""
    return _get_api().wait_for_config(timeout)


def declare_capability(code: int, payload: str, encoding: str = 'b64') -> None:
    """Declare a capability for OPEN messages (Stage 3)."""
    _get_api().declare_capability(code, payload, encoding)


def capability_done() -> None:
    """Signal Stage 3 capability declaration complete."""
    _get_api().capability_done()


def wait_for_registry(timeout: float = 10.0) -> dict:
    """Wait for registry sharing from ZeBGP (Stage 4)."""
    return _get_api().wait_for_registry(timeout)


def subscribe(events: list[str], peers: list[str] | None = None, fmt: str = '') -> None:
    """Set event subscriptions for ready RPC."""
    _get_api().subscribe(events, peers, fmt)


def fail(message: str, encoding: str = 'text') -> None:
    """Signal startup failure to ZeBGP."""
    _get_api().fail(message, encoding)
