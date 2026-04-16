#!/usr/bin/env python3
"""Shared library for ZeBGP API test scripts.

Provides YANG RPC communication with ZeBGP daemon.
Uses newline-delimited #id verb [json] framing.

Transport modes (auto-detected from environment):
  - TLS: single connection via ze.plugin.hub.host/port/token (preferred)
  - FD:  inherited file descriptors via ze.engine.fd/ze.callback.fd (legacy)

Environment variables support dot, underscore, and uppercase notation:
  ze.plugin.hub.host / ze_plugin_hub_host / ZE_PLUGIN_HUB_HOST

5-stage plugin registration protocol (YANG RPC):
  - Stage 1: declare-registration (plugin -> engine)
  - Stage 2: configure (engine -> plugin)
  - Stage 3: declare-capabilities (plugin -> engine)
  - Stage 4: share-registry (engine -> plugin)
  - Stage 5: ready (plugin -> engine)

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
import ssl
import sys
from typing import Any, Callable


def _ze_env(key: str, default: str = "") -> str:
    """Look up a Ze environment variable in dot, lowercase underscore, and uppercase underscore notation.

    Priority: ze.foo.bar > ze_foo_bar > ZE_FOO_BAR
    """
    v = os.environ.get(key, "")
    if v:
        return v
    under = key.replace(".", "_")
    v = os.environ.get(under, "")
    if v:
        return v
    return os.environ.get(under.upper(), default)


class API:
    """ZeBGP API client using YANG RPC protocol.

    Transport is auto-detected from environment:
      - TLS mode (ZE_PLUGIN_HUB_TOKEN set): single TLS connection, bidirectional mux
      - FD mode (ZE_ENGINE_FD set): inherited file descriptors (legacy)

    Messages are newline-delimited lines: #<id> <verb> [<json-payload>]
    """

    def __init__(self, engine_fd: int | None = None, callback_fd: int | None = None):
        """Initialize API client.

        Auto-detects transport from environment variables:
          - ZE_PLUGIN_HUB_TOKEN -> TLS mode (single connection)
          - ZE_ENGINE_FD/ZE_CALLBACK_FD -> FD mode (legacy)
        """
        self._tls_sock: ssl.SSLSocket | None = None
        self._tls_mode = False
        self._read_buf = b""
        self._pending_requests: list[tuple[int, str, dict | None]] = []
        self._engine_fd = -1
        self._callback_fd = -1

        token = _ze_env("ze.plugin.hub.token")
        if token:
            self._init_tls(token)
        else:
            if engine_fd is None:
                engine_fd = int(_ze_env("ze.engine.fd", "3"))
            if callback_fd is None:
                callback_fd = int(_ze_env("ze.callback.fd", "4"))
            self._engine_fd = engine_fd
            self._callback_fd = callback_fd

        self._engine_buf = b""
        self._callback_buf = b""
        self._req_id = 0
        self._shutdown = False

        # Plugin name (set during TLS auth or registry sharing)
        self._name = _ze_env("ze.plugin.name", "python-plugin")

        # Accumulated declarations for Stage 1
        self._families: list[dict[str, str]] = []
        self._commands: list[dict[str, str]] = []
        self._wants_config: list[str] = []

        # Accumulated connection handlers for Stage 1
        self._connection_handlers: list[dict[str, Any]] = []

        # Accumulated filter declarations for Stage 1
        self._filters: list[dict[str, Any]] = []
        # Filter callback handler (runtime)
        self._filter_handler: Callable | None = None
        # Config transaction callback handlers (runtime, driven by the
        # engine-side RPC bridge on top of TxCoordinator). Each handler
        # receives the unmarshalled params dict and returns a dict that
        # is marshalled back as the RPC result. None means "accept" (the
        # default matching the Go SDK's initCallbackDefaults).
        self._config_verify_handler: Callable | None = None
        self._config_apply_handler: Callable | None = None
        self._config_rollback_handler: Callable | None = None

        # Accumulated capabilities for Stage 3
        self._capabilities: list[dict[str, Any]] = []

        # Accumulated subscription for Stage 5 (ready RPC)
        self._subscription: dict[str, Any] | None = None

        # Plugin name from registry sharing
        self._plugin_name = ""

        # Pending events from deliver-batch (returned one per read_line call)
        self._pending_events: list[str] = []

        # Install SIGPIPE handler
        signal.signal(signal.SIGPIPE, signal.SIG_DFL)

    def _init_tls(self, token: str) -> None:
        """Connect to engine via TLS and authenticate."""
        host = _ze_env("ze.plugin.hub.host", "127.0.0.1")
        port = int(_ze_env("ze.plugin.hub.port", "12700"))

        ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        ctx.check_hostname = False
        ctx.verify_mode = ssl.CERT_NONE
        ctx.minimum_version = ssl.TLSVersion.TLSv1_3

        raw_sock = socket.create_connection((host, port), timeout=10)
        self._tls_sock = ctx.wrap_socket(raw_sock, server_hostname=host)
        self._tls_mode = True

        # Auth: send #0 auth {"token":"...","name":"..."}
        name = _ze_env("ze.plugin.name", "python-plugin")
        auth_line = self._format_line(0, "auth", {"token": token, "name": name})
        self._tls_sock.sendall(auth_line)

        # Read auth response
        resp_line = self._read_tls_line(timeout=10.0)
        if resp_line is None:
            raise RuntimeError("no auth response from engine")
        _, verb, payload = self._parse_line(resp_line)
        if verb == "error":
            msg = payload.get("message", "") if payload else ""
            raise RuntimeError(f"auth rejected: {msg}")

        # In TLS mode, use the TLS socket fd for both engine and callback.
        self._engine_fd = self._tls_sock.fileno()
        self._callback_fd = self._tls_sock.fileno()

    def _read_tls_line(self, timeout: float | None = None) -> str | None:
        """Read a newline-terminated line from the TLS socket."""
        while True:
            nl_pos = self._read_buf.find(b"\n")
            if nl_pos >= 0:
                line_bytes = self._read_buf[:nl_pos]
                self._read_buf = self._read_buf[nl_pos + 1 :]
                return line_bytes.decode("utf-8")

            if timeout is not None:
                # TLS may have decrypted data buffered internally that
                # select() can't see on the raw socket. Check pending()
                # before falling through to select.
                if not self._tls_sock.pending():
                    ready_fds, _, _ = select.select([self._tls_sock], [], [], timeout)
                    if not ready_fds:
                        return None

            try:
                chunk = self._tls_sock.recv(65536)
            except (OSError, ssl.SSLError):
                return None
            if not chunk:
                return None
            self._read_buf += chunk

    # ==================================================================
    # Low-level newline-framed line transport
    # ==================================================================

    def _format_line(
        self, req_id: int, verb: str, payload: dict | None = None
    ) -> bytes:
        """Format #<id> <verb> [<json-payload>] newline-terminated line."""
        if payload is not None:
            json_str = json.dumps(payload, separators=(",", ":"))
            return f"#{req_id} {verb} {json_str}\n".encode("utf-8")
        return f"#{req_id} {verb}\n".encode("utf-8")

    def _parse_line(self, line: str) -> tuple[int, str, dict | None]:
        """Parse #<id> <verb> [<json-payload>] from a raw line.

        Returns:
            Tuple of (request_id, verb, payload_dict_or_None)
        """
        if not line.startswith("#"):
            raise RuntimeError(f"line missing # prefix: {line[:80]}")
        rest = line[1:]
        id_str, _, body = rest.partition(" ")
        req_id = int(id_str)
        verb, _, payload_str = body.partition(" ")
        payload = json.loads(payload_str) if payload_str else None
        return req_id, verb, payload

    def _send_rpc(
        self, fd: int, req_id: int, method: str, params: dict | None = None
    ) -> None:
        """Send a newline-terminated RPC line: #<id> <method> [<json-params>]."""
        line = self._format_line(req_id, method, params)
        if self._tls_mode:
            self._tls_sock.sendall(line)
        else:
            os.write(fd, line)

    def _read_line(
        self, fd: int, buf_attr: str, timeout: float | None = None
    ) -> str | None:
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
            nl_pos = buf.find(b"\n")
            if nl_pos >= 0:
                line_bytes = buf[:nl_pos]
                setattr(self, buf_attr, buf[nl_pos + 1 :])
                return line_bytes.decode("utf-8")

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
        """Send RPC to engine and wait for response.

        Sends: #<id> <method> [<json-params>]
        Expects: #<id> ok [<json>] or #<id> error [<json>]

        In TLS mode, reads from the shared connection. If an inbound request
        arrives instead of the expected response, it is queued for _serve_one.
        """
        req_id = self._next_id()
        self._send_rpc(self._engine_fd, req_id, method, params)

        # Read lines until we get the response for our request.
        while True:
            if self._tls_mode:
                line = self._read_tls_line(timeout=30.0)
            else:
                line = self._read_line(self._engine_fd, "_engine_buf")
            if line is None:
                raise RuntimeError(f"no response for {method}")

            resp_id, verb, payload = self._parse_line(line)

            # In TLS mode, we might receive inbound requests (engine calling us)
            # while waiting for our response. Queue them.
            if self._tls_mode and verb not in ("ok", "error"):
                self._pending_requests.append((resp_id, verb, payload))
                continue

            if verb == "error":
                msg = ""
                if payload:
                    msg = payload.get("message", str(payload))
                raise RuntimeError(f"RPC error from {method}: {msg}")
            # Wrap payload in {"result": ...} envelope for backward compatibility.
            return {"result": payload}

    def _respond_ok(self, req_id: int) -> None:
        """Send OK response: #<id> ok."""
        line = self._format_line(req_id, "ok")
        if self._tls_mode:
            self._tls_sock.sendall(line)
        else:
            os.write(self._callback_fd, line)

    def _respond_result(self, req_id: int, result: dict) -> None:
        """Send OK response with JSON result: #<id> ok <json>."""
        line = self._format_line(req_id, "ok", result)
        if self._tls_mode:
            self._tls_sock.sendall(line)
        else:
            os.write(self._callback_fd, line)

    def _handle_callback(self, req_id: int, method: str, params: dict | None) -> None:
        """Handle an inbound callback RPC, respond, and buffer any events.

        Centralizes callback dispatch for both the _pending_requests drain
        and the main read loop in read_line().
        """
        if method == "ze-plugin-callback:deliver-batch":
            self._respond_ok(req_id)
            if params:
                events = params.get("events", [])
                for event in events:
                    self._pending_events.append(
                        json.dumps(event, separators=(",", ":"))
                        if isinstance(event, dict)
                        else str(event)
                    )
        elif method == "ze-plugin-callback:deliver-event":
            self._respond_ok(req_id)
            if params:
                self._pending_events.append(params.get("event", ""))
        elif method == "ze-plugin-callback:bye":
            self._respond_ok(req_id)
            self._shutdown = True
        elif method == "ze-plugin-callback:filter-update":
            if self._filter_handler and params:
                result = self._filter_handler(params)
                self._respond_result(req_id, result)
            else:
                self._respond_result(req_id, {"action": "accept"})
        elif method == "ze-plugin-callback:config-verify":
            # Bridge dispatches config-verify during reload transactions.
            # Plugin default is accept, matching the Go SDK contract.
            if self._config_verify_handler:
                result = self._config_verify_handler(params or {})
                self._respond_result(req_id, result)
            else:
                self._respond_result(req_id, {"status": "ok"})
        elif method == "ze-plugin-callback:config-apply":
            if self._config_apply_handler:
                result = self._config_apply_handler(params or {})
                self._respond_result(req_id, result)
            else:
                self._respond_result(req_id, {"status": "ok"})
        elif method == "ze-plugin-callback:config-rollback":
            if self._config_rollback_handler:
                self._config_rollback_handler(params or {})
            self._respond_ok(req_id)
        else:
            self._respond_ok(req_id)

    def _serve_one(self, expected_method: str, timeout: float = 10.0) -> dict | None:
        """Read one RPC request, verify method, respond OK.

        In TLS mode, checks the pending request queue first (requests that
        arrived while _call_engine was waiting for a response).
        """
        # Check pending queue first (TLS mode muxing).
        if self._tls_mode and self._pending_requests:
            req_id, method, params = self._pending_requests.pop(0)
            if method != expected_method:
                raise RuntimeError(f"expected {expected_method}, got {method}")
            self._respond_ok(req_id)
            return params

        if self._tls_mode:
            line = self._read_tls_line(timeout=timeout)
        else:
            line = self._read_line(self._callback_fd, "_callback_buf", timeout=timeout)
        if line is None:
            raise RuntimeError(f"timeout waiting for {expected_method}")

        req_id, method, params = self._parse_line(line)
        if method != expected_method:
            raise RuntimeError(f"expected {expected_method}, got {method}")

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

    def declare_family(
        self, afi: str, safi: str | None = None, mode: str = "both"
    ) -> None:
        """Declare an address family (Stage 1).

        Args:
            afi: Address Family Identifier (ipv4, ipv6, l2vpn, or 'all')
            safi: Subsequent AFI (unicast, multicast, flow, etc.)
            mode: 'encode', 'decode', or 'both'
        """
        if afi == "all":
            # 'all' is not valid in YANG RPC, skip (engine handles all families)
            return
        name = f"{afi}/{safi}" if safi else afi
        self._families.append({"name": name, "mode": mode})

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
        self._commands.append({"name": command})

    def declare_connection_handler(
        self, handler_type: str = "listen", port: int = 0, address: str = ""
    ) -> None:
        """Declare a connection handler for listen socket handoff (Stage 1).

        The engine will create a listen socket on the specified port and
        send the fd via SCM_RIGHTS on callback connection after Stage 1.
        Call receive_listener() after declare_done() to receive the fd.

        Args:
            handler_type: Handler type ('listen' for Mode A)
            port: TCP port to listen on (1-65535)
            address: Bind address (empty = all interfaces)
        """
        self._connection_handlers.append(
            {
                "type": handler_type,
                "port": port,
                "address": address,
            }
        )

    def declare_filter(
        self,
        name: str,
        direction: str = "both",
        attributes: list[str] | None = None,
        on_error: str = "reject",
        overrides: list[str] | None = None,
    ) -> None:
        """Declare a named route filter (Stage 1).

        Args:
            name: Filter name (referenced in config as <plugin>:<name>)
            direction: 'import', 'export', or 'both'
            attributes: Attribute names to receive (empty = all)
            on_error: 'reject' (fail-closed) or 'accept' (fail-open)
            overrides: Default filters this filter replaces
        """
        f: dict[str, Any] = {"name": name, "direction": direction, "on-error": on_error}
        if attributes:
            f["attributes"] = attributes
        if overrides:
            f["overrides"] = overrides
        self._filters.append(f)

    def on_filter_update(self, handler: Callable[[dict], dict]) -> None:
        """Register a handler for filter-update callbacks (runtime).

        The handler receives the filter-update input dict and must return
        a dict with 'action' ('accept', 'reject', or 'modify') and
        optionally 'update' (delta text for modify).

        Args:
            handler: Callback function(input_dict) -> response_dict
        """
        self._filter_handler = handler

    def on_config_verify(self, handler: Callable[[dict], dict]) -> None:
        """Register a handler for config-verify RPCs (runtime, reload).

        The engine-side RPC bridge (internal/component/plugin/server/
        config_tx_bridge.go) dispatches ze-plugin-callback:config-verify
        during reload transactions. The handler receives the full params
        dict (with a 'sections' key carrying the candidate config) and
        must return a dict with 'status' ('ok' or 'error') and optional
        'error' string.

        Args:
            handler: Callback function(params_dict) -> response_dict
        """
        self._config_verify_handler = handler

    def on_config_apply(self, handler: Callable[[dict], dict]) -> None:
        """Register a handler for config-apply RPCs (runtime, reload).

        Same contract as on_config_verify but invoked during the apply
        phase. Return {'status': 'error', 'error': '...'} to trigger
        rollback.

        Args:
            handler: Callback function(params_dict) -> response_dict
        """
        self._config_apply_handler = handler

    def on_config_rollback(self, handler: Callable[[dict], None]) -> None:
        """Register a handler for config-rollback RPCs (runtime, reload).

        Invoked when the orchestrator broadcasts rollback after an apply
        failure. The handler is fire-and-forget; the plugin always
        responds with an empty ok ack (matching the Go SDK default).

        Args:
            handler: Callback function(params_dict) -> None
        """
        self._config_rollback_handler = handler

    def declare_done(self) -> None:
        """Signal Stage 1 declaration complete.

        Sends ze-plugin-engine:declare-registration RPC with all
        accumulated declarations.
        """
        params: dict[str, Any] = {}
        if self._families:
            params["families"] = self._families
        if self._commands:
            params["commands"] = self._commands
        if self._wants_config:
            params["wants-config"] = self._wants_config
        if self._connection_handlers:
            params["connection-handlers"] = self._connection_handlers
        if self._filters:
            params["filters"] = self._filters

        self._call_engine("ze-plugin-engine:declare-registration", params)

    def receive_listener(self) -> socket.socket:
        """Receive a listen socket fd from the engine via SCM_RIGHTS on callback connection.

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
            fds = array.array("i")
            msg, ancdata, _flags, _addr = sock.recvmsg(
                1, socket.CMSG_SPACE(fds.itemsize)
            )
            if not msg:
                raise RuntimeError("no data received (connection closed?)")
            for cmsg_level, cmsg_type, cmsg_data in ancdata:
                if cmsg_level == socket.SOL_SOCKET and cmsg_type == socket.SCM_RIGHTS:
                    received_fds = array.array("i", cmsg_data)
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
            raise RuntimeError("no fd in control message")
        finally:
            # Close the dup'd socket -- the original _callback_fd is unaffected.
            sock.close()

    # ==================================================================
    # Stage 2: Config Delivery
    # ==================================================================

    def wait_for_config(self, timeout: float = 10.0) -> list[dict]:
        """Wait for config delivery from ZeBGP (Stage 2).

        Reads ze-plugin-callback:configure RPC from callback connection.

        Args:
            timeout: Maximum time to wait

        Returns:
            List of config sections, each with 'root' and 'data' keys
        """
        params = self._serve_one("ze-plugin-callback:configure", timeout=timeout)
        if params is None:
            return []
        sections = params.get("sections", []) or []
        # Convert to compatible format
        configs = []
        for section in sections:
            configs.append(
                {
                    "context": section.get("root", ""),
                    "name": "",
                    "value": section.get("data", ""),
                    "root": section.get("root", ""),
                    "data": section.get("data", ""),
                }
            )
        return configs

    # ==================================================================
    # Stage 3: Capability Declaration
    # ==================================================================

    def declare_capability(
        self, code: int, payload: str, encoding: str = "b64"
    ) -> None:
        """Declare a capability for OPEN messages (Stage 3).

        Args:
            code: Capability type code
            payload: Encoded capability value
            encoding: Encoding of payload (b64, hex, or text)
        """
        self._capabilities.append(
            {
                "code": code,
                "encoding": encoding,
                "payload": payload,
            }
        )

    def capability_done(self) -> None:
        """Signal Stage 3 capability declaration complete.

        Sends ze-plugin-engine:declare-capabilities RPC.
        """
        params = {"capabilities": self._capabilities}
        self._call_engine("ze-plugin-engine:declare-capabilities", params)

    # ==================================================================
    # Stage 4: Registry Sharing
    # ==================================================================

    def wait_for_registry(self, timeout: float = 10.0) -> dict:
        """Wait for registry sharing from ZeBGP (Stage 4).

        Reads ze-plugin-callback:share-registry RPC from callback connection.

        Args:
            timeout: Maximum time to wait

        Returns:
            Dict with 'name' (empty in YANG RPC) and 'commands' list
        """
        params = self._serve_one("ze-plugin-callback:share-registry", timeout=timeout)
        commands = []
        if params:
            for cmd in params.get("commands", []) or []:
                commands.append(
                    {
                        "plugin": cmd.get("plugin", ""),
                        "encoding": cmd.get("encoding", ""),
                        "command": cmd.get("name", ""),
                        "name": cmd.get("name", ""),
                    }
                )
        return {"name": self._plugin_name, "commands": commands}

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
            params["subscribe"] = self._subscription
        self._call_engine("ze-plugin-engine:ready", params)

    # ==================================================================
    # Runtime: Send commands
    # ==================================================================

    def flush(self, msg: str) -> None:
        """Write message (strip trailing newline, route to appropriate RPC).

        Args:
            msg: Message to send (trailing newline stripped)
        """
        msg = msg.rstrip("\n")
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

        if command.startswith("subscribe "):
            self._handle_subscribe_command(command)
        elif command.startswith("peer ") or command.startswith("update "):
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
            if parts[i] == "event":
                i += 1
                continue
            events.append(parts[i])
            i += 1

        if events:
            self._subscription = {
                "events": events,
                "peers": [],
                "format": "",
            }

    def subscribe(
        self, events: list[str], peers: list[str] | None = None, fmt: str = ""
    ) -> None:
        """Set event subscriptions for the ready RPC.

        Must be called before ready(). Subscriptions are included
        atomically in the ready RPC to avoid race conditions.

        Args:
            events: Event types ('update', 'open', 'state', etc.)
            peers: Optional peer filter (empty = all)
            fmt: Wire format ('hex', 'parsed', etc.)
        """
        self._subscription = {
            "events": events,
            "peers": peers or [],
            "format": fmt,
        }

    def _send_update_route(self, command: str) -> None:
        """Send ze-plugin-engine:update-route RPC.

        Args:
            command: Full command string (e.g., 'peer * update text ...')
        """
        # Extract peer selector from 'peer <selector> <rest>'
        peer_selector = "*"
        cmd = command
        if command.startswith("peer "):
            rest = command[len("peer ") :]
            # Find the next space-separated token as peer selector
            parts = rest.split(" ", 1)
            peer_selector = parts[0]
            cmd = parts[1] if len(parts) > 1 else ""

        params = {
            "peer-selector": peer_selector,
            "command": cmd,
        }
        self._call_engine("ze-plugin-engine:update-route", params)

    # ==================================================================
    # Runtime: Read events / responses
    # ==================================================================

    def read_line(self, timeout: float = 0.1) -> str | None:
        """Read the next event from callback connection.

        Handles incoming RPC callbacks:
        - deliver-event: extracts event JSON string, responds OK, returns event
        - bye: responds OK, marks shutdown, returns None
        - other methods: responds OK, skips

        Args:
            timeout: Seconds to wait for data

        Returns:
            Event JSON string, or None if no event available
        """
        # Iterative dispatch loop: handles pending requests, buffered events,
        # and incoming RPCs without recursion (avoids stack overflow under
        # sustained filter-update or unknown callback streams).
        while True:
            if self._shutdown:
                return None

            # Drain RPCs that arrived during _call_engine() (TLS muxing).
            # When _call_engine() reads a callback (e.g. deliver-batch) before
            # the expected response, it queues the callback in _pending_requests.
            # Process them here so they are not lost.
            while self._tls_mode and self._pending_requests:
                req_id, method, params = self._pending_requests.pop(0)
                self._handle_callback(req_id, method, params)

            # Return buffered events from a previous deliver-batch first.
            if self._pending_events:
                return self._pending_events.pop(0)

            if self._shutdown:
                return None

            if self._tls_mode:
                raw = self._read_tls_line(timeout=timeout)
            else:
                raw = self._read_line(
                    self._callback_fd, "_callback_buf", timeout=timeout
                )
            if raw is None:
                return None

            req_id, method, params = self._parse_line(raw)

            if method in (
                "ze-plugin-callback:deliver-event",
                "ze-plugin-callback:deliver-batch",
            ):
                self._handle_callback(req_id, method, params)
                # Events were buffered; loop back to return from _pending_events.
                continue

            if method == "ze-plugin-callback:bye":
                self._handle_callback(req_id, method, params)
                # Flush any buffered events before returning None.
                if self._pending_events:
                    return self._pending_events.pop(0)
                return None

            if method == "ze-plugin-callback:filter-update":
                self._handle_callback(req_id, method, params)
                # Filter handled; loop back to read next event.
                continue

            # Other callbacks -- respond OK and skip, loop back.
            self._respond_ok(req_id)
            continue

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
        if line.startswith("{"):
            try:
                data = json.loads(line)
                return data.get("answer")
            except (json.JSONDecodeError, TypeError):
                return None
        if line in ("done", "error", "shutdown"):
            return line
        return None

    def wait_for_ack(self, expected_count: int = 1, timeout: float = 2.0) -> bool:
        """Wait for route delivery after send().

        Sends a ze-bgp:peer-flush RPC that blocks until all forward pool
        workers have drained their queued items to peer sockets, then adds
        a short delay for ze-peer cmd=api command interleaving.

        Args:
            expected_count: Number of routes sent (scales post-flush delay)
            timeout: Timeout in seconds (unused, kept for API compat)

        Returns:
            True (always succeeds)
        """
        import time

        try:
            self._call_engine("ze-bgp:peer-flush", {"selector": "*"})
        except RuntimeError:
            pass
        # After flush confirms forward pool drained, allow time for
        # ze-peer cmd=api commands to be interleaved and processed.
        # The flush guarantees OUR routes are on the wire, but ze-peer
        # may still be sending its own commands (e.g., EOR via cmd=api).
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

        Processes incoming RPC callbacks on callback connection until bye is received,
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

    def fail(self, message: str, encoding: str = "text") -> None:
        """Signal startup failure to ZeBGP.

        In YANG RPC, errors are signaled via RPC error responses.
        This raises an exception to abort the plugin.

        Args:
            message: Error message
            encoding: Encoding of message (ignored in YANG RPC)
        """
        raise RuntimeError(f"plugin startup failed: {message}")


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


def declare_family(afi: str, safi: str | None = None, mode: str = "both") -> None:
    """Declare an address family (Stage 1)."""
    _get_api().declare_family(afi, safi, mode)


def declare_config(pattern: str) -> None:
    """Declare a config pattern hook (Stage 1)."""
    _get_api().declare_config(pattern)


def declare_command(command: str) -> None:
    """Declare a command handler (Stage 1)."""
    _get_api().declare_command(command)


def declare_connection_handler(
    handler_type: str = "listen", port: int = 0, address: str = ""
) -> None:
    """Declare a connection handler for listen socket handoff (Stage 1)."""
    _get_api().declare_connection_handler(handler_type, port, address)


def receive_listener() -> socket.socket:
    """Receive a listen socket fd from the engine via SCM_RIGHTS."""
    return _get_api().receive_listener()


def declare_filter(
    name: str,
    direction: str = "both",
    attributes: list[str] | None = None,
    on_error: str = "reject",
    overrides: list[str] | None = None,
) -> None:
    """Declare a named route filter (Stage 1)."""
    _get_api().declare_filter(name, direction, attributes, on_error, overrides)


def on_filter_update(handler: Callable[[dict], dict]) -> None:
    """Register a handler for filter-update callbacks."""
    _get_api().on_filter_update(handler)


def declare_done() -> None:
    """Signal Stage 1 declaration complete."""
    _get_api().declare_done()


def wait_for_config(timeout: float = 10.0) -> list[dict]:
    """Wait for config delivery from ZeBGP (Stage 2)."""
    return _get_api().wait_for_config(timeout)


def declare_capability(code: int, payload: str, encoding: str = "b64") -> None:
    """Declare a capability for OPEN messages (Stage 3)."""
    _get_api().declare_capability(code, payload, encoding)


def capability_done() -> None:
    """Signal Stage 3 capability declaration complete."""
    _get_api().capability_done()


def wait_for_registry(timeout: float = 10.0) -> dict:
    """Wait for registry sharing from ZeBGP (Stage 4)."""
    return _get_api().wait_for_registry(timeout)


def subscribe(events: list[str], peers: list[str] | None = None, fmt: str = "") -> None:
    """Set event subscriptions for ready RPC."""
    _get_api().subscribe(events, peers, fmt)


def fail(message: str, encoding: str = "text") -> None:
    """Signal startup failure to ZeBGP."""
    _get_api().fail(message, encoding)


# Sentinel string recognised by the .ci test runner. When this prefix appears
# on ze's relayed stderr, the runner forces the test to FAIL regardless of
# ze's own exit code. See internal/test/runner/runner_validate.go and
# .claude/known-failures.md (section 8) for background. Keeping the literal
# in code and in the runner makes this a two-point coupling; change both.
_OBSERVER_FAIL_SENTINEL = "ZE-OBSERVER-FAIL"


def runtime_fail(message: str) -> None:
    """Signal a runtime assertion failure from a Python observer plugin.

    The ``dispatch daemon shutdown ; sys.exit(1)`` pattern used by many older
    tests is a silent no-op: ze handles ``daemon shutdown`` cleanly and exits
    with code 0, the observer's ``sys.exit(1)`` never propagates to the test
    runner, and the runner reports success. This helper replaces that pattern:

    1. Emit a slog-formatted ERROR line on the observer's stderr with the
       ``ZE-OBSERVER-FAIL`` sentinel in ``msg=``. The engine relays plugin
       stderr; ERROR-level lines always pass classifyStderrLine regardless of
       ``ze.log.relay`` so the line reaches the runner.
    2. Request a clean ``daemon shutdown`` so ze stops and the runner unblocks.
    3. ``sys.exit(1)`` defensively (unreachable after the sentinel has been
       flushed, but kept so a reader understands intent).

    The runner's ``validateLogging`` applies an implicit reject check for the
    sentinel, so ANY test using this helper automatically fails when the
    observer signals failure -- no per-test ``reject=stderr:pattern=`` needed.

    Args:
        message: Short human-readable reason. Interpolated into the slog
            ``msg=`` field between the sentinel and any trailing context.
    """
    # slog format is `time=... level=... msg="..." key=value ...`. Level must
    # be present for classifyStderrLine to treat it as "valid slog" and pass
    # the relay filter. The subsystem attr identifies the source in telemetry.
    line = (
        f"time=runtime level=ERROR "
        f'msg="{_OBSERVER_FAIL_SENTINEL}: {message}" '
        f"subsystem=test.observer\n"
    )
    sys.stderr.write(line)
    sys.stderr.flush()
    try:
        _get_api()._call_engine(
            "ze-plugin-engine:dispatch-command",
            {"command": "daemon shutdown"},
        )
    except Exception:  # noqa: BLE001  # best-effort shutdown after fatal signal
        pass
    try:
        _get_api().wait_for_shutdown(timeout=5.0)
    except Exception:  # noqa: BLE001  # best-effort wait after fatal signal
        pass
    sys.exit(1)
